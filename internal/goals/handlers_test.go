package goals_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupGoals(t *testing.T) (*gin.Engine, *goals.Repo) {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := goals.NewRepo(pool)
	r := gin.New()
	rg := r.Group("/")
	goals.NewHandlers(repo).Register(rg)
	return r, repo
}

func TestGoalsGet_EmptyReturnsNull(t *testing.T) {
	r, _ := setupGoals(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/goals", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"goals":null}`, rec.Body.String())
}

func TestGoalsPut_CreatesAndRoundTrips(t *testing.T) {
	r, _ := setupGoals(t)
	body := `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"fiber_g":{"min":30}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Goals *goals.Goals `json:"goals"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Goals)
	require.NotNil(t, resp.Goals.Kcal)
	require.NotNil(t, resp.Goals.Kcal.Min)
	require.NotNil(t, resp.Goals.Kcal.Max)
	assert.InDelta(t, 2090, *resp.Goals.Kcal.Min, 0.001)
	assert.InDelta(t, 2310, *resp.Goals.Kcal.Max, 0.001)
	require.NotNil(t, resp.Goals.ProteinG)
	require.NotNil(t, resp.Goals.ProteinG.Min)
	assert.InDelta(t, 150, *resp.Goals.ProteinG.Min, 0.001)
	// fiber_g was supplied as min-only — max stays nil and is omitted on read.
	require.NotNil(t, resp.Goals.FiberG)
	require.NotNil(t, resp.Goals.FiberG.Min)
	assert.Nil(t, resp.Goals.FiberG.Max)

	// GET round-trip carries the unified shape.
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/goals", nil))
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), `"kcal":{"min":2090,"max":2310}`)
}

func TestGoalsPut_ClearsPreviouslySetFields(t *testing.T) {
	r, _ := setupGoals(t)
	first := `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(first)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Second PUT omits protein — it must be cleared (the canonical Bug #1
	// repro the harden-write-paths change also covers).
	second := `{"kcal":{"min":2280,"max":2520}}`
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(second)))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/goals", nil))
	require.Equal(t, http.StatusOK, getRec.Code)
	body := getRec.Body.String()
	assert.Contains(t, body, `"kcal":{"min":2280,"max":2520}`)
	assert.NotContains(t, body, `"protein_g"`, "previously-set field should be cleared by absent-on-PUT")
}

func TestGoalsPut_LegacyKcalTargetIsRejected(t *testing.T) {
	r, _ := setupGoals(t)
	body := `{"kcal_target":2200}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var b map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "goal_value_invalid", b["error"])
	assert.Equal(t, "kcal_target", b["field"])
}

func TestGoalsPut_EmptyRangeIsRejected(t *testing.T) {
	r, _ := setupGoals(t)
	body := `{"protein_g":{}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var b map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "goal_value_invalid", b["error"])
	assert.Equal(t, "protein_g", b["field"])
}

func TestGoalsPut_NegativeValueReturns400(t *testing.T) {
	r, _ := setupGoals(t)
	body := `{"kcal":{"min":-100}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var b map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "goal_value_invalid", b["error"])
	assert.Equal(t, "kcal.min", b["field"])
}

func TestGoalsPut_InvertedRangeReturns400(t *testing.T) {
	r, _ := setupGoals(t)
	body := `{"protein_g":{"min":200,"max":150}}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(body)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var b map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "goal_range_invalid", b["error"])
	assert.Equal(t, "protein_g", b["field"])
}

// Bug #1 reproduction (harden-write-paths): three sequential PUTs (15-field →
// kcal-only → 15-field) followed by GET must return the latest 15 fields. The
// original MCP-driven bug was caused by the wrapper's auto-derived
// Idempotency-Key making the THIRD PUT replay the FIRST cached response — see
// 5.8 / the MCP test for that path. Direct handler calls don't go through the
// middleware, so this test passes as documentation that the backend itself is
// consistent and the only failure point was the wrapper.
func TestGoalsPut_Bug1FullRoundTrip(t *testing.T) {
	r, _ := setupGoals(t)
	full := `{
        "kcal":{"min":2090,"max":2310},
        "protein_g":{"min":150,"max":190},
        "carbs_g":{"min":200,"max":300},
        "fat_g":{"min":60,"max":90},
        "fiber_g":{"min":30},
        "sugar_g":{"max":50},
        "salt_g":{"max":6},
        "iron_mg":{"min":14},
        "calcium_mg":{"min":1000},
        "vitamin_d_mcg":{"min":20},
        "vitamin_b12_mcg":{"min":2.4},
        "vitamin_c_mg":{"min":90},
        "magnesium_mg":{"min":400},
        "potassium_mg":{"min":3500},
        "zinc_mg":{"min":11}
    }`
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(full)))
	require.Equal(t, http.StatusOK, rec1.Code, rec1.Body.String())

	kcalOnly := `{"kcal":{"min":2090,"max":2310}}`
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(kcalOnly)))
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, httptest.NewRequest(http.MethodPut, "/goals", bytes.NewBufferString(full)))
	require.Equal(t, http.StatusOK, rec3.Code)

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/goals", nil))
	require.Equal(t, http.StatusOK, getRec.Code)

	var resp struct {
		Goals *goals.Goals `json:"goals"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Goals)
	// All 15 fields must be present after the third PUT.
	require.NotNil(t, resp.Goals.Kcal)
	require.NotNil(t, resp.Goals.ProteinG)
	require.NotNil(t, resp.Goals.CarbsG)
	require.NotNil(t, resp.Goals.FatG)
	require.NotNil(t, resp.Goals.FiberG)
	require.NotNil(t, resp.Goals.SugarG)
	require.NotNil(t, resp.Goals.SaltG)
	require.NotNil(t, resp.Goals.IronMg)
	require.NotNil(t, resp.Goals.CalciumMg)
	require.NotNil(t, resp.Goals.VitaminDMcg)
	require.NotNil(t, resp.Goals.VitaminB12Mcg)
	require.NotNil(t, resp.Goals.VitaminCMg)
	require.NotNil(t, resp.Goals.MagnesiumMg)
	require.NotNil(t, resp.Goals.PotassiumMg)
	require.NotNil(t, resp.Goals.ZincMg)
}
