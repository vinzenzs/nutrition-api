package goals_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

// setupResolverWithPhases wires the full four-step resolver chain so we can
// exercise the phase_template branch end-to-end.
func setupResolverWithPhases(t *testing.T) (
	*goals.Resolver,
	*goals.Repo,
	*goals.OverridesRepo,
	*trainingphases.PhasesRepo,
	*trainingphases.TemplatesRepo,
) {
	t.Helper()
	pool := storetest.NewPool(t)
	defaults := goals.NewRepo(pool)
	overrides := goals.NewOverridesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	resolver := goals.NewResolver(
		defaults, overrides,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)
	return resolver, defaults, overrides, phRepo, tplRepo
}

func TestEffectiveFor_PhaseTemplateWinsOverDefault(t *testing.T) {
	res, defaults, _, phRepo, tplRepo := setupResolverWithPhases(t)
	ctx := context.Background()

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2090.0), Max: ptrF(2310.0)},
	}))
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: ptrF(2400.0), Max: ptrF(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block-2",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	g, src, phaseName, err := res.EffectiveFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourcePhaseTemplate, src)
	assert.Equal(t, "build-block-2", phaseName)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2400, *g.Kcal.Min, 0.001)
}

func TestEffectiveFor_OverrideWinsOverPhase(t *testing.T) {
	res, _, overrides, phRepo, tplRepo := setupResolverWithPhases(t)
	ctx := context.Background()

	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptrF(2400.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "build", Type: trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, overrides.Upsert(ctx, date, &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2800.0)},
	}))

	g, src, phaseName, err := res.EffectiveFor(ctx, date)
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourceOverride, src)
	assert.Empty(t, phaseName)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2800, *g.Kcal.Min, 0.001)
}

func TestEffectiveFor_PhaseWithoutTemplateFallsThroughToDefault(t *testing.T) {
	res, defaults, _, phRepo, _ := setupResolverWithPhases(t)
	ctx := context.Background()

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2000.0)},
	}))
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "recovery", Type: trainingphases.PhaseTypeRecovery,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		// DefaultTemplateID intentionally nil — phase exists but has no goals.
	}))

	g, src, phaseName, err := res.EffectiveFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourceDefault, src)
	assert.Empty(t, phaseName)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2000, *g.Kcal.Min, 0.001)
}

func TestEffectiveFor_OverlappingPhasesUseMostRecentlyUpdated(t *testing.T) {
	res, _, _, phRepo, tplRepo := setupResolverWithPhases(t)
	ctx := context.Background()

	tplA, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "tpl-a", Kcal: &goals.Range{Min: ptrF(2000.0)},
	})
	require.NoError(t, err)
	tplB, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "tpl-b", Kcal: &goals.Range{Min: ptrF(2800.0)},
	})
	require.NoError(t, err)

	idA, idB := tplA.ID, tplB.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "recovery", Type: trainingphases.PhaseTypeRecovery,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &idA,
	}))
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "race-week", Type: trainingphases.PhaseTypeRaceWeek,
		StartDate:         time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &idB,
	}))

	g, src, phaseName, err := res.EffectiveFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourcePhaseTemplate, src)
	assert.Equal(t, "race-week", phaseName)
	require.NotNil(t, g.Kcal)
	assert.InDelta(t, 2800, *g.Kcal.Min, 0.001)
}

func TestEffectiveForRange_MixesOverridePhaseDefault(t *testing.T) {
	res, defaults, overrides, phRepo, tplRepo := setupResolverWithPhases(t)
	ctx := context.Background()

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2000.0)},
	}))
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptrF(2400.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name: "build", Type: trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))
	require.NoError(t, overrides.Upsert(ctx,
		time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
		&goals.Goals{Kcal: &goals.Range{Min: ptrF(2800.0)}}))

	effective, sources, phaseNames, err := res.EffectiveForRange(ctx,
		time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	// Day 7/4: default. Days 7/5-7/9: phase or override. Day 7/10: default.
	assert.Equal(t, goals.GoalSourceDefault, sources["2026-07-04"])
	assert.Equal(t, goals.GoalSourcePhaseTemplate, sources["2026-07-05"])
	assert.Equal(t, goals.GoalSourcePhaseTemplate, sources["2026-07-06"])
	assert.Equal(t, goals.GoalSourceOverride, sources["2026-07-07"])
	assert.Equal(t, goals.GoalSourcePhaseTemplate, sources["2026-07-08"])
	assert.Equal(t, goals.GoalSourcePhaseTemplate, sources["2026-07-09"])
	assert.Equal(t, goals.GoalSourceDefault, sources["2026-07-10"])

	assert.Empty(t, phaseNames["2026-07-04"])
	assert.Equal(t, "build", phaseNames["2026-07-05"])
	assert.Empty(t, phaseNames["2026-07-07"], "override day has no phase_name")
	assert.Equal(t, "build", phaseNames["2026-07-08"])
	assert.Empty(t, phaseNames["2026-07-10"])

	// Spot-check goals come from the right source.
	assert.InDelta(t, 2400, *effective["2026-07-05"].Kcal.Min, 0.001)
	assert.InDelta(t, 2800, *effective["2026-07-07"].Kcal.Min, 0.001)
	assert.InDelta(t, 2000, *effective["2026-07-10"].Kcal.Min, 0.001)
}

func TestEffectiveFor_DeletedPhaseHasNoResidualEffect(t *testing.T) {
	res, defaults, _, phRepo, tplRepo := setupResolverWithPhases(t)
	ctx := context.Background()

	require.NoError(t, defaults.Upsert(ctx, &goals.Goals{
		Kcal: &goals.Range{Min: ptrF(2000.0)},
	}))
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptrF(2400.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	phase := &trainingphases.Phase{
		Name: "build", Type: trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}
	require.NoError(t, phRepo.Insert(ctx, phase))

	// Before delete: phase drives adherence.
	_, src, _, err := res.EffectiveFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, goals.GoalSourcePhaseTemplate, src)

	// After delete: default takes over.
	require.NoError(t, phRepo.Delete(ctx, phase.ID))
	_, src, _, err = res.EffectiveFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, goals.GoalSourceDefault, src)
}
