package products_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func ptrF(v float64) *float64 { return &v }

func insertSimpleProduct(t *testing.T, repo *products.Repo, name string, kcal, protein float64) *products.Product {
	t.Helper()
	p := &products.Product{
		Name:   name,
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:     ptrF(kcal),
			ProteinGPer100g: ptrF(protein),
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p
}

func TestProductRoundTripWithMicros(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)

	p := &products.Product{
		Name:   "Fortified oat milk",
		Source: products.SourceManual,
		Nutriments: products.Nutriments{
			KcalPer100g:          ptrF(48),
			ProteinGPer100g:      ptrF(1.5),
			CalciumMgPer100g:     ptrF(120),
			VitaminDMcgPer100g:   ptrF(1.5),
			VitaminB12McgPer100g: ptrF(0.4),
		},
	}
	require.NoError(t, repo.Insert(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID)
	require.NoError(t, err)

	require.NotNil(t, got.Nutriments.CalciumMgPer100g)
	assert.InDelta(t, 120, *got.Nutriments.CalciumMgPer100g, 0.001)
	require.NotNil(t, got.Nutriments.VitaminDMcgPer100g)
	assert.InDelta(t, 1.5, *got.Nutriments.VitaminDMcgPer100g, 0.001)
	require.NotNil(t, got.Nutriments.VitaminB12McgPer100g)
	assert.InDelta(t, 0.4, *got.Nutriments.VitaminB12McgPer100g, 0.001)

	// Unsupplied micros stay nil — never coerced to zero.
	assert.Nil(t, got.Nutriments.IronMgPer100g)
	assert.Nil(t, got.Nutriments.PotassiumMgPer100g)
	assert.Nil(t, got.NutrimentComputedAt)
}

func TestComponentsRoundTrip(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)
	crepo := products.NewComponentsRepo(pool)

	skyr := insertSimpleProduct(t, repo, "Skyr", 60, 11)
	oats := insertSimpleProduct(t, repo, "Oats", 380, 13)
	honey := insertSimpleProduct(t, repo, "Honey", 304, 0.3)

	// Recipe parent product (would normally be inserted with computed nutriments).
	recipe := &products.Product{
		Name:   "Morning skyr bowl",
		Source: products.SourceRecipe,
	}
	require.NoError(t, repo.Insert(context.Background(), recipe))

	comps := []products.Component{
		{ComponentProductID: skyr.ID, QuantityG: 200},
		{ComponentProductID: oats.ID, QuantityG: 40},
		{ComponentProductID: honey.ID, QuantityG: 10},
	}
	require.NoError(t, crepo.InsertComponents(context.Background(), recipe.ID, comps))

	listed, err := crepo.ListComponents(context.Background(), recipe.ID)
	require.NoError(t, err)
	require.Len(t, listed, 3)

	// Positions assigned from slice index by default.
	assert.Equal(t, 0, listed[0].Position)
	assert.Equal(t, 1, listed[1].Position)
	assert.Equal(t, 2, listed[2].Position)

	// Components carry the joined product info.
	assert.Equal(t, skyr.ID, listed[0].ComponentProduct.ID)
	assert.Equal(t, "Skyr", listed[0].ComponentProduct.Name)
	assert.Equal(t, 200.0, listed[0].QuantityG)
	assert.Equal(t, oats.ID, listed[1].ComponentProduct.ID)
	assert.Equal(t, honey.ID, listed[2].ComponentProduct.ID)

	require.NoError(t, crepo.DeleteComponents(context.Background(), recipe.ID))
	listed, err = crepo.ListComponents(context.Background(), recipe.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestComponentsListMissingRecipeReturnsEmpty(t *testing.T) {
	pool := storetest.NewPool(t)
	crepo := products.NewComponentsRepo(pool)

	listed, err := crepo.ListComponents(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, listed)
}

func TestUpdateRecipeNutriments(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)

	recipe := &products.Product{
		Name:   "Test recipe",
		Source: products.SourceRecipe,
	}
	require.NoError(t, repo.Insert(context.Background(), recipe))

	n := products.Nutriments{
		KcalPer100g:     ptrF(123.4),
		ProteinGPer100g: ptrF(5.6),
		IronMgPer100g:   ptrF(2.1),
	}
	computedAt := recipe.CreatedAt.Add(0) // any real time; storetest containers have synced clock
	require.NoError(t, repo.UpdateRecipeNutriments(context.Background(), recipe.ID, n, computedAt))

	got, err := repo.GetByID(context.Background(), recipe.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Nutriments.KcalPer100g)
	assert.InDelta(t, 123.4, *got.Nutriments.KcalPer100g, 0.001)
	require.NotNil(t, got.Nutriments.IronMgPer100g)
	assert.InDelta(t, 2.1, *got.Nutriments.IronMgPer100g, 0.001)
	require.NotNil(t, got.NutrimentComputedAt)
}

func TestUpdateRecipeNutrimentsRejectsNonRecipe(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)

	manual := insertSimpleProduct(t, repo, "Plain manual", 100, 5)

	err := repo.UpdateRecipeNutriments(context.Background(), manual.ID, products.Nutriments{
		KcalPer100g: ptrF(200),
	}, manual.CreatedAt)
	assert.ErrorIs(t, err, products.ErrNotFound, "non-recipe rows must not be updateable via UpdateRecipeNutriments")
}

func TestProductRoundTripWithLastLoggedQuantity(t *testing.T) {
	pool := storetest.NewPool(t)
	repo := products.NewRepo(pool)

	p := &products.Product{
		Name:                "Round-trip target",
		Source:              products.SourceManual,
		Nutriments:          products.Nutriments{KcalPer100g: ptrF(100)},
		LastLoggedQuantityG: ptrF(225),
	}
	require.NoError(t, repo.Insert(context.Background(), p))

	got, err := repo.GetByID(context.Background(), p.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LastLoggedQuantityG)
	assert.InDelta(t, 225.0, *got.LastLoggedQuantityG, 0.001)

	// A row inserted with nil must remain nil and JSON-omit the field.
	p2 := &products.Product{Name: "Never logged", Source: products.SourceManual,
		Nutriments: products.Nutriments{KcalPer100g: ptrF(50)}}
	require.NoError(t, repo.Insert(context.Background(), p2))
	got2, err := repo.GetByID(context.Background(), p2.ID)
	require.NoError(t, err)
	assert.Nil(t, got2.LastLoggedQuantityG)
}
