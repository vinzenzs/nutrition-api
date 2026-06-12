"""Structured (JSON-line) logging with secret redaction.

The Garmin password, the backend bearer token, and the serialized garth token
blob must never reach the logs. A logging filter scrubs known secret strings
from every record's message and args before it is formatted, so even an
accidental ``logger.info("logging in as %s", password)`` is neutralized.
"""

from __future__ import annotations

import json
import logging
import sys
from typing import Iterable

REDACTED = "***REDACTED***"

# Substrings that always look like a token blob and should be scrubbed even if
# they were never registered as a secret (defense in depth for the garth blob,
# which is minted at runtime and not known at startup).
_BLOB_MARKERS = ("oauth1_token", "oauth2_token", "refresh_token", "access_token")


class RedactingFilter(logging.Filter):
    """Scrubs registered secret substrings and token-shaped blobs from records."""

    def __init__(self, secrets: Iterable[str] = ()):
        super().__init__()
        # Longest-first so a token that contains a shorter secret is fully cut.
        self._secrets = sorted({s for s in secrets if s}, key=len, reverse=True)

    def add_secret(self, value: str) -> None:
        if value and value not in self._secrets:
            self._secrets.append(value)
            self._secrets.sort(key=len, reverse=True)

    def _scrub(self, text: str) -> str:
        for secret in self._secrets:
            if secret in text:
                text = text.replace(secret, REDACTED)
        for marker in _BLOB_MARKERS:
            if marker in text:
                return REDACTED
        return text

    def filter(self, record: logging.LogRecord) -> bool:
        if isinstance(record.msg, str):
            record.msg = self._scrub(record.msg)
        if record.args:
            if isinstance(record.args, dict):
                record.args = {
                    k: self._scrub(v) if isinstance(v, str) else v
                    for k, v in record.args.items()
                }
            else:
                record.args = tuple(
                    self._scrub(a) if isinstance(a, str) else a for a in record.args
                )
        return True


class JSONFormatter(logging.Formatter):
    """Emits one JSON object per line: timestamp, level, logger, message."""

    def format(self, record: logging.LogRecord) -> str:
        payload = {
            "ts": self.formatTime(record, "%Y-%m-%dT%H:%M:%S%z"),
            "level": record.levelname,
            "logger": record.name,
            "msg": record.getMessage(),
        }
        if record.exc_info:
            payload["exc"] = self.formatException(record.exc_info)
        return json.dumps(payload)


_redactor = RedactingFilter()


def configure(level: str = "INFO", secrets: Iterable[str] = ()) -> None:
    """Install JSON logging + redaction on the root logger (idempotent)."""
    for secret in secrets:
        _redactor.add_secret(secret)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JSONFormatter())
    handler.addFilter(_redactor)

    root = logging.getLogger()
    root.handlers = [handler]
    root.setLevel(level)


def register_secret(value: str) -> None:
    """Register a secret discovered at runtime (e.g. a freshly minted token)."""
    _redactor.add_secret(value)
