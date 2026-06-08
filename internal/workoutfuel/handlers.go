package workoutfuel

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

const maxWindowDays = 92

// Handlers wires workout-fuel endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/workout-fuel", h.create)
	rg.GET("/workout-fuel", h.list)
	rg.PATCH("/workout-fuel/:id", h.patch)
	rg.DELETE("/workout-fuel/:id", h.delete)
}

type createRequest struct {
	Name        string   `json:"name"`
	LoggedAt    string   `json:"logged_at"`
	QuantityMl  *float64 `json:"quantity_ml,omitempty"`
	CarbsG      *float64 `json:"carbs_g,omitempty"`
	SodiumMg    *float64 `json:"sodium_mg,omitempty"`
	PotassiumMg *float64 `json:"potassium_mg,omitempty"`
	CaffeineMg  *float64 `json:"caffeine_mg,omitempty"`
	Note        *string  `json:"note,omitempty"`
	WorkoutID   *string  `json:"workout_id,omitempty"`
}

// create godoc
// @Summary      Log a workout-fuel entry
// @Description  Records an in-session fueling event — gel, electrolyte drink, salt tab, caffeine pill. At least one of quantity_ml/carbs_g/sodium_mg/potassium_mg/caffeine_mg MUST be set. `caffeine_mg: 0` means "measured, no caffeine"; omitting means "not measured" (NULL). Plain water belongs in /hydration; anything with electrolytes/carbs/caffeine belongs here.
// @Tags         workout-fuel
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Workout-fuel entry"
// @Success      201  {object}  Entry
// @Failure      400  {object}  map[string]string  "name_required | empty_entry | quantity_ml_invalid | carbs_g_invalid | sodium_mg_invalid | potassium_mg_invalid | caffeine_mg_invalid | logged_at_invalid | logged_at_too_far_future | note_too_long | workout_id_invalid | workout_not_found"
// @Security     BearerAuth
// @Router       /workout-fuel [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	ts, err := parseLoggedAt(req.LoggedAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "logged_at_invalid")
		return
	}
	in := CreateInput{
		Name:        req.Name,
		LoggedAt:    ts,
		QuantityMl:  req.QuantityMl,
		CarbsG:      req.CarbsG,
		SodiumMg:    req.SodiumMg,
		PotassiumMg: req.PotassiumMg,
		CaffeineMg:  req.CaffeineMg,
		Note:        req.Note,
	}
	if req.WorkoutID != nil && *req.WorkoutID != "" {
		wid, err := uuid.Parse(*req.WorkoutID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "workout_id_invalid")
			return
		}
		in.WorkoutID = &wid
	}
	e, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, roundEntry(e))
}

// list godoc
// @Summary      List workout-fuel entries in a window
// @Tags         workout-fuel
// @Produce      json
// @Param        from  query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to    query  string  true   "Exclusive RFC3339 upper bound"
// @Success      200  {object}  map[string]interface{}  "{ entries: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /workout-fuel [get]
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

// patch godoc
// @Summary      Partially update a workout-fuel entry
// @Description  Any field may be sent. For nullable fields, an explicit JSON `null` clears the column (sets it to NULL); omitting the field leaves it untouched. `workout_id` additionally honours the empty-string clear sentinel ("") for consistency with meals/hydration. PATCH that would leave all five quantitative fields null is rejected as `empty_entry`.
// @Tags         workout-fuel
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Workout-fuel entry UUID"
// @Param        body  body  object  true  "Fields to update (any subset)"
// @Success      200  {object}  Entry
// @Failure      400  {object}  map[string]string  "name_required | empty_entry | quantity_ml_invalid | carbs_g_invalid | sodium_mg_invalid | potassium_mg_invalid | caffeine_mg_invalid | logged_at_invalid | logged_at_too_far_future | note_too_long | workout_id_invalid | workout_not_found"
// @Failure      404  {object}  map[string]string  "workout_fuel_not_found"
// @Security     BearerAuth
// @Router       /workout-fuel/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_fuel_not_found")
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	// Decode the body twice: once as a raw key→raw-message map so we can
	// distinguish "field absent" from "field present with null value", and
	// once into a typed struct for the non-null values. Standard struct
	// decoding collapses both into a nil pointer, which loses the clear
	// semantic this endpoint requires.
	in := PatchInput{}
	if len(raw) > 0 {
		var fields map[string]json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(raw))
		if err := dec.Decode(&fields); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		if errCode, ok := h.applyPatchFields(fields, &in); !ok {
			respondError(c, http.StatusBadRequest, errCode)
			return
		}
	}
	e, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_fuel_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, roundEntry(e))
}

