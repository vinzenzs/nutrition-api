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

type racePrepRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
}

func newRacePrepRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]racePrepRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []racePrepRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, racePrepRecord{
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

func TestPlanCarbLoad_RequiredParamsOnly(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{"race_date":"2026-07-24"}`)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:     "2026-07-24",
		BodyWeightKg: 70,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/race-prep/carb-load", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-24", q.Get("race_date"))
	assert.Equal(t, "70", q.Get("body_weight_kg"))
	assert.Empty(t, q.Get("days_before"), "unset optionals must not be sent")
	assert.Empty(t, q.Get("carbs_per_kg_per_day"))
	assert.Empty(t, q.Get("race_day_carbs_per_kg"))
	assert.Empty(t, rec.idemKey, "read-only — no Idempotency-Key")
}

func TestPlanCarbLoad_OptionalParamsForwarded(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	db := 2
	cpd := 8.0
	rdc := 2.5
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:          "2026-07-24",
		BodyWeightKg:      70,
		DaysBefore:        &db,
		CarbsPerKgPerDay:  &cpd,
		RaceDayCarbsPerKg: &rdc,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2", q.Get("days_before"))
	assert.Equal(t, "8", q.Get("carbs_per_kg_per_day"))
	assert.Equal(t, "2.5", q.Get("race_day_carbs_per_kg"))
}

func TestPlanCarbLoad_400FromBackendForwarded(t *testing.T) {
	c, _ := newRacePrepRecorder(t, 400, `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 25,
	})
	assert.True(t, r.IsError)
}

func TestPlanCarbLoad_ResponseBodyForwardedVerbatim(t *testing.T) {
	body := `{"race_date":"2026-07-24","body_weight_kg":70,"params":{"days_before":3,"carbs_per_kg_per_day":10,"race_day_carbs_per_kg":2},"schedule":[]}`
	c, _ := newRacePrepRecorder(t, 200, body)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 70,
	})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	// The text content is the body verbatim — no envelope, no transformation.
}
