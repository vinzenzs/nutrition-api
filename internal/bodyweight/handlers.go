package bodyweight

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

const (
	maxWindowDays  = 92  // for /weight list (matches /meals, /hydration)
	maxTrendDays   = 366 // for /weight/trend (matches /goals/overrides list)
	defaultWindowDays = 7
	minWindowDays  = 1
	maxWindowDaysParam = 30
)

// Handlers wires the body-weight CRUD + trend endpoints.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/weight", h.create)
	rg.GET("/weight", h.list)
	rg.GET("/weight/trend", h.trend)
	rg.PATCH("/weight/:id", h.patch)
	rg.DELETE("/weight/:id", h.delete)
}

// ----- POST /weight -----

type createRequest struct {
	WeightKg   *float64 `json:"weight_kg"`
	LoggedAt   string   `json:"logged_at"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
	Note       *string  `json:"note,omitempty"`
}

// create godoc
// @Summary      Log a body-weight measurement
// @Description  Records a body weight (kg), optionally with body-fat %. Multiple measurements per day are allowed and smoothed by /weight/trend. Standard `Idempotency-Key` header supported.
// @Tags         body-weight
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Body-weight entry"
// @Success      201  {object}  Entry
// @Failure      400  {object}  map[string]string  "weight_kg_invalid | body_fat_pct_invalid | logged_at_invalid | logged_at_too_far_future | note_too_long"
// @Security     BearerAuth
// @Router       /weight [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if req.WeightKg == nil {
		respondError(c, http.StatusBadRequest, "weight_kg_invalid")
		return
	}
	ts, err := parseLoggedAt(req.LoggedAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "logged_at_invalid")
		return
	}
	e, err := h.svc.Create(c.Request.Context(), CreateInput{
		WeightKg:   *req.WeightKg,
		LoggedAt:   ts,
		BodyFatPct: req.BodyFatPct,
		Note:       req.Note,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, roundEntry(e))
}

// ----- GET /weight -----

// list godoc
// @Summary      List body-weight entries in a window
// @Tags         body-weight
// @Produce      json
// @Param        from  query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to    query  string  true   "Exclusive RFC3339 upper bound; max 92-day span"
// @Success      200  {object}  map[string]interface{}  "{ entries: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /weight [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if !from.Before(to) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Sub(from) > time.Duration(maxWindowDays)*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	entries, err := h.svc.List(c.Request.Context(), from, to)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Entry, 0, len(entries))
	for _, e := range entries {
		out = append(out, roundEntry(e))
	}
	c.JSON(http.StatusOK, gin.H{"entries": out})
}

// ----- PATCH /weight/:id -----

type patchRequest struct {
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
	LoggedAt   *string  `json:"logged_at,omitempty"`
	Note       *string  `json:"note,omitempty"`
}

// patch godoc
// @Summary      Partially update a body-weight entry
// @Tags         body-weight
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Body-weight entry UUID"
// @Param        body  body  patchRequest  true  "Fields to update"
// @Success      200  {object}  Entry
// @Failure      400  {object}  map[string]string  "weight_kg_invalid | body_fat_pct_invalid | logged_at_invalid | note_too_long"
// @Failure      404  {object}  map[string]string  "weight_not_found"
// @Security     BearerAuth
// @Router       /weight/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "weight_not_found")
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	var req patchRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := PatchInput{
		WeightKg:   req.WeightKg,
		BodyFatPct: req.BodyFatPct,
		Note:       req.Note,
	}
	if req.LoggedAt != nil {
		ts, err := time.Parse(time.RFC3339, *req.LoggedAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "logged_at_invalid")
			return
		}
		in.LoggedAt = &ts
	}
	e, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "weight_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, roundEntry(e))
}

// ----- DELETE /weight/:id -----

// delete godoc
// @Summary      Delete a body-weight entry
// @Tags         body-weight
// @Param        id   path  string  true  "Body-weight entry UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "weight_not_found"
// @Security     BearerAuth
// @Router       /weight/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "weight_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "weight_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- GET /weight/trend -----

// trend godoc
// @Summary      Rolling-average body-weight trend
// @Description  Returns one point per calendar date in [from, to], each carrying the trailing rolling average over `window_days` days and the `sample_count` that fed it. A `sample_count` of 0 means no data in the trailing window; the matching `rolling_avg_kg` is null.
// @Tags         body-weight
// @Produce      json
// @Param        from         query  string   true   "Inclusive start date YYYY-MM-DD"
// @Param        to           query  string   true   "Inclusive end date YYYY-MM-DD; max 366-day span"
// @Param        window_days  query  integer  false  "Trailing-window length in days (1..30, default 7)"
// @Param        tz           query  string   false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Trend
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | window_days_invalid | tz_invalid"
// @Security     BearerAuth
// @Router       /weight/trend [get]
func (h *Handlers) trend(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "range_required")
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	if from.After(to) {
		respondError(c, http.StatusBadRequest, "range_invalid")
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxTrendDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxTrendDays})
		return
	}

	windowDays := defaultWindowDays
	if s := c.Query("window_days"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < minWindowDays || v > maxWindowDaysParam {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "window_days_invalid",
				"range": gin.H{"min": minWindowDays, "max": maxWindowDaysParam},
			})
			return
		}
		windowDays = v
	}

	loc, err := h.resolveTZ(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "tz_invalid")
		return
	}

	tr, err := h.svc.TrendFor(c.Request.Context(), TrendParams{
		From:       from,
		To:         to,
		Loc:        loc,
		WindowDays: windowDays,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "trend_failed")
		return
	}
	c.JSON(http.StatusOK, tr)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWeightKgInvalid):
		respondError(c, http.StatusBadRequest, "weight_kg_invalid")
	case errors.Is(err, ErrBodyFatPctInvalid):
		respondError(c, http.StatusBadRequest, "body_fat_pct_invalid")
	case errors.Is(err, ErrLoggedAtFuture):
		respondError(c, http.StatusBadRequest, "logged_at_too_far_future")
	case errors.Is(err, ErrNoteTooLong):
		respondError(c, http.StatusBadRequest, "note_too_long")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}

func parseLoggedAt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	return time.Parse(time.RFC3339, s)
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
		h.logger.Warn("weight trend used default tz", "route", c.FullPath(), "default_tz", tz)
	}
	return time.LoadLocation(tz)
}

// roundEntry rounds weight_kg + body_fat_pct to 1dp at response time.
func roundEntry(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	out := *e
	out.WeightKg = numfmt.Round1(e.WeightKg)
	out.BodyFatPct = numfmt.Round1Ptr(e.BodyFatPct)
	return &out
}
