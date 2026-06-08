package goals

import "time"

// Range is the unified target shape for every nutrient. Both bounds are
// optional; at least one must be present when the field is supplied (handler
// validation enforces that). A nil pointer at the parent level means "no goal
// for this nutrient".
type Range struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Goals is the singleton nutrition_goals row. Each pointer field is nil when
// no target has been set for that nutrient. The JSON marshaller omits empty
// objects so callers see only populated targets.
type Goals struct {
	Kcal *Range `json:"kcal,omitempty"`

	ProteinG *Range `json:"protein_g,omitempty"`
	CarbsG   *Range `json:"carbs_g,omitempty"`
	FatG     *Range `json:"fat_g,omitempty"`

	FiberG *Range `json:"fiber_g,omitempty"`
	SugarG *Range `json:"sugar_g,omitempty"`
	SaltG  *Range `json:"salt_g,omitempty"`

	IronMg        *Range `json:"iron_mg,omitempty"`
	CalciumMg     *Range `json:"calcium_mg,omitempty"`
	VitaminDMcg   *Range `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *Range `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *Range `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *Range `json:"magnesium_mg,omitempty"`
	PotassiumMg   *Range `json:"potassium_mg,omitempty"`
	ZincMg        *Range `json:"zinc_mg,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
