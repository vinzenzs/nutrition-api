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


def _as_str(value: Any) -> str | None:
    """Trimmed non-empty string, else None (bools/numbers are not labels)."""
    if not isinstance(value, str):
        return None
    trimmed = value.strip()
    return trimmed or None


def _prune(body: dict[str, Any]) -> dict[str, Any]:
    """Drop None-valued keys so omitempty fields stay absent."""
    return {k: v for k, v in body.items() if v is not None}


def _has_metrics(snapshot: dict[str, Any]) -> bool:
    """True if the snapshot carries at least one metric beyond `date`."""
    return any(k != "date" for k in snapshot)


# --- per-capability mappers ---------------------------------------------


def map_recovery(raw: dict[str, Any], date: str) -> dict[str, Any] | None:
    """sleep / HRV / RHR / stress / readiness / body-battery + SpO2 / respiration
    / sleep-stage breakdown → recovery Snapshot.

    The four sleep-stage seconds come free from the sleep DTO already dug for
    ``sleep_seconds``; SpO2 and respiration come from their own per-day fetches.
    """
    bb = _dig(raw, "body_battery", 0) or {}
    sleep_dto = _dig(raw, "sleep", "dailySleepDTO") or {}
    snap = _prune(
        {
            "date": date,
            "sleep_seconds": _as_int(sleep_dto.get("sleepTimeSeconds")),
            "sleep_score": _as_int(
                _dig(sleep_dto, "sleepScores", "overall", "value")
            ),
            "hrv_ms": _as_float(_dig(raw, "hrv", "hrvSummary", "lastNightAvg")),
            "resting_hr": _as_int(_dig(raw, "rhr", "restingHeartRate")),
            "stress_avg": _as_int(_dig(raw, "stress", "avgStressLevel")),
            "body_battery_charged": _as_int(bb.get("charged")),
            "body_battery_drained": _as_int(bb.get("drained")),
            "training_readiness": _as_int(_dig(raw, "training_readiness", 0, "score")),
            "spo2_avg": _as_int(_dig(raw, "spo2", "averageSpO2")),
            "spo2_lowest": _as_int(_dig(raw, "spo2", "lowestSpO2")),
            "respiration_avg": _as_float(_dig(raw, "respiration", "avgSleepRespirationValue")),
            "respiration_lowest": _as_float(_dig(raw, "respiration", "lowestRespirationValue")),
            "deep_sleep_seconds": _as_int(sleep_dto.get("deepSleepSeconds")),
            "light_sleep_seconds": _as_int(sleep_dto.get("lightSleepSeconds")),
            "rem_sleep_seconds": _as_int(sleep_dto.get("remSleepSeconds")),
            "awake_seconds": _as_int(sleep_dto.get("awakeSleepSeconds")),
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
            "endurance_score": _as_int(
                _dig(raw, "endurance_score", "overallScore")
                or _dig(raw, "endurance_score", "enduranceScore")
            ),
            "hill_score": _as_int(_dig(raw, "hill_score", "overallScore")),
            "fitness_age": _as_float(
                _dig(raw, "fitness_age", "fitnessAge")
                or _dig(raw, "fitness_age", "achievableFitnessAge")
            ),
            "training_status": _training_status_label(raw),
        }
    )
    return snap if _has_metrics(snap) else None


def _training_status_label(raw: dict[str, Any]) -> str | None:
    """The human-readable training-status phrase from the already-fetched payload.

    Garmin nests the per-device phrase under ``latestTrainingStatusData[*]``;
    older/top-level shapes carry it directly. Stored verbatim (not enum-gated),
    so an unrecognised future word is preserved rather than dropped.
    """
    latest = _dig(raw, "training_status", "latestTrainingStatusData")
    if isinstance(latest, dict):
        for entry in latest.values():
            if isinstance(entry, dict):
                label = _as_str(entry.get("trainingStatus"))
                if label is not None:
                    return label
    return _as_str(_dig(raw, "training_status", "trainingStatus"))


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


