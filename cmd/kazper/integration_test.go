//go:build integration

// Run with:  go test -tags=integration ./cmd/kazper/
//
// Builds the kazper binary and exercises the serve, migrate, and
// version subcommands against a testcontainers-managed Postgres.

package main_test

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "kazper")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/kazper")
	build.Dir = filepath.Join("..", "..")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)
	return binPath
}

func startPostgres(t *testing.T) string {
	t.Helper()
	if _, ok := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED"); !ok {
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("nutrition"),
		tcpostgres.WithUsername("nutrition"),
		tcpostgres.WithPassword("nutrition"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func TestVersionSubcommand(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "version").CombinedOutput()
	require.NoError(t, err, "version failed: %s", out)
	s := string(out)
	if !strings.Contains(s, "kazper") {
		t.Errorf("version output missing binary name: %q", s)
	}
	if !strings.Contains(s, "version=") {
		t.Errorf("version output missing version= field: %q", s)
	}
}

func TestMigrateSubcommand(t *testing.T) {
	bin := buildBinary(t)
	dsn := startPostgres(t)

	cmd := exec.Command(bin, "migrate")
	cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "migrate failed: %s", out)
	require.Contains(t, string(out), "migrations applied")

	// Idempotent re-run should also succeed (golang-migrate reports "no change").
	cmd2 := exec.Command(bin, "migrate")
	cmd2.Env = append(os.Environ(), "DATABASE_URL="+dsn)
	out2, err := cmd2.CombinedOutput()
	require.NoError(t, err, "second migrate failed: %s", out2)
}

func TestServeSubcommandHealthz(t *testing.T) {
	bin := buildBinary(t)
	dsn := startPostgres(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "serve", "--addr", ":18081")
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dsn,
		"MOBILE_API_TOKEN=mobile-token-aaaaaaaaaaaaaaaa",
		"AGENT_API_TOKEN=agent-token-bbbbbbbbbbbbbbbbb",
		"MIGRATE_ON_START=true",
		"DEFAULT_USER_TZ=UTC",
	)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	healthURL := "http://127.0.0.1:18081/healthz"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return // success
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("server never returned 200 on /healthz; stderr: %s", stderr.String())
}
