"""Login control-plane: MFA path, token persistence, secret redaction."""

from __future__ import annotations

import logging
import types

from fastapi.testclient import TestClient

from garmin_bridge import garmin_client, logging_setup
from garmin_bridge.app import create_app
from tests.conftest import FakeBackend


def _gc_stub(*, needs_mfa=True, token="minted-token-blob", raise_on=None):
    """Build a stub garmin_client module-like object."""
    stub = types.SimpleNamespace(
        NEEDS_MFA=garmin_client.NEEDS_MFA,
        OK=garmin_client.OK,
        LoginError=garmin_client.LoginError,
    )

    def begin_login(email, password):
        if raise_on == "begin":
            raise garmin_client.LoginError("bad_credentials", "nope")
        if needs_mfa:
            return garmin_client.NEEDS_MFA, {"sso": "state"}
        return garmin_client.OK, token

    def resume_login(state, code):
        if raise_on == "resume":
            raise garmin_client.LoginError("mfa_invalid", "wrong code")
        return token

    stub.begin_login = begin_login
    stub.resume_login = resume_login
    return stub


def test_login_requires_mfa_returns_needs_mfa(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(needs_mfa=True), backend_factory=lambda: backend)
    client = TestClient(app)

    resp = client.post("/login")
    assert resp.status_code == 200
    assert resp.json() == {"needs_mfa": True}
    # No token persisted yet — MFA still pending.
    assert backend.put_calls == []


def test_mfa_completes_and_persists_blob(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(needs_mfa=True, token="MINTED-BLOB"), backend_factory=lambda: backend)
    client = TestClient(app)

    assert client.post("/login").json() == {"needs_mfa": True}

    resp = client.post("/login/mfa", json={"code": "123456"})
    assert resp.status_code == 200
    assert resp.json() == {"logged_in": True}
    # The minted blob was persisted to the backend, and never returned.
    assert backend.put_calls == [b"MINTED-BLOB"]
    assert "MINTED-BLOB" not in resp.text


def test_mfa_without_login_in_progress_409(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(), backend_factory=lambda: backend)
    client = TestClient(app)
    resp = client.post("/login/mfa", json={"code": "123456"})
    assert resp.status_code == 409
    assert resp.json()["error"] == "no_login_in_progress"


def test_bad_credentials_returns_typed_error(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(raise_on="begin"), backend_factory=lambda: backend)
    client = TestClient(app)
    resp = client.post("/login")
    assert resp.status_code == 401
    assert resp.json()["error"] == "bad_credentials"


def test_wrong_mfa_code_returns_typed_error(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(needs_mfa=True, raise_on="resume"), backend_factory=lambda: backend)
    client = TestClient(app)
    client.post("/login")
    resp = client.post("/login/mfa", json={"code": "000000"})
    assert resp.status_code == 401
    assert resp.json()["error"] == "mfa_invalid"


def test_no_login_without_mfa_persists_immediately(config):
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(needs_mfa=False, token="DIRECT-BLOB"), backend_factory=lambda: backend)
    client = TestClient(app)
    resp = client.post("/login")
    assert resp.status_code == 200
    assert resp.json() == {"logged_in": True}
    assert backend.put_calls == [b"DIRECT-BLOB"]


def test_password_and_blob_absent_from_logs(config, capsys):
    # configure() binds a StreamHandler to the (capsys-captured) stdout and
    # installs the redacting filter — the same path production uses.
    logging_setup.configure(level="DEBUG", secrets=config.sensitive_values)
    backend = FakeBackend()
    app = create_app(config, gc=_gc_stub(needs_mfa=True, token="SUPER-SECRET-BLOB"), backend_factory=lambda: backend)
    client = TestClient(app)

    # Deliberately try to log each secret; the filter must scrub them.
    log = logging.getLogger("garmin_bridge.app")
    log.info("attempting to leak password=%s", config.garmin_password)
    log.info("attempting to leak token=%s", config.garmin_api_token)

    client.post("/login")
    client.post("/login/mfa", json={"code": "123456"})

    out = capsys.readouterr().out
    assert config.garmin_password not in out
    assert "SUPER-SECRET-BLOB" not in out
    assert config.garmin_api_token not in out
    # Sanity: logging actually happened (otherwise the asserts are vacuous).
    assert "REDACTED" in out
