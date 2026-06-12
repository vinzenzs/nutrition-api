package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
)

func init() { gin.SetMode(gin.TestMode) }

const testToken = "test-agent-token"

// fakeStore is an in-memory chat.SessionStore for the loop tests, so they run
// without a database. The real persistence is covered by the chatsessions
// package's integration tests.
type fakeStore struct {
	mu     sync.Mutex
	exists map[uuid.UUID]bool
	turns  map[uuid.UUID][]StoredTurn
	titles map[uuid.UUID]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		exists: map[uuid.UUID]bool{},
		turns:  map[uuid.UUID][]StoredTurn{},
		titles: map[uuid.UUID]string{},
	}
}

func (f *fakeStore) create() uuid.UUID {
	id := uuid.New()
	f.mu.Lock()
	f.exists[id] = true
	f.mu.Unlock()
	return id
}

func (f *fakeStore) seed(id uuid.UUID, turns ...StoredTurn) {
	f.mu.Lock()
	f.turns[id] = append(f.turns[id], turns...)
	f.mu.Unlock()
}

func (f *fakeStore) loaded(id uuid.UUID) []StoredTurn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]StoredTurn(nil), f.turns[id]...)
}

func (f *fakeStore) SessionExists(_ context.Context, id uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exists[id], nil
}

func (f *fakeStore) LoadTurns(_ context.Context, id uuid.UUID, limit int) ([]StoredTurn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.turns[id]
	if limit > 0 && len(t) > limit {
		t = t[len(t)-limit:]
	}
	return append([]StoredTurn(nil), t...), nil
}

func (f *fakeStore) AppendTurns(_ context.Context, id uuid.UUID, turns []StoredTurn) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.turns[id] = append(f.turns[id], turns...)
	return nil
}

func (f *fakeStore) SetTitleIfEmpty(_ context.Context, id uuid.UUID, title string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.titles[id] == "" {
		f.titles[id] = title
	}
	return nil
}

// scriptedAnthropic returns an SSE handler that emits `turns` in order, one per
// request. The last turn is reused if more requests arrive.
func scriptedAnthropic(t *testing.T, turns []string) *httptest.Server {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(atomic.AddInt32(&n, 1)) - 1
		if i >= len(turns) {
			i = len(turns) - 1
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, turns[i])
	}))
	t.Cleanup(srv.Close)
	return srv
}

// loopEnv wires a stub Anthropic upstream behind a chat.Service, plus a Gin
// engine carrying the auth middleware, stub tool endpoints, and the chat route
// — the loopback target. The service is backed by an in-memory session store
// with one pre-created session (sessionID).
type loopEnv struct {
	engine       *gin.Engine
	anthropic    *httptest.Server
	store        *fakeStore
	sessionID    uuid.UUID
	planCreates  *int32
	planKeys     *[]string
	contextCalls *int32
}

func newLoopEnv(t *testing.T, anthropic *httptest.Server, cfg Config) *loopEnv {
	t.Helper()
	cfg.BaseURL = anthropic.URL
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
	svc, err := New(testToken /* any non-empty key */, cfg)
	require.NoError(t, err)

	store := newFakeStore()
	svc.SetSessionStore(store)
	sessionID := store.create()

	var planCreates, contextCalls int32
	var planKeys []string

	r := gin.New()
	api := r.Group("/")
	api.Use(auth.Middleware(auth.Config{MobileToken: "m", AgentToken: testToken}))
	// Stub tool endpoints (no DB) — enough for the loop to dispatch against.
	api.GET("/context/daily", func(c *gin.Context) {
		atomic.AddInt32(&contextCalls, 1)
		c.JSON(http.StatusOK, gin.H{"date": c.Query("date"), "nutrition": gin.H{"totals": gin.H{"kcal": 800}}})
	})
	api.POST("/plan", func(c *gin.Context) {
		atomic.AddInt32(&planCreates, 1)
		planKeys = append(planKeys, c.GetHeader("Idempotency-Key"))
		c.JSON(http.StatusCreated, gin.H{"id": "plan-1", "status": "planned"})
	})
	NewHandlers(svc).Register(api)
	svc.SetLoopbackHandler(r)

	return &loopEnv{
		engine: r, anthropic: anthropic, store: store, sessionID: sessionID,
		planCreates: &planCreates, planKeys: &planKeys, contextCalls: &contextCalls,
	}
}