// applyPatchFields walks the parsed JSON body and fills the PatchInput,
// distinguishing "field absent" (leave alone), "field: null" (clear), and
// "field: value" (set). Returns a (code, false) pair on parse failure.
func (h *Handlers) applyPatchFields(fields map[string]json.RawMessage, in *PatchInput) (string, bool) {
	isNull := func(raw json.RawMessage) bool {
		return string(bytes.TrimSpace(raw)) == "null"
	}

	if v, ok := fields["name"]; ok && !isNull(v) {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "invalid_json", false
		}
		in.Name = &s
	}
	if v, ok := fields["logged_at"]; ok && !isNull(v) {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "logged_at_invalid", false
		}
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return "logged_at_invalid", false
		}
		in.LoggedAt = &ts
	}
	if v, ok := fields["quantity_ml"]; ok {
		if isNull(v) {
			in.ClearQuantityMl = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return "quantity_ml_invalid", false
			}
			in.QuantityMl = &f
		}
	}
	if v, ok := fields["carbs_g"]; ok {
		if isNull(v) {
			in.ClearCarbsG = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return "carbs_g_invalid", false
			}
			in.CarbsG = &f
		}
	}
	if v, ok := fields["sodium_mg"]; ok {
		if isNull(v) {
			in.ClearSodiumMg = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return "sodium_mg_invalid", false
			}
			in.SodiumMg = &f
		}
	}
	if v, ok := fields["potassium_mg"]; ok {
		if isNull(v) {
			in.ClearPotassiumMg = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return "potassium_mg_invalid", false
			}
			in.PotassiumMg = &f
		}
	}
	if v, ok := fields["caffeine_mg"]; ok {
		if isNull(v) {
			in.ClearCaffeineMg = true
		} else {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return "caffeine_mg_invalid", false
			}
			in.CaffeineMg = &f
		}
	}
	if v, ok := fields["note"]; ok {
		if isNull(v) {
			in.ClearNote = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return "invalid_json", false
			}
			in.Note = &s
		}
	}
	if v, ok := fields["workout_id"]; ok {
		if isNull(v) {
			in.ClearWorkoutID = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return "workout_id_invalid", false
			}
			if s == "" {
				in.ClearWorkoutID = true
			} else {
				wid, err := uuid.Parse(s)
				if err != nil {
					return "workout_id_invalid", false
				}
				in.WorkoutID = &wid
			}
		}
	}
	return "", true
}

// delete godoc
// @Summary      Delete a workout-fuel entry
// @Tags         workout-fuel
// @Param        id   path  string  true  "Workout-fuel entry UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "workout_fuel_not_found"
// @Security     BearerAuth
// @Router       /workout-fuel/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "workout_fuel_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "workout_fuel_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWorkoutNotFound):
		respondError(c, http.StatusBadRequest, "workout_not_found")
	case errors.Is(err, ErrNameRequired):
		respondError(c, http.StatusBadRequest, "name_required")
	case errors.Is(err, ErrEmptyEntry):
		respondError(c, http.StatusBadRequest, "empty_entry")
	case errors.Is(err, ErrQuantityInvalid):
		respondError(c, http.StatusBadRequest, "quantity_ml_invalid")
	case errors.Is(err, ErrCarbsInvalid):
		respondError(c, http.StatusBadRequest, "carbs_g_invalid")
	case errors.Is(err, ErrSodiumInvalid):
		respondError(c, http.StatusBadRequest, "sodium_mg_invalid")
	case errors.Is(err, ErrPotassInvalid):
		respondError(c, http.StatusBadRequest, "potassium_mg_invalid")
	case errors.Is(err, ErrCaffeineInvalid):
		respondError(c, http.StatusBadRequest, "caffeine_mg_invalid")
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

// roundEntry returns a copy with all quantitative fields rounded to 1dp.
func roundEntry(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	out := *e
	out.QuantityMl = numfmt.Round1Ptr(e.QuantityMl)
	out.CarbsG = numfmt.Round1Ptr(e.CarbsG)
	out.SodiumMg = numfmt.Round1Ptr(e.SodiumMg)
	out.PotassiumMg = numfmt.Round1Ptr(e.PotassiumMg)
	out.CaffeineMg = numfmt.Round1Ptr(e.CaffeineMg)
	return &out
}
