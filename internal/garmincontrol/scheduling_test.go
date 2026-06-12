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
	server      *httptest.Server
	mu          sync.Mutex
	createCalls int
	schedCalls  int
	unschedIDs  []string
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