// postMsg posts a session-backed turn for the env's pre-created session.
func postMsg(t *testing.T, env *loopEnv, message string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":%q,"message":%q}`, env.sessionID.String(), message)
	return postChat(t, env.engine, body)
}

func postChat(t *testing.T, engine http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// Happy path: a grounding tool call (get_daily_context) then a text answer,
// streamed as tool + text + done events, with all turns persisted to the session.
func TestChat_GroundedRecommendationStream(t *testing.T) {
	turn1 := sseFrames(frameMessageStart, toolUseFrames("t1", "get_daily_context", `{"date":"2026-06-12"}`), messageDelta("tool_use"), frameMessageStop)
	turn2 := sseFrames(frameMessageStart, textBlockFrames("Three options: A, B, C."), messageDelta("end_turn"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{turn1, turn2}), Config{})

	rec := postMsg(t, env, "what should I eat today?")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// The grounding tool executed and was reported.
	assert.EqualValues(t, 1, atomic.LoadInt32(env.contextCalls))
	assert.Contains(t, body, "event: tool")
	assert.Contains(t, body, `"name":"get_daily_context"`)
	assert.Contains(t, body, `"status":"started"`)
	assert.Contains(t, body, `"status":"ok"`)
	// Then the streamed answer + a terminal done event.
	assert.Contains(t, body, "event: text")
	assert.Contains(t, body, "Three options")
	assert.Contains(t, body, "event: done")
	assert.Contains(t, body, `"stop_reason":"end_turn"`)
	assert.Contains(t, body, `"input_tokens":12`)

	// Persisted: user turn, assistant(tool_use), tool_result, final assistant.
	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 4)
	assert.Equal(t, "user", turns[0].Role)
	assert.Equal(t, "assistant", turns[1].Role)
	assert.Contains(t, string(turns[1].Content), "tool_use")
	assert.Equal(t, "user", turns[2].Role)
	assert.Contains(t, string(turns[2].Content), "tool_result")
	assert.Equal(t, "assistant", turns[3].Role)
	assert.Contains(t, string(turns[3].Content), "Three options")
}

