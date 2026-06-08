package products

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/nutrition-api/internal/off"
	"github.com/vinzenzs/nutrition-api/internal/store"
)

// OFFClient is the subset of off.Client behaviour the service depends on,
// so tests can swap in a stub.
type OFFClient interface {
	Fetch(ctx context.Context, barcode string) (*off.Product, error)
}

// Service orchestrates product lookups, manual creation, and recipe management.
type Service struct {
	pool *pgxpool.Pool
	repo *Repo
	off  OFFClient
}

// NewService builds a service. pool is required for operations that span
// multiple tables (recipe create / recompute); the simpler operations work
// directly against repo and don't need it.
func NewService(pool *pgxpool.Pool, repo *Repo, offClient OFFClient) *Service {
	return &Service{pool: pool, repo: repo, off: offClient}
}

// Lookup returns the product for barcode, fetching from OFF on the first hit
// and serving from cache thereafter. When refresh is true, OFF is re-queried
// and the cached row is updated.
func (s *Service) Lookup(ctx context.Context, barcode string, refresh bool) (*Product, error) {
	cached, err := s.repo.GetByBarcode(ctx, barcode)
	switch {
	case err == nil && !refresh:
		return cached, nil
	case err != nil && !errors.Is(err, ErrNotFound):
		return nil, err
	}

	fetched, ferr := s.off.Fetch(ctx, barcode)
	if ferr != nil {
		return nil, ferr
	}
	now := time.Now().UTC()

	if cached != nil {
		// Refresh existing row in place.
		cached.Name = fetched.Name
		if fetched.Brand != "" {
			b := fetched.Brand
			cached.Brand = &b
		} else {
			cached.Brand = nil
		}
		cached.Nutriments = toRepoNutriments(fetched.Nutriments)
		cached.ServingSizeG = fetched.ServingSizeG
		cached.OFFPayload = fetched.RawPayload
		cached.FetchedAt = &now
		if err := s.repo.UpdateFromOFF(ctx, cached); err != nil {
			return nil, err
		}
		return cached, nil
	}

	p := &Product{
		Barcode:      &barcode,
		Name:         fetched.Name,
		Source:       SourceOFF,
		Nutriments:   toRepoNutriments(fetched.Nutriments),
		ServingSizeG: fetched.ServingSizeG,
		OFFPayload:   fetched.RawPayload,
		FetchedAt:    &now,
	}
	if fetched.Brand != "" {
		b := fetched.Brand
		p.Brand = &b
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// CreateManualInput captures the body of POST /products. Despite the historic
// name, it now also covers flat-imported recipes (Source=recipe + ExternalURL).
type CreateManualInput struct {
	Name         string
	Brand        *string
	Barcode      *string
	Source       Source  // defaults to SourceManual when zero-value
	ExternalURL  *string // optional provenance link (≤ 2048 chars; handler validates)
	ServingSizeG *float64
	Nutriments   Nutriments
}

// CreateManual creates a product row. Source defaults to SourceManual; callers
// (handler) may pass SourceRecipe + ExternalURL to register a flat-imported
// recipe — those rows have no product_components and nutriment_computed_at
// stays null. Returns *ErrBarcodeExists when the supplied barcode collides.
func (s *Service) CreateManual(ctx context.Context, in CreateManualInput) (*Product, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("name required")
	}
	if in.Barcode != nil {
		if existing, err := s.repo.GetByBarcode(ctx, *in.Barcode); err == nil {
			return nil, &ErrBarcodeExists{ExistingID: existing.ID}
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	source := in.Source
	if source == "" {
		source = SourceManual
	}
	p := &Product{
		Name:         in.Name,
		Brand:        in.Brand,
		Barcode:      in.Barcode,
		ExternalURL:  in.ExternalURL,
		Source:       source,
		ServingSizeG: in.ServingSizeG,
		Nutriments:   in.Nutriments,
	}
	if err := s.repo.Insert(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Recipe-related errors. Distinct from ErrNotFound because handlers map them
// to specific REST shapes.
var (
	ErrComponentsRequired = errors.New("components required")
	ErrRecipeAsComponent  = errors.New("recipe as component not supported")
	ErrNotARecipe         = errors.New("not a recipe")
)

// ErrProductInUseAsComponent is returned by Delete when the product is
// referenced by at least one product_components row. Recipes carries the
// list of using recipes so the handler can surface them in the 409 body.
type ErrProductInUseAsComponent struct {
	Recipes []RecipeRef
}

func (e *ErrProductInUseAsComponent) Error() string {
	return fmt.Sprintf("product is in use by %d recipe(s)", len(e.Recipes))
}

// ErrDuplicateComponent is returned by CreateRecipe when the same product_id
// appears more than once in the supplied components.
type ErrDuplicateComponent struct {
	ProductID   uuid.UUID
	Occurrences int
}

func (e *ErrDuplicateComponent) Error() string {
	return fmt.Sprintf("component %s appears %d times", e.ProductID, e.Occurrences)
}

// Delete removes a product, preserving historical meal_entries via snapshot
// materialisation. Refuses with *ErrProductInUseAsComponent when any recipe
// references this product as a component (RecipesUsing pre-check) — the FK
// itself is ON DELETE RESTRICT, so we'd also fail at the DELETE step, but the
// pre-check gives the handler an actionable recipe list for the 409 body.
func (s *Service) Delete(ctx context.Context, productID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, productID); err != nil {
		return err
	}
	return store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := NewRepo(tx)
		recipes, err := txRepo.RecipesUsing(ctx, productID)
		if err != nil {
			return err
		}
		if len(recipes) > 0 {
			return &ErrProductInUseAsComponent{Recipes: recipes}
		}
		if err := txRepo.MaterialiseSnapshot(ctx, productID); err != nil {
			return err
		}
		return txRepo.Delete(ctx, productID)
	})
}

// ErrComponentNotFound carries the missing component product_id so the handler
// can echo it in the error body.
type ErrComponentNotFound struct {
	ProductID uuid.UUID
}

func (e *ErrComponentNotFound) Error() string {
	return fmt.Sprintf("component product %s not found", e.ProductID)
}

// ErrComponentQuantityInvalid carries the offending component id.
type ErrComponentQuantityInvalid struct {
	ProductID uuid.UUID
}

func (e *ErrComponentQuantityInvalid) Error() string {
	return fmt.Sprintf("component %s quantity_g must be > 0", e.ProductID)
}

// RecipeComponentInput is one ingredient in a CreateRecipeInput.
type RecipeComponentInput struct {
	ProductID uuid.UUID
	QuantityG float64
}

// CreateRecipeInput is the body of POST /products/recipes.
type CreateRecipeInput struct {
	Name         string
	Components   []RecipeComponentInput
	ServingSizeG *float64
}

// CreateRecipeResult is what the handler echoes: the new product plus the
// resolved components (with the joined component-product info).
type CreateRecipeResult struct {
	Product    *Product
	Components []*ComponentWithProduct
}

// CreateRecipe creates a composite product with source=recipe and persists its
// component_components rows in a single transaction. Nutriments are computed
// from components at creation time.
func (s *Service) CreateRecipe(ctx context.Context, in CreateRecipeInput) (*CreateRecipeResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("name required")
	}
	if len(in.Components) == 0 {
		return nil, ErrComponentsRequired
	}

	// Reject duplicate product_id references: silently summing two rows for
	// the same ingredient is a footgun (and we have no DB UNIQUE constraint
	// to fall back on). Loud-over-silent — agents learn the rule fast and
	// the hint tells them to sum the quantities themselves.
	occurrences := make(map[uuid.UUID]int, len(in.Components))
	for _, c := range in.Components {
		occurrences[c.ProductID]++
	}
	for pid, n := range occurrences {
		if n > 1 {
			return nil, &ErrDuplicateComponent{ProductID: pid, Occurrences: n}
		}
	}

	// Resolve components: each input must reference a real, non-recipe product
	// with a positive quantity_g. We resolve outside the transaction so the
	// validation errors return without touching the DB.
	resolved := make([]*ComponentWithProduct, 0, len(in.Components))
	for i, c := range in.Components {
		if c.QuantityG <= 0 {
			return nil, &ErrComponentQuantityInvalid{ProductID: c.ProductID}
		}
		cp, err := s.repo.GetByID(ctx, c.ProductID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, &ErrComponentNotFound{ProductID: c.ProductID}
			}
			return nil, err
		}
		if cp.Source == SourceRecipe {
			return nil, ErrRecipeAsComponent
		}
		resolved = append(resolved, &ComponentWithProduct{
			Component:        Component{ComponentProductID: c.ProductID, QuantityG: c.QuantityG, Position: i},
			ComponentProduct: *cp,
		})
	}

	now := time.Now().UTC()
	computed := ComputeRecipeNutriments(resolved)

	p := &Product{
		Name:                in.Name,
		Source:              SourceRecipe,
		ServingSizeG:        in.ServingSizeG,
		Nutriments:          computed,
		NutrimentComputedAt: &now,
	}

	if err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := NewRepo(tx)
		if err := txRepo.Insert(ctx, p); err != nil {
			return err
		}
		comps := make([]Component, 0, len(resolved))
		for i, r := range resolved {
			comps = append(comps, Component{
				ComponentProductID: r.ComponentProductID,
				QuantityG:          r.QuantityG,
				Position:           i,
			})
		}
		txComponents := NewComponentsRepo(tx)
		if err := txComponents.InsertComponents(ctx, p.ID, comps); err != nil {
			return err
		}
		// Reflect the new component IDs back on resolved for the response.
		for i := range resolved {
			resolved[i].Component = comps[i]
			resolved[i].ProductID = p.ID
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("create recipe: %w", err)
	}

	return &CreateRecipeResult{Product: p, Components: resolved}, nil
}

