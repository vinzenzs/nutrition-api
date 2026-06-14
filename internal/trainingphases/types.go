// Package trainingphases owns the training_phases and goal_templates tables.
// Phases are named date ranges tagged with a training type (build, peak, etc.);
// templates are reusable goal-sets that a phase can point at. Together they
// feed the goals resolver as a new step in the effective-goals chain (between
// per-date overrides and the singleton default).
package trainingphases

import (
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/goals"
)

// PhaseType is the small enumerated set of phase categorisations. Stored as
// TEXT + CHECK in Postgres so adding values later is one migration.
type PhaseType string

const (
	PhaseTypeBase      PhaseType = "base"
	PhaseTypeBuild     PhaseType = "build"
	PhaseTypePeak      PhaseType = "peak"
	PhaseTypeRecovery  PhaseType = "recovery"
	PhaseTypeRaceWeek  PhaseType = "race_week"
	PhaseTypeOffSeason PhaseType = "off_season"
	PhaseTypeOther     PhaseType = "other"
)

// AllPhaseTypes is the source of truth for the enum; used by validators and
// the JSON-error response that echoes allowed values.
var AllPhaseTypes = []PhaseType{
	PhaseTypeBase,
	PhaseTypeBuild,
	PhaseTypePeak,
	PhaseTypeRecovery,
	PhaseTypeRaceWeek,
	PhaseTypeOffSeason,
	PhaseTypeOther,
}

// IsValid reports whether t is one of AllPhaseTypes.
func (t PhaseType) IsValid() bool {
	for _, v := range AllPhaseTypes {
		if v == t {
			return true
		}
	}
	return false
}

// AllowedPhaseTypeStrings returns the enum values as plain strings — useful
// for error responses.
func AllowedPhaseTypeStrings() []string {
	out := make([]string, 0, len(AllPhaseTypes))
	for _, v := range AllPhaseTypes {
		out = append(out, string(v))
	}
	return out
}

// Template is a reusable goal-set. The nutrient bounds use the same Range
// shape and rules as goals.Goals (see internal/goals/types.go). The 15
// per-nutrient pointers map 1:1 to the columns on goal_templates.
type Template struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Notes *string   `json:"notes,omitempty"`

	Kcal *goals.Range `json:"kcal,omitempty"`

	ProteinG *goals.Range `json:"protein_g,omitempty"`
	CarbsG   *goals.Range `json:"carbs_g,omitempty"`
	FatG     *goals.Range `json:"fat_g,omitempty"`

	FiberG *goals.Range `json:"fiber_g,omitempty"`
	SugarG *goals.Range `json:"sugar_g,omitempty"`
	SaltG  *goals.Range `json:"salt_g,omitempty"`

	IronMg        *goals.Range `json:"iron_mg,omitempty"`
	CalciumMg     *goals.Range `json:"calcium_mg,omitempty"`
	VitaminDMcg   *goals.Range `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *goals.Range `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *goals.Range `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *goals.Range `json:"magnesium_mg,omitempty"`
	PotassiumMg   *goals.Range `json:"potassium_mg,omitempty"`
	ZincMg        *goals.Range `json:"zinc_mg,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AsGoals converts a Template to a *goals.Goals for use by the effective-
// goals resolver. The two timestamp fields are carried over so adherence
// responses can surface "the template was last updated at ..." if needed.
func (t *Template) AsGoals() *goals.Goals {
	if t == nil {
		return nil
	}
	return &goals.Goals{
		Kcal:          t.Kcal,
		ProteinG:      t.ProteinG,
		CarbsG:        t.CarbsG,
		FatG:          t.FatG,
		FiberG:        t.FiberG,
		SugarG:        t.SugarG,
		SaltG:         t.SaltG,
		IronMg:        t.IronMg,
		CalciumMg:     t.CalciumMg,
		VitaminDMcg:   t.VitaminDMcg,
		VitaminB12Mcg: t.VitaminB12Mcg,
		VitaminCMg:    t.VitaminCMg,
		MagnesiumMg:   t.MagnesiumMg,
		PotassiumMg:   t.PotassiumMg,
		ZincMg:        t.ZincMg,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

// Phase is a date range tagged with a training type. The optional
// DefaultTemplateID is the FK that links the phase to a template; when set
// AND a date in [StartDate, EndDate] is queried, the template's bounds
// drive adherence on that date (subject to per-date overrides winning).
type Phase struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	Type                PhaseType  `json:"type"`
	StartDate           time.Time  `json:"start_date"`
	EndDate             time.Time  `json:"end_date"`
	DefaultTemplateID   *uuid.UUID `json:"default_template_id,omitempty"`
	DefaultTemplateName *string    `json:"default_template_name,omitempty"`
	Notes               *string    `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
