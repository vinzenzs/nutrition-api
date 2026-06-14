"""garmin_client wiring against a fake garth client (no network).

Locks the contract we verified against the installed garminconnect 0.3.2:
``login()`` returns ``("needs_mfa", client_state)``; the garth client is reached
via ``.client``; ``dumps``/``loads``/``refresh_oauth2`` are how tokens move.
"""

from __future__ import annotations

import garmin_bridge.garmin_client as gc


class FakeGarth:
    def __init__(self, profile=None):
        self.loaded = None
        self.refreshed = False
        # garth.dumps/loads persists only OAuth tokens, so load_api re-fetches
        # the profile live via connectapi; this fake serves it from that call.
        self._profile = profile if profile is not None else {}

    def dumps(self) -> str:
        return "TOKEN-BLOB"

    def loads(self, blob: str) -> None:
        self.loaded = blob

    def refresh_oauth2(self) -> None:
        self.refreshed = True

    def connectapi(self, path: str, **kwargs):
        if path == "/userprofile-service/socialProfile":
            return self._profile
        return None


class FakeGarmin:
    """Mimics garminconnect.Garmin's MFA two-step + .client exposure."""

    def __init__(self, email="", password="", return_on_mfa=False, needs_mfa=True, profile=None):
        self.client = FakeGarth(profile=profile)
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


def test_load_api_restores_display_name_from_profile(monkeypatch):
    # The live profile fetch returns the display name → the rehydrated client
    # must expose it, so per-user endpoints don't interpolate None into the path
    # (the cause of the .../None 403s).
    fake = FakeGarmin(needs_mfa=False, profile={"displayName": "edge_sport", "fullName": "Edge Sport"})
    import garminconnect

    monkeypatch.setattr(garminconnect, "Garmin", lambda *a, **k: fake)
    api = gc.load_api("STORED-BLOB")
    assert api.display_name == "edge_sport"
    assert api.full_name == "Edge Sport"


def test_load_api_missing_display_name_is_none(monkeypatch):
    # The account's profile has no displayName → None (and a warning naming the
    # returned fields is logged); the display name must be set on the account.
    fake = FakeGarmin(needs_mfa=False, profile={})
    import garminconnect

    monkeypatch.setattr(garminconnect, "Garmin", lambda *a, **k: fake)
    api = gc.load_api("STORED-BLOB")
    assert api.display_name is None


def test_garth_accessor_prefers_client():
    api = FakeGarmin()
    assert gc._garth(api) is api.client


def test_classify_maps_known_errors():
    assert gc._classify(Exception("invalid MFA code")).code == "mfa_invalid"
    assert gc._classify(Exception("Too many login attempts")).code == "locked_out"
    assert gc._classify(Exception("401 bad credential")).code == "bad_credentials"
    assert gc._classify(Exception("weird")).code == "login_failed"


# --- workout-library management + export (garmin-workout-library-mgmt) ----


class RecordingGarth:
    """A garth stub that records connectapi calls and can raise a 404."""

    def __init__(self, raise_404_on_delete=False):
        self.calls: list[tuple[str, str]] = []
        self._raise_404 = raise_404_on_delete

    def connectapi(self, path, method="GET", **kwargs):
        self.calls.append((method, path))
        if method == "DELETE" and self._raise_404:
            raise Exception("Error 404: not found")
        if method == "GET" and "/workouts?" in path:
            return {"workouts": [{"workoutId": 1}]}
        if method == "GET":
            return {"workoutId": 99}
        if method == "POST":
            return {"workoutId": 1, "id": "sched"}
        return None


class ApiWithGarth:
    def __init__(self, garth):
        self.client = garth
        self.hydration_calls: list[dict] = []
        self.download_calls: list[tuple] = []

    def add_hydration_data(self, value_in_ml, timestamp=None, cdate=None):
        self.hydration_calls.append({"value_in_ml": value_in_ml, "cdate": cdate})
        return {"ok": True}

    def download_activity(self, activity_id, dl_fmt=None):
        self.download_calls.append((activity_id, dl_fmt))
        return b"FITBYTES"


