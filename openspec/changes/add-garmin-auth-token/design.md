# Design: add-garmin-auth-token

## Context

This is the trunk of the Garmin integration (decided in an explore session). The
integration uses approach "B": a separate Python `garmin-bridge` service owns all
Garmin auth/fetch via the maintained `garth`/`garminconnect` libraries, so that
when Garmin breaks its private API the fix is a `pip upgrade`, not Go work. The
bridge runs as its own k8s Deployment + ClusterIP + CronJob.

The bridge has two planes: a **control plane** (interactive MFA login, rare —
~yearly) that mints a long-lived garth token, and a **data plane** (headless
daily sync) that reuses that token. For the bridge to be stateless across k8s
reschedules, the token must persist somewhere durable it can read each sync. We
chose Postgres, owned by this backend — keeping secrets in one store and the
bridge free of a PVC. This change is *only* that store + the bridge's identity;
it contains zero Garmin protocol knowledge.

## Goals / Non-Goals

**Goals:**
- A durable, encrypted-at-rest home for one opaque token blob.
- A dedicated, revocable identity (`GARMIN_API_TOKEN`) the bridge calls under.
- Opt-in: absent config means the feature is simply off, never a startup failure.
- Treat the blob as opaque — the backend stores and returns it, nothing more.

**Non-Goals:**
- No Garmin auth/fetch/mapping (that's `add-garmin-bridge`).
- No general secrets/KV store; no multi-token or rotation policy.
- No interpretation of the blob's structure (it's garth's internal format).

## Decisions

### D1: One opaque encrypted blob, single row

`garmin_tokens(id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1), ciphertext
BYTEA, nonce BYTEA, updated_at TIMESTAMPTZ)`. The single-row constraint encodes
"one user, one token". `PUT` upserts row 1; `GET` returns the decrypted blob;
`DELETE` removes it. The blob is whatever the bridge sends (garth's serialized
token) — the backend never parses it. **Alternative rejected:** a structured
schema mirroring garth's token fields — that couples the backend to garth's
internal format, which is exactly the churn we pushed into the Python side.

### D2: AES-256-GCM at rest, key from config

`GARMIN_TOKEN_ENC_KEY` is a 32-byte key (base64 in env). On `PUT` the backend
seals `plaintext` → `(ciphertext, nonce)`; on `GET` it opens. The key is config,
not in the DB — DB compromise alone does not yield the token. The plaintext blob
is never logged (the request/response logger must redact the body for these
routes, or they opt out of body logging). **Alternative rejected:** storing the
blob plaintext and relying on DB encryption — weaker (a DB dump leaks it) and the
project already treats tokens as secret-by-default.

### D3: `GARMIN_API_TOKEN` is an OPTIONAL third identity

The auth middleware gains a third comparison: a request bearing `GARMIN_API_TOKEN`
resolves to `client_id = "garmin"`. It mirrors `ANTHROPIC_API_KEY`'s opt-in
shape: when unset the integration is off. Startup validation: *if set*, it must
be ≥16 bytes and differ from the mobile and agent tokens; when unset, startup is
unaffected. **Alternative rejected:** reusing `AGENT_API_TOKEN` for the bridge —
loses independent revocation and muddies provenance (the bridge is not the agent).

### D4: The `/garmin/token` endpoints are garmin-identity-only

Only `client_id = "garmin"` may read/write/clear the token (the mobile and agent
identities get `403 forbidden`). This keeps the most sensitive blob in the system
reachable by exactly one caller. When `GARMIN_API_TOKEN` is unset, the routes
return `503 garmin_disabled` (there is no garmin identity to authorize). A small
per-route guard (middleware or handler check on the context `client_id`) enforces
this — the first capability in the repo to gate by identity, so the pattern is
worth doing cleanly.

### D5: Standard capability package, plus a crypto helper

`internal/garminauth/{types,repo,service,handlers}.go` per the template, with
`crypto.go` (AES-GCM seal/open over the config key). Repo is a trivial single-row
upsert/get/delete. Service validates presence of the enc key and seals/opens.

## Risks / Trade-offs

- [Enc key lost → token unrecoverable] → accepted; re-login via the bridge mints a
  fresh blob. The key is operational config (k8s Secret), backed up with the rest.
- [Blob format drift if garth changes its serialization] → none on the backend —
  it's opaque; the bridge owns the format. This is the point of D1.
- [Identity gate is a new pattern] → small surface (one capability); done once,
  cleanly, it's reusable if future endpoints need per-identity scoping.
- [Body logging could leak the plaintext] → mitigated by redacting/omitting the
  body on `/garmin/token` routes; called out as a task.

## Migration Plan

One migration: `garmin_tokens` single-row table. Down drops it. Additive; safe
rollback. No backfill (the bridge populates it on first login).

## Open Questions

- **Key rotation**: re-encrypting the blob under a new `GARMIN_TOKEN_ENC_KEY` is
  out of scope; if ever needed, a `DELETE` + re-login is the trivial path. Noted,
  not built.
