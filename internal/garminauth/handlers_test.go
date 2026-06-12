package garminauth_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/garminauth"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
	garminToken = "garmin-token-cccccccccccccc"
)

func encKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i * 7)
	}
	return k
}

type fixture struct {
	r    *gin.Engine
	pool *pgxpool.Pool
}

// setup builds a router with the full auth + idempotency middleware stack and
// the garmin handlers. `enabled` mirrors GARMIN_API_TOKEN being configured.
func setup(t *testing.T, enabled bool) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)

	var key []byte
	authCfg := auth.Config{MobileToken: mobileToken, AgentToken: agentToken}
	if enabled {
		key = encKey()
		authCfg.GarminToken = garminToken
	}
	svc, err := garminauth.NewService(garminauth.NewRepo(pool), key)
	require.NoError(t, err)

	r := gin.New()
	r.Use(auth.Middleware(authCfg))
	r.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	rg := r.Group("/")
	garminauth.NewHandlers(svc, enabled).Register(rg)
	return &fixture{r: r, pool: pool}
}

func doReq(t *testing.T, r *gin.Engine, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPutThenGet_RoundTripVerbatim(t *testing.T) {
	f := setup(t, true)
	blob := []byte("garth-serialized-oauth1-oauth2-token-blob\x00\x01\x02")

	rec := doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, blob)
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = doReq(t, f.r, http.MethodGet, "/garmin/token", garminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, bytes.Equal(blob, rec.Body.Bytes()), "GET must return the blob byte-identical")
}

func TestPut_ReplacesPriorValue(t *testing.T) {
	f := setup(t, true)
	require.Equal(t, http.StatusNoContent,
		doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, []byte("first")).Code)
	require.Equal(t, http.StatusNoContent,
		doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, []byte("second")).Code)

	rec := doReq(t, f.r, http.MethodGet, "/garmin/token", garminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "second", rec.Body.String())
}

func TestGet_WhenEmpty_404(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodGet, "/garmin/token", garminToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"garmin_token_not_found"}`, rec.Body.String())
}

func TestDelete_ClearsThenGet404(t *testing.T) {
	f := setup(t, true)
	require.Equal(t, http.StatusNoContent,
		doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, []byte("blob")).Code)

	rec := doReq(t, f.r, http.MethodDelete, "/garmin/token", garminToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = doReq(t, f.r, http.MethodGet, "/garmin/token", garminToken, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDelete_WhenEmpty_404(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodDelete, "/garmin/token", garminToken, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPut_EmptyBody_400(t *testing.T) {
	f := setup(t, true)
	rec := doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"garmin_token_empty"}`, rec.Body.String())
}

func TestOtherIdentities_Forbidden(t *testing.T) {
	f := setup(t, true)
	for _, tok := range []string{mobileToken, agentToken} {
		// PUT
		rec := doReq(t, f.r, http.MethodPut, "/garmin/token", tok, []byte("blob"))
		assert.Equal(t, http.StatusForbidden, rec.Code, "PUT with non-garmin token")
		assert.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
		// GET
		rec = doReq(t, f.r, http.MethodGet, "/garmin/token", tok, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "GET with non-garmin token")
		// DELETE
		rec = doReq(t, f.r, http.MethodDelete, "/garmin/token", tok, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "DELETE with non-garmin token")
	}
	// No write should have occurred.
	var count int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM garmin_tokens`).Scan(&count))
	assert.Equal(t, 0, count, "forbidden requests must not write")
}

func TestDisabled_503(t *testing.T) {
	f := setup(t, false)
	// Even a valid mobile identity gets 503 when the integration is off.
	for _, m := range []string{http.MethodGet, http.MethodDelete} {
		rec := doReq(t, f.r, m, "/garmin/token", mobileToken, nil)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, m)
		assert.JSONEq(t, `{"error":"garmin_disabled"}`, rec.Body.String())
	}
	rec := doReq(t, f.r, http.MethodPut, "/garmin/token", mobileToken, []byte("blob"))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestAtRest_RowHoldsCiphertextNotPlaintext(t *testing.T) {
	f := setup(t, true)
	blob := []byte("super-secret-garmin-token-plaintext")
	require.Equal(t, http.StatusNoContent,
		doReq(t, f.r, http.MethodPut, "/garmin/token", garminToken, blob).Code)

	var ciphertext, nonce []byte
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT ciphertext, nonce FROM garmin_tokens WHERE id = 1`).Scan(&ciphertext, &nonce))
	assert.NotEmpty(t, ciphertext)
	assert.NotEmpty(t, nonce)
	assert.False(t, bytes.Contains(ciphertext, blob), "stored bytes must not contain the plaintext")
	assert.False(t, bytes.Equal(ciphertext, blob), "stored bytes must be ciphertext, not plaintext")
}

func TestPut_RejectsIdempotencyKey(t *testing.T) {
	// PUT is full-replace; the idempotency middleware rejects the header.
	f := setup(t, true)
	req := httptest.NewRequest(http.MethodPut, "/garmin/token", bytes.NewReader([]byte("blob")))
	req.Header.Set("Authorization", "Bearer "+garminToken)
	req.Header.Set(idempotency.HeaderName, "some-key")
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.Contains(t, string(body), "idempotency_unsupported_for_put")
}
