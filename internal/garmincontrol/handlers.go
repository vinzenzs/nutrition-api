// Package garmincontrol proxies the garmin-bridge's interactive login to
// authenticated REST callers (per add-garmin-mcp-login). The MCP server is a
// REST-API client that generally cannot reach the bridge's internal ClusterIP,
// so the backend — which can — forwards `POST /garmin/login` and
// `POST /garmin/login/mfa` to the bridge at GARMIN_BRIDGE_URL and returns the
// bridge's status + body verbatim.
//
// This package owns NO Garmin or token logic: it forwards bytes. The Garmin
// password never transits this path (it lives only in the bridge's Secret);
// only the ephemeral 6-digit MFA code passes through. Nothing here logs request
// or response bodies, so neither the code nor any bridge-minted value is logged.
package garmincontrol

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// forwardTimeout bounds the proxied call. The bridge's SSO handshake can take a
// few seconds (and MFA submission completes the OAuth exchange), so this is
// generous but still bounded (design open question resolved here).
const forwardTimeout = 30 * time.Second

// maxBodyBytes caps both the forwarded request body and the bridge's response
// we read back. The MFA payload is a few bytes ({"code":"123456"}); the bridge
// responses are small JSON. A generous ceiling that rejects accidents.
const maxBodyBytes = 16 * 1024

// Handlers forward to the bridge. bridgeURL == "" means the integration is
// unconfigured and every route short-circuits with 503 garmin_disabled.
type Handlers struct {
	bridgeURL string
	client    *http.Client

	// Scheduling dependencies (per add-garmin-scheduling), wired via
	// SetSchedulingDeps. Nil until set — the login proxy needs none of them.
	workoutsRepo  workoutsRepo
	templatesRepo templatesRepo
	planSvc       planService
}

// NewHandlers builds the proxy. An empty bridgeURL disables it.
func NewHandlers(bridgeURL string) *Handlers {
	return &Handlers{
		bridgeURL: strings.TrimRight(strings.TrimSpace(bridgeURL), "/"),
		client:    &http.Client{Timeout: forwardTimeout},
	}
}

// SetSchedulingDeps wires the repos/service the scheduling endpoints need.
func (h *Handlers) SetSchedulingDeps(w workoutsRepo, t templatesRepo, p planService) {
	h.workoutsRepo = w
	h.templatesRepo = t
	h.planSvc = p
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/garmin/login", h.login)
	rg.POST("/garmin/login/mfa", h.loginMFA)

	rg.POST("/garmin/schedule/workout", h.scheduleWorkout)
	rg.DELETE("/garmin/schedule/workout/:id", h.unscheduleWorkout)
	rg.POST("/garmin/schedule/template", h.scheduleTemplate)
	rg.POST("/garmin/schedule/plan", h.schedulePlan)
	rg.GET("/garmin/calendar", h.calendar)

	// Workout-library management + blob export (garmin-workout-library-mgmt).
	rg.DELETE("/garmin/workout/:id", h.deleteWorkoutObject)
	rg.GET("/garmin/workouts", h.listGarminWorkouts)
	rg.GET("/garmin/workout/:id", h.getGarminWorkout)
	rg.POST("/garmin/hydration", h.pushHydration)
	rg.GET("/garmin/activity/:activity_id/export", h.exportActivity)

	// Activity-level control operations (add-garmin-misc-mirror).
	rg.GET("/garmin/activity/:activity_id/gear", h.activityGear)
	rg.GET("/garmin/workout/:id/download", h.downloadWorkout)
	rg.POST("/garmin/activity/upload", h.uploadActivity)
	rg.PATCH("/garmin/activity/:activity_id", h.renameActivity)
	rg.DELETE("/garmin/activity/:activity_id", h.deleteActivity)

	// History backfill (add-garmin-history-backfill).
	rg.POST("/garmin/backfill", h.backfill)
}

func (h *Handlers) enabled() bool { return h.bridgeURL != "" }

// login godoc
// @Summary      Start the Garmin bridge login (proxy)
// @Description  Forwards to the garmin-bridge's POST /login, returning its response verbatim (e.g. `{"needs_mfa": true}`). Carries NO credentials — the bridge reads them from its own configuration. Returns 503 garmin_disabled when GARMIN_BRIDGE_URL is unset. Any authenticated identity may call it.
// @Tags         garmin
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Bridge response verbatim (e.g. {needs_mfa:true} or {logged_in:true})"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/login [post]
func (h *Handlers) login(c *gin.Context) {
	// No body: POST /garmin/login carries no credentials (design D3).
	h.forward(c, "/login", nil)
}

// loginMFA godoc
// @Summary      Submit the Garmin bridge MFA code (proxy)
// @Description  Forwards the supplied 6-digit code to the garmin-bridge's POST /login/mfa, returning its success/error response verbatim. On success the bridge persists the minted token to the backend; it is never returned here. Returns 503 garmin_disabled when GARMIN_BRIDGE_URL is unset.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  map[string]string  true  "{ \"code\": \"123456\" }"
// @Success      200  {object}  map[string]interface{}  "Bridge response verbatim (e.g. {logged_in:true})"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/login/mfa [post]
func (h *Handlers) loginMFA(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	h.forward(c, "/login/mfa", body)
}

// forward POSTs to the bridge and copies its status + body back verbatim. It
// interprets nothing and logs no bodies. Transport failures surface as
// 502 garmin_bridge_unreachable so the agent can relay an actionable error.
func (h *Handlers) forward(c *gin.Context, bridgePath string, body []byte) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.bridgeURL+bridgePath, reader)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)
}
