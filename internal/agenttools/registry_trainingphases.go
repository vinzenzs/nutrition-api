package agenttools

import (
	"encoding/json"
	"net/url"
)

// Training-phase + goal-template tools — the periodization layer the desktop
// coach reads and edits. Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). The arg structs (including the goal-range model
// tpGoalRange) and descriptions are byte-identical to the prior bespoke
// registrations so the announced schema is unchanged.
//
// This file ports two resource families the bespoke tools_trainingphases.go
// registered together: phase CRUD (create/list/get/update/delete) and goal
// template CRUD (set/list/get/delete).

func init() { registerMCPDomain(trainingPhasesSpecs()) }

// tpGoalRange mirrors the unified REST `{min?, max?}` body. Both bounds are
// optional; at least one MUST be present when the field is supplied. It is a
// package-local copy of the goals.GoalRange shape (the goals domain has not been
// ported to this registry yet); the type name does not appear in the reflected
// JSON schema, so the structural parity is what matters.
type tpGoalRange struct {
	Min *float64 `json:"min,omitempty" jsonschema:"minimum target value"`
	Max *float64 `json:"max,omitempty" jsonschema:"maximum target value"`
}

// ----------------------------------------------------------------------
// Phases: 5 tools (create / list / get / patch / delete)
// ----------------------------------------------------------------------

// CreatePhaseArgs is the input to create_phase.
type CreatePhaseArgs struct {
	Name              string  `json:"name" jsonschema:"phase name (user-chosen, e.g. 'build-block-2')"`
	Type              string  `json:"type" jsonschema:"one of: base, build, peak, recovery, race_week, off_season, other"`
	StartDate         string  `json:"start_date" jsonschema:"inclusive start date in YYYY-MM-DD"`
	EndDate           string  `json:"end_date" jsonschema:"inclusive end date in YYYY-MM-DD (must be >= start_date)"`
	DefaultTemplateID *string `json:"default_template_id,omitempty" jsonschema:"optional goal-template UUID; when set, the template's bounds drive adherence on every date in the phase (subject to per-date overrides winning)"`
	Notes             *string `json:"notes,omitempty" jsonschema:"optional free-text notes"`
	IdempotencyKey    string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ListPhasesArgs is the input to list_phases.
type ListPhasesArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD (max 730 days from from)"`
}

// GetPhaseArgs is the input to get_phase.
type GetPhaseArgs struct {
	PhaseID string `json:"phase_id" jsonschema:"phase UUID"`
}

