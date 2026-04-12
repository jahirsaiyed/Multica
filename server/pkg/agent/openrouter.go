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
	openrouterAPIBaseURL    = "https://openrouter.ai/api/v1/chat/completions"
	openrouterDefaultModel  = "meta-llama/llama-3.3-70b-instruct"
	openrouterDefaultTimeout = 20 * time.Minute
)

// openrouterBackend implements Backend by calling OpenRouter's OpenAI-compatible HTTP API.
// It is stateless and does not spawn a subprocess.
type openrouterBackend struct {
	cfg Config
}

func (b *openrouterBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		// Resolve API key.
		apiKey := strings.TrimSpace(os.Getenv("MULTICA_OPENROUTER_API_KEY"))
		if apiKey == "" {
			resCh <- Result{
				Status:     "failed",
				Error:      "MULTICA_OPENROUTER_API_KEY is not set",
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		// Resolve model.
		model := opts.Model
		if model == "" {
			model = strings.TrimSpace(os.Getenv("MULTICA_OPENROUTER_MODEL"))
		}
		if model == "" {
			model = openrouterDefaultModel
		}

		// Apply timeout.
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = openrouterDefaultTimeout
		}
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Warn about unsupported options (non-fatal).
		if opts.ResumeSessionID != "" {
			b.cfg.Logger.Warn("openrouter: session resumption is not supported (HTTP API is stateless), starting fresh")
		}
		if opts.MaxTurns > 0 {
			b.cfg.Logger.Debug("openrouter: MaxTurns is ignored (single-turn HTTP completion)")
		}
		if opts.Cwd != "" {
			b.cfg.Logger.Debug("openrouter: Cwd is ignored (HTTP API has no working directory)")
		}

		// Build messages array.
		var messages []openrouterMessage
		if opts.SystemPrompt != "" {
			messages = append(messages, openrouterMessage{Role: "system", Content: opts.SystemPrompt})
		}
		messages = append(messages, openrouterMessage{Role: "user", Content: prompt})

		// Build request body.
		reqBody := openrouterRequest{
			Model:    model,
			Stream:   true,
			Messages: messages,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("openrouter: marshal request: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		httpReq, err := http.NewRequestWithContext(runCtx, http.MethodPost, openrouterAPIBaseURL, bytes.NewReader(bodyBytes))
		if err != nil {
			resCh <- Result{
				Status:     "failed",
				Error:      fmt.Sprintf("openrouter: build request: %v", err),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		b.cfg.Logger.Info("openrouter: sending request", "model", model, "url", openrouterAPIBaseURL)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			finalStatus := "failed"
			finalError := fmt.Sprintf("openrouter: HTTP request failed: %v", err)
			if runCtx.Err() == context.DeadlineExceeded {
				finalStatus = "timeout"
				finalError = fmt.Sprintf("openrouter: timed out after %s", timeout)
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
				Error:      fmt.Sprintf("openrouter: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
				DurationMs: time.Since(startTime).Milliseconds(),
			}
			return
		}

		// Stream-parse the SSE response body.
		// Format: lines prefixed with "data: " containing JSON chunks.
		// Final chunk before [DONE] contains usage.
		var output strings.Builder
		var lastUsage *openrouterUsage

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// Strip SSE prefix.
			after, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			line = strings.TrimSpace(after)

			if line == "" || line == "[DONE]" {
				continue
			}

			var chunk openrouterChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			// Capture final usage (only present on the last data chunk before [DONE]).
			if chunk.Usage != nil {
				lastUsage = chunk.Usage
			}

			// Emit text from delta content.
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					output.WriteString(choice.Delta.Content)
					trySend(msgCh, Message{Type: MessageText, Content: choice.Delta.Content})
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
				finalError = fmt.Sprintf("openrouter: timed out after %s", timeout)
			} else if runCtx.Err() == context.Canceled {
				finalStatus = "aborted"
				finalError = "execution cancelled"
			} else {
				finalStatus = "failed"
				finalError = fmt.Sprintf("openrouter: read response: %v", scanErr)
			}
		} else if runCtx.Err() == context.DeadlineExceeded {
			finalStatus = "timeout"
			finalError = fmt.Sprintf("openrouter: timed out after %s", timeout)
		} else if runCtx.Err() == context.Canceled {
			finalStatus = "aborted"
			finalError = "execution cancelled"
		}

		b.cfg.Logger.Info("openrouter: finished", "model", model, "status", finalStatus, "duration", duration.Round(time.Millisecond).String())

		// Build usage map.
		var usageMap map[string]TokenUsage
		if lastUsage != nil && (lastUsage.PromptTokens > 0 || lastUsage.CompletionTokens > 0) {
			usageMap = map[string]TokenUsage{
				model: {
					InputTokens:  int64(lastUsage.PromptTokens),
					OutputTokens: int64(lastUsage.CompletionTokens),
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

// ── OpenRouter API JSON types ──

type openrouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openrouterRequest struct {
	Model    string              `json:"model"`
	Stream   bool                `json:"stream"`
	Messages []openrouterMessage `json:"messages"`
}

type openrouterChunk struct {
	Choices []openrouterChoice `json:"choices"`
	Usage   *openrouterUsage   `json:"usage"`
}

type openrouterChoice struct {
	Delta        openrouterDelta `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type openrouterDelta struct {
	Content string `json:"content"`
}

type openrouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
