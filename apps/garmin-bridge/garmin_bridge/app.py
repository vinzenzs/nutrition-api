"""FastAPI surface: /healthz, /login, /login/mfa, /sync.

Two planes share one process (design D1): the rare interactive login (two calls
holding transient SSO state in memory between them) and the frequent headless
sync (stateless — reads the token fresh from the backend each time). Because the
login handshake keeps state in memory, the deployment runs a single replica.

Route handlers are plain ``def`` (not ``async def``): garminconnect and the
backend client are blocking, and FastAPI runs sync handlers in a threadpool, so
the blocking work never stalls the event loop.
"""

from __future__ import annotations

import logging
from datetime import datetime
from typing import Any
from zoneinfo import ZoneInfo

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from . import garmin_client, logging_setup, sync
from .backend import Backend, BackendError, TokenNotFound
from .config import Config

logger = logging.getLogger("garmin_bridge.app")


class MFARequest(BaseModel):
    code: str


class SyncRequest(BaseModel):
    date: str | None = None


def _today(tz: str) -> str:
    return datetime.now(ZoneInfo(tz)).strftime("%Y-%m-%d")


def create_app(
    config: Config,
    *,
    gc=garmin_client,
    backend_factory=None,
    now=_today,
) -> FastAPI:
    """Build the app. The seams (gc, backend_factory, now) are injectable for
    tests; production uses the real garmin_client and a backend over httpx."""

    if backend_factory is None:
        def backend_factory() -> Backend:
            return Backend(config.nutrition_api_url, config.garmin_api_token)

    app = FastAPI(title="garmin-bridge", version="0.1.0")

    # Transient SSO state between POST /login and POST /login/mfa. Single-replica
    # deployment guarantees the resume call reaches the pod that began login.
    state: dict[str, Any] = {"pending_mfa": None}

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/login")
    def login() -> JSONResponse:
        """Start Garmin SSO with configured credentials (never the request body)."""
        try:
            result, payload = gc.begin_login(config.garmin_email, config.garmin_password)
        except garmin_client.LoginError as exc:
            logger.warning("login failed: %s", exc.code)
            return JSONResponse(status_code=401, content={"error": exc.code, "message": exc.message})

        if result == garmin_client.NEEDS_MFA:
            state["pending_mfa"] = payload
            logger.info("login started; MFA required")
            return JSONResponse(status_code=200, content={"needs_mfa": True})

        # Login completed without MFA — persist immediately.
        return _persist_token(payload, backend_factory)

    @app.post("/login/mfa")
    def login_mfa(req: MFARequest) -> JSONResponse:
        """Resume the in-progress login with the supplied code and persist."""
        pending = state.get("pending_mfa")
        if pending is None:
            return JSONResponse(status_code=409, content={"error": "no_login_in_progress"})

        code = (req.code or "").strip()
        if not code:
            return JSONResponse(status_code=400, content={"error": "mfa_code_required"})

        try:
            token_b64 = gc.resume_login(pending, code)
        except garmin_client.LoginError as exc:
            logger.warning("mfa resume failed: %s", exc.code)
            return JSONResponse(status_code=401, content={"error": exc.code, "message": exc.message})
        finally:
            state["pending_mfa"] = None

        return _persist_token(token_b64, backend_factory)

    @app.post("/sync")
    def do_sync(req: SyncRequest | None = None) -> JSONResponse:
        """Headless: read the stored token, fetch the day, map and POST it."""
        date = (req.date if req and req.date else None) or now(config.sync_tz)

        try:
            with backend_factory() as backend:
                try:
                    token = backend.get_token()
                except TokenNotFound:
                    return JSONResponse(
                        status_code=409,
                        content={"error": "login_required", "message": "no stored Garmin token; run POST /login first"},
                    )
                api = gc.load_api(token.decode("utf-8"))
                raw = gc.fetch_day(api, date)
                summary = sync.sync_day(backend, raw, date)
        except BackendError as exc:
            logger.error("sync backend error: %s", exc)
            return JSONResponse(status_code=502, content={"error": "backend_error", "message": str(exc)})
        except Exception as exc:  # noqa: BLE001
            logger.error("sync failed: %s", exc)
            return JSONResponse(status_code=500, content={"error": "sync_failed", "message": str(exc)})

        status = 200 if summary.get("ok") else 207
        return JSONResponse(status_code=status, content=summary)

    return app


def _persist_token(token_b64: str, backend_factory) -> JSONResponse:
    """PUT the freshly minted token to the backend; never return the blob."""
    # Register the blob as a secret so it can never leak through logs.
    logging_setup.register_secret(token_b64)
    try:
        with backend_factory() as backend:
            backend.put_token(token_b64.encode("utf-8"))
    except Exception as exc:  # noqa: BLE001
        logger.error("token persist failed: %s", type(exc).__name__)
        return JSONResponse(status_code=502, content={"error": "token_persist_failed"})
    logger.info("login complete; token persisted")
    return JSONResponse(status_code=200, content={"logged_in": True})
