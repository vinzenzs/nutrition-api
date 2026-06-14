package summary_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

// TestSummaryDaily_PhaseTemplateAdherenceAndPhaseName wires the full
// resolver chain (overrides + phases + templates + defaults) and confirms
// that a date covered by a phase with a template produces `goal_source:
// "phase_template"` and `phase_name: "<phase>"` on the daily summary.
func TestSummaryDaily_PhaseTemplateAdherenceAndPhaseName(t *testing.T) {
	pool := storetest.NewPool(t)
	ctx := context.Background()

	defaults := goals.NewRepo(pool)
	overrides := goals.NewOverridesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	resolver := goals.NewResolver(
		defaults, overrides,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)
	mRepo := meals.NewRepo(pool)
	svc := summary.NewService(pool, mRepo, resolver)

	// Pre-seed: template, phase pointing at it.
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: floatPtr(2400.0), Max: floatPtr(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "build-block-2",
		Type: trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	r := gin.New()
	rg := r.Group("/")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	summary.NewHandlers(svc, "UTC", logger).Register(rg)

	req := httptest.NewRequest(http.MethodGet, "/summary/daily?date=2026-07-15", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "phase_template", resp.GoalSource)
	assert.Equal(t, "build-block-2", resp.PhaseName)
	// Adherence rows present for kcal.
	if assert.Contains(t, resp.Adherence, "kcal") {
		entry := resp.Adherence["kcal"]
		require.NotNil(t, entry.Target.Min)
		assert.InDelta(t, 2400, *entry.Target.Min, 0.001)
	}
}

// TestSummaryDaily_PhaseNameAbsentOnDefaultSource confirms phase_name omitted
// when goal_source != phase_template.
func TestSummaryDaily_PhaseNameAbsentOnDefaultSource(t *testing.T) {
	pool := storetest.NewPool(t)
	ctx := context.Background()

	defaults := goals.NewRepo(pool)
	overrides := goals.NewOverridesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	resolver := goals.NewResolver(
		defaults, overrides,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)
	mRepo := meals.NewRepo(pool)
	svc := summary.NewService(pool, mRepo, resolver)

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: floatPtr(2000.0)},
	}))

	r := gin.New()
	rg := r.Group("/")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	summary.NewHandlers(svc, "UTC", logger).Register(rg)

	req := httptest.NewRequest(http.MethodGet, "/summary/daily?date=2026-07-15", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Assert phase_name is absent from the raw JSON, not just empty in the struct.
	assert.NotContains(t, rec.Body.String(), `"phase_name"`,
		"phase_name field must omit (omitempty) on non-phase_template responses")
	var resp summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "default", resp.GoalSource)
	assert.Empty(t, resp.PhaseName)
}

func floatPtr(v float64) *float64 { return &v }
