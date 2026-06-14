//go:build goldengen

// One-shot generator for the announced-schema golden baseline.
//
//	go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/
//
// It builds the binary, boots `nutrition-api mcp`, calls tools/list, and writes
// every tool's announced inputSchema to testdata/announced_schemas.json. Run it
// ONCE, from the pre-port surface, to freeze the baseline; the committed golden
// test (schema_golden_test.go) then asserts each ported tool's reflected schema
// still matches this baseline byte-for-byte. Do NOT regenerate it after porting
// a tool — that would defeat the drift check.
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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaptureAnnouncedSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "nutrition-api")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/nutrition-api")
	build.Dir = filepath.Join("..", "..")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer stub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "mcp")
	cmd.Env = append(os.Environ(),
		"NUTRITION_API_URL="+stub.URL,
		"AGENT_API_TOKEN=goldengen-token",
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
	wr := func(msg map[string]any) {
		b, _ := json.Marshal(msg)
		_, err := stdin.Write(append(b, '\n'))
		require.NoError(t, err)
	}
	rd := func() map[string]any {
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var m map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &m))
			if _, hasID := m["id"]; hasID {
				return m
			}
		}
	}

	wr(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "goldengen", "version": "0"},
	}})
	_ = rd()
	wr(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	wr(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	listResp := rd()

	result := listResp["result"].(map[string]any)
	tools := result["tools"].([]any)
	schemas := map[string]json.RawMessage{}
	for _, tool := range tools {
		obj := tool.(map[string]any)
		name, _ := obj["name"].(string)
		raw, err := json.Marshal(obj["inputSchema"])
		require.NoError(t, err)
		schemas[name] = raw
	}
	require.GreaterOrEqual(t, len(schemas), 120, "expected the full tool surface")

	// Stable, pretty output keyed by sorted tool name.
	names := make([]string, 0, len(schemas))
	for n := range schemas {
		names = append(names, n)
	}
	sort.Strings(names)
	ordered := make(map[string]json.RawMessage, len(schemas))
	_ = ordered
	var buf strings.Builder
	buf.WriteString("{\n")
	for i, n := range names {
		nb, _ := json.Marshal(n)
		buf.Write(nb)
		buf.WriteString(": ")
		buf.Write(schemas[n])
		if i < len(names)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}\n")

	require.NoError(t, os.MkdirAll("testdata", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("testdata", "announced_schemas.json"), []byte(buf.String()), 0o644))

	_ = stdin.Close()
	_ = cmd.Wait()
}
