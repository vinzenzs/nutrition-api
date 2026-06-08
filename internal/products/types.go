package products

import (
	"time"

	"github.com/google/uuid"
)

// Source is the provenance of a product row.
type Source string

const (
	SourceOFF    Source = "off"
	SourceManual Source = "manual"
	SourceRecipe Source = "recipe"
)

// Nutriments holds nullable per-100g values for macros and micros.
type Nutriments struct {
	KcalPer100g     *float64 `json:"kcal,omitempty"`
	ProteinGPer100g *float64 `json:"protein_g,omitempty"`
	CarbsGPer100g   *float64 `json:"carbs_g,omitempty"`
	FatGPer100g     *float64 `json:"fat_g,omitempty"`
	FiberGPer100g   *float64 `json:"fiber_g,omitempty"`
	SugarGPer100g   *float64 `json:"sugar_g,omitempty"`
	SaltGPer100g    *float64 `json:"salt_g,omitempty"`

	IronMgPer100g        *float64 `json:"iron_mg,omitempty"`
	CalciumMgPer100g     *float64 `json:"calcium_mg,omitempty"`
	VitaminDMcgPer100g   *float64 `json:"vitamin_d_mcg,omitempty"`
	VitaminB12McgPer100g *float64 `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMgPer100g    *float64 `json:"vitamin_c_mg,omitempty"`
	MagnesiumMgPer100g   *float64 `json:"magnesium_mg,omitempty"`
	PotassiumMgPer100g   *float64 `json:"potassium_mg,omitempty"`
	ZincMgPer100g        *float64 `json:"zinc_mg,omitempty"`
}

// Product mirrors a row in the products table.
type Product struct {
	ID           uuid.UUID  `json:"id"`
	Barcode      *string    `json:"barcode,omitempty"`
	Name         string     `json:"name"`
	Brand        *string    `json:"brand,omitempty"`
	// ExternalURL records where this product came from when neither OFF nor a
	// manual user entry. Today's first user is the Cookidoo extension.
	ExternalURL  *string    `json:"external_url,omitempty"`
	Source       Source     `json:"source"`
	ServingSizeG *float64   `json:"serving_size_g,omitempty"`
	Nutriments   Nutriments `json:"nutriments_per_100g"`
	OFFPayload   []byte     `json:"-"`
	FetchedAt    *time.Time `json:"fetched_at,omitempty"`
	LastLoggedAt *time.Time `json:"last_logged_at,omitempty"`
	// LastLoggedQuantityG is the quantity_g of the most recent meal entry that
	// advanced last_logged_at. The phone's scan→log flow defaults to this
	// value, dropping a typical scan from 3 taps to 2. Lockstep with
	// LastLoggedAt — see internal/products.Repo.TouchLastLoggedAt.
	LastLoggedQuantityG *float64 `json:"last_logged_quantity_g,omitempty"`
	// NutrimentComputedAt is set when a recipe product's nutriments were derived
	// from its components. Null for non-recipe products.
	NutrimentComputedAt *time.Time `json:"nutriment_computed_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Component is a single ingredient row in product_components.
type Component struct {
	ID                 uuid.UUID `json:"id"`
	ProductID          uuid.UUID `json:"product_id"`
	ComponentProductID uuid.UUID `json:"component_product_id"`
	QuantityG          float64   `json:"quantity_g"`
	Position           int       `json:"position"`
}

// ComponentWithProduct pairs a component row with its referenced product, used
// by recipe-nutriment computation and the ?expand=components view.
type ComponentWithProduct struct {
	Component
	ComponentProduct Product `json:"-"`
}
