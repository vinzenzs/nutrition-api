package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// ComponentsRepo persists product_components rows. Independent from Repo so it
// can run inside a transaction (any store.Querier) without dragging in
// Repo's larger surface.
type ComponentsRepo struct {
	q store.Querier
}

func NewComponentsRepo(q store.Querier) *ComponentsRepo {
	return &ComponentsRepo{q: q}
}

// InsertComponents writes the supplied components for productID. Positions are
// assigned from the slice index unless the caller already set Position. Each
// row's ID is generated when zero. Caller supplies a transactional store.Querier
// to keep this atomic with the parent product insert.
func (r *ComponentsRepo) InsertComponents(ctx context.Context, productID uuid.UUID, comps []Component) error {
	if len(comps) == 0 {
		return nil
	}
	const q = `
        INSERT INTO product_components (id, product_id, component_product_id, quantity_g, position)
        VALUES ($1, $2, $3, $4, $5)
    `
	for i := range comps {
		c := &comps[i]
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		c.ProductID = productID
		if c.Position == 0 {
			c.Position = i
		}
		if _, err := r.q.Exec(ctx, q, c.ID, c.ProductID, c.ComponentProductID, c.QuantityG, c.Position); err != nil {
			return fmt.Errorf("insert component %d: %w", i, err)
		}
	}
	return nil
}

// ListComponents returns the components of productID joined with each
// component's product row, ordered by position ASC.
func (r *ComponentsRepo) ListComponents(ctx context.Context, productID uuid.UUID) ([]*ComponentWithProduct, error) {
	// Columns are fully qualified (products.* aliased to p.*) because both
	// product_components and products carry an `id` column — unqualified
	// references would be ambiguous.
	const q = `
        SELECT
            pc.id, pc.product_id, pc.component_product_id, pc.quantity_g, pc.position,
            p.id, p.barcode, p.name, p.brand, p.source,
            p.kcal_per_100g, p.protein_g_per_100g, p.carbs_g_per_100g, p.fat_g_per_100g,
            p.fiber_g_per_100g, p.sugar_g_per_100g, p.salt_g_per_100g,
            p.iron_mg_per_100g, p.calcium_mg_per_100g, p.vitamin_d_mcg_per_100g, p.vitamin_b12_mcg_per_100g,
            p.vitamin_c_mg_per_100g, p.magnesium_mg_per_100g, p.potassium_mg_per_100g, p.zinc_mg_per_100g,
            p.serving_size_g, p.off_payload, p.fetched_at, p.last_logged_at,
            p.nutriment_computed_at, p.created_at, p.updated_at
        FROM product_components pc
        JOIN products p ON p.id = pc.component_product_id
        WHERE pc.product_id = $1
        ORDER BY pc.position ASC
    `
	rows, err := r.q.Query(ctx, q, productID)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	defer rows.Close()

	var out []*ComponentWithProduct
	for rows.Next() {
		var (
			cwp ComponentWithProduct
			p   Product
		)
		err := rows.Scan(
			&cwp.Component.ID, &cwp.Component.ProductID, &cwp.Component.ComponentProductID,
			&cwp.Component.QuantityG, &cwp.Component.Position,
			&p.ID, &p.Barcode, &p.Name, &p.Brand, &p.Source,
			&p.Nutriments.KcalPer100g, &p.Nutriments.ProteinGPer100g, &p.Nutriments.CarbsGPer100g, &p.Nutriments.FatGPer100g,
			&p.Nutriments.FiberGPer100g, &p.Nutriments.SugarGPer100g, &p.Nutriments.SaltGPer100g,
			&p.Nutriments.IronMgPer100g, &p.Nutriments.CalciumMgPer100g, &p.Nutriments.VitaminDMcgPer100g, &p.Nutriments.VitaminB12McgPer100g,
			&p.Nutriments.VitaminCMgPer100g, &p.Nutriments.MagnesiumMgPer100g, &p.Nutriments.PotassiumMgPer100g, &p.Nutriments.ZincMgPer100g,
			&p.ServingSizeG, &p.OFFPayload, &p.FetchedAt, &p.LastLoggedAt,
			&p.NutrimentComputedAt, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("scan component row: %w", err)
		}
		cwp.ComponentProduct = p
		out = append(out, &cwp)
	}
	return out, rows.Err()
}

// DeleteComponents removes every component row for productID.
func (r *ComponentsRepo) DeleteComponents(ctx context.Context, productID uuid.UUID) error {
	_, err := r.q.Exec(ctx, `DELETE FROM product_components WHERE product_id = $1`, productID)
	if err != nil {
		return fmt.Errorf("delete components: %w", err)
	}
	return nil
}
