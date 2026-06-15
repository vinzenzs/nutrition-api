//go:build integration

// End-to-end exercise of the two recent changes THROUGH the MCP server:
//
//	go test -tags=integration -run TestMCPServer_E2E ./internal/mcpserver/
//
// Unlike TestMCPServer_AnnouncesEightTools (which stubs the REST API), this boots
// a real Postgres + `kazper serve`, then drives the real `kazper mcp` binary over
// stdio JSON-RPC, calling actual tools end to end:
//
//   - add-slot-duration-override: create template → plan → week → slot WITH a
//     duration_override → materialize → get_workout_program, asserting the
//     overridden duration reaches the resolved program and the materialized window.
//   - add-coach-methodology: create_phase WITH methodology → get_training_context,
//     asserting the covering phase's methodology rides the grounding bundle.
//   - schedule-adhoc-yoga-mobility: create_workout_template with sport "yoga" →
//     get_workout_template, asserting the widened sport vocabulary round-trips
//     through MCP → REST → DB (the bridge-backed schedule push is covered by
//     garmincontrol's stub-bridge tests, since this harness runs no bridge).
package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/vinzenzs/kazper/internal/store"
)

const e2eToken = "agent-token-e2e-aaaaaaaaaaaa"

func TestMCPServer_E2E(t *testing.T) {
	binPath := buildKazper(t)
	dsn := bootPostgres(t)
	apiURL := startServe(t, binPath, dsn)

	// Boot `kazper mcp` pointed at the live REST API.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, "mcp")
	cmd.Env = append(os.Environ(),
		"NUTRITION_API_URL="+apiURL,
		"AGENT_API_TOKEN="+e2eToken,
		"MCP_REQUEST_TIMEOUT_SECONDS=15",
	)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	r := bufio.NewReader(stdout)

	// initialize handshake.
	writeJSONRPC(t, stdin, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "e2e", "version": "0"},
		},
	})
	require.Contains(t, readJSONRPC(t, r), "result")
	writeJSONRPC(t, stdin, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})

	id := 1
	next := func() int { id++; return id }

	// ---- Change 1: per-slot duration override, end to end ----

	tmpl := callTool(t, stdin, r, next(), "create_workout_template", map[string]any{
		"sport": "run", "name": "Tempo", "estimated_duration_sec": 3600,
		"steps": []any{map[string]any{
			"type": "step", "intent": "active",
			"duration": map[string]any{"kind": "time", "seconds": 3600},
			"target":   map[string]any{"kind": "hr_zone", "low": 1, "high": 2},
		}},
	})
	templateID := tmpl["id"].(string)

	plan := callTool(t, stdin, r, next(), "create_training_plan", map[string]any{
		"name": "e2e-plan", "start_date": "2026-07-06", // a Monday
	})
	planID := plan["id"].(string)

	week := callTool(t, stdin, r, next(), "add_plan_week", map[string]any{
		"plan_id": planID, "ordinal": 1,
	})
	weekID := week["id"].(string)

	// Slot bumps the active block to 80 minutes (4800s) via a duration override.
	callTool(t, stdin, r, next(), "add_plan_slot", map[string]any{
		"plan_id": planID, "week_id": weekID, "weekday": 0, "ordinal": 0, "template_id": templateID,
		"duration_overrides": []any{map[string]any{
			"intent":   "active",
			"duration": map[string]any{"kind": "time", "seconds": 4800},
		}},
	})

	mat := callTool(t, stdin, r, next(), "materialize_training_plan", map[string]any{
		"plan_id": planID, "scope": "all",
	})
	workouts := mat["workouts"].([]any)
	require.Len(t, workouts, 1, "one slot → one planned workout")
	w := workouts[0].(map[string]any)
	workoutID := w["id"].(string)

	// The materialized window reflects the override: 80 minutes, not the template's 60.
	start, err := time.Parse(time.RFC3339, w["started_at"].(string))
	require.NoError(t, err)
	end, err := time.Parse(time.RFC3339, w["ended_at"].(string))
	require.NoError(t, err)
	assert.Equal(t, 80.0, end.Sub(start).Minutes(), "duration override moves the materialized window")

	// The resolved effective program shows the overridden step duration.
	prog := callTool(t, stdin, r, next(), "get_workout_program", map[string]any{"id": workoutID})
	steps := prog["steps"].([]any)
	require.Len(t, steps, 1)
	dur := steps[0].(map[string]any)["duration"].(map[string]any)
	assert.Equal(t, 4800.0, dur["seconds"], "effective program carries the overridden duration")

	// ---- Change 2: coach methodology, end to end ----

	phaseResp := callTool(t, stdin, r, next(), "create_phase", map[string]any{
		"name": "build", "type": "build",
		"start_date": "2026-07-01", "end_date": "2026-07-28",
		"methodology": "## Build\nPolarized per Seiler — 80/20.",
	})
	phase := phaseResp["phase"].(map[string]any)
	assert.Contains(t, phase["methodology"], "Seiler", "create_phase persists methodology")

	// The covering phase's methodology rides the grounding bundle.
	tc := callTool(t, stdin, r, next(), "get_training_context", map[string]any{"date": "2026-07-15"})
	ctxPhase := tc["phase"].(map[string]any)
	require.NotNil(t, ctxPhase["methodology"])
	assert.Contains(t, ctxPhase["methodology"], "Seiler",
		"the covering phase's methodology is surfaced in /context/training")

	// ---- Change 3: yoga sport round-trips through MCP → REST → DB ----
	// The full garmin_schedule_template push needs a Garmin bridge, which this
	// harness does not run (the bridge call is covered by garmincontrol's
	// stub-bridge tests). Here we verify the widened sport vocabulary end to end:
	// a yoga template created over MCP persists and reads back with its real sport.
	yoga := callTool(t, stdin, r, next(), "create_workout_template", map[string]any{
		"sport": "yoga", "name": "Recovery yoga",
		"steps": []any{map[string]any{
			"type": "step", "intent": "active",
			"duration": map[string]any{"kind": "time", "seconds": 1800},
			"target":   map[string]any{"kind": "none"},
		}},
	})
	yogaID := yoga["id"].(string)
	assert.Equal(t, "yoga", yoga["sport"], "create_workout_template persists the yoga sport")

	got := callTool(t, stdin, r, next(), "get_workout_template", map[string]any{"id": yogaID})
	assert.Equal(t, "yoga", got["sport"], "yoga sport reads back unchanged")

	_ = stdin.Close()
	_ = cmd.Wait()
}

