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
	for _, want := range AnnouncedToolNames {
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
