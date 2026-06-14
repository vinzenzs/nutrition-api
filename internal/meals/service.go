package meals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrProductIDRequired = errors.New("product_id_required")
	ErrProductNotFound   = errors.New("product_not_found")
	ErrQuantityInvalid   = errors.New("quantity_g_invalid")
	ErrLoggedAtFuture    = errors.New("logged_at_too_far_future")
	ErrMealTypeInvalid   = errors.New("meal_type_invalid")
	ErrNameRequired      = errors.New("name_required")
	ErrWorkoutNotFound   = errors.New("workout_not_found")
)

// ErrNutrimentInvalid carries which nutriment field was rejected.
type ErrNutrimentInvalid struct {
	Field string
}

func (e *ErrNutrimentInvalid) Error() string {
	return fmt.Sprintf("nutriments_invalid: %s", e.Field)
}

// Service orchestrates meal entry CRUD against the meals repo and the
// products repo (for FK validation + last_logged_at touches). The optional
// workouts repo enables the workout_id link introduced by
// add-meal-workout-link; it may be nil for callers that don't need link
// validation (e.g. older tests).
type Service struct {
	pool           *pgxpool.Pool
	mealsRepo      *Repo
	productsRepo   *products.Repo
	componentsRepo *products.ComponentsRepo
	workoutsRepo   *workouts.Repo
}

func NewService(pool *pgxpool.Pool, mealsRepo *Repo, productsRepo *products.Repo) *Service {
	return &Service{
		pool:           pool,
		mealsRepo:      mealsRepo,
		productsRepo:   productsRepo,
		componentsRepo: products.NewComponentsRepo(pool),
	}
}

// SetWorkoutsRepo wires the workouts repo for workout_id link validation.
// Optional — callers that don't set it skip the existence check (the FK
// constraint will still reject unknown ids at the DB layer with 5xx).
func (s *Service) SetWorkoutsRepo(r *workouts.Repo) { s.workoutsRepo = r }

// validateWorkoutID returns ErrWorkoutNotFound if the supplied id doesn't
// exist. If the service has no workouts repo wired, validation is skipped.
func (s *Service) validateWorkoutID(ctx context.Context, id uuid.UUID) error {
	if s.workoutsRepo == nil {
		return nil
	}
	if _, err := s.workoutsRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			return ErrWorkoutNotFound
		}
		return err
	}
	return nil
}

// CreateInput is the payload for POST /meals.
type CreateInput struct {
	ProductID *uuid.UUID
	QuantityG float64
	LoggedAt  time.Time
	MealType  *string
	Note      *string
	WorkoutID *uuid.UUID
}

// Create validates the input, inserts a meal entry, and advances the
// linked product's last_logged_at.
func (s *Service) Create(ctx context.Context, in CreateInput) (*MealEntry, error) {
	return s.CreateInTx(ctx, s.pool, in)
}

