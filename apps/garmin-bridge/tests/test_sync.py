"""Sync data-plane: fixture → expected REST calls, idempotency, missing token."""

from __future__ import annotations

import types

from fastapi.testclient import TestClient

from garmin_bridge import sync
from garmin_bridge.app import create_app
from tests.conftest import FakeBackend, FakeResponse


def _gc_stub(raw_day):
    """Stub gc whose load_api/fetch_day return the recorded fixture."""
    stub = types.SimpleNamespace()
    stub.load_api = lambda token_b64: object()
    stub.fetch_day = lambda api, date: raw_day
    return stub


def test_sync_day_issues_expected_rest_calls(raw_day):
    backend = FakeBackend()
    summary = sync.sync_day(backend, raw_day, "2026-06-12")

    paths = [p for p, _ in backend.posts]
    assert "/recovery-metrics" in paths
    assert "/fitness-metrics" in paths
    assert "/hydration-balance" in paths
    assert "/weight" in paths
    assert "/workouts/bulk" in paths

    # The bulk body carries both activities with garmin external_ids.
    bulk_body = next(b for p, b in backend.posts if p == "/workouts/bulk")
    ext_ids = {w["external_id"] for w in bulk_body["workouts"]}
    assert ext_ids == {"garmin:1234567", "garmin:1234568", "garmin:1234569"}
    assert all(w["source"] == "garmin" for w in bulk_body["workouts"])
    # The run carries its nested splits + zone detail through the bulk post
    # unchanged (no per-activity round-trips at the sync layer).
    run = next(w for w in bulk_body["workouts"] if w["external_id"] == "garmin:1234567")
    assert len(run["splits"]) == 2
    assert run["secs_in_zone_3"] == 1500

    assert summary["ok"] is True


def test_sync_is_idempotent_on_rerun(raw_day):
    """Re-running the same day produces the same write set — the backend's
    date-upsert + external_id dedup make it safe; the bridge sends identically."""
    first = FakeBackend()
    sync.sync_day(first, raw_day, "2026-06-12")
    second = FakeBackend()
    sync.sync_day(second, raw_day, "2026-06-12")
    assert first.posts == second.posts


def test_partial_failure_does_not_abort_sync(raw_day):
    backend = FakeBackend()
    # recovery endpoint errors; everything else must still be attempted.
    backend.responses["/recovery-metrics"] = FakeResponse(500)
    summary = sync.sync_day(backend, raw_day, "2026-06-12")

    paths = [p for p, _ in backend.posts]
    assert "/fitness-metrics" in paths  # attempted despite recovery failing
    assert "/workouts/bulk" in paths
    assert summary["ok"] is False
    assert "recovery" in summary["errors"]


def test_empty_day_skips_everything():
    backend = FakeBackend()
    summary = sync.sync_day(backend, {"date": "2026-06-12"}, "2026-06-12")
    assert backend.posts == []
    assert summary["ok"] is True
    assert summary["results"]["recovery"].startswith("skipped")


def test_sync_endpoint_missing_token_returns_login_required(config, raw_day):
    backend = FakeBackend(token=None)  # GET /garmin/token → 404
    app = create_app(config, gc=_gc_stub(raw_day), backend_factory=lambda: backend)
    client = TestClient(app)

    resp = client.post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 409
    assert resp.json()["error"] == "login_required"
    # Nothing was written.
    assert backend.posts == []


def test_sync_endpoint_happy_path(config, raw_day):
    backend = FakeBackend(token=b"stored-blob")
    app = create_app(config, gc=_gc_stub(raw_day), backend_factory=lambda: backend)
    client = TestClient(app)

    resp = client.post("/sync", json={"date": "2026-06-12"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["ok"] is True
    assert body["date"] == "2026-06-12"


def test_sync_defaults_to_today_when_no_date(config, raw_day):
    backend = FakeBackend(token=b"stored-blob")
    app = create_app(
        config,
        gc=_gc_stub(raw_day),
        backend_factory=lambda: backend,
        now=lambda tz: "2026-06-11",
    )
    client = TestClient(app)
    resp = client.post("/sync")
    assert resp.status_code == 200
    assert resp.json()["date"] == "2026-06-11"
