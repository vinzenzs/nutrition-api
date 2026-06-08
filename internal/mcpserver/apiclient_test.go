package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*apiClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "test-token",
		userAgent: "nutrition-mcp/test",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, srv
}

func TestAPIClient_InjectsHeadersOnGet(t *testing.T) {
	var gotAuth, gotUA string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	status, body, err := c.Get(context.Background(), "/things", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, `{"ok":true}`, string(body))
	assert.Equal(t, "Bearer test-token", gotAuth)
	assert.Equal(t, "nutrition-mcp/test", gotUA)
}

func TestAPIClient_PostForwardsBodyAndIdempotencyKey(t *testing.T) {
	var (
		gotBody    []byte
		gotIdemKey string
		gotCType   string
	)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotIdemKey = r.Header.Get("Idempotency-Key")
		gotCType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"x"}`)
	})
	status, _, err := c.Post(context.Background(), "/meals", nil, []byte(`{"a":1}`), "key-1")
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.JSONEq(t, `{"a":1}`, string(gotBody))
	assert.Equal(t, "key-1", gotIdemKey)
	assert.Equal(t, "application/json", gotCType)
}

func TestAPIClient_OmitsIdempotencyKeyWhenEmpty(t *testing.T) {
	var gotIdemKey string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotIdemKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
	})
	_, _, err := c.Post(context.Background(), "/x", nil, []byte(`{}`), "")
	require.NoError(t, err)
	assert.Empty(t, gotIdemKey)
}

func TestAPIClient_PassesThroughNon2xx(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"missing"}`)
	})
	status, body, err := c.Get(context.Background(), "/missing", nil)
	require.NoError(t, err, "non-2xx is not a transport error")
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, `{"error":"missing"}`, string(body))
}

func TestAPIClient_TransportFailureIsClassified(t *testing.T) {
	c := &apiClient{
		baseURL:   mustURL("http://127.0.0.1:1"), // closed port
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 200 * time.Millisecond},
	}
	_, _, err := c.Get(context.Background(), "/x", nil)
	require.Error(t, err)
	assert.True(t, IsTransportError(err), "DNS/connect/timeout failures should classify as transport errors")
}

func TestAPIClient_QueryParamsAppended(t *testing.T) {
	var gotURL string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	})
	q := url.Values{}
	q.Set("from", "2026-06-01")
	q.Set("to", "2026-06-07")
	_, _, err := c.Get(context.Background(), "/summary/range", q)
	require.NoError(t, err)
	assert.Contains(t, gotURL, "/summary/range?")
	assert.Contains(t, gotURL, "from=2026-06-01")
	assert.Contains(t, gotURL, "to=2026-06-07")
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
