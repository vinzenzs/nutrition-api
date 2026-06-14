"""Shared test fixtures and stubs."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from garmin_bridge.config import Config

_FIXTURE = Path(__file__).parent / "fixtures" / "garmin_day.json"


@pytest.fixture
def raw_day() -> dict:
    """A recorded-shape Garmin day."""
    return json.loads(_FIXTURE.read_text())


@pytest.fixture
def config() -> Config:
    return Config(
        garmin_email="athlete@example.com",
        garmin_password="hunter2-very-secret",
        nutrition_api_url="http://kazper",
        garmin_api_token="garmin-token-abcdef-0123456789",
        sync_tz="UTC",
    )


class FakeResponse:
    def __init__(self, status_code: int, payload: dict | None = None):
        self.status_code = status_code
        self._payload = payload or {}

    def json(self) -> dict:
        return self._payload


class FakeBackend:
    """Records calls; configurable token presence and per-route responses."""

    def __init__(self, *, token: bytes | None = b"stored-token-blob"):
        self._token = token
        self.put_calls: list[bytes] = []
        self.posts: list[tuple[str, dict]] = []
        self.puts: list[tuple[str, dict]] = []
        self.responses: dict[str, FakeResponse] = {}
        self.closed = False

    # context manager
    def __enter__(self) -> "FakeBackend":
        return self

    def __exit__(self, *_exc) -> None:
        self.closed = True

    def close(self) -> None:
        self.closed = True

    # token store
    def get_token(self) -> bytes:
        from garmin_bridge.backend import TokenNotFound

        if self._token is None:
            raise TokenNotFound()
        return self._token

    def put_token(self, blob: bytes) -> None:
        self.put_calls.append(blob)

    # capability writes
    def post_json(self, path: str, body: dict) -> FakeResponse:
        self.posts.append((path, body))
        return self.responses.get(path, FakeResponse(200, {"results": []}))

    def put_json(self, path: str, body: dict) -> FakeResponse:
        self.puts.append((path, body))
        return self.responses.get(path, FakeResponse(200, {"results": []}))
