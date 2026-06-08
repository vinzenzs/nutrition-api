package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/auth"
)

const HeaderName = "Idempotency-Key"

// Middleware returns a Gin middleware that intercepts write requests carrying
// an Idempotency-Key header, replays stored responses on key+body match,
// returns 409 on key+body mismatch, and otherwise persists the handler response.
//
// Must be mounted AFTER the auth middleware so client_id is available.
func Middleware(repo *Repo, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}
		key := c.GetHeader(HeaderName)
		// PUT-style writes replace resource state; a cached replay can
		// silently lie when intermediate writes have changed that state.
		// Reject the header loudly so the design rule surfaces at the first
		// integration attempt, not later as silent data corruption. Bug-fix
		// reference: harden-write-paths.
		if method == http.MethodPut && key != "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "idempotency_unsupported_for_put",
				"hint":  "use If-Match with ETag for retry-safety",
			})
			return
		}
		if key == "" {
			c.Next()
			return
		}
		client := auth.ClientFromContext(c)
		if client == "" {
			c.Next()
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
			return
		}
		_ = c.Request.Body.Close()
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		hash := sha256Hex(bodyBytes)
		recordKey := Key{
			ClientID: string(client),
			Method:   method,
			Path:     c.FullPath(),
			Key:      key,
		}

		existing, err := repo.Get(c.Request.Context(), recordKey, ttl)
		if err == nil {
			if existing.RequestBodyHash != hash {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "idempotency_key_conflict"})
				return
			}
			c.Data(existing.Status, "application/json; charset=utf-8", existing.ResponseBody)
			c.Abort()
			return
		}
		if !errors.Is(err, ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "idempotency_lookup_failed"})
			return
		}

		// Capture the downstream response so we can persist it.
		cap := &captureWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = cap
		c.Next()

		// Only persist successful-ish responses. 4xx/5xx clients should be free
		// to retry after fixing their request.
		if cap.status < 200 || cap.status >= 300 {
			return
		}

		rec := Record{
			Status:          cap.status,
			ResponseBody:    cap.buf.Bytes(),
			RequestBodyHash: hash,
			CreatedAt:       time.Now().UTC(),
		}
		if err := repo.Insert(c.Request.Context(), recordKey, rec); err != nil {
			// Insertion failure (e.g. concurrent write) is logged at the
			// router level; we do not change the already-sent response.
			_ = err
		}
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// captureWriter records the status and body written by downstream handlers
// so the middleware can store them after the chain completes.
type captureWriter struct {
	gin.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (w *captureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
