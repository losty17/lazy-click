# lazy-click oauth backend

Dead-simple FastAPI service used to manage OAuth token exchange for external integrations.

This service is deployed separately from the Go TUI binary.

## Why

`CLICKUP_CLIENT_SECRET` must not be shipped in the desktop binary.
The backend holds provider secrets and performs token exchange server-side.

## Endpoints

- `GET /health`
- `GET /oauth/clickup/begin`
  - creates a short-lived OAuth session
  - returns `auth_url` + `session_id`
- `GET /oauth/clickup/callback`
  - receives ClickUp redirect
  - exchanges auth code for access token
- `GET /oauth/clickup/complete?session_id=...`
  - returns `202` while pending
  - returns `{ "provider": "clickup", "access_token": "..." }` when done

## Run locally

```bash
cd backend
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env
uvicorn app.main:app --host 127.0.0.1 --port 8787 --reload
```

## Run with Docker

From repository root:

```bash
cp backend/.env.example backend/.env
docker compose up -d --build oauth-backend
```

Stop:

```bash
docker compose down
```

## Env vars

- `CLICKUP_CLIENT_ID` (required)
- `CLICKUP_CLIENT_SECRET` (required)
- `OAUTH_BACKEND_BASE_URL` (required for callback URL generation, e.g. `https://oauth.yourdomain.com`)

## TUI integration

Set `LAZY_CLICK_OAUTH_BACKEND_URL` in the Go app environment to point to this service.
