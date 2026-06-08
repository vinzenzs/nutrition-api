## Why

The REST API now stores meals and answers daily/range summary questions, but the LLM coaching agent — one of the two clients the platform was designed for — has no way to reach it yet. MCP is the right transport: it's how Claude Desktop, Claude Code, and other agent runtimes call into external systems, and the REST API was deliberately shaped to be agent-friendly (actionable error bodies, idempotency keys, freeform write path). All that's missing is a small Go process that exposes those endpoints as MCP tools the agent can call.

## What Changes

- Add a new Go binary at `cmd/mcp/` that runs an MCP server over stdio. It is a thin wrapper that translates MCP tool calls into HTTP calls against the existing REST API, using `AGENT_API_TOKEN` from env.
- Expose the following MCP tools (one per agent-relevant REST endpoint), with input schemas derived from the REST request bodies and output shapes that are stable JSON:
  - `lookup_product_by_barcode` → `POST /products/lookup/{barcode}` (cached lookup, including the `?refresh=true` option)
  - `search_products` → `GET /products/search?q=…` (recall by name/brand, ranked by recency of use)
  - `log_meal` → `POST /meals` (when the agent already has a product_id, e.g. after a successful lookup)
  - `log_meal_freeform` → `POST /meals/freeform` (the agent's primary write path — name + nutriment estimate + quantity)
  - `patch_meal` → `PATCH /meals/{id}` (corrections)
  - `delete_meal` → `DELETE /meals/{id}`
  - `daily_summary` → `GET /summary/daily?date=…&tz=…`
  - `range_summary` → `GET /summary/range?from=…&to=…&tz=…`
- Idempotency for write tools: each write tool accepts an optional `idempotency_key`. When omitted, the wrapper derives one deterministically from the tool inputs (SHA-256 of the canonical JSON of args) so the agent gets idempotent retries even when it forgets to supply a key.
- Error mapping: REST 4xx/5xx responses are surfaced as MCP tool results with `isError=true` and a JSON body that preserves the structured shape (`{"error":"...","next":"..."}`) so the agent can reason about what to do next, rather than seeing an opaque MCP protocol error.
- Configuration via env: `NUTRITION_API_URL` (default `http://localhost:8080`), `AGENT_API_TOKEN` (required), `MCP_REQUEST_TIMEOUT_SECONDS` (default 10).
- Update the README with: what the MCP server is, how to register it with Claude Desktop and Claude Code (sample `claude_desktop_config.json` and `mcp.json` entries), and how it interacts with the REST API.
- Add a small `mcp-build` Makefile target and an `mcp-run` target that runs the binary on stdio for local sanity-checks.

## Capabilities

### New Capabilities
- `mcp-server`: An MCP server that exposes the REST API's agent-relevant endpoints as MCP tools, with idempotent writes, agent-shaped errors, and stdio transport.

### Modified Capabilities
<!-- None. The REST API contracts captured in `openspec/specs/{auth,meals,products,off-integration}/spec.md` are unchanged. -->

## Impact

- **New code**: `cmd/mcp/` (entry point + tool definitions + REST client). Optionally a small shared package under `internal/apiclient/` if the REST client grows beyond the cmd/mcp package's needs.
- **New dependency**: a Go MCP SDK. The proposal picks `github.com/modelcontextprotocol/go-sdk` (the official SDK as of late 2025); the design document captures the fallback to `github.com/mark3labs/mcp-go` if integration friction shows up during implementation.
- **New configuration (env)**: `NUTRITION_API_URL`, `AGENT_API_TOKEN`, `MCP_REQUEST_TIMEOUT_SECONDS`.
- **External systems**: the agent runtime (Claude Desktop, Claude Code, or any MCP-aware client) launches the binary as a subprocess. No public-network exposure.
- **No REST API changes**: the existing endpoints, error bodies, and idempotency contract are sufficient. The MCP wrapper does not require new endpoints.
- **Out of scope (later changes)**:
  - HTTP / SSE transport for remote agents — stdio is enough for personal use today.
  - Resources or prompts exposed via MCP (e.g. a "coaching system prompt" resource). Tools only in v1.
  - In-process direct service calls (skipping HTTP). The thin-wrapper HTTP boundary is intentional and worth keeping for testability.
  - A trends / coaching API surface. Daily and range summaries are enough for the agent to reason over.
