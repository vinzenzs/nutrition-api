package workouts

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const (
	maxWindowDays = 92
	bulkMaxItems  = 100
)

// Handlers wires workouts endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/workouts", h.create)
	rg.POST("/workouts/bulk", h.bulkCreate)
	rg.GET("/workouts", h.list)
	rg.GET("/workouts/:id", h.get)
	rg.PATCH("/workouts/:id", h.patch)
	rg.DELETE("/workouts/:id", h.delete)
	rg.POST("/workouts/:id/fulfill", h.fulfill)
	rg.POST("/workouts/:id/unfulfill", h.unfulfill)
}

// createRequest mirrors the POST /workouts body shape. All nullable columns
// arrive as pointers so we can distinguish "absent" from "explicit zero".
type createRequest struct {
	ExternalID *string  `json:"external_id,omitempty"`
	Source     string   `json:"source"`
	Sport      string   `json:"sport"`
	Status     string   `json:"status,omitempty"`
	Name       *string  `json:"name,omitempty"`
	StartedAt  string   `json:"started_at"`
	EndedAt    string   `json:"ended_at"`
	KcalBurned *float64 `json:"kcal_burned,omitempty"`
	AvgHR      *int     `json:"avg_hr,omitempty"`
	TSS        *float64 `json:"tss,omitempty"`
	// RPE + GI use RawMessage so non-integer payloads (`"seven"`, `7.5`) can
	// be rejected with the precise `rpe_invalid` / `gi_distress_score_invalid`
	// error codes the spec requires — a plain *int would surface `invalid_json`
	// and lose the field-specific diagnostic.
	RPE             json.RawMessage `json:"rpe,omitempty"`
	GIDistressScore json.RawMessage `json:"gi_distress_score,omitempty"`
	// Ingestion metrics. avg_power_w uses RawMessage for the same reason as
	// rpe/gi: a non-integer payload must surface `avg_power_w_invalid`, not
	// `invalid_json`. The float fields tolerate integer JSON natively.
	DistanceM    *float64        `json:"distance_m,omitempty"`
	AvgPowerW    json.RawMessage `json:"avg_power_w,omitempty"`
	TemperatureC *float64        `json:"temperature_c,omitempty"`
	SweatLossML  *float64        `json:"sweat_loss_ml,omitempty"`
	SessionGroup *string         `json:"session_group,omitempty"`
	Notes        *string         `json:"notes,omitempty"`

	// Per-activity detail (per add-garmin-workout-detail). Scalar performance +
	// weather + fixed HR-zone columns ride flat on the body; splits/sets are
	// nested arrays written as child rows. A nil Splits/Sets (key absent) means
	// "leave children untouched on re-sync"; a present array (even empty) is the
	// authoritative replacement set.
	ElevationGainM   *float64       `json:"elevation_gain_m,omitempty"`
	ElevationLossM   *float64       `json:"elevation_loss_m,omitempty"`
	NormalizedPowerW *int           `json:"normalized_power_w,omitempty"`
	IntensityFactor  *float64       `json:"intensity_factor,omitempty"`
	AvgCadence       *int           `json:"avg_cadence,omitempty"`
	AvgStrideM       *float64       `json:"avg_stride_m,omitempty"`
	MaxHR            *int           `json:"max_hr,omitempty"`
	AerobicTE        *float64       `json:"aerobic_te,omitempty"`
	AnaerobicTE      *float64       `json:"anaerobic_te,omitempty"`
	HumidityPct      *float64       `json:"humidity_pct,omitempty"`
	WindSpeedMPS     *float64       `json:"wind_speed_mps,omitempty"`
	SecsInZone1      *int           `json:"secs_in_zone_1,omitempty"`
	SecsInZone2      *int           `json:"secs_in_zone_2,omitempty"`
	SecsInZone3      *int           `json:"secs_in_zone_3,omitempty"`
	SecsInZone4      *int           `json:"secs_in_zone_4,omitempty"`
	SecsInZone5      *int           `json:"secs_in_zone_5,omitempty"`
	Splits           []splitRequest `json:"splits,omitempty"`
	Sets             []setRequest   `json:"sets,omitempty"`
}

