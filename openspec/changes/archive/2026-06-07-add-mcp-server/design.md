## Context

The REST API exists, is tested, and is already shaped for two clients (mobile + agent). The mobile app drives barcode-scan flows; the agent drives "I had a banana" freeform writes and "how did I do this week?" reads. This change implements the agent's mouth — an MCP server — without changing any REST contracts.

Three things shape this design:

1. **MCP is a tools-over-stdio protocol** for the common case (Claude Desktop, Claude Code, any local agent runtime spawning a subprocess and talking JSON-RPC over stdin/stdout). That's the entire universe we need to cover for "personal use, one user, agent runs on the same Mac as the REST API."
2. **The REST API is already agent-friendly.** Errors carry `next` hints. Writes accept `Idempotency-Key`. Reads return stable JSON. The MCP layer should preserve all of this, not re-invent it.
3. **The wrapper should be boring.** No business logic, no caching, no schema translation beyond what MCP requires. Every tool is a tiny HTTP-call-and-passthrough function. The interesting design problems are at the edges: error mapping, idempotency derivation, schema shapes for the LLM, and runtime ergonomics for the user registering the server.

## Goals / Non-Goals

**Goals:**

- Expose every REST endpoint that the agent actually uses as an MCP tool.
- Preserve the REST API's actionable error shapes inside MCP tool results so the agent can reason about what to do next.
- Make write tools idempotent by default, even when the agent does not explicitly supply an idempotency key.
- Keep the binary small enough to register, debug, and re-launch quickly during development.
- Ship a single binary (or Makefile-driven workflow) the user can copy-paste into Claude Desktop's `claude_desktop_config.json` and Claude Code's `mcp.json` without thinking.

**Non-Goals:**

- HTTP / SSE / WebSocket transports — stdio only. Adding remote transport later is a small follow-up; doing it now would force decisions (auth, TLS, multi-tenant) that don't exist for a personal app.
- MCP resources, prompts, or sampling APIs. v1 is tools-only.
- In-process direct service calls (skipping the HTTP layer). Tempting for "fewer hops" but the HTTP boundary keeps the wrapper genuinely thin, lets `cmd/mcp/` be deployed separately, and forces us to dogfood our own REST error contracts.
- Persistent state in the MCP server. It is stateless and idempotent across restarts.
- Concurrent multi-client behaviour. One agent runtime, one subprocess.

## Decisions

### 1. Stdio transport, single binary at `cmd/mcp/`

Layout:

```
cmd/
├── api/         existing REST server
└── mcp/         new MCP server (stdio)
    ├── main.go            entry, env, signal handling
    ├── server.go          tool registration with the MCP SDK
    ├── tools_products.go  lookup_product_by_barcode, search_products
    ├── tools_meals.go     log_meal, log_meal_freeform, patch_meal, delete_meal
    ├── tools_summary.go   daily_summary, range_summary
    ├── apiclient.go       thin HTTP client targeting NUTRITION_API_URL
    └── errors.go          REST-error → MCP-tool-result helper
```

Stdio means the agent runtime spawns the binary and pipes JSON-RPC frames. The binary takes no flags, reads config from env, and exits cleanly on stdin EOF or SIGTERM.

**Alternatives considered:**
- *HTTP/SSE transport.* Right for a remote agent; overkill for personal use. The Go MCP SDK supports both — switching later is small.
- *Subcommand on the existing binary (`nutrition-api mcp`).* Reduces deployment artifacts but couples lifecycles. Two separate binaries keeps the REST server's startup story unchanged.

### 2. SDK choice: `github.com/modelcontextprotocol/go-sdk`

The official Go SDK is the default. It is the most likely to track protocol updates, and its tool-registration API is straightforward (typed input struct, return value or error, the SDK handles JSON-schema generation from struct tags).

**Fallback:** if the official SDK has integration friction during implementation (rare APIs missing, awkward stdio plumbing on darwin/arm64, etc.), drop to `github.com/mark3labs/mcp-go`. The tool-by-tool registration is similar; only the imports change. Captured here so the implementing change does not stall on an SDK choice.

### 3. HTTP-call passthrough — one tool, one REST endpoint

Every tool's implementation is:

