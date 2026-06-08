package meals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ErrNotFound is returned when a meal entry does not exist.
var ErrNotFound = errors.New("meal entry not found")

// Repo persists meal entries.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectEffective is the SELECT projection that builds an effective view of a
// meal entry by coalescing snapshot columns with the linked product's columns.
const selectEffective = `
    me.id,
    me.product_id,
    me.logged_at,
    me.quantity_g,
    me.meal_type,
    me.note,
    coalesce(me.snapshot_name, p.name)                              AS effective_name,
    coalesce(me.snapshot_kcal_per_100g,        p.kcal_per_100g)        AS eff_kcal,
    coalesce(me.snapshot_protein_g_per_100g,   p.protein_g_per_100g)   AS eff_protein,
    coalesce(me.snapshot_carbs_g_per_100g,     p.carbs_g_per_100g)     AS eff_carbs,
    coalesce(me.snapshot_fat_g_per_100g,       p.fat_g_per_100g)       AS eff_fat,
    coalesce(me.snapshot_fiber_g_per_100g,     p.fiber_g_per_100g)     AS eff_fiber,
    coalesce(me.snapshot_sugar_g_per_100g,     p.sugar_g_per_100g)     AS eff_sugar,
    coalesce(me.snapshot_salt_g_per_100g,      p.salt_g_per_100g)      AS eff_salt,
    coalesce(me.snapshot_iron_mg_per_100g,         p.iron_mg_per_100g)         AS eff_iron,
    coalesce(me.snapshot_calcium_mg_per_100g,      p.calcium_mg_per_100g)      AS eff_calcium,
    coalesce(me.snapshot_vitamin_d_mcg_per_100g,   p.vitamin_d_mcg_per_100g)   AS eff_vit_d,
    coalesce(me.snapshot_vitamin_b12_mcg_per_100g, p.vitamin_b12_mcg_per_100g) AS eff_vit_b12,
    coalesce(me.snapshot_vitamin_c_mg_per_100g,    p.vitamin_c_mg_per_100g)    AS eff_vit_c,
    coalesce(me.snapshot_magnesium_mg_per_100g,    p.magnesium_mg_per_100g)    AS eff_magnesium,
    coalesce(me.snapshot_potassium_mg_per_100g,    p.potassium_mg_per_100g)    AS eff_potassium,
    coalesce(me.snapshot_zinc_mg_per_100g,         p.zinc_mg_per_100g)         AS eff_zinc,
    me.workout_id,
    me.created_at,
    me.updated_at
`

const fromJoin = `FROM meal_entries me LEFT JOIN products p ON me.product_id = p.id`

// InsertParams captures what Insert / InsertFreeform need.
type InsertParams struct {
	ProductID *uuid.UUID
	LoggedAt  time.Time
	QuantityG float64
	MealType  *MealType
	Note      *string
	WorkoutID *uuid.UUID

	SnapshotName       *string
	SnapshotNutriments Nutriments // ignored fields stay nil
}

// Insert creates a meal_entries row and returns its id.
func (r *Repo) Insert(ctx context.Context, p InsertParams) (uuid.UUID, error) {
	id := uuid.New()
	now := time.Now().UTC()
	const q = `
        INSERT INTO meal_entries (
            id, product_id, logged_at, quantity_g, meal_type, note,
            snapshot_name,
            snapshot_kcal_per_100g, snapshot_protein_g_per_100g, snapshot_carbs_g_per_100g,
            snapshot_fat_g_per_100g, snapshot_fiber_g_per_100g, snapshot_sugar_g_per_100g, snapshot_salt_g_per_100g,
            snapshot_iron_mg_per_100g, snapshot_calcium_mg_per_100g, snapshot_vitamin_d_mcg_per_100g,
            snapshot_vitamin_b12_mcg_per_100g, snapshot_vitamin_c_mg_per_100g, snapshot_magnesium_mg_per_100g,
            snapshot_potassium_mg_per_100g, snapshot_zinc_mg_per_100g,
            workout_id,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6,
            $7,
            $8, $9, $10, $11, $12, $13, $14,
            $15, $16, $17, $18, $19, $20, $21, $22,
            $23,
            $24, $24
        )
    `
	_, err := r.q.Exec(ctx, q,
		id, p.ProductID, p.LoggedAt, p.QuantityG, p.MealType, p.Note,
		p.SnapshotName,
		p.SnapshotNutriments.KcalPer100g, p.SnapshotNutriments.ProteinGPer100g, p.SnapshotNutriments.CarbsGPer100g,
		p.SnapshotNutriments.FatGPer100g, p.SnapshotNutriments.FiberGPer100g, p.SnapshotNutriments.SugarGPer100g, p.SnapshotNutriments.SaltGPer100g,
		p.SnapshotNutriments.IronMgPer100g, p.SnapshotNutriments.CalciumMgPer100g, p.SnapshotNutriments.VitaminDMcgPer100g,
		p.SnapshotNutriments.VitaminB12McgPer100g, p.SnapshotNutriments.VitaminCMgPer100g, p.SnapshotNutriments.MagnesiumMgPer100g,
		p.SnapshotNutriments.PotassiumMgPer100g, p.SnapshotNutriments.ZincMgPer100g,
		p.WorkoutID,
		now,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert meal entry: %w", err)
	}
	return id, nil
}

