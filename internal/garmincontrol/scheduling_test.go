package garmincontrol_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/garmincontrol"
	"github.com/vinzenzs/nutrition-api/internal/trainingplan"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
	"github.com/vinzenzs/nutrition-api/internal/workouttemplates"
)

// --- stub repos/service satisfying garmincontrol's unexported interfaces ---

type fakeWorkouts struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*workouts.Workout
}

func newFakeWorkouts() *fakeWorkouts { return &fakeWorkouts{rows: map[uuid.UUID]*workouts.Workout{}} }

func (f *fakeWorkouts) GetByID(_ context.Context, id uuid.UUID) (*workouts.Workout, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return nil, workouts.ErrNotFound
	}
	cp := *w
	return &cp, nil
}

func (f *fakeWorkouts) SetGarminIDs(_ context.Context, id uuid.UUID, gw, gs *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.rows[id]
	if !ok {
		return workouts.ErrNotFound
	}
	w.GarminWorkoutID, w.GarminScheduleID = gw, gs
	return nil
}

type fakeTemplates struct{ missing map[string]bool }

func (f *fakeTemplates) GetByID(_ context.Context, id string) (*workouttemplates.Template, error) {
	if f.missing[id] {
		return nil, workouttemplates.ErrNotFound
	}
	return &workouttemplates.Template{
		ID: id, Sport: "run", Name: "Easy run",
		Steps: []workouttemplates.Step{{Type: "step", Intent: "active",
			Duration: &workouttemplates.Duration{Kind: "open"}, Target: &workouttemplates.Target{Kind: "none"}}},
	}, nil
}

type fakePlan struct{ ids []uuid.UUID }

func (f *fakePlan) PlannedWorkoutsInScope(_ context.Context, _ uuid.UUID, _ trainingplan.Scope) ([]uuid.UUID, error) {
	return f.ids, nil
}

func (f *fakePlan) EffectiveProgram(_ context.Context, workoutID uuid.UUID) (*trainingplan.Program, error) {
	return &trainingplan.Program{
		WorkoutID: workoutID,
		Sport:     "run",
		Steps: []workouttemplates.Step{{Type: "step", Intent: "active",
			Duration: &workouttemplates.Duration{Kind: "open"}, Target: &workouttemplates.Target{Kind: "none"}}},
	}, nil
}

// --- stub bridge ---

type bridgeStub struct {
	server         *httptest.Server
	mu             sync.Mutex
	createCalls    int
	schedCalls     int
	unschedIDs       []string
	deleteWorkouts   []string
	hydrationBody    string
	uploadBody       string
	renameBody       string
	deleteActivities []string
}

