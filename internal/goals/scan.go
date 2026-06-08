package goals

// goalColumns lists the 30 nutrient bound columns shared by `nutrition_goals`
// (post-unify-adherence-shape) and `daily_goal_overrides`. Used to assemble
// SELECT projections and the parameter list for INSERT/UPSERT statements.
var goalColumns = []string{
	"kcal_min", "kcal_max",
	"protein_g_min", "protein_g_max",
	"carbs_g_min", "carbs_g_max",
	"fat_g_min", "fat_g_max",
	"fiber_g_min", "fiber_g_max",
	"sugar_g_min", "sugar_g_max",
	"salt_g_min", "salt_g_max",
	"iron_mg_min", "iron_mg_max",
	"calcium_mg_min", "calcium_mg_max",
	"vitamin_d_mcg_min", "vitamin_d_mcg_max",
	"vitamin_b12_mcg_min", "vitamin_b12_mcg_max",
	"vitamin_c_mg_min", "vitamin_c_mg_max",
	"magnesium_mg_min", "magnesium_mg_max",
	"potassium_mg_min", "potassium_mg_max",
	"zinc_mg_min", "zinc_mg_max",
}

// goalValueArgs returns the 30 ordered min/max values for g, suitable as
// arguments to INSERT/UPDATE statements that follow goalColumns.
func goalValueArgs(g *Goals) []any {
	return []any{
		rangeMin(g.Kcal), rangeMax(g.Kcal),
		rangeMin(g.ProteinG), rangeMax(g.ProteinG),
		rangeMin(g.CarbsG), rangeMax(g.CarbsG),
		rangeMin(g.FatG), rangeMax(g.FatG),
		rangeMin(g.FiberG), rangeMax(g.FiberG),
		rangeMin(g.SugarG), rangeMax(g.SugarG),
		rangeMin(g.SaltG), rangeMax(g.SaltG),
		rangeMin(g.IronMg), rangeMax(g.IronMg),
		rangeMin(g.CalciumMg), rangeMax(g.CalciumMg),
		rangeMin(g.VitaminDMcg), rangeMax(g.VitaminDMcg),
		rangeMin(g.VitaminB12Mcg), rangeMax(g.VitaminB12Mcg),
		rangeMin(g.VitaminCMg), rangeMax(g.VitaminCMg),
		rangeMin(g.MagnesiumMg), rangeMax(g.MagnesiumMg),
		rangeMin(g.PotassiumMg), rangeMax(g.PotassiumMg),
		rangeMin(g.ZincMg), rangeMax(g.ZincMg),
	}
}

// scanGoalRow scans goalColumns + (created_at, updated_at) into a *Goals.
// The caller passes a Scan-able (pgx.Row or pgx.Rows). On pgx.ErrNoRows the
// caller decides whether to return (nil, nil) or surface the error.
func scanGoalRow(s scanner) (*Goals, error) {
	var (
		g                                                                 Goals
		kcalMin, kcalMax                                                  *float64
		proteinMin, proteinMax, carbsMin, carbsMax, fatMin, fatMax        *float64
		fiberMin, fiberMax                                                *float64
		sugarMin, sugarMax                                                *float64
		saltMin, saltMax                                                  *float64
		ironMin, ironMax                                                  *float64
		calciumMin, calciumMax                                            *float64
		vitDMin, vitDMax                                                  *float64
		vitB12Min, vitB12Max                                              *float64
		vitCMin, vitCMax                                                  *float64
		magnesiumMin, magnesiumMax                                        *float64
		potassiumMin, potassiumMax                                        *float64
		zincMin, zincMax                                                  *float64
	)
	err := s.Scan(
		&kcalMin, &kcalMax,
		&proteinMin, &proteinMax,
		&carbsMin, &carbsMax,
		&fatMin, &fatMax,
		&fiberMin, &fiberMax,
		&sugarMin, &sugarMax,
		&saltMin, &saltMax,
		&ironMin, &ironMax,
		&calciumMin, &calciumMax,
		&vitDMin, &vitDMax,
		&vitB12Min, &vitB12Max,
		&vitCMin, &vitCMax,
		&magnesiumMin, &magnesiumMax,
		&potassiumMin, &potassiumMax,
		&zincMin, &zincMax,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	g.Kcal = rangeOrNil(kcalMin, kcalMax)
	g.ProteinG = rangeOrNil(proteinMin, proteinMax)
	g.CarbsG = rangeOrNil(carbsMin, carbsMax)
	g.FatG = rangeOrNil(fatMin, fatMax)
	g.FiberG = rangeOrNil(fiberMin, fiberMax)
	g.SugarG = rangeOrNil(sugarMin, sugarMax)
	g.SaltG = rangeOrNil(saltMin, saltMax)
	g.IronMg = rangeOrNil(ironMin, ironMax)
	g.CalciumMg = rangeOrNil(calciumMin, calciumMax)
	g.VitaminDMcg = rangeOrNil(vitDMin, vitDMax)
	g.VitaminB12Mcg = rangeOrNil(vitB12Min, vitB12Max)
	g.VitaminCMg = rangeOrNil(vitCMin, vitCMax)
	g.MagnesiumMg = rangeOrNil(magnesiumMin, magnesiumMax)
	g.PotassiumMg = rangeOrNil(potassiumMin, potassiumMax)
	g.ZincMg = rangeOrNil(zincMin, zincMax)
	return &g, nil
}

// scanner is the minimal interface both pgx.Row and pgx.Rows satisfy.
type scanner interface {
	Scan(dest ...any) error
}
