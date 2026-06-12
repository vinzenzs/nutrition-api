package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	ID        string  `json:"id" jsonschema:"the plan UUID"`
	Name      *string `json:"name,omitempty" jsonschema:"optional new name"`
	RaceID    *string `json:"race_id,omitempty" jsonschema:"optional new race UUID"`
	StartDate *string `json:"start_date,omitempty" jsonschema:"optional new start date YYYY-MM-DD (moves all materialized dates on re-materialize)"`
	Notes     *string `json:"notes,omitempty" jsonschema:"optional new notes"`
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

type AddPlanSlotArgs struct {
	PlanID          string                  `json:"plan_id" jsonschema:"the plan UUID"`
	WeekID          string                  `json:"week_id" jsonschema:"the week UUID"`
	Weekday         int                     `json:"weekday" jsonschema:"day of week 0 (Monday) through 6 (Sunday)"`
	Ordinal         int                     `json:"ordinal" jsonschema:"order of this session within the day (0-based)"`
	TemplateID      string                  `json:"template_id" jsonschema:"the workout-template UUID this slot schedules"`
	TimeOfDay       *string                 `json:"time_of_day,omitempty" jsonschema:"optional local start time HH:MM or HH:MM:SS"`
	TargetOverrides []SlotTargetOverrideArg `json:"target_overrides,omitempty" jsonschema:"optional per-intent target overrides that supersede the template's targets (e.g. progress an interval pace across weeks); at most one per intent"`
	IdempotencyKey  string                  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type PatchPlanSlotArgs struct {
	PlanID          string                   `json:"plan_id" jsonschema:"the plan UUID"`
	SlotID          string                   `json:"slot_id" jsonschema:"the slot UUID"`
	Weekday         *int                     `json:"weekday,omitempty" jsonschema:"optional new weekday 0..6"`
	Ordinal         *int                     `json:"ordinal,omitempty" jsonschema:"optional new within-day order"`
	TemplateID      *string                  `json:"template_id,omitempty" jsonschema:"optional new template UUID"`
	TimeOfDay       *string                  `json:"time_of_day,omitempty" jsonschema:"optional new local start time HH:MM or HH:MM:SS"`
	TargetOverrides *[]SlotTargetOverrideArg `json:"target_overrides,omitempty" jsonschema:"optional replacement override list (replaces wholesale; empty list clears all overrides)"`
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

// ----- handlers -----

func handleCreateTrainingPlan(ctx context.Context, c *apiClient, args CreateTrainingPlanArgs) *mcp.CallToolResult {
	body, err := json.Marshal(map[string]any{"name": args.Name, "race_id": args.RaceID, "start_date": args.StartDate, "notes": args.Notes})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_training_plan", args)
	status, resp, err := c.Post(ctx, "/training-plans", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleListTrainingPlans(ctx context.Context, c *apiClient, _ ListTrainingPlansArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/training-plans", nil)
	return toToolResult(status, body, err)
}

func handleGetTrainingPlan(ctx context.Context, c *apiClient, args GetTrainingPlanArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/training-plans/"+url.PathEscape(args.ID), nil)
	return toToolResult(status, body, err)
}

func handlePatchTrainingPlan(ctx context.Context, c *apiClient, args PatchTrainingPlanArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Name != nil {
		payload["name"] = *args.Name
	}
	if args.RaceID != nil {
		payload["race_id"] = *args.RaceID
	}
	if args.StartDate != nil {
		payload["start_date"] = *args.StartDate
	}
	if args.Notes != nil {
		payload["notes"] = *args.Notes
	}
	return patchJSON(ctx, c, "/training-plans/"+url.PathEscape(args.ID), payload)
}

func handleDeleteTrainingPlan(ctx context.Context, c *apiClient, args DeleteTrainingPlanArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_training_plan", args)
	return deleteNoContent(ctx, c, "/training-plans/"+url.PathEscape(args.ID), key)
}

func handleAddPlanWeek(ctx context.Context, c *apiClient, args AddPlanWeekArgs) *mcp.CallToolResult {
	body, err := json.Marshal(map[string]any{"ordinal": args.Ordinal, "phase_id": args.PhaseID, "notes": args.Notes})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "add_plan_week", args)
	status, resp, err := c.Post(ctx, "/training-plans/"+url.PathEscape(args.PlanID)+"/weeks", nil, body, key)
	return toToolResult(status, resp, err)
}

func handlePatchPlanWeek(ctx context.Context, c *apiClient, args PatchPlanWeekArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Ordinal != nil {
		payload["ordinal"] = *args.Ordinal
	}
	if args.PhaseID != nil {
		payload["phase_id"] = *args.PhaseID
	}
	if args.Notes != nil {
		payload["notes"] = *args.Notes
	}
	return patchJSON(ctx, c, "/training-plans/"+url.PathEscape(args.PlanID)+"/weeks/"+url.PathEscape(args.WeekID), payload)
}

func handleDeletePlanWeek(ctx context.Context, c *apiClient, args DeletePlanWeekArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_plan_week", args)
	return deleteNoContent(ctx, c, "/training-plans/"+url.PathEscape(args.PlanID)+"/weeks/"+url.PathEscape(args.WeekID), key)
}

func handleAddPlanSlot(ctx context.Context, c *apiClient, args AddPlanSlotArgs) *mcp.CallToolResult {
	payload := map[string]any{"weekday": args.Weekday, "ordinal": args.Ordinal, "template_id": args.TemplateID, "time_of_day": args.TimeOfDay}
	if args.TargetOverrides != nil {
		payload["target_overrides"] = args.TargetOverrides
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "add_plan_slot", args)
	status, resp, err := c.Post(ctx, "/training-plans/"+url.PathEscape(args.PlanID)+"/weeks/"+url.PathEscape(args.WeekID)+"/slots", nil, body, key)
	return toToolResult(status, resp, err)
}

func handlePatchPlanSlot(ctx context.Context, c *apiClient, args PatchPlanSlotArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Weekday != nil {
		payload["weekday"] = *args.Weekday
	}
	if args.Ordinal != nil {
		payload["ordinal"] = *args.Ordinal
	}
	if args.TemplateID != nil {
		payload["template_id"] = *args.TemplateID
	}
	if args.TimeOfDay != nil {
		payload["time_of_day"] = *args.TimeOfDay
	}
	if args.TargetOverrides != nil {
		// Present (possibly empty → clears all overrides); replaces wholesale.
		payload["target_overrides"] = *args.TargetOverrides
	}
	return patchJSON(ctx, c, "/training-plans/"+url.PathEscape(args.PlanID)+"/slots/"+url.PathEscape(args.SlotID), payload)
}

func handleGetWorkoutProgram(ctx context.Context, c *apiClient, args GetWorkoutProgramArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/workouts/"+url.PathEscape(args.ID)+"/program", nil)
	return toToolResult(status, body, err)
}

func handleDeletePlanSlot(ctx context.Context, c *apiClient, args DeletePlanSlotArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_plan_slot", args)
	return deleteNoContent(ctx, c, "/training-plans/"+url.PathEscape(args.PlanID)+"/slots/"+url.PathEscape(args.SlotID), key)
}

func handleMaterializeTrainingPlan(ctx context.Context, c *apiClient, args MaterializeTrainingPlanArgs) *mcp.CallToolResult {
	body, err := json.Marshal(map[string]any{"scope": args.Scope, "week": args.Week, "from": args.From, "to": args.To})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	// Naturally idempotent (slot-keyed); no Idempotency-Key needed.
	status, resp, err := c.Post(ctx, "/training-plans/"+url.PathEscape(args.PlanID)+"/materialize", nil, body, "")
	return toToolResult(status, resp, err)
}

// patchJSON marshals payload and PATCHes it (no idempotency key on PATCH).
func patchJSON(ctx context.Context, c *apiClient, path string, payload map[string]any) *mcp.CallToolResult {
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	status, resp, err := c.Patch(ctx, path, body, "")
	return toToolResult(status, resp, err)
}

// deleteNoContent issues a DELETE and returns an empty result on 204.
func deleteNoContent(ctx context.Context, c *apiClient, path, key string) *mcp.CallToolResult {
	status, resp, err := c.Delete(ctx, path, key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, resp, err)
}

func registerTrainingPlanTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{Name: "create_training_plan",
		Description: "Create a training plan (name + start_date = the Monday of week 1, optional race link). Then add weeks and slots, and materialize."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a CreateTrainingPlanArgs) (*mcp.CallToolResult, any, error) {
			return handleCreateTrainingPlan(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "list_training_plans", Description: "List training plans."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a ListTrainingPlansArgs) (*mcp.CallToolResult, any, error) {
			return handleListTrainingPlans(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_training_plan", Description: "Get a training plan with its nested weeks and slots."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a GetTrainingPlanArgs) (*mcp.CallToolResult, any, error) {
			return handleGetTrainingPlan(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "patch_training_plan",
		Description: "Update a plan's name / race_id / start_date / notes. Shifting start_date and re-materializing moves all planned dates."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a PatchTrainingPlanArgs) (*mcp.CallToolResult, any, error) {
			return handlePatchTrainingPlan(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "delete_training_plan", Description: "Delete a plan (cascades weeks and slots). Materialized workouts are detached, not deleted."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a DeleteTrainingPlanArgs) (*mcp.CallToolResult, any, error) {
			return handleDeleteTrainingPlan(ctx, c, a), nil, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "add_plan_week", Description: "Add a week (ordinal >= 1, unique in the plan; optional phase link) to a plan."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a AddPlanWeekArgs) (*mcp.CallToolResult, any, error) {
			return handleAddPlanWeek(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "patch_plan_week", Description: "Update a plan week's ordinal / phase_id / notes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a PatchPlanWeekArgs) (*mcp.CallToolResult, any, error) {
			return handlePatchPlanWeek(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "delete_plan_week", Description: "Delete a plan week (cascades its slots)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a DeletePlanWeekArgs) (*mcp.CallToolResult, any, error) {
			return handleDeletePlanWeek(ctx, c, a), nil, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "add_plan_slot",
		Description: "Add a day-slot to a plan week: a weekday (0=Mon..6=Sun), a within-day ordinal, the workout-template to schedule, an optional time_of_day, and optional per-intent target_overrides (e.g. run a tempo template's interval at pace 7:15 this week, faster next week) so one template can progress across the plan."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a AddPlanSlotArgs) (*mcp.CallToolResult, any, error) {
			return handleAddPlanSlot(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "patch_plan_slot", Description: "Update a slot's weekday / ordinal / template_id / time_of_day / target_overrides (replaces the override list wholesale; empty clears). Re-materialize to retarget the planned workout."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a PatchPlanSlotArgs) (*mcp.CallToolResult, any, error) {
			return handlePatchPlanSlot(ctx, c, a), nil, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: "delete_plan_slot", Description: "Delete a plan slot."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a DeletePlanSlotArgs) (*mcp.CallToolResult, any, error) {
			return handleDeletePlanSlot(ctx, c, a), nil, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "materialize_training_plan",
		Description: "Expand a plan into dated, planned workouts. scope is all, week (with week ordinal), or range (with from/to dates). Idempotent and slot-keyed: re-running updates the same planned workouts rather than duplicating; completed sessions are never reverted."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a MaterializeTrainingPlanArgs) (*mcp.CallToolResult, any, error) {
			return handleMaterializeTrainingPlan(ctx, c, a), nil, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_workout_program",
		Description: "Get a planned workout's effective program — its template steps with the plan slot's per-intent target overrides applied (e.g. the interval at pace 7:15). A workout with no template returns sport/name and no steps."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a GetWorkoutProgramArgs) (*mcp.CallToolResult, any, error) {
			return handleGetWorkoutProgram(ctx, c, a), nil, nil
		})
}
