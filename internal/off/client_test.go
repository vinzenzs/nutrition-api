package off

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixturesDir = "../../testdata/off"

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, name))
	require.NoError(t, err, "read fixture %s", name)
	return b
}

// stubTransport routes requests by barcode → fixture body.
type stubTransport struct {
	// byBarcode maps URL path containing a barcode → response body + status.
	responses map[string]stubResponse
	calls     int
}

type stubResponse struct {
	status int
	body   []byte
	err    error
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	path := req.URL.Path
	for key, r := range s.responses {
		if strings.Contains(path, key) {
			if r.err != nil {
				return nil, r.err
			}
			return &http.Response{
				StatusCode: r.status,
				Body:       io.NopCloser(bytes.NewReader(r.body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	}
	return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}}, nil
}

func newTestClient(t *testing.T, transport *stubTransport, logger *slog.Logger) *Client {
	t.Helper()
	c, err := New(Config{HTTPClient: &http.Client{Transport: transport}}, logger)
	require.NoError(t, err)
	return c
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestFetch_FullyPopulatedProduct(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"3017624010701": {status: 200, body: loadFixture(t, "3017624010701.json")},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	p, err := c.Fetch(context.Background(), "3017624010701")
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "3017624010701", p.Barcode)
	assert.Equal(t, "Nutella", p.Name)
	assert.Equal(t, "Ferrero", p.Brand)
	require.NotNil(t, p.ServingSizeG)
	assert.InDelta(t, 15.0, *p.ServingSizeG, 0.001)
	require.NotNil(t, p.Nutriments.KcalPer100g)
	assert.InDelta(t, 539.0, *p.Nutriments.KcalPer100g, 0.001)
	require.NotNil(t, p.Nutriments.ProteinGPer100g)
	assert.InDelta(t, 6.3, *p.Nutriments.ProteinGPer100g, 0.001)
	require.NotNil(t, p.Nutriments.SaltGPer100g)
	assert.InDelta(t, 0.107, *p.Nutriments.SaltGPer100g, 0.001)
	assert.NotEmpty(t, p.RawPayload, "raw payload should be preserved")
}

func TestFetch_KjOnlyDerivesKcal(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"2222222222222": {status: 200, body: loadFixture(t, "kj_only.json")},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	p, err := c.Fetch(context.Background(), "2222222222222")
	require.NoError(t, err)
	require.NotNil(t, p.Nutriments.KcalPer100g)
	// 418.4 kJ / 4.184 = 100.0 kcal
	assert.InDelta(t, 100.0, *p.Nutriments.KcalPer100g, 0.1)
}

func TestFetch_MissingNutrimentsAreNil(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"1111111111111": {status: 200, body: loadFixture(t, "missing_nutriments.json")},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	p, err := c.Fetch(context.Background(), "1111111111111")
	require.NoError(t, err)

	require.NotNil(t, p.Nutriments.KcalPer100g)
	require.NotNil(t, p.Nutriments.ProteinGPer100g)
	assert.Nil(t, p.Nutriments.CarbsGPer100g)
	assert.Nil(t, p.Nutriments.FatGPer100g)
	assert.Nil(t, p.Nutriments.FiberGPer100g)
	assert.Nil(t, p.Nutriments.SugarGPer100g)
	assert.Nil(t, p.Nutriments.SaltGPer100g)

	// Micros are absent from this fixture entirely — must be nil, not zero.
	assert.Nil(t, p.Nutriments.IronMgPer100g)
	assert.Nil(t, p.Nutriments.CalciumMgPer100g)
	assert.Nil(t, p.Nutriments.VitaminDMcgPer100g)
	assert.Nil(t, p.Nutriments.VitaminB12McgPer100g)
	assert.Nil(t, p.Nutriments.VitaminCMgPer100g)
	assert.Nil(t, p.Nutriments.MagnesiumMgPer100g)
	assert.Nil(t, p.Nutriments.PotassiumMgPer100g)
	assert.Nil(t, p.Nutriments.ZincMgPer100g)
}

func TestFetch_FullyPopulatedMicros(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"5060337640008": {status: 200, body: loadFixture(t, "fully_populated_micros.json")},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	p, err := c.Fetch(context.Background(), "5060337640008")
	require.NoError(t, err)
	require.NotNil(t, p)

	// Macros sanity check — same path as the nutella fixture, just confirms
	// the parser still works for the macros side when micros are present too.
	require.NotNil(t, p.Nutriments.KcalPer100g)
	assert.InDelta(t, 48.0, *p.Nutriments.KcalPer100g, 0.001)
	require.NotNil(t, p.Nutriments.ProteinGPer100g)
	assert.InDelta(t, 1.5, *p.Nutriments.ProteinGPer100g, 0.001)

	// Each micro is extracted with the unit OFF reports it in (mg or mcg).
	require.NotNil(t, p.Nutriments.IronMgPer100g)
	assert.InDelta(t, 1.8, *p.Nutriments.IronMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.CalciumMgPer100g)
	assert.InDelta(t, 120.0, *p.Nutriments.CalciumMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.VitaminDMcgPer100g)
	assert.InDelta(t, 1.5, *p.Nutriments.VitaminDMcgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.VitaminB12McgPer100g)
	assert.InDelta(t, 0.38, *p.Nutriments.VitaminB12McgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.VitaminCMgPer100g)
	assert.InDelta(t, 7.5, *p.Nutriments.VitaminCMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.MagnesiumMgPer100g)
	assert.InDelta(t, 22.0, *p.Nutriments.MagnesiumMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.PotassiumMgPer100g)
	assert.InDelta(t, 175.0, *p.Nutriments.PotassiumMgPer100g, 0.001)
	require.NotNil(t, p.Nutriments.ZincMgPer100g)
	assert.InDelta(t, 0.8, *p.Nutriments.ZincMgPer100g, 0.001)
}

func TestFetch_UnparseableServingSizeIsTolerated(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"3333333333333": {status: 200, body: loadFixture(t, "unparseable_serving_size.json")},
		},
	}
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	c := newTestClient(t, stub, logger)
	p, err := c.Fetch(context.Background(), "3333333333333")
	require.NoError(t, err)
	assert.Nil(t, p.ServingSizeG, "unparseable serving_size should be nil")
	assert.Contains(t, logBuf.String(), "unparseable serving_size", "warning should be logged")
}

