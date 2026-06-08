package goals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrOverrideNotFound is returned when no override row exists for a date.
var ErrOverrideNotFound = errors.New("override not found")

// Override pairs a date with the goal set stored for it.
type Override struct {
	Date  time.Time
	Goals *Goals
}

// OverridesRepo persists per-date goal overrides.
type OverridesRepo struct {
	q store.Querier
}

func NewOverridesRepo(q store.Querier) *OverridesRepo {
	return &OverridesRepo{q: q}
}

const overridesTable = "daily_goal_overrides"

// overridesSelectCols is the column projection used by GetOverride and List.
// Same shape as goalsSelect minus `id`: 30 nutrient columns + created_at + updated_at.
var overridesSelectCols = strings.Join(goalColumns, ", ") + ", created_at, updated_at"

// GetOverride returns the override for `date`, or ErrOverrideNotFound when
// no row exists.
func (r *OverridesRepo) GetOverride(ctx context.Context, date time.Time) (*Goals, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+overridesSelectCols+` FROM `+overridesTable+` WHERE date = $1`,
		date,
	)
	g, err := scanGoalRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOverrideNotFound
		}
		return nil, fmt.Errorf("scan override: %w", err)
	}
	return g, nil
}

// Upsert writes the override for `date`, replacing every nutrient bound with
// what's on g (nil pointers become NULL — full-replace semantics).
func (r *OverridesRepo) Upsert(ctx context.Context, date time.Time, g *Goals) error {
	// Build placeholder list $2..$31 for the 30 nutrient columns; $1 is date.
	placeholders := make([]string, len(goalColumns))
	setClauses := make([]string, len(goalColumns))
	for i, col := range goalColumns {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		setClauses[i] = fmt.Sprintf("%s = EXCLUDED.%s", col, col)
	}

	q := `INSERT INTO ` + overridesTable + ` (date, ` + strings.Join(goalColumns, ", ") + `, updated_at) ` +
		`VALUES ($1, ` + strings.Join(placeholders, ", ") + `, now()) ` +
		`ON CONFLICT (date) DO UPDATE SET ` + strings.Join(setClauses, ", ") + `, updated_at = now()`

	args := append([]any{date}, goalValueArgs(g)...)
	if _, err := r.q.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("upsert override: %w", err)
	}
	return nil
}

// UpsertPatch performs a merge-style upsert: it overlays ONLY the non-nil
// Range fields from `patch` onto the existing override for `date`, then
// writes the merged row. Fields that are nil on `patch` keep whatever value
// the existing row had. If no override exists for `date`, the patch itself
// becomes the new row (only the non-nil fields, every other field NULL).
//
// Returns `created == true` when no prior row existed (a new row was
// inserted), `created == false` when an existing row was merged into.
//
// Current consumer: the race-prep carb-load apply endpoint (it writes only
// `carbs_g`, preserving any prior `kcal`/`protein_g`/etc. on the same date).
// Not exposed as a public REST verb yet — if a second consumer appears,
// promote to `PATCH /goals/overrides/{date}` then. Validation of `patch`
// fields is the caller's responsibility (mirrors the existing handler-side
// `validateGoals` rules); this method is a repository primitive.
func (r *OverridesRepo) UpsertPatch(ctx context.Context, date time.Time, patch *Goals) (bool, error) {
	existing, err := r.GetOverride(ctx, date)
	created := false
	if err != nil {
		if !errors.Is(err, ErrOverrideNotFound) {
			return false, fmt.Errorf("upsert patch fetch: %w", err)
		}
		created = true
		existing = &Goals{}
	}
	merged := mergeGoalsPatch(existing, patch)
	if err := r.Upsert(ctx, date, merged); err != nil {
		return false, err
	}
	return created, nil
}

