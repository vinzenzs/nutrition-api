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

type goalOverrideRecord struct {
	method   string
	path     string
	rawQuery string
	body     []byte
	idemKey  string
}

func newOverrideRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]goalOverrideRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []goalOverrideRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, goalOverrideRecord{
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

func TestSetDailyGoalOverride_PutsWithoutIdempotencyKey(t *testing.T) {
	c, recs := newOverrideRecorder(t, 200, `{"goals":{"kcal":{"min":2280,"max":2520}}}`)
	min, max := 2280.0, 2520.0
	args := SetDailyGoalOverrideArgs{
		Date: "2026-06-15",
		Kcal: &GoalRange{Min: &min, Max: &max},
	}
	r := handleSetDailyGoalOverride(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/goals/overrides/2026-06-15", rec.path)
	assert.JSONEq(t, `{"kcal":{"min":2280,"max":2520}}`, string(rec.body))
	assert.Empty(t, rec.idemKey, "set_daily_goal_override must not send Idempotency-Key (PUT)")
}

func TestSetDailyGoalOverride_ForwardsBackend4xx(t *testing.T) {
	c, _ := newOverrideRecorder(t, 400, `{"error":"goal_range_invalid","field":"kcal"}`)
	min, max := 2500.0, 2000.0 // inverted
	r := handleSetDailyGoalOverride(context.Background(), c, SetDailyGoalOverrideArgs{
		Date: "2026-06-15",
		Kcal: &GoalRange{Min: &min, Max: &max},
	})
	assert.True(t, r.IsError)
}

func TestGetDailyGoalOverride_HitsDateURL(t *testing.T) {
	c, recs := newOverrideRecorder(t, 200, `{"goals":{}}`)
	r := handleGetDailyGoalOverride(context.Background(), c, GetDailyGoalOverrideArgs{
		Date: "2026-06-15",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/goals/overrides/2026-06-15", rec.path)
	assert.Empty(t, rec.idemKey)
}

func TestGetDailyGoalOverride_Forwards404(t *testing.T) {
	c, _ := newOverrideRecorder(t, 404, `{"error":"override_not_found"}`)
	r := handleGetDailyGoalOverride(context.Background(), c, GetDailyGoalOverrideArgs{
		Date: "2026-06-15",
	})
	assert.True(t, r.IsError)
}

func TestDeleteDailyGoalOverride_204ReturnsEmptySuccess(t *testing.T) {
	c, recs := newOverrideRecorder(t, 204, "")
	r := handleDeleteDailyGoalOverride(context.Background(), c, DeleteDailyGoalOverrideArgs{
		Date: "2026-06-15",
	})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/goals/overrides/2026-06-15", (*recs)[0].path)
	assert.NotEmpty(t, (*recs)[0].idemKey, "DELETE auto-derives an idempotency key")
}

func TestDeleteDailyGoalOverride_404ReturnsIsError(t *testing.T) {
	c, _ := newOverrideRecorder(t, 404, `{"error":"override_not_found"}`)
	r := handleDeleteDailyGoalOverride(context.Background(), c, DeleteDailyGoalOverrideArgs{
		Date: "2026-06-15",
	})
	assert.True(t, r.IsError)
}

func TestListDailyGoalOverrides_BuildsRangeQuery(t *testing.T) {
	c, recs := newOverrideRecorder(t, 200, `{"overrides":[]}`)
	r := handleListDailyGoalOverrides(context.Background(), c, ListDailyGoalOverridesArgs{
		From: "2026-06-01",
		To:   "2026-06-30",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/goals/overrides", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01", q.Get("from"))
	assert.Equal(t, "2026-06-30", q.Get("to"))
	assert.Empty(t, rec.idemKey)
}
