package agenttools

import (
	"encoding/json"
	"net/url"
)

// Training-plan tools — the structured, multi-week plan the desktop coach builds
// (plan → weeks → day-slots) and materializes into dated, planned workouts.
// Ported from internal/mcpserver onto the shared registry (unify-mcp-tool-registry).
// The arg structs (including SlotTargetOverrideArg) and descriptions are
// byte-identical to the prior bespoke registrations so the announced schema is
// unchanged. The Build funcs reproduce the bespoke REST mapping exactly — in
// particular the create/add/materialize bodies are marshalled from map[string]any
// with raw pointer values, so absent optional pointers serialize as JSON null
// (not omitted), matching the bespoke handlers verbatim.

func init() { registerMCPDomain(trainingPlanSpecs()) }

// ----- plan -----

type CreateTrainingPlanArgs struct {
	Name           string  `json:"name" jsonschema:"plan name"`
	RaceID         *string `json:"race_id,omitempty" jsonschema:"optional race UUID this plan targets"`
	StartDate      string  `json:"start_date" jsonschema:"the Monday of week 1, YYYY-MM-DD"`
	Notes          *string `json:"notes,omitempty" jsonschema:"optional notes"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type ListTrainingPlansArgs struct{}

type GetTrainingPlanArgs struct {
	ID string `json:"id" jsonschema:"the plan UUID; returns the nested weeks and slots"`
}

type PatchTrainingPlanArgs struct {
	ID          string  `json:"id" jsonschema:"the plan UUID"`
	Name        *string `json:"name,omitempty" jsonschema:"optional new name"`
	RaceID      *string `json:"race_id,omitempty" jsonschema:"optional new race UUID"`
	StartDate   *string `json:"start_date,omitempty" jsonschema:"optional new start date YYYY-MM-DD (moves all materialized dates on re-materialize)"`
	Notes       *string `json:"notes,omitempty" jsonschema:"optional new notes"`
	Methodology *string `json:"methodology,omitempty" jsonschema:"optional curated Markdown plan-level methodology (Key Principles, cross-cutting reference) the coach reads via get_training_plan"`
}

type DeleteTrainingPlanArgs struct {
	ID             string `json:"id" jsonschema:"the plan UUID to delete (cascades weeks and slots)"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ----- week -----

type AddPlanWeekArgs struct {
	PlanID         string  `json:"plan_id" jsonschema:"the plan UUID"`
	Ordinal        int     `json:"ordinal" jsonschema:"week number, at least 1, unique within the plan"`
	PhaseID        *string `json:"phase_id,omitempty" jsonschema:"optional training-phase UUID for this week"`
	Notes          *string `json:"notes,omitempty" jsonschema:"optional notes"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type PatchPlanWeekArgs struct {
	PlanID  string  `json:"plan_id" jsonschema:"the plan UUID"`
	WeekID  string  `json:"week_id" jsonschema:"the week UUID"`
	Ordinal *int    `json:"ordinal,omitempty" jsonschema:"optional new ordinal"`
	PhaseID *string `json:"phase_id,omitempty" jsonschema:"optional new phase UUID"`
	Notes   *string `json:"notes,omitempty" jsonschema:"optional new notes"`
}

type DeletePlanWeekArgs struct {
	PlanID         string `json:"plan_id" jsonschema:"the plan UUID"`
	WeekID         string `json:"week_id" jsonschema:"the week UUID to delete (cascades its slots)"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ----- slot -----

// SlotTargetOverrideArg overrides the effort target of every template step whose
// intent matches, when the planned workout's effective program is resolved.
type SlotTargetOverrideArg struct {
	Intent string         `json:"intent" jsonschema:"step intent to override: warmup|active|interval|recovery|rest|cooldown"`
	Target map[string]any `json:"target" jsonschema:"effort target, same shape as a workout-template step target (e.g. {\"kind\":\"pace\",\"low_sec_per_km\":435,\"high_sec_per_km\":435} for 7:15/km)"`
}

// SlotDurationOverrideArg overrides the duration of every template step whose
// intent matches, when the planned workout's effective program is resolved (and
// drives the materialized session length).
type SlotDurationOverrideArg struct {
	Intent   string         `json:"intent" jsonschema:"step intent to override: warmup|active|interval|recovery|rest|cooldown"`
	Duration map[string]any `json:"duration" jsonschema:"bounded duration, same shape as a workout-template step duration (e.g. {\"kind\":\"time\",\"seconds\":3600} for 60min or {\"kind\":\"distance\",\"meters\":10000}); open/lap_button are rejected"`
}

type AddPlanSlotArgs struct {
	PlanID            string                    `json:"plan_id" jsonschema:"the plan UUID"`
	WeekID            string                    `json:"week_id" jsonschema:"the week UUID"`
	Weekday           int                       `json:"weekday" jsonschema:"day of week 0 (Monday) through 6 (Sunday)"`
	Ordinal           int                       `json:"ordinal" jsonschema:"order of this session within the day (0-based)"`
	TemplateID        string                    `json:"template_id" jsonschema:"the workout-template UUID this slot schedules"`
	TimeOfDay         *string                   `json:"time_of_day,omitempty" jsonschema:"optional local start time HH:MM or HH:MM:SS"`
	TargetOverrides   []SlotTargetOverrideArg   `json:"target_overrides,omitempty" jsonschema:"optional per-intent target overrides that supersede the template's targets (e.g. progress an interval pace across weeks); at most one per intent"`
	DurationOverrides []SlotDurationOverrideArg `json:"duration_overrides,omitempty" jsonschema:"optional per-intent duration overrides that supersede the template's step durations (e.g. progress a tempo block 75min→80min across weeks) and drive the materialized session length; at most one per intent"`
	IdempotencyKey    string                    `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type PatchPlanSlotArgs struct {
	PlanID            string                     `json:"plan_id" jsonschema:"the plan UUID"`
	SlotID            string                     `json:"slot_id" jsonschema:"the slot UUID"`
	Weekday           *int                       `json:"weekday,omitempty" jsonschema:"optional new weekday 0..6"`
	Ordinal           *int                       `json:"ordinal,omitempty" jsonschema:"optional new within-day order"`
	TemplateID        *string                    `json:"template_id,omitempty" jsonschema:"optional new template UUID"`
	TimeOfDay         *string                    `json:"time_of_day,omitempty" jsonschema:"optional new local start time HH:MM or HH:MM:SS"`
	TargetOverrides   *[]SlotTargetOverrideArg   `json:"target_overrides,omitempty" jsonschema:"optional replacement override list (replaces wholesale; empty list clears all overrides)"`
	DurationOverrides *[]SlotDurationOverrideArg `json:"duration_overrides,omitempty" jsonschema:"optional replacement duration-override list (replaces wholesale; empty list clears all duration overrides)"`
}

type GetWorkoutProgramArgs struct {
	ID string `json:"id" jsonschema:"the planned workout UUID; returns the effective steps (template + slot target overrides)"`
}

type DeletePlanSlotArgs struct {
	PlanID         string `json:"plan_id" jsonschema:"the plan UUID"`
	SlotID         string `json:"slot_id" jsonschema:"the slot UUID to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ----- materialize -----

type MaterializeTrainingPlanArgs struct {
	PlanID string  `json:"plan_id" jsonschema:"the plan UUID"`
	Scope  string  `json:"scope" jsonschema:"all | week | range"`
	Week   *int    `json:"week,omitempty" jsonschema:"required when scope is week: the week ordinal"`
	From   *string `json:"from,omitempty" jsonschema:"required when scope is range: inclusive start YYYY-MM-DD"`
	To     *string `json:"to,omitempty" jsonschema:"required when scope is range: inclusive end YYYY-MM-DD"`
}

func trainingPlanSpecs() []Spec {
	return []Spec{
		// ----- plan -----
		{
			Name:        "create_training_plan",
			Description: "Create a training plan (name + start_date = the Monday of week 1, optional race link). Then add weeks and slots, and materialize.",
			SchemaType:  CreateTrainingPlanArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateTrainingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(map[string]any{"name": a.Name, "race_id": a.RaceID, "start_date": a.StartDate, "notes": a.Notes})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/training-plans", Body: body}, nil
			},
		},
		{
			Name:        "list_training_plans",
			Description: "List training plans.",
			SchemaType:  ListTrainingPlansArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/training-plans"}, nil
			},
		},
		{
			Name:        "get_training_plan",
			Description: "Get a training plan with its nested weeks and slots.",
			SchemaType:  GetTrainingPlanArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetTrainingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/training-plans/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name:        "patch_training_plan",
			Description: "Update a plan's name / race_id / start_date / notes / methodology (plan-level coach reference). Shifting start_date and re-materializing moves all planned dates.",
			SchemaType:  PatchTrainingPlanArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchTrainingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.Name != nil {
					payload["name"] = *a.Name
				}
				if a.RaceID != nil {
					payload["race_id"] = *a.RaceID
				}
				if a.StartDate != nil {
					payload["start_date"] = *a.StartDate
				}
				if a.Notes != nil {
					payload["notes"] = *a.Notes
				}
				if a.Methodology != nil {
					payload["methodology"] = *a.Methodology
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/training-plans/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_training_plan",
			Description: "Delete a plan (cascades weeks and slots). Materialized workouts are detached, not deleted.",
			SchemaType:  DeleteTrainingPlanArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteTrainingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/training-plans/" + url.PathEscape(a.ID)}, nil
			},
		},

		// ----- week -----
		{
			Name:        "add_plan_week",
			Description: "Add a week (ordinal >= 1, unique in the plan; optional phase link) to a plan.",
			SchemaType:  AddPlanWeekArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a AddPlanWeekArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(map[string]any{"ordinal": a.Ordinal, "phase_id": a.PhaseID, "notes": a.Notes})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/weeks", Body: body}, nil
			},
		},
		{
			Name:        "patch_plan_week",
			Description: "Update a plan week's ordinal / phase_id / notes.",
			SchemaType:  PatchPlanWeekArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchPlanWeekArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.Ordinal != nil {
					payload["ordinal"] = *a.Ordinal
				}
				if a.PhaseID != nil {
					payload["phase_id"] = *a.PhaseID
				}
				if a.Notes != nil {
					payload["notes"] = *a.Notes
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/weeks/" + url.PathEscape(a.WeekID), Body: body}, nil
			},
		},
		{
			Name:        "delete_plan_week",
			Description: "Delete a plan week (cascades its slots).",
			SchemaType:  DeletePlanWeekArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeletePlanWeekArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/weeks/" + url.PathEscape(a.WeekID)}, nil
			},
		},

		// ----- slot -----
		{
			Name:        "add_plan_slot",
			Description: "Add a day-slot to a plan week: a weekday (0=Mon..6=Sun), a within-day ordinal, the workout-template to schedule, an optional time_of_day, and optional per-intent target_overrides (progress an interval pace across weeks) and duration_overrides (progress a block's length, e.g. 75min→80min) so one template can progress across the plan.",
			SchemaType:  AddPlanSlotArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a AddPlanSlotArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{"weekday": a.Weekday, "ordinal": a.Ordinal, "template_id": a.TemplateID, "time_of_day": a.TimeOfDay}
				if a.TargetOverrides != nil {
					payload["target_overrides"] = a.TargetOverrides
				}
				if a.DurationOverrides != nil {
					payload["duration_overrides"] = a.DurationOverrides
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/weeks/" + url.PathEscape(a.WeekID) + "/slots", Body: body}, nil
			},
		},
		{
			Name:        "patch_plan_slot",
			Description: "Update a slot's weekday / ordinal / template_id / time_of_day / target_overrides / duration_overrides (each override list replaces wholesale; empty clears). Re-materialize to retarget the planned workout.",
			SchemaType:  PatchPlanSlotArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchPlanSlotArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.Weekday != nil {
					payload["weekday"] = *a.Weekday
				}
				if a.Ordinal != nil {
					payload["ordinal"] = *a.Ordinal
				}
				if a.TemplateID != nil {
					payload["template_id"] = *a.TemplateID
				}
				if a.TimeOfDay != nil {
					payload["time_of_day"] = *a.TimeOfDay
				}
				if a.TargetOverrides != nil {
					// Present (possibly empty → clears all overrides); replaces wholesale.
					payload["target_overrides"] = *a.TargetOverrides
				}
				if a.DurationOverrides != nil {
					// Present (possibly empty → clears all duration overrides); replaces wholesale.
					payload["duration_overrides"] = *a.DurationOverrides
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/slots/" + url.PathEscape(a.SlotID), Body: body}, nil
			},
		},
		{
			Name:        "delete_plan_slot",
			Description: "Delete a plan slot.",
			SchemaType:  DeletePlanSlotArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeletePlanSlotArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/slots/" + url.PathEscape(a.SlotID)}, nil
			},
		},

		// ----- materialize -----
		{
			Name:        "materialize_training_plan",
			Description: "Expand a plan into dated, planned workouts. scope is all, week (with week ordinal), or range (with from/to dates). Idempotent and slot-keyed: re-running updates the same planned workouts rather than duplicating; completed sessions are never reverted.",
			SchemaType:  MaterializeTrainingPlanArgs{},
			Tier:        TierWriteAuto,
			// Re-runnable by design (see Spec.OmitIdempotencyKey): the bespoke
			// handler sent no key so re-materialize after a plan edit always
			// re-runs rather than replaying a stale response.
			OmitIdempotencyKey: true,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a MaterializeTrainingPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(map[string]any{"scope": a.Scope, "week": a.Week, "from": a.From, "to": a.To})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/training-plans/" + url.PathEscape(a.PlanID) + "/materialize", Body: body}, nil
			},
		},

		// ----- program -----
		{
			Name:        "get_workout_program",
			Description: "Get a planned workout's effective program — its template steps with the plan slot's per-intent target overrides applied (e.g. the interval at pace 7:15). A workout with no template returns sport/name and no steps.",
			SchemaType:  GetWorkoutProgramArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetWorkoutProgramArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/workouts/" + url.PathEscape(a.ID) + "/program"}, nil
			},
		},
	}
}