```
1. Read tool input (typed Go struct, SDK-decoded from JSON).
2. Build the corresponding REST request.
3. Set Authorization: Bearer <AGENT_API_TOKEN>.
4. Set Idempotency-Key for write tools (see Decision 4).
5. Read the REST response body.
6. If 2xx: return the raw body as the tool result content
   (MCP tool result contains a single text item with JSON).
7. If 4xx/5xx: return a tool result with isError=true and the
   same JSON body (preserving the REST API's structured error
   shape).
```

The wrapper does NOT re-serialize REST responses into MCP-specific shapes. The agent sees the same JSON the REST API would return to curl — which keeps the contract single-source-of-truth in `openspec/specs/{products,meals,off-integration,auth}/spec.md` and avoids drift.

**Alternative considered:** *Translate REST responses into MCP-typed structured tool outputs (the SDK supports structured outputs in addition to text).* Rejected for v1 because it doubles the contract surface — every change to a REST response now needs a matching change to the MCP schema. The agent reads the JSON either way.

### 4. Idempotency keys derived deterministically when not supplied

Every write tool (`log_meal`, `log_meal_freeform`, `patch_meal`, `delete_meal`) accepts an optional `idempotency_key` field on its input schema. If the agent supplies one, it is used as the `Idempotency-Key` header. If it does not, the wrapper derives one as:

```
idempotency_key = sha256_hex(tool_name + "|" + canonical_json(args))
```

`canonical_json` sorts object keys. The derived key is stable across retries with the same arguments — which is exactly what the REST API's idempotency layer needs. It also means: an agent that calls `log_meal_freeform` with identical args twice in a row gets the same meal id back, not a double-log.

This shifts a real failure mode: if the agent intentionally wants to log the same meal twice (e.g., the user really did eat the same banana twice), it must pass a distinct `idempotency_key` (or a tiny note in `note` field that changes the canonical JSON). The tool's input schema documents this so the agent learns it from the schema description.

**Alternative considered:** *Skip auto-derivation; let the agent forget and double-log.* Rejected: in practice, agents forget. The cost of "two identical bananas requires a distinct key" is much smaller than the cost of accidental double-logs.

### 5. Error mapping preserves the REST `next` hint

The REST API already returns:

```json
{"error":"product_not_found","barcode":"…","next":"POST /meals/freeform"}
```

When the wrapper sees a non-2xx response, it returns the body verbatim inside an MCP tool result with `isError=true`. The agent receives the structured error in its tool-output content. We do not rewrite `"POST /meals/freeform"` into `"log_meal_freeform"` (the MCP tool name) — that translation is the agent's job, and is well within an LLM's competence given a good tool description. Keeping the REST error body literal means the same error contract serves curl, mobile, and the agent.

For non-HTTP errors (network failure, timeout reaching the REST server), the wrapper returns a tool result with `isError=true` and body `{"error":"transport","detail":"..."}` so the agent does not see a bare network error.

### 6. Tool input/output schemas

Each tool input is a Go struct with JSON tags. The SDK generates the JSON Schema the agent sees. Conventions:

- Required fields are non-pointer types; optional fields are pointers.
- Field descriptions live in struct tags (`description:"…"` or similar — SDK-dependent). The description is the agent's main interface, so it carries the contract: units (grams, RFC 3339, IANA tz), validation rules ("must be > 0"), and pointers to related tools ("if lookup_product_by_barcode returns product_not_found, call log_meal_freeform instead").
- Output is the raw REST JSON. The tool description tells the agent the response shape it can expect (effectively a one-line summary plus a link to the spec file).

The eight tools and their inputs:

| Tool                         | Input fields                                                                                                                                    |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `lookup_product_by_barcode`  | `barcode` (required, string), `refresh` (optional bool)                                                                                         |
| `search_products`            | `q` (required, string)                                                                                                                          |
| `log_meal`                   | `product_id` (required, uuid), `quantity_g` (required, >0), `logged_at` (required, RFC 3339), `meal_type` (optional), `note` (optional), `idempotency_key` (optional) |
| `log_meal_freeform`          | `name` (required), `nutriments_per_100g` (object with optional kcal/protein_g/carbs_g/fat_g/fiber_g/sugar_g/salt_g), `quantity_g` (required), `logged_at` (required), `meal_type` (optional), `note` (optional), `save_as_product` (optional bool), `idempotency_key` (optional) |
| `patch_meal`                 | `meal_id` (required, uuid), `quantity_g` (optional), `logged_at` (optional), `meal_type` (optional), `note` (optional), `idempotency_key` (optional) |
| `delete_meal`                | `meal_id` (required, uuid), `idempotency_key` (optional)                                                                                        |
| `daily_summary`              | `date` (required, YYYY-MM-DD), `tz` (optional, IANA — defaults to server-side `DEFAULT_USER_TZ`)                                                |
| `range_summary`              | `from` (required, YYYY-MM-DD), `to` (required, YYYY-MM-DD), `tz` (optional)                                                                     |