func newBridgeStub(t *testing.T) *bridgeStub {
	t.Helper()
	b := &bridgeStub{}
	b.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/workouts":
			b.createCalls++
			_, _ = io.WriteString(w, `{"garmin_workout_id":"gw-1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/schedule":
			b.schedCalls++
			_, _ = io.WriteString(w, `{"garmin_schedule_id":"gs-1"}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/schedule":
			b.unschedIDs = append(b.unschedIDs, r.URL.Query().Get("schedule_id"))
			_, _ = io.WriteString(w, `{"unscheduled":true}`)
		case r.Method == http.MethodGet && r.URL.Path == "/calendar":
			_, _ = io.WriteString(w, `{"from":"`+r.URL.Query().Get("from")+`","items":[{"garminScheduleId":"gs-1"}]}`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/workouts/"):
			b.deleteWorkouts = append(b.deleteWorkouts, strings.TrimPrefix(r.URL.Path, "/workouts/"))
			_, _ = io.WriteString(w, `{"deleted":true}`)
		case r.Method == http.MethodGet && r.URL.Path == "/workouts":
			_, _ = io.WriteString(w, `{"workouts":[{"workoutId":"gw-1"}],"start":"`+r.URL.Query().Get("start")+`"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/workouts/"):
			_, _ = io.WriteString(w, `{"workoutId":"`+strings.TrimPrefix(r.URL.Path, "/workouts/")+`"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/hydration":
			body, _ := io.ReadAll(r.Body)
			b.hydrationBody = string(body)
			_, _ = io.WriteString(w, `{"pushed":true}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/export"):
			_, _ = io.WriteString(w, `{"activity_id":"act-1","format":"`+r.URL.Query().Get("format")+`","content_base64":"Rk9P"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/gear"):
			_, _ = io.WriteString(w, `{"gear":[{"uuid":"gear-shoes-1"}]}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/download"):
			_, _ = io.WriteString(w, `{"garmin_workout_id":"gw-1","format":"`+r.URL.Query().Get("format")+`","content_base64":"V09SS09VVA=="}`)
		case r.Method == http.MethodPost && r.URL.Path == "/activity/upload":
			body, _ := io.ReadAll(r.Body)
			b.uploadBody = string(body)
			_, _ = io.WriteString(w, `{"uploaded":true}`)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/activity/"):
			body, _ := io.ReadAll(r.Body)
			b.renameBody = string(body)
			_, _ = io.WriteString(w, `{"renamed":true}`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/activity/"):
			b.deleteActivities = append(b.deleteActivities, strings.TrimPrefix(r.URL.Path, "/activity/"))
			_, _ = io.WriteString(w, `{"deleted":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/sync/backfill":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"to":"fail"`) {
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = io.WriteString(w, `{"days_total":2,"days_ok":1,"days_failed":1}`)
			} else {
				_, _ = io.WriteString(w, `{"days_total":3,"days_ok":3,"days_failed":0}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(b.server.Close)
	return b
}

const agentTok = "agent-token-bbbbbbbbbbbbbbbb"

func newEngine(bridgeURL string, fw *fakeWorkouts, ft *fakeTemplates, fp *fakePlan) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := garmincontrol.NewHandlers(bridgeURL)
	h.SetSchedulingDeps(fw, ft, fp)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: "mobile-token-aaaaaaaaaaaaaa", AgentToken: agentTok}))
	h.Register(r.Group("/"))
	return r
}

func req(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	rq := httptest.NewRequest(method, path, rdr)
	rq.Header.Set("Authorization", "Bearer "+agentTok)
	if body != "" {
		rq.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, rq)
	return w
}

func plannedWorkout(templateID uuid.UUID) *workouts.Workout {
	return &workouts.Workout{
		ID: uuid.New(), Status: workouts.StatusPlanned, Sport: workouts.SportRun,
		TemplateID: &templateID,
		StartedAt:  time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC),
		EndedAt:    time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC),
	}
}

func TestScheduleWorkout_StoresIDs(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodPost, "/garmin/schedule/workout", `{"workout_id":"`+w.ID.String()+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, bridge.createCalls)
	assert.Equal(t, 1, bridge.schedCalls)
	require.NotNil(t, fw.rows[w.ID].GarminWorkoutID)
	assert.Equal(t, "gw-1", *fw.rows[w.ID].GarminWorkoutID)
	assert.Equal(t, "gs-1", *fw.rows[w.ID].GarminScheduleID)
}

func TestRePush_UnschedulesPriorFirst(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	old := "gs-old"
	oldW := "gw-old"
	w.GarminScheduleID, w.GarminWorkoutID = &old, &oldW
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodPost, "/garmin/schedule/workout", `{"workout_id":"`+w.ID.String()+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"gs-old"}, bridge.unschedIDs, "prior entry unscheduled before re-create")
	assert.Equal(t, []string{"gw-old"}, bridge.deleteWorkouts, "prior object reaped before re-create (no orphan)")
	assert.Equal(t, "gs-1", *fw.rows[w.ID].GarminScheduleID, "ids updated to the new entry")
}

func TestUnschedule_ClearsIDs(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	old, oldW := "gs-x", "gw-x"
	w.GarminScheduleID, w.GarminWorkoutID = &old, &oldW
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodDelete, "/garmin/schedule/workout/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"gs-x"}, bridge.unschedIDs)
	assert.Equal(t, []string{"gw-x"}, bridge.deleteWorkouts, "unschedule also reaps the workout object")
	assert.Nil(t, fw.rows[w.ID].GarminScheduleID)
	assert.Nil(t, fw.rows[w.ID].GarminWorkoutID)
}

func TestUnschedule_NoScheduleIsNoop(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodDelete, "/garmin/schedule/workout/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, bridge.unschedIDs, "nothing to unschedule")
}

func TestScheduleWorkout_NonPlannedRejected(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	w.Status = workouts.StatusCompleted
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodPost, "/garmin/schedule/workout", `{"workout_id":"`+w.ID.String()+`"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_not_schedulable")
	assert.Equal(t, 0, bridge.createCalls)
}

func TestSchedulePlan_PartialSuccess(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	good := plannedWorkout(uuid.New())
	badTemplate := uuid.New()
	bad := plannedWorkout(badTemplate)
	fw.rows[good.ID] = good
	fw.rows[bad.ID] = bad
	ft := &fakeTemplates{missing: map[string]bool{badTemplate.String(): true}}
	fp := &fakePlan{ids: []uuid.UUID{good.ID, bad.ID}}
	r := newEngine(bridge.server.URL, fw, ft, fp)

	rec := req(t, r, http.MethodPost, "/garmin/schedule/plan", `{"plan_id":"`+uuid.New().String()+`","scope":"all"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp struct {
		Results []struct {
			WorkoutID string `json:"workout_id"`
			OK        bool   `json:"ok"`
			Error     string `json:"error"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	oks := map[bool]int{}
	for _, r := range resp.Results {
		oks[r.OK]++
	}
	assert.Equal(t, 1, oks[true], "the good workout scheduled")
	assert.Equal(t, 1, oks[false], "the bad workout reported a failure, not aborting the batch")
}

func TestScheduling_DisabledWhenBridgeUnset(t *testing.T) {
	fw := newFakeWorkouts()
	r := newEngine("", fw, &fakeTemplates{}, &fakePlan{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/garmin/schedule/workout", `{"workout_id":"` + uuid.New().String() + `"}`},
		{http.MethodPost, "/garmin/schedule/plan", `{"plan_id":"` + uuid.New().String() + `","scope":"all"}`},
		{http.MethodGet, "/garmin/calendar?from=2026-06-01&to=2026-06-30", ""},
		{http.MethodDelete, "/garmin/schedule/workout/" + uuid.New().String(), ""},
		{http.MethodDelete, "/garmin/workout/" + uuid.New().String(), ""},
		{http.MethodGet, "/garmin/workouts", ""},
		{http.MethodGet, "/garmin/workout/gw-1", ""},
		{http.MethodPost, "/garmin/hydration", `{"value_ml":750,"date":"2026-06-13"}`},
		{http.MethodGet, "/garmin/activity/act-1/export", ""},
	} {
		rec := req(t, r, tc.method, tc.path, tc.body)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, tc.path)
		assert.Contains(t, rec.Body.String(), "garmin_disabled", tc.path)
	}
}

// ----- workout-library management + export (garmin-workout-library-mgmt) -----

func TestDeleteWorkoutObject_ReapsAndClearsWorkoutID(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	gw, gs := "gw-keep", "gs-keep"
	w.GarminWorkoutID, w.GarminScheduleID = &gw, &gs
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodDelete, "/garmin/workout/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"gw-keep"}, bridge.deleteWorkouts)
	assert.Nil(t, fw.rows[w.ID].GarminWorkoutID, "workout id cleared")
	require.NotNil(t, fw.rows[w.ID].GarminScheduleID, "schedule id left intact (object delete ≠ unschedule)")
	assert.Equal(t, "gs-keep", *fw.rows[w.ID].GarminScheduleID)
}

func TestDeleteWorkoutObject_NoopWhenNoID(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedWorkout(uuid.New())
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodDelete, "/garmin/workout/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"deleted":false`)
	assert.Empty(t, bridge.deleteWorkouts, "no object to delete → no bridge call")
}

func TestDeleteWorkoutObject_UnknownWorkout404(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodDelete, "/garmin/workout/"+uuid.New().String(), "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_not_found")
}

func TestGarminLibrary_ListAndGetPassthrough(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	lst := req(t, r, http.MethodGet, "/garmin/workouts?start=5&limit=3", "")
	require.Equal(t, http.StatusOK, lst.Code, lst.Body.String())
	assert.Contains(t, lst.Body.String(), `"workoutId":"gw-1"`)
	assert.Contains(t, lst.Body.String(), `"start":"5"`)

	one := req(t, r, http.MethodGet, "/garmin/workout/gw-42", "")
	require.Equal(t, http.StatusOK, one.Code, one.Body.String())
	assert.Contains(t, one.Body.String(), `"workoutId":"gw-42"`)
}

func TestGarminPushHydration_Forwards(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodPost, "/garmin/hydration", `{"value_ml":750,"date":"2026-06-13"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"pushed":true`)
	assert.Contains(t, bridge.hydrationBody, `"value_ml":750`)
	assert.Contains(t, bridge.hydrationBody, `"date":"2026-06-13"`)
}

func TestGarminExportActivity_Passthrough(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})

	rec := req(t, r, http.MethodGet, "/garmin/activity/act-1/export?format=gpx", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"content_base64":"Rk9P"`)
	assert.Contains(t, rec.Body.String(), `"format":"gpx"`)
}

// ----- activity-level control operations (add-garmin-misc-mirror) -----

func TestActivityGear_Passthrough(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodGet, "/garmin/activity/act-1/gear", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"gear-shoes-1"`)
}

func TestDownloadWorkout_Passthrough(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodGet, "/garmin/workout/gw-1/download?format=fit", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"content_base64":"V09SS09VVA=="`)
	assert.Contains(t, rec.Body.String(), `"format":"fit"`)
}

func TestUploadActivity_Forwards(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodPost, "/garmin/activity/upload", `{"filename":"ride.fit","content_base64":"Rk9P"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"uploaded":true`)
	assert.Contains(t, bridge.uploadBody, `"ride.fit"`)
}

func TestRenameActivity_ForwardsAndRequiresName(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})

	ok := req(t, r, http.MethodPatch, "/garmin/activity/act-1", `{"name":"Evening Z2 ride"}`)
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	assert.Contains(t, ok.Body.String(), `"renamed":true`)
	assert.Contains(t, bridge.renameBody, "Evening Z2 ride")

	bad := req(t, r, http.MethodPatch, "/garmin/activity/act-1", `{}`)
	require.Equal(t, http.StatusBadRequest, bad.Code)
	assert.JSONEq(t, `{"error":"name_required"}`, bad.Body.String())
}

func TestDeleteActivity_Forwards(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodDelete, "/garmin/activity/act-1", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"deleted":true`)
	assert.Equal(t, []string{"act-1"}, bridge.deleteActivities)
}

// ----- history backfill (add-garmin-history-backfill) -----

func TestBackfill_ForwardsVerbatim(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodPost, "/garmin/backfill", `{"from":"2026-03-01","to":"2026-03-03"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"days_total":3`)
}

func TestBackfill_207PassThrough(t *testing.T) {
	bridge := newBridgeStub(t)
	r := newEngine(bridge.server.URL, newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodPost, "/garmin/backfill", `{"from":"2026-03-01","to":"fail"}`)
	require.Equal(t, http.StatusMultiStatus, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"days_failed":1`)
}

func TestBackfill_DisabledWhenBridgeUnset(t *testing.T) {
	r := newEngine("", newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodPost, "/garmin/backfill", `{"from":"2026-03-01","to":"2026-03-03"}`)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "garmin_disabled")
}

func TestActivityOps_DisabledWhenBridgeUnset(t *testing.T) {
	r := newEngine("", newFakeWorkouts(), &fakeTemplates{}, &fakePlan{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/garmin/activity/act-1/gear", ""},
		{http.MethodGet, "/garmin/workout/gw-1/download", ""},
		{http.MethodPost, "/garmin/activity/upload", `{"filename":"r.fit","content_base64":"Rk9P"}`},
		{http.MethodPatch, "/garmin/activity/act-1", `{"name":"x"}`},
		{http.MethodDelete, "/garmin/activity/act-1", ""},
	} {
		rec := req(t, r, tc.method, tc.path, tc.body)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, tc.path)
		assert.Contains(t, rec.Body.String(), "garmin_disabled", tc.path)
	}
}

func TestCalendar_PassesThrough(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{})
	rec := req(t, r, http.MethodGet, "/garmin/calendar?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"garminScheduleId":"gs-1"`)
}
