"""Thin adapter over garminconnect — the only place Garmin's API is touched.

All the fragile, frequently-broken surface (SSO login, MFA resume, OAuth token
minting, the per-metric fetch endpoints) is isolated here so the rest of the
service stays pure and testable. When Garmin breaks, this file and a
``pip upgrade garminconnect`` are the blast radius.

The two login calls share in-memory SSO state: ``begin_login`` may return a
``needs_mfa`` state object that ``resume_login`` consumes. This is why the
bridge runs as a single replica (design D3).
"""

from __future__ import annotations

import logging
from typing import Any

logger = logging.getLogger("garmin_bridge.garmin")


class LoginError(Exception):
    """Login failed (bad credentials, wrong/expired MFA code, lockout)."""

    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


# Result tags returned by begin_login / used by the app's state machine.
NEEDS_MFA = "needs_mfa"
OK = "ok"


def _new_api(email: str = "", password: str = ""):
    """Construct a garminconnect.Garmin (imported lazily so tests can stub)."""
    from garminconnect import Garmin

    return Garmin(email=email, password=password, return_on_mfa=True)


def _garth(api):
    """Return the underlying garth client across garminconnect versions.

    Newer garminconnect exposes it as ``.client``; older releases used
    ``.garth``. We probe both so a library bump doesn't silently break token
    serialization.
    """
    client = getattr(api, "client", None) or getattr(api, "garth", None)
    if client is None:  # pragma: no cover - defensive
        raise LoginError("login_failed", "garminconnect exposes no garth client")
    return client


def begin_login(email: str, password: str) -> tuple[str, Any]:
    """Start Garmin SSO.

    Returns ``(NEEDS_MFA, state)`` when Garmin demands a code — ``state`` is the
    opaque handle to pass to ``resume_login``. Returns ``(OK, token_b64)`` when
    login completed without MFA. Raises LoginError on bad credentials/lockout.
    """
    try:
        api = _new_api(email, password)
        result1, result2 = api.login()
    except Exception as exc:  # garminconnect raises a zoo of error types
        raise _classify(exc)

    if result1 == NEEDS_MFA:
        # result2 is garth's client_state; keep the api object with it so the
        # resume call reuses the same in-progress SSO context.
        return NEEDS_MFA, {"api": api, "client_state": result2}
    return OK, _dump_token(api)


def resume_login(state: Any, code: str) -> str:
    """Complete an MFA login with the supplied code; return the token blob."""
    api = state["api"]
    client_state = state["client_state"]
    try:
        api.resume_login(client_state, code)
    except Exception as exc:
        raise _classify(exc)
    return _dump_token(api)


def load_api(token_b64: str):
    """Rehydrate a Garmin client from a stored token blob (no MFA, no login)."""
    from garminconnect import Garmin

    api = Garmin()
    garth = _garth(api)
    garth.loads(token_b64)
    # Touch the connection so an expired OAuth2 access token is refreshed from
    # the stored OAuth1 token before the fetch loop runs (garth handles the
    # exchange; no human/MFA involved — design D4).
    refresh = getattr(garth, "refresh_oauth2", None)
    if callable(refresh):
        try:
            refresh()
        except Exception as exc:  # noqa: BLE001 — lazy refresh on first request is fine
            logger.debug("eager oauth2 refresh skipped: %s", exc)
    return api


def _dump_token(api) -> str:
    """Serialize the garth token to a base64 blob for backend persistence."""
    return _garth(api).dumps()


def fetch_day(api, date: str) -> dict[str, Any]:
    """Fetch a single day's raw Garmin data into the shape mapping.map_day wants.

    Each sub-fetch is individually guarded: a Garmin endpoint that errors or
    isn't available for this account yields a missing key rather than aborting
    the whole day (the mapper already tolerates absent sections).
    """

    def safe(label: str, fn):
        try:
            return fn()
        except Exception as exc:  # noqa: BLE001 — one bad endpoint must not abort
            logger.warning("garmin fetch %s failed for %s: %s", label, date, exc)
            return None

    raw: dict[str, Any] = {"date": date}
    raw["sleep"] = safe("sleep", lambda: api.get_sleep_data(date))
    raw["hrv"] = safe("hrv", lambda: api.get_hrv_data(date))
    raw["rhr"] = safe("rhr", lambda: api.get_rhr_day(date))
    raw["stress"] = safe("stress", lambda: api.get_stress_data(date))
    raw["training_readiness"] = safe("readiness", lambda: api.get_training_readiness(date))
    raw["body_battery"] = safe("body_battery", lambda: api.get_body_battery(date, date))
    raw["max_metrics"] = safe("max_metrics", lambda: api.get_max_metrics(date))
    raw["training_status"] = safe("training_status", lambda: api.get_training_status(date))
    raw["race_predictions"] = safe("race_predictions", lambda: api.get_race_predictions())
    raw["hydration"] = safe("hydration", lambda: api.get_hydration_data(date))
    raw["weigh_ins"] = safe("weigh_ins", lambda: api.get_weigh_ins(date, date))
    raw["activities"] = safe(
        "activities", lambda: api.get_activities_by_date(date, date)
    )
    return raw


def _classify(exc: Exception) -> LoginError:
    """Map a garminconnect exception to a typed, log-safe LoginError.

    The original exception text may embed the email; we keep messages generic
    and never include credentials.
    """
    name = type(exc).__name__
    text = str(exc).lower()
    if "mfa" in text or "code" in text:
        return LoginError("mfa_invalid", "the MFA code was wrong or expired")
    if "too many" in text or "lock" in text or "429" in text:
        return LoginError("locked_out", "Garmin temporarily locked out the account")
    if "auth" in name.lower() or "401" in text or "credential" in text:
        return LoginError("bad_credentials", "Garmin rejected the credentials")
    return LoginError("login_failed", f"Garmin login failed ({name})")
