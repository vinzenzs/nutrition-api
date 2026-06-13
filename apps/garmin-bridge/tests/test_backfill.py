"""History backfill: the pure run_backfill range loop + config pacing knobs.

These exercise the day-loop directly (no TestClient), so they run in any env:
the FastAPI route is a thin wrapper that parses the range, enforces the cap, and
delegates to sync.run_backfill.
"""

from __future__ import annotations

import types
from datetime import date

from garmin_bridge import config as config_mod
from garmin_bridge import sync


class FakeBackend:
    def close(self) -> None:  # pragma: no cover - nothing to close
        pass


def _gc(fail_dates=()):
    """A gc stub whose fetch_day raises for the given dates."""
    stub = types.SimpleNamespace()

    def fetch_day(api, date_str):
        if date_str in fail_dates:
            raise RuntimeError(f"garmin boom on {date_str}")
        return {"date": date_str}

    stub.fetch_day = fetch_day
    return stub


def test_run_backfill_replays_each_day(monkeypatch):
    # sync_day is exercised elsewhere; stub it to a deterministic ok-summary.
    monkeypatch.setattr(sync, "sync_day", lambda backend, raw, d: {"date": d, "ok": True, "results": {}})
    slept: list[float] = []
    result = sync.run_backfill(
        FakeBackend(), _gc(), object(), date(2026, 3, 1), date(2026, 3, 5),
        day_delay_seconds=3, sleeper=slept.append,
    )
    assert result["days_total"] == 5
    assert result["days_ok"] == 5
    assert result["days_failed"] == 0
    assert [d["date"] for d in result["days"]] == [
        "2026-03-01", "2026-03-02", "2026-03-03", "2026-03-04", "2026-03-05",
    ]
    # paced: one sleep between each of the 5 days (4 gaps)
    assert slept == [3, 3, 3, 3]


def test_run_backfill_one_bad_day_continues(monkeypatch):
    monkeypatch.setattr(sync, "sync_day", lambda backend, raw, d: {"date": d, "ok": True})
    result = sync.run_backfill(
        FakeBackend(), _gc(fail_dates={"2026-03-02"}), object(),
        date(2026, 3, 1), date(2026, 3, 3), day_delay_seconds=0, sleeper=lambda s: None,
    )
    assert result["days_total"] == 3
    assert result["days_ok"] == 2
    assert result["days_failed"] == 1
    bad = next(d for d in result["days"] if d["date"] == "2026-03-02")
    assert bad["ok"] is False
    assert "boom" in bad["error"]


def test_run_backfill_delay_zero_no_sleep(monkeypatch):
    monkeypatch.setattr(sync, "sync_day", lambda backend, raw, d: {"date": d, "ok": True})
    slept: list[float] = []
    sync.run_backfill(
        FakeBackend(), _gc(), object(), date(2026, 3, 1), date(2026, 3, 2),
        day_delay_seconds=0, sleeper=slept.append,
    )
    assert slept == [], "delay=0 disables the pause"


def test_run_backfill_single_day(monkeypatch):
    monkeypatch.setattr(sync, "sync_day", lambda backend, raw, d: {"date": d, "ok": True})
    result = sync.run_backfill(
        FakeBackend(), _gc(), object(), date(2026, 3, 1), date(2026, 3, 1),
        day_delay_seconds=5, sleeper=lambda s: (_ for _ in ()).throw(AssertionError("slept")),
    )
    assert result["days_total"] == 1  # single day → no inter-day sleep


def test_config_backfill_defaults_and_overrides():
    base = {
        "GARMIN_EMAIL": "a@b.com",
        "GARMIN_PASSWORD": "pw",
        "NUTRITION_API_URL": "http://x",
        "GARMIN_API_TOKEN": "t",
    }
    cfg = config_mod.load(base)
    assert cfg.backfill_day_delay_seconds == 3
    assert cfg.backfill_max_days == 120

    cfg2 = config_mod.load({**base, "BACKFILL_DAY_DELAY_SECONDS": "0", "BACKFILL_MAX_DAYS": "30"})
    assert cfg2.backfill_day_delay_seconds == 0
    assert cfg2.backfill_max_days == 30

    # invalid → fall back to default
    cfg3 = config_mod.load({**base, "BACKFILL_MAX_DAYS": "notanint"})
    assert cfg3.backfill_max_days == 120
