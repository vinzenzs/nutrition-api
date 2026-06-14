package coachcontext_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/coachcontext"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func ptr[T any](v T) *T { return &v }

type fix struct {
	svc      *coachcontext.Service
	workouts *workouts.Repo
	fitness  *fitnessmetrics.Repo
	recovery *recoverymetrics.Repo
	phases   *trainingphases.PhasesRepo
}

func setup(t *testing.T) *fix {
	t.Helper()
	pool := storetest.NewPool(t)
	w := workouts.NewRepo(pool)
	fm := fitnessmetrics.NewRepo(pool)
	rm := recoverymetrics.NewRepo(pool)
	ph := trainingphases.NewPhasesRepo(pool)
	return &fix{
		svc:      coachcontext.NewService(w, fm, rm, ph),
		workouts: w, fitness: fm, recovery: rm, phases: ph,
	}
}

func seedWorkout(t *testing.T, f *fix, day time.Time, sport workouts.Sport, status workouts.Status, durMin float64, kcal float64) {
	t.Helper()
	var kcalPtr *float64
	if kcal > 0 {
		kcalPtr = ptr(kcal)
	}
	_, err := f.workouts.Upsert(context.Background(), &workouts.Workout{
		ID:         uuid.New(),
		Source:     workouts.SourceManual,
		Sport:      sport,
		Status:     status,
		StartedAt:  day,
		EndedAt:    day.Add(time.Duration(durMin) * time.Minute),
		KcalBurned: kcalPtr,
	})
	require.NoError(t, err)
}

func TestBuildTraining_HappyPath(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	// Phase covering the date.
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:      "build-block",
		Type:      trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, loc),
	}))

	// Fitness: an older snapshot and the anchor-day one (latest wins). ACWR=1.2.
	_, err := f.fitness.Upsert(ctx, &fitnessmetrics.Snapshot{Date: "2026-07-10", VO2MaxRunning: ptr(52.0)})
	require.NoError(t, err)
	_, err = f.fitness.Upsert(ctx, &fitnessmetrics.Snapshot{
		Date: "2026-07-15", VO2MaxRunning: ptr(54.0), AcuteLoad: ptr(60.0), ChronicLoad: ptr(50.0),
	})
	require.NoError(t, err)

	// Completed workouts: two in the lookback window, one well outside it.
	seedWorkout(t, f, time.Date(2026, 7, 13, 9, 0, 0, 0, loc), workouts.SportRun, workouts.StatusCompleted, 60, 600)
	seedWorkout(t, f, time.Date(2026, 7, 14, 9, 0, 0, 0, loc), workouts.SportBike, workouts.StatusCompleted, 90, 800)
	seedWorkout(t, f, time.Date(2026, 6, 20, 9, 0, 0, 0, loc), workouts.SportRun, workouts.StatusCompleted, 45, 400) // outside 14d

	// Planned workouts in the lookahead window.
	seedWorkout(t, f, time.Date(2026, 7, 16, 9, 0, 0, 0, loc), workouts.SportSwim, workouts.StatusPlanned, 45, 0)
	seedWorkout(t, f, time.Date(2026, 7, 18, 9, 0, 0, 0, loc), workouts.SportRun, workouts.StatusPlanned, 75, 0)

	out, err := f.svc.BuildTraining(ctx, date, loc, 0, 0)
	require.NoError(t, err)

	assert.Equal(t, "2026-07-15", out.Date)
	assert.Equal(t, 14, out.LookbackDays)
	assert.Equal(t, 7, out.LookaheadDays)

	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)

	require.NotNil(t, out.Fitness)
	assert.Equal(t, "2026-07-15", out.Fitness.Date, "latest snapshot wins")
	require.NotNil(t, out.ACWR)
	assert.InDelta(t, 1.2, *out.ACWR, 0.001)

	// Recent load: the two in-window completed workouts only.
	assert.Equal(t, 2, out.RecentLoad.Count)
	assert.InDelta(t, 150.0, out.RecentLoad.TotalDurationMin, 0.001)
	assert.InDelta(t, 1400.0, out.RecentLoad.TotalKcal, 0.001)
	assert.Equal(t, 1, out.RecentLoad.BySport["run"])
	assert.Equal(t, 1, out.RecentLoad.BySport["bike"])
	assert.Len(t, out.RecentWorkouts, 2)

	// Upcoming: the two planned.
	require.Len(t, out.UpcomingWorkouts, 2)
	for _, w := range out.UpcomingWorkouts {
		assert.Equal(t, "planned", w.Status)
	}
}