// splitRequest / setRequest mirror the nested detail arrays on POST/bulk items.
type splitRequest struct {
	SplitIndex     int      `json:"split_index"`
	DistanceM      *float64 `json:"distance_m,omitempty"`
	DurationS      *float64 `json:"duration_s,omitempty"`
	AvgHR          *int     `json:"avg_hr,omitempty"`
	AvgPowerW      *int     `json:"avg_power_w,omitempty"`
	AvgSpeedMPS    *float64 `json:"avg_speed_mps,omitempty"`
	ElevationGainM *float64 `json:"elevation_gain_m,omitempty"`
}

type setRequest struct {
	SetIndex         int      `json:"set_index"`
	ExerciseName     *string  `json:"exercise_name,omitempty"`
	ExerciseCategory *string  `json:"exercise_category,omitempty"`
	Reps             *int     `json:"reps,omitempty"`
	WeightKg         *float64 `json:"weight_kg,omitempty"`
	DurationS        *float64 `json:"duration_s,omitempty"`
}

// create godoc
// @Summary      Upsert a workout
// @Description  Creates a new workout when external_id is absent or unseen; updates the existing row when external_id matches an existing workout. Garmin re-syncs land here without the writer needing to track sync state. Optional nested `splits` / `sets` arrays are written as child rows in the same transaction and fully replaced on an external_id re-sync.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key (writers normally rely on external_id instead)"
// @Param        body             body    createRequest  true   "Workout"
// @Success      201  {object}  Workout  "INSERT"
// @Success      200  {object}  Workout  "UPDATE (external_id collision)"
// @Failure      400  {object}  map[string]string  "source_invalid | sport_invalid | window_invalid | started_at_too_far_future | kcal_burned_invalid | avg_hr_invalid | tss_invalid | distance_m_invalid | avg_power_w_invalid | temperature_c_invalid | sweat_loss_ml_invalid | session_group_invalid | status_invalid | split_invalid | set_invalid"
// @Security     BearerAuth
// @Router       /workouts [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	in, errCode := buildCreateInput(req)
	if errCode != "" {
		switch errCode {
		case "rpe_invalid":
			respondRangeError(c, "rpe_invalid", RPEMin, RPEMax)
		case "gi_distress_score_invalid":
			respondRangeError(c, "gi_distress_score_invalid", GIDistressScoreMin, GIDistressScoreMax)
		default:
			respondError(c, http.StatusBadRequest, errCode)
		}
		return
	}
	w, created, err := h.svc.Upsert(c.Request.Context(), in)
	if err != nil {
		// rpe / gi_distress_score / temperature_c validation errors carry a
		// `range` hint; the others map to a plain error code.
		if errors.Is(err, ErrRPEInvalid) {
			respondRangeError(c, "rpe_invalid", RPEMin, RPEMax)
			return
		}
		if errors.Is(err, ErrGIDistressScoreInvalid) {
			respondRangeError(c, "gi_distress_score_invalid", GIDistressScoreMin, GIDistressScoreMax)
			return
		}
		if errors.Is(err, ErrTemperatureCInvalid) {
			respondRangeError(c, "temperature_c_invalid", TemperatureCMin, TemperatureCMax)
			return
		}
		respondServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	roundWorkoutDetail(w)
	c.JSON(status, w)
}

// bulkCreateRequest envelope. We decode in two passes so we can reject
// missing/non-array shape before decoding each item.
type bulkCreateRequest struct {
	Workouts []json.RawMessage `json:"workouts"`
}

type bulkItemSuccess struct {
	Index   int       `json:"index"`
	ID      uuid.UUID `json:"id"`
	Created bool      `json:"created"`
}

