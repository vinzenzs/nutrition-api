package products

// ComputeRecipeNutriments returns the gram-weighted average of each component's
// effective per-100g nutriment value. A field is left nil when no component
// supplied a non-nil value (null in, null out). Components are weighted by
// their `QuantityG`; per-100g math: each component contributes
//
//	value_per_100g * quantity_g
//
// to the numerator and `quantity_g` to the denominator (for components that
// had a non-nil value for that field). The result is the per-100g average over
// the contributing portion of the recipe.
//
// This intentionally does NOT scale by recipe serving_size_g — the stored
// nutriments-per-100g represent the per-100g composition of the recipe as a
// whole, so meal-logging math (`per_100g * quantity_g / 100`) keeps working
// without recipe-specific branches.
func ComputeRecipeNutriments(components []*ComponentWithProduct) Nutriments {
	if len(components) == 0 {
		return Nutriments{}
	}

	type accum struct {
		sum    float64
		weight float64
		has    bool
	}
	macros := map[string]*accum{
		"kcal":     {},
		"protein":  {},
		"carbs":    {},
		"fat":      {},
		"fiber":    {},
		"sugar":    {},
		"salt":     {},
		"iron":     {},
		"calcium":  {},
		"vit_d":    {},
		"vit_b12":  {},
		"vit_c":    {},
		"magnesium": {},
		"potassium": {},
		"zinc":     {},
	}

	add := func(key string, v *float64, qty float64) {
		if v == nil {
			return
		}
		a := macros[key]
		a.sum += *v * qty
		a.weight += qty
		a.has = true
	}

	for _, c := range components {
		q := c.QuantityG
		n := c.ComponentProduct.Nutriments
		add("kcal", n.KcalPer100g, q)
		add("protein", n.ProteinGPer100g, q)
		add("carbs", n.CarbsGPer100g, q)
		add("fat", n.FatGPer100g, q)
		add("fiber", n.FiberGPer100g, q)
		add("sugar", n.SugarGPer100g, q)
		add("salt", n.SaltGPer100g, q)
		add("iron", n.IronMgPer100g, q)
		add("calcium", n.CalciumMgPer100g, q)
		add("vit_d", n.VitaminDMcgPer100g, q)
		add("vit_b12", n.VitaminB12McgPer100g, q)
		add("vit_c", n.VitaminCMgPer100g, q)
		add("magnesium", n.MagnesiumMgPer100g, q)
		add("potassium", n.PotassiumMgPer100g, q)
		add("zinc", n.ZincMgPer100g, q)
	}

	out := Nutriments{}
	resolve := func(key string) *float64 {
		a := macros[key]
		if !a.has || a.weight == 0 {
			return nil
		}
		v := a.sum / a.weight
		return &v
	}
	out.KcalPer100g = resolve("kcal")
	out.ProteinGPer100g = resolve("protein")
	out.CarbsGPer100g = resolve("carbs")
	out.FatGPer100g = resolve("fat")
	out.FiberGPer100g = resolve("fiber")
	out.SugarGPer100g = resolve("sugar")
	out.SaltGPer100g = resolve("salt")
	out.IronMgPer100g = resolve("iron")
	out.CalciumMgPer100g = resolve("calcium")
	out.VitaminDMcgPer100g = resolve("vit_d")
	out.VitaminB12McgPer100g = resolve("vit_b12")
	out.VitaminCMgPer100g = resolve("vit_c")
	out.MagnesiumMgPer100g = resolve("magnesium")
	out.PotassiumMgPer100g = resolve("potassium")
	out.ZincMgPer100g = resolve("zinc")
	return out
}
