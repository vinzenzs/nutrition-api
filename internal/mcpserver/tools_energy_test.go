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
)

type energyRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
}

func newEnergyRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]energyRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []energyRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, energyRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
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

func TestWeeklyEnergySummary_GETsToEndpointWithRequiredParams(t *testing.T) {
	c, recs := newEnergyRecorder(t, 200, `{"days":[],"window":{"avg_ea":null}}`)
	r := handleWeeklyEnergySummary(context.Background(), c, WeeklyEnergySummaryArgs{
		From: "2026-06-01T00:00:00Z",
		To:   "2026-06-08T00:00:00Z",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/energy/availability", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", q.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", q.Get("to"))
	assert.Empty(t, q.Get("tz"))
	assert.Empty(t, q.Get("lean_mass_kg"))
	assert.Empty(t, q.Get("body_fat_pct"))
	assert.Empty(t, rec.idemKey, "energy summary is read-only; no idempotency-key")
}

func TestWeeklyEnergySummary_ForwardsOptionalOverrides(t *testing.T) {
	c, recs := newEnergyRecorder(t, 200, `{}`)
	lean := 62.0
	bf := 15.0
	_ = handleWeeklyEnergySummary(context.Background(), c, WeeklyEnergySummaryArgs{
		From:       "2026-06-01T00:00:00Z",
		To:         "2026-06-08T00:00:00Z",
		TZ:         "Europe/Berlin",
		LeanMassKg: &lean,
		BodyFatPct: &bf,
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", q.Get("tz"))
	assert.Equal(t, "62", q.Get("lean_mass_kg"))
	assert.Equal(t, "15", q.Get("body_fat_pct"))
}

func TestWeeklyEnergySummary_OmitsBodyFatWhenOnlyLeanMassSet(t *testing.T) {
	c, recs := newEnergyRecorder(t, 200, `{}`)
	lean := 62.0
	_ = handleWeeklyEnergySummary(context.Background(), c, WeeklyEnergySummaryArgs{
		From:       "2026-06-01T00:00:00Z",
		To:         "2026-06-08T00:00:00Z",
		LeanMassKg: &lean,
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "62", q.Get("lean_mass_kg"))
	assert.Empty(t, q.Get("body_fat_pct"), "body_fat_pct must be omitted when nil")
}

func TestWeeklyEnergySummary_400Forwarded(t *testing.T) {
	c, _ := newEnergyRecorder(t, 400, `{"error":"weight_data_missing"}`)
	r := handleWeeklyEnergySummary(context.Background(), c, WeeklyEnergySummaryArgs{
		From: "2026-06-01T00:00:00Z",
		To:   "2026-06-08T00:00:00Z",
	})
	assert.True(t, r.IsError)
}
