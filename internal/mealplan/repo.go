package mealplan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a planned meal does not exist.
var ErrNotFound = errors.New("planned meal not found")

// Repo persists planned meals against a store.Querier (pool or tx).
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// selectCols + a product-name join; aliased so scanRow is shared.
const selectSQL = `
	SELECT pm.id, pm.plan_date, pm.slot, pm.product_id, p.name,
	       pm.quantity_g, pm.status, pm.meal_entry_id, pm.notes,
	       pm.created_at, pm.updated_at
	FROM planned_meals pm
	JOIN products p ON p.id = pm.product_id`

// slotRankSQL orders slots breakfast→snack within a date.
const slotRankSQL = `CASE pm.slot WHEN 'breakfast' THEN 0 WHEN 'lunch' THEN 1 WHEN 'dinner' THEN 2 ELSE 3 END`

// Insert creates a planned_meals row. The supplied PlannedMeal's
// ID/CreatedAt/UpdatedAt are set on success.
func (r *Repo) Insert(ctx context.Context, pm *PlannedMeal, planDate time.Time) error {
	if pm.ID == uuid.Nil {
		pm.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
		INSERT INTO planned_meals
			(id, plan_date, slot, product_id, quantity_g, status, meal_entry_id, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`
	if _, err := r.q.Exec(ctx, q,
		pm.ID, planDate, pm.Slot, pm.ProductID, pm.QuantityG, pm.Status, pm.MealEntryID, pm.Notes, now,
	); err != nil {
		return fmt.Errorf("insert planned meal: %w", err)
	}
	pm.PlanDate = planDate.Format(dateLayout)
	pm.CreatedAt = now
	pm.UpdatedAt = now
	return nil
}

// GetByID returns a planned meal with its product name. ErrNotFound if absent.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*PlannedMeal, error) {
	row := r.q.QueryRow(ctx, selectSQL+` WHERE pm.id = $1`, id)
	return scanRow(row)
}

// ListRange returns planned meals with plan_date in [from, to] inclusive,
// ordered by plan_date then slot then created_at.
func (r *Repo) ListRange(ctx context.Context, from, to time.Time) ([]*PlannedMeal, error) {
	rows, err := r.q.Query(ctx,
		selectSQL+` WHERE pm.plan_date >= $1 AND pm.plan_date <= $2 ORDER BY pm.plan_date ASC, `+slotRankSQL+`, pm.created_at ASC`,
		from, to)
	if err != nil {
		return nil, fmt.Errorf("list planned meals: %w", err)
	}
	defer rows.Close()
	out := []*PlannedMeal{}
	for rows.Next() {
		pm, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, rows.Err()
}

// UpdateParams holds the editable fields. A nil pointer leaves the column
// unchanged; ClearQuantityG / ClearNotes set the column NULL.
type UpdateParams struct {
	PlanDate       *time.Time
	Slot           *string
	ProductID      *uuid.UUID
	QuantityG      *float64
	ClearQuantityG bool
	Status         *string
	Notes          *string
	ClearNotes     bool
}

// Update applies a partial update. ErrNotFound if no row matches.
func (r *Repo) Update(ctx context.Context, id uuid.UUID, p UpdateParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}
	if p.PlanDate != nil {
		add("plan_date", *p.PlanDate)
	}
	if p.Slot != nil {
		add("slot", *p.Slot)
	}
	if p.ProductID != nil {
		add("product_id", *p.ProductID)
	}
	switch {
	case p.ClearQuantityG:
		sets = append(sets, "quantity_g = NULL")
	case p.QuantityG != nil:
		add("quantity_g", *p.QuantityG)
	}
	if p.Status != nil {
		add("status", *p.Status)
	}
	switch {
	case p.ClearNotes:
		sets = append(sets, "notes = NULL")
	case p.Notes != nil:
		add("notes", *p.Notes)
	}
	q := "UPDATE planned_meals SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update planned meal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetEaten flips a planned row to eaten and records the meal entry id, but only
// if it is still 'planned' (guards a race with a concurrent second tap).
// Returns the number of rows updated.
func (r *Repo) SetEaten(ctx context.Context, id, mealEntryID uuid.UUID) (int64, error) {
	tag, err := r.q.Exec(ctx,
		`UPDATE planned_meals SET status = 'eaten', meal_entry_id = $2, updated_at = now()
		 WHERE id = $1 AND status = 'planned'`,
		id, mealEntryID)
	if err != nil {
		return 0, fmt.Errorf("mark planned meal eaten: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Delete removes a planned meal. ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM planned_meals WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete planned meal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRow(s interface{ Scan(...any) error }) (*PlannedMeal, error) {
	var pm PlannedMeal
	var planDate time.Time
	err := s.Scan(&pm.ID, &planDate, &pm.Slot, &pm.ProductID, &pm.ProductName,
		&pm.QuantityG, &pm.Status, &pm.MealEntryID, &pm.Notes, &pm.CreatedAt, &pm.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan planned meal: %w", err)
	}
	pm.PlanDate = planDate.Format(dateLayout)
	return &pm, nil
}
