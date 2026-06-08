// Package storetest provides a testcontainers-backed Postgres pool with
// migrations already applied, for use by integration tests.
package storetest

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

var configOnce sync.Once

// configureRuntime disables the testcontainers Ryuk reaper. Ryuk requires a
// "bridge" network that Podman does not expose in the form testcontainers
// expects; t.Cleanup handles container teardown for normal test runs.
func configureRuntime() {
	configOnce.Do(func() {
		if _, ok := os.LookupEnv("TESTCONTAINERS_RYUK_DISABLED"); !ok {
			os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
		}
	})
}

// NewPool boots a fresh Postgres container, runs migrations, and returns a
// pgxpool.Pool wired to it. The container and pool are torn down via t.Cleanup.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	configureRuntime()
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
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get connection string")

	require.NoError(t, store.Migrate(dsn), "run migrations")

	pool, err := store.NewPool(ctx, dsn)
	require.NoError(t, err, "build pool")
	t.Cleanup(pool.Close)
	return pool
}
