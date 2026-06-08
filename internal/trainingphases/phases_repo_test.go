package trainingphases_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/trainingphases"
)

func setupPhases(t *testing.T) (*trainingphases.PhasesRepo, *trainingphases.TemplatesRepo, context.Context) {
	t.Helper()
	pool := storetest.NewPool(t)
	return trainingphases.NewPhasesRepo(pool),
		trainingphases.NewTemplatesRepo(pool),
		context.Background()
}

func TestPhasesRepo_InsertAndGet(t *testing.T) {
	repo, tplRepo, ctx := setupPhases(t)
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "easy", Kcal: &goals.Range{Min: ptr(2200.0)},
	})
	require.NoError(t, err)

	tid := tpl.ID
	p := &trainingphases.Phase{
		Name:              "build-block-2",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
		Notes:             ptr("weeks 5-8 of 16-week plan"),
	}
	require.NoError(t, repo.Insert(ctx, p))
	require.NotEqual(t, uuid.Nil, p.ID)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "build-block-2", got.Name)
	assert.Equal(t, trainingphases.PhaseTypeBuild, got.Type)
	require.NotNil(t, got.DefaultTemplateID)
	assert.Equal(t, tpl.ID, *got.DefaultTemplateID)
	require.NotNil(t, got.DefaultTemplateName, "JOIN should populate template name")
	assert.Equal(t, "easy", *got.DefaultTemplateName)
}

func TestPhasesRepo_GetByIDMissing(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	_, err := repo.GetByID(ctx, uuid.New())
	assert.ErrorIs(t, err, trainingphases.ErrPhaseNotFound)
}

func TestPhasesRepo_ListIntersecting(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	insertPhase := func(name string, start, end time.Time) {
		require.NoError(t, repo.Insert(ctx, &trainingphases.Phase{
			Name:      name,
			Type:      trainingphases.PhaseTypeBuild,
			StartDate: start,
			EndDate:   end,
		}))
	}
	insertPhase("inside",
		time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC))
	insertPhase("overlap-start",
		time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC))
	insertPhase("overlap-end",
		time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC))
	insertPhase("outside",
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))

	out, err := repo.ListIntersecting(ctx,
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, out, 3)
	names := []string{out[0].Name, out[1].Name, out[2].Name}
	assert.NotContains(t, names, "outside")
	assert.Contains(t, names, "inside")
	assert.Contains(t, names, "overlap-start")
	assert.Contains(t, names, "overlap-end")
}

func TestPhasesRepo_PhaseForReturnsMostRecentOnOverlap(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	d := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	// Phase A: covers d, inserted first.
	a := &trainingphases.Phase{
		Name: "phase-a", Type: trainingphases.PhaseTypeRecovery,
		StartDate: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Insert(ctx, a))

	// Phase B: also covers d, inserted later → updated_at newer.
	// Force a clock gap so updated_at differs.
	time.Sleep(50 * time.Millisecond)
	b := &trainingphases.Phase{
		Name: "phase-b", Type: trainingphases.PhaseTypeRaceWeek,
		StartDate: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Insert(ctx, b))

	got, err := repo.PhaseFor(ctx, d)
	require.NoError(t, err)
	assert.Equal(t, "phase-b", got.Name, "most-recently-updated wins")
}

func TestPhasesRepo_PhaseForNoMatch(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	_, err := repo.PhaseFor(ctx, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	assert.ErrorIs(t, err, trainingphases.ErrPhaseNotFound)
}

func TestPhasesRepo_Patch(t *testing.T) {
	repo, tplRepo, ctx := setupPhases(t)
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "easy", Kcal: &goals.Range{Min: ptr(2200.0)},
	})
	require.NoError(t, err)

	p := &trainingphases.Phase{
		Name:      "build-block-2",
		Type:      trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Insert(ctx, p))

	tid := tpl.ID
	newName := "build-block-2-revised"
	require.NoError(t, repo.Patch(ctx, p.ID, trainingphases.PatchParams{
		Name:              &newName,
		DefaultTemplateID: &tid,
	}))

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "build-block-2-revised", got.Name)
	require.NotNil(t, got.DefaultTemplateID)
	assert.Equal(t, tpl.ID, *got.DefaultTemplateID)
	// Dates unchanged.
	assert.Equal(t, p.StartDate.UTC(), got.StartDate.UTC())
}

func TestPhasesRepo_PatchClearsDefaultTemplate(t *testing.T) {
	repo, tplRepo, ctx := setupPhases(t)
	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "easy", Kcal: &goals.Range{Min: ptr(2200.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID

	p := &trainingphases.Phase{
		Name:              "build-block-2",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}
	require.NoError(t, repo.Insert(ctx, p))

	require.NoError(t, repo.Patch(ctx, p.ID, trainingphases.PatchParams{
		ClearDefaultTemplateID: true,
	}))

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, got.DefaultTemplateID)
	assert.Nil(t, got.DefaultTemplateName)
}

func TestPhasesRepo_Delete(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	p := &trainingphases.Phase{
		Name: "block", Type: trainingphases.PhaseTypeBuild,
		StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Insert(ctx, p))
	require.NoError(t, repo.Delete(ctx, p.ID))
	_, err := repo.GetByID(ctx, p.ID)
	assert.ErrorIs(t, err, trainingphases.ErrPhaseNotFound)
}

func TestPhasesRepo_DeleteMissingReturnsErrNotFound(t *testing.T) {
	repo, _, ctx := setupPhases(t)
	err := repo.Delete(ctx, uuid.New())
	assert.ErrorIs(t, err, trainingphases.ErrPhaseNotFound)
}
