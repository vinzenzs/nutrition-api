package hydrationbalance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/hydrationbalance"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := hydrationbalance.NewService(hydrationbalance.NewRepo(pool))
	r := gin.New()
	hydrationbalance.NewHandlers(svc).Register(r.Group("/"))
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
	body := `{"date":"2026-06-09","sweat_loss_ml":2400,"activity_intake_ml":1800,"goal_ml":3000}`
	rec := do(t, r, http.MethodPost, "/hydration-balance", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var s hydrationbalance.Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	assert.Equal(t, "2026-06-09", s.Date)
	require.NotNil(t, s.SweatLossML)
	assert.InDelta(t, 2400, *s.SweatLossML, 0.05)

	// Second POST for same date with fewer fields → 200, full-replace nulls the rest.
	rec2 := do(t, r, http.MethodPost, "/hydration-balance", `{"date":"2026-06-09","sweat_loss_ml":2600}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var s2 hydrationbalance.Snapshot
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &s2))
	assert.InDelta(t, 2600, *s2.SweatLossML, 0.05)
	assert.Nil(t, s2.ActivityIntakeML, "full-replace upsert nulls omitted fields")
	assert.Nil(t, s2.GoalML)
}

func TestUpsert_ActivityIntakeZeroIsStored(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/hydration-balance", `{"date":"2026-06-09","activity_intake_ml":0}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var s hydrationbalance.Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	require.NotNil(t, s.ActivityIntakeML, "a real zero must round-trip, not become null")
	assert.Equal(t, 0.0, *s.ActivityIntakeML)
	assert.Nil(t, s.SweatLossML)
	// And the JSON literally carries the zero.
	assert.Contains(t, rec.Body.String(), `"activity_intake_ml":0`)
}

func TestUpsert_OmittedFieldsOmittedFromResponse(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/hydration-balance", `{"date":"2026-06-09","sweat_loss_ml":2400}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"sweat_loss_ml":2400`)
	assert.NotContains(t, body, `"activity_intake_ml"`)
	assert.NotContains(t, body, `"goal_ml"`)
}

func TestUpsert_MissingOrInvalidDate(t *testing.T) {
	r := setup(t)
	for _, b := range []string{`{"sweat_loss_ml":2400}`, `{"date":"2026-13-99"}`, `{"date":"yesterday"}`} {
		rec := do(t, r, http.MethodPost, "/hydration-balance", b)
		require.Equal(t, http.StatusBadRequest, rec.Code, b)
		assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())
	}
}

func TestUpsert_OutOfRangeMetrics(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"date":"2026-06-09","sweat_loss_ml":0}`:       "sweat_loss_ml_invalid",
		`{"date":"2026-06-09","activity_intake_ml":-1}`: "activity_intake_ml_invalid",
		`{"date":"2026-06-09","goal_ml":0}`:             "goal_ml_invalid",
	}
	for body, want := range cases {
		rec := do(t, r, http.MethodPost, "/hydration-balance", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		var got map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, want, got["error"], body)
	}
}

func TestList_WindowAndCaps(t *testing.T) {
	r := setup(t)
	for _, d := range []string{"2026-06-01", "2026-06-15", "2026-07-05"} {
		require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/hydration-balance", `{"date":"`+d+`","sweat_loss_ml":2000}`).Code)
	}
	rec := do(t, r, http.MethodGet, "/hydration-balance?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		HydrationBalance []hydrationbalance.Snapshot `json:"hydration_balance"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.HydrationBalance, 2)
	assert.Equal(t, "2026-06-01", out.HydrationBalance[0].Date)
	assert.Equal(t, "2026-06-15", out.HydrationBalance[1].Date)

	rec = do(t, r, http.MethodGet, "/hydration-balance?from=2026-06-01", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())

	rec = do(t, r, http.MethodGet, "/hydration-balance?from=2026-01-01&to=2026-12-31", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Equal(t, "range_too_large", m["error"])
}

func TestGetAndDeleteByDate(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/hydration-balance", `{"date":"2026-06-09","sweat_loss_ml":2400}`).Code)

	require.Equal(t, http.StatusOK, do(t, r, http.MethodGet, "/hydration-balance/2026-06-09", "").Code)

	rec := do(t, r, http.MethodGet, "/hydration-balance/2026-06-10", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"hydration_balance_not_found"}`, rec.Body.String())

	require.Equal(t, http.StatusNoContent, do(t, r, http.MethodDelete, "/hydration-balance/2026-06-09", "").Code)
	require.Equal(t, http.StatusNotFound, do(t, r, http.MethodDelete, "/hydration-balance/2026-06-09", "").Code)
}

func TestUnitIsolation_NoForeignFields(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/hydration-balance", `{"date":"2026-06-09","sweat_loss_ml":2400}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	for _, k := range []string{`"total_ml"`, `"entries_count"`, `"kcal"`, `"weight_kg"`} {
		assert.NotContains(t, body, k)
	}
}
