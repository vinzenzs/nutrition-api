package goals

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// singletonID is the fixed primary key of the one allowed nutrition_goals row.
const singletonID = "00000000-0000-0000-0000-000000000001"

// Repo persists the nutrition_goals singleton row.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const goalsSelect = `
    kcal_min, kcal_max,
    protein_g_min, protein_g_max,
    carbs_g_min,   carbs_g_max,
    fat_g_min,     fat_g_max,
    fiber_g_min,   fiber_g_max,
    sugar_g_min,   sugar_g_max,
    salt_g_min,    salt_g_max,
    iron_mg_min, iron_mg_max,
    calcium_mg_min, calcium_mg_max,
    vitamin_d_mcg_min, vitamin_d_mcg_max,
    vitamin_b12_mcg_min, vitamin_b12_mcg_max,
    vitamin_c_mg_min, vitamin_c_mg_max,
    magnesium_mg_min, magnesium_mg_max,
    potassium_mg_min, potassium_mg_max,
    zinc_mg_min, zinc_mg_max,
    created_at, updated_at
`

// Get returns the goals row, or (nil, nil) if no row exists yet. Other errors
// are propagated. The nil-row signal is distinct from any DB error so the
// handler layer can return `{"goals": null}` straightforwardly.
func (r *Repo) Get(ctx context.Context) (*Goals, error) {
	row := r.q.QueryRow(ctx, `SELECT `+goalsSelect+` FROM nutrition_goals WHERE id = $1`, singletonID)
	g, err := scanGoalRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan goals: %w", err)
	}
	return g, nil
}

// Upsert writes the goals row, replacing all field values with what's on g.
// Absent fields (nil pointers on g) overwrite to NULL — this is full-replace
// PUT semantics per the spec.
func (r *Repo) Upsert(ctx context.Context, g *Goals) error {
	const q = `
        INSERT INTO nutrition_goals (
            id,
            kcal_min, kcal_max,
            protein_g_min, protein_g_max,
            carbs_g_min,   carbs_g_max,
            fat_g_min,     fat_g_max,
            fiber_g_min,   fiber_g_max,
            sugar_g_min,   sugar_g_max,
            salt_g_min,    salt_g_max,
            iron_mg_min, iron_mg_max,
            calcium_mg_min, calcium_mg_max,
            vitamin_d_mcg_min, vitamin_d_mcg_max,
            vitamin_b12_mcg_min, vitamin_b12_mcg_max,
            vitamin_c_mg_min, vitamin_c_mg_max,
            magnesium_mg_min, magnesium_mg_max,
            potassium_mg_min, potassium_mg_max,
            zinc_mg_min, zinc_mg_max,
            updated_at
        ) VALUES (
            $1,
            $2, $3,
            $4, $5,
            $6, $7,
            $8, $9,
            $10, $11,
            $12, $13,
            $14, $15,
            $16, $17,
            $18, $19,
            $20, $21,
            $22, $23,
            $24, $25,
            $26, $27,
            $28, $29,
            $30, $31,
            now()
        )
        ON CONFLICT (id) DO UPDATE SET
            kcal_min            = EXCLUDED.kcal_min,
            kcal_max            = EXCLUDED.kcal_max,
            protein_g_min       = EXCLUDED.protein_g_min,
            protein_g_max       = EXCLUDED.protein_g_max,
            carbs_g_min         = EXCLUDED.carbs_g_min,
            carbs_g_max         = EXCLUDED.carbs_g_max,
            fat_g_min           = EXCLUDED.fat_g_min,
            fat_g_max           = EXCLUDED.fat_g_max,
            fiber_g_min         = EXCLUDED.fiber_g_min,
            fiber_g_max         = EXCLUDED.fiber_g_max,
            sugar_g_min         = EXCLUDED.sugar_g_min,
            sugar_g_max         = EXCLUDED.sugar_g_max,
            salt_g_min          = EXCLUDED.salt_g_min,
            salt_g_max          = EXCLUDED.salt_g_max,
            iron_mg_min         = EXCLUDED.iron_mg_min,
            iron_mg_max         = EXCLUDED.iron_mg_max,
            calcium_mg_min      = EXCLUDED.calcium_mg_min,
            calcium_mg_max      = EXCLUDED.calcium_mg_max,
            vitamin_d_mcg_min   = EXCLUDED.vitamin_d_mcg_min,
            vitamin_d_mcg_max   = EXCLUDED.vitamin_d_mcg_max,
            vitamin_b12_mcg_min = EXCLUDED.vitamin_b12_mcg_min,
            vitamin_b12_mcg_max = EXCLUDED.vitamin_b12_mcg_max,
            vitamin_c_mg_min    = EXCLUDED.vitamin_c_mg_min,
            vitamin_c_mg_max    = EXCLUDED.vitamin_c_mg_max,
            magnesium_mg_min    = EXCLUDED.magnesium_mg_min,
            magnesium_mg_max    = EXCLUDED.magnesium_mg_max,
            potassium_mg_min    = EXCLUDED.potassium_mg_min,
            potassium_mg_max    = EXCLUDED.potassium_mg_max,
            zinc_mg_min         = EXCLUDED.zinc_mg_min,
            zinc_mg_max         = EXCLUDED.zinc_mg_max,
            updated_at          = now()
    `
	_, err := r.q.Exec(ctx, q,
		singletonID,
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
	)
	if err != nil {
		return fmt.Errorf("upsert goals: %w", err)
	}
	return nil
}

func rangeOrNil(min, max *float64) *Range {
	if min == nil && max == nil {
		return nil
	}
	return &Range{Min: min, Max: max}
}

func rangeMin(r *Range) *float64 {
	if r == nil {
		return nil
	}
	return r.Min
}

func rangeMax(r *Range) *float64 {
	if r == nil {
		return nil
	}
	return r.Max
}
