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
        "spo2_avg": 95,
        "spo2_lowest": 89,
        "respiration_avg": 13.4,
        "respiration_lowest": 9.8,
        "deep_sleep_seconds": 6000,
        "light_sleep_seconds": 15000,
        "rem_sleep_seconds": 5400,
        "awake_seconds": 600,
    }


def test_recovery_omits_absent_extended_fields():
    """SpO2 / respiration / sleep-stage fields absent → simply omitted."""
    raw = {"sleep": {"dailySleepDTO": {"sleepTimeSeconds": 27000}}}
    rec = mapping.map_recovery(raw, "2026-06-12")
    assert rec == {"date": "2026-06-12", "sleep_seconds": 27000}
    for k in (
        "spo2_avg",
        "spo2_lowest",
        "respiration_avg",
        "respiration_lowest",
        "deep_sleep_seconds",
        "light_sleep_seconds",
        "rem_sleep_seconds",
        "awake_seconds",
    ):
        assert k not in rec


def test_fitness_mapping(raw_day):
    fit = mapping.map_fitness(raw_day, "2026-06-12")
    assert fit["vo2max_running"] == 56.4
    assert fit["vo2max_cycling"] == 60.1
    assert fit["race_predictor_5k_seconds"] == 1180
    assert fit["race_predictor_full_seconds"] == 11820
    assert fit["acute_load"] == 412.0
    assert fit["chronic_load"] == 388.5
    assert fit["endurance_score"] == 7200
    assert fit["hill_score"] == 61
    assert fit["fitness_age"] == 34.0
    # training_status label passes through verbatim from the already-fetched payload
    assert fit["training_status"] == "productive"


def test_fitness_omits_absent_extended_fields():
    """endurance / hill / fitness-age / training-status absent → omitted."""
    raw = {"max_metrics": [{"generic": {"vo2MaxPreciseValue": 54.0}}]}
    fit = mapping.map_fitness(raw, "2026-06-12")
    assert fit == {"date": "2026-06-12", "vo2max_running": 54.0}
    for k in ("endurance_score", "hill_score", "fitness_age", "training_status"):
        assert k not in fit


def test_training_status_label_falls_back_to_top_level():
    """A top-level trainingStatus phrase is used when no per-device entry."""
    raw = {"training_status": {"trainingStatus": "maintaining"}}
    fit = mapping.map_fitness(raw, "2026-06-12")
    assert fit["training_status"] == "maintaining"


def test_training_status_ignores_non_string_codes():
    """A numeric trainingStatus code is not a label → omitted, not coerced."""
    raw = {"training_status": {"latestTrainingStatusData": {"42": {"trainingStatus": 3}}}}
    fit = mapping.map_fitness(raw, "2026-06-12")
    assert fit is None or "training_status" not in fit


def test_hydration_balance_mapping(raw_day):
    hb = mapping.map_hydration_balance(raw_day, "2026-06-12")
    assert hb == {
        "date": "2026-06-12",
        "sweat_loss_ml": 950.0,
        "activity_intake_ml": 400.0,
        "goal_ml": 2600.0,
    }


def test_daily_summary_mapping(raw_day):
    ds = mapping.map_daily_summary(raw_day, "2026-06-12")
    assert ds == {
        "date": "2026-06-12",
        "active_kcal": 820,
        "resting_kcal": 1650,
        "total_kcal": 2470,
        "steps": 12400,
        "floors": 14,
        "moderate_intensity_minutes": 35,
        "vigorous_intensity_minutes": 48,
        # raw float preserved; the backend rounds at the response boundary
        "distance_m": 9320.4789,
    }


def test_daily_summary_omits_absent_fields():
    raw = {"user_summary": {"totalKilocalories": 2100}}
    ds = mapping.map_daily_summary(raw, "2026-06-12")
    assert ds == {"date": "2026-06-12", "total_kcal": 2100}


def test_daily_summary_empty_payload_yields_none():
    assert mapping.map_daily_summary({}, "2026-06-12") is None
    assert mapping.map_daily_summary({"user_summary": {}}, "2026-06-12") is None


def test_gear_mapping_joins_stats(raw_day):
    gear = mapping.map_gear(raw_day)
    by_id = {g["external_id"]: g for g in gear}

    shoes = by_id["gear-shoes-1"]
    assert shoes["gear_type"] == "shoes"
    assert shoes["display_name"] == "Daily Trainers"
    assert shoes["total_distance_m"] == 780000.0  # joined from gear_stats
    assert shoes["total_activities"] == 120
    assert shoes.get("retired", False) is False
    assert shoes["date_begin"] == "2025-01-01"

    bike = by_id["gear-bike-1"]
    assert bike["gear_type"] == "bike"
    assert bike["retired"] is True
    assert bike["total_distance_m"] == 5000000.0
    assert bike["date_end"] == "2026-04-01"


def test_gear_mapping_absent_stats_and_type_fallback(raw_day):
    """A gear with no stats syncs without mileage; an unmapped type → other."""
    kayak = next(g for g in mapping.map_gear(raw_day) if g["external_id"] == "gear-kayak-1")
    assert kayak["gear_type"] == "other"  # 'Kayak' not in the enum
    assert kayak["display_name"] == "Sea Kayak"
    assert "total_distance_m" not in kayak  # no gear_stats entry → omitted
    assert "total_activities" not in kayak


def test_personal_records_mapping(raw_day):
    prs = mapping.map_personal_records(raw_day)
    by_id = {p["external_id"]: p for p in prs}

    fivek = by_id["101"]
    assert fivek["pr_type"] == "5k"
    assert fivek["value"] == 1320.0
    assert fivek["unit"] == "s"
    assert fivek["activity_id"] == "555"
    assert fivek["achieved_at"] == "2026-05-20T08:00:00Z"

    tenk = by_id["102"]
    assert tenk["pr_type"] == "10k"
    assert tenk["value"] == 2790.5
    assert "activity_id" not in tenk  # absent → omitted

    ride = by_id["103"]
    assert ride["pr_type"] == "longest-ride"
    assert ride["unit"] == "m"
    assert ride["value"] == 90000.0