// RecomputeRecipe re-derives a recipe product's nutriments from the CURRENT
// effective nutriments of its components.
func (s *Service) RecomputeRecipe(ctx context.Context, id uuid.UUID) (*CreateRecipeResult, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Source != SourceRecipe {
		return nil, ErrNotARecipe
	}

	crepo := NewComponentsRepo(s.pool)
	resolved, err := crepo.ListComponents(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	computed := ComputeRecipeNutriments(resolved)
	if err := s.repo.UpdateRecipeNutriments(ctx, p.ID, computed, now); err != nil {
		return nil, err
	}
	// Refresh the product row for the response.
	refreshed, err := s.repo.GetByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return &CreateRecipeResult{Product: refreshed, Components: resolved}, nil
}

func toRepoNutriments(n off.Nutriments) Nutriments {
	return Nutriments{
		KcalPer100g:     n.KcalPer100g,
		ProteinGPer100g: n.ProteinGPer100g,
		CarbsGPer100g:   n.CarbsGPer100g,
		FatGPer100g:     n.FatGPer100g,
		FiberGPer100g:   n.FiberGPer100g,
		SugarGPer100g:   n.SugarGPer100g,
		SaltGPer100g:    n.SaltGPer100g,

		IronMgPer100g:        n.IronMgPer100g,
		CalciumMgPer100g:     n.CalciumMgPer100g,
		VitaminDMcgPer100g:   n.VitaminDMcgPer100g,
		VitaminB12McgPer100g: n.VitaminB12McgPer100g,
		VitaminCMgPer100g:    n.VitaminCMgPer100g,
		MagnesiumMgPer100g:   n.MagnesiumMgPer100g,
		PotassiumMgPer100g:   n.PotassiumMgPer100g,
		ZincMgPer100g:        n.ZincMgPer100g,
	}
}
