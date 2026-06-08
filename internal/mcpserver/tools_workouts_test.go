package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workoutRecord struct {
	method   string
	path     string
	rawQuery string
	body     []byte
	idemKey  string
}

func newWorkoutRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]workoutRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []workoutRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, workoutRecord{
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

func TestLogWorkout_PostsFullBodyAndForwardsIdempotencyKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)

	tss := 78.0
	kcal := 850.0
	hr := 135
	ext := "garmin:1"
	name := "Morning Z2"
	args := LogWorkoutArgs{
		ExternalID:     &ext,
		Source:         "garmin",
		Sport:          "bike",
		Name:           &name,
		StartedAt:      "2026-06-07T08:00:00Z",
		EndedAt:        "2026-06-07T09:30:00Z",
		KcalBurned:     &kcal,
		AvgHR:          &hr,
		TSS:            &tss,
		IdempotencyKey: "explicit-key",
	}
	r := handleLogWorkout(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/workouts", rec.path)
	assert.Equal(t, "explicit-key", rec.idemKey)

	// Body contains all supplied fields.
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &body))
	assert.Equal(t, "garmin:1", body["external_id"])
	assert.Equal(t, "garmin", body["source"])
	assert.Equal(t, "bike", body["sport"])
	assert.Equal(t, "Morning Z2", body["name"])
	assert.EqualValues(t, 850, body["kcal_burned"])
	assert.EqualValues(t, 135, body["avg_hr"])
	assert.EqualValues(t, 78, body["tss"])
}

func TestLogWorkout_DerivesIdempotencyKeyWhenOmitted(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	args := LogWorkoutArgs{
		Source:    "manual",
		Sport:     "strength",
		StartedAt: "2026-06-07T18:00:00Z",
		EndedAt:   "2026-06-07T19:00:00Z",
	}
	_ = handleLogWorkout(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.NotEmpty(t, (*recs)[0].idemKey, "missing key should be auto-derived")
}

func TestLogWorkout_SameArgsTwiceSameDerivedKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	args := LogWorkoutArgs{
		Source:    "manual",
		Sport:     "strength",
		StartedAt: "2026-06-07T18:00:00Z",
		EndedAt:   "2026-06-07T19:00:00Z",
	}
	_ = handleLogWorkout(context.Background(), c, args)
	_ = handleLogWorkout(context.Background(), c, args)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey)
}

func TestListWorkouts_BuildsQueryStringNoIdempotencyKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"workouts":[]}`)
	args := ListWorkoutsArgs{From: "2026-06-01T00:00:00Z", To: "2026-06-08T00:00:00Z"}
	_ = handleListWorkouts(context.Background(), c, args)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/workouts", rec.path)
	values, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01T00:00:00Z", values.Get("from"))
	assert.Equal(t, "2026-06-08T00:00:00Z", values.Get("to"))
	assert.Empty(t, rec.idemKey)
}

func TestGetWorkout_CallsPathEscapedURL(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"id":"abc"}`)
	args := GetWorkoutArgs{ID: "abc/123"}
	_ = handleGetWorkout(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.Equal(t, "/workouts/abc%2F123", (*recs)[0].path)
}

func TestGetWorkout_404IsForwardedAsError(t *testing.T) {
	c, _ := newWorkoutRecorder(t, 404, `{"error":"workout_not_found"}`)
	r := handleGetWorkout(context.Background(), c, GetWorkoutArgs{ID: "missing"})
	assert.True(t, r.IsError)
}

func TestPatchWorkout_OmitsUnsetFields(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"id":"w1"}`)
	tss := 85.0
	args := PatchWorkoutArgs{ID: "w1", TSS: &tss}
	_ = handlePatchWorkout(context.Background(), c, args)

	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/workouts/w1", rec.path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &body))
	assert.EqualValues(t, 85, body["tss"])
	_, hasName := body["name"]
	assert.False(t, hasName, "unset fields must NOT appear in the PATCH body")
}

func TestDeleteWorkout_204ReturnsEmptyContent(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 204, "")
	r := handleDeleteWorkout(context.Background(), c, DeleteWorkoutArgs{ID: "w1"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/workouts/w1", (*recs)[0].path)
}

func TestDeleteWorkout_404IsForwardedAsError(t *testing.T) {
	c, _ := newWorkoutRecorder(t, 404, `{"error":"workout_not_found"}`)
	r := handleDeleteWorkout(context.Background(), c, DeleteWorkoutArgs{ID: "missing"})
	assert.True(t, r.IsError)
}
