package agenttools

import (
	"encoding/json"
)

// Goals tools — the user's configured daily nutrition targets (macros + micros)
// as unified {min?, max?} ranges. Ported from internal/mcpserver onto the shared
// registry (unify-mcp-tool-registry). get_goals is a pure read; set_goals is a
// full-replace PUT. The arg structs (and their json + jsonschema tags) are
// byte-identical to the prior bespoke registrations so the announced MCP schema
// is unchanged.

func init() { registerMCPDomain(goalsSpecs()) }

// GetGoalsArgs has no inputs but exists so the SDK can register the schema.
type GetGoalsArgs struct{}

// GoalRange mirrors the unified REST `{min?, max?}` body. Both bounds are
// optional; at least one MUST be present when the field is supplied.
type GoalRange struct {
	Min *float64 `json:"min,omitempty" jsonschema:"minimum target value"`
	Max *float64 `json:"max,omitempty" jsonschema:"maximum target value"`
}

// SetGoalsArgs is the full PUT /goals payload. Absent fields clear any
// previously stored target (full-replace semantics). After
// unify-adherence-shape, every goal uses the unified GoalRange shape — kcal
// included.
type SetGoalsArgs struct {
	Kcal *GoalRange `json:"kcal,omitempty" jsonschema:"daily kilocalorie target as a range {min, max}. For 'I want N kcal a day' the agent should pick a tolerance — e.g. {min: N*0.95, max: N*1.05} — unless the user states one explicitly."`

	ProteinG *GoalRange `json:"protein_g,omitempty" jsonschema:"protein grams per day (min/max)"`
	CarbsG   *GoalRange `json:"carbs_g,omitempty" jsonschema:"carb grams per day (min/max)"`
	FatG     *GoalRange `json:"fat_g,omitempty" jsonschema:"fat grams per day (min/max)"`

	FiberG *GoalRange `json:"fiber_g,omitempty" jsonschema:"fiber grams per day. Typically min-only ({min: 30}) but max also accepted."`
	SugarG *GoalRange `json:"sugar_g,omitempty" jsonschema:"sugar grams per day. Typically max-only ({max: 50})."`
	SaltG  *GoalRange `json:"salt_g,omitempty" jsonschema:"salt grams per day. Typically max-only."`

	IronMg        *GoalRange `json:"iron_mg,omitempty" jsonschema:"iron mg per day. Typically min-only."`
	CalciumMg     *GoalRange `json:"calcium_mg,omitempty" jsonschema:"calcium mg per day. Typically min-only."`
	VitaminDMcg   *GoalRange `json:"vitamin_d_mcg,omitempty" jsonschema:"vitamin D mcg per day. Typically min-only."`
	VitaminB12Mcg *GoalRange `json:"vitamin_b12_mcg,omitempty" jsonschema:"vitamin B12 mcg per day. Typically min-only."`
	VitaminCMg    *GoalRange `json:"vitamin_c_mg,omitempty" jsonschema:"vitamin C mg per day. Typically min-only."`
	MagnesiumMg   *GoalRange `json:"magnesium_mg,omitempty" jsonschema:"magnesium mg per day. Typically min-only."`
	PotassiumMg   *GoalRange `json:"potassium_mg,omitempty" jsonschema:"potassium mg per day. Typically min-only."`
	ZincMg        *GoalRange `json:"zinc_mg,omitempty" jsonschema:"zinc mg per day. Typically min-only."`
}

func goalsSpecs() []Spec {
	return []Spec{
		{
			Name: "get_goals",
			Description: "Get the user's currently configured nutrition goals (daily targets for macros and " +
				"micros). Returns {\"goals\": null} when no goals have been set yet — call set_goals to " +
				"establish them.",
			SchemaType: GetGoalsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/goals"}, nil
			},
		},
		{
			Name: "set_goals",
			Description: "Set or replace the user's nutrition goals. Full-replace semantics: a goal field " +
				"omitted from the call is CLEARED on the server (PUT, not PATCH). Every goal uses the unified " +
				"{min?, max?} range shape — including kcal. Examples: {\"kcal\":{\"min\":2090,\"max\":2310}, " +
				"\"protein_g\":{\"min\":150,\"max\":190}, \"fiber_g\":{\"min\":30}, \"sugar_g\":{\"max\":50}, " +
				"\"iron_mg\":{\"min\":14}}. For 'I want N kcal a day' pick a tolerance — e.g. ±5% — and emit " +
				"min and max explicitly. set_goals retries are NOT safe today (the REST backend rejects " +
				"Idempotency-Key on PUT after the harden-write-paths change); a network blip mid-call may " +
				"land twice — re-issue if you're unsure.",
			SchemaType: SetGoalsArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args SetGoalsArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				// Build the JSON body excluding idempotency_key (which goes on
				// the header). The payload uses the same unified GoalRange shape
				// for every field; nil fields are omitted (omitempty), giving the
				// full-replace clear-on-omit behaviour. PUT /goals does not accept
				// Idempotency-Key — the backend rejects it with 400
				// idempotency_unsupported_for_put, and the generic dispatcher
				// already skips the header on PUT.
				payload := struct {
					Kcal          *GoalRange `json:"kcal,omitempty"`
					ProteinG      *GoalRange `json:"protein_g,omitempty"`
					CarbsG        *GoalRange `json:"carbs_g,omitempty"`
					FatG          *GoalRange `json:"fat_g,omitempty"`
					FiberG        *GoalRange `json:"fiber_g,omitempty"`
					SugarG        *GoalRange `json:"sugar_g,omitempty"`
					SaltG         *GoalRange `json:"salt_g,omitempty"`
					IronMg        *GoalRange `json:"iron_mg,omitempty"`
					CalciumMg     *GoalRange `json:"calcium_mg,omitempty"`
					VitaminDMcg   *GoalRange `json:"vitamin_d_mcg,omitempty"`
					VitaminB12Mcg *GoalRange `json:"vitamin_b12_mcg,omitempty"`
					VitaminCMg    *GoalRange `json:"vitamin_c_mg,omitempty"`
					MagnesiumMg   *GoalRange `json:"magnesium_mg,omitempty"`
					PotassiumMg   *GoalRange `json:"potassium_mg,omitempty"`
					ZincMg        *GoalRange `json:"zinc_mg,omitempty"`
				}{
					Kcal:          args.Kcal,
					ProteinG:      args.ProteinG,
					CarbsG:        args.CarbsG,
					FatG:          args.FatG,
					FiberG:        args.FiberG,
					SugarG:        args.SugarG,
					SaltG:         args.SaltG,
					IronMg:        args.IronMg,
					CalciumMg:     args.CalciumMg,
					VitaminDMcg:   args.VitaminDMcg,
					VitaminB12Mcg: args.VitaminB12Mcg,
					VitaminCMg:    args.VitaminCMg,
					MagnesiumMg:   args.MagnesiumMg,
					PotassiumMg:   args.PotassiumMg,
					ZincMg:        args.ZincMg,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PUT", Path: "/goals", Body: body}, nil
			},
		},
	}
}
