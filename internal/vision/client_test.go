package vision_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/vision"
)

// loadFixture pulls a recorded Anthropic response from testdata/.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err, "fixture %s missing", name)
	return raw
}

// fakeImage is a 1-byte payload that satisfies the client's base64-encoder.
// The client doesn't actually verify the image bytes — that's the server's
// job at multipart parse time — so any non-empty byte slice works for tests
// that exercise the client's request shape.
var fakeImage = []byte{0xff}

// stubTransport routes by URL with one canned reply. Capturing the request
// body lets the tests assert what we sent to Anthropic.
type stubTransport struct {
	mu          sync.Mutex
	status      int
	body        []byte
	retryAfter  string
	callCount   int
	lastReqBody []byte
	// onCall is invoked synchronously before responding; tests can mutate
	// the stub mid-conversation (e.g. switch the body on the second call to
	// simulate the retry path).
	onCall func(call int)
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, _ := io.ReadAll(req.Body)
	s.lastReqBody = body
	s.callCount++
	if s.onCall != nil {
		s.onCall(s.callCount)
	}
	h := make(http.Header)
	if s.retryAfter != "" {
		h.Set("Retry-After", s.retryAfter)
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     h,
		Request:    req,
	}, nil
}

func newClient(t *testing.T, transport http.RoundTripper) *vision.Client {
	t.Helper()
	c, err := vision.New(vision.Config{APIKey: "test-key"})
	require.NoError(t, err)
	c.SetHTTPClient(&http.Client{Transport: transport, Timeout: 5 * time.Second})
	return c
}

// ============================================================================
// Constructor
// ============================================================================

func TestNew_MissingAPIKeyReturnsSentinel(t *testing.T) {
	_, err := vision.New(vision.Config{})
	assert.ErrorIs(t, err, vision.ErrAPIKeyMissing)
}

