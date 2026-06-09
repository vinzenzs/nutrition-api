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

type dcRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
}

func newDCRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]dcRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []dcRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, dcRecord{
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
	return &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}, &records
}

func TestDailyContext_DateOnlyHitsGET(t *testing.T) {
	c, recs := newDCRecorder(t, 200, `{}`)
	r := handleDailyContext(context.Background(), c, DailyContextArgs{Date: "2026-07-15"})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/context/daily", rec.path)
	q, _ := url.ParseQuery(rec.rawQuery)
	assert.Equal(t, "2026-07-15", q.Get("date"))
	assert.Empty(t, q.Get("tz"), "unset tz must not be sent")
	assert.Empty(t, rec.idemKey, "read-only — no Idempotency-Key")
}

func TestDailyContext_OptionalTZForwarded(t *testing.T) {
	c, recs := newDCRecorder(t, 200, `{}`)
	_ = handleDailyContext(context.Background(), c, DailyContextArgs{
		Date: "2026-07-15", TZ: "Europe/Berlin",
	})
	require.Len(t, *recs, 1)
	q, _ := url.ParseQuery((*recs)[0].rawQuery)
	assert.Equal(t, "Europe/Berlin", q.Get("tz"))
}

func TestDailyContext_400FromBackendForwarded(t *testing.T) {
	c, _ := newDCRecorder(t, 400, `{"error":"date_invalid"}`)
	r := handleDailyContext(context.Background(), c, DailyContextArgs{Date: "bad"})
	assert.True(t, r.IsError)
}

func TestDailyContext_ResponseBodyForwardedVerbatim(t *testing.T) {
	body := `{"date":"2026-07-15","tz":"UTC","adherence":{"goal_source":"none"}}`
	c, _ := newDCRecorder(t, 200, body)
	r := handleDailyContext(context.Background(), c, DailyContextArgs{Date: "2026-07-15"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
}