// GetByID returns a single meal entry with effective nutriments resolved.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*MealEntry, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectEffective+` `+fromJoin+` WHERE me.id = $1`, id)
	return scanEffective(row)
}

// PatchParams holds the optional fields editable via PATCH /meals/{id}.
// nil pointers mean "do not update".
//
// workout_id uses a tri-state: nil = no change; non-nil pointer = update with
// the dereferenced value (which may be uuid.Nil to clear the link).
type PatchParams struct {
	QuantityG *float64
	LoggedAt  *time.Time
	MealType  *MealType
	Note      *string

	// WorkoutID set to a non-nil pointer triggers an update. The pointed-to
	// value is the new value (uuid.Nil clears the link). Set ClearWorkoutID
	// instead of a uuid.Nil pointer for clarity at call sites.
	WorkoutID      *uuid.UUID
	ClearWorkoutID bool
}

// HasUpdates reports whether at least one field is set for update.
func (p PatchParams) HasUpdates() bool {
	return p.QuantityG != nil || p.LoggedAt != nil || p.MealType != nil || p.Note != nil ||
		p.WorkoutID != nil || p.ClearWorkoutID
}

// Patch applies a partial update. Returns ErrNotFound if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.QuantityG != nil {
		sets = append(sets, fmt.Sprintf("quantity_g = $%d", next))
		args = append(args, *p.QuantityG)
		next++
	}
	if p.LoggedAt != nil {
		sets = append(sets, fmt.Sprintf("logged_at = $%d", next))
		args = append(args, *p.LoggedAt)
		next++
	}
	if p.MealType != nil {
		sets = append(sets, fmt.Sprintf("meal_type = $%d", next))
		args = append(args, string(*p.MealType))
		next++
	}
	if p.Note != nil {
		sets = append(sets, fmt.Sprintf("note = $%d", next))
		args = append(args, *p.Note)
		next++
	}
	if p.ClearWorkoutID {
		sets = append(sets, "workout_id = NULL")
	} else if p.WorkoutID != nil {
		sets = append(sets, fmt.Sprintf("workout_id = $%d", next))
		args = append(args, *p.WorkoutID)
		next++
	}
	if len(sets) == 1 {
		// No fields to update — just confirm the row exists.
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE meal_entries SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch meal entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a meal entry. Returns ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM meal_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete meal entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListParams scopes a List query.
type ListParams struct {
	From     time.Time
	To       time.Time
	MealType *MealType
}

// List returns meal entries with logged_at in [From, To), optionally filtered
// by meal_type, ordered by logged_at ASC.
func (r *Repo) List(ctx context.Context, p ListParams) ([]*MealEntry, error) {
	q := `SELECT ` + selectEffective + ` ` + fromJoin + ` WHERE me.logged_at >= $1 AND me.logged_at < $2`
	args := []any{p.From, p.To}
	if p.MealType != nil {
		q += ` AND me.meal_type = $3`
		args = append(args, string(*p.MealType))
	}
	q += ` ORDER BY me.logged_at ASC`

	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list meals: %w", err)
	}
	defer rows.Close()
	var out []*MealEntry
	for rows.Next() {
		m, err := scanEffective(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEffective(s scanner) (*MealEntry, error) {
	var (
		m  MealEntry
		mt *string
	)
	err := s.Scan(
		&m.ID,
		&m.ProductID,
		&m.LoggedAt,
		&m.QuantityG,
		&mt,
		&m.Note,
		&m.EffectiveName,
		&m.EffectiveNutrimentsPer100g.KcalPer100g,
		&m.EffectiveNutrimentsPer100g.ProteinGPer100g,
		&m.EffectiveNutrimentsPer100g.CarbsGPer100g,
		&m.EffectiveNutrimentsPer100g.FatGPer100g,
		&m.EffectiveNutrimentsPer100g.FiberGPer100g,
		&m.EffectiveNutrimentsPer100g.SugarGPer100g,
		&m.EffectiveNutrimentsPer100g.SaltGPer100g,
		&m.EffectiveNutrimentsPer100g.IronMgPer100g,
		&m.EffectiveNutrimentsPer100g.CalciumMgPer100g,
		&m.EffectiveNutrimentsPer100g.VitaminDMcgPer100g,
		&m.EffectiveNutrimentsPer100g.VitaminB12McgPer100g,
		&m.EffectiveNutrimentsPer100g.VitaminCMgPer100g,
		&m.EffectiveNutrimentsPer100g.MagnesiumMgPer100g,
		&m.EffectiveNutrimentsPer100g.PotassiumMgPer100g,
		&m.EffectiveNutrimentsPer100g.ZincMgPer100g,
		&m.WorkoutID,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan meal entry: %w", err)
	}
	if mt != nil {
		v := MealType(*mt)
		m.MealType = &v
	}
	return &m, nil
}
