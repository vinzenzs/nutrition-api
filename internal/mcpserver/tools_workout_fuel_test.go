package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wfuelRecord struct {
	method   string
	path     string
	rawQuery string
	body     []byte
	idemKey  string
}

func newWFuelRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]wfuelRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []wfuelRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, wfuelRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			body:     body,
			idemKey:  r.Header.Get("Idempotency-Key"),
		})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, &records
}

// ----- log_workout_fuel -----

func TestLogWorkoutFuel_PostsBodyAndForwardsOptionalFields(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs, sodium := 25.0, 100.0
	r := handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name:     "Maurten Gel 100",
		LoggedAt: "2026-06-07T08:45:00Z",
		CarbsG:   &carbs,
		SodiumMg: &sodium,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/workout-fuel", rec.path)
	assert.JSONEq(t,
		`{"name":"Maurten Gel 100","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"sodium_mg":100}`,
		string(rec.body))
	assert.NotEmpty(t, rec.idemKey, "derived idempotency key should be present")
}

func TestLogWorkoutFuel_OmitsUnsetNutriments(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs := 25.0
	_ = handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name:     "Gel",
		LoggedAt: "2026-06-07T08:45:00Z",
		CarbsG:   &carbs,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t,
		`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25}`,
		string((*recs)[0].body),
		"unset nutriments must be omitted from the body")
}

func TestLogWorkoutFuel_ExplicitZeroCaffeineForwarded(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs, caff := 25.0, 0.0
	_ = handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name:       "Decaf gel",
		LoggedAt:   "2026-06-07T08:45:00Z",
		CarbsG:     &carbs,
		CaffeineMg: &caff,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t,
		`{"name":"Decaf gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"caffeine_mg":0}`,
		string((*recs)[0].body),
		"explicit zero must round-trip — distinct from omitting")
}

func TestLogWorkoutFuel_WorkoutIDForwarded(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs := 25.0
	_ = handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name:      "Gel",
		LoggedAt:  "2026-06-07T08:45:00Z",
		CarbsG:    &carbs,
		WorkoutID: "11111111-1111-1111-1111-111111111111",
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t,
		`{"name":"Gel","logged_at":"2026-06-07T08:45:00Z","carbs_g":25,"workout_id":"11111111-1111-1111-1111-111111111111"}`,
		string((*recs)[0].body))
}

func TestLogWorkoutFuel_ExplicitIdempotencyKeyForwarded(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs := 25.0
	_ = handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name:           "Gel",
		LoggedAt:       "2026-06-07T08:45:00Z",
		CarbsG:         &carbs,
		IdempotencyKey: "explicit-wf-key",
	})
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-wf-key", (*recs)[0].idemKey)
}

func TestLogWorkoutFuel_SameArgsProduceSameDerivedKey(t *testing.T) {
	c, recs := newWFuelRecorder(t, 201, `{"id":"wf1"}`)
	carbs := 25.0
	args := LogWorkoutFuelArgs{Name: "Gel", LoggedAt: "2026-06-07T08:45:00Z", CarbsG: &carbs}
	_ = handleLogWorkoutFuel(context.Background(), c, args)
	_ = handleLogWorkoutFuel(context.Background(), c, args)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey)
}

func TestLogWorkoutFuel_400Forwarded(t *testing.T) {
	c, _ := newWFuelRecorder(t, 400, `{"error":"empty_entry"}`)
	r := handleLogWorkoutFuel(context.Background(), c, LogWorkoutFuelArgs{
		Name: "Gel", LoggedAt: "2026-06-07T08:45:00Z",
	})
	assert.True(t, r.IsError)
}

// ----- list_workout_fuel -----

func TestListWorkoutFuel_GetsWithWindowQuery(t *testing.T) {
	c, recs := newWFuelRecorder(t, 200, `{"entries":[]}`)
	r := handleListWorkoutFuel(context.Background(), c, ListWorkoutFuelArgs{
		From: "2026-06-01T00:00:00Z",
		To:   "2026-06-08T00:00:00Z",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/workout-fuel", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", q.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", q.Get("to"))
	assert.Empty(t, rec.idemKey, "list is read-only; no idempotency key")
}

// ----- patch_workout_fuel -----

func TestPatchWorkoutFuel_OnlySuppliedFieldsAreSent(t *testing.T) {
	c, recs := newWFuelRecorder(t, 200, `{"id":"wf1"}`)
	sodium := 420.0
	_ = handlePatchWorkoutFuel(context.Background(), c, PatchWorkoutFuelArgs{
		ID:       "abc",
		SodiumMg: &sodium,
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/workout-fuel/abc", rec.path)
	assert.JSONEq(t, `{"sodium_mg":420}`, string(rec.body))
	assert.NotEmpty(t, rec.idemKey)
}

func TestPatchWorkoutFuel_WorkoutIDEmptyStringForwardedAsClear(t *testing.T) {
	c, recs := newWFuelRecorder(t, 200, `{"id":"wf1"}`)
	clear := ""
	_ = handlePatchWorkoutFuel(context.Background(), c, PatchWorkoutFuelArgs{
		ID:        "abc",
		WorkoutID: &clear,
	})
	require.Len(t, *recs, 1)
	assert.JSONEq(t, `{"workout_id":""}`, string((*recs)[0].body),
		"empty-string clear sentinel must be forwarded verbatim")
}

func TestPatchWorkoutFuel_404Forwarded(t *testing.T) {
	c, _ := newWFuelRecorder(t, 404, `{"error":"workout_fuel_not_found"}`)
	sodium := 420.0
	r := handlePatchWorkoutFuel(context.Background(), c, PatchWorkoutFuelArgs{
		ID:       "abc",
		SodiumMg: &sodium,
	})
	assert.True(t, r.IsError)
}

// ----- delete_workout_fuel -----

func TestDeleteWorkoutFuel_204ReturnsEmptySuccessResult(t *testing.T) {
	c, recs := newWFuelRecorder(t, 204, "")
	r := handleDeleteWorkoutFuel(context.Background(), c, DeleteWorkoutFuelArgs{ID: "abc"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/workout-fuel/abc", (*recs)[0].path)
}

func TestDeleteWorkoutFuel_404ReturnsIsError(t *testing.T) {
	c, _ := newWFuelRecorder(t, 404, `{"error":"workout_fuel_not_found"}`)
	r := handleDeleteWorkoutFuel(context.Background(), c, DeleteWorkoutFuelArgs{ID: "abc"})
	assert.True(t, r.IsError)
}
