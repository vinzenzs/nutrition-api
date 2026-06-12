"""garmin-bridge — Garmin Connect → nutrition REST API.

A small Python service that owns all Garmin auth and fetch (the churning,
frequently-broken private-API surface) and maps a day's data onto the existing
nutrition REST endpoints. The Go backend stays Garmin-agnostic; when Garmin
breaks its API the fix is a ``pip upgrade``, not Go work. See README.md.
"""

__version__ = "0.1.0"
