package garmincontrol_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/garmincontrol"
)

func init() { gin.SetMode(gin.TestMode) }

const agentToken = "agent-token-bbbbbbbbbbbbbbbb"

// recordingBridge is a stub of the garmin-bridge: it records the path + body it
// received and replies with a canned status + body to assert verbatim relay.
type recordingBridge struct {
	server   *httptest.Server
	gotPath  string
	gotBody  string
	respCode int
	respBody string
}

func newRecordingBridge(t *testing.T, code int, body string) *recordingBridge {
	t.Helper()
	b := &recordingBridge{respCode: code, respBody: body}
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		b.gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(b.respCode)
		_, _ = w.Write([]byte(b.respBody))
	}))
	t.Cleanup(b.server.Close)
	return b
}

// setup builds a router with auth middleware + the proxy pointed at bridgeURL
// ("" disables the integration).
func setup(bridgeURL string) *gin.Engine {
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: "mobile-token-aaaaaaaaaaaaaa", AgentToken: agentToken}))
	rg := r.Group("/")
	garmincontrol.NewHandlers(bridgeURL).Register(rg)
	return r
}

func doPost(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, rdr)
	req.Header.Set("Authorization", "Bearer "+agentToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLoginForwardsToBridgeVerbatim(t *testing.T) {
	bridge := newRecordingBridge(t, http.StatusOK, `{"needs_mfa":true}`)
	r := setup(bridge.server.URL)

	w := doPost(t, r, "/garmin/login", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"needs_mfa":true}`, w.Body.String())
	assert.Equal(t, "/login", bridge.gotPath)
	// Carries no credentials: the forwarded request body is empty.
	assert.Empty(t, bridge.gotBody)
}

func TestLoginMFAForwardsCodeVerbatim(t *testing.T) {
	bridge := newRecordingBridge(t, http.StatusOK, `{"logged_in":true}`)
	r := setup(bridge.server.URL)

	w := doPost(t, r, "/garmin/login/mfa", `{"code":"418923"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"logged_in":true}`, w.Body.String())
	assert.Equal(t, "/login/mfa", bridge.gotPath)
	assert.JSONEq(t, `{"code":"418923"}`, bridge.gotBody)
}

func TestBridgeErrorStatusRelayedVerbatim(t *testing.T) {
	// A 401 from the bridge (wrong MFA code) must pass through unchanged.
	bridge := newRecordingBridge(t, http.StatusUnauthorized, `{"error":"mfa_invalid"}`)
	r := setup(bridge.server.URL)

	w := doPost(t, r, "/garmin/login/mfa", `{"code":"000000"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":"mfa_invalid"}`, w.Body.String())
}

func TestDisabledWhenBridgeURLUnset(t *testing.T) {
	r := setup("")

	for _, path := range []string{"/garmin/login", "/garmin/login/mfa"} {
		w := doPost(t, r, path, "")
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, path)
		assert.JSONEq(t, `{"error":"garmin_disabled"}`, w.Body.String(), path)
	}
}

func TestUnreachableBridgeSurfaces502(t *testing.T) {
	// Point at a closed server: the proxy must surface a typed 502.
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := closed.URL
	closed.Close()

	r := setup(url)
	w := doPost(t, r, "/garmin/login", "")
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"garmin_bridge_unreachable"}`, w.Body.String())
}

func TestRequiresAuth(t *testing.T) {
	bridge := newRecordingBridge(t, http.StatusOK, `{}`)
	r := setup(bridge.server.URL)

	req := httptest.NewRequest(http.MethodPost, "/garmin/login", nil)
	// no Authorization header
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, bridge.gotPath, "must not forward to the bridge unauthenticated")
}
