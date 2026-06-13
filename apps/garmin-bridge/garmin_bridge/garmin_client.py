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
    # extend-recovery-fitness (change C): the remaining cheap daily signals. Each
    # is individually guarded — a failing/unavailable endpoint yields a missing
    # key, never an aborted day. Sleep-stage seconds and the training_status
    # label come free from already-fetched payloads (sleep DTO / training_status).
    raw["spo2"] = safe("spo2", lambda: api.get_spo2_data(date))
    raw["respiration"] = safe("respiration", lambda: api.get_respiration_data(date))
    raw["endurance_score"] = safe("endurance_score", lambda: api.get_endurance_score(date))
    raw["hill_score"] = safe("hill_score", lambda: api.get_hill_score(date))
    raw["fitness_age"] = safe("fitness_age", lambda: api.get_fitnessage_data(date))
    raw["hydration"] = safe("hydration", lambda: api.get_hydration_data(date))
    raw["user_summary"] = safe("user_summary", lambda: api.get_user_summary(date))
    # Athlete physiology config (per add-garmin-athlete-config): FTP, thresholds,
    # max HR, and HR/power-zone boundaries. Slowly-changing — refreshed in place
    # via the singleton PUT each sync. Sourced from the user profile + settings
    # (this garminconnect build exposes no dedicated heart-rate-zones call; the
    # zones ride in the user-settings payload). Both fetches guarded.
    raw["user_profile"] = safe("user_profile", lambda: api.get_user_profile())
    raw["userprofile_settings"] = safe(
        "userprofile_settings", lambda: api.get_userprofile_settings()
    )
    raw["weigh_ins"] = safe("weigh_ins", lambda: api.get_weigh_ins(date, date))
    raw["activities"] = safe(
        "activities", lambda: api.get_activities_by_date(date, date)
    )

    # Per-activity detail fan-out (per add-garmin-workout-detail). For each
    # activity we pull HR-zone time, per-lap splits, strength sets, and weather.
    # Each fetch is individually guarded: a failing/unavailable endpoint (e.g.
    # an indoor activity has no weather) yields absent detail for that activity,
    # never an aborted day. Keyed by activity id so the mapper can join them to
    # the matching activity summary.
    details: dict[str, Any] = {}
    for act in raw["activities"] or []:
        aid = act.get("activityId")
        if aid is None:
            continue
        details[str(aid)] = {
            "zones": safe(
                "activity_zones", lambda aid=aid: api.get_activity_hr_in_timezones(aid)
            ),
            "splits": safe("activity_splits", lambda aid=aid: api.get_activity_splits(aid)),
            "sets": safe(
                "activity_sets", lambda aid=aid: api.get_activity_exercise_sets(aid)
            ),
            "weather": safe("activity_weather", lambda aid=aid: api.get_activity_weather(aid)),
        }
    raw["activity_details"] = details

    # Slowly-changing inventory (per add-garmin-gear-and-prs): gear + personal
    # records. Not date-keyed — the backend upserts them by Garmin external id.
    # Each fetch is guarded so a bad inventory endpoint degrades to "no refresh
    # this sync", never an aborted day. Gear stats are fetched per gear uuid and
    # keyed so the mapper can join mileage onto the gear record.
    upn = _user_profile_number(api)
    gear_list = safe("gear", lambda: api.get_gear(upn)) if upn is not None else None
    raw["gear"] = gear_list
    gear_stats: dict[str, Any] = {}
    for g in gear_list or []:
        guid = g.get("uuid") or g.get("gearPk") or g.get("gearUuid")
        if guid is None:
            continue
        stat = safe("gear_stats", lambda guid=guid: api.get_gear_stats(guid))
        if stat is not None:
            gear_stats[str(guid)] = stat
    raw["gear_stats"] = gear_stats
    raw["personal_records"] = safe(
        "personal_records", lambda: api.get_personal_record()
    )
    return raw


def _user_profile_number(api) -> Any:
    """The Garmin `userProfilePk` the gear endpoints are keyed by.

    It lives on the garth client's social profile (``garth.profile``), not on the
    Garmin wrapper. We probe the common key shapes and return None if it can't be
    found — the gear fetch is then skipped, never fatal. Fully defensive: this
    runs outside the ``safe()`` guard, so it must not raise.
    """
    garth = getattr(api, "client", None) or getattr(api, "garth", None)
    prof = getattr(garth, "profile", None)
    if isinstance(prof, dict):
        for key in ("userProfilePk", "profileId", "userProfileId", "id"):
            if prof.get(key) is not None:
                return prof[key]
    return None


# --- structured-workout write/read (per add-garmin-scheduling) ----------
#
# These hit Garmin's workout-service / calendar-service through the garth
# client. Like fetch_day, they touch Garmin's churning private API and can't be
# unit-tested here; the payload they send is built by the pure, tested
# workout_builder module.


def create_workout(api, payload: dict[str, Any]) -> str:
    """Create a structured workout in the Garmin library; return its id."""
    resp = _garth(api).connectapi("/workout-service/workout", method="POST", json=payload)
    wid = (resp or {}).get("workoutId")
    if wid is None:
        raise RuntimeError("garmin did not return a workoutId")
    return str(wid)


def schedule_workout(api, workout_id: str, date: str) -> str:
    """Place a Garmin workout on a calendar date; return the schedule id."""
    resp = _garth(api).connectapi(
        f"/workout-service/schedule/{workout_id}", method="POST", json={"date": date}
    )
    sid = (resp or {}).get("id")
    if sid is None:
        raise RuntimeError("garmin did not return a schedule id")
    return str(sid)


def unschedule_workout(api, schedule_id: str) -> None:
    """Remove a scheduled calendar entry. A missing id is treated as a no-op."""
    try:
        _garth(api).connectapi(f"/workout-service/schedule/{schedule_id}", method="DELETE")
    except Exception as exc:  # noqa: BLE001
        # Idempotent unschedule: a 404 (already gone) is success.
        if "404" in str(exc) or "not found" in str(exc).lower():
            logger.info("schedule %s already absent; treating as no-op", schedule_id)
            return
        raise


def get_calendar(api, from_date: str, to_date: str) -> dict[str, Any]:
    """Return scheduled calendar items in [from_date, to_date] (YYYY-MM-DD).

    Garmin's calendar API is month-keyed, so we fetch each month the range spans
    and merge the items, then filter to the range.
    """
    from datetime import date as _date

    start = _date.fromisoformat(from_date)
    end = _date.fromisoformat(to_date)
    items: list[Any] = []
    seen_months: set[tuple[int, int]] = set()
    cur = start
    while cur <= end:
        key = (cur.year, cur.month)
        if key not in seen_months:
            seen_months.add(key)
            # Garmin month index is 0-based.
            resp = _garth(api).connectapi(
                f"/calendar-service/year/{cur.year}/month/{cur.month - 1}"
            )
            for it in (resp or {}).get("calendarItems", []) or []:
                d = it.get("date")
                if d is None or (from_date <= d <= to_date):
                    items.append(it)
        # Advance to the first of next month.
        if cur.month == 12:
            cur = _date(cur.year + 1, 1, 1)
        else:
            cur = _date(cur.year, cur.month + 1, 1)
    return {"from": from_date, "to": to_date, "items": items}


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
