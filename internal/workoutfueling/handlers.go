package workoutfueling

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

// Handlers wires the /workouts/:id/fueling endpoint onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/:id/fueling", h.fueling)
}

// fueling godoc
// @Summary      Workout fueling: pre / intra / post windows
// @Description  Returns three time-anchored buckets (pre / intra / post) of meal + hydration intake around a workout. Aggregation is by `logged_at` time-window matching, NOT by the `workout_id` tag on intake rows — an untagged meal in the pre-window still contributes. `nutrition` and `hydration` sub-objects are kept separate so units don't mix.
// @Tags         workouts
// @Produce      json
// @Param        id                path   string   true   "Workout UUID"
// @Param        pre_window_min    query  integer  false  "Pre-window length in minutes (default 240, range 0..720)"
// @Param        post_window_min   query  integer  false  "Post-window length in minutes (default 60, range 0..720)"
// @Success      200  {object}  WorkoutFueling
// @Failure      400  {object}  map[string]interface{}  "window_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/fueling [get]
func (h *Handlers) fueling(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}

	preMin := DefaultPreWindowMin
	if s := c.Query("pre_window_min"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < MinWindowMin || v > MaxWindowMin {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "window_invalid",
				"range": gin.H{"min": MinWindowMin, "max": MaxWindowMin},
			})
			return
		}
		preMin = v
	}
	postMin := DefaultPostWindowMin
	if s := c.Query("post_window_min"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < MinWindowMin || v > MaxWindowMin {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "window_invalid",
				"range": gin.H{"min": MinWindowMin, "max": MaxWindowMin},
			})
			return
		}
		postMin = v
	}

	out, err := h.svc.FueledFor(c.Request.Context(), id, preMin, postMin)
	if err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fueling_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}
