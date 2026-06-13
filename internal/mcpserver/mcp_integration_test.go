//go:build integration

// Run with:  go test -tags=integration ./internal/mcpserver/
//
// Builds the nutrition-api binary, spawns `nutrition-api mcp` as a
// subprocess with a stub REST server for healthz, exchanges JSON-RPC
// frames over stdio, and asserts that the eight expected tools are
// announced via tools/list.

package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_AnnouncesEightTools(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "nutrition-api")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/nutrition-api")
	// Build from the repo root so the package path resolves.
	build.Dir = filepath.Join("..", "..")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	// Stub the REST API so the startup healthz smoke check succeeds.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer stub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "mcp")
	cmd.Env = append(os.Environ(),
		"NUTRITION_API_URL="+stub.URL,
		"AGENT_API_TOKEN=integration-test-token",
		"MCP_REQUEST_TIMEOUT_SECONDS=5",
	)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	reader := bufio.NewReader(stdout)
	writeJSONRPC(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "integration-test", "version": "0"},
		},
	})
	initResp := readJSONRPC(t, reader)
	assert.Equal(t, float64(1), initResp["id"])
	assert.Contains(t, initResp, "result", "initialize should produce a result")

	// MCP requires sending the initialized notification after initialize.
	writeJSONRPC(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	writeJSONRPC(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	listResp := readJSONRPC(t, reader)
	assert.Equal(t, float64(2), listResp["id"])
	result, ok := listResp["result"].(map[string]any)
	require.True(t, ok, "tools/list response missing result")
	tools, ok := result["tools"].([]any)
	require.True(t, ok, "tools/list result missing tools array")

	names := map[string]bool{}
	for _, tool := range tools {
		obj, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := obj["name"].(string); name != "" {
			names[name] = true
		}
	}
	for _, want := range []string{
		"lookup_product_by_barcode",
		"search_products",
		"log_meal",
		"log_meal_freeform",
		"patch_meal",
		"delete_meal",
		"log_meal_from_photo",
		"create_planned_meal",
		"list_planned_meals",
		"update_planned_meal",
		"delete_planned_meal",
		"mark_planned_meal_eaten",
		"add_shopping_items",
		"list_shopping_items",
		"update_shopping_item",
		"delete_shopping_item",
		"clear_checked_shopping_items",
		"daily_summary",
		"range_summary",
		"list_products",
		"delete_product",
		"import_cookidoo_recipe",
		"log_hydration",
		"list_hydration",
		"patch_hydration",
		"delete_hydration",
		"daily_hydration_summary",
		"set_daily_goal_override",
		"get_daily_goal_override",
		"delete_daily_goal_override",
		"list_daily_goal_overrides",
		"plan_carb_load",
		"recommend_workout_fuel",
		"create_race",
		"list_races",
		"get_race",
		"update_race",
		"delete_race",
		"plan_race_fueling",
		"log_workout",
		"list_workouts",
		"get_workout",
		"patch_workout",
		"delete_workout",
		"fulfill_workout",
		"unfulfill_workout",
		"create_workout_template",
		"list_workout_templates",
		"get_workout_template",
		"patch_workout_template",
		"delete_workout_template",
		"create_training_plan",
		"list_training_plans",
		"get_training_plan",
		"patch_training_plan",
		"delete_training_plan",
		"add_plan_week",
		"patch_plan_week",
		"delete_plan_week",
		"add_plan_slot",
		"patch_plan_slot",
		"delete_plan_slot",
		"materialize_training_plan",
		"get_workout_program",
		"log_weight",
		"list_weights",
		"patch_weight",
		"delete_weight",
		"weight_trend",
		"log_recovery_metrics",
		"list_recovery_metrics",
		"get_recovery_metrics",
		"delete_recovery_metrics",
		"log_fitness_metrics",
		"list_fitness_metrics",
		"get_fitness_metrics",
		"delete_fitness_metrics",
		"log_hydration_balance",
		"list_hydration_balance",
		"get_hydration_balance",
		"delete_hydration_balance",
		"daily_summary_get",
		"gear_list",
		"personal_records_list",
		"athlete_config_get",
		"workout_fueling_summary",
		"log_workout_fuel",
		"list_workout_fuel",
		"patch_workout_fuel",
		"delete_workout_fuel",
		"weekly_energy_summary",
		"rolling_summary",
		"protein_distribution",
		"create_phase",
		"list_phases",
		"get_phase",
		"update_phase",
		"delete_phase",
		"set_goal_template",
		"list_goal_templates",
		"get_goal_template",
		"delete_goal_template",
		"daily_context",
		"garmin_login",
		"garmin_submit_mfa",
		"garmin_schedule_workout",
		"garmin_unschedule_workout",
		"garmin_schedule_plan",
		"garmin_list_scheduled",
	} {
		assert.True(t, names[want], "tool %q should be announced", want)
	}

	_ = stdin.Close()
	_ = cmd.Wait()
}

func writeJSONRPC(t *testing.T, w io.Writer, msg map[string]any) {
	t.Helper()
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	b = append(b, '\n')
	_, err = w.Write(b)
	require.NoError(t, err)
}

func readJSONRPC(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &msg))
		// Skip notifications (no id); we want responses.
		if _, hasID := msg["id"]; hasID {
			return msg
		}
	}
}
