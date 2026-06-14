package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/agenttools"
	"github.com/vinzenzs/kazper/internal/auth"
)

// scheduleWorkoutSpec is a write-confirm tool injected only in confirm tests —
// the production registry carries no write-confirm tools until phase 3. It has a
// Format formatter so the proposal preview is exercised.
func scheduleWorkoutSpec() agenttools.Spec {
	return agenttools.Spec{
		Name:        "schedule_workout",
		Description: "Schedule a workout on a date.",
		Schema:      `{"type":"object","properties":{"date":{"type":"string"},"type":{"type":"string"}},"required":["date"]}`,
		Tier:        agenttools.TierWriteConfirm,
		Build: func(in json.RawMessage) (agenttools.HTTPCall, error) {
			return agenttools.Passthrough("POST", "/workouts/schedule", in)
		},
		Format: func(in json.RawMessage) string {
			var a struct {
				Date string `json:"date"`
				Type string `json:"type"`
			}
			_ = json.Unmarshal(in, &a)
			return "Schedule a " + a.Type + " on " + a.Date
		},
	}
}

// confirmEnv mirrors loopEnv but injects the write-confirm schedule_workout tool
// and a stub endpoint for it, so the confirmation protocol can be exercised.
type confirmEnv struct {
	engine        *gin.Engine
	store         *fakeStore
	sessionID     uuid.UUID
	scheduleCalls *int32
	scheduleKeys  *[]string
}

func newConfirmEnv(t *testing.T, anthropic *httptest.Server, cfg Config) *confirmEnv {
	t.Helper()
	cfg.BaseURL = anthropic.URL
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	svc, err := New(testToken, cfg)
	require.NoError(t, err)
	svc.SetToolSpecs(append(agenttools.Registry(), scheduleWorkoutSpec()))

	store := newFakeStore()
	svc.SetSessionStore(store)
	sessionID := store.create()

	var scheduleCalls int32
	var scheduleKeys []string

	r := gin.New()
	api := r.Group("/")
	api.Use(auth.Middleware(auth.Config{MobileToken: "m", AgentToken: testToken}))
	api.GET("/context/daily", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"date": c.Query("date")})
	})
	api.POST("/workouts/schedule", func(c *gin.Context) {
		atomic.AddInt32(&scheduleCalls, 1)
		scheduleKeys = append(scheduleKeys, c.GetHeader("Idempotency-Key"))
		c.JSON(http.StatusCreated, gin.H{"id": "w1", "status": "scheduled"})
	})
	NewHandlers(svc).Register(api)
	svc.SetLoopbackHandler(r)

	return &confirmEnv{engine: r, store: store, sessionID: sessionID, scheduleCalls: &scheduleCalls, scheduleKeys: &scheduleKeys}
}

func postConfirm(t *testing.T, engine http.Handler, sessionID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/chat/sessions/"+sessionID.String()+"/confirm", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func schedTurn(id, date, typ string) string {
	return sseFrames(frameMessageStart,
		toolUseFrames(id, "schedule_workout", fmt.Sprintf(`{"date":%q,"type":%q}`, date, typ)),
		messageDelta("tool_use"), frameMessageStop)
}

func doneTurn(text string) string {
	return sseFrames(frameMessageStart, textBlockFrames(text), messageDelta("end_turn"), frameMessageStop)
}

// pausedAssistantTurn is a stored assistant turn with one unanswered
// schedule_workout tool_use block (a session awaiting confirmation).
func pausedAssistantTurn(id, date, typ string) StoredTurn {
	return StoredTurn{Role: "assistant", Content: json.RawMessage(
		fmt.Sprintf(`[{"type":"tool_use","id":%q,"name":"schedule_workout","input":{"date":%q,"type":%q}}]`, id, date, typ))}
}

// A write-confirm call pauses the stream: proposal + awaiting_confirmation, the
// assistant turn persisted with no tool_result, and NOTHING dispatched.
func TestConfirm_PauseOnWriteConfirm(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{schedTurn("c1", "2026-06-20", "ride")}), Config{})
	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule my ride saturday"}`, env.sessionID.String())
	rec := postChat(t, env.engine, body)
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()

	assert.Contains(t, out, "event: proposal")
	assert.Contains(t, out, `"name":"schedule_workout"`)
	assert.Contains(t, out, `"tier":"write-confirm"`)
	assert.Contains(t, out, `"preview":"Schedule a ride on 2026-06-20"`)
	assert.Contains(t, out, `"stop_reason":"awaiting_confirmation"`)
	assert.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls), "nothing dispatched while paused")

	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 2) // user + assistant(tool_use)
	assert.Equal(t, "assistant", turns[1].Role)
	assert.Contains(t, string(turns[1].Content), "tool_use")
	assert.NotContains(t, string(turns[1].Content), "tool_result")
}

