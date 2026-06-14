package products

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned by Get* methods when no row matches.
var ErrNotFound = errors.New("product not found")

// ErrBarcodeExists is returned when an Insert / Upsert would violate the
// unique barcode constraint.
type ErrBarcodeExists struct {
	ExistingID uuid.UUID
}

func (e *ErrBarcodeExists) Error() string {
	return fmt.Sprintf("barcode already exists on product %s", e.ExistingID)
}

// Repo persists product rows.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectAllColumns = `
    id, barcode, name, brand, external_url, source,
    kcal_per_100g, protein_g_per_100g, carbs_g_per_100g, fat_g_per_100g,
    fiber_g_per_100g, sugar_g_per_100g, salt_g_per_100g,
    iron_mg_per_100g, calcium_mg_per_100g, vitamin_d_mcg_per_100g, vitamin_b12_mcg_per_100g,
    vitamin_c_mg_per_100g, magnesium_mg_per_100g, potassium_mg_per_100g, zinc_mg_per_100g,
    serving_size_g, off_payload, fetched_at, last_logged_at, last_logged_quantity_g,
    nutriment_computed_at, ingredients, created_at, updated_at
`

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectAllColumns+` FROM products WHERE id = $1`, id)
	return scanProduct(row)
}

func (r *Repo) GetByBarcode(ctx context.Context, barcode string) (*Product, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectAllColumns+` FROM products WHERE barcode = $1`, barcode)
	return scanProduct(row)
}

// GetByExternalURL returns the most recently created product whose external_url
// matches exactly. Used by the Cookidoo import to make re-import idempotent.
// Returns ErrNotFound when no product carries that URL.
func (r *Repo) GetByExternalURL(ctx context.Context, externalURL string) (*Product, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectAllColumns+`
        FROM products WHERE external_url = $1
        ORDER BY created_at DESC LIMIT 1`, externalURL)
	return scanProduct(row)
}

// Insert creates a new product row. On unique-violation for barcode, returns
// *ErrBarcodeExists carrying the conflicting row's id.
func (r *Repo) Insert(ctx context.Context, p *Product) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	const q = `
        INSERT INTO products (
            id, barcode, name, brand, external_url, source,
            kcal_per_100g, protein_g_per_100g, carbs_g_per_100g, fat_g_per_100g,
            fiber_g_per_100g, sugar_g_per_100g, salt_g_per_100g,
            iron_mg_per_100g, calcium_mg_per_100g, vitamin_d_mcg_per_100g, vitamin_b12_mcg_per_100g,
            vitamin_c_mg_per_100g, magnesium_mg_per_100g, potassium_mg_per_100g, zinc_mg_per_100g,
            serving_size_g, off_payload, fetched_at, last_logged_at, last_logged_quantity_g,
            nutriment_computed_at, ingredients, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6,
            $7, $8, $9, $10, $11, $12, $13,
            $14, $15, $16, $17, $18, $19, $20, $21,
            $22, $23, $24, $25, $26,
            $27, $28, $29, $30
        )
    `
	_, err := r.q.Exec(ctx, q,
		p.ID, p.Barcode, p.Name, p.Brand, p.ExternalURL, p.Source,
		p.Nutriments.KcalPer100g, p.Nutriments.ProteinGPer100g, p.Nutriments.CarbsGPer100g, p.Nutriments.FatGPer100g,
		p.Nutriments.FiberGPer100g, p.Nutriments.SugarGPer100g, p.Nutriments.SaltGPer100g,
		p.Nutriments.IronMgPer100g, p.Nutriments.CalciumMgPer100g, p.Nutriments.VitaminDMcgPer100g, p.Nutriments.VitaminB12McgPer100g,
		p.Nutriments.VitaminCMgPer100g, p.Nutriments.MagnesiumMgPer100g, p.Nutriments.PotassiumMgPer100g, p.Nutriments.ZincMgPer100g,
		p.ServingSizeG, p.OFFPayload, p.FetchedAt, p.LastLoggedAt, p.LastLoggedQuantityG,
		p.NutrimentComputedAt, ingredientsParam(p.Ingredients), p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" && strings.Contains(pg.ConstraintName, "barcode") {
			if p.Barcode != nil {
				existing, getErr := r.GetByBarcode(ctx, *p.Barcode)
				if getErr == nil {
					return &ErrBarcodeExists{ExistingID: existing.ID}
				}
			}
			return &ErrBarcodeExists{}
		}
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

// UpdateFromOFF refreshes an existing product row with new parsed OFF data.
// Caller is expected to set the IDs and source on p. Refreshes macros AND
// micros from the supplied Nutriments; leaves nutriment_computed_at untouched.
func (r *Repo) UpdateFromOFF(ctx context.Context, p *Product) error {
	p.UpdatedAt = time.Now().UTC()
	const q = `
        UPDATE products SET
            name = $2, brand = $3,
            kcal_per_100g = $4, protein_g_per_100g = $5, carbs_g_per_100g = $6, fat_g_per_100g = $7,
            fiber_g_per_100g = $8, sugar_g_per_100g = $9, salt_g_per_100g = $10,
            iron_mg_per_100g = $11, calcium_mg_per_100g = $12, vitamin_d_mcg_per_100g = $13,
            vitamin_b12_mcg_per_100g = $14, vitamin_c_mg_per_100g = $15, magnesium_mg_per_100g = $16,
            potassium_mg_per_100g = $17, zinc_mg_per_100g = $18,
            serving_size_g = $19, off_payload = $20, fetched_at = $21, updated_at = $22
        WHERE id = $1
    `
	tag, err := r.q.Exec(ctx, q,
		p.ID, p.Name, p.Brand,
		p.Nutriments.KcalPer100g, p.Nutriments.ProteinGPer100g, p.Nutriments.CarbsGPer100g, p.Nutriments.FatGPer100g,
		p.Nutriments.FiberGPer100g, p.Nutriments.SugarGPer100g, p.Nutriments.SaltGPer100g,
		p.Nutriments.IronMgPer100g, p.Nutriments.CalciumMgPer100g, p.Nutriments.VitaminDMcgPer100g,
		p.Nutriments.VitaminB12McgPer100g, p.Nutriments.VitaminCMgPer100g, p.Nutriments.MagnesiumMgPer100g,
		p.Nutriments.PotassiumMgPer100g, p.Nutriments.ZincMgPer100g,
		p.ServingSizeG, p.OFFPayload, p.FetchedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateMutable overwrites the user-editable columns (name, serving_size_g, and
// all nutriment macros/micros) plus updated_at on an existing product row. The
// caller is expected to have read the current row, overlaid the patch fields,
// and passed the merged Product — so omitted fields retain their current values.
// Used by PATCH /products/{id}. Leaves source, external_url, barcode, and the
// last-logged columns untouched.
func (r *Repo) UpdateMutable(ctx context.Context, p *Product) error {
	p.UpdatedAt = time.Now().UTC()
	const q = `
        UPDATE products SET
            name = $2, serving_size_g = $3,
            kcal_per_100g = $4, protein_g_per_100g = $5, carbs_g_per_100g = $6, fat_g_per_100g = $7,
            fiber_g_per_100g = $8, sugar_g_per_100g = $9, salt_g_per_100g = $10,
            iron_mg_per_100g = $11, calcium_mg_per_100g = $12, vitamin_d_mcg_per_100g = $13,
            vitamin_b12_mcg_per_100g = $14, vitamin_c_mg_per_100g = $15, magnesium_mg_per_100g = $16,
            potassium_mg_per_100g = $17, zinc_mg_per_100g = $18,
            updated_at = $19
        WHERE id = $1
    `
	tag, err := r.q.Exec(ctx, q,
		p.ID, p.Name, p.ServingSizeG,
		p.Nutriments.KcalPer100g, p.Nutriments.ProteinGPer100g, p.Nutriments.CarbsGPer100g, p.Nutriments.FatGPer100g,
		p.Nutriments.FiberGPer100g, p.Nutriments.SugarGPer100g, p.Nutriments.SaltGPer100g,
		p.Nutriments.IronMgPer100g, p.Nutriments.CalciumMgPer100g, p.Nutriments.VitaminDMcgPer100g,
		p.Nutriments.VitaminB12McgPer100g, p.Nutriments.VitaminCMgPer100g, p.Nutriments.MagnesiumMgPer100g,
		p.Nutriments.PotassiumMgPer100g, p.Nutriments.ZincMgPer100g,
		p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update product mutable: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateRecipeNutriments overwrites all macros and micros plus
// nutriment_computed_at on an existing recipe product row.
func (r *Repo) UpdateRecipeNutriments(ctx context.Context, id uuid.UUID, n Nutriments, computedAt time.Time) error {
	const q = `
        UPDATE products SET
            kcal_per_100g = $2, protein_g_per_100g = $3, carbs_g_per_100g = $4, fat_g_per_100g = $5,
            fiber_g_per_100g = $6, sugar_g_per_100g = $7, salt_g_per_100g = $8,
            iron_mg_per_100g = $9, calcium_mg_per_100g = $10, vitamin_d_mcg_per_100g = $11,
            vitamin_b12_mcg_per_100g = $12, vitamin_c_mg_per_100g = $13, magnesium_mg_per_100g = $14,
            potassium_mg_per_100g = $15, zinc_mg_per_100g = $16,
            nutriment_computed_at = $17, updated_at = now()
        WHERE id = $1 AND source = 'recipe'
    `
	tag, err := r.q.Exec(ctx, q,
		id,
		n.KcalPer100g, n.ProteinGPer100g, n.CarbsGPer100g, n.FatGPer100g,
		n.FiberGPer100g, n.SugarGPer100g, n.SaltGPer100g,
		n.IronMgPer100g, n.CalciumMgPer100g, n.VitaminDMcgPer100g,
		n.VitaminB12McgPer100g, n.VitaminCMgPer100g, n.MagnesiumMgPer100g,
		n.PotassiumMgPer100g, n.ZincMgPer100g,
		computedAt,
	)
	if err != nil {
		return fmt.Errorf("update recipe nutriments: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Search returns products whose lower(name) or lower(brand) contains q,
// ordered by last_logged_at desc (nulls last), then name asc.
func (r *Repo) Search(ctx context.Context, q string, limit int) ([]*Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	pattern := "%" + strings.ToLower(q) + "%"
	const sql = `
        SELECT ` + selectAllColumns + `
        FROM products
        WHERE lower(name) LIKE $1 OR (brand IS NOT NULL AND lower(brand) LIKE $1)
        ORDER BY last_logged_at DESC NULLS LAST, name ASC
        LIMIT $2
    `
	rows, err := r.q.Query(ctx, sql, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	var out []*Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListParams scopes a paginated /products listing.
type ListParams struct {
	Source string // "" means no filter; otherwise "off" | "manual" | "recipe"
	Limit  int
	Offset int
}

// List returns a page of products, ordered most-recently-used first then by
// name. Caller is responsible for clamping Limit/Offset (handler enforces the
// spec's max-200 / non-negative rules).
func (r *Repo) List(ctx context.Context, p ListParams) ([]*Product, error) {
	const sql = `
        SELECT ` + selectAllColumns + `
        FROM products
        WHERE ($1::text = '' OR source = $1)
        ORDER BY last_logged_at DESC NULLS LAST, name ASC
        LIMIT $2 OFFSET $3
    `
	rows, err := r.q.Query(ctx, sql, p.Source, p.Limit, p.Offset)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var out []*Product
	for rows.Next() {
		prod, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, prod)
	}
	return out, rows.Err()
}

// Count returns the unpaginated count for the given source filter.
func (r *Repo) Count(ctx context.Context, source string) (int64, error) {
	const sql = `SELECT COUNT(*) FROM products WHERE ($1::text = '' OR source = $1)`
	var n int64
	if err := r.q.QueryRow(ctx, sql, source).Scan(&n); err != nil {
		return 0, fmt.Errorf("count products: %w", err)
	}
	return n, nil
}

// RecipeRef pairs a recipe product's id with its name, used in the
// product-in-use-as-component 409 body so the user knows which recipes to
// clean up first.
type RecipeRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// RecipesUsing returns every recipe product that references productID via
// product_components.component_product_id. Empty slice when none. DISTINCT
// dedupes legacy rows where the same recipe contains the same component more
// than once (the create-recipe path now rejects duplicates upfront, but any
// row created before that validation can still produce a multi-row join).
func (r *Repo) RecipesUsing(ctx context.Context, productID uuid.UUID) ([]RecipeRef, error) {
	const sql = `
        SELECT DISTINCT p.id, p.name
        FROM product_components pc
        JOIN products p ON p.id = pc.product_id
        WHERE pc.component_product_id = $1
        ORDER BY p.name ASC
    `
	rows, err := r.q.Query(ctx, sql, productID)
	if err != nil {
		return nil, fmt.Errorf("recipes using: %w", err)
	}
	defer rows.Close()
	var out []RecipeRef
	for rows.Next() {
		var ref RecipeRef
		if err := rows.Scan(&ref.ID, &ref.Name); err != nil {
			return nil, fmt.Errorf("scan recipe ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// MaterialiseSnapshot copies productID's current name and nutriment columns
// into the snapshot_* fields of any meal_entries row whose product_id matches
// AND whose snapshot_name is still null. Freeform entries (with a snapshot
// already set) are left untouched.
func (r *Repo) MaterialiseSnapshot(ctx context.Context, productID uuid.UUID) error {
	const sql = `
        UPDATE meal_entries SET
            snapshot_name                  = COALESCE(snapshot_name,                  p.name),
            snapshot_kcal_per_100g         = COALESCE(snapshot_kcal_per_100g,         p.kcal_per_100g),
            snapshot_protein_g_per_100g    = COALESCE(snapshot_protein_g_per_100g,    p.protein_g_per_100g),
            snapshot_carbs_g_per_100g      = COALESCE(snapshot_carbs_g_per_100g,      p.carbs_g_per_100g),
            snapshot_fat_g_per_100g        = COALESCE(snapshot_fat_g_per_100g,        p.fat_g_per_100g),
            snapshot_fiber_g_per_100g      = COALESCE(snapshot_fiber_g_per_100g,      p.fiber_g_per_100g),
            snapshot_sugar_g_per_100g      = COALESCE(snapshot_sugar_g_per_100g,      p.sugar_g_per_100g),
            snapshot_salt_g_per_100g       = COALESCE(snapshot_salt_g_per_100g,       p.salt_g_per_100g),
            snapshot_iron_mg_per_100g      = COALESCE(snapshot_iron_mg_per_100g,      p.iron_mg_per_100g),
            snapshot_calcium_mg_per_100g   = COALESCE(snapshot_calcium_mg_per_100g,   p.calcium_mg_per_100g),
            snapshot_vitamin_d_mcg_per_100g  = COALESCE(snapshot_vitamin_d_mcg_per_100g,  p.vitamin_d_mcg_per_100g),
            snapshot_vitamin_b12_mcg_per_100g = COALESCE(snapshot_vitamin_b12_mcg_per_100g, p.vitamin_b12_mcg_per_100g),
            snapshot_vitamin_c_mg_per_100g = COALESCE(snapshot_vitamin_c_mg_per_100g, p.vitamin_c_mg_per_100g),
            snapshot_magnesium_mg_per_100g = COALESCE(snapshot_magnesium_mg_per_100g, p.magnesium_mg_per_100g),
            snapshot_potassium_mg_per_100g = COALESCE(snapshot_potassium_mg_per_100g, p.potassium_mg_per_100g),
            snapshot_zinc_mg_per_100g      = COALESCE(snapshot_zinc_mg_per_100g,      p.zinc_mg_per_100g),
            updated_at                     = now()
        FROM products p
        WHERE meal_entries.product_id = $1 AND p.id = $1 AND meal_entries.snapshot_name IS NULL
    `
	if _, err := r.q.Exec(ctx, sql, productID); err != nil {
		return fmt.Errorf("materialise snapshot: %w", err)
	}
	return nil
}

// Delete removes a single product row. FK ON DELETE SET NULL handles the
// meal_entries.product_id cleanup; product_components.component_product_id is
// ON DELETE RESTRICT and will reject the DELETE if any rows reference it
// (callers should pre-check via RecipesUsing).
func (r *Repo) Delete(ctx context.Context, productID uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchLastLoggedAt advances last_logged_at if ts is strictly greater than
// the current value (or if it is null) and writes the supplied quantityG to
// last_logged_quantity_g in the same atomic UPDATE so the two columns stay
// in lockstep. Older timestamps are ignored — backdated meal entries do not
// regress either column.
func (r *Repo) TouchLastLoggedAt(ctx context.Context, id uuid.UUID, ts time.Time, quantityG float64) error {
	const q = `
        UPDATE products
        SET last_logged_at = $2, last_logged_quantity_g = $3, updated_at = now()
        WHERE id = $1
          AND (last_logged_at IS NULL OR last_logged_at < $2)
    `
	_, err := r.q.Exec(ctx, q, id, ts, quantityG)
	if err != nil {
		return fmt.Errorf("touch last_logged_at: %w", err)
	}
	return nil
}

// TouchLastLoggedQuantityIfCurrent writes quantityG to last_logged_quantity_g
// ONLY when last_logged_at equals mealLoggedAt — i.e. when the meal being
// PATCHed is the one whose timestamp currently identifies this product's most
// recent log. Used by the meals PATCH path when only quantity_g changes (no
// logged_at change). When the patched meal is older than the most-recent log,
// the product's quantity is unchanged.
func (r *Repo) TouchLastLoggedQuantityIfCurrent(ctx context.Context, id uuid.UUID, mealLoggedAt time.Time, quantityG float64) error {
	const q = `
        UPDATE products
        SET last_logged_quantity_g = $3, updated_at = now()
        WHERE id = $1
          AND last_logged_at = $2
    `
	_, err := r.q.Exec(ctx, q, id, mealLoggedAt, quantityG)
	if err != nil {
		return fmt.Errorf("touch last_logged_quantity_g: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(s scanner) (*Product, error) {
	var p Product
	var ingredientsRaw []byte
	err := s.Scan(
		&p.ID, &p.Barcode, &p.Name, &p.Brand, &p.ExternalURL, &p.Source,
		&p.Nutriments.KcalPer100g, &p.Nutriments.ProteinGPer100g, &p.Nutriments.CarbsGPer100g, &p.Nutriments.FatGPer100g,
		&p.Nutriments.FiberGPer100g, &p.Nutriments.SugarGPer100g, &p.Nutriments.SaltGPer100g,
		&p.Nutriments.IronMgPer100g, &p.Nutriments.CalciumMgPer100g, &p.Nutriments.VitaminDMcgPer100g, &p.Nutriments.VitaminB12McgPer100g,
		&p.Nutriments.VitaminCMgPer100g, &p.Nutriments.MagnesiumMgPer100g, &p.Nutriments.PotassiumMgPer100g, &p.Nutriments.ZincMgPer100g,
		&p.ServingSizeG, &p.OFFPayload, &p.FetchedAt, &p.LastLoggedAt, &p.LastLoggedQuantityG,
		&p.NutrimentComputedAt, &ingredientsRaw, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan product: %w", err)
	}
	// jsonb arrives as raw JSON bytes; SQL NULL and jsonb 'null' both decode to
	// a nil slice (omitted from the response by the omitempty tag).
	if len(ingredientsRaw) > 0 {
		if err := json.Unmarshal(ingredientsRaw, &p.Ingredients); err != nil {
			return nil, fmt.Errorf("scan product ingredients: %w", err)
		}
	}
	return &p, nil
}

// ingredientsParam renders the ingredient slice for a jsonb column: SQL NULL
// when empty, otherwise the JSON-encoded array. Keeping empty as NULL (rather
// than jsonb 'null' or '[]') makes "no ingredients" a single canonical state.
func ingredientsParam(ings []string) any {
	if len(ings) == 0 {
		return nil
	}
	b, err := json.Marshal(ings)
	if err != nil {
		// []string always marshals; defensively fall back to NULL.
		return nil
	}
	return b
}
