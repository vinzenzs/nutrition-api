# Design: add-garmin-mcp-login

## Context

The bridge's MFA login (from `add-garmin-bridge`) is interactive and rare. We want
to trigger it from the companion chat so re-auth doesn't mean a `kubectl exec`.
The MCP server is a REST-API client (often running off-cluster, e.g. Claude
Desktop), so it can't reach the bridge's internal ClusterIP — the call must go
through the backend, which is in-cluster. This change is the thin glue: backend
proxy endpoints + two MCP tools.

## Goals / Non-Goals

**Goals:**
- Re-login Garmin from the agent: start the flow, then submit the MFA code.
- Keep the "one MCP tool = one HTTP call to the REST API" invariant.
- Never let the Garmin password near the agent or logs — only the ephemeral code.

**Non-Goals:**
- No Garmin/token logic in the backend (the proxy forwards bytes).
- No companion-app UI (same endpoints, later). No new auth identity.

## Decisions

### D1: Backend proxies because MCP can't reach the bridge

The MCP server talks to `NUTRITION_API_URL`. The bridge is a ClusterIP the backend
can reach but the MCP client generally cannot. So `POST /garmin/login` and
`POST /garmin/login/mfa` forward to `GARMIN_BRIDGE_URL` and return the bridge's
status + body verbatim. **Alternative rejected:** pointing the MCP client at the
bridge directly — breaks the single-endpoint model, needs bridge auth/exposure,
and fails when MCP runs off-cluster.

### D2: Forward verbatim; the backend interprets nothing

The proxy copies the request body to the bridge and the bridge's response back —
status code and JSON unchanged (`{needs_mfa: true}`, errors, success). It adds no
fields and parses nothing. This keeps all login semantics in one place (the
bridge) and makes the backend immune to bridge-side changes.

### D3: Password stays in the bridge; only the MFA code transits the agent

`POST /garmin/login` takes **no credentials** — the bridge reads them from its
Secret. The agent calls it with an empty body; the bridge starts SSO and reports
`needs_mfa`. The user reads the 6-digit code from their authenticator and gives it
to the agent, which calls `garmin_submit_mfa(code)`. So the only secret on the
chat/LLM path is a throwaway 6-digit code — never the password, never the token.

### D4: Off when unconfigured

When `GARMIN_BRIDGE_URL` is unset, the proxy endpoints return `503
garmin_disabled` and the MCP tools surface that as an error — the feature is
opt-in, consistent with the rest of the Garmin stack.

### D5: Who may call it

The endpoints require authentication; they're meant for the human-driven
identities (agent via MCP, later mobile via the app). They are not restricted to
the `garmin` identity (that identity is the *bridge*, which is the login target,
not the trigger). No new identity is introduced.

## Risks / Trade-offs

- [A login could be triggered by anyone with the agent token] → acceptable
  single-user; the bridge still requires the correct MFA code to complete, and the
  password never leaves the bridge.
- [Bridge unreachable / down] → the proxy surfaces the upstream error
  (`502/504`-style) verbatim; the agent relays it. No retry magic.
- [MFA code in chat history] → it's single-use and expires in ~30s; low residual
  value. The password — the durable secret — is never there.

## Migration Plan

No DB change. Additive endpoints + tools + one config var. Deploy after
`add-garmin-bridge` (the `/login` it proxies must exist) and
`add-garmin-auth-token`. Rollback: unset `GARMIN_BRIDGE_URL` (endpoints go 503) or
remove the routes/tools.

## Open Questions

- **Timeout posture** — the bridge's SSO call can take a few seconds; the proxy
  needs a generous-but-bounded timeout. Decide the exact value at implementation.
