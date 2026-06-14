"""Config fail-fast behaviour."""

from __future__ import annotations

import pytest

from garmin_bridge import config


def _full_env() -> dict[str, str]:
    return {
        "GARMIN_EMAIL": "a@b.com",
        "GARMIN_PASSWORD": "pw",
        "NUTRITION_API_URL": "http://kazper/",
        "GARMIN_API_TOKEN": "tok",
    }


def test_load_ok_strips_trailing_slash():
    cfg = config.load(_full_env())
    assert cfg.nutrition_api_url == "http://kazper"
    assert cfg.sync_tz == "UTC"


def test_sync_lookback_days_default_and_overrides():
    assert config.load(_full_env()).sync_lookback_days == 2  # default → 3-day window
    assert config.load({**_full_env(), "SYNC_LOOKBACK_DAYS": "5"}).sync_lookback_days == 5
    assert config.load({**_full_env(), "SYNC_LOOKBACK_DAYS": "0"}).sync_lookback_days == 0
    # negative clamps to the floor; invalid falls back to default.
    assert config.load({**_full_env(), "SYNC_LOOKBACK_DAYS": "-3"}).sync_lookback_days == 0
    assert config.load({**_full_env(), "SYNC_LOOKBACK_DAYS": "x"}).sync_lookback_days == 2


@pytest.mark.parametrize("missing", ["GARMIN_EMAIL", "GARMIN_PASSWORD", "NUTRITION_API_URL", "GARMIN_API_TOKEN"])
def test_missing_required_raises(missing):
    env = _full_env()
    del env[missing]
    with pytest.raises(config.ConfigError) as exc:
        config.load(env)
    assert missing in str(exc.value)


def test_bad_url_scheme_raises():
    env = _full_env()
    env["NUTRITION_API_URL"] = "kazper"
    with pytest.raises(config.ConfigError):
        config.load(env)


def test_sensitive_values_excludes_empty():
    cfg = config.load(_full_env())
    assert set(cfg.sensitive_values) == {"pw", "tok"}