// Approving the proposal dispatches the write and resumes the loop to a done.
func TestConfirm_ApproveExecutesAndResumes(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{schedTurn("c1", "2026-06-20", "ride"), doneTurn("Scheduled.")}), Config{})
	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule it"}`, env.sessionID.String())
	require.Equal(t, http.StatusOK, postChat(t, env.engine, body).Code)
	require.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls))

	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[{"tool_id":"c1","approve":true}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()
	assert.EqualValues(t, 1, atomic.LoadInt32(env.scheduleCalls), "approved write dispatched once")
	assert.Contains(t, out, `"name":"schedule_workout"`)
	assert.Contains(t, out, `"status":"ok"`)
	assert.Contains(t, out, "Scheduled.")
	assert.Contains(t, out, `"stop_reason":"end_turn"`)

	// Write carried a derived idempotency key.
	require.Len(t, *env.scheduleKeys, 1)
	assert.NotEmpty(t, (*env.scheduleKeys)[0])

	// Persisted: user, assistant(tool_use), user(tool_result), assistant(text).
	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 4)
	assert.Contains(t, string(turns[2].Content), "tool_result")
	assert.Contains(t, string(turns[3].Content), "Scheduled.")
}

// Rejecting the proposal dispatches nothing, appends a declined tool_result, and
// resumes so the agent can adapt.
func TestConfirm_RejectSynthesizesDeclinedAndResumes(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{schedTurn("c1", "2026-06-20", "ride"), doneTurn("Okay, left it.")}), Config{})
	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule it"}`, env.sessionID.String())
	require.Equal(t, http.StatusOK, postChat(t, env.engine, body).Code)

	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[{"tool_id":"c1","approve":false}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls), "rejected write never dispatched")
	assert.Contains(t, rec.Body.String(), "Okay, left it.")

	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 4)
	assert.Contains(t, string(turns[2].Content), "declined")
}

// A turn proposing two writes can be approved per-item: only the approved one
// dispatches; the rejected one gets a declined result.
func TestConfirm_PerItemSubset(t *testing.T) {
	twoCalls := sseFrames(frameMessageStart,
		toolUseFrames("c1", "schedule_workout", `{"date":"2026-06-20","type":"ride"}`),
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"c2\",\"name\":\"schedule_workout\"}}\n\n"+
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"date\\\":\\\"2026-06-21\\\",\\\"type\\\":\\\"run\\\"}\"}}\n\n"+
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n",
		messageDelta("tool_use"), frameMessageStop)
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{twoCalls, doneTurn("Done one.")}), Config{})
	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule both"}`, env.sessionID.String())
	out := postChat(t, env.engine, body).Body.String()
	assert.Equal(t, 2, strings.Count(out, `"tool_id":"c`), "both writes proposed")

	rec := postConfirm(t, env.engine, env.sessionID,
		`{"decisions":[{"tool_id":"c1","approve":true},{"tool_id":"c2","approve":false}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 1, atomic.LoadInt32(env.scheduleCalls), "only the approved write dispatched")

	turns := env.store.loaded(env.sessionID)
	toolResult := string(turns[2].Content)
	assert.Contains(t, toolResult, "declined", "rejected call has a declined result")
	assert.Contains(t, toolResult, "scheduled", "approved call has the real result")
}

// Sending a new /chat message while paused implicitly rejects the pending write
// (declined tool_result appended) and proceeds with the new message.
func TestConfirm_NewMessageImplicitlyRejects(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("Sure, what else?")}), Config{})
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"schedule it"`)},
		pausedAssistantTurn("c1", "2026-06-20", "ride"),
	)
	body := fmt.Sprintf(`{"session_id":%q,"message":"actually never mind, what's for dinner?"}`, env.sessionID.String())
	rec := postChat(t, env.engine, body)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls), "pending write never fires")
	assert.Contains(t, rec.Body.String(), "Sure, what else?")

	turns := env.store.loaded(env.sessionID)
	// seed(2) + declined tool_result + new user + final assistant = 5
	require.Len(t, turns, 5)
	assert.Contains(t, string(turns[2].Content), "declined")
	assert.Contains(t, string(turns[3].Content), "never mind")
}

