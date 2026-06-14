package mealplan

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store"
)

// Validation / transition errors map 1:1 to API error codes.
var (
	ErrSlotInvalid       = errors.New("slot_invalid")
	ErrStatusInvalid     = errors.New("status_invalid")
	ErrPlanDateInvalid   = errors.New("plan_date_invalid")
	ErrProductIDRequired = errors.New("product_id_required")
	ErrProductNotFound   = errors.New("product_not_found")
	ErrQuantityInvalid   = errors.New("quantity_g_invalid")
	ErrLoggedAtFuture    = errors.New("logged_at_too_far_future")
	ErrLoggedAtInvalid   = errors.New("logged_at_invalid")
	ErrAlreadyEaten      = errors.New("plan_entry_already_eaten")
	ErrNotPlanned        = errors.New("plan_entry_not_planned")
	ErrEatenTerminal     = errors.New("plan_entry_eaten_terminal")
	ErrEatenViaEndpoint  = errors.New("plan_entry_eaten_via_endpoint_only")
)

const defaultQuantityG = 100.0

// Service orchestrates planned-meal CRUD and the eaten transition. It holds the
// pool so the eaten transition (meal create + status flip) commits atomically,
// the products repo for FK validation, and the meals service to log the entry.
type Service struct {
	pool         *pgxpool.Pool
	repo         *Repo
	productsRepo *products.Repo
	mealsSvc     *meals.Service
}

func NewService(pool *pgxpool.Pool, repo *Repo) *Service {
	return &Service{pool: pool, repo: repo}
}

// SetProductsRepo wires product FK validation (mirrors mealsSvc.SetWorkoutsRepo).
func (s *Service) SetProductsRepo(r *products.Repo) { s.productsRepo = r }

// SetMealsService wires the meals service used by the eaten transition.
func (s *Service) SetMealsService(m *meals.Service) { s.mealsSvc = m }

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

// CreateInput is the payload for POST /plan.
type CreateInput struct {
	PlanDate  string
	Slot      string
	ProductID uuid.UUID
	QuantityG *float64
	Notes     *string
}

// Create validates and persists a planned meal (status defaults to planned).
func (s *Service) Create(ctx context.Context, in CreateInput) (*PlannedMeal, error) {
	if !validSlot(in.Slot) {
		return nil, ErrSlotInvalid
	}
	date, err := parseDate(in.PlanDate)
	if err != nil {
		return nil, err
	}
	if in.ProductID == uuid.Nil {
		return nil, ErrProductIDRequired
	}
	if err := validateQuantity(in.QuantityG); err != nil {
		return nil, err
	}
	if err := s.productExists(ctx, in.ProductID); err != nil {
		return nil, err
	}
	pm := &PlannedMeal{
		Slot:      in.Slot,
		ProductID: in.ProductID,
		QuantityG: in.QuantityG,
		Status:    StatusPlanned,
		Notes:     in.Notes,
	}
	if err := s.repo.Insert(ctx, pm, date); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, pm.ID)
}

// Get returns a planned meal by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*PlannedMeal, error) {
	return s.repo.GetByID(ctx, id)
}

// ListRange returns planned meals in [from, to] inclusive.
func (s *Service) ListRange(ctx context.Context, from, to string) ([]*PlannedMeal, error) {
	f, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	return s.repo.ListRange(ctx, f, t)
}

// UpdateInput is the editable subset on PATCH /plan/{id}.
type UpdateInput struct {
	PlanDate       *string
	Slot           *string
	ProductID      *uuid.UUID
	QuantityG      *float64
	ClearQuantityG bool
	Status         *string
	Notes          *string
	ClearNotes     bool
}