### 7. Configuration

| Variable                       | Default                  | Purpose                                                            |
|--------------------------------|--------------------------|--------------------------------------------------------------------|
| `NUTRITION_API_URL`            | `http://localhost:8080`  | Where the REST API lives.                                          |
| `AGENT_API_TOKEN`              | _required_               | Bearer token for the REST API (same env var the REST server reads).|
| `MCP_REQUEST_TIMEOUT_SECONDS`  | `10`                     | Per-tool HTTP timeout. Slightly larger than the REST API's OFF timeout to absorb cold-cache lookups. |

The MCP binary refuses to start if `AGENT_API_TOKEN` is empty.

### 8. Registration ergonomics

The README will add a section showing exactly:

```json
// claude_desktop_config.json
{
  "mcpServers": {
    "nutrition": {
      "command": "/absolute/path/to/nutrition-mcp",
      "env": {
        "NUTRITION_API_URL": "http://localhost:8080",
        "AGENT_API_TOKEN": "<token>"
      }
    }
  }
}
```

and the equivalent `mcp.json` for Claude Code. The Makefile target `make mcp-install` will copy the binary to `~/.local/bin/nutrition-mcp` so the absolute path is stable across rebuilds.

### 9. Testing

- **Unit tests** for the REST client (request shape, header injection, error decoding) against `httptest.Server` stubs.
- **Unit tests** for each tool: stub the API client, drive the tool function with sample input, assert on the MCP tool result shape (content + isError).
- **An optional MCP protocol-level integration test** that spawns the binary as a subprocess and exchanges a JSON-RPC tool call over stdio. Marked `//go:build integration` so it only runs when explicitly requested — no value in running it on every CI loop, and it would tie test runtime to whichever MCP SDK we picked.

## Risks / Trade-offs

- **MCP SDK churn.** The Go SDK is younger than the TypeScript/Python ones. *Mitigation:* the tool functions are tiny and SDK-isolated to `server.go`; a swap to `mark3labs/mcp-go` is well-scoped.
- **Auto-derived idempotency keys hide real intent.** If the agent genuinely wants to double-log, it must change the args. *Mitigation:* document this on every write tool's description; the agent can vary `note` or pass `idempotency_key` when it means it.
- **Stale REST URL in the agent config.** Users running the REST API on a non-default port will see "transport" errors. *Mitigation:* startup smoke check — the binary does a `GET /healthz` once before announcing tool readiness, and logs a clear error if it fails (visible in Claude Desktop's MCP log).
- **No HTTP transport yet.** A user who wants to call the API from a remote agent will hit a wall. *Mitigation:* explicit non-goal; HTTP transport is a small follow-up change once we see the actual need.
- **Two binaries to keep in sync.** The MCP wrapper depends on REST response shapes. *Mitigation:* the wrapper is read-mostly and contract changes already require spec updates; mismatches will surface in unit tests via the typed schema descriptions.

## Migration Plan

- Greenfield component; no migrations.
- Rollback strategy: do not register the MCP server (or remove the entry from `claude_desktop_config.json`). The REST API and mobile path are unaffected.

## Open Questions

- Whether to surface a single MCP resource — `coaching_context` — that exposes the current day's summary as a resource the agent can subscribe to. Useful for "the agent gets passive context without spending a tool call." Deferred to a follow-up change; v1 is tools-only.
- Whether the agent should be able to call `lookup_product_by_barcode` with `refresh=true` autonomously. The risk is that the agent burns OFF requests gratuitously. *Tentative answer:* yes, with the tool description making clear that refresh is for fixing known-stale data, not the default.
