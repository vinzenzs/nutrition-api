package shoppinglist

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

// ErrNotFound is returned when a shopping item does not exist.
var ErrNotFound = errors.New("shopping item not found")

// Repo persists shopping items against a store.Querier (pool or tx).
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, name, quantity_text, recipe_product_id, plan_date, checked, checked_at, created_at`

// InsertRow is one prepared item for a bulk insert (plan_date already parsed).
type InsertRow struct {
	Name            string
	QuantityText    *string
	RecipeProductID *uuid.UUID
	PlanDate        *time.Time
}

// BulkInsert inserts all rows in one statement, atomically, preserving input
// order, and returns the created items in that order.
func (r *Repo) BulkInsert(ctx context.Context, rows []InsertRow) ([]*Item, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`INSERT INTO shopping_items (id, name, quantity_text, recipe_product_id, plan_date, checked, checked_at, created_at) VALUES `)
	out := make([]*Item, 0, len(rows))
	for i, row := range rows {
		id := uuid.New()
		base := i * 6
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d, false, NULL, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, id, row.Name, row.QuantityText, row.RecipeProductID, row.PlanDate, now)
		item := &Item{
			ID:              id,
			Name:            row.Name,
			QuantityText:    row.QuantityText,
			RecipeProductID: row.RecipeProductID,
			Checked:         false,
			CreatedAt:       now,
		}
		if row.PlanDate != nil {
			s := row.PlanDate.Format(dateLayout)
			item.PlanDate = &s
		}
		out = append(out, item)
	}
	if _, err := r.q.Exec(ctx, sb.String(), args...); err != nil {
		return nil, fmt.Errorf("bulk insert shopping items: %w", err)
	}
	return out, nil
}

// List returns shopping items. When includeChecked is false only unchecked
// items are returned (created order); otherwise all items with checked ones
// last.
func (r *Repo) List(ctx context.Context, includeChecked bool) ([]*Item, error) {
	q := `SELECT ` + selectCols + ` FROM shopping_items `
	if includeChecked {
		q += `ORDER BY checked ASC, created_at ASC, seq ASC`
	} else {
		q += `WHERE checked = false ORDER BY created_at ASC, seq ASC`
	}
	rows, err := r.q.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list shopping items: %w", err)
	}
	defer rows.Close()
	out := []*Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetByID returns one item. ErrNotFound if absent.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Item, error) {
	return scanItem(r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM shopping_items WHERE id = $1`, id))
}

// UpdateParams holds the editable fields. Checked, when non-nil, also sets or
// clears checked_at.
type UpdateParams struct {
	Name              *string
	QuantityText      *string
	ClearQuantityText bool
	Checked           *bool
}

// Update applies a partial update. ErrNotFound if no row matches.
func (r *Repo) Update(ctx context.Context, id uuid.UUID, p UpdateParams) error {
	sets := []string{}
	args := []any{id}
	next := 2
	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}
	if p.Name != nil {
		add("name", *p.Name)
	}
	switch {
	case p.ClearQuantityText:
		sets = append(sets, "quantity_text = NULL")
	case p.QuantityText != nil:
		add("quantity_text", *p.QuantityText)
	}
	if p.Checked != nil {
		add("checked", *p.Checked)
		if *p.Checked {
			sets = append(sets, "checked_at = now()")
		} else {
			sets = append(sets, "checked_at = NULL")
		}
	}
	if len(sets) == 0 {
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE shopping_items SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update shopping item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes one item. ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM shopping_items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete shopping item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteChecked removes all checked items and returns how many were deleted.
func (r *Repo) DeleteChecked(ctx context.Context) (int64, error) {
	tag, err := r.q.Exec(ctx, `DELETE FROM shopping_items WHERE checked = true`)
	if err != nil {
		return 0, fmt.Errorf("clear checked shopping items: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanItem(s interface{ Scan(...any) error }) (*Item, error) {
	var it Item
	var planDate *time.Time
	err := s.Scan(&it.ID, &it.Name, &it.QuantityText, &it.RecipeProductID, &planDate,
		&it.Checked, &it.CheckedAt, &it.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan shopping item: %w", err)
	}
	if planDate != nil {
		s := planDate.Format(dateLayout)
		it.PlanDate = &s
	}
	return &it, nil
}