// Update validates and applies a partial update, guarding status transitions.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*PlannedMeal, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Slot != nil && !validSlot(*in.Slot) {
		return nil, ErrSlotInvalid
	}
	if in.QuantityG != nil {
		if err := validateQuantity(in.QuantityG); err != nil {
			return nil, err
		}
	}
	if in.ProductID != nil {
		if err := s.productExists(ctx, *in.ProductID); err != nil {
			return nil, err
		}
	}
	var datePtr *time.Time
	if in.PlanDate != nil {
		d, err := parseDate(*in.PlanDate)
		if err != nil {
			return nil, err
		}
		datePtr = &d
	}
	if in.Status != nil {
		if err := guardStatusTransition(current.Status, *in.Status); err != nil {
			return nil, err
		}
	}

	if err := s.repo.Update(ctx, id, UpdateParams{
		PlanDate:       datePtr,
		Slot:           in.Slot,
		ProductID:      in.ProductID,
		QuantityG:      in.QuantityG,
		ClearQuantityG: in.ClearQuantityG,
		Status:         in.Status,
		Notes:          in.Notes,
		ClearNotes:     in.ClearNotes,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a planned meal.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// EatenInput carries optional corrections for the eaten transition.
type EatenInput struct {
	QuantityG *float64
	LoggedAt  *time.Time
}

// EatenResult is returned by MarkEaten: both the updated plan and the new meal.
type EatenResult struct {
	Plan *PlannedMeal
	Meal *meals.MealEntry
}

// MarkEaten atomically logs a real meal entry for a planned meal and flips its
// status to eaten, recording the new meal entry id. If anything fails the whole
// transaction rolls back and the plan stays planned.
func (s *Service) MarkEaten(ctx context.Context, id uuid.UUID, in EatenInput) (*EatenResult, error) {
	plan, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	switch plan.Status {
	case StatusEaten:
		return nil, ErrAlreadyEaten
	case StatusSkipped:
		return nil, ErrNotPlanned
	}

	loggedAt := time.Now().UTC()
	if in.LoggedAt != nil {
		loggedAt = *in.LoggedAt
	}
	if loggedAt.After(time.Now().Add(24 * time.Hour)) {
		return nil, ErrLoggedAtFuture
	}

	quantity, err := s.effectiveQuantity(ctx, plan, in.QuantityG)
	if err != nil {
		return nil, err
	}

	var result *EatenResult
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		slot := plan.Slot
		meal, err := s.mealsSvc.CreateInTx(ctx, tx, meals.CreateInput{
			ProductID: &plan.ProductID,
			QuantityG: quantity,
			LoggedAt:  loggedAt,
			MealType:  &slot,
		})
		if err != nil {
			return mapMealsErr(err)
		}
		txRepo := NewRepo(tx)
		n, err := txRepo.SetEaten(ctx, id, meal.ID)
		if err != nil {
			return err
		}
		if n == 0 {
			// Lost the race to a concurrent eaten — roll back.
			return ErrAlreadyEaten
		}
		updated, err := txRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		result = &EatenResult{Plan: updated, Meal: meal}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// effectiveQuantity resolves body override → plan quantity_g → product serving
// default → 100 g.
func (s *Service) effectiveQuantity(ctx context.Context, plan *PlannedMeal, override *float64) (float64, error) {
	if override != nil {
		if err := validateQuantity(override); err != nil {
			return 0, err
		}
		return *override, nil
	}
	if plan.QuantityG != nil {
		return *plan.QuantityG, nil
	}
	if s.productsRepo != nil {
		if p, err := s.productsRepo.GetByID(ctx, plan.ProductID); err == nil && p.ServingSizeG != nil && *p.ServingSizeG > 0 {
			return *p.ServingSizeG, nil
		}
	}
	return defaultQuantityG, nil
}

// ----- helpers -----

func parseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(dateLayout, strings.TrimSpace(s), time.UTC)
	if err != nil {
		return time.Time{}, ErrPlanDateInvalid
	}
	return t, nil
}

func validateQuantity(q *float64) error {
	if q == nil {
		return nil
	}
	if math.IsNaN(*q) || math.IsInf(*q, 0) || *q <= 0 {
		return ErrQuantityInvalid
	}
	return nil
}

// guardStatusTransition enforces the one-way lifecycle for PATCH:
//   - eaten is terminal — any change away from it is rejected.
//   - status can only be set to eaten via POST /plan/{id}/eaten.
//   - planned ↔ skipped (incl. no-ops) are allowed.
func guardStatusTransition(current, requested string) error {
	if !validStatus(requested) {
		return ErrStatusInvalid
	}
	if current == StatusEaten {
		return ErrEatenTerminal
	}
	if requested == StatusEaten {
		return ErrEatenViaEndpoint
	}
	return nil
}

func mapMealsErr(err error) error {
	switch {
	case errors.Is(err, meals.ErrProductNotFound):
		return ErrProductNotFound
	case errors.Is(err, meals.ErrQuantityInvalid):
		return ErrQuantityInvalid
	case errors.Is(err, meals.ErrLoggedAtFuture):
		return ErrLoggedAtFuture
	default:
		return err
	}
}
