package meals

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MealType is breakfast / lunch / dinner / snack.
type MealType string

const (
	Breakfast MealType = "breakfast"
	Lunch     MealType = "lunch"
	Dinner    MealType = "dinner"
	Snack     MealType = "snack"
)

// ValidMealType returns true if s is one of the four allowed meal types.
func ValidMealType(s string) bool {
	switch MealType(s) {
	case Breakfast, Lunch, Dinner, Snack:
		return true
	}
	return false
}

// ParseMealType is a strict parser that returns an error for unknown values.
func ParseMealType(s string) (MealType, error) {
	if !ValidMealType(s) {
		return "", fmt.Errorf("invalid meal_type %q", s)
	}
	return MealType(s), nil
}

// Nutriments are the per-100g values stored on either side of the snapshot /
// product resolution. Includes macros and the micros added by daily-use-essentials.
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

// MealEntry mirrors a meal_entries row joined with its (optional) product, so
// the response always carries the effective name + nutriments.
type MealEntry struct {
	ID        uuid.UUID  `json:"id"`
	ProductID *uuid.UUID `json:"product_id,omitempty"`
	LoggedAt  time.Time  `json:"logged_at"`
	QuantityG float64    `json:"quantity_g"`
	MealType  *MealType  `json:"meal_type,omitempty"`
	Note      *string    `json:"note,omitempty"`
	WorkoutID *uuid.UUID `json:"workout_id,omitempty"`

	EffectiveName              string     `json:"effective_name"`
	EffectiveNutrimentsPer100g Nutriments `json:"effective_nutriments_per_100g"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
