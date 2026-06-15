"""Unit tests for the step-model → garminconnect payload translation."""

from __future__ import annotations

import pytest

from garmin_bridge import workout_builder as wb


def _steps():
    return [
        {"type": "step", "intent": "warmup",
         "duration": {"kind": "time", "seconds": 600},
         "target": {"kind": "hr_zone", "low": 1, "high": 2}},
        {"type": "repeat", "count": 5, "steps": [
            {"type": "step", "intent": "interval",
             "duration": {"kind": "time", "seconds": 180},
             "target": {"kind": "power_zone", "low": 4, "high": 4}},
            {"type": "step", "intent": "recovery",
             "duration": {"kind": "distance", "meters": 200},
             "target": {"kind": "none"}},
        ]},
        {"type": "step", "intent": "cooldown",
         "duration": {"kind": "lap_button"},
         "target": {"kind": "pace", "low_sec_per_km": 300, "high_sec_per_km": 330}},
    ]


def test_top_level_shape():
    p = wb.build_payload("run", "VO2", _steps())
    assert p["workoutName"] == "VO2"
    assert p["sportType"] == {"sportTypeId": 1, "sportTypeKey": "running"}
    seg = p["workoutSegments"][0]
    assert seg["segmentOrder"] == 1
    assert len(seg["workoutSteps"]) == 3


def test_warmup_step_and_hr_zone_range():
    p = wb.build_payload("run", "VO2", _steps())
    warmup = p["workoutSegments"][0]["workoutSteps"][0]
    assert warmup["type"] == "ExecutableStepDTO"
    assert warmup["stepType"]["stepTypeKey"] == "warmup"
    assert warmup["endCondition"]["conditionTypeKey"] == "time"
    assert warmup["endConditionValue"] == 600.0
    assert warmup["targetType"]["workoutTargetTypeKey"] == "heart.rate.zone"
    assert warmup["targetValueOne"] == 1
    assert warmup["targetValueTwo"] == 2


def test_repeat_group_shape_and_nesting():
    p = wb.build_payload("run", "VO2", _steps())
    rep = p["workoutSegments"][0]["workoutSteps"][1]
    assert rep["type"] == "RepeatGroupDTO"
    assert rep["stepType"]["stepTypeKey"] == "repeat"
    assert rep["numberOfIterations"] == 5
    assert rep["endCondition"]["conditionTypeKey"] == "iterations"
    assert rep["endConditionValue"] == 5
    assert len(rep["workoutSteps"]) == 2
    # Single power zone (low==high) collapses to zoneNumber.
    interval = rep["workoutSteps"][0]
    assert interval["stepType"]["stepTypeKey"] == "interval"
    assert interval["targetType"]["workoutTargetTypeKey"] == "power.zone"
    assert interval["zoneNumber"] == 4
    # Distance recovery, untargeted.
    recovery = rep["workoutSteps"][1]
    assert recovery["endCondition"]["conditionTypeKey"] == "distance"
    assert recovery["endConditionValue"] == 200.0
    assert recovery["targetType"]["workoutTargetTypeKey"] == "no.target"


def test_steporder_is_monotonic_including_nested():
    p = wb.build_payload("run", "VO2", _steps())
    steps = p["workoutSegments"][0]["workoutSteps"]
    orders = [steps[0]["stepOrder"], steps[1]["stepOrder"]]
    orders += [c["stepOrder"] for c in steps[1]["workoutSteps"]]
    orders.append(steps[2]["stepOrder"])
    assert orders == sorted(orders), "stepOrder strictly increases across nesting"
    assert len(set(orders)) == len(orders), "stepOrder values are unique"


def test_cooldown_lapbutton_has_no_value_and_pace_converts():
    p = wb.build_payload("run", "VO2", _steps())
    cooldown = p["workoutSegments"][0]["workoutSteps"][2]
    assert cooldown["endCondition"]["conditionTypeKey"] == "lap.button"
    assert cooldown["endConditionValue"] is None
    assert cooldown["targetType"]["workoutTargetTypeKey"] == "pace.zone"
    # 300 sec/km → 1000/300 m/s.
    assert cooldown["targetValueOne"] == pytest.approx(1000.0 / 300.0)
    assert cooldown["targetValueTwo"] == pytest.approx(1000.0 / 330.0)


def test_open_duration_maps_to_lapbutton():
    p = wb.build_payload("swim", "Swim", [
        {"type": "step", "intent": "active", "duration": {"kind": "open"}, "target": {"kind": "none"}},
    ])
    step = p["workoutSegments"][0]["workoutSteps"][0]
    assert step["endCondition"]["conditionTypeKey"] == "lap.button"
    assert step["stepType"]["stepTypeKey"] == "other"  # 'active' maps to Garmin 'other'


def test_each_sport_maps():
    # (sportTypeId, sportTypeKey) per Garmin's workout-service vocabulary. Garmin
    # validates by id, so the id matters as much as the key. "other" is id 3
    # (id 9 is hiit); yoga=7, mobility=11.
    cases = [
        ("run", 1, "running"),
        ("bike", 2, "cycling"),
        ("swim", 4, "swimming"),
        ("strength", 5, "strength_training"),
        ("yoga", 7, "yoga"),
        ("mobility", 11, "mobility"),
        ("other", 3, "other"),
    ]
    for sport, sid, key in cases:
        p = wb.build_payload(sport, "x", [{"type": "step", "intent": "active", "duration": {"kind": "open"}, "target": {"kind": "none"}}])
        assert p["sportType"] == {"sportTypeId": sid, "sportTypeKey": key}


def test_absolute_hr_and_power_targets_use_value_range():
    p = wb.build_payload("run", "x", [
        {"type": "step", "intent": "active", "duration": {"kind": "time", "seconds": 60},
         "target": {"kind": "hr_bpm", "low": 140, "high": 155}},
    ])
    step = p["workoutSegments"][0]["workoutSteps"][0]
    assert step["targetType"]["workoutTargetTypeKey"] == "heart.rate.zone"
    assert step["targetValueOne"] == 140
    assert step["targetValueTwo"] == 155


@pytest.mark.parametrize("bad", [
    {"sport": "chess", "name": "x", "steps": [{"type": "step", "intent": "active", "duration": {"kind": "open"}, "target": {"kind": "none"}}]},
    {"sport": "run", "name": "x", "steps": []},
    {"sport": "run", "name": "x", "steps": [{"type": "mystery"}]},
    {"sport": "run", "name": "x", "steps": [{"type": "step", "intent": "vibe", "duration": {"kind": "open"}, "target": {"kind": "none"}}]},
    {"sport": "run", "name": "x", "steps": [{"type": "step", "intent": "active", "duration": {"kind": "warp"}, "target": {"kind": "none"}}]},
    {"sport": "run", "name": "x", "steps": [{"type": "step", "intent": "active", "duration": {"kind": "time"}, "target": {"kind": "none"}}]},
    {"sport": "run", "name": "x", "steps": [{"type": "repeat", "count": 1, "steps": [{"type": "step", "intent": "active", "duration": {"kind": "open"}, "target": {"kind": "none"}}]}]},
    {"sport": "run", "name": "x", "steps": [{"type": "repeat", "count": 2, "steps": [{"type": "repeat", "count": 2, "steps": []}]}]},
])
def test_build_errors(bad):
    with pytest.raises(wb.BuildError):
        wb.build_payload(bad["sport"], bad["name"], bad["steps"])