func TestBuildTraining_CoveringPhaseCarriesMethodology(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:        "build-block",
		Type:        trainingphases.PhaseTypeBuild,
		StartDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		EndDate:     time.Date(2026, 7, 28, 0, 0, 0, 0, loc),
		Methodology: ptr("## Build\nPolarized per Seiler — 80/20."),
	}))

	out, err := f.svc.BuildTraining(ctx, date, loc, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Phase)
	require.NotNil(t, out.Phase.Methodology, "covering phase's methodology rides the bundle")
	assert.Contains(t, *out.Phase.Methodology, "Seiler")
}

func TestBuildTraining_PhaseWithoutMethodologyIsNull(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:      "build-block",
		Type:      trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, loc),
	}))

	out, err := f.svc.BuildTraining(ctx, date, loc, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Phase)
	assert.Nil(t, out.Phase.Methodology, "no methodology set → null in the bundle")
}

func TestBuildTraining_EmptyIsNotError(t *testing.T) {
	f := setup(t)
	out, err := f.svc.BuildTraining(context.Background(), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), time.UTC, 0, 0)
	require.NoError(t, err)
	assert.Nil(t, out.Phase)
	assert.Nil(t, out.Fitness)
	assert.Nil(t, out.ACWR)
	assert.Equal(t, 0, out.RecentLoad.Count)
	assert.NotNil(t, out.RecentWorkouts)
	assert.Empty(t, out.RecentWorkouts)
	assert.NotNil(t, out.UpcomingWorkouts)
	assert.Empty(t, out.UpcomingWorkouts)
}

func TestBuildTraining_ClampsWindows(t *testing.T) {
	f := setup(t)
	out, err := f.svc.BuildTraining(context.Background(), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), time.UTC, 9999, -5)
	require.NoError(t, err)
	assert.Equal(t, 90, out.LookbackDays, "lookback clamped to max")
	assert.Equal(t, 7, out.LookaheadDays, "non-positive lookahead → default")
}

func TestBuildRecovery_LatestAndTrend(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, d := range []string{"2026-07-13", "2026-07-14", "2026-07-15"} {
		_, err := f.recovery.Upsert(ctx, &recoverymetrics.Snapshot{Date: d, SleepScore: ptr(80), HRVMs: ptr(45.0)})
		require.NoError(t, err)
	}
	// One outside the 7-day window.
	_, err := f.recovery.Upsert(ctx, &recoverymetrics.Snapshot{Date: "2026-07-01", SleepScore: ptr(70)})
	require.NoError(t, err)

	out, err := f.svc.BuildRecovery(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), 7)
	require.NoError(t, err)
	assert.Equal(t, 7, out.Days)
	require.NotNil(t, out.Latest)
	assert.Equal(t, "2026-07-15", out.Latest.Date)
	require.Len(t, out.Recent, 3, "only the in-window snapshots")
	assert.Equal(t, "2026-07-13", out.Recent[0].Date, "ascending")
}

func TestBuildRecovery_EmptyIsNotError(t *testing.T) {
	f := setup(t)
	out, err := f.svc.BuildRecovery(context.Background(), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), 7)
	require.NoError(t, err)
	assert.Nil(t, out.Latest)
	assert.NotNil(t, out.Recent)
	assert.Empty(t, out.Recent)
}