// Prior turns stored in the session are loaded as upstream context — the client
// resends nothing but the new message.
func TestChat_LoadsStoredHistory(t *testing.T) {
	env := newLoopEnv(t, scriptedAnthropic(t, []string{
		sseFrames(frameMessageStart, textBlockFrames("Sure."), messageDelta("end_turn"), frameMessageStop),
	}), Config{})
	// Seed an earlier exchange.
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"hello earlier"`)},
		StoredTurn{Role: "assistant", Content: []byte(`[{"type":"text","text":"hi"}]`)},
	)

	rec := postMsg(t, env, "and now?")
	require.Equal(t, http.StatusOK, rec.Code)

	// The seeded two turns plus the new user turn and the final assistant turn.
	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 4)
	assert.Contains(t, string(turns[0].Content), "hello earlier")
	assert.Contains(t, string(turns[2].Content), "and now?")
}

// Round cap: the upstream always asks for a tool while tools are offered; after
// MaxToolRounds the loop withholds tools and forces a final text answer.
func TestChat_RoundCapForcesFinalAnswer(t *testing.T) {
	toolTurn := sseFrames(frameMessageStart, toolUseFrames("t1", "get_daily_context", `{"date":"2026-06-12"}`), messageDelta("tool_use"), frameMessageStop)
	finalTurn := sseFrames(frameMessageStart, textBlockFrames("Best I can do."), messageDelta("end_turn"), frameMessageStop)

	// Upstream returns a tool_use whenever the request offers tools, else text.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if strings.Contains(string(bodyBytes), `"tools"`) {
			_, _ = io.WriteString(w, toolTurn)
		} else {
			_, _ = io.WriteString(w, finalTurn)
		}
	}))
	t.Cleanup(srv.Close)

	env := newLoopEnv(t, srv, Config{MaxToolRounds: 2})
	rec := postMsg(t, env, "loop please")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.EqualValues(t, 2, atomic.LoadInt32(env.contextCalls), "exactly MaxToolRounds tool executions")
	assert.Contains(t, body, `"stop_reason":"max_tool_rounds"`)
	assert.Contains(t, body, "Best I can do")
}

// A mid-stream upstream error event surfaces as an error SSE event; the user
// turn is persisted (resumable) and the session does not end on a tool_use.
func TestChat_MidStreamErrorEvent(t *testing.T) {
	bad := frameMessageStart + "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"busy\"}}\n\n"
	env := newLoopEnv(t, scriptedAnthropic(t, []string{bad}), Config{})
	rec := postMsg(t, env, "hi")
	require.Equal(t, http.StatusOK, rec.Code) // stream started, then errored
	body := rec.Body.String()
	assert.Contains(t, body, "event: error")
	assert.Contains(t, body, `"code":"upstream_unavailable"`)

	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 1)
	assert.Equal(t, "user", turns[0].Role)
}

// A write tool dispatched by the loop carries the caller's bearer (it passed
// auth) and a derived Idempotency-Key; resubmitting the identical message
// reuses the same key — the retry-replay guarantee.
func TestChat_WriteToolForwardsAuthAndStableIdempotencyKey(t *testing.T) {
	planTurn := sseFrames(frameMessageStart, toolUseFrames("t1", "create_planned_meal", `{"plan_date":"2026-06-12","slot":"dinner","product_id":"prod-1"}`), messageDelta("tool_use"), frameMessageStop)
	doneTurn := sseFrames(frameMessageStart, textBlockFrames("Planned."), messageDelta("end_turn"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{planTurn, doneTurn}), Config{})

	rec := postMsg(t, env, "plan dinner")
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 1, atomic.LoadInt32(env.planCreates))

	// Resubmit the identical turn against a fresh env — same derived idempotency key.
	env2 := newLoopEnv(t, scriptedAnthropic(t, []string{planTurn, doneTurn}), Config{})
	rec2 := postMsg(t, env2, "plan dinner")
	require.Equal(t, http.StatusOK, rec2.Code)
	require.EqualValues(t, 1, atomic.LoadInt32(env2.planCreates))

	require.Len(t, *env.planKeys, 1)
	require.Len(t, *env2.planKeys, 1)
	assert.NotEmpty(t, (*env.planKeys)[0])
	assert.Equal(t, (*env.planKeys)[0], (*env2.planKeys)[0], "identical turn yields identical idempotency key")
}

// 503 when the service is unconfigured (no API key).
func TestChat_NilServiceReturns503(t *testing.T) {
	r := gin.New()
	NewHandlers(nil).Register(r.Group("/"))
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "chat_unavailable")
}

// An unknown session id refuses with 404 before any stream is started.
func TestChat_UnknownSessionReturns404(t *testing.T) {
	env := newLoopEnv(t, scriptedAnthropic(t, []string{sseFrames(frameMessageStart, textBlockFrames("x"), messageDelta("end_turn"), frameMessageStop)}), Config{})
	body := fmt.Sprintf(`{"session_id":%q,"message":"hi"}`, uuid.New().String())
	rec := postChat(t, env.engine, body)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_not_found")
	assert.NotContains(t, rec.Body.String(), "event:")
}

// A malformed session id is also a clean 404 (no stream).
func TestChat_InvalidSessionIDReturns404(t *testing.T) {
	env := newLoopEnv(t, scriptedAnthropic(t, []string{sseFrames(frameMessageStart, textBlockFrames("x"), messageDelta("end_turn"), frameMessageStop)}), Config{})
	rec := postChat(t, env.engine, `{"session_id":"not-a-uuid","message":"hi"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "session_not_found")
}

// An empty message is rejected with 400 before any stream.
func TestChat_EmptyMessageRejected(t *testing.T) {
	env := newLoopEnv(t, scriptedAnthropic(t, []string{""}), Config{})
	rec := postMsg(t, env, "   ")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "empty_message")
}
