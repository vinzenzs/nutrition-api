package cookidoo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureHTML mirrors the structure of a real cookidoo.de recipe page (captured
// 2026-06-11 from r386806): two ld+json blocks, one Recipe and one
// AggregateRating, with the Recipe's per-serving nutrition and a verbatim
// ingredient list — including " Butter" (leading space) to prove we don't trim.
const fixtureHTML = `<!doctype html><html><head>
<script type="application/ld+json">
{"@context":"http://schema.org/","@type":"AggregateRating","ratingValue":"4.6","ratingCount":"123"}
</script>
<script type="application/ld+json">
{"@context":"http://schema.org/","@type":"Recipe","name":"Vegetarische Linsen-Lasagne",
"recipeYield":"6 Portionen","totalTime":"PT1H30M",
"recipeIngredient":["1 Zwiebel","100 g Staudensellerie"," Butter","400 g Feta"],
"nutrition":{"@type":"NutritionInformation","calories":"589 kcal","carbohydrateContent":"45 g","fatContent":"32 g","proteinContent":"24 g"}}
</script>
</head><body></body></html>`

func TestParseRecipe_HappyPath(t *testing.T) {
	r, err := ParseRecipe([]byte(fixtureHTML))
	require.NoError(t, err)

	assert.Equal(t, "Vegetarische Linsen-Lasagne", r.Name)
	// Ingredients verbatim and in order, including the leading-space " Butter".
	assert.Equal(t, []string{"1 Zwiebel", "100 g Staudensellerie", " Butter", "400 g Feta"}, r.Ingredients)

	require.NotNil(t, r.ServingsYield)
	assert.Equal(t, 6, *r.ServingsYield)
	require.NotNil(t, r.TotalTimeMin)
	assert.Equal(t, 90, *r.TotalTimeMin)

	require.NotNil(t, r.NutritionPerServing.Kcal)
	assert.InDelta(t, 589.0, *r.NutritionPerServing.Kcal, 0.001)
	require.NotNil(t, r.NutritionPerServing.CarbsG)
	assert.InDelta(t, 45.0, *r.NutritionPerServing.CarbsG, 0.001)
	require.NotNil(t, r.NutritionPerServing.FatG)
	assert.InDelta(t, 32.0, *r.NutritionPerServing.FatG, 0.001)
	require.NotNil(t, r.NutritionPerServing.ProteinG)
	assert.InDelta(t, 24.0, *r.NutritionPerServing.ProteinG, 0.001)
	// Fields the page didn't provide stay nil.
	assert.Nil(t, r.NutritionPerServing.FiberG)
	assert.Nil(t, r.NutritionPerServing.SaltG)
}

func TestParseRecipe_NoJSONLD(t *testing.T) {
	_, err := ParseRecipe([]byte(`<html><head><title>not a recipe</title></head><body>hello</body></html>`))
	assert.ErrorIs(t, err, ErrNoRecipeJSONLD)
}

func TestParseRecipe_JSONLDButNoRecipeType(t *testing.T) {
	page := `<html><head>
<script type="application/ld+json">{"@type":"WebPage","name":"x"}</script>
</head></html>`
	_, err := ParseRecipe([]byte(page))
	assert.ErrorIs(t, err, ErrNoRecipeJSONLD)
}

func TestParseRecipe_MalformedBlockSkippedNotFatal(t *testing.T) {
	// A broken JSON-LD block must not prevent parsing a later valid Recipe block.
	page := `<html><head>
<script type="application/ld+json">{ this is not json </script>
<script type="application/ld+json">{"@type":"Recipe","name":"Soup","recipeIngredient":["water"]}</script>
</head></html>`
	r, err := ParseRecipe([]byte(page))
	require.NoError(t, err)
	assert.Equal(t, "Soup", r.Name)
	assert.Equal(t, []string{"water"}, r.Ingredients)
}

func TestParseRecipe_GraphContainer(t *testing.T) {
	// Some pages wrap entities in @graph rather than separate script blocks.
	page := `<html><head>
<script type="application/ld+json">
{"@context":"http://schema.org/","@graph":[
  {"@type":"Organization","name":"Vorwerk"},
  {"@type":"Recipe","name":"Curry","recipeIngredient":["1 onion"],"recipeYield":4}
]}
</script></head></html>`
	r, err := ParseRecipe([]byte(page))
	require.NoError(t, err)
	assert.Equal(t, "Curry", r.Name)
	require.NotNil(t, r.ServingsYield)
	assert.Equal(t, 4, *r.ServingsYield)
}

func TestParseRecipe_TypeAsArray(t *testing.T) {
	page := `<script type="application/ld+json">{"@type":["Recipe","Thing"],"name":"Stew","recipeIngredient":["beans"]}</script>`
	r, err := ParseRecipe([]byte(page))
	require.NoError(t, err)
	assert.Equal(t, "Stew", r.Name)
}

func TestParseNumberWithUnit_CommaDecimalsAndUnits(t *testing.T) {
	cases := map[string]float64{
		"589 kcal": 589,
		"1,5 g":    1.5,
		"45 g":     45,
		"0.107 g":  0.107,
	}
	for in, want := range cases {
		got := parseNumberWithUnit(in)
		require.NotNil(t, got, "input %q", in)
		assert.InDelta(t, want, *got, 0.0001, "input %q", in)
	}
	assert.Nil(t, parseNumberWithUnit("no number here"))
	assert.Nil(t, parseNumberWithUnit(""))
}

func TestParseISODurationMinutes(t *testing.T) {
	cases := map[string]int{
		"PT1H30M":   90,
		"PT45M":     45,
		"PT2H":      120,
		"P1DT2H":    24*60 + 120,
		"PT1H30M0S": 90,
	}
	for in, want := range cases {
		got := parseISODurationMinutes(in)
		require.NotNil(t, got, "input %q", in)
		assert.Equal(t, want, *got, "input %q", in)
	}
	assert.Nil(t, parseISODurationMinutes(""))
	assert.Nil(t, parseISODurationMinutes("90 minutes"))
}

func TestParseYield_Variants(t *testing.T) {
	require.Equal(t, 6, *parseYield("6 Portionen"))
	require.Equal(t, 4, *parseYield(float64(4)))
	require.Equal(t, 8, *parseYield([]any{"8 servings"}))
	assert.Nil(t, parseYield("no number"))
	assert.Nil(t, parseYield(nil))
}