// CreateInTx runs the product-backed meal create against the supplied Querier
// instead of the pool. This lets another capability compose meal creation into
// its own transaction — used by mealplan's "eaten" transition so the meal
// entry and the plan-row status flip commit atomically. It does NOT open or
// commit the transaction; the caller owns that. Pass s.pool for the standalone
// path (what Create does).
func (s *Service) CreateInTx(ctx context.Context, q store.Querier, in CreateInput) (*MealEntry, error) {
	if in.ProductID == nil {
		return nil, ErrProductIDRequired
	}
	if err := validateQuantity(in.QuantityG); err != nil {
		return nil, err
	}
	if err := validateLoggedAt(in.LoggedAt); err != nil {
		return nil, err
	}
	mealType, err := normalizeMealType(in.MealType)
	if err != nil {
		return nil, err
	}

	mealsRepo := NewRepo(q)
	productsRepo := products.NewRepo(q)

	if _, err := productsRepo.GetByID(ctx, *in.ProductID); err != nil {
		if errors.Is(err, products.ErrNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	if in.WorkoutID != nil {
		if err := s.validateWorkoutID(ctx, *in.WorkoutID); err != nil {
			return nil, err
		}
	}

	id, err := mealsRepo.Insert(ctx, InsertParams{
		ProductID: in.ProductID,
		LoggedAt:  in.LoggedAt,
		QuantityG: in.QuantityG,
		MealType:  mealType,
		Note:      in.Note,
		WorkoutID: in.WorkoutID,
	})
	if err != nil {
		return nil, err
	}
	if err := productsRepo.TouchLastLoggedAt(ctx, *in.ProductID, in.LoggedAt, in.QuantityG); err != nil {
		return nil, err
	}
	return mealsRepo.GetByID(ctx, id)
}

// FreeformInput is the payload for POST /meals/freeform.
type FreeformInput struct {
	Name          string
	Nutriments    Nutriments
	QuantityG     float64
	LoggedAt      time.Time
	MealType      *string
	Note          *string
	SaveAsProduct bool
	WorkoutID     *uuid.UUID
}

// CreateFreeform persists a meal entry from client-supplied nutriments,
// optionally promoting them to a reusable product row.
func (s *Service) CreateFreeform(ctx context.Context, in FreeformInput) (*MealEntry, error) {
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if err := validateQuantity(in.QuantityG); err != nil {
		return nil, err
	}
	if err := validateLoggedAt(in.LoggedAt); err != nil {
		return nil, err
	}
	if err := validateNutriments(in.Nutriments); err != nil {
		return nil, err
	}
	mealType, err := normalizeMealType(in.MealType)
	if err != nil {
		return nil, err
	}
	if in.WorkoutID != nil {
		if err := s.validateWorkoutID(ctx, *in.WorkoutID); err != nil {
			return nil, err
		}
	}

	var productID *uuid.UUID
	if in.SaveAsProduct {
		p := &products.Product{
			Name:   in.Name,
			Source: products.SourceManual,
			Nutriments: products.Nutriments{
				KcalPer100g:     in.Nutriments.KcalPer100g,
				ProteinGPer100g: in.Nutriments.ProteinGPer100g,
				CarbsGPer100g:   in.Nutriments.CarbsGPer100g,
				FatGPer100g:     in.Nutriments.FatGPer100g,
				FiberGPer100g:   in.Nutriments.FiberGPer100g,
				SugarGPer100g:   in.Nutriments.SugarGPer100g,
				SaltGPer100g:    in.Nutriments.SaltGPer100g,

				IronMgPer100g:        in.Nutriments.IronMgPer100g,
				CalciumMgPer100g:     in.Nutriments.CalciumMgPer100g,
				VitaminDMcgPer100g:   in.Nutriments.VitaminDMcgPer100g,
				VitaminB12McgPer100g: in.Nutriments.VitaminB12McgPer100g,
				VitaminCMgPer100g:    in.Nutriments.VitaminCMgPer100g,
				MagnesiumMgPer100g:   in.Nutriments.MagnesiumMgPer100g,
				PotassiumMgPer100g:   in.Nutriments.PotassiumMgPer100g,
				ZincMgPer100g:        in.Nutriments.ZincMgPer100g,
			},
		}
		if err := s.productsRepo.Insert(ctx, p); err != nil {
			return nil, err
		}
		productID = &p.ID
	}

	name := in.Name
	id, err := s.mealsRepo.Insert(ctx, InsertParams{
		ProductID:          productID,
		LoggedAt:           in.LoggedAt,
		QuantityG:          in.QuantityG,
		MealType:           mealType,
		Note:               in.Note,
		WorkoutID:          in.WorkoutID,
		SnapshotName:       &name,
		SnapshotNutriments: in.Nutriments,
	})
	if err != nil {
		return nil, err
	}
	if productID != nil {
		if err := s.productsRepo.TouchLastLoggedAt(ctx, *productID, in.LoggedAt, in.QuantityG); err != nil {
			return nil, err
		}
	}
	return s.mealsRepo.GetByID(ctx, id)
}

// Get returns a meal entry by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*MealEntry, error) {
	return s.mealsRepo.GetByID(ctx, id)
}

// ScaledComponent is one ingredient of a recipe-backed meal, with grams scaled
// from the recipe's per-serving definition to the meal's actual quantity.
type ScaledComponent struct {
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
	QuantityG float64   `json:"quantity_g"`
}

// GetWithComponents returns the meal entry plus, if the meal's product is a
// recipe, a scaled component breakdown. For non-recipe and freeform meals the
// components slice is empty.
func (s *Service) GetWithComponents(ctx context.Context, id uuid.UUID) (*MealEntry, []ScaledComponent, error) {
	m, err := s.mealsRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if m.ProductID == nil {
		return m, []ScaledComponent{}, nil
	}
	p, err := s.productsRepo.GetByID(ctx, *m.ProductID)
	if err != nil {
		if errors.Is(err, products.ErrNotFound) {
			return m, []ScaledComponent{}, nil
		}
		return nil, nil, err
	}
	if p.Source != products.SourceRecipe {
		return m, []ScaledComponent{}, nil
	}
	comps, err := s.componentsRepo.ListComponents(ctx, p.ID)
	if err != nil {
		return nil, nil, err
	}

	// Scale by meal.QuantityG / (recipe.ServingSizeG OR 100). The /100 fallback
	// matches the recipe's "per-100g composition" interpretation.
	denom := 100.0
	if p.ServingSizeG != nil && *p.ServingSizeG > 0 {
		denom = *p.ServingSizeG
	}
	factor := m.QuantityG / denom

	scaled := make([]ScaledComponent, 0, len(comps))
	for _, c := range comps {
		scaled = append(scaled, ScaledComponent{
			ProductID: c.ComponentProductID,
			Name:      c.ComponentProduct.Name,
			QuantityG: c.QuantityG * factor,
		})
	}
	return m, scaled, nil
}

// PatchInput is the editable subset on PATCH /meals/{id}.
//
// WorkoutID tri-state: nil = no change; non-nil pointer = update with the
// pointed-to value. ClearWorkoutID = true clears the link (passed as a
// separate flag because Go's JSON decoder collapses missing-vs-null into the
// same nil pointer, so the handler converts the empty-string sentinel into
// this flag before calling Patch).
type PatchInput struct {
	QuantityG      *float64
	LoggedAt       *time.Time
	MealType       *string // raw value to validate; nil means leave unchanged
	Note           *string
	WorkoutID      *uuid.UUID
	ClearWorkoutID bool
}

// Patch validates and applies a partial update.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*MealEntry, error) {
	if in.QuantityG != nil {
		if err := validateQuantity(*in.QuantityG); err != nil {
			return nil, err
		}
	}
	if in.LoggedAt != nil {
		if err := validateLoggedAt(*in.LoggedAt); err != nil {
			return nil, err
		}
	}
	var mealType *MealType
	if in.MealType != nil {
		mt, err := normalizeMealType(in.MealType)
		if err != nil {
			return nil, err
		}
		mealType = mt
	}
	if in.WorkoutID != nil && !in.ClearWorkoutID {
		if err := s.validateWorkoutID(ctx, *in.WorkoutID); err != nil {
			return nil, err
		}
	}

	patchParams := PatchParams{
		QuantityG:      in.QuantityG,
		LoggedAt:       in.LoggedAt,
		MealType:       mealType,
		Note:           in.Note,
		WorkoutID:      in.WorkoutID,
		ClearWorkoutID: in.ClearWorkoutID,
	}
	if err := s.mealsRepo.Patch(ctx, id, patchParams); err != nil {
		return nil, err
	}
	updated, err := s.mealsRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Two PATCH cases that touch the product's last-logged columns:
	//
	//   1. logged_at moved forward: the conditional TouchLastLoggedAt advances
	//      both last_logged_at and last_logged_quantity_g together, using the
	//      meal's current (post-PATCH) quantity_g.
	//
	//   2. logged_at unchanged, quantity_g changed: only update
	//      last_logged_quantity_g, and only when this PATCHed meal IS the
	//      most-recent log for this product (its logged_at equals the
	//      product's last_logged_at). Older meals don't propagate.
	if updated.ProductID != nil {
		if in.LoggedAt != nil {
			if err := s.productsRepo.TouchLastLoggedAt(ctx, *updated.ProductID, *in.LoggedAt, updated.QuantityG); err != nil {
				return nil, err
			}
		} else if in.QuantityG != nil {
			if err := s.productsRepo.TouchLastLoggedQuantityIfCurrent(ctx, *updated.ProductID, updated.LoggedAt, *in.QuantityG); err != nil {
				return nil, err
			}
		}
	}
	return updated, nil
}

