"""garmin_client wiring against a fake garth client (no network).

Locks the contract we verified against the installed garminconnect 0.3.2:
``login()`` returns ``("needs_mfa", client_state)``; the garth client is reached
via ``.client``; ``dumps``/``loads``/``refresh_oauth2`` are how tokens move.
"""

from __future__ import annotations

import garmin_bridge.garmin_client as gc


class FakeGarth:
    def __init__(self):
        self.loaded = None
        self.refreshed = False

    def dumps(self) -> str:
        return "TOKEN-BLOB"

    def loads(self, blob: str) -> None:
        self.loaded = blob

    def refresh_oauth2(self) -> None:
        self.refreshed = True


class FakeGarmin:
    """Mimics garminconnect.Garmin's MFA two-step + .client exposure."""

    def __init__(self, email="", password="", return_on_mfa=False, needs_mfa=True):
        self.client = FakeGarth()
        self._needs_mfa = needs_mfa

    def login(self):
        if self._needs_mfa:
            return "needs_mfa", {"sso": "state"}
        return None, None

    def resume_login(self, client_state, code):
        assert client_state == {"sso": "state"}
        assert code == "123456"
        return None, None


def test_begin_login_needs_mfa(monkeypatch):
    monkeypatch.setattr(gc, "_new_api", lambda e="", p="": FakeGarmin(needs_mfa=True))
    result, state = gc.begin_login("a@b.com", "pw")
    assert result == gc.NEEDS_MFA
    assert state["client_state"] == {"sso": "state"}


def test_begin_login_no_mfa_returns_token(monkeypatch):
    monkeypatch.setattr(gc, "_new_api", lambda e="", p="": FakeGarmin(needs_mfa=False))
    result, token = gc.begin_login("a@b.com", "pw")
    assert result == gc.OK
    assert token == "TOKEN-BLOB"


def test_resume_login_returns_token(monkeypatch):
    monkeypatch.setattr(gc, "_new_api", lambda e="", p="": FakeGarmin(needs_mfa=True))
    _, state = gc.begin_login("a@b.com", "pw")
    token = gc.resume_login(state, "123456")
    assert token == "TOKEN-BLOB"


def test_load_api_loads_and_refreshes(monkeypatch):
    fake = FakeGarmin(needs_mfa=False)
    monkeypatch.setattr(gc, "Garmin", FakeGarmin, raising=False)
    # load_api constructs garminconnect.Garmin() directly; patch the import site.
    import garminconnect

    monkeypatch.setattr(garminconnect, "Garmin", lambda *a, **k: fake)
    api = gc.load_api("STORED-BLOB")
    assert api is fake
    assert fake.client.loaded == "STORED-BLOB"
    assert fake.client.refreshed is True


def test_garth_accessor_prefers_client():
    api = FakeGarmin()
    assert gc._garth(api) is api.client


def test_classify_maps_known_errors():
    assert gc._classify(Exception("invalid MFA code")).code == "mfa_invalid"
    assert gc._classify(Exception("Too many login attempts")).code == "locked_out"
    assert gc._classify(Exception("401 bad credential")).code == "bad_credentials"
    assert gc._classify(Exception("weird")).code == "login_failed"
