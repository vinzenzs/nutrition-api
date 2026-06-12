"""RedactingFilter unit tests."""

from __future__ import annotations

import logging

from garmin_bridge.logging_setup import REDACTED, RedactingFilter


def _record(msg, args=None):
    return logging.LogRecord("t", logging.INFO, __file__, 1, msg, args or (), None)


def test_scrubs_registered_secret_in_message():
    f = RedactingFilter(["hunter2"])
    rec = _record("logging in with hunter2 now")
    f.filter(rec)
    assert "hunter2" not in rec.getMessage()
    assert REDACTED in rec.getMessage()


def test_scrubs_secret_in_args():
    f = RedactingFilter(["tok-secret"])
    rec = _record("token is %s", ("tok-secret",))
    f.filter(rec)
    assert "tok-secret" not in rec.getMessage()


def test_scrubs_token_shaped_blob_even_if_unregistered():
    f = RedactingFilter([])
    rec = _record('{"oauth1_token": "abc", "refresh_token": "xyz"}')
    f.filter(rec)
    assert rec.getMessage() == REDACTED


def test_add_secret_at_runtime():
    f = RedactingFilter([])
    f.add_secret("late-token")
    rec = _record("late-token leaked")
    f.filter(rec)
    assert "late-token" not in rec.getMessage()