func TestFetch_StatusZeroReturnsProductNotFound(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"0000000000000": {status: 200, body: loadFixture(t, "not_found.json")},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	_, err := c.Fetch(context.Background(), "0000000000000")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestFetch_Timeout(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"anything": {err: &timeoutErr{}},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	_, err := c.Fetch(context.Background(), "anything")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUpstreamTimeout)
}

func TestFetch_5xxReturnsServerError(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"anything": {status: 503, body: []byte(`upstream busy`)},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	_, err := c.Fetch(context.Background(), "anything")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUpstreamServerError)
}

func TestFetch_Unexpected4xxReturnsUnexpectedStatus(t *testing.T) {
	stub := &stubTransport{
		responses: map[string]stubResponse{
			"anything": {status: 403, body: []byte(`forbidden`)},
		},
	}
	c := newTestClient(t, stub, discardLogger())
	_, err := c.Fetch(context.Background(), "anything")
	require.Error(t, err)
	var u *UnexpectedStatusError
	require.True(t, errors.As(err, &u))
	assert.Equal(t, 403, u.StatusCode)
}

func TestFetch_UserAgentIsSet(t *testing.T) {
	var capturedUA string
	transport := &captureUATransport{ua: &capturedUA, body: loadFixture(t, "3017624010701.json")}
	c, err := New(Config{
		HTTPClient: &http.Client{Transport: transport},
		Contact:    "+test@example.com",
	}, discardLogger())
	require.NoError(t, err)
	_, _ = c.Fetch(context.Background(), "3017624010701")
	assert.Contains(t, capturedUA, "kazper/")
	assert.Contains(t, capturedUA, "+test@example.com")
}

func TestFetch_RealTimeoutFromContext(t *testing.T) {
	// Verify that a context deadline maps to ErrUpstreamTimeout via the real
	// http.Client path (not just our stubTransport shortcut).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			defer conn.Close()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	c, err := New(Config{
		BaseURL: "http://" + listener.Addr().String(),
		Timeout: 50 * time.Millisecond,
	}, discardLogger())
	require.NoError(t, err)
	_, err = c.Fetch(context.Background(), "anything")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUpstreamTimeout)
}

// timeoutErr satisfies net.Error with Timeout()=true, so the client maps it
// to ErrUpstreamTimeout regardless of the transport path.
type timeoutErr struct{}

func (*timeoutErr) Error() string   { return "i/o timeout" }
func (*timeoutErr) Timeout() bool   { return true }
func (*timeoutErr) Temporary() bool { return true }

type captureUATransport struct {
	ua   *string
	body []byte
}

func (t *captureUATransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.ua = req.Header.Get("User-Agent")
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(t.body)),
		Header:     http.Header{},
	}, nil
}
