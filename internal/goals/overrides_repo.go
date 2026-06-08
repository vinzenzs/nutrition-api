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
