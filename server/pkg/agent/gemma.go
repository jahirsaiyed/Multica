package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	gemmaAPIBaseURL    = "https://generativelanguage.googleapis.com/v1beta/models"
	gemmaDefaultModel  = "gemma-4-26b-a4b-it"
	gemmaDefaultTimeout = 20 * time.Minute
)

// gemmaBackend implements Backend by calling Google's Generative Language HTTP API.
// Unlike the other backends, it is stateless and does not spawn a subprocess.
type gemmaBackend struct {
	cfg Config
}

func (b *gemmaBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		// Resolve API key.
		apiKey := strings.TrimSpace(os.Getenv("MULTICA_GEMMA_API_KEY"))
		if apiKey == "" {
			resCh <- Result{
				Status:     "failed",
				Error:      "MULTICA_GEMMA_API_KEY is not set",
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		// Resolve model.
		model := opts.Model
		if model == "" {
			model = strings.TrimSpace(os.Getenv("MULTICA_GEMMA_MODEL"))
		}
		if model == "" {
			model = gemmaDefaultModel
		}

		// Apply timeout.
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = gemmaDefaultTimeout
		}
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Warn about unsupported options (non-fatal).
		if opts.ResumeSessionID != "" {
			b.cfg.Logger.Warn("gemma: session resumption is not supported (HTTP API is stateless), starting fresh")
		}
		if opts.MaxTurns > 0 {
			b.cfg.Logger.Debug("gemma: MaxTurns is ignored (single-turn HTTP completion)")
		}
		if opts.Cwd != "" {
			b.cfg.Logger.Debug("gemma: Cwd is ignored (HTTP API has no working directory)")
		}

		// Build request body.
		reqBody := gemmaRequest{
			Contents: []gemmaContent{
				{
					Role:  "user",
					Parts: []gemmaPart{{Text: prompt}},
				},
			},
		}
		if opts.SystemPrompt != "" {
			reqBody.SystemInstruction = &gemmaSystemContent{
				Parts: []gemmaPart{{Text: opts.SystemPrompt}},
			}
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("gemma: marshal request: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		url := fmt.Sprintf("%s/%s:streamGenerateContent", gemmaAPIBaseURL, model)
		httpReq, err := http.NewRequestWithContext(runCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("gemma: build request: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-goog-api-key", apiKey)

		b.cfg.Logger.Info("gemma: sending request", "model", model, "url", url)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			finalStatus := "failed"
			finalError := fmt.Sprintf("gemma: HTTP request failed: %v", err)
			if runCtx.Err() == context.DeadlineExceeded {
				finalStatus = "timeout"
				finalError = fmt.Sprintf("gemma: timed out after %s", timeout)
			} else if runCtx.Err() == context.Canceled {
				finalStatus = "aborted"
				finalError = "execution cancelled"
			}
			resCh <- Result{
				Status:     finalStatus,
				Error:      finalError,
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("gemma: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		// Stream-parse the response body.
		// Google's streaming format is a JSON array split across lines:
		//   [
		//   {"candidates":[...]},
		//   ,{"candidates":[...],"usageMetadata":{...}}
		//   ]
		// Each element may also be prefixed with "data: " (SSE resilience).
		var output strings.Builder
		var lastUsage *gemmaUsageMetadata

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// Strip SSE prefix if present.
			if after, ok := strings.CutPrefix(line, "data: "); ok {
				line = after
			}

			// Strip JSON array envelope characters.
			line = strings.TrimSpace(line)
			line = strings.TrimLeft(line, "[,")
			line = strings.TrimRight(line, "]")
			line = strings.TrimSpace(line)

			if line == "" {
				continue
			}

			var chunk gemmaChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			// Capture usage metadata (last chunk usually has the final counts).
			if chunk.UsageMetadata != nil {
				lastUsage = chunk.UsageMetadata
			}

			// Emit text from each candidate's parts.
			for _, candidate := range chunk.Candidates {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						output.WriteString(part.Text)
						trySend(msgCh, Message{Type: MessageText, Content: part.Text})
					}
				}
			}
		}

		duration := time.Since(startTime)

		// Determine final status.
		finalStatus := "completed"
		var finalError string
		if scanErr := scanner.Err(); scanErr != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				finalStatus = "timeout"
				finalError = fmt.Sprintf("gemma: timed out after %s", timeout)
			} else if runCtx.Err() == context.Canceled {
				finalStatus = "aborted"
				finalError = "execution cancelled"
			} else {
				finalStatus = "failed"
				finalError = fmt.Sprintf("gemma: read response: %v", scanErr)
			}
		} else if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("gemma: timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		}

		b.cfg.Logger.Info("gemma: finished", "model", model, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		// Build usage map.
		var usageMap map[string]TokenUsage
		if lastUsage != nil && (lastUsage.PromptTokenCount > 0 || lastUsage.CandidatesTokenCount > 0) {
			usageMap = map[string]TokenUsage{
				model: {
					InputTokens:  lastUsage.PromptTokenCount,
					OutputTokens: lastUsage.CandidatesTokenCount,
				},
			}
		}

		resCh <- Result{
			Status:     finalStatus,
			Output:     output.String(),
			Error:      finalError,
			DurationMs: duration.Milliseconds(),
			Usage:      usageMap,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// ── Gemma API JSON types ──

type gemmaRequest struct {
	Contents          []gemmaContent      `json:"contents"`
	SystemInstruction *gemmaSystemContent `json:"systemInstruction,omitempty"`
}

type gemmaSystemContent struct {
	Parts []gemmaPart `json:"parts"`
}

type gemmaContent struct {
	Role  string      `json:"role,omitempty"`
	Parts []gemmaPart `json:"parts"`
}

type gemmaPart struct {
	Text string `json:"text"`
}

type gemmaChunk struct {
	Candidates    []gemmaCandidate    `json:"candidates"`
	UsageMetadata *gemmaUsageMetadata `json:"usageMetadata"`
	ModelVersion  string              `json:"modelVersion"`
}

type gemmaCandidate struct {
	Content      gemmaContent `json:"content"`
	FinishReason string       `json:"finishReason"`
}

type gemmaUsageMetadata struct {
	PromptTokenCount     int64 `json:"promptTokenCount"`
	CandidatesTokenCount int64 `json:"candidatesTokenCount"`
}
