package personalrecords

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// Handlers wires the personal-records endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/personal-records", h.upsert)
	rg.GET("/personal-records", h.list)
}

// upsertRequest mirrors the POST /personal-records body. external_id/pr_type/
// value/unit/achieved_at are required; activity_id is optional.
type upsertRequest struct {
	ExternalID string     `json:"external_id"`
	PRType     string     `json:"pr_type"`
	Value      *float64   `json:"value"`
	Unit       string     `json:"unit"`
	ActivityID *string    `json:"activity_id,omitempty"`
	AchievedAt *time.Time `json:"achieved_at"`
}

// upsert godoc
// @Summary      Upsert a personal record (by Garmin PR id)
// @Description  Creates or updates a personal record, upserting by `external_id` (the Garmin PR id). A beaten PR overwrites the prior value in place. Standard `Idempotency-Key` header supported. PR values are coaching context only; they never feed nutrition computation.
// @Tags         personal-records
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    upsertRequest  true   "Personal record"
// @Success      201  {object}  PersonalRecord  "INSERT"
// @Success      200  {object}  PersonalRecord  "UPDATE (external_id already present)"
// @Failure      400  {object}  map[string]string  "external_id_required | pr_type_required | value_invalid | unit_required | achieved_at_required"
// @Security     BearerAuth
// @Router       /personal-records [post]
func (h *Handlers) upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	pr := &PersonalRecord{
		ExternalID: req.ExternalID,
		PRType:     req.PRType,
		Value:      req.Value,
		Unit:       req.Unit,
		ActivityID: req.ActivityID,
	}
	if req.AchievedAt != nil {
		pr.AchievedAt = *req.AchievedAt
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), pr)
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
// @Summary      List personal records
// @Tags         personal-records
// @Produce      json
// @Param        pr_type  query  string  false  "Filter to a single PR type (e.g. 5k)"
// @Success      200  {object}  map[string]interface{}  "{ personal_records: [...] }"
// @Security     BearerAuth
// @Router       /personal-records [get]
func (h *Handlers) list(c *gin.Context) {
	var prType *string
	if v := c.Query("pr_type"); v != "" {
		prType = &v
	}
	rows, err := h.svc.List(c.Request.Context(), prType)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*PersonalRecord, 0, len(rows))
	for _, pr := range rows {
		out = append(out, round(pr))
	}
	c.JSON(http.StatusOK, gin.H{"personal_records": out})
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrExternalIDRequired, ErrPRTypeRequired, ErrValueInvalid,
		ErrUnitRequired, ErrAchievedAtRequired,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}

// round applies 1dp rounding to value at the boundary.
func round(pr *PersonalRecord) *PersonalRecord {
	if pr == nil {
		return nil
	}
	out := *pr
	out.Value = numfmt.Round1Ptr(pr.Value)
	return &out
}