// mergeGoalsPatch returns a new Goals where each Range field is the patch's
// value if non-nil, otherwise the base's value. Timestamps from base are NOT
// carried over — Upsert sets `updated_at = now()` itself.
func mergeGoalsPatch(base, patch *Goals) *Goals {
	pick := func(p, b *Range) *Range {
		if p != nil {
			return p
		}
		return b
	}
	return &Goals{
		Kcal:          pick(patch.Kcal, base.Kcal),
		ProteinG:      pick(patch.ProteinG, base.ProteinG),
		CarbsG:        pick(patch.CarbsG, base.CarbsG),
		FatG:          pick(patch.FatG, base.FatG),
		FiberG:        pick(patch.FiberG, base.FiberG),
		SugarG:        pick(patch.SugarG, base.SugarG),
		SaltG:         pick(patch.SaltG, base.SaltG),
		IronMg:        pick(patch.IronMg, base.IronMg),
		CalciumMg:     pick(patch.CalciumMg, base.CalciumMg),
		VitaminDMcg:   pick(patch.VitaminDMcg, base.VitaminDMcg),
		VitaminB12Mcg: pick(patch.VitaminB12Mcg, base.VitaminB12Mcg),
		VitaminCMg:    pick(patch.VitaminCMg, base.VitaminCMg),
		MagnesiumMg:   pick(patch.MagnesiumMg, base.MagnesiumMg),
		PotassiumMg:   pick(patch.PotassiumMg, base.PotassiumMg),
		ZincMg:        pick(patch.ZincMg, base.ZincMg),
	}
}

// Delete removes the override for `date`. Returns ErrOverrideNotFound if no row matched.
func (r *OverridesRepo) Delete(ctx context.Context, date time.Time) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM `+overridesTable+` WHERE date = $1`, date)
	if err != nil {
		return fmt.Errorf("delete override: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOverrideNotFound
	}
	return nil
}

// List returns overrides in [from, to] inclusive, ordered by date ascending.
func (r *OverridesRepo) List(ctx context.Context, from, to time.Time) ([]*Override, error) {
	rows, err := r.q.Query(ctx,
		`SELECT date, `+overridesSelectCols+` FROM `+overridesTable+
			` WHERE date >= $1 AND date <= $2 ORDER BY date ASC`,
		from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list overrides: %w", err)
	}
	defer rows.Close()

	var out []*Override
	for rows.Next() {
		// Scan date first, then defer the rest to scanGoalRow via a row adapter.
		// Easiest: scan everything inline using the same approach as scanGoalRow
		// but with an additional date column up front.
		var date time.Time
		var (
			kcalMin, kcalMax                                           *float64
			proteinMin, proteinMax, carbsMin, carbsMax, fatMin, fatMax *float64
			fiberMin, fiberMax, sugarMin, sugarMax, saltMin, saltMax   *float64
			ironMin, ironMax                                           *float64
			calciumMin, calciumMax                                     *float64
			vitDMin, vitDMax, vitB12Min, vitB12Max                     *float64
			vitCMin, vitCMax, magnesiumMin, magnesiumMax               *float64
			potassiumMin, potassiumMax, zincMin, zincMax               *float64
			createdAt, updatedAt                                       time.Time
		)
		if err := rows.Scan(
			&date,
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
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan override row: %w", err)
		}
		g := &Goals{
			Kcal:          rangeOrNil(kcalMin, kcalMax),
			ProteinG:      rangeOrNil(proteinMin, proteinMax),
			CarbsG:        rangeOrNil(carbsMin, carbsMax),
			FatG:          rangeOrNil(fatMin, fatMax),
			FiberG:        rangeOrNil(fiberMin, fiberMax),
			SugarG:        rangeOrNil(sugarMin, sugarMax),
			SaltG:         rangeOrNil(saltMin, saltMax),
			IronMg:        rangeOrNil(ironMin, ironMax),
			CalciumMg:     rangeOrNil(calciumMin, calciumMax),
			VitaminDMcg:   rangeOrNil(vitDMin, vitDMax),
			VitaminB12Mcg: rangeOrNil(vitB12Min, vitB12Max),
			VitaminCMg:    rangeOrNil(vitCMin, vitCMax),
			MagnesiumMg:   rangeOrNil(magnesiumMin, magnesiumMax),
			PotassiumMg:   rangeOrNil(potassiumMin, potassiumMax),
			ZincMg:        rangeOrNil(zincMin, zincMax),
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		}
		out = append(out, &Override{Date: date, Goals: g})
	}
	return out, rows.Err()
}
