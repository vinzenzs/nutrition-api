package idempotency_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

func setupRouter(t *testing.T) (*gin.Engine, *idempotency.Repo, *int) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := idempotency.NewRepo(pool)

	calls := 0
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(repo, time.Hour))
	r.POST("/things", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusCreated, gin.H{"id": "thing-" + strconv.Itoa(calls)})
	})
	r.GET("/ping", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r, repo, &calls
}

func post(r *gin.Engine, body, token, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/things", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_FirstWriteStoresAndReplaysSameResponse(t *testing.T) {
	r, _, calls := setupRouter(t)

	first := post(r, `{"a":1}`, mobileToken, "key-1")
	require.Equal(t, http.StatusCreated, first.Code)
	assert.Equal(t, 1, *calls)
	firstBody := first.Body.String()

	second := post(r, `{"a":1}`, mobileToken, "key-1")
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, 1, *calls, "handler should not run on replay")
	assert.Equal(t, firstBody, second.Body.String())
}

func TestMiddleware_DifferentBodyReturns409(t *testing.T) {
	r, _, _ := setupRouter(t)

	first := post(r, `{"a":1}`, mobileToken, "key-conflict")
	require.Equal(t, http.StatusCreated, first.Code)

	conflict := post(r, `{"a":2}`, mobileToken, "key-conflict")
	require.Equal(t, http.StatusConflict, conflict.Code)
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
}

func TestMiddleware_DifferentClientsAreIsolated(t *testing.T) {
	r, _, calls := setupRouter(t)

	mobile := post(r, `{"a":1}`, mobileToken, "shared-key")
	require.Equal(t, http.StatusCreated, mobile.Code)
	mobileBody := mobile.Body.String()

	agent := post(r, `{"a":1}`, agentToken, "shared-key")
	require.Equal(t, http.StatusCreated, agent.Code)
	agentBody := agent.Body.String()

	assert.Equal(t, 2, *calls, "each client should run the handler independently")
	assert.NotEqual(t, mobileBody, agentBody, "different ids returned per client")
}

func TestMiddleware_GetRequestsIgnoreIdempotencyKey(t *testing.T) {
	r, _, calls := setupRouter(t)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Authorization", "Bearer "+mobileToken)
		req.Header.Set("Idempotency-Key", "any-key")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	assert.Equal(t, 3, *calls, "GET should always run, never replay")
}

func TestMiddleware_ExpiredRecordIsTreatedAsFirstArrival(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := idempotency.NewRepo(pool)

	calls := 0
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	// TTL of zero means everything is immediately expired.
	r.Use(idempotency.Middleware(repo, time.Nanosecond))
	r.POST("/things", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusCreated, gin.H{"id": "thing-" + strconv.Itoa(calls)})
	})

	first := post(r, `{"a":1}`, mobileToken, "ttl-key")
	require.Equal(t, http.StatusCreated, first.Code)

	time.Sleep(2 * time.Nanosecond)

	second := post(r, `{"a":1}`, mobileToken, "ttl-key")
	require.Equal(t, http.StatusCreated, second.Code)
	assert.Equal(t, 2, calls, "expired key should run the handler again")
}

func TestMiddleware_NoKeyHeaderRunsHandlerEveryTime(t *testing.T) {
	r, _, calls := setupRouter(t)
	for i := 0; i < 2; i++ {
		rec := post(r, `{"a":1}`, mobileToken, "")
		require.Equal(t, http.StatusCreated, rec.Code)
	}
	assert.Equal(t, 2, *calls)
}

// setupRouterWithPut wires a tiny PUT /thing endpoint so we can drive the
// middleware's PUT branch directly without depending on a real domain handler.
func setupRouterWithPut(t *testing.T) (*gin.Engine, *idempotency.Repo, *int) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := idempotency.NewRepo(pool)

	calls := 0
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(repo, time.Hour))
	r.PUT("/thing", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r, repo, &calls
}

func TestMiddleware_PutWithIdempotencyKeyRejected(t *testing.T) {
	r, _, calls := setupRouterWithPut(t)

	req := httptest.NewRequest(http.MethodPut, "/thing", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+mobileToken)
	req.Header.Set("Idempotency-Key", "anykey")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t,
		`{"error":"idempotency_unsupported_for_put","hint":"use If-Match with ETag for retry-safety"}`,
		rec.Body.String())
	assert.Equal(t, 0, *calls, "handler must not run when PUT is rejected")
}

func TestMiddleware_PutWithoutIdempotencyKeyPassesThrough(t *testing.T) {
	r, repo, calls := setupRouterWithPut(t)

	req := httptest.NewRequest(http.MethodPut, "/thing", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+mobileToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, *calls, "handler should run when no Idempotency-Key is supplied")

	// No idempotency_records row should be inserted for a header-less PUT.
	// We don't have a direct count helper; this assertion uses the fact that a
	// subsequent identical PUT also runs the handler (no cached replay).
	req2 := httptest.NewRequest(http.MethodPut, "/thing", bytes.NewBufferString(`{}`))
	req2.Header.Set("Authorization", "Bearer "+mobileToken)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, 2, *calls, "header-less PUT must not store/replay anything")
	_ = repo
}
