package mcpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// racePrepRecord and newRacePrepRecorder were defined in tools_raceprep_test.go
// until the raceprep domain was ported onto the shared registry
// (unify-mcp-tool-registry). The still-bespoke mealplan and shopping tests
// share this recorder. Remove it once those domains are ported and their tests
// move to agenttools Build-shape assertions.
type racePrepRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
	body     string
}

func newRacePrepRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]racePrepRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []racePrepRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, racePrepRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			idemKey:  r.Header.Get("Idempotency-Key"),
			body:     string(raw),
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
