package products

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func f(v float64) *float64 { return &v }

func TestComputeRecipeNutriments_GramWeightedAverage(t *testing.T) {
	// Two components: 200g of "Skyr" (60 kcal, 11g protein) and 40g of "Oats"
	// (380 kcal, 13g protein). Per-100g average weighted by gram contribution:
	//   kcal:    (60*200 + 380*40) / 240 = (12000 + 15200) / 240 = 113.333...
	//   protein: (11*200 + 13*40)  / 240 = (2200 + 520)    / 240 = 11.333...
	comps := []*ComponentWithProduct{
		{
			Component: Component{QuantityG: 200},
			ComponentProduct: Product{Nutriments: Nutriments{
				KcalPer100g:     f(60),
				ProteinGPer100g: f(11),
			}},
		},
		{
			Component: Component{QuantityG: 40},
			ComponentProduct: Product{Nutriments: Nutriments{
				KcalPer100g:     f(380),
				ProteinGPer100g: f(13),
			}},
		},
	}
	n := ComputeRecipeNutriments(comps)
	require.NotNil(t, n.KcalPer100g)
	assert.InDelta(t, 113.333, *n.KcalPer100g, 0.01)
	require.NotNil(t, n.ProteinGPer100g)
	assert.InDelta(t, 11.333, *n.ProteinGPer100g, 0.01)
}

func TestComputeRecipeNutriments_NullPropagatesWhenAllNull(t *testing.T) {
	// No component supplies an iron value → recipe's iron is nil, not zero.
	comps := []*ComponentWithProduct{
		{
			Component: Component{QuantityG: 100},
			ComponentProduct: Product{Nutriments: Nutriments{
				KcalPer100g: f(50),
			}},
		},
	}
	n := ComputeRecipeNutriments(comps)
	require.NotNil(t, n.KcalPer100g)
	assert.Nil(t, n.IronMgPer100g)
	assert.Nil(t, n.CalciumMgPer100g)
}

func TestComputeRecipeNutriments_PartialNullsUseOnlyContributors(t *testing.T) {
	// Iron is only set on component B (40g, 5 mg). Component A contributes 200g
	// but no iron value, so it's NOT counted in the iron denominator.
	// Expected iron per 100g = (5*40) / 40 = 5 mg.
	comps := []*ComponentWithProduct{
		{
			Component: Component{QuantityG: 200},
			ComponentProduct: Product{Nutriments: Nutriments{
				KcalPer100g: f(100),
			}},
		},
		{
			Component: Component{QuantityG: 40},
			ComponentProduct: Product{Nutriments: Nutriments{
				KcalPer100g:   f(200),
				IronMgPer100g: f(5),
			}},
		},
	}
	n := ComputeRecipeNutriments(comps)
	require.NotNil(t, n.IronMgPer100g)
	assert.InDelta(t, 5.0, *n.IronMgPer100g, 0.001)

	// kcal sees both contributors and is weighted normally:
	// (100*200 + 200*40) / 240 = (20000 + 8000) / 240 = 116.666...
	require.NotNil(t, n.KcalPer100g)
	assert.InDelta(t, 116.666, *n.KcalPer100g, 0.01)
}

func TestComputeRecipeNutriments_NoComponentsReturnsEmpty(t *testing.T) {
	n := ComputeRecipeNutriments(nil)
	assert.Nil(t, n.KcalPer100g)
	assert.Nil(t, n.IronMgPer100g)
}