// UpdatePhaseArgs is the input to update_phase.
type UpdatePhaseArgs struct {
	PhaseID           string  `json:"phase_id" jsonschema:"phase UUID"`
	Name              *string `json:"name,omitempty"`
	Type              *string `json:"type,omitempty" jsonschema:"one of: base, build, peak, recovery, race_week, off_season, other"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	DefaultTemplateID *string `json:"default_template_id,omitempty" jsonschema:"empty string clears, UUID string sets, missing leaves unchanged"`
	Notes             *string `json:"notes,omitempty"`
	IdempotencyKey    string  `json:"idempotency_key,omitempty"`
}

// DeletePhaseArgs is the input to delete_phase.
type DeletePhaseArgs struct {
	PhaseID        string `json:"phase_id" jsonschema:"phase UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// ----------------------------------------------------------------------
// Goal templates: 4 tools (set / list / get / delete)
// ----------------------------------------------------------------------

// SetGoalTemplateArgs is the input to set_goal_template.
type SetGoalTemplateArgs struct {
	Name  string  `json:"name" jsonschema:"template name (user-chosen, kebab-case-ish, e.g. 'weekday-easy-training')"`
	Notes *string `json:"notes,omitempty" jsonschema:"optional free-text notes"`

	Kcal *tpGoalRange `json:"kcal,omitempty"`

	ProteinG *tpGoalRange `json:"protein_g,omitempty"`
	CarbsG   *tpGoalRange `json:"carbs_g,omitempty"`
	FatG     *tpGoalRange `json:"fat_g,omitempty"`

	FiberG *tpGoalRange `json:"fiber_g,omitempty"`
	SugarG *tpGoalRange `json:"sugar_g,omitempty"`
	SaltG  *tpGoalRange `json:"salt_g,omitempty"`

	IronMg        *tpGoalRange `json:"iron_mg,omitempty"`
	CalciumMg     *tpGoalRange `json:"calcium_mg,omitempty"`
	VitaminDMcg   *tpGoalRange `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *tpGoalRange `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *tpGoalRange `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *tpGoalRange `json:"magnesium_mg,omitempty"`
	PotassiumMg   *tpGoalRange `json:"potassium_mg,omitempty"`
	ZincMg        *tpGoalRange `json:"zinc_mg,omitempty"`
}

// ListGoalTemplatesArgs is the (empty) input to list_goal_templates.
type ListGoalTemplatesArgs struct{}

// GetGoalTemplateArgs is the input to get_goal_template.
type GetGoalTemplateArgs struct {
	Name string `json:"name" jsonschema:"template name"`
}

// DeleteGoalTemplateArgs is the input to delete_goal_template.
type DeleteGoalTemplateArgs struct {
	Name           string `json:"name" jsonschema:"template name"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func trainingPhasesSpecs() []Spec {
	return []Spec{
		{
			Name: "create_phase",
			Description: "Create a training phase: a named date range tagged with type (base, build, peak, recovery, race_week, off_season, other). " +
				"Optionally points at a goal template via default_template_id — when set, the template's bounds become the default daily goals for every date in [start_date, end_date]. " +
				"Per-date overrides (set_daily_goal_override) still win over the phase's template. " +
				"Omitting default_template_id creates a phase visible in list_phases that does NOT drive adherence — useful for tagging a date range with a type without committing to a template yet.",
			SchemaType: CreatePhaseArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreatePhaseArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name              string  `json:"name"`
					Type              string  `json:"type"`
					StartDate         string  `json:"start_date"`
					EndDate           string  `json:"end_date"`
					DefaultTemplateID *string `json:"default_template_id,omitempty"`
					Notes             *string `json:"notes,omitempty"`
				}{
					Name:              a.Name,
					Type:              a.Type,
					StartDate:         a.StartDate,
					EndDate:           a.EndDate,
					DefaultTemplateID: a.DefaultTemplateID,
					Notes:             a.Notes,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/phases", Body: body}, nil
			},
		},
		{
			Name:        "list_phases",
			Description: "List training phases intersecting [from, to] (both inclusive YYYY-MM-DD; max 730 days). Each result includes default_template_name when a template is attached.",
			SchemaType:  ListPhasesArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListPhasesArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/phases", Query: q}, nil
			},
		},
		{
			Name:        "get_phase",
			Description: "Fetch a single phase by UUID.",
			SchemaType:  GetPhaseArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetPhaseArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/phases/" + url.PathEscape(a.PhaseID)}, nil
			},
		},
		{
			Name: "update_phase",
			Description: "Partially update a phase. Tri-state on default_template_id: empty string clears the template link, " +
				"a UUID string sets a new one, and omitting the field leaves it unchanged. Bumps updated_at, " +
				"which is also the tiebreaker when phases overlap (most-recently-updated wins for adherence).",
			SchemaType: UpdatePhaseArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UpdatePhaseArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name              *string `json:"name,omitempty"`
					Type              *string `json:"type,omitempty"`
					StartDate         *string `json:"start_date,omitempty"`
					EndDate           *string `json:"end_date,omitempty"`
					DefaultTemplateID *string `json:"default_template_id,omitempty"`
					Notes             *string `json:"notes,omitempty"`
				}{
					Name:              a.Name,
					Type:              a.Type,
					StartDate:         a.StartDate,
					EndDate:           a.EndDate,
					DefaultTemplateID: a.DefaultTemplateID,
					Notes:             a.Notes,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/phases/" + url.PathEscape(a.PhaseID), Body: body}, nil
			},
		},
		{
			Name:        "delete_phase",
			Description: "Delete a phase. Adherence on dates that were in the phase falls through to the next step (override, singleton default, or none). Any template the phase pointed at is unaffected.",
			SchemaType:  DeletePhaseArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeletePhaseArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/phases/" + url.PathEscape(a.PhaseID)}, nil
			},
		},
		{
			Name: "set_goal_template",
			Description: "Create or replace a named goal template (reusable goal-set). Full-replace semantics: absent nutrient bounds are stored as NULL. " +
				"A template is meant to be attached to a phase via create_phase or update_phase's default_template_id — once attached, editing the template's bounds propagates to every phase pointing at it on the next adherence read. " +
				"No apply step required; template edits are intentionally cheap. Retries are NOT safe (PUT rejects Idempotency-Key).",
			SchemaType: SetGoalTemplateArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a SetGoalTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Notes         *string      `json:"notes,omitempty"`
					Kcal          *tpGoalRange `json:"kcal,omitempty"`
					ProteinG      *tpGoalRange `json:"protein_g,omitempty"`
					CarbsG        *tpGoalRange `json:"carbs_g,omitempty"`
					FatG          *tpGoalRange `json:"fat_g,omitempty"`
					FiberG        *tpGoalRange `json:"fiber_g,omitempty"`
					SugarG        *tpGoalRange `json:"sugar_g,omitempty"`
					SaltG         *tpGoalRange `json:"salt_g,omitempty"`
					IronMg        *tpGoalRange `json:"iron_mg,omitempty"`
					CalciumMg     *tpGoalRange `json:"calcium_mg,omitempty"`
					VitaminDMcg   *tpGoalRange `json:"vitamin_d_mcg,omitempty"`
					VitaminB12Mcg *tpGoalRange `json:"vitamin_b12_mcg,omitempty"`
					VitaminCMg    *tpGoalRange `json:"vitamin_c_mg,omitempty"`
					MagnesiumMg   *tpGoalRange `json:"magnesium_mg,omitempty"`
					PotassiumMg   *tpGoalRange `json:"potassium_mg,omitempty"`
					ZincMg        *tpGoalRange `json:"zinc_mg,omitempty"`
				}{
					Notes:         a.Notes,
					Kcal:          a.Kcal,
					ProteinG:      a.ProteinG,
					CarbsG:        a.CarbsG,
					FatG:          a.FatG,
					FiberG:        a.FiberG,
					SugarG:        a.SugarG,
					SaltG:         a.SaltG,
					IronMg:        a.IronMg,
					CalciumMg:     a.CalciumMg,
					VitaminDMcg:   a.VitaminDMcg,
					VitaminB12Mcg: a.VitaminB12Mcg,
					VitaminCMg:    a.VitaminCMg,
					MagnesiumMg:   a.MagnesiumMg,
					PotassiumMg:   a.PotassiumMg,
					ZincMg:        a.ZincMg,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				// PUT — no Idempotency-Key (the backend rejects it on PUT; the
				// generic dispatcher centrally drops the header on PUT).
				return HTTPCall{Method: "PUT", Path: "/goal-templates/" + url.PathEscape(a.Name), Body: body}, nil
			},
		},
		{
			Name:        "list_goal_templates",
			Description: "List every goal template ordered by name ascending. Each entry includes its full goal-bound set.",
			SchemaType:  ListGoalTemplatesArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/goal-templates"}, nil
			},
		},
		{
			Name:        "get_goal_template",
			Description: "Fetch a goal template by name.",
			SchemaType:  GetGoalTemplateArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetGoalTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/goal-templates/" + url.PathEscape(a.Name)}, nil
			},
		},
		{
			Name:        "delete_goal_template",
			Description: "Delete a goal template by name. Refused with 409 template_in_use (and a referencing_phases list) if any phase points at it via default_template_id. Reassign or delete those phases first.",
			SchemaType:  DeleteGoalTemplateArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteGoalTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/goal-templates/" + url.PathEscape(a.Name)}, nil
			},
		},
	}
}