def test_inventory_empty_when_absent():
    assert mapping.map_gear({}) == []
    assert mapping.map_personal_records({}) == []


def test_athlete_config_mapping(raw_day):
    cfg = mapping.map_athlete_config(raw_day)
    assert cfg["ftp_watts"] == 265
    assert cfg["threshold_hr"] == 168
    assert cfg["lactate_threshold_hr"] == 165
    assert cfg["max_hr"] == 188
    # threshold speeds (m/s) → paces
    assert cfg["threshold_pace_sec_per_km"] == 250.0  # 1000 / 4.0
    assert cfg["threshold_swim_pace_sec_per_100m"] == 80.0  # 100 / 1.25
    assert cfg["hr_zone_1_max"] == 120
    assert cfg["hr_zone_5_max"] == 182
    assert cfg["power_zone_3_max"] == 240
    assert cfg["power_zone_5_max"] == 350


def test_athlete_config_power_zones_absent():
    """HR zones present, power zones absent → power_zone_* omitted."""
    raw = {
        "userprofile_settings": {
            "userData": {"ftpAutoDetected": 250},
            "heartRateZones": [
                {"zoneNumber": 1, "zoneHigh": 120},
                {"zoneNumber": 2, "zoneHigh": 140},
            ],
        }
    }
    cfg = mapping.map_athlete_config(raw)
    assert cfg["ftp_watts"] == 250
    assert cfg["hr_zone_1_max"] == 120
    assert not any(k.startswith("power_zone_") for k in cfg)
    # threshold paces absent (no speeds supplied) → omitted
    assert "threshold_pace_sec_per_km" not in cfg


def test_athlete_config_empty_yields_none():
    assert mapping.map_athlete_config({}) is None
    assert mapping.map_athlete_config({"userprofile_settings": {"userData": {}}}) is None


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
    assert len(workouts) == 3

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


def test_workouts_scalar_detail_from_summary(raw_day):
    """Scalar performance fields ride along the activity summary — no extra call."""
    run = mapping.map_workouts(raw_day)[0]
    assert run["elevation_gain_m"] == 120.0
    assert run["elevation_loss_m"] == 95.0
    assert run["normalized_power_w"] == 245
    assert run["intensity_factor"] == 0.82
    assert run["avg_cadence"] == 178
    assert run["avg_stride_m"] == 1.15  # 115 cm → m
    assert run["max_hr"] == 176
    assert run["aerobic_te"] == 3.4
    assert run["anaerobic_te"] == 1.2


def test_workouts_zone_and_weather_detail(raw_day):
    run = mapping.map_workouts(raw_day)[0]
    assert run["secs_in_zone_1"] == 300
    assert run["secs_in_zone_2"] == 1200
    assert run["secs_in_zone_3"] == 1500
    assert run["secs_in_zone_4"] == 500
    assert run["secs_in_zone_5"] == 100
    assert run["humidity_pct"] == 72.0
    assert run["wind_speed_mps"] == 3.5


def test_workouts_splits_detail(raw_day):
    run = mapping.map_workouts(raw_day)[0]
    splits = run["splits"]
    assert len(splits) == 2
    assert splits[0] == {
        "split_index": 0,
        "distance_m": 6000.0,
        "duration_s": 1800.0,
        "avg_hr": 150,
        "avg_power_w": 240,
        "avg_speed_mps": 3.333,
        "elevation_gain_m": 60.0,
    }
    assert splits[1]["split_index"] == 1
    assert splits[1]["avg_speed_mps"] == 3.305
    # endurance run carries no strength sets
    assert "sets" not in run


def test_workouts_sets_detail_for_strength(raw_day):
    """The strength activity carries sets (active only), no splits/weather."""
    lift = mapping.map_workouts(raw_day)[2]
    assert lift["external_id"] == "garmin:1234569"
    assert lift["sport"] == "strength"
    sets = lift["sets"]
    assert len(sets) == 2  # the REST set is dropped
    assert sets[0] == {
        "set_index": 0,
        "exercise_name": "BENCH_PRESS",
        "exercise_category": "BENCH_PRESS",
        "reps": 12,
        "weight_kg": 40.0,  # 40000 g → kg
        "duration_s": 45.0,
    }
    assert sets[1]["set_index"] == 1
    assert sets[1]["weight_kg"] == 42.5
    # indoor strength session → no weather, no splits
    assert "humidity_pct" not in lift
    assert "wind_speed_mps" not in lift
    assert "splits" not in lift


def test_workouts_detail_absent_when_no_activity_details():
    """An activity with no detail entry maps to a bare item (no scalar/zone keys)."""
    raw = {
        "activities": [
            {
                "activityId": 42,
                "activityType": {"typeKey": "running"},
                "startTimeGMT": "2026-06-12 10:00:00",
                "duration": 600.0,
            }
        ]
    }
    item = mapping.map_workouts(raw)[0]
    assert "elevation_gain_m" not in item
    assert "secs_in_zone_1" not in item
    assert "splits" not in item
    assert "sets" not in item


def test_map_day_aggregates_all(raw_day):
    mapped = mapping.map_day(raw_day, "2026-06-12")
    assert mapped["recovery"] is not None
    assert mapped["fitness"] is not None
    assert mapped["hydration_balance"] is not None
    assert mapped["daily_summary"] is not None
    assert len(mapped["weights"]) == 1
    assert len(mapped["workouts"]) == 3


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