def map_daily_summary(raw: dict[str, Any], date: str) -> dict[str, Any] | None:
    """Garmin get_user_summary → /daily-summary Snapshot (whole-day totals).

    Active/resting/total kcal, steps, floors, intensity minutes, distance — the
    NEAT-inclusive total-expenditure picture EA deliberately excludes. Every
    field is defensively extracted; an absent key is simply omitted (stored
    NULL). Returns None when the day carried no usable totals.
    """
    us = raw.get("user_summary") or {}
    snap = _prune(
        {
            "date": date,
            "active_kcal": _as_int(us.get("activeKilocalories")),
            "resting_kcal": _as_int(us.get("bmrKilocalories")),
            "total_kcal": _as_int(us.get("totalKilocalories")),
            "steps": _as_int(us.get("totalSteps")),
            "floors": _as_int(us.get("floorsAscended")),
            "moderate_intensity_minutes": _as_int(us.get("moderateIntensityMinutes")),
            "vigorous_intensity_minutes": _as_int(us.get("vigorousIntensityMinutes")),
            "distance_m": _as_float(us.get("totalDistanceMeters")),
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
    """Garmin activities → /workouts/bulk items (source=garmin, external_id).

    Each item carries the scalar performance + weather + HR-zone fields when the
    summary / per-activity detail provides them, plus nested ``splits``/``sets``
    arrays. Absent detail is simply omitted (an indoor activity with no weather
    drops the humidity/wind keys; an endurance activity has no ``sets``).
    """
    out: list[dict[str, Any]] = []
    details = raw.get("activity_details") or {}
    for act in raw.get("activities") or []:
        activity_id = act.get("activityId")
        started = _garmin_ts_to_rfc3339(act.get("startTimeGMT"))
        duration_s = _as_float(act.get("duration"))
        if activity_id is None or started is None or duration_s is None:
            continue
        ended = _shift_rfc3339(started, duration_s)
        type_key = _dig(act, "activityType", "typeKey") or ""
        detail = details.get(str(activity_id)) or {}
        weather = detail.get("weather") or {}
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
                # Scalar performance fields — ride along the activity summary,
                # no extra Garmin call.
                "elevation_gain_m": _as_float(act.get("elevationGain")),
                "elevation_loss_m": _as_float(act.get("elevationLoss")),
                "normalized_power_w": _as_int(act.get("normPower")),
                "intensity_factor": _as_float(act.get("intensityFactor")),
                "avg_cadence": _as_int(_avg_cadence(act)),
                # Garmin reports stride length in centimetres; the column is metres.
                "avg_stride_m": _cm_to_m(_as_float(act.get("avgStrideLength"))),
                "max_hr": _as_int(act.get("maxHR")),
                "aerobic_te": _as_float(act.get("aerobicTrainingEffect")),
                "anaerobic_te": _as_float(act.get("anaerobicTrainingEffect")),
                # Weather (per-activity fetch); humidity is a primary sweat-rate
                # driver. Indoor activities have none → keys stay absent.
                "humidity_pct": _as_float(weather.get("relativeHumidity")),
                "wind_speed_mps": _as_float(weather.get("windSpeed")),
            }
        )
        item.update(_map_zones(detail.get("zones")))
        splits = _map_splits(detail.get("splits"))
        if splits:
            item["splits"] = splits
        sets = _map_sets(detail.get("sets"))
        if sets:
            item["sets"] = sets
        out.append(item)
    return out


def _avg_cadence(act: dict[str, Any]) -> Any:
    """Average cadence across run/bike summary shapes (steps/min or rev/min)."""
    return (
        act.get("averageRunningCadenceInStepsPerMinute")
        or act.get("averageBikingCadenceInRevPerMinute")
        or act.get("averageCadence")
    )


def _map_zones(zones: Any) -> dict[str, Any]:
    """get_activity_hr_in_timezones list → secs_in_zone_1..5 (absent omitted)."""
    out: dict[str, Any] = {}
    for z in zones or []:
        num = _as_int(z.get("zoneNumber"))
        secs = _as_int(z.get("secsInZone"))
        if num is not None and 1 <= num <= 5 and secs is not None:
            out[f"secs_in_zone_{num}"] = secs
    return out


def _map_splits(splits: Any) -> list[dict[str, Any]]:
    """get_activity_splits → ordered split items (0-based split_index)."""
    laps = _dig(splits, "lapDTOs") if isinstance(splits, dict) else splits
    out: list[dict[str, Any]] = []
    for i, lap in enumerate(laps or []):
        out.append(
            _prune(
                {
                    "split_index": i,
                    "distance_m": _as_float(lap.get("distance")),
                    "duration_s": _as_float(lap.get("duration")),
                    "avg_hr": _as_int(lap.get("averageHR")),
                    "avg_power_w": _as_int(lap.get("averagePower")),
                    "avg_speed_mps": _as_float(lap.get("averageSpeed")),
                    "elevation_gain_m": _as_float(lap.get("elevationGain")),
                }
            )
        )
    return out


def _map_sets(sets: Any) -> list[dict[str, Any]]:
    """get_activity_exercise_sets → ordered set items (0-based set_index).

    Only counts active (working) sets — rest periods carry no exercise. Weight
    is grams on Garmin → kg.
    """
    raw_sets = _dig(sets, "exerciseSets") if isinstance(sets, dict) else sets
    out: list[dict[str, Any]] = []
    for s in raw_sets or []:
        if s.get("setType") not in (None, "ACTIVE"):
            continue
        out.append(
            _prune(
                {
                    "set_index": len(out),
                    "exercise_name": _dig(s, "exercises", 0, "name"),
                    "exercise_category": _dig(s, "exercises", 0, "category"),
                    "reps": _as_int(s.get("repetitionCount")),
                    "weight_kg": _grams_to_kg(_as_float(s.get("weight"))),
                    "duration_s": _as_float(s.get("duration")),
                }
            )
        )
    return out


def _cm_to_m(cm: float | None) -> float | None:
    return None if cm is None else cm / 100.0


# --- inventory mappers (gear + personal records) ------------------------

