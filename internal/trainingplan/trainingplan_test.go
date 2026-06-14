package trainingplan_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	g := r.Group("/")
	tr := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(tr)).Register(g)
	wr := workouts.NewRepo(pool)
	workouts.NewHandlers(workouts.NewService(wr, pool, "UTC")).Register(g)
	trainingplan.NewHandlers(trainingplan.NewService(trainingplan.NewRepo(pool), pool, wr, tr, "UTC")).Register(g)
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rdr := bytes.NewBuffer(nil)
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func mustID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return m["id"].(string)
}

func createTemplate(t *testing.T, r *gin.Engine) string {
	body := `{"sport":"run","name":"Easy run","estimated_duration_sec":3600,"steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":3600},"target":{"kind":"hr_zone","low":1,"high":2}}]}`
	return mustID(t, do(t, r, http.MethodPost, "/workout-templates", body))
}

// buildPlan creates a plan starting Mon 2026-06-01 with one week containing one
// Monday slot, and returns plan/week/slot/template ids.
func buildPlan(t *testing.T, r *gin.Engine) (planID, weekID, slotID, templateID string) {
	templateID = createTemplate(t, r)
	planID = mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"18wk","start_date":"2026-06-01"}`))
	weekID = mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	slotID = mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`"}`))
	return
}

// utcInstant parses an RFC3339 timestamp and renders it in UTC, so assertions
// are insensitive to the machine-local tz pgx uses when reading timestamptz.
func utcInstant(t *testing.T, ts string) string {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, ts)
	require.NoError(t, err)
	return parsed.UTC().Format("2006-01-02T15:04:05Z")
}

func materialize(t *testing.T, r *gin.Engine, planID, body string) []map[string]any {
	t.Helper()
	rec := do(t, r, http.MethodPost, "/training-plans/"+planID+"/materialize", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Workouts []map[string]any `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Workouts
}

func TestMaterializeWeek_LandsOnCorrectDate(t *testing.T) {
	r := setup(t)
	planID, _, _, _ := buildPlan(t, r)

	ws := materialize(t, r, planID, `{"scope":"week","week":1}`)
	require.Len(t, ws, 1)
	w := ws[0]
	assert.Equal(t, "run", w["sport"])
	assert.Equal(t, "Easy run", w["name"])
	assert.Equal(t, "planned", w["status"])
	// Monday of week 1 at the default 06:00 UTC (compared as an instant — pgx
	// renders timestamptz in machine-local tz on read-back).
	assert.Equal(t, "2026-06-01T06:00:00Z", utcInstant(t, w["started_at"].(string)))
	assert.NotEmpty(t, w["plan_slot_id"])
}

func TestMaterialize_Idempotent(t *testing.T) {
	r := setup(t)
	planID, _, _, _ := buildPlan(t, r)

	first := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, first, 1)
	second := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, second, 1)
	assert.Equal(t, first[0]["id"], second[0]["id"], "re-materialize updates the same workout")
}

func TestMaterialize_EditingSlotRetargets(t *testing.T) {
	r := setup(t)
	planID, _, slotID, _ := buildPlan(t, r)
	first := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, first, 1)

	// Move the slot to Wednesday (weekday 2) and re-materialize.
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPatch, "/training-plans/"+planID+"/slots/"+slotID, `{"weekday":2}`).Code)
	second := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, second, 1)
	assert.Equal(t, first[0]["id"], second[0]["id"], "same workout row")
	assert.Equal(t, "2026-06-03T06:00:00Z", utcInstant(t, second[0]["started_at"].(string)), "moved to Wednesday")
}

func TestMaterialize_MultiSlotDaySharesSessionGroup(t *testing.T) {
	r := setup(t)
	planID, weekID, _, templateID := buildPlan(t, r)
	// Add a second Monday slot (ordinal 1).
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":1,"template_id":"`+templateID+`"}`).Code)

	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 2)
	require.NotEmpty(t, ws[0]["session_group"])
	assert.Equal(t, ws[0]["session_group"], ws[1]["session_group"], "brick legs share a session_group")
}

func TestDeletePlan_CascadesAndNullsWorkoutLink(t *testing.T) {
	pool := storetest.NewPool(t)
	r := gin.New()
	g := r.Group("/")
	tr := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(tr)).Register(g)
	trainingplan.NewHandlers(trainingplan.NewService(trainingplan.NewRepo(pool), pool, workouts.NewRepo(pool), tr, "UTC")).Register(g)

	tmpl := createTemplate(t, r)
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots", `{"weekday":0,"ordinal":0,"template_id":"`+tmpl+`"}`)
	materialize(t, r, planID, `{"scope":"all"}`)

	// Delete the plan → weeks+slots cascade; the planned workout survives with a
	// nulled plan_slot_id (ON DELETE SET NULL).
	require.Equal(t, http.StatusNoContent, do(t, r, http.MethodDelete, "/training-plans/"+planID, "").Code)

	var count, withSlot int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*), count(plan_slot_id) FROM workouts`).Scan(&count, &withSlot))
	assert.Equal(t, 1, count, "planned workout is preserved")
	assert.Equal(t, 0, withSlot, "plan_slot_id was set null on slot cascade")
}

func TestDeleteTemplate_RestrictedWhileReferenced(t *testing.T) {
	r := setup(t)
	_, _, _, templateID := buildPlan(t, r)
	// A slot references the template → delete is refused with 409.
	rec := do(t, r, http.MethodDelete, "/workout-templates/"+templateID, "")
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "template_in_use")
}
