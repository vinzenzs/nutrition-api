"""Sync orchestration: post a mapped Garmin day to the backend, tolerant of
per-capability failure.

One failing capability (say Garmin had no HRV today, or the recovery endpoint
hiccups) must not abort the rest of the sync — each write is attempted and its
outcome recorded, and the call returns a summary of what landed and what didn't.
The mapping itself is in mapping.py; this module is purely about issuing the
writes and accounting for them.
"""

from __future__ import annotations

import logging
import time
from datetime import date, timedelta
from typing import Any, Callable

from . import mapping
from .backend import Backend

logger = logging.getLogger("garmin_bridge.sync")

# Date-keyed singleton snapshots (upsert by date on the backend).
_SNAPSHOT_ROUTES = {
    "recovery": "/recovery-metrics",
    "fitness": "/fitness-metrics",
    "hydration_balance": "/hydration-balance",
    "daily_summary": "/daily-summary",
    "health_vitals": "/health-vitals",
}


def sync_day(backend: Backend, raw: dict[str, Any], date: str) -> dict[str, Any]:
    """Map ``raw`` and POST each capability; return a per-capability summary."""
    mapped = mapping.map_day(raw, date)
    summary: dict[str, Any] = {"date": date, "results": {}, "errors": {}}

    # Date-keyed snapshots: one POST each, skipped when the day had no metrics.
    for key, route in _SNAPSHOT_ROUTES.items():
        body = mapped.get(key)
        if body is None:
            summary["results"][key] = "skipped (no data)"
            continue
        _attempt(backend, route, body, key, summary)

    # Weigh-ins: each is an independent create.
    weights = mapped.get("weights") or []
    if not weights:
        summary["results"]["weight"] = "skipped (no data)"
    else:
        ok = 0
        for body in weights:
            try:
                resp = backend.post_json("/weight", body)
                if resp.status_code in (200, 201):
                    ok += 1
                else:
                    summary["errors"].setdefault("weight", []).append(
                        f"{resp.status_code}"
                    )
            except Exception as exc:  # noqa: BLE001
                logger.warning("weight post failed: %s", exc)
                summary["errors"].setdefault("weight", []).append(str(exc))
        summary["results"]["weight"] = f"{ok}/{len(weights)} posted"

    # Activities: one bulk upsert (external_id dedups re-syncs).
    workouts = mapped.get("workouts") or []
    if not workouts:
        summary["results"]["workouts"] = "skipped (no data)"
    else:
        try:
            resp = backend.post_json("/workouts/bulk", {"workouts": workouts})
            if resp.status_code == 200:
                results = resp.json().get("results", [])
                errors = [r for r in results if "error" in r]
                summary["results"]["workouts"] = (
                    f"{len(results) - len(errors)}/{len(results)} upserted"
                )
                if errors:
                    summary["errors"]["workouts"] = errors
            else:
                summary["errors"]["workouts"] = f"bulk -> {resp.status_code}"
        except Exception as exc:  # noqa: BLE001
            logger.warning("workouts bulk post failed: %s", exc)
            summary["errors"]["workouts"] = str(exc)

    # Athlete physiology config: a non-date-keyed singleton, refreshed in place
    # via PUT each sync (Garmin source-of-truth). Skipped when the mapper found
    # nothing. No Idempotency-Key — the backend rejects it on PUT.
    config = mapped.get("athlete_config")
    if not config:
        summary["results"]["athlete_config"] = "skipped (no data)"
    else:
        try:
            resp = backend.put_json("/athlete-config", config)
            if resp.status_code in (200, 201):
                summary["results"]["athlete_config"] = "updated"
            else:
                summary["results"]["athlete_config"] = "failed"
                summary["errors"]["athlete_config"] = f"PUT -> {resp.status_code}"
        except Exception as exc:  # noqa: BLE001
            logger.warning("athlete_config put failed: %s", exc)
            summary["results"]["athlete_config"] = "failed"
            summary["errors"]["athlete_config"] = str(exc)

    # Inventory: gear + personal records, each an idempotent upsert by external
    # id. Mirrors the weight loop's per-item partial-failure accounting; one bad
    # inventory item (or an empty inventory) never aborts the rest of the sync.
    _upsert_each(backend, "/gear", mapped.get("gear") or [], "gear", summary)
    _upsert_each(
        backend,
        "/personal-records",
        mapped.get("personal_records") or [],
        "personal_records",
        summary,
    )
    _upsert_each(backend, "/devices", mapped.get("devices") or [], "devices", summary)
    _upsert_each(
        backend, "/achievements", mapped.get("achievements") or [], "achievements", summary
    )

    summary["ok"] = not summary["errors"]
    return summary


def _upsert_each(
    backend: Backend, route: str, items: list, key: str, summary: dict[str, Any]
) -> None:
    """POST each inventory item to its upsert endpoint, accounting per item."""
    if not items:
        summary["results"][key] = "skipped (no data)"
        return
    ok = 0
    for body in items:
        try:
            resp = backend.post_json(route, body)
            if resp.status_code in (200, 201):
                ok += 1
            else:
                summary["errors"].setdefault(key, []).append(f"{resp.status_code}")
        except Exception as exc:  # noqa: BLE001
            logger.warning("%s post failed: %s", key, exc)
            summary["errors"].setdefault(key, []).append(str(exc))
    summary["results"][key] = f"{ok}/{len(items)} upserted"


def run_backfill(
    backend: Backend,
    gc: Any,
    api: Any,
    start: date,
    end: date,
    *,
    day_delay_seconds: int = 0,
    sleeper: Callable[[float], None] = time.sleep,
) -> dict[str, Any]:
    """Replay the per-day sync over [start, end] inclusive, oldest-first.

    Each day runs the same ``gc.fetch_day`` + ``sync_day`` path as ``POST /sync``,
    so any enrichment elsewhere is picked up with no backfill-specific work. A
    failing day is recorded ``{date, ok:false, error}`` and the range continues.
    ``sleeper`` is injectable so tests need not actually wait.
    """
    days: list[dict[str, Any]] = []
    ok = 0
    first = True
    cur = start
    while cur <= end:
        if not first and day_delay_seconds > 0:
            sleeper(day_delay_seconds)
        first = False
        dstr = cur.isoformat()
        try:
            raw = gc.fetch_day(api, dstr)
            summary = sync_day(backend, raw, dstr)
            if summary.get("ok"):
                ok += 1
            days.append(summary)
        except Exception as exc:  # noqa: BLE001 — one bad day must not abort the range
            logger.warning("backfill day %s failed: %s", dstr, exc)
            days.append({"date": dstr, "ok": False, "error": str(exc)})
        cur += timedelta(days=1)
    return {
        "days": days,
        "days_total": len(days),
        "days_ok": ok,
        "days_failed": len(days) - ok,
    }


def _attempt(
    backend: Backend, route: str, body: dict, key: str, summary: dict[str, Any]
) -> None:
    try:
        resp = backend.post_json(route, body)
        if resp.status_code in (200, 201):
            summary["results"][key] = "posted"
        else:
            summary["results"][key] = "failed"
            summary["errors"][key] = f"{route} -> {resp.status_code}"
    except Exception as exc:  # noqa: BLE001
        logger.warning("%s post failed: %s", key, exc)
        summary["results"][key] = "failed"
        summary["errors"][key] = str(exc)
