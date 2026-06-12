"""Garmin payload → nutrition REST request bodies.

This is the single coupling point between Garmin's churning shapes and the
backend's stable request contracts, so it is pure (no I/O) and exhaustively
tested against a recorded fixture. Every extractor is defensive: a missing or
malformed Garmin field yields ``None`` and the corresponding REST field is
simply omitted (the Snapshot/Workout structs treat absent as distinct from a
real zero).

Mapping (see spec / design D5):
  sleep / HRV / RHR / stress / readiness / body-battery → /recovery-metrics
  VO2max / training-load / race predictions             → /fitness-metrics
  sweat loss                                            → /hydration-balance
  weigh-ins                                             → /weight
  activities                                            → /workouts/bulk
                                                          (source=garmin,
                                                           external_id=garmin:<id>)
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Any

# Garmin activityType.typeKey → our small Sport enum. Anything unmapped is
# "other" so an unknown sport never drops the activity.
_SPORT_BY_TYPEKEY = {
    "running": "run",
    "trail_running": "run",
    "treadmill_running": "run",
    "track_running": "run",
    "indoor_running": "run",
    "cycling": "bike",
    "road_biking": "bike",
    "mountain_biking": "bike",
    "indoor_cycling": "bike",
    "virtual_ride": "bike",
    "gravel_cycling": "bike",
    "lap_swimming": "swim",
    "open_water_swimming": "swim",
    "swimming": "swim",
    "strength_training": "strength",
    "indoor_cardio": "strength",
}


# --- safe extraction helpers --------------------------------------------


def _dig(obj: Any, *path: Any) -> Any:
    """Walk nested dicts/lists by key/index; return None on any miss."""
    cur = obj
    for key in path:
        if isinstance(key, int):
            if isinstance(cur, (list, tuple)) and -len(cur) <= key < len(cur):
                cur = cur[key]
            else:
                return None
        elif isinstance(cur, dict):
            cur = cur.get(key)
        else:
            return None
        if cur is None:
            return None
    return cur


def _as_int(value: Any) -> int | None:
    if isinstance(value, bool) or value is None:
        return None
    try:
        return int(round(float(value)))
    except (TypeError, ValueError):
        return None


def _as_float(value: Any) -> float | None:
    if isinstance(value, bool) or value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _prune(body: dict[str, Any]) -> dict[str, Any]:
    """Drop None-valued keys so omitempty fields stay absent."""
    return {k: v for k, v in body.items() if v is not None}


def _has_metrics(snapshot: dict[str, Any]) -> bool:
    """True if the snapshot carries at least one metric beyond `date`."""
    return any(k != "date" for k in snapshot)


# --- per-capability mappers ---------------------------------------------


def map_recovery(raw: dict[str, Any], date: str) -> dict[str, Any] | None:
    """sleep / HRV / RHR / stress / readiness / body-battery → recovery Snapshot."""
    bb = _dig(raw, "body_battery", 0) or {}
    snap = _prune(
        {
            "date": date,
            "sleep_seconds": _as_int(_dig(raw, "sleep", "dailySleepDTO", "sleepTimeSeconds")),
            "sleep_score": _as_int(
                _dig(raw, "sleep", "dailySleepDTO", "sleepScores", "overall", "value")
            ),
            "hrv_ms": _as_float(_dig(raw, "hrv", "hrvSummary", "lastNightAvg")),
            "resting_hr": _as_int(_dig(raw, "rhr", "restingHeartRate")),
            "stress_avg": _as_int(_dig(raw, "stress", "avgStressLevel")),
            "body_battery_charged": _as_int(bb.get("charged")),
            "body_battery_drained": _as_int(bb.get("drained")),
            "training_readiness": _as_int(_dig(raw, "training_readiness", 0, "score")),
        }
    )
    return snap if _has_metrics(snap) else None


def map_fitness(raw: dict[str, Any], date: str) -> dict[str, Any] | None:
    """VO2max / training-load / race predictions → fitness Snapshot."""
    snap = _prune(
        {
            "date": date,
            "vo2max_running": _as_float(
                _dig(raw, "max_metrics", 0, "generic", "vo2MaxPreciseValue")
                or _dig(raw, "max_metrics", 0, "generic", "vo2MaxValue")
            ),
            "vo2max_cycling": _as_float(
                _dig(raw, "max_metrics", 0, "cycling", "vo2MaxPreciseValue")
                or _dig(raw, "max_metrics", 0, "cycling", "vo2MaxValue")
            ),
            "race_predictor_5k_seconds": _as_int(_dig(raw, "race_predictions", "time5K")),
            "race_predictor_10k_seconds": _as_int(_dig(raw, "race_predictions", "time10K")),
            "race_predictor_half_seconds": _as_int(
                _dig(raw, "race_predictions", "timeHalfMarathon")
            ),
            "race_predictor_full_seconds": _as_int(
                _dig(raw, "race_predictions", "timeMarathon")
            ),
            "acute_load": _as_float(
                _dig(raw, "training_status", "acuteTrainingLoad")
                or _dig(
                    raw,
                    "training_status",
                    "mostRecentTrainingLoadBalance",
                    "metricsTrainingLoadBalanceDTOMap",
                    "acwrPercent",
                )
            ),
            "chronic_load": _as_float(_dig(raw, "training_status", "chronicTrainingLoad")),
        }
    )
    return snap if _has_metrics(snap) else None


def map_hydration_balance(raw: dict[str, Any], date: str) -> dict[str, Any] | None:
    """Garmin daily hydration estimate → hydration-balance Snapshot (all ml)."""
    snap = _prune(
        {
            "date": date,
            "sweat_loss_ml": _as_float(_dig(raw, "hydration", "sweatLossInML")),
            "activity_intake_ml": _as_float(_dig(raw, "hydration", "activityIntakeInML")),
            "goal_ml": _as_float(_dig(raw, "hydration", "goalInML")),
        }
    )
    return snap if _has_metrics(snap) else None


def map_weights(raw: dict[str, Any]) -> list[dict[str, Any]]:
    """Garmin weigh-ins → /weight create bodies. Weight/mass are grams → kg."""
    out: list[dict[str, Any]] = []
    summaries = _dig(raw, "weigh_ins", "dailyWeightSummaries") or []
    for summary in summaries:
        metrics = _dig(summary, "allWeightMetrics") or []
        for m in metrics:
            weight_g = _as_float(m.get("weight"))
            logged_ms = _as_int(m.get("timestampGMT") or m.get("date"))
            body = _prune(
                {
                    "weight_kg": (weight_g / 1000.0) if weight_g is not None else None,
                    "logged_at": _epoch_ms_to_rfc3339(logged_ms),
                    "body_fat_pct": _as_float(m.get("bodyFat")),
                    "muscle_mass_kg": _grams_to_kg(_as_float(m.get("muscleMass"))),
                    "body_water_pct": _as_float(m.get("bodyWater")),
                    "bone_mass_kg": _grams_to_kg(_as_float(m.get("boneMass"))),
                    "bmi": _as_float(m.get("bmi")),
                }
            )
            # weight_kg + logged_at are the minimum the endpoint needs.
            if "weight_kg" in body and "logged_at" in body:
                out.append(body)
    return out


def map_workouts(raw: dict[str, Any]) -> list[dict[str, Any]]:
    """Garmin activities → /workouts/bulk items (source=garmin, external_id)."""
    out: list[dict[str, Any]] = []
    for act in raw.get("activities") or []:
        activity_id = act.get("activityId")
        started = _garmin_ts_to_rfc3339(act.get("startTimeGMT"))
        duration_s = _as_float(act.get("duration"))
        if activity_id is None or started is None or duration_s is None:
            continue
        ended = _shift_rfc3339(started, duration_s)
        type_key = _dig(act, "activityType", "typeKey") or ""
        item = _prune(
            {
                "external_id": f"garmin:{activity_id}",
                "source": "garmin",
                "sport": _SPORT_BY_TYPEKEY.get(type_key, "other"),
                "status": "completed",
                "name": act.get("activityName"),
                "started_at": started,
                "ended_at": ended,
                "kcal_burned": _as_float(act.get("calories")),
                "avg_hr": _as_int(act.get("averageHR")),
                "tss": _as_float(act.get("trainingStressScore")),
                "distance_m": _as_float(act.get("distance")),
                "avg_power_w": _as_int(act.get("avgPower")),
                "temperature_c": _as_float(act.get("minTemperature")),
                "session_group": _parent_id(act),
            }
        )
        out.append(item)
    return out


def map_day(raw: dict[str, Any], date: str) -> dict[str, Any]:
    """Map a full raw Garmin day into the per-capability request bodies."""
    return {
        "recovery": map_recovery(raw, date),
        "fitness": map_fitness(raw, date),
        "hydration_balance": map_hydration_balance(raw, date),
        "weights": map_weights(raw),
        "workouts": map_workouts(raw),
    }


# --- time helpers -------------------------------------------------------


def _grams_to_kg(grams: float | None) -> float | None:
    return None if grams is None else grams / 1000.0


def _parent_id(act: dict[str, Any]) -> str | None:
    """The Garmin parent multisport id, linking brick legs (session_group)."""
    parent = act.get("parentId") or _dig(act, "parent", "activityId")
    return f"garmin:{parent}" if parent is not None else None


def _garmin_ts_to_rfc3339(value: Any) -> str | None:
    """Garmin 'YYYY-MM-DD HH:MM:SS' (GMT) → RFC3339 'YYYY-MM-DDTHH:MM:SSZ'."""
    if not isinstance(value, str) or not value.strip():
        return None
    try:
        dt = datetime.strptime(value.strip(), "%Y-%m-%d %H:%M:%S")
    except ValueError:
        return None
    return dt.replace(tzinfo=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _shift_rfc3339(rfc3339: str, seconds: float) -> str:
    dt = datetime.strptime(rfc3339, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    return (dt + timedelta(seconds=seconds)).strftime("%Y-%m-%dT%H:%M:%SZ")


def _epoch_ms_to_rfc3339(ms: int | None) -> str | None:
    if ms is None:
        return None
    dt = datetime.fromtimestamp(ms / 1000.0, tz=timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")
