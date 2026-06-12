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
from typing import Any

from . import mapping
from .backend import Backend

logger = logging.getLogger("garmin_bridge.sync")

# Date-keyed singleton snapshots (upsert by date on the backend).
_SNAPSHOT_ROUTES = {
    "recovery": "/recovery-metrics",
    "fitness": "/fitness-metrics",
    "hydration_balance": "/hydration-balance",
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

    summary["ok"] = not summary["errors"]
    return summary


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
