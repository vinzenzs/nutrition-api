package cookidoo

// Recipe is the parsed view of a Cookidoo recipe page's Schema.org Recipe
// JSON-LD, normalised to the fields we persist. Pointer fields are nullable —
// Cookidoo pages are inconsistent and an unparseable field becomes nil rather
// than failing the whole import.
type Recipe struct {
	// Name is the recipe title. Always present on a successful parse.
	Name string
	// Ingredients are the verbatim recipeIngredient strings in page order.
	Ingredients []string
	// NutritionPerServing holds the page's per-portion nutrition. Cookidoo
	// reports nutrition per serving with no serving mass, so callers convert to
	// per-100g only when a serving_size_g is supplied.
	NutritionPerServing NutritionPerServing
	// ServingsYield is the parsed leading integer of recipeYield ("6 Portionen"
	// → 6). Nil when absent or unparseable.
	ServingsYield *int
	// TotalTimeMin is the parsed ISO-8601 totalTime in whole minutes. Nil when
	// absent or unparseable.
	TotalTimeMin *int
}

// NutritionPerServing mirrors the macro fields we store, valued per portion.
// Every field is nullable. Micros are not reported by Cookidoo and are omitted.
type NutritionPerServing struct {
	Kcal     *float64
	ProteinG *float64
	CarbsG   *float64
	FatG     *float64
	FiberG   *float64
	SugarG   *float64
	SaltG    *float64
}
