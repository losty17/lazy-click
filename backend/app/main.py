import os
import secrets
import threading
from urllib.parse import urlencode
from dataclasses import dataclass
from typing import Dict, Optional

import httpx
from fastapi import FastAPI, HTTPException, Query
from fastapi.responses import HTMLResponse


CLICKUP_AUTH_URL = "https://app.clickup.com/api"
CLICKUP_TOKEN_URL = "https://api.clickup.com/api/v2/oauth/token"


@dataclass
class OAuthSession:
    session_id: str
    state: str
    status: str
    code: Optional[str] = None
    token: Optional[str] = None
    error: Optional[str] = None


app = FastAPI(title="lazy-click oauth backend", version="0.1.0")
_lock = threading.Lock()
_sessions_by_id: Dict[str, OAuthSession] = {}
_session_id_by_state: Dict[str, str] = {}


def _required_env(key: str) -> str:
    value = os.getenv(key, "").strip()
    if not value:
        raise RuntimeError(f"Missing required env var: {key}")
    return value


def _public_base_url() -> str:
    return os.getenv("OAUTH_BACKEND_BASE_URL", "http://127.0.0.1:8787").rstrip("/")


@app.get("/health")
def health() -> dict:
    return {"ok": True}


@app.get("/oauth/clickup/begin")
def begin_clickup_oauth() -> dict:
    client_id = _required_env("CLICKUP_CLIENT_ID")
    session_id = secrets.token_urlsafe(24)
    state = secrets.token_urlsafe(24)
    redirect_uri = f"{_public_base_url()}/oauth/clickup/callback"

    session = OAuthSession(session_id=session_id, state=state, status="pending")
    with _lock:
        _sessions_by_id[session_id] = session
        _session_id_by_state[state] = session_id

    auth_url = f"{CLICKUP_AUTH_URL}?" + urlencode(
        {
            "client_id": client_id,
            "redirect_uri": redirect_uri,
            "state": state,
            "response_type": "code",
        }
    )
    return {"auth_url": auth_url, "session_id": session_id}


@app.get("/oauth/clickup/callback", response_class=HTMLResponse)
async def clickup_callback(
    state: str = Query(default=""),
    code: str = Query(default=""),
    error: str = Query(default=""),
) -> HTMLResponse:
    if not state:
        raise HTTPException(status_code=400, detail="missing state")

    with _lock:
        session_id = _session_id_by_state.get(state)
        session = _sessions_by_id.get(session_id) if session_id else None

    if not session:
        raise HTTPException(status_code=400, detail="unknown oauth session")

    if error:
        with _lock:
            session.status = "failed"
            session.error = error
        return HTMLResponse("ClickUp authorization failed. You can close this window.", status_code=400)

    if not code:
        with _lock:
            session.status = "failed"
            session.error = "missing code"
        return HTMLResponse("ClickUp callback missing authorization code.", status_code=400)

    client_id = _required_env("CLICKUP_CLIENT_ID")
    client_secret = _required_env("CLICKUP_CLIENT_SECRET")

    payload = {"client_id": client_id, "client_secret": client_secret, "code": code}
    async with httpx.AsyncClient(timeout=20.0) as client:
        response = await client.post(CLICKUP_TOKEN_URL, json=payload)

    if response.status_code >= 400:
        with _lock:
            session.status = "failed"
            session.error = f"token exchange failed: {response.status_code} {response.text.strip()}"
        return HTMLResponse("Token exchange failed. Return to lazy-click for details.", status_code=500)

    data = response.json()
    access_token = str(data.get("access_token", "")).strip()
    if not access_token:
        with _lock:
            session.status = "failed"
            session.error = "token exchange succeeded but access_token missing"
        return HTMLResponse("Token exchange response missing token.", status_code=500)

    with _lock:
        session.status = "done"
        session.code = code
        session.token = access_token
        session.error = None

    return HTMLResponse("ClickUp connected. You can close this window and return to lazy-click.")


@app.get("/oauth/clickup/complete")
def complete_clickup_oauth(session_id: str = Query(default="")) -> dict:
    if not session_id:
        raise HTTPException(status_code=400, detail="missing session_id")

    with _lock:
        session = _sessions_by_id.get(session_id)

    if not session:
        raise HTTPException(status_code=404, detail="oauth session not found")
    if session.status == "pending":
        raise HTTPException(status_code=202, detail="pending")
    if session.status == "failed":
        raise HTTPException(status_code=502, detail=session.error or "oauth failed")

    token = session.token or ""
    with _lock:
        _sessions_by_id.pop(session_id, None)
        _session_id_by_state.pop(session.state, None)

    return {"provider": "clickup", "access_token": token}
