"""Environment-driven configuration with fail-fast validation.

Every required variable is read once at startup; a missing one raises so the
container crashes loudly rather than limping along and failing mid-sync. The
Garmin password and the backend bearer token are secrets — they live here only
and are never echoed back (see logging.py for redaction).
"""

from __future__ import annotations

import os
from dataclasses import dataclass


class ConfigError(RuntimeError):
    """Raised when a required environment variable is unset or invalid."""


@dataclass(frozen=True)
class Config:
    """Resolved runtime configuration.

    garmin_email / garmin_password
        Credentials for the interactive SSO login. Read from a Secret, never
        from a request body.
    nutrition_api_url
        Base URL of the nutrition REST API (e.g. http://nutrition-api).
    garmin_api_token
        Bearer token the bridge authenticates to the backend with — the
        ``garmin`` identity (per add-garmin-auth-token).
    sync_tz
        IANA timezone the daily sync resolves "today" in (the user's local
        day, not UTC, so a late-evening workout lands on the right date).
    """

    garmin_email: str
    garmin_password: str
    nutrition_api_url: str
    garmin_api_token: str
    sync_tz: str

    @property
    def sensitive_values(self) -> tuple[str, ...]:
        """Substrings that must never appear in logs or responses."""
        return tuple(v for v in (self.garmin_password, self.garmin_api_token) if v)


_REQUIRED = (
    "GARMIN_EMAIL",
    "GARMIN_PASSWORD",
    "NUTRITION_API_URL",
    "GARMIN_API_TOKEN",
)


def load(env: dict[str, str] | None = None) -> Config:
    """Build a Config from the environment, raising ConfigError if incomplete.

    Accepts an explicit mapping for tests; defaults to os.environ.
    """
    src = os.environ if env is None else env

    missing = [key for key in _REQUIRED if not src.get(key, "").strip()]
    if missing:
        raise ConfigError(
            "missing required environment variables: " + ", ".join(sorted(missing))
        )

    url = src["NUTRITION_API_URL"].strip().rstrip("/")
    if not url.startswith(("http://", "https://")):
        raise ConfigError("NUTRITION_API_URL must start with http:// or https://")

    return Config(
        garmin_email=src["GARMIN_EMAIL"].strip(),
        garmin_password=src["GARMIN_PASSWORD"],
        nutrition_api_url=url,
        garmin_api_token=src["GARMIN_API_TOKEN"].strip(),
        sync_tz=src.get("SYNC_TZ", "UTC").strip() or "UTC",
    )
