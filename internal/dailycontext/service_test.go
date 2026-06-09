package dailycontext_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
	"github.com/vinzenzs/nutrition-api/internal/dailycontext"
	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/hydration"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/summary"
	"github.com/vinzenzs/nutrition-api/internal/trainingphases"
	"github.com/vinzenzs/nutrition-api/internal/workoutfuel"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

func ptr[T any](v T) *T { return &v }

type fix struct {
	svc            *dailycontext.Service
	meals          *meals.Repo
	hydration      *hydration.Repo
	workouts       *workouts.Repo
	workoutFuel    *workoutfuel.Repo
	bodyWeight     *bodyweight.Repo
	goalsDefault   *goals.Repo
	goalsOverrides *goals.OverridesRepo
	templates      *trainingphases.TemplatesRepo
	phases         *trainingphases.PhasesRepo
}

func setup(t *testing.T) *fix {
	t.Helper()
	pool := storetest.NewPool(t)
	mealsRepo := meals.NewRepo(pool)
	hydrationRepo := hydration.NewRepo(pool)
	workoutsRepo := workouts.NewRepo(pool)
	workoutFuelRepo := workoutfuel.NewRepo(pool)
	bodyWeightRepo := bodyweight.NewRepo(pool)
	goalsRepo := goals.NewRepo(pool)
	overridesRepo := goals.NewOverridesRepo(pool)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	resolver := goals.NewResolver(
		goalsRepo, overridesRepo,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)
	summarySvc := summary.NewService(pool, mealsRepo, resolver)
	svc := dailycontext.NewService(
		summarySvc, hydrationRepo, workoutsRepo, workoutFuelRepo,
		bodyWeightRepo, overridesRepo, phRepo,
	)
	return &fix{
		svc:            svc,
		meals:          mealsRepo,
		hydration:      hydrationRepo,
		workouts:       workoutsRepo,
		workoutFuel:    workoutFuelRepo,
		bodyWeight:     bodyWeightRepo,
		goalsDefault:   goalsRepo,
		goalsOverrides: overridesRepo,
		templates:      tplRepo,
		phases:         phRepo,
	}
}

