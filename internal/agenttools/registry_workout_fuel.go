package agenttools

import (
	"encoding/json"
	"net/url"
)

// Workout-fuel domain — in-session fueling events (gels, drinks, salt tabs,
// caffeine). Ported from internal/mcpserver/tools_workout_fuel.go onto the
// shared registry (unify-mcp-tool-registry). MCP-only; arg structs and
// descriptions are byte-identical to the prior bespoke registration so the
// announced schema is unchanged. patch_workout_fuel preserves the workout_id
// empty-string-clear tri-state (omit = leave, "<uuid>" = set, "" = clear).

func init() { registerMCPDomain(workoutFuelSpecs()) }

type LogWorkoutFuelArgs struct {
	Name           string   `json:"name" jsonschema:"product/brand name of the gel, drink, salt tab, or caffeine source. Required — rehearsal data depends on knowing WHAT was taken."`
	LoggedAt       string   `json:"logged_at" jsonschema:"when the fueling event happened, RFC 3339 timestamp"`
	QuantityMl     *float64 `json:"quantity_ml,omitempty" jsonschema:"optional volume in millilitres (drinks only); must be greater than zero if supplied"`
	CarbsG         *float64 `json:"carbs_g,omitempty" jsonschema:"optional carbohydrate amount in grams; >= 0"`
	SodiumMg       *float64 `json:"sodium_mg,omitempty" jsonschema:"optional sodium amount in milligrams; >= 0"`
	PotassiumMg    *float64 `json:"potassium_mg,omitempty" jsonschema:"optional potassium amount in milligrams; >= 0"`
	CaffeineMg     *float64 `json:"caffeine_mg,omitempty" jsonschema:"optional caffeine amount in milligrams; >= 0. Pass 0 explicitly to signal 'measured, no caffeine' (e.g. a decaf product) — distinct from omitting (which means 'not measured')."`
	Note           string   `json:"note,omitempty" jsonschema:"optional free-text note (rehearsal observations, flavour, GI feel)"`
	WorkoutID      string   `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this entry to. The link is metadata; workout_fueling_summary aggregates by logged_at time-window matching, not by this tag."`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args."`
}

type ListWorkoutFuelArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

type PatchWorkoutFuelArgs struct {
	ID             string   `json:"id" jsonschema:"the id of the workout-fuel entry to update"`
	Name           *string  `json:"name,omitempty" jsonschema:"new name"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	QuantityMl     *float64 `json:"quantity_ml,omitempty" jsonschema:"new volume in millilitres; must be greater than zero"`
	CarbsG         *float64 `json:"carbs_g,omitempty" jsonschema:"new carbohydrate amount in grams; >= 0"`
	SodiumMg       *float64 `json:"sodium_mg,omitempty" jsonschema:"new sodium amount in milligrams; >= 0"`
	PotassiumMg    *float64 `json:"potassium_mg,omitempty" jsonschema:"new potassium amount in milligrams; >= 0"`
	CaffeineMg     *float64 `json:"caffeine_mg,omitempty" jsonschema:"new caffeine amount in milligrams; >= 0"`
	Note           *string  `json:"note,omitempty" jsonschema:"new note"`
	WorkoutID      *string  `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteWorkoutFuelArgs struct {
	ID             string `json:"id" jsonschema:"the id of the workout-fuel entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func workoutFuelSpecs() []Spec {
	return []Spec{
		{
			Name: "log_workout_fuel",
			Description: "Record an in-session fueling event — gel, electrolyte drink, salt tab, " +
				"caffeine pill, pre-race espresso. ROUTING RULE: plain water / juice (volume only) " +
				"goes to log_hydration; anything with electrolytes / carbs / caffeine goes here. " +
				"`name` is REQUIRED (rehearsal data depends on knowing WHAT was taken). At least " +
				"one of quantity_ml/carbs_g/sodium_mg/potassium_mg/caffeine_mg must be supplied — " +
				"the API rejects empty entries. Pass `caffeine_mg: 0` explicitly to signal " +
				"'measured, no caffeine' (decaf product); omit the field to mean 'not measured'. " +
				"The optional workout_id link is metadata; workout_fueling_summary aggregates by " +
				"logged_at time-window matching, not by this tag.",
			SchemaType: LogWorkoutFuelArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogWorkoutFuelArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					Name        string   `json:"name"`
					LoggedAt    string   `json:"logged_at"`
					QuantityMl  *float64 `json:"quantity_ml,omitempty"`
					CarbsG      *float64 `json:"carbs_g,omitempty"`
					SodiumMg    *float64 `json:"sodium_mg,omitempty"`
					PotassiumMg *float64 `json:"potassium_mg,omitempty"`
					CaffeineMg  *float64 `json:"caffeine_mg,omitempty"`
					Note        string   `json:"note,omitempty"`
					WorkoutID   string   `json:"workout_id,omitempty"`
				}{
					Name: a.Name, LoggedAt: a.LoggedAt, QuantityMl: a.QuantityMl,
					CarbsG: a.CarbsG, SodiumMg: a.SodiumMg, PotassiumMg: a.PotassiumMg,
					CaffeineMg: a.CaffeineMg, Note: a.Note, WorkoutID: a.WorkoutID,
				})
				return HTTPCall{Method: "POST", Path: "/workout-fuel", Body: body}, nil
			},
		},
		{
			Name: "list_workout_fuel",
			Description: "List workout-fuel entries whose logged_at falls within the half-open " +
				"[from, to) RFC 3339 window. Window is capped at 92 days. Use workout_fueling_summary " +
				"instead when you want the per-workout pre/intra/post composition.",
			SchemaType: ListWorkoutFuelArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListWorkoutFuelArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/workout-fuel", Query: q}, nil
			},
		},
		{
			Name: "patch_workout_fuel",
			Description: "Partially update an existing workout-fuel entry. Only supplied fields " +
				"are changed. `workout_id`: pass \"<uuid>\" to set, \"\" to clear, omit to leave " +
				"unchanged.",
			SchemaType: PatchWorkoutFuelArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchWorkoutFuelArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.Name != nil {
					payload["name"] = *a.Name
				}
				if a.LoggedAt != nil {
					payload["logged_at"] = *a.LoggedAt
				}
				if a.QuantityMl != nil {
					payload["quantity_ml"] = *a.QuantityMl
				}
				if a.CarbsG != nil {
					payload["carbs_g"] = *a.CarbsG
				}
				if a.SodiumMg != nil {
					payload["sodium_mg"] = *a.SodiumMg
				}
				if a.PotassiumMg != nil {
					payload["potassium_mg"] = *a.PotassiumMg
				}
				if a.CaffeineMg != nil {
					payload["caffeine_mg"] = *a.CaffeineMg
				}
				if a.Note != nil {
					payload["note"] = *a.Note
				}
				if a.WorkoutID != nil {
					payload["workout_id"] = *a.WorkoutID
				}
				body, _ := json.Marshal(payload)
				return HTTPCall{Method: "PATCH", Path: "/workout-fuel/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_workout_fuel",
			Description: "Delete a workout-fuel entry. Returns an empty result on success.",
			SchemaType:  DeleteWorkoutFuelArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteWorkoutFuelArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/workout-fuel/" + url.PathEscape(a.ID)}, nil
			},
		},
	}
}
