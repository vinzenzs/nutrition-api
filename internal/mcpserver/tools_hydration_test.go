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

type hydrationRecord struct {
	method  string
	path    string
	rawQuery string
	body    []byte
	idemKey string
}

func newHydrationRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]hydrationRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []hydrationRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, hydrationRecord{
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

func TestLogHydration_PostsToHydrationWithIdempotencyKey(t *testing.T) {
	c, recs := newHydrationRecorder(t, 201, `{"id":"h1","quantity_ml":500}`)
	args := LogHydrationArgs{
		QuantityMl: 500,
		LoggedAt:   "2026-06-07T08:00:00Z",
		Note:       "water",
	}
	r := handleLogHydration(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/hydration", rec.path)
	assert.JSONEq(t,
		`{"quantity_ml":500,"logged_at":"2026-06-07T08:00:00Z","note":"water"}`,
		string(rec.body))
	assert.NotEmpty(t, rec.idemKey, "derived idempotency key should be present")
}

func TestLogHydration_ExplicitIdempotencyKeyIsForwarded(t *testing.T) {
	c, recs := newHydrationRecorder(t, 201, `{"id":"h1"}`)
	args := LogHydrationArgs{
		QuantityMl:     500,
		LoggedAt:       "2026-06-07T08:00:00Z",
		IdempotencyKey: "explicit-h-key",
	}
	_ = handleLogHydration(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-h-key", (*recs)[0].idemKey)
}

func TestLogHydration_SameArgsProduceSameDerivedKey(t *testing.T) {
	c, recs := newHydrationRecorder(t, 201, `{"id":"h1"}`)
	args := LogHydrationArgs{QuantityMl: 500, LoggedAt: "2026-06-07T08:00:00Z"}
	_ = handleLogHydration(context.Background(), c, args)
	_ = handleLogHydration(context.Background(), c, args)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey)
}

func TestListHydration_GetsWithWindowQuery(t *testing.T) {
	c, recs := newHydrationRecorder(t, 200, `{"entries":[]}`)
	args := ListHydrationArgs{
		From: "2026-06-01T00:00:00Z",
		To:   "2026-06-08T00:00:00Z",
	}
	r := handleListHydration(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/hydration", rec.path)
	// URL-encoded ':' is %3A; net/url Values.Encode will escape it.
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", q.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", q.Get("to"))
	assert.Empty(t, rec.idemKey, "list is read-only; no idempotency key")
}

func TestPatchHydration_OnlySuppliedFieldsAreSent(t *testing.T) {
	c, recs := newHydrationRecorder(t, 200, `{"id":"h1"}`)
	qty := 250.0
	args := PatchHydrationArgs{
		ID:         "abc",
		QuantityMl: &qty,
	}
	_ = handlePatchHydration(context.Background(), c, args)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/hydration/abc", rec.path)
	assert.JSONEq(t, `{"quantity_ml":250}`, string(rec.body))
	assert.NotEmpty(t, rec.idemKey)
}

func TestDeleteHydration_204ReturnsEmptySuccessResult(t *testing.T) {
	c, recs := newHydrationRecorder(t, 204, "")
	r := handleDeleteHydration(context.Background(), c, DeleteHydrationArgs{ID: "abc"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/hydration/abc", (*recs)[0].path)
}

func TestDeleteHydration_404ReturnsIsError(t *testing.T) {
	c, _ := newHydrationRecorder(t, 404, `{"error":"hydration_not_found"}`)
	r := handleDeleteHydration(context.Background(), c, DeleteHydrationArgs{ID: "abc"})
	assert.True(t, r.IsError)
}

func TestDailyHydrationSummary_GetsWithDateAndTZ(t *testing.T) {
	c, recs := newHydrationRecorder(t, 200, `{"date":"2026-06-07","total_ml":2250}`)
	args := DailyHydrationSummaryArgs{
		Date: "2026-06-07",
		TZ:   "Europe/Berlin",
	}
	r := handleDailyHydrationSummary(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/summary/hydration/daily", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-07", q.Get("date"))
	assert.Equal(t, "Europe/Berlin", q.Get("tz"))
	assert.Empty(t, rec.idemKey, "summary is read-only; no idempotency key")
}

func TestDailyHydrationSummary_OmitsTZWhenUnset(t *testing.T) {
	c, recs := newHydrationRecorder(t, 200, `{"date":"2026-06-07"}`)
	_ = handleDailyHydrationSummary(context.Background(), c,
		DailyHydrationSummaryArgs{Date: "2026-06-07"})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-07", q.Get("date"))
	assert.Empty(t, q.Get("tz"))
}

func TestLogHydration_400Forwarded(t *testing.T) {
	c, _ := newHydrationRecorder(t, 400, `{"error":"quantity_ml_invalid"}`)
	r := handleLogHydration(context.Background(), c, LogHydrationArgs{
		QuantityMl: 0, LoggedAt: "2026-06-07T08:00:00Z",
	})
	assert.True(t, r.IsError)
}
