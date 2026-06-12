package garminauth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/auth"
)

// contentType is the media type used for the opaque blob on the wire. The blob
// is bytes, not JSON — the backend stores and returns it verbatim.
const contentType = "application/octet-stream"

// maxBlobBytes caps the stored blob. A garth token blob is a few KB; this is a
// generous ceiling that still rejects accidental large uploads.
const maxBlobBytes = 64 * 1024

// Handlers wires the garmin token endpoints. `enabled` reflects whether
// GARMIN_API_TOKEN is configured; when false every route short-circuits with
// 503 garmin_disabled (there is no garmin identity to authorize).
type Handlers struct {
	svc     *Service
	enabled bool
}

func NewHandlers(svc *Service, enabled bool) *Handlers {
	return &Handlers{svc: svc, enabled: enabled}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.PUT("/garmin/token", h.put)
	rg.GET("/garmin/token", h.get)
	rg.DELETE("/garmin/token", h.delete)
}

// guard enforces the two access rules shared by every route: the integration
// must be configured (else 503), and only the garmin identity may proceed
// (else 403). Returns false when the request has been aborted.
func (h *Handlers) guard(c *gin.Context) bool {
	if !h.enabled {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return false
	}
	if auth.ClientFromContext(c) != auth.ClientGarmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

// put godoc
// @Summary      Store the garmin-bridge auth token blob
// @Description  Persists the opaque garth token blob (encrypted at rest), replacing any prior value. The body is treated as raw bytes and returned byte-identical on GET. Restricted to the `garmin` identity; returns 503 when the Garmin integration is unconfigured. `Idempotency-Key` is rejected on PUT (full-replace semantics).
// @Tags         garmin
// @Accept       octet-stream
// @Produce      json
// @Param        body  body  string  true  "Opaque token blob"
// @Success      204  "no content"
// @Failure      400  {object}  map[string]string  "garmin_token_empty | garmin_token_too_large"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/token [put]
func (h *Handlers) put(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	blob, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	if len(blob) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "garmin_token_empty"})
		return
	}
	if len(blob) > maxBlobBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "garmin_token_too_large"})
		return
	}
	if err := h.svc.Store(c.Request.Context(), blob); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// get godoc
// @Summary      Read the garmin-bridge auth token blob
// @Description  Returns the decrypted token blob byte-identical to what was stored. Restricted to the `garmin` identity; 404 when nothing is stored; 503 when the integration is unconfigured.
// @Tags         garmin
// @Produce      octet-stream
// @Success      200  {string}  string  "Opaque token blob"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Failure      404  {object}  map[string]string  "garmin_token_not_found"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/token [get]
func (h *Handlers) get(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	blob, err := h.svc.Get(c.Request.Context())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "garmin_token_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read_failed"})
		return
	}
	c.Data(http.StatusOK, contentType, blob)
}

// delete godoc
// @Summary      Clear the garmin-bridge auth token blob
// @Description  Removes the stored token, forcing the bridge to re-login. Restricted to the `garmin` identity; 404 when nothing is stored; 503 when the integration is unconfigured.
// @Tags         garmin
// @Produce      json
// @Success      204  "no content"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Failure      404  {object}  map[string]string  "garmin_token_not_found"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/token [delete]
func (h *Handlers) delete(c *gin.Context) {
	if !h.guard(c) {
		return
	}
	if err := h.svc.Delete(c.Request.Context()); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "garmin_token_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