type bulkItemError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// bulkCreate godoc
// @Summary      Upsert a batch of workouts (max 100 per request)
// @Description  Each item upserts independently using external_id semantics, and may carry nested `splits` / `sets` arrays (written transactionally with the parent, replaced on re-sync). Per-item validation/persistence failures are reported in the results array; the overall response is 200 whenever the request body is well-formed and within the size cap. Partial failure is allowed.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        body  body  bulkCreateRequest  true  "{ workouts: [Workout, ...] }"
// @Success      200  {object}  map[string]interface{}  "{ results: [...] }"
// @Failure      400  {object}  map[string]interface{}  "bulk_invalid | bulk_empty | bulk_too_large"
// @Security     BearerAuth
// @Router       /workouts/bulk [post]
func (h *Handlers) bulkCreate(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		respondError(c, http.StatusBadRequest, "bulk_invalid")
		return
	}
	var req bulkCreateRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&req); err != nil || req.Workouts == nil {
		respondError(c, http.StatusBadRequest, "bulk_invalid")
		return
	}
	if len(req.Workouts) == 0 {
		respondError(c, http.StatusBadRequest, "bulk_empty")
		return
	}
	if len(req.Workouts) > bulkMaxItems {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bulk_too_large", "max": bulkMaxItems})
		return
	}

	// Decode each item into a createRequest; per-item decode failures become
	// per-item errors with the same `invalid_json` code we use elsewhere.
	inputs := make([]CreateInput, 0, len(req.Workouts))
	decodeErrors := make(map[int]string, 0)
	for i, raw := range req.Workouts {
		var item createRequest
		if err := json.Unmarshal(raw, &item); err != nil {
			decodeErrors[i] = "invalid_json"
			// Push a zero-value placeholder so indices line up with BulkUpsert.
			inputs = append(inputs, CreateInput{})
			continue
		}
		in, errCode := buildCreateInput(item)
		if errCode != "" {
			decodeErrors[i] = errCode
			inputs = append(inputs, CreateInput{})
			continue
		}
		inputs = append(inputs, in)
	}

	// Run BulkUpsert for the inputs that decoded; we still pass all indices so
	// the service returns one result per item.
	rawResults := h.svc.BulkUpsert(c.Request.Context(), inputs)

	results := make([]any, 0, len(rawResults))
	for i, r := range rawResults {
		if code, ok := decodeErrors[i]; ok {
			results = append(results, bulkItemError{Index: i, Error: code})
			continue
		}
		if r.Err != nil {
			results = append(results, bulkItemError{Index: i, Error: errCodeFor(r.Err)})
			continue
		}
		results = append(results, bulkItemSuccess{Index: i, ID: r.ID, Created: r.Created})
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// list godoc
// @Summary      List workouts in a time window
// @Tags         workouts
// @Produce      json
// @Param        from           query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to             query  string  true   "Inclusive RFC3339 upper bound"
// @Param        session_group  query  string  false  "Narrow to the legs of one brick/multisport session (exact-match on session_group)"
// @Param        status         query  string  false  "Filter by lifecycle status: planned | completed"
// @Success      200  {object}  map[string]interface{}  "{ workouts: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /workouts [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	// Optional exact-match filters — the window stays mandatory, these compose
	// as additional AND-predicates. Empty string means "no filter".
	var sessionGroup *string
	if sg := c.Query("session_group"); sg != "" {
		sessionGroup = &sg
	}
	var status *string
	if st := c.Query("status"); st != "" {
		if !ValidStatus(st) {
			respondError(c, http.StatusBadRequest, "status_invalid")
			return
		}
		status = &st
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
	if to.Before(from) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Sub(from) > time.Duration(maxWindowDays)*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	rows, err := h.svc.ListWindow(c.Request.Context(), from, to, sessionGroup, status)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Workout, 0, len(rows))
	for _, w := range rows {
		// List carries scalar + zone fields (rounded) but never nested detail —
		// the list query does not load children, so Splits/Sets stay nil here.
		roundWorkoutDetail(w)
		out = append(out, w)
	}
	c.JSON(http.StatusOK, gin.H{"workouts": out})
}

// get godoc
// @Summary      Get a workout by id
// @Description  Returns the workout including scalar performance + HR-zone fields inline (when set) and the nested `splits` / `sets` detail arrays (each ordered by index; empty arrays omitted). List responses carry the scalar + zone fields but never the nested arrays.
// @Tags         workouts
// @Produce      json
// @Param        id   path  string  true  "Workout UUID"
// @Success      200  {object}  Workout
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	w, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	roundWorkoutDetail(w)
	c.JSON(http.StatusOK, w)
}

