package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type garminRecord struct {
	method string
	path   string
	body   []byte
}

func newGarminRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]garminRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []garminRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, garminRecord{method: r.Method, path: r.URL.Path, body: raw})
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

func TestGarminLogin_PostsNoBodyNoKey(t *testing.T) {
	c, recs := newGarminRecorder(t, 200, `{"needs_mfa":true}`)
	res := handleGarminLogin(context.Background(), c, GarminLoginArgs{})

	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/garmin/login", rec.path)
	// No credentials in the call: the body is empty.
	assert.Empty(t, rec.body)
	// Tool result forwards the proxy body verbatim, not an error.
	assert.False(t, res.IsError)
	assert.Equal(t, `{"needs_mfa":true}`, textOf(t, res))
}

func TestGarminSubmitMFA_ForwardsCode(t *testing.T) {
	c, recs := newGarminRecorder(t, 200, `{"logged_in":true}`)
	res := handleGarminSubmitMFA(context.Background(), c, GarminSubmitMFAArgs{Code: "418923"})

	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/garmin/login/mfa", rec.path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &body))
	assert.Equal(t, "418923", body["code"])
	assert.False(t, res.IsError)
	assert.Equal(t, `{"logged_in":true}`, textOf(t, res))
}

func TestGarminTools_DisabledBridgeIsToolError(t *testing.T) {
	// The proxy returns 503 garmin_disabled; the wrapper must mark isError.
	c, _ := newGarminRecorder(t, http.StatusServiceUnavailable, `{"error":"garmin_disabled"}`)
	res := handleGarminLogin(context.Background(), c, GarminLoginArgs{})
	assert.True(t, res.IsError)
	assert.Equal(t, `{"error":"garmin_disabled"}`, textOf(t, res))

	res2 := handleGarminSubmitMFA(context.Background(), c, GarminSubmitMFAArgs{Code: "000000"})
	assert.True(t, res2.IsError)
}

// textOf extracts the single text content from a tool result.
func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	return tc.Text
}
