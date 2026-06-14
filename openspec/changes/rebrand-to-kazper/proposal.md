## Why

The project name `kazper` describes a backend, not the product. The product is **Kazper** — an endurance-fueling and training coach that the app embodies. The in-app coach persona (shipped generically in `expand-chat-to-coach` phase 3) should *be* Kazper: one identity shared by the product and the assistant, matching the direction that arc already set (one coach, embodied by the app). This change rebrands the user-facing identity to Kazper without disturbing the codebase's internal plumbing.

**The name.** Kazper (Casper → Kazper) reads as a *name*, not a category: trademark-clean, no mattress-brand collision, ownable via the K/Z spelling. It carries two intended readings — **Casper**, the friendly, ever-present companion (the tone: warm, ambient, non-judgmental), and **Kaspar the Magus**, the wise guide who follows a star bearing gifts (the arc: guiding the athlete over a months-long journey to a destination race).

## What Changes

- **Scope: full rename.** Mid-apply the user renamed the GitHub repo to `kazper`, which reopened the original "keep module path" call — keeping it would split the Go import path (`nutrition-api`) from the repo URL (`kazper`). Decision flipped to a **full rename**: the Go module path `github.com/vinzenzs/nutrition-api` → `github.com/vinzenzs/kazper` (186 import sites + `go.mod`), verified with `go build ./...` + `go vet`.
- **The AI coach is named Kazper.** `internal/chat`'s system prompt names the assistant Kazper (a follow-on edit to the persona shipped in `expand-chat-to-coach` phase 3).
- **Binary + entrypoint:** `bin/nutrition-api` → `bin/kazper`; `cmd/nutrition-api/` → `cmd/kazper/`; Taskfile `build`/`install` targets and the installed `~/.local/bin/` name.
- **API + MCP identity:** the REST API swag `@title` → "Kazper" (`docs/` regenerated); the MCP server announced name `"nutrition"` → `"kazper"` (visible in the agent's server list); the Open Food Facts User-Agent and `kazper version` output.
- **Deploy:** Helm chart `deploy/helm/nutrition-api/` → `deploy/helm/kazper/` + `Chart.yaml`/template namespace + image name (a redeploy under the new release name, not an in-place upgrade — see design).
- **Companion + satellites:** the Flutter app display name (iOS `CFBundleDisplayName`, Android `android:label` → Kazper) and the `cookidoo` extension name; `garmin-bridge` product strings.
- **Docs:** README, RUN_LOCAL, CLAUDE.md, and other prose referring to the old name.
- **Explicitly kept:** the Postgres database name + `nutrition-pg` container, env var names (`NUTRITION_API_URL`, `CHAT_*`, `ANTHROPIC_API_KEY`, …), domain terms (`nutrition_goals`, `nutrition_per_serving`), the OpenSpec capability spec directory names (`nutrition-chat`, …), and the Flutter **Dart package id** `nutrition_companion` (a separate Flutter-build-gated rename — see design).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `nutrition-chat`: the assistant identifies itself as **Kazper** (a named persona). Additive — layers on top of the coaching persona from `expand-chat-to-coach`.
- `mcp-server`: the server's announced identity is **kazper** (and the REST API title is Kazper) rather than "nutrition".

## Impact

- **Code:** `cmd/` dir rename (main package — no importers); `internal/chat/prompt.go` (coach name); swag `@title` + `task swag` regen; `internal/mcpserver/server.go` server name. No module-path or import churn.
- **Build/deploy:** `Taskfile.yml` targets, `Dockerfile`, Helm chart dir + `Chart.yaml`/templates, GitHub workflows (`pr.yml`, `main.yml`, `release.yml`) referencing the binary/image/chart names.
- **Satellites:** `apps/companion/` (pubspec name + Android app label), `apps/garmin-bridge/`, `extensions/cookidoo/manifest.json`.
- **Outward-facing (needs the user / `gh`):** GitHub repo rename and the local working-dir rename are **out of scope for this change** — flagged for the user to do separately, since they break clone URLs and tooling paths.
- **Sequencing:** lands **after** `expand-chat-to-coach` and `unify-mcp-tool-registry` archive, so the coach persona and the agenttools/MCP surface are stable before the Kazper naming layers on (avoids rewriting in-flight code/proposals).
