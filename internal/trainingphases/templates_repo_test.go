package trainingphases_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

func ptr[T any](v T) *T { return &v }

func TestTemplatesRepo_UpsertAndGetByName(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	ctx := context.Background()

	in := &trainingphases.Template{
		Name:     "weekday-easy-training",
		Notes:    ptr("baseline for easy training days"),
		Kcal:     &goals.Range{Min: ptr(2090.0), Max: ptr(2310.0)},
		ProteinG: &goals.Range{Min: ptr(150.0), Max: ptr(190.0)},
		CarbsG:   &goals.Range{Min: ptr(280.0), Max: ptr(340.0)},
	}
	stored, err := repo.Upsert(ctx, in)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, stored.ID)
	assert.Equal(t, "weekday-easy-training", stored.Name)
	require.NotNil(t, stored.Notes)
	assert.Equal(t, "baseline for easy training days", *stored.Notes)
	require.NotNil(t, stored.Kcal)
	assert.InDelta(t, 2090, *stored.Kcal.Min, 0.001)

	got, err := repo.GetByName(ctx, "weekday-easy-training")
	require.NoError(t, err)
	assert.Equal(t, stored.ID, got.ID)
}

func TestTemplatesRepo_UpsertReplacesWholesale(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	ctx := context.Background()

	_, err := repo.Upsert(ctx, &trainingphases.Template{
		Name:     "weekday-easy-training",
		Kcal:     &goals.Range{Min: ptr(2000.0)},
		ProteinG: &goals.Range{Min: ptr(150.0)},
	})
	require.NoError(t, err)

	// Second upsert omits protein — full-replace should clear it.
	_, err = repo.Upsert(ctx, &trainingphases.Template{
		Name: "weekday-easy-training",
		Kcal: &goals.Range{Min: ptr(2400.0)},
	})
	require.NoError(t, err)

	got, err := repo.GetByName(ctx, "weekday-easy-training")
	require.NoError(t, err)
	require.NotNil(t, got.Kcal)
	assert.InDelta(t, 2400, *got.Kcal.Min, 0.001)
	assert.Nil(t, got.ProteinG, "PUT full-replace must clear previously-set fields")
}

func TestTemplatesRepo_GetByNameMissingReturnsErrNotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	_, err := repo.GetByName(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, trainingphases.ErrTemplateNotFound)
}

func TestTemplatesRepo_List(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	ctx := context.Background()

	for _, name := range []string{"weekday-hard", "weekday-easy", "race-week"} {
		_, err := repo.Upsert(ctx, &trainingphases.Template{
			Name: name, Kcal: &goals.Range{Min: ptr(2000.0)},
		})
		require.NoError(t, err)
	}
	out, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, out, 3)
	// Sorted ascending.
	assert.Equal(t, "race-week", out[0].Name)
	assert.Equal(t, "weekday-easy", out[1].Name)
	assert.Equal(t, "weekday-hard", out[2].Name)
}

func TestTemplatesRepo_DeleteHappyPath(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	ctx := context.Background()
	_, err := repo.Upsert(ctx, &trainingphases.Template{
		Name: "weekday-easy", Kcal: &goals.Range{Min: ptr(2000.0)},
	})
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, "weekday-easy"))
	_, err = repo.GetByName(ctx, "weekday-easy")
	assert.ErrorIs(t, err, trainingphases.ErrTemplateNotFound)
}

func TestTemplatesRepo_DeleteMissingReturnsErrNotFound(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	err := repo.Delete(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, trainingphases.ErrTemplateNotFound)
}

func TestTemplatesRepo_DeleteRefusedWhenReferenced(t *testing.T) {
	pool := storetest.NewPool(t)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	ctx := context.Background()

	tpl, err := tplRepo.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptr(2200.0)},
	})
	require.NoError(t, err)

	templateID := tpl.ID
	require.NoError(t, phRepo.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block-2",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &templateID,
	}))

	err = tplRepo.Delete(ctx, "build-default")
	require.Error(t, err)
	var inUse *trainingphases.InUseError
	require.ErrorAs(t, err, &inUse, "FK-RESTRICT must surface as InUseError")
	require.Len(t, inUse.ReferencingPhases, 1)
	assert.Equal(t, "build-block-2", inUse.ReferencingPhases[0].Name)
}

func TestTemplatesRepo_GetByIDsBatch(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := trainingphases.NewTemplatesRepo(pool)
	ctx := context.Background()

	t1, err := repo.Upsert(ctx, &trainingphases.Template{
		Name: "t1", Kcal: &goals.Range{Min: ptr(2000.0)},
	})
	require.NoError(t, err)
	t2, err := repo.Upsert(ctx, &trainingphases.Template{
		Name: "t2", Kcal: &goals.Range{Min: ptr(2100.0)},
	})
	require.NoError(t, err)

	got, err := repo.GetByIDs(ctx, []uuid.UUID{t1.ID, t2.ID, uuid.New()})
	require.NoError(t, err)
	require.Len(t, got, 2, "non-existent IDs are silently omitted")
	assert.Contains(t, got, t1.ID)
	assert.Contains(t, got, t2.ID)
}
