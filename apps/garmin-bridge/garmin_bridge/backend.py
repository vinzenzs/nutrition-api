"""HTTP client to the nutrition REST API.

The bridge is an ordinary authenticated client of the backend: it stores and
reads the garth token blob (``/garmin/token``) and POSTs mapped day data to the
capability endpoints, all under the ``garmin`` bearer identity. No DB coupling —
the backend's own validation and date/external_id upserts give idempotency for
free (see design D2/D5).
"""

from __future__ import annotations

import logging

import httpx

logger = logging.getLogger("garmin_bridge.backend")

# The blob is opaque bytes on the wire; the backend stores and returns it
# byte-identical (octet-stream), so we never JSON-wrap it.
_OCTET = "application/octet-stream"


class TokenNotFound(Exception):
    """The backend has no stored Garmin token — a login is required."""


class BackendError(Exception):
    """A backend call failed in a way that should surface to the caller."""


class Backend:
    """Thin wrapper over the nutrition REST API for the bridge's needs."""

    def __init__(self, base_url: str, token: str, *, timeout: float = 30.0):
        self._client = httpx.Client(
            base_url=base_url,
            headers={"Authorization": f"Bearer {token}"},
            timeout=timeout,
        )

    def close(self) -> None:
        self._client.close()

    def __enter__(self) -> "Backend":
        return self

    def __exit__(self, *_exc) -> None:
        self.close()

    # --- token store -----------------------------------------------------

    def get_token(self) -> bytes:
        """Read the stored garth token blob. Raises TokenNotFound on 404."""
        resp = self._client.get("/garmin/token")
        if resp.status_code == 404:
            raise TokenNotFound()
        if resp.status_code != 200:
            raise BackendError(f"GET /garmin/token -> {resp.status_code}")
        return resp.content

    def put_token(self, blob: bytes) -> None:
        """Persist the garth token blob (full-replace; no Idempotency-Key)."""
        resp = self._client.put(
            "/garmin/token", content=blob, headers={"Content-Type": _OCTET}
        )
        if resp.status_code not in (200, 204):
            raise BackendError(f"PUT /garmin/token -> {resp.status_code}")

    # --- capability writes ----------------------------------------------

    def post_json(self, path: str, body: dict) -> httpx.Response:
        """POST a JSON body; returns the response for the caller to inspect."""
        return self._client.post(path, json=body)
