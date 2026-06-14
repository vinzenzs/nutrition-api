package achievements_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/achievements"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := achievements.NewService(achievements.NewRepo(pool))
	r := gin.New()
	achievements.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"challenge:1","kind":"challenge","name":"June Climb","progress_pct":40.0}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var a achievements.Achievement
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &a))
	require.NotNil(t, a.ProgressPct)
	assert.InDelta(t, 40.0, *a.ProgressPct, 0.001)

	rec2 := do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"challenge:1","kind":"challenge","name":"June Climb","progress_pct":80.46}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &a))
	require.NotNil(t, a.ProgressPct)
	assert.InDelta(t, 80.5, *a.ProgressPct, 0.001, "progress rounded at the boundary")

	all := do(t, r, http.MethodGet, "/achievements", "")
	var out struct {
		Achievements []achievements.Achievement `json:"achievements"`
	}
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &out))
	assert.Len(t, out.Achievements, 1)
}

func TestUpsert_Validation(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"kind":"badge","name":"x"}`:                                      "external_id_required",
		`{"external_id":"a","kind":"trophy","name":"x"}`:                   "kind_invalid",
		`{"external_id":"a","kind":"badge"}`:                               "name_required",
		`{"external_id":"a","kind":"badge","name":"x","progress_pct":150}`: "progress_pct_invalid",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/achievements", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String(), body)
	}
}

func TestList_OrderAndKindFilter(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"badge:1","kind":"badge","name":"Old","earned_at":"2026-05-01T00:00:00Z"}`).Code)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"badge:2","kind":"badge","name":"New","earned_at":"2026-06-01T00:00:00Z"}`).Code)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"challenge:1","kind":"challenge","name":"InProgress","progress_pct":50}`).Code)

	all := do(t, r, http.MethodGet, "/achievements", "")
	var out struct {
		Achievements []achievements.Achievement `json:"achievements"`
	}
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &out))
	require.Len(t, out.Achievements, 3)
	assert.Equal(t, "New", out.Achievements[0].Name, "most recent earned first")
	assert.Equal(t, "InProgress", out.Achievements[2].Name, "NULL earned_at last")

	filtered := do(t, r, http.MethodGet, "/achievements?kind=challenge", "")
	require.NoError(t, json.Unmarshal(filtered.Body.Bytes(), &out))
	require.Len(t, out.Achievements, 1)
	assert.Equal(t, achievements.KindChallenge, out.Achievements[0].Kind)
}

func TestUnitIsolationAndOmitEmpty(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/achievements",
		`{"external_id":"challenge:1","kind":"challenge","name":"x","progress_pct":40}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "earned_at", "null earned_at omitted")
	assert.NotContains(t, body, "kcal")
	assert.NotContains(t, body, "total_ml")
}
