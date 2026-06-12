package chatsessions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/chatsessions"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r    *gin.Engine
	pool *pgxpool.Pool
	repo *chatsessions.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := chatsessions.NewRepo(pool)
	r := gin.New()
	chatsessions.NewHandlers(chatsessions.NewService(repo)).Register(r.Group("/"))
	return &fixture{r: r, pool: pool, repo: repo}
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

func msgCount(t *testing.T, pool *pgxpool.Pool, sessionID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM chat_messages WHERE session_id = $1", sessionID).Scan(&n))
	return n
}

func TestCreate_WithAndWithoutTitle(t *testing.T) {
	f := setup(t)

	w := do(t, f.r, http.MethodPost, "/chat/sessions", `{"title":"Race week meals"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	body := decode(t, w)
	assert.Equal(t, "Race week meals", body["title"])
	assert.NotEmpty(t, body["id"])

	w2 := do(t, f.r, http.MethodPost, "/chat/sessions", `{}`)
	require.Equal(t, http.StatusCreated, w2.Code)
	body2 := decode(t, w2)
	_, hasTitle := body2["title"]
	assert.False(t, hasTitle, "untitled session omits the title field")
}

func TestCreate_TitleTooLong(t *testing.T) {
	f := setup(t)
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	w := do(t, f.r, http.MethodPost, "/chat/sessions", `{"title":"`+string(long)+`"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "title_invalid")
}

func TestList_OrdersByRecency(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	a, err := f.repo.CreateSession(ctx, ptr("A"))
	require.NoError(t, err)
	b, err := f.repo.CreateSession(ctx, ptr("B"))
	require.NoError(t, err)

	// Give A more recent activity than B.
	require.NoError(t, f.repo.AppendMessages(ctx, a.ID, []chatsessions.Message{
		{Role: "user", Content: json.RawMessage(`"hi"`)},
	}))

	w := do(t, f.r, http.MethodGet, "/chat/sessions", "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Sessions []chatsessions.Session `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Sessions, 2)
	assert.Equal(t, a.ID, resp.Sessions[0].ID, "most recent activity first")
	assert.Equal(t, b.ID, resp.Sessions[1].ID)
	// The list carries no transcript.
	assert.NotContains(t, w.Body.String(), "messages")
}

func TestGet_ReturnsTranscriptInOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, err := f.repo.CreateSession(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, f.repo.AppendMessages(ctx, s.ID, []chatsessions.Message{
		{Role: "user", Content: json.RawMessage(`"what should I eat?"`)},
		{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"Try pasta."}]`)},
	}))

	w := do(t, f.r, http.MethodGet, "/chat/sessions/"+s.ID.String(), "")
	require.Equal(t, http.StatusOK, w.Code)
	var resp chatsessions.SessionWithMessages
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Messages, 2)
	assert.Equal(t, "user", resp.Messages[0].Role)
	assert.Equal(t, "assistant", resp.Messages[1].Role)
	assert.Contains(t, string(resp.Messages[1].Content), "Try pasta")
}

func TestGet_UnknownIs404(t *testing.T) {
	f := setup(t)
	w := do(t, f.r, http.MethodGet, "/chat/sessions/"+uuid.New().String(), "")
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "session_not_found")

	// A malformed id is also 404.
	w2 := do(t, f.r, http.MethodGet, "/chat/sessions/not-a-uuid", "")
	require.Equal(t, http.StatusNotFound, w2.Code)
}

func TestRename_SetAndClear(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, err := f.repo.CreateSession(ctx, ptr("old"))
	require.NoError(t, err)

	w := do(t, f.r, http.MethodPatch, "/chat/sessions/"+s.ID.String(), `{"title":"Taper week"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Taper week", decode(t, w)["title"])

	// Clear with empty string.
	w2 := do(t, f.r, http.MethodPatch, "/chat/sessions/"+s.ID.String(), `{"title":""}`)
	require.Equal(t, http.StatusOK, w2.Code)
	_, hasTitle := decode(t, w2)["title"]
	assert.False(t, hasTitle, "cleared title is untitled")
}

func TestRename_UnknownIs404(t *testing.T) {
	f := setup(t)
	w := do(t, f.r, http.MethodPatch, "/chat/sessions/"+uuid.New().String(), `{"title":"x"}`)
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "session_not_found")
}

func TestDelete_CascadesMessages(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, err := f.repo.CreateSession(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, f.repo.AppendMessages(ctx, s.ID, []chatsessions.Message{
		{Role: "user", Content: json.RawMessage(`"hi"`)},
	}))
	require.Equal(t, 1, msgCount(t, f.pool, s.ID.String()))

	w := do(t, f.r, http.MethodDelete, "/chat/sessions/"+s.ID.String(), "")
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, msgCount(t, f.pool, s.ID.String()), "messages cascade-deleted")

	// Gone now.
	w2 := do(t, f.r, http.MethodGet, "/chat/sessions/"+s.ID.String(), "")
	require.Equal(t, http.StatusNotFound, w2.Code)
}

func TestDelete_UnknownIs404(t *testing.T) {
	f := setup(t)
	w := do(t, f.r, http.MethodDelete, "/chat/sessions/"+uuid.New().String(), "")
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "session_not_found")
}

func TestLoadTurns_MostRecentInOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s, err := f.repo.CreateSession(ctx, nil)
	require.NoError(t, err)
	for _, txt := range []string{"one", "two", "three"} {
		require.NoError(t, f.repo.AppendMessages(ctx, s.ID, []chatsessions.Message{
			{Role: "user", Content: json.RawMessage(`"` + txt + `"`)},
		}))
	}
	turns, err := f.repo.LoadTurns(ctx, s.ID, 2)
	require.NoError(t, err)
	require.Len(t, turns, 2)
	assert.Contains(t, string(turns[0].Content), "two")
	assert.Contains(t, string(turns[1].Content), "three")
}

func ptr(s string) *string { return &s }