# Garmin gearTypeName phrase → our small enum. Substring match; anything
# unmapped falls back to "other" (mirrors the sport-enum fallback).
def _gear_type(name: Any) -> str:
    if not isinstance(name, str):
        return "other"
    low = name.strip().lower()
    if "shoe" in low:
        return "shoes"
    if "bike" in low or "cycl" in low or "bicycle" in low:
        return "bike"
    return "other"


def _date_only(value: Any) -> str | None:
    """Garmin date/datetime string → YYYY-MM-DD (or None)."""
    if not isinstance(value, str) or len(value) < 10:
        return None
    candidate = value[:10]
    if candidate[4] == "-" and candidate[7] == "-":
        return candidate
    return None


def map_gear(raw: dict[str, Any]) -> list[dict[str, Any]]:
    """Garmin gear (joined with gear-stats by uuid) → /gear upsert items.

    Mileage/activity totals come from ``gear_stats`` keyed by gear uuid; an item
    whose stats are absent still syncs with type + display name (distance/count
    omitted → stored NULL). Defensive: a gear without a uuid or display name is
    skipped, never crashes the sync.
    """
    out: list[dict[str, Any]] = []
    stats = raw.get("gear_stats") or {}
    for g in raw.get("gear") or []:
        guid = g.get("uuid") or g.get("gearPk") or g.get("gearUuid")
        if guid is None:
            continue
        st = stats.get(str(guid)) or {}
        display = (
            _as_str(g.get("displayName"))
            or _as_str(g.get("customMakeModel"))
            or _as_str(
                " ".join(
                    p for p in (g.get("gearMakeName"), g.get("gearModelName")) if p
                )
            )
        )
        if display is None:
            continue
        status = g.get("gearStatusName")
        item = _prune(
            {
                "external_id": str(guid),
                "gear_type": _gear_type(g.get("gearTypeName") or g.get("typeName")),
                "display_name": display,
                "total_distance_m": _as_float(st.get("totalDistance"))
                if st.get("totalDistance") is not None
                else _as_float(g.get("totalDistance")),
                "total_activities": _as_int(st.get("totalActivities"))
                if st.get("totalActivities") is not None
                else _as_int(g.get("totalActivities")),
                "retired": isinstance(status, str) and status.strip().lower() == "retired",
                "date_begin": _date_only(g.get("dateBegin")),
                "date_end": _date_only(g.get("dateEnd")),
            }
        )
        out.append(item)
    return out


# Garmin personal-record typeId → (pr_type, unit). value is seconds for time
# PRs, metres for distance PRs. Best-effort; an unmapped typeId becomes a
# generic label with a seconds unit (the common case) rather than dropping it.
_PR_TYPE_BY_TYPEID = {
    1: ("1k", "s"),
    2: ("1mi", "s"),
    3: ("5k", "s"),
    4: ("10k", "s"),
    7: ("longest-run", "m"),
    8: ("longest-ride", "m"),
    12: ("total-ascent", "m"),
}


def map_personal_records(raw: dict[str, Any]) -> list[dict[str, Any]]:
    """Garmin personal records → /personal-records upsert items."""
    out: list[dict[str, Any]] = []
    for pr in raw.get("personal_records") or []:
        pid = pr.get("id")
        value = _as_float(pr.get("value"))
        achieved = _pr_achieved_at(pr)
        if pid is None or value is None or achieved is None:
            continue
        type_id = _as_int(pr.get("typeId"))
        pr_type, unit = _PR_TYPE_BY_TYPEID.get(
            type_id,
            (f"garmin-type-{type_id}" if type_id is not None else "unknown", "s"),
        )
        activity_id = pr.get("activityId")
        item = _prune(
            {
                "external_id": str(pid),
                "pr_type": pr_type,
                "value": value,
                "unit": unit,
                "activity_id": str(activity_id) if activity_id is not None else None,
                "achieved_at": achieved,
            }
        )
        out.append(item)
    return out


def _pr_achieved_at(pr: dict[str, Any]) -> str | None:
    """PR achievement timestamp → RFC3339.

    Prefers Garmin's already-GMT formatted string (e.g. ``2026-05-20T08:00:00.0``)
    and falls back to the epoch-ms field.
    """
    fmt = _as_str(pr.get("prStartTimeGmtFormatted"))
    if fmt and "T" in fmt:
        return fmt.split(".")[0].rstrip("Z") + "Z"
    ms = _as_int(pr.get("prStartTimeGmt"))
    if ms is not None:
        return _epoch_ms_to_rfc3339(ms)
    return None


def map_day(raw: dict[str, Any], date: str) -> dict[str, Any]:
    """Map a full raw Garmin day into the per-capability request bodies."""
    return {
        "recovery": map_recovery(raw, date),
        "fitness": map_fitness(raw, date),
        "hydration_balance": map_hydration_balance(raw, date),
        "daily_summary": map_daily_summary(raw, date),
        "weights": map_weights(raw),
        "workouts": map_workouts(raw),
        "gear": map_gear(raw),
        "personal_records": map_personal_records(raw),
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
