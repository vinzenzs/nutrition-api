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

type tpRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
	body     string
}

func newTPRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]tpRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []tpRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, tpRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			idemKey:  r.Header.Get("Idempotency-Key"),
			body:     string(raw),
		})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}, &records
}

// ----- create_phase -----

func TestCreatePhase_PostsWithIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 201, `{"phase":{}}`)
	r := handleCreatePhase(context.Background(), c, CreatePhaseArgs{
		Name: "build-1", Type: "build",
		StartDate: "2026-07-01", EndDate: "2026-07-28",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/phases", rec.path)
	assert.NotEmpty(t, rec.idemKey, "create_phase is a POST write — Idempotency-Key required")

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(rec.body), &body))
	assert.Equal(t, "build-1", body["name"])
	assert.Equal(t, "build", body["type"])
	_, hasIdemKey := body["idempotency_key"]
	assert.False(t, hasIdemKey, "idempotency_key is a header, not a body field")
}

func TestCreatePhase_ExplicitIdempotencyKeyForwarded(t *testing.T) {
	c, recs := newTPRecorder(t, 201, `{}`)
	_ = handleCreatePhase(context.Background(), c, CreatePhaseArgs{
		Name: "build-1", Type: "build",
		StartDate: "2026-07-01", EndDate: "2026-07-28",
		IdempotencyKey: "my-key",
	})
	require.Len(t, *recs, 1)
	assert.Equal(t, "my-key", (*recs)[0].idemKey)
}

// ----- list_phases -----

func TestListPhases_GetsRangeWithoutIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{"phases":[]}`)
	_ = handleListPhases(context.Background(), c, ListPhasesArgs{
		From: "2026-07-01", To: "2026-07-31",
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/phases", rec.path)
	q, _ := url.ParseQuery(rec.rawQuery)
	assert.Equal(t, "2026-07-01", q.Get("from"))
	assert.Equal(t, "2026-07-31", q.Get("to"))
	assert.Empty(t, rec.idemKey)
}

// ----- update_phase -----

func TestUpdatePhase_PatchesWithBodyMinusPhaseID(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{"phase":{}}`)
	name := "build-1-revised"
	_ = handleUpdatePhase(context.Background(), c, UpdatePhaseArgs{
		PhaseID: "abc-123",
		Name:    &name,
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/phases/abc-123", rec.path)
	assert.NotEmpty(t, rec.idemKey)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(rec.body), &body))
	_, hasPhaseID := body["phase_id"]
	assert.False(t, hasPhaseID, "phase_id consumed by wrapper for URL path; not in body")
	assert.Equal(t, "build-1-revised", body["name"])
}

func TestUpdatePhase_EmptyStringClearsTemplate(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{}`)
	clear := ""
	_ = handleUpdatePhase(context.Background(), c, UpdatePhaseArgs{
		PhaseID:           "abc",
		DefaultTemplateID: &clear,
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte((*recs)[0].body), &body))
	v, ok := body["default_template_id"]
	require.True(t, ok)
	assert.Equal(t, "", v, "empty string sentinel forwarded to backend")
}

// ----- delete_phase -----

func TestDeletePhase_DeletesWithIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 204, ``)
	_ = handleDeletePhase(context.Background(), c, DeletePhaseArgs{PhaseID: "abc"})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/phases/abc", rec.path)
	assert.NotEmpty(t, rec.idemKey)
}

// ----- set_goal_template -----

func TestSetGoalTemplate_PutsWithoutIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{"template":{}}`)
	r := handleSetGoalTemplate(context.Background(), c, SetGoalTemplateArgs{
		Name: "weekday-easy",
		Kcal: &GoalRange{Min: floatPtr(2090), Max: floatPtr(2310)},
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/goal-templates/weekday-easy", rec.path)
	assert.Empty(t, rec.idemKey, "PUT — no Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(rec.body), &body))
	_, hasName := body["name"]
	assert.False(t, hasName, "name is the URL path, not a body field")
}

// ----- list / get / delete templates -----

func TestListGoalTemplates_GetsWithoutIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{"templates":[]}`)
	_ = handleListGoalTemplates(context.Background(), c, ListGoalTemplatesArgs{})
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodGet, (*recs)[0].method)
	assert.Equal(t, "/goal-templates", (*recs)[0].path)
	assert.Empty(t, (*recs)[0].idemKey)
}

func TestGetGoalTemplate_GetsByName(t *testing.T) {
	c, recs := newTPRecorder(t, 200, `{"template":{}}`)
	_ = handleGetGoalTemplate(context.Background(), c, GetGoalTemplateArgs{Name: "weekday-easy"})
	require.Len(t, *recs, 1)
	assert.Equal(t, "/goal-templates/weekday-easy", (*recs)[0].path)
}

func TestDeleteGoalTemplate_DeletesWithIdempotencyKey(t *testing.T) {
	c, recs := newTPRecorder(t, 204, ``)
	_ = handleDeleteGoalTemplate(context.Background(), c, DeleteGoalTemplateArgs{Name: "weekday-easy"})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/goal-templates/weekday-easy", rec.path)
	assert.NotEmpty(t, rec.idemKey)
}

func TestDeleteGoalTemplate_409InUseForwarded(t *testing.T) {
	c, _ := newTPRecorder(t, 409,
		`{"error":"template_in_use","referencing_phases":[{"id":"abc","name":"build-1"}]}`)
	r := handleDeleteGoalTemplate(context.Background(), c, DeleteGoalTemplateArgs{Name: "weekday-easy"})
	assert.True(t, r.IsError, "409 must surface as error so the agent can decide what to do")
}

func floatPtr(v float64) *float64 { return &v }
