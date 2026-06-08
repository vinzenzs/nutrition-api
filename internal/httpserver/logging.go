package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
)

// requestLogger logs one JSON line per request with client_id, status,
// latency, route, and a hashed idempotency key (never the raw key).
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"client_id", string(auth.ClientFromContext(c)),
		}
		if k := c.GetHeader(idempotency.HeaderName); k != "" {
			attrs = append(attrs, "idempotency_key_sha256", hashKey(k))
		}
		logger.Info("request", attrs...)
	}
}

func hashKey(k string) string {
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:8])
}