// A continuation owed (trailing tool_result, no following assistant — a prior
// resume's stream died after the write committed) is recovered by re-posting the
// confirm: it continues the loop without re-executing the write.
func TestConfirm_ContinuationDiedRecovers(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("All set.")}), Config{})
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"schedule it"`)},
		pausedAssistantTurn("c1", "2026-06-20", "ride"),
		StoredTurn{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"c1","content":"{\"id\":\"w1\"}"}]`)},
	)
	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls), "writes already committed are not re-run")
	assert.Contains(t, rec.Body.String(), "All set.")

	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 4) // seed(3) + final assistant
	assert.Contains(t, string(turns[3].Content), "All set.")
}

// Confirming a session that is not paused (trailing assistant text) is 409.
func TestConfirm_NothingToConfirm(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("x")}), Config{})
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"hi"`)},
		StoredTurn{Role: "assistant", Content: []byte(`[{"type":"text","text":"hello"}]`)},
	)
	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[]}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "nothing_to_confirm")
	assert.NotContains(t, rec.Body.String(), "event:")
}

// Decisions that do not match the pending calls are 400.
func TestConfirm_InvalidConfirmation(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("x")}), Config{})
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"schedule it"`)},
		pausedAssistantTurn("c1", "2026-06-20", "ride"),
	)
	// Wrong tool_id.
	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[{"tool_id":"nope","approve":true}]}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_confirmation")

	// Missing decision (none for c1).
	rec2 := postConfirm(t, env.engine, env.sessionID, `{"decisions":[]}`)
	assert.Equal(t, http.StatusBadRequest, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "invalid_confirmation")
}

// An unknown session is 404 before any stream.
func TestConfirm_UnknownSession(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("x")}), Config{})
	rec := postConfirm(t, env.engine, uuid.New(), `{"decisions":[]}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_not_found")
}

// A real coach write-confirm tool from the production registry (log_workout)
// pauses the loop with its code-composed preview — proving phase-3 surface
// tools integrate with the phase-2 confirm gate (no injected specs here).
func TestConfirm_RealCoachToolPauses(t *testing.T) {
	logTurn := sseFrames(frameMessageStart,
		toolUseFrames("c1", "log_workout",
			`{"source":"manual","sport":"run","started_at":"2026-06-20T09:00:00Z","ended_at":"2026-06-20T10:00:00Z"}`),
		messageDelta("tool_use"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{logTurn}), Config{})

	rec := postMsg(t, env, "log my run this morning")
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()
	assert.Contains(t, out, "event: proposal")
	assert.Contains(t, out, `"name":"log_workout"`)
	assert.Contains(t, out, `"tier":"write-confirm"`)
	assert.Contains(t, out, `"preview":"Log run workout on 2026-06-20"`)
	assert.Contains(t, out, `"stop_reason":"awaiting_confirmation"`)
}

// sanitizeHistory preserves a trailing awaiting-confirmation turn (resume
// anchor) but still drops a truncation-dangling non-confirm tool_use turn.
func TestSanitizeHistory_PreservesPausedDropsTruncated(t *testing.T) {
	specs := agenttools.ByName([]agenttools.Spec{
		scheduleWorkoutSpec(),
		{Name: "search_products", Tier: agenttools.TierRead},
	})

	paused := []StoredTurn{
		{Role: "user", Content: []byte(`"hi"`)},
		pausedAssistantTurn("c1", "2026-06-20", "ride"),
	}
	got := sanitizeHistory(paused, specs)
	require.Len(t, got, 2, "awaiting-confirmation anchor preserved")
	assert.Equal(t, "assistant", got[1].Role)

	truncated := []StoredTurn{
		{Role: "user", Content: []byte(`"hi"`)},
		{Role: "assistant", Content: json.RawMessage(`[{"type":"tool_use","id":"t1","name":"search_products","input":{"q":"x"}}]`)},
	}
	got2 := sanitizeHistory(truncated, specs)
	require.Len(t, got2, 1, "dangling non-confirm tool_use dropped")
	assert.Equal(t, "user", got2[0].Role)
}
