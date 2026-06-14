package devices

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Handlers wires the devices endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/devices", h.upsert)
	rg.GET("/devices", h.list)
	rg.GET("/devices/:id", h.get)
}

type upsertRequest struct {
	ExternalID      string     `json:"external_id"`
	DisplayName     string     `json:"display_name"`
	Model           *string    `json:"model,omitempty"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	BatteryPct      *float64   `json:"battery_pct,omitempty"`
	FirmwareVersion *string    `json:"firmware_version,omitempty"`
}

// upsert godoc
// @Summary      Upsert a device record (by Garmin device id)
// @Description  Creates or updates a paired Garmin device, upserting by `external_id`. Slowly-changing inventory — a fresh sync advances last_sync_at/battery. Reference context only; never feeds nutrition computation.
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    upsertRequest  true   "Device record"
// @Success      201  {object}  Device  "INSERT"
// @Success      200  {object}  Device  "UPDATE (external_id already present)"
// @Failure      400  {object}  map[string]string  "external_id_required | display_name_required | battery_pct_invalid"
// @Security     BearerAuth
// @Router       /devices [post]
func (h *Handlers) upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	d := &Device{
		ExternalID:      req.ExternalID,
		DisplayName:     req.DisplayName,
		Model:           req.Model,
		LastSyncAt:      req.LastSyncAt,
		BatteryPct:      req.BatteryPct,
		FirmwareVersion: req.FirmwareVersion,
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), d)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, round(out))
}

// list godoc
// @Summary      List device records
// @Tags         devices
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{ devices: [...] }"
// @Security     BearerAuth
// @Router       /devices [get]
func (h *Handlers) list(c *gin.Context) {
	rows, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Device, 0, len(rows))
	for _, d := range rows {
		out = append(out, round(d))
	}
	c.JSON(http.StatusOK, gin.H{"devices": out})
}

// get godoc
// @Summary      Get a device record by id
// @Tags         devices
// @Produce      json
// @Param        id   path  string  true  "Device UUID"
// @Success      200  {object}  Device
// @Failure      404  {object}  map[string]string  "device_not_found"
// @Security     BearerAuth
// @Router       /devices/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "device_not_found")
		return
	}
	out, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "device_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, round(out))
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{ErrExternalIDRequired, ErrDisplayNameRequired, ErrBatteryPctInvalid} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to battery_pct at the boundary.
func round(d *Device) *Device {
	if d == nil {
		return nil
	}
	out := *d
	out.BatteryPct = numfmt.Round1Ptr(d.BatteryPct)
	return &out
}
