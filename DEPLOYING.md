# Deployment Guide — Vercel + Render

This guide covers deploying Multica to Vercel (frontend) and Render (backend). Deployments trigger automatically on every push to `main` via GitHub Actions after CI passes.

> **Self-hosting on your own infrastructure?** See [SELF_HOSTING.md](SELF_HOSTING.md) instead.

---

## Overview

| Layer | Platform | Trigger |
|-------|----------|---------|
| Frontend (Next.js) | Vercel | Push to `main` via GitHub Actions |
| Backend (Go API) | Render | Push to `main` via Render deploy hook |
| Database (PostgreSQL) | Render Managed Postgres | Provisioned via `render.yaml` |

Migrations run automatically on each backend deploy via the Docker entrypoint.

---

## 1. Render — Backend + Database

### a) Connect the repo

1. Go to [render.com](https://render.com) → **New** → **Blueprint**
2. Connect your GitHub repo and select the branch (`main`)
3. Render detects `render.yaml` and creates:
   - `multica-api` — Go backend web service (Docker)
   - `multica-db` — managed PostgreSQL

### b) Set environment secrets

After the Blueprint is created, go to each service's **Environment** tab and set the `sync: false` variables:

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Long random string — `openssl rand -hex 32` |
| `RESEND_API_KEY` | [Resend](https://resend.com) API key (or leave empty to log codes) |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID (optional) |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret (optional) |
| `GOOGLE_REDIRECT_URI` | `https://<your-vercel-url>/auth/callback` |
| `FRONTEND_ORIGIN` | Your Vercel URL, e.g. `https://your-app.vercel.app` |
| `S3_BUCKET` | S3 bucket name (optional, for file uploads) |
| `CLOUDFRONT_DOMAIN` | CloudFront domain (optional) |
| `CLOUDFRONT_KEY_PAIR_ID` | CloudFront key pair ID (optional) |
| `CLOUDFRONT_PRIVATE_KEY` | CloudFront private key PEM (optional) |
| `COOKIE_DOMAIN` | Cookie domain, e.g. `your-app.vercel.app` (optional) |

`DATABASE_URL` is injected automatically from `multica-db`.

### c) Get the deploy hook URL

1. Go to the `multica-api` service → **Settings** → **Deploy Hook**
2. Copy the URL — you'll add it as a GitHub secret in step 3

### d) Note your backend URL

After the first deploy, your API will be at something like:
```
https://multica-api.onrender.com
```

---

## 2. Vercel — Frontend

### a) Create the project

1. Go to [vercel.com](https://vercel.com) → **Add New Project**
2. Import your GitHub repo
3. In **Configure Project**:
   - **Framework Preset:** Next.js
   - **Root Directory:** `apps/web`
   - **Build Command:** leave as default (`next build`)
   - **Output Directory:** leave as default

### b) Set environment variables

In **Settings → Environment Variables**, add:

| Variable | Value |
|----------|-------|
| `REMOTE_API_URL` | Your Render backend URL, e.g. `https://multica-api.onrender.com` |
| `NEXT_PUBLIC_API_URL` | Same as `REMOTE_API_URL` |
| `NEXT_PUBLIC_WS_URL` | `wss://multica-api.onrender.com/ws` |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | Your Google OAuth client ID (optional) |

> `REMOTE_API_URL` tells Next.js's rewrite rules where to proxy `/api`, `/ws`, and `/auth` requests.

### c) Get Vercel credentials for GitHub Actions

You need three values from Vercel to wire up the deploy workflow:

1. **`VERCEL_TOKEN`** — [vercel.com/account/tokens](https://vercel.com/account/tokens) → create a token
2. **`VERCEL_ORG_ID`** — Project → **Settings** → **General** → Team ID (or Personal Account ID)
3. **`VERCEL_PROJECT_ID`** — Project → **Settings** → **General** → Project ID

---

## 3. GitHub Actions Secrets

In your GitHub repo → **Settings → Secrets and variables → Actions**, add:

| Secret | Description |
|--------|-------------|
| `VERCEL_TOKEN` | Vercel API token |
| `VERCEL_ORG_ID` | Vercel org/team ID |
| `VERCEL_PROJECT_ID` | Vercel project ID |
| `RENDER_DEPLOY_HOOK_URL` | Render deploy hook URL for `multica-api` |
| `REMOTE_API_URL` | Render backend URL (used during Vercel build) |
| `NEXT_PUBLIC_API_URL` | Render backend URL |
| `NEXT_PUBLIC_WS_URL` | `wss://multica-api.onrender.com/ws` |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | Google OAuth client ID (optional) |

---

## 4. Deploy

Push to `main`:

```bash
git push origin main
```

The `deploy` workflow (`deploy.yml`) will:

1. Run the full CI suite (frontend build + typecheck + tests, Go build + migrations + tests)
2. On success, deploy the frontend to Vercel and trigger the Render deploy hook in parallel

Monitor progress in **GitHub → Actions → Deploy**.

---

## Connecting the CLI to Your Deployment

Once both services are live, point the CLI at your production server:

```bash
multica config set server_url https://multica-api.onrender.com
multica config set app_url https://your-app.vercel.app
multica login
multica daemon start
```

---

## Environment Variable Reference

For a full list of all supported environment variables, see [SELF_HOSTING_ADVANCED.md](SELF_HOSTING_ADVANCED.md).

---

## Troubleshooting

**Backend deploy fails at migrations**
- Check Render logs: **multica-api → Logs**
- Ensure `DATABASE_URL` is set (auto-injected from `multica-db` if using `render.yaml`)

**Frontend can't reach the API**
- Verify `REMOTE_API_URL` is set in Vercel environment variables
- Check that `FRONTEND_ORIGIN` on Render matches your Vercel URL exactly (no trailing slash)

**Auth redirects to wrong URL**
- `GOOGLE_REDIRECT_URI` must match the URI registered in Google Cloud Console
- Should be: `https://<your-vercel-url>/auth/callback`

**Vercel deploy skipped in GitHub Actions**
- Check that all three `VERCEL_*` secrets are set correctly in GitHub
- Run `vercel whoami --token=<your-token>` locally to validate the token