// Delete removes a meal entry.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.mealsRepo.Delete(ctx, id)
}

// List proxies to the repo.
func (s *Service) List(ctx context.Context, p ListParams) ([]*MealEntry, error) {
	return s.mealsRepo.List(ctx, p)
}

// ----- validators -----

func validateQuantity(q float64) error {
	if q <= 0 {
		return ErrQuantityInvalid
	}
	return nil
}

func validateLoggedAt(ts time.Time) error {
	if ts.After(time.Now().Add(24 * time.Hour)) {
		return ErrLoggedAtFuture
	}
	return nil
}

func normalizeMealType(s *string) (*MealType, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	mt, err := ParseMealType(*s)
	if err != nil {
		return nil, ErrMealTypeInvalid
	}
	return &mt, nil
}

func validateNutriments(n Nutriments) error {
	check := func(field string, v *float64) error {
		if v == nil {
			return nil
		}
		if *v < 0 {
			return &ErrNutrimentInvalid{Field: field}
		}
		return nil
	}
	if err := check("kcal", n.KcalPer100g); err != nil {
		return err
	}
	if err := check("protein_g", n.ProteinGPer100g); err != nil {
		return err
	}
	if err := check("carbs_g", n.CarbsGPer100g); err != nil {
		return err
	}
	if err := check("fat_g", n.FatGPer100g); err != nil {
		return err
	}
	if err := check("fiber_g", n.FiberGPer100g); err != nil {
		return err
	}
	if err := check("sugar_g", n.SugarGPer100g); err != nil {
		return err
	}
	if err := check("salt_g", n.SaltGPer100g); err != nil {
		return err
	}
	if err := check("iron_mg", n.IronMgPer100g); err != nil {
		return err
	}
	if err := check("calcium_mg", n.CalciumMgPer100g); err != nil {
		return err
	}
	if err := check("vitamin_d_mcg", n.VitaminDMcgPer100g); err != nil {
		return err
	}
	if err := check("vitamin_b12_mcg", n.VitaminB12McgPer100g); err != nil {
		return err
	}
	if err := check("vitamin_c_mg", n.VitaminCMgPer100g); err != nil {
		return err
	}
	if err := check("magnesium_mg", n.MagnesiumMgPer100g); err != nil {
		return err
	}
	if err := check("potassium_mg", n.PotassiumMgPer100g); err != nil {
		return err
	}
	if err := check("zinc_mg", n.ZincMgPer100g); err != nil {
		return err
	}
	return nil
}