// TestBuildFor_HappyPath_AllSlicesPopulated is the "shape integrity" guard
// the design called out: seed every slice and assert every nested field.
func TestBuildFor_HappyPath_AllSlicesPopulated(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)
	dayMid := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	// 2 meals via freeform Insert (snapshot nutriments).
	for _, kcal := range []float64{300, 500} {
		_, err := f.meals.Insert(ctx, meals.InsertParams{
			LoggedAt:     dayMid,
			QuantityG:    150,
			SnapshotName: ptr("Test meal"),
			SnapshotNutriments: meals.Nutriments{
				KcalPer100g:     ptr(kcal),
				ProteinGPer100g: ptr(20.0),
			},
		})
		require.NoError(t, err)
	}

	// 3 hydration entries totalling 1500ml.
	for _, ml := range []float64{500, 500, 500} {
		require.NoError(t, f.hydration.Insert(ctx, &hydration.Entry{
			LoggedAt:   dayMid,
			QuantityMl: ml,
		}))
	}

	// 1 workout: 60-minute ride, 600 kcal.
	wkID := uuid.New()
	_, err := f.workouts.Upsert(ctx, &workouts.Workout{
		ID:         wkID,
		Source:     workouts.SourceManual,
		Sport:      workouts.SportBike,
		StartedAt:  time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
		EndedAt:    time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		KcalBurned: ptr(600.0),
	})
	require.NoError(t, err)

	// 2 workout-fuel entries: one linked to the workout, one freestanding.
	require.NoError(t, f.workoutFuel.Insert(ctx, &workoutfuel.Entry{
		LoggedAt:  time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC),
		Name:      "gel",
		CarbsG:    ptr(25.0),
		WorkoutID: &wkID,
	}))
	require.NoError(t, f.workoutFuel.Insert(ctx, &workoutfuel.Entry{
		LoggedAt: dayMid,
		Name:     "post-ride drink",
		CarbsG:   ptr(40.0),
		SodiumMg: ptr(300.0),
	}))

	// Body weight: 1 fresh today + 1 prior 5 days back.
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt: time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC),
		WeightKg: 71.0,
	}))
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt:   time.Date(2026, 7, 15, 7, 0, 0, 0, time.UTC),
		WeightKg:   70.5,
		BodyFatPct: ptr(14.2),
	}))

	// Template + phase covering today.
	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: ptr(2400.0), Max: ptr(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	// Override on today (wins over phase).
	require.NoError(t, f.goalsOverrides.Upsert(ctx, date, &goals.Goals{
		Kcal:   &goals.Range{Min: ptr(2800.0)},
		CarbsG: &goals.Range{Min: ptr(700.0)},
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Top-level echoes.
	assert.Equal(t, "2026-07-15", out.Date)
	assert.Equal(t, "UTC", out.TZ)

	// Adherence: override beats phase, so goal_source=override.
	assert.Equal(t, "override", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	assert.Contains(t, out.Adherence.Adherence, "kcal")

	// Nutrition: 2 meals.
	assert.Equal(t, 2, out.Nutrition.EntriesCount)
	assert.Greater(t, out.Nutrition.Totals.Kcal, 0.0)

	// Hydration: 1500ml across 3 entries.
	assert.InDelta(t, 1500.0, out.Hydration.TotalMl, 0.001)
	assert.Equal(t, 3, out.Hydration.EntriesCount)

	// Workouts: one bike workout, 60 min.
	require.Len(t, out.Workouts, 1)
	w := out.Workouts[0]
	assert.Equal(t, "bike", w.Sport)
	assert.InDelta(t, 60.0, w.DurationMin, 0.001)
	require.NotNil(t, w.KcalBurned)
	assert.InDelta(t, 600.0, *w.KcalBurned, 0.001)

	// Workout-fuel: 2 entries.
	assert.Len(t, out.WorkoutFuel, 2)

	// Weight: fresh same-day entry, NOT carryover.
	require.NotNil(t, out.Weight)
	assert.False(t, out.Weight.IsCarryover)
	assert.InDelta(t, 70.5, out.Weight.WeightKg, 0.001)
	require.NotNil(t, out.Weight.BodyFatPct)
	assert.InDelta(t, 14.2, *out.Weight.BodyFatPct, 0.001)

	// Phase: the build-block phase row.
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
	assert.Equal(t, trainingphases.PhaseTypeBuild, out.Phase.Type)
	require.NotNil(t, out.Phase.DefaultTemplateName)
	assert.Equal(t, "build-default", *out.Phase.DefaultTemplateName)

	// Goal override: present with kcal+carbs.
	assert.True(t, out.GoalOverride.Present)
	require.NotNil(t, out.GoalOverride.Goals)
	require.NotNil(t, out.GoalOverride.Goals.Kcal)
	assert.InDelta(t, 2800.0, *out.GoalOverride.Goals.Kcal.Min, 0.001)
}

// TestBuildFor_EmptyDay returns the bundle with empty arrays and nulls.
func TestBuildFor_EmptyDay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	assert.Equal(t, "2026-07-15", out.Date)
	assert.Equal(t, "none", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	assert.Equal(t, 0, out.Nutrition.EntriesCount)
	assert.InDelta(t, 0.0, out.Hydration.TotalMl, 0.001)
	assert.Equal(t, 0, out.Hydration.EntriesCount)
	// Empty arrays, NOT nil — agents branch on length.
	require.NotNil(t, out.Workouts)
	assert.Empty(t, out.Workouts)
	require.NotNil(t, out.WorkoutFuel)
	assert.Empty(t, out.WorkoutFuel)
	assert.Nil(t, out.Weight)
	assert.Nil(t, out.Phase)
	assert.False(t, out.GoalOverride.Present)
	assert.Nil(t, out.GoalOverride.Goals)
}

// TestBuildFor_WeightCarryover_PriorEntry: no entry today, one 5 days back.
func TestBuildFor_WeightCarryover_PriorEntry(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	priorTS := time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC)
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt: priorTS,
		WeightKg: 71.2,
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	require.NotNil(t, out.Weight)
	assert.True(t, out.Weight.IsCarryover)
	assert.InDelta(t, 71.2, out.Weight.WeightKg, 0.001)
	assert.Equal(t, priorTS.UTC(), out.Weight.LoggedAt.UTC())
}

// TestBuildFor_PhaseDrivesAdherenceWhenNoOverride: phase template wins
// adherence; goal_source=phase_template; phase block also populated.
func TestBuildFor_PhaseDrivesAdherenceWhenNoOverride(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: ptr(2400.0), Max: ptr(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	assert.Equal(t, "phase_template", out.Adherence.GoalSource)
	assert.Equal(t, "build-block", out.Adherence.PhaseName)
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
}

// TestBuildFor_PhasePersistsEvenWhenOverrideWins: phase block populated
// even when goal_source=override (the phase still covers the date).
func TestBuildFor_PhasePersistsEvenWhenOverrideWins(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptr(2400.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))
	require.NoError(t, f.goalsOverrides.Upsert(ctx, date, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2800.0)},
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	assert.Equal(t, "override", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	// Phase block still populated.
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
}

// TestBuildFor_NoGoroutineLeak_UnderRace runs the happy-path fixture many
// times to validate the errgroup composition under -race. Goroutine leaks
// or racy access would show up as a flake or `go test -race` complaint.
func TestBuildFor_NoGoroutineLeak_UnderRace(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	// Light fixture — just enough that all goroutines have work.
	require.NoError(t, f.hydration.Insert(context.Background(), &hydration.Entry{
		LoggedAt: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), QuantityMl: 500,
	}))
	for i := 0; i < 50; i++ {
		_, err := f.svc.BuildFor(ctx, date, loc)
		require.NoError(t, err)
	}
}