class BundledGarth:
    """Mimics the garminconnect-bundled client (garminconnect 0.3.2).

    Its ``connectapi`` hardcodes GET and forwards ``**kwargs`` into
    ``_run_request(method, path, **kwargs)``, so passing ``method=`` collides on
    the positional ``method`` ("got multiple values for argument 'method'") —
    the exact prod 502 on POST /workouts. Writes must go through the verb helpers.
    """

    def __init__(self):
        self.calls: list[tuple[str, str]] = []

    def _run_request(self, method, path, **kwargs):
        self.calls.append((method, path))
        return {"workoutId": 555, "id": "sched-1"}

    def connectapi(self, path, **kwargs):  # no `method` param — like the real one
        return self._run_request("GET", path, **kwargs)

    def post(self, _domain, path, **kwargs):
        kwargs.pop("api", None)
        return self._run_request("POST", path, **kwargs)

    def delete(self, _domain, path, **kwargs):
        kwargs.pop("api", None)
        return self._run_request("DELETE", path, **kwargs)


def test_bundled_connectapi_method_kwarg_collides():
    # Guards that BundledGarth faithfully reproduces the prod failure mode: the
    # old code path (connectapi(path, method=...)) raises the very TypeError seen
    # in the prod logs, which _connect_write must avoid.
    client = BundledGarth()
    try:
        client.connectapi("/workout-service/workout", method="POST")
        raised = None
    except TypeError as exc:
        raised = str(exc)
    assert raised is not None and "method" in raised


def test_create_workout_bundled_client_uses_post():
    # Regression for the prod 502: with the bundled client, create must route
    # through .post (not connectapi(method=)) and still return the workout id.
    api = ApiWithGarth(BundledGarth())
    assert gc.create_workout(api, {"name": "Z2"}) == "555"
    assert api.client.calls == [("POST", "/workout-service/workout")]


def test_create_workout_garth_client_uses_connectapi():
    # The other variant (garth's connectapi accepts method=) still works.
    api = ApiWithGarth(RecordingGarth())
    gc.create_workout(api, {"name": "Z2"})
    assert ("POST", "/workout-service/workout") in api.client.calls


def test_schedule_workout_reads_workout_schedule_id():
    # Garmin returns the schedule id as `workoutScheduleId` (not bare `id`);
    # reading `id` raised "garmin did not return a schedule id" -> false 502.
    class G:
        def connectapi(self, path, **kwargs):  # bundled-style (no method param)
            return None

        def post(self, _domain, path, **kwargs):
            return {"workoutScheduleId": 778899, "workout": {"workoutId": 1}}

    api = ApiWithGarth(G())
    assert gc.schedule_workout(api, "gw-1", "2026-06-20") == "778899"


def test_schedule_and_unschedule_bundled_client():
    api = ApiWithGarth(BundledGarth())
    assert gc.schedule_workout(api, "gw-1", "2026-06-20") == "sched-1"
    gc.unschedule_workout(api, "sched-1")
    assert ("POST", "/workout-service/schedule/gw-1") in api.client.calls
    assert ("DELETE", "/workout-service/schedule/sched-1") in api.client.calls


def test_delete_workout_bundled_client_uses_delete():
    api = ApiWithGarth(BundledGarth())
    assert gc.delete_workout(api, "gw-1") is True
    assert api.client.calls == [("DELETE", "/workout-service/workout/gw-1")]


def test_delete_workout_issues_delete():
    api = ApiWithGarth(RecordingGarth())
    assert gc.delete_workout(api, "gw-1") is True
    assert api.client.calls == [("DELETE", "/workout-service/workout/gw-1")]


def test_delete_workout_404_is_noop():
    api = ApiWithGarth(RecordingGarth(raise_404_on_delete=True))
    assert gc.delete_workout(api, "gw-gone") is False


def test_get_workouts_and_by_id_paths():
    api = ApiWithGarth(RecordingGarth())
    assert gc.get_workouts(api, start=10, limit=5) == {"workouts": [{"workoutId": 1}]}
    assert gc.get_workout_by_id(api, "gw-7") == {"workoutId": 99}
    assert ("GET", "/workout-service/workouts?start=10&limit=5") in api.client.calls
    assert ("GET", "/workout-service/workout/gw-7") in api.client.calls


def test_add_hydration_data_passes_value_and_date():
    api = ApiWithGarth(RecordingGarth())
    gc.add_hydration_data(api, 750.0, "2026-06-13")
    assert api.hydration_calls == [{"value_in_ml": 750.0, "cdate": "2026-06-13"}]


def test_download_activity_maps_format():
    from garminconnect import Garmin

    api = ApiWithGarth(RecordingGarth())
    data = gc.download_activity(api, "act-1", "fit")
    assert data == b"FITBYTES"
    assert api.download_calls[0][0] == "act-1"
    assert api.download_calls[0][1] == Garmin.ActivityDownloadFormat.ORIGINAL
    gc.download_activity(api, "act-1", "gpx")
    assert api.download_calls[1][1] == Garmin.ActivityDownloadFormat.GPX