// patch godoc
// @Summary      Partially update a workout (mutable fields only)
// @Description  Accepts `name`, `notes`, `kcal_burned`, `avg_hr`, `tss`, `rpe`, `gi_distress_score`, `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, `session_group`. `source`, `external_id`, `sport`, `started_at`, `ended_at` are immutable — delete and re-create if those are wrong. Tri-state on the nullable fields: omit to leave unchanged, value to set, JSON null to clear.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Workout UUID"
// @Param        body  body  map[string]interface{}  true  "Mutable fields"
// @Success      200  {object}  Workout
// @Failure      400  {object}  map[string]string  "field_immutable | kcal_burned_invalid | avg_hr_invalid | tss_invalid | rpe_invalid | gi_distress_score_invalid | distance_m_invalid | avg_power_w_invalid | temperature_c_invalid | sweat_loss_ml_invalid | session_group_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}

	// Custom decode: walk the JSON object once, reject any immutable field name
	// before touching the mutable subset. DisallowUnknownFields on a struct
	// would conflate "immutable" with "unknown"; we want a precise
	// `field_immutable` error code per spec.
	if len(raw) > 0 {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		immutable := map[string]bool{
			"source":      true,
			"external_id": true,
			"sport":       true,
			"started_at":  true,
			"ended_at":    true,
		}
		mutable := map[string]bool{
			"name":              true,
			"notes":             true,
			"kcal_burned":       true,
			"avg_hr":            true,
			"tss":               true,
			"rpe":               true,
			"gi_distress_score": true,
			"distance_m":        true,
			"avg_power_w":       true,
			"temperature_c":     true,
			"sweat_loss_ml":     true,
			"session_group":     true,
			"status":            true,
		}
		for field := range probe {
			if immutable[field] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "field_immutable", "field": field})
				return
			}
			if !mutable[field] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "field_immutable", "field": field})
				return
			}
		}
	}

	var body struct {
		Name            *string         `json:"name,omitempty"`
		Notes           *string         `json:"notes,omitempty"`
		KcalBurned      *float64        `json:"kcal_burned,omitempty"`
		AvgHR           *int            `json:"avg_hr,omitempty"`
		TSS             *float64        `json:"tss,omitempty"`
		RPE             json.RawMessage `json:"rpe,omitempty"`
		GIDistressScore json.RawMessage `json:"gi_distress_score,omitempty"`
		DistanceM       json.RawMessage `json:"distance_m,omitempty"`
		AvgPowerW       json.RawMessage `json:"avg_power_w,omitempty"`
		TemperatureC    json.RawMessage `json:"temperature_c,omitempty"`
		SweatLossML     json.RawMessage `json:"sweat_loss_ml,omitempty"`
		SessionGroup    json.RawMessage `json:"session_group,omitempty"`
		Status          *string         `json:"status,omitempty"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := PatchInput{
		Name:       body.Name,
		Notes:      body.Notes,
		KcalBurned: body.KcalBurned,
		AvgHR:      body.AvgHR,
		TSS:        body.TSS,
		Status:     body.Status,
	}
	// Tri-state decode for rpe: raw == nil → absent (leave unchanged);
	// raw == "null" → ClearRPE=true; otherwise unmarshal to *int.
	if body.RPE != nil {
		if string(body.RPE) == "null" {
			in.ClearRPE = true
		} else {
			var v int
			if err := json.Unmarshal(body.RPE, &v); err != nil {
				respondRangeError(c, "rpe_invalid", RPEMin, RPEMax)
				return
			}
			in.RPE = &v
		}
	}
	if body.GIDistressScore != nil {
		if string(body.GIDistressScore) == "null" {
			in.ClearGIDistressScore = true
		} else {
			var v int
			if err := json.Unmarshal(body.GIDistressScore, &v); err != nil {
				respondRangeError(c, "gi_distress_score_invalid", GIDistressScoreMin, GIDistressScoreMax)
				return
			}
			in.GIDistressScore = &v
		}
	}
	// Ingestion-metric tri-state: same null-clears convention. A non-null
	// payload that fails to decode to the field's type surfaces the precise
	// per-field error code rather than `invalid_json`.
	if body.DistanceM != nil {
		if string(body.DistanceM) == "null" {
			in.ClearDistanceM = true
		} else {
			var v float64
			if err := json.Unmarshal(body.DistanceM, &v); err != nil {
				respondError(c, http.StatusBadRequest, "distance_m_invalid")
				return
			}
			in.DistanceM = &v
		}
	}
	if body.AvgPowerW != nil {
		if string(body.AvgPowerW) == "null" {
			in.ClearAvgPowerW = true
		} else {
			var v int
			if err := json.Unmarshal(body.AvgPowerW, &v); err != nil {
				respondError(c, http.StatusBadRequest, "avg_power_w_invalid")
				return
			}
			in.AvgPowerW = &v
		}
	}
	if body.TemperatureC != nil {
		if string(body.TemperatureC) == "null" {
			in.ClearTemperatureC = true
		} else {
			var v float64
			if err := json.Unmarshal(body.TemperatureC, &v); err != nil {
				respondRangeError(c, "temperature_c_invalid", TemperatureCMin, TemperatureCMax)
				return
			}
			in.TemperatureC = &v
		}
	}
	if body.SweatLossML != nil {
		if string(body.SweatLossML) == "null" {
			in.ClearSweatLossML = true
		} else {
			var v float64
			if err := json.Unmarshal(body.SweatLossML, &v); err != nil {
				respondError(c, http.StatusBadRequest, "sweat_loss_ml_invalid")
				return
			}
			in.SweatLossML = &v
		}
	}
	if body.SessionGroup != nil {
		if string(body.SessionGroup) == "null" {
			in.ClearSessionGroup = true
		} else {
			var v string
			if err := json.Unmarshal(body.SessionGroup, &v); err != nil {
				respondError(c, http.StatusBadRequest, "session_group_invalid")
				return
			}
			in.SessionGroup = &v
		}
	}
	w, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_not_found")
			return
		}
		if errors.Is(err, ErrRPEInvalid) {
			respondRangeError(c, "rpe_invalid", RPEMin, RPEMax)
			return
		}
		if errors.Is(err, ErrTemperatureCInvalid) {
			respondRangeError(c, "temperature_c_invalid", TemperatureCMin, TemperatureCMax)
			return
		}
		if errors.Is(err, ErrGIDistressScoreInvalid) {
			respondRangeError(c, "gi_distress_score_invalid", GIDistressScoreMin, GIDistressScoreMax)
			return
		}
		respondServiceError(c, err)
		return
	}
	roundWorkoutDetail(w)
	c.JSON(http.StatusOK, w)
}

// fulfillRequest is the POST /workouts/{id}/fulfill body.
type fulfillRequest struct {
	CompletedID string `json:"completed_id"`
}

// fulfill godoc
// @Summary      Fulfill a planned workout with a completed activity
// @Description  Merges an existing completed activity (`completed_id`) into the planned workout at `{id}`: copies the activity's external_id, source, and actual metrics onto the planned row, flips it to `completed`, removes the redundant standalone completed row, and clears any needs-link flag. The planned row survives so its `plan_slot_id` stays stable. Used for the ambiguous/cross-day cases auto-reconciliation declines.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        id    path  string          true  "Planned workout UUID (the merge target)"
// @Param        body  body  fulfillRequest  true  "{ completed_id }"
// @Success      200  {object}  Workout
// @Failure      400  {object}  map[string]string  "invalid_json | completed_id_invalid | planned_workout_required | completed_workout_required | sport_mismatch"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/fulfill [post]
func (h *Handlers) fulfill(c *gin.Context) {
	plannedID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	var req fulfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	completedID, err := uuid.Parse(req.CompletedID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "completed_id_invalid")
		return
	}
	w, err := h.svc.Fulfill(c.Request.Context(), plannedID, completedID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respondError(c, http.StatusNotFound, "workout_not_found")
		case errors.Is(err, ErrFulfillTargetNotPlanned):
			respondError(c, http.StatusBadRequest, "planned_workout_required")
		case errors.Is(err, ErrFulfillSourceNotCompleted):
			respondError(c, http.StatusBadRequest, "completed_workout_required")
		case errors.Is(err, ErrFulfillSportMismatch):
			respondError(c, http.StatusBadRequest, "sport_mismatch")
		default:
			respondError(c, http.StatusInternalServerError, "fulfill_failed")
		}
		return
	}
	roundWorkoutDetail(w)
	c.JSON(http.StatusOK, w)
}

// unfulfill godoc
// @Summary      Reverse a fulfilled workout back to planned
// @Description  Reverses a merge: clears the workout's external_id and actual metrics and restores `status='planned'`, retaining `template_id` and `plan_slot_id`. The activity is not re-fetched; the next Garmin sync re-imports it as a fresh standalone row. Errors unless the workout is a fulfilled planned row (completed with a plan_slot_id).
// @Tags         workouts
// @Produce      json
// @Param        id   path  string  true  "Workout UUID"
// @Success      200  {object}  Workout
// @Failure      400  {object}  map[string]string  "workout_not_fulfilled"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/unfulfill [post]
func (h *Handlers) unfulfill(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	w, err := h.svc.Unfulfill(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			respondError(c, http.StatusNotFound, "workout_not_found")
		case errors.Is(err, ErrNotFulfilled):
			respondError(c, http.StatusBadRequest, "workout_not_fulfilled")
		default:
			respondError(c, http.StatusInternalServerError, "unfulfill_failed")
		}
		return
	}
	roundWorkoutDetail(w)
	c.JSON(http.StatusOK, w)
}

// delete godoc
// @Summary      Delete a workout
// @Tags         workouts
// @Param        id   path  string  true  "Workout UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

// buildCreateInput parses a createRequest into a CreateInput. Returns the
// service-layer input plus an empty string on success; on failure it returns
// the API error code (e.g. "window_invalid") so the caller can respond 400.
func buildCreateInput(req createRequest) (CreateInput, string) {
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return CreateInput{}, "window_invalid"
	}
	endedAt, err := time.Parse(time.RFC3339, req.EndedAt)
	if err != nil {
		return CreateInput{}, "window_invalid"
	}
	if req.Source == "" {
		return CreateInput{}, "source_invalid"
	}
	if req.Sport == "" {
		return CreateInput{}, "sport_invalid"
	}
	in := CreateInput{
		ExternalID:       req.ExternalID,
		Source:           req.Source,
		Sport:            req.Sport,
		Status:           req.Status,
		Name:             req.Name,
		StartedAt:        startedAt,
		EndedAt:          endedAt,
		KcalBurned:       req.KcalBurned,
		AvgHR:            req.AvgHR,
		TSS:              req.TSS,
		DistanceM:        req.DistanceM,
		TemperatureC:     req.TemperatureC,
		SweatLossML:      req.SweatLossML,
		SessionGroup:     req.SessionGroup,
		Notes:            req.Notes,
		ElevationGainM:   req.ElevationGainM,
		ElevationLossM:   req.ElevationLossM,
		NormalizedPowerW: req.NormalizedPowerW,
		IntensityFactor:  req.IntensityFactor,
		AvgCadence:       req.AvgCadence,
		AvgStrideM:       req.AvgStrideM,
		MaxHR:            req.MaxHR,
		AerobicTE:        req.AerobicTE,
		AnaerobicTE:      req.AnaerobicTE,
		HumidityPct:      req.HumidityPct,
		WindSpeedMPS:     req.WindSpeedMPS,
		SecsInZone1:      req.SecsInZone1,
		SecsInZone2:      req.SecsInZone2,
		SecsInZone3:      req.SecsInZone3,
		SecsInZone4:      req.SecsInZone4,
		SecsInZone5:      req.SecsInZone5,
		Splits:           toSplits(req.Splits),
		Sets:             toSets(req.Sets),
	}
	// Parse RPE / GI as strict integers; non-integer payloads surface the
	// precise per-field error code rather than `invalid_json`.
	if req.RPE != nil {
		var v int
		if err := json.Unmarshal(req.RPE, &v); err != nil {
			return CreateInput{}, "rpe_invalid"
		}
		in.RPE = &v
	}
	if req.GIDistressScore != nil {
		var v int
		if err := json.Unmarshal(req.GIDistressScore, &v); err != nil {
			return CreateInput{}, "gi_distress_score_invalid"
		}
		in.GIDistressScore = &v
	}
	// avg_power_w is a strict integer (watts) — same rejection rule as avg_hr.
	if req.AvgPowerW != nil {
		var v int
		if err := json.Unmarshal(req.AvgPowerW, &v); err != nil {
			return CreateInput{}, "avg_power_w_invalid"
		}
		in.AvgPowerW = &v
	}
	return in, ""
}

// toSplits / toSets convert the request sub-types into the service types,
// preserving the nil-vs-empty distinction (nil slice → leave children untouched
// on re-sync; empty-but-present → authoritative replacement).
func toSplits(in []splitRequest) []Split {
	if in == nil {
		return nil
	}
	out := make([]Split, len(in))
	for i, s := range in {
		out[i] = Split{
			SplitIndex:     s.SplitIndex,
			DistanceM:      s.DistanceM,
			DurationS:      s.DurationS,
			AvgHR:          s.AvgHR,
			AvgPowerW:      s.AvgPowerW,
			AvgSpeedMPS:    s.AvgSpeedMPS,
			ElevationGainM: s.ElevationGainM,
		}
	}
	return out
}

func toSets(in []setRequest) []Set {
	if in == nil {
		return nil
	}
	out := make([]Set, len(in))
	for i, s := range in {
		out[i] = Set{
			SetIndex:         s.SetIndex,
			ExerciseName:     s.ExerciseName,
			ExerciseCategory: s.ExerciseCategory,
			Reps:             s.Reps,
			WeightKg:         s.WeightKg,
			DurationS:        s.DurationS,
		}
	}
	return out
}

// roundWorkoutDetail applies numfmt.Round1 to the new detail floats that are
// stored at one decimal place, at the response boundary (project-wide rounding
// rule). Fields deliberately stored at higher precision — intensity_factor (2dp),
// avg_stride_m (2dp), split avg_speed_mps (3dp), set weight_kg (2dp) — keep their
// native precision and are intentionally left untouched.
func roundWorkoutDetail(w *Workout) {
	if w == nil {
		return
	}
	w.ElevationGainM = numfmt.Round1Ptr(w.ElevationGainM)
	w.ElevationLossM = numfmt.Round1Ptr(w.ElevationLossM)
	w.AerobicTE = numfmt.Round1Ptr(w.AerobicTE)
	w.AnaerobicTE = numfmt.Round1Ptr(w.AnaerobicTE)
	w.HumidityPct = numfmt.Round1Ptr(w.HumidityPct)
	w.WindSpeedMPS = numfmt.Round1Ptr(w.WindSpeedMPS)
	for i := range w.Splits {
		w.Splits[i].DistanceM = numfmt.Round1Ptr(w.Splits[i].DistanceM)
		w.Splits[i].DurationS = numfmt.Round1Ptr(w.Splits[i].DurationS)
		w.Splits[i].ElevationGainM = numfmt.Round1Ptr(w.Splits[i].ElevationGainM)
	}
	for i := range w.Sets {
		w.Sets[i].DurationS = numfmt.Round1Ptr(w.Sets[i].DurationS)
	}
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	respondError(c, http.StatusBadRequest, errCodeFor(err))
}

// errCodeFor maps a service-layer validation sentinel to the documented API
// error code. Falls back to a generic write_failed for unknown errors.
func errCodeFor(err error) string {
	switch {
	case errors.Is(err, ErrSourceInvalid):
		return "source_invalid"
	case errors.Is(err, ErrSportInvalid):
		return "sport_invalid"
	case errors.Is(err, ErrWindowInvalid):
		return "window_invalid"
	case errors.Is(err, ErrStartedAtFarFuture):
		return "started_at_too_far_future"
	case errors.Is(err, ErrKcalBurnedInvalid):
		return "kcal_burned_invalid"
	case errors.Is(err, ErrAvgHRInvalid):
		return "avg_hr_invalid"
	case errors.Is(err, ErrTSSInvalid):
		return "tss_invalid"
	case errors.Is(err, ErrRPEInvalid):
		return "rpe_invalid"
	case errors.Is(err, ErrGIDistressScoreInvalid):
		return "gi_distress_score_invalid"
	case errors.Is(err, ErrDistanceMInvalid):
		return "distance_m_invalid"
	case errors.Is(err, ErrAvgPowerWInvalid):
		return "avg_power_w_invalid"
	case errors.Is(err, ErrTemperatureCInvalid):
		return "temperature_c_invalid"
	case errors.Is(err, ErrSweatLossMLInvalid):
		return "sweat_loss_ml_invalid"
	case errors.Is(err, ErrSessionGroupInvalid):
		return "session_group_invalid"
	case errors.Is(err, ErrStatusInvalid):
		return "status_invalid"
	case errors.Is(err, ErrSplitInvalid):
		return "split_invalid"
	case errors.Is(err, ErrSetInvalid):
		return "set_invalid"
	}
	if strings.Contains(err.Error(), "upsert") {
		return "write_failed"
	}
	return "write_failed"
}

// respondRangeError surfaces a 400 with the error code AND a `range: {min, max}`
// body so clients can render the rule without re-encoding it. Used for the
// rpe and gi_distress_score validation failures.
func respondRangeError(c *gin.Context, code string, min, max int) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": code,
		"range": gin.H{"min": min, "max": max},
	})
}