func TestNew_DefaultsApplied(t *testing.T) {
	c, err := vision.New(vision.Config{APIKey: "test"})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

// ============================================================================
// Parse — happy paths
// ============================================================================

func TestParse_WellFormed(t *testing.T) {
	transport := &stubTransport{status: 200, body: loadFixture(t, "well_formed.json")}
	c := newClient(t, transport)

	out, err := c.Parse(context.Background(), vision.ParseRequest{
		Image:         fakeImage,
		ResizedTo:     [2]int{1568, 1045},
		OriginalBytes: 4_128_719,
	})
	require.NoError(t, err)
	assert.Equal(t, "Grilled chicken caesar salad", out.Name)
	assert.Equal(t, 165.0, out.NutrimentsPer100g.Kcal)
	assert.Equal(t, 0.85, out.Confidence)
	assert.Equal(t, "claude-sonnet-4-6", out.Model)
	assert.Equal(t, 1240, out.InputTokens)
	assert.Equal(t, 89, out.OutputTokens)
	assert.Equal(t, [2]int{1568, 1045}, out.ResizedTo, "ResizedTo echoes the request")
	assert.Equal(t, 4_128_719, out.OriginalBytes)
	assert.Equal(t, 1, transport.callCount, "single-shot success makes one call")
}

func TestParse_LowConfidence_NullableMicrosPreserved(t *testing.T) {
	transport := &stubTransport{status: 200, body: loadFixture(t, "low_confidence.json")}
	c := newClient(t, transport)

	out, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	require.NoError(t, err)
	assert.Equal(t, 0.4, out.Confidence)
	assert.Equal(t, "Unidentified casserole", out.Name)
	assert.Nil(t, out.NutrimentsPer100g.FiberG)
	assert.Nil(t, out.NutrimentsPer100g.SugarG)
	assert.Nil(t, out.NutrimentsPer100g.SaltG)
}

// ============================================================================
// Parse — request shape verification
// ============================================================================

func TestParse_RequestShape(t *testing.T) {
	transport := &stubTransport{status: 200, body: loadFixture(t, "well_formed.json")}
	c := newClient(t, transport)
	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(transport.lastReqBody, &req))
	assert.Equal(t, "claude-sonnet-4-6", req["model"])

	// tool_choice forces the report_meal tool.
	tc, ok := req["tool_choice"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tool", tc["type"])
	assert.Equal(t, "report_meal", tc["name"])

	// tools[] contains exactly the report_meal definition.
	tools, ok := req["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	t0 := tools[0].(map[string]any)
	assert.Equal(t, "report_meal", t0["name"])

	// First content block is the image; media_type is image/jpeg.
	msgs, ok := req["messages"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 1)
	content := msgs[0].(map[string]any)["content"].([]any)
	imageBlock := content[0].(map[string]any)
	assert.Equal(t, "image", imageBlock["type"])
	source := imageBlock["source"].(map[string]any)
	assert.Equal(t, "base64", source["type"])
	assert.Equal(t, "image/jpeg", source["media_type"])
	assert.NotEmpty(t, source["data"], "image bytes must be base64-encoded")
}

// ============================================================================
// Parse — retry path on missing tool_use
// ============================================================================

func TestParse_RetriesOnceWhenToolUseMissing(t *testing.T) {
	wellFormed := loadFixture(t, "well_formed.json")
	missing := loadFixture(t, "missing_tool_use.json")

	transport := &stubTransport{status: 200, body: missing}
	transport.onCall = func(call int) {
		if call == 2 {
			// Second call: model finally complies.
			transport.body = wellFormed
		}
	}
	c := newClient(t, transport)

	out, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	require.NoError(t, err)
	assert.Equal(t, "Grilled chicken caesar salad", out.Name)
	assert.Equal(t, 2, transport.callCount, "one initial + one retry")
}

func TestParse_ReturnsUnparseableAfterRetryStillFails(t *testing.T) {
	missing := loadFixture(t, "missing_tool_use.json")
	transport := &stubTransport{status: 200, body: missing}
	c := newClient(t, transport)

	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	require.Error(t, err)
	assert.ErrorIs(t, err, vision.ErrVisionResponseUnparseable)
	assert.Equal(t, 2, transport.callCount, "tried once, retried once, then gave up")
}

// ============================================================================
// Parse — error mapping
// ============================================================================

func TestParse_RateLimit_ReturnsTypedErrorWithRetryAfter(t *testing.T) {
	transport := &stubTransport{
		status:     429,
		body:       loadFixture(t, "rate_limit.json"),
		retryAfter: "42",
	}
	c := newClient(t, transport)

	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	require.Error(t, err)
	var rl *vision.ErrVisionRateLimited
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, 42, rl.RetryAfterSeconds)
}

func TestParse_RateLimit_MissingHeaderDefaultsToZero(t *testing.T) {
	transport := &stubTransport{status: 429, body: loadFixture(t, "rate_limit.json")}
	c := newClient(t, transport)

	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	var rl *vision.ErrVisionRateLimited
	require.True(t, errors.As(err, &rl))
	assert.Equal(t, 0, rl.RetryAfterSeconds, "missing Retry-After -> 0, client backs off via its own policy")
}

func TestParse_5xx_MapsToUpstreamError(t *testing.T) {
	transport := &stubTransport{status: 503, body: loadFixture(t, "server_error.json")}
	c := newClient(t, transport)
	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	assert.ErrorIs(t, err, vision.ErrVisionUpstreamError)
}

func TestParse_Other4xx_MapsToUnexpectedResponse(t *testing.T) {
	transport := &stubTransport{status: 401, body: []byte(`{"type":"error"}`)}
	c := newClient(t, transport)
	_, err := c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	var ur *vision.ErrVisionUnexpectedResponse
	require.True(t, errors.As(err, &ur))
	assert.Equal(t, 401, ur.StatusCode)
}

func TestParse_ContextDeadlineMapsToTimeout(t *testing.T) {
	// httptest.NewServer that hangs forever; client deadline trips.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	c, err := vision.New(vision.Config{APIKey: "test", BaseURL: srv.URL, Timeout: 100 * time.Millisecond})
	require.NoError(t, err)
	_, err = c.Parse(context.Background(), vision.ParseRequest{Image: fakeImage})
	assert.ErrorIs(t, err, vision.ErrVisionTimeout)
}

// ============================================================================
// IsVisionError
// ============================================================================

func TestIsVisionError(t *testing.T) {
	assert.False(t, vision.IsVisionError(nil))
	assert.True(t, vision.IsVisionError(vision.ErrVisionTimeout))
	assert.True(t, vision.IsVisionError(vision.ErrAPIKeyMissing))
	assert.True(t, vision.IsVisionError(&vision.ErrVisionRateLimited{RetryAfterSeconds: 10}))
	assert.True(t, vision.IsVisionError(&vision.ErrVisionUnexpectedResponse{StatusCode: 401}))
	assert.False(t, vision.IsVisionError(errors.New("some other thing")))
}
