"""Mapping: recorded Garmin fixture → expected REST request bodies."""

from __future__ import annotations

from garmin_bridge import mapping


def test_recovery_mapping(raw_day):
    rec = mapping.map_recovery(raw_day, "2026-06-12")
    assert rec == {
        "date": "2026-06-12",
        "sleep_seconds": 27000,
        "sleep_score": 82,
        "hrv_ms": 64.5,
        "resting_hr": 48,
        "stress_avg": 31,
        "body_battery_charged": 61,
        "body_battery_drained": 44,
        "training_readiness": 73,
    }


def test_fitness_mapping(raw_day):
    fit = mapping.map_fitness(raw_day, "2026-06-12")
    assert fit["vo2max_running"] == 56.4
    assert fit["vo2max_cycling"] == 60.1
    assert fit["race_predictor_5k_seconds"] == 1180
    assert fit["race_predictor_full_seconds"] == 11820
    assert fit["acute_load"] == 412.0
    assert fit["chronic_load"] == 388.5


def test_hydration_balance_mapping(raw_day):
    hb = mapping.map_hydration_balance(raw_day, "2026-06-12")
    assert hb == {
        "date": "2026-06-12",
        "sweat_loss_ml": 950.0,
        "activity_intake_ml": 400.0,
        "goal_ml": 2600.0,
    }


def test_weight_mapping_grams_to_kg(raw_day):
    weights = mapping.map_weights(raw_day)
    assert len(weights) == 1
    w = weights[0]
    assert w["weight_kg"] == 72.5
    assert w["muscle_mass_kg"] == 34.2
    assert w["bone_mass_kg"] == 3.1
    assert w["body_fat_pct"] == 12.4
    assert w["bmi"] == 22.1
    assert w["logged_at"] == "2026-06-12T00:00:00Z"


def test_workouts_mapping(raw_day):
    workouts = mapping.map_workouts(raw_day)
    assert len(workouts) == 2

    run = workouts[0]
    assert run["external_id"] == "garmin:1234567"
    assert run["source"] == "garmin"
    assert run["sport"] == "run"
    assert run["status"] == "completed"
    assert run["name"] == "Morning Run"
    assert run["started_at"] == "2026-06-12T05:30:00Z"
    assert run["ended_at"] == "2026-06-12T06:30:00Z"  # +3600s
    assert run["kcal_burned"] == 780.0
    assert run["avg_hr"] == 152
    assert run["distance_m"] == 12000.0
    # avg_power 0 is falsy but valid; _as_int keeps it
    assert run["avg_power_w"] == 0

    bike = workouts[1]
    assert bike["sport"] == "bike"
    assert bike["avg_power_w"] == 210
    assert bike["session_group"] == "garmin:1234566"  # parentId → brick link


def test_map_day_aggregates_all(raw_day):
    mapped = mapping.map_day(raw_day, "2026-06-12")
    assert mapped["recovery"] is not None
    assert mapped["fitness"] is not None
    assert mapped["hydration_balance"] is not None
    assert len(mapped["weights"]) == 1
    assert len(mapped["workouts"]) == 2


def test_empty_day_yields_none_snapshots():
    mapped = mapping.map_day({"date": "2026-06-12"}, "2026-06-12")
    assert mapped["recovery"] is None
    assert mapped["fitness"] is None
    assert mapped["hydration_balance"] is None
    assert mapped["weights"] == []
    assert mapped["workouts"] == []


def test_malformed_sections_are_tolerated():
    raw = {
        "sleep": "not-a-dict",
        "hrv": None,
        "max_metrics": [],
        "activities": [{"activityId": 9, "startTimeGMT": "bad"}],
    }
    mapped = mapping.map_day(raw, "2026-06-12")
    assert mapped["recovery"] is None
    assert mapped["fitness"] is None
    # activity with unparseable start time is dropped, not crashed
    assert mapped["workouts"] == []


def test_unknown_sport_falls_back_to_other():
    raw = {
        "activities": [
            {
                "activityId": 5,
                "activityType": {"typeKey": "underwater_basket_weaving"},
                "startTimeGMT": "2026-06-12 10:00:00",
                "duration": 600.0,
            }
        ]
    }
    workouts = mapping.map_workouts(raw)
    assert workouts[0]["sport"] == "other"
