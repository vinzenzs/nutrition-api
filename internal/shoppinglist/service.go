package shoppinglist

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/products"
)

// Batch-level errors map 1:1 to API error codes.
var (
	ErrBatchEmpty      = errors.New("items_required")
	ErrBatchTooLarge   = errors.New("batch_too_large")
	ErrProductNotFound = errors.New("product_not_found")
)

// ItemError flags which item in a batch failed and why, so the API can return
// the offending index.
type ItemError struct {
	Index int
	Code  string
}

func (e *ItemError) Error() string { return fmt.Sprintf("item[%d]: %s", e.Index, e.Code) }

// Service orchestrates shopping-list CRUD. The products repo is injected only
// for the soft-provenance existence check.
type Service struct {
	repo         *Repo
	productsRepo *products.Repo
}

func NewService(repo *Repo) *Service { return &Service{repo: repo} }

// SetProductsRepo wires the products repo for recipe_product_id existence
// checks. Optional — when nil the check is skipped (the FK still enforces it).
func (s *Service) SetProductsRepo(r *products.Repo) { s.productsRepo = r }

// CreateItemInput is one item in a bulk create.
type CreateItemInput struct {
	Name            string
	QuantityText    *string
	RecipeProductID *uuid.UUID
	PlanDate        *string
}

// BulkCreate validates the whole batch then inserts it atomically. A single
// invalid item fails the batch with an ItemError carrying its index.
func (s *Service) BulkCreate(ctx context.Context, items []CreateItemInput) ([]*Item, error) {
	if len(items) == 0 {
		return nil, ErrBatchEmpty
	}
	if len(items) > maxBatchSize {
		return nil, ErrBatchTooLarge
	}
	rows := make([]InsertRow, 0, len(items))
	for i, in := range items {
		name := strings.TrimSpace(in.Name)
		if name == "" || len(name) > maxNameLen {
			return nil, &ItemError{Index: i, Code: "name_invalid"}
		}
		var planDate *time.Time
		if in.PlanDate != nil && *in.PlanDate != "" {
			d, err := time.ParseInLocation(dateLayout, strings.TrimSpace(*in.PlanDate), time.UTC)
			if err != nil {
				return nil, &ItemError{Index: i, Code: "plan_date_invalid"}
			}
			planDate = &d
		}
		if in.RecipeProductID != nil {
			if err := s.productExists(ctx, *in.RecipeProductID); err != nil {
				if errors.Is(err, ErrProductNotFound) {
					return nil, &ItemError{Index: i, Code: "product_not_found"}
				}
				return nil, err
			}
		}
		rows = append(rows, InsertRow{
			Name:            name,
			QuantityText:    in.QuantityText,
			RecipeProductID: in.RecipeProductID,
			PlanDate:        planDate,
		})
	}
	return s.repo.BulkInsert(ctx, rows)
}

func (s *Service) productExists(ctx context.Context, id uuid.UUID) error {
	if s.productsRepo == nil {
		return nil
	}
	if _, err := s.productsRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, products.ErrNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	return nil
}

// List returns the checklist.
func (s *Service) List(ctx context.Context, includeChecked bool) ([]*Item, error) {
	return s.repo.List(ctx, includeChecked)
}

// UpdateInput is the editable subset on PATCH.
type UpdateInput struct {
	Name              *string
	QuantityText      *string
	ClearQuantityText bool
	Checked           *bool
}

// ErrNameInvalid is returned when a PATCH sets an empty/too-long name.
var ErrNameInvalid = errors.New("name_invalid")

// Update validates and applies a partial update (stamping checked_at).
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Item, error) {
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxNameLen {
			return nil, ErrNameInvalid
		}
		in.Name = &name
	}
	if err := s.repo.Update(ctx, id, UpdateParams{
		Name:              in.Name,
		QuantityText:      in.QuantityText,
		ClearQuantityText: in.ClearQuantityText,
		Checked:           in.Checked,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes one item.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ClearChecked deletes all checked items, returning the count.
func (s *Service) ClearChecked(ctx context.Context) (int64, error) {
	return s.repo.DeleteChecked(ctx)
}
