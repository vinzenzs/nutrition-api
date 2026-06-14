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

// Shared request recorder for the remaining bespoke MCP tests (registry
// dispatch, multipart photo). The per-domain bespoke recorders moved out with
// their domains as they were ported onto the shared registry
// (unify-mcp-tool-registry); this one survives for the generic dispatcher tests.
type recordedRequest struct {
	method   string
	path     string
	rawQuery string
	body     []byte
	idemKey  string
}

func newWorkoutRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]recordedRequest) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []recordedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, recordedRequest{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			body:     raw,
			idemKey:  r.Header.Get("Idempotency-Key"),
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
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