// callTool issues one tools/call and returns the forwarded REST body parsed as
// JSON (the MCP server wraps it verbatim in the text content block).
func callTool(t *testing.T, w io.Writer, r *bufio.Reader, id int, name string, args map[string]any) map[string]any {
	t.Helper()
	writeJSONRPC(t, w, map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	var resp map[string]any
	for {
		resp = readJSONRPC(t, r)
		if resp["id"] == float64(id) {
			break
		}
	}
	result, ok := resp["result"].(map[string]any)
	require.Truef(t, ok, "tool %s: no result in %v", name, resp)
	content, ok := result["content"].([]any)
	require.Truef(t, ok && len(content) > 0, "tool %s: no content", name)
	text := content[0].(map[string]any)["text"].(string)
	var body map[string]any
	require.NoErrorf(t, json.Unmarshal([]byte(text), &body), "tool %s: body not JSON: %s", name, text)
	require.Falsef(t, result["isError"] == true, "tool %s returned error: %s", name, text)
	return body
}

func buildKazper(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "kazper")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/kazper")
	build.Dir = filepath.Join("..", "..")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)
	return binPath
}

func bootPostgres(t *testing.T) string {
	t.Helper()
	if _, ok := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED"); !ok {
		os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("nutrition"),
		tcpostgres.WithUsername("nutrition"),
		tcpostgres.WithPassword("nutrition"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres")
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, store.Migrate(dsn), "run migrations")
	return dsn
}

func startServe(t *testing.T, binPath, dsn string) string {
	t.Helper()
	// Grab a free port, then hand it to serve via HTTP_ADDR.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, "serve")
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dsn,
		"HTTP_ADDR="+addr,
		"MOBILE_API_TOKEN=mobile-token-e2e-bbbbbbbbbbbb",
		"AGENT_API_TOKEN="+e2eToken,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	url := "http://" + addr
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(url + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("kazper serve did not become healthy in time")
		}
		time.Sleep(200 * time.Millisecond)
	}
	return url
}
