package idempotency

import (
	"context"
	"log/slog"
	"time"
)

// RunCleanup runs an immediate purge of expired records, then re-runs
// every `interval` until ctx is done. Errors are logged but do not stop
// the loop.
func RunCleanup(ctx context.Context, repo *Repo, ttl, interval time.Duration, logger *slog.Logger) {
	purge := func() {
		n, err := repo.DeleteExpired(ctx, ttl)
		if err != nil {
			logger.Warn("idempotency cleanup failed", "err", err)
			return
		}
		if n > 0 {
			logger.Info("idempotency cleanup", "removed", n)
		}
	}

	purge()

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			purge()
		}
	}
}
