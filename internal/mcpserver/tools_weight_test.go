package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type weightRecord struct {
	method   string
	path     string
	rawQuery string
	body     []byte
	idemKey  string
}

func newWeightRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]weightRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []weightRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, weightRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			body:     body,
			idemKey:  r.Header.Get("Idempotency-Key"),
		})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, &records
}

func TestLogWeight_PostsToWeightWithIdempotencyKey(t *testing.T) {
	c, recs := newWeightRecorder(t, 201, `{"id":"w1","weight_kg":72.5}`)
	bf := 14.2
	args := LogWeightArgs{
		WeightKg:   72.5,
		LoggedAt:   "2026-06-07T07:00:00Z",
		BodyFatPct: &bf,
		Note:       "morning, fasted",
	}
	r := handleLogWeight(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/weight", rec.path)
	assert.JSONEq(t,
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z","body_fat_pct":14.2,"note":"morning, fasted"}`,
		string(rec.body))
	assert.NotEmpty(t, rec.idemKey, "derived idempotency key should be present")
}

func TestLogWeight_OmitsUnsetOptionalFields(t *testing.T) {
	c, recs := newWeightRecorder(t, 201, `{"id":"w1"}`)
	_ = handleLogWeight(context.Background(), c, LogWeightArgs{
		WeightKg: 72.5,
		LoggedAt: "2026-06-07T07:00:00Z",
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t,
		`{"weight_kg":72.5,"logged_at":"2026-06-07T07:00:00Z"}`,
		string((*recs)[0].body))
}

func TestLogWeight_ExplicitIdempotencyKeyIsForwarded(t *testing.T) {
	c, recs := newWeightRecorder(t, 201, `{"id":"w1"}`)
	args := LogWeightArgs{
		WeightKg:       72.5,
		LoggedAt:       "2026-06-07T07:00:00Z",
		IdempotencyKey: "explicit-weight-key",
	}
	_ = handleLogWeight(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-weight-key", (*recs)[0].idemKey)
}

func TestLogWeight_SameArgsProduceSameDerivedKey(t *testing.T) {
	c, recs := newWeightRecorder(t, 201, `{"id":"w1"}`)
	args := LogWeightArgs{WeightKg: 72.5, LoggedAt: "2026-06-07T07:00:00Z"}
	_ = handleLogWeight(context.Background(), c, args)
	_ = handleLogWeight(context.Background(), c, args)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey)
}

func TestListWeights_GetsWithWindowQuery(t *testing.T) {
	c, recs := newWeightRecorder(t, 200, `{"entries":[]}`)
	args := ListWeightsArgs{
		From: "2026-06-01T00:00:00Z",
		To:   "2026-06-08T00:00:00Z",
	}
	r := handleListWeights(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/weight", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", q.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", q.Get("to"))
	assert.Empty(t, rec.idemKey, "list is read-only; no idempotency key")
}

func TestPatchWeight_OnlySuppliedFieldsAreSent(t *testing.T) {
	c, recs := newWeightRecorder(t, 200, `{"id":"w1"}`)
	bf := 13.8
	args := PatchWeightArgs{
		ID:         "abc",
		BodyFatPct: &bf,
	}
	_ = handlePatchWeight(context.Background(), c, args)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/weight/abc", rec.path)
	assert.JSONEq(t, `{"body_fat_pct":13.8}`, string(rec.body))
	assert.NotEmpty(t, rec.idemKey)
}

func TestDeleteWeight_204ReturnsEmptySuccessResult(t *testing.T) {
	c, recs := newWeightRecorder(t, 204, "")
	r := handleDeleteWeight(context.Background(), c, DeleteWeightArgs{ID: "abc"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/weight/abc", (*recs)[0].path)
}

func TestDeleteWeight_404ReturnsIsError(t *testing.T) {
	c, _ := newWeightRecorder(t, 404, `{"error":"weight_not_found"}`)
	r := handleDeleteWeight(context.Background(), c, DeleteWeightArgs{ID: "abc"})
	assert.True(t, r.IsError)
}

func TestWeightTrend_GetsWithFromTo(t *testing.T) {
	c, recs := newWeightRecorder(t, 200, `{"points":[]}`)
	wd := 7
	_ = handleWeightTrend(context.Background(), c, WeightTrendArgs{
		From:       "2026-05-01",
		To:         "2026-06-07",
		WindowDays: &wd,
		TZ:         "Europe/Berlin",
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/weight/trend", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-05-01", q.Get("from"))
	assert.Equal(t, "2026-06-07", q.Get("to"))
	assert.Equal(t, "7", q.Get("window_days"))
	assert.Equal(t, "Europe/Berlin", q.Get("tz"))
	assert.Empty(t, rec.idemKey, "trend is read-only; no idempotency key")
}

func TestWeightTrend_OmitsOptionalsWhenUnset(t *testing.T) {
	c, recs := newWeightRecorder(t, 200, `{"points":[]}`)
	_ = handleWeightTrend(context.Background(), c, WeightTrendArgs{
		From: "2026-05-01",
		To:   "2026-06-07",
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Empty(t, q.Get("window_days"))
	assert.Empty(t, q.Get("tz"))
}

func TestLogWeight_400Forwarded(t *testing.T) {
	c, _ := newWeightRecorder(t, 400, `{"error":"weight_kg_invalid"}`)
	r := handleLogWeight(context.Background(), c, LogWeightArgs{
		WeightKg: -1, LoggedAt: "2026-06-07T07:00:00Z",
	})
	assert.True(t, r.IsError)
}
