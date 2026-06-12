package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The step model mirrors the workout-templates REST contract. It is split into
// wtStep (a top-level node: single step OR repeat group) and wtSubStep (a
// repeat's children — single steps only), which both gives the agent a clean
// schema and encodes the single-level-repeat rule at the type level.

type wtDuration struct {
	Kind    string `json:"kind" jsonschema:"end condition: one of time, distance, lap_button, open"`
	Seconds *int   `json:"seconds,omitempty" jsonschema:"required for time durations; greater than 0"`
	Meters  *int   `json:"meters,omitempty" jsonschema:"required for distance durations; greater than 0"`
}

type wtTarget struct {
	Kind         string `json:"kind" jsonschema:"one of none, hr_zone, power_zone, pace, hr_bpm, power_w, rpe"`
	Low          *int   `json:"low,omitempty" jsonschema:"lower bound (zones are 1 to 5; hr_bpm, power_w, rpe in their own units)"`
	High         *int   `json:"high,omitempty" jsonschema:"upper bound; must be at least the lower bound"`
	LowSecPerKM  *int   `json:"low_sec_per_km,omitempty" jsonschema:"pace lower bound, seconds per km"`
	HighSecPerKM *int   `json:"high_sec_per_km,omitempty" jsonschema:"pace upper bound, seconds per km"`
}

type wtSubStep struct {
	Type     string      `json:"type" jsonschema:"always the literal step inside a repeat group"`
	Intent   string      `json:"intent" jsonschema:"one of warmup, active, interval, recovery, rest, cooldown"`
	Duration *wtDuration `json:"duration" jsonschema:"the step's end condition"`
	Target   *wtTarget   `json:"target" jsonschema:"the step's effort target; use kind none for untargeted"`
	Note     string      `json:"note,omitempty" jsonschema:"optional free-text cue"`
}

type wtStep struct {
	Type string `json:"type" jsonschema:"step for a single executable step, repeat for a repeat group"`
	// step fields
	Intent   string      `json:"intent,omitempty" jsonschema:"for a single step: one of warmup, active, interval, recovery, rest, cooldown"`
	Duration *wtDuration `json:"duration,omitempty" jsonschema:"for a single step: the end condition"`
	Target   *wtTarget   `json:"target,omitempty" jsonschema:"for a single step: the effort target"`
	Note     string      `json:"note,omitempty" jsonschema:"for a single step: optional free-text cue"`
	// repeat fields
	Count int         `json:"count,omitempty" jsonschema:"for a repeat group: number of iterations, at least 2"`
	Steps []wtSubStep `json:"steps,omitempty" jsonschema:"for a repeat group: single steps to repeat (no nested repeats)"`
}

type CreateWorkoutTemplateArgs struct {
	Sport                string   `json:"sport" jsonschema:"run | bike | swim | strength | other"`
	Name                 string   `json:"name" jsonschema:"human-readable session name"`
	Description          *string  `json:"description,omitempty" jsonschema:"optional notes about the session"`
	EstimatedDurationSec *int     `json:"estimated_duration_sec,omitempty" jsonschema:"optional advisory total duration in seconds; > 0"`
	Steps                []wtStep `json:"steps" jsonschema:"ordered, non-empty program of steps and repeat groups"`
	IdempotencyKey       string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived from args when omitted"`
}

type ListWorkoutTemplatesArgs struct {
	Sport string `json:"sport,omitempty" jsonschema:"optional filter: run | bike | swim | strength | other"`
}

type GetWorkoutTemplateArgs struct {
	ID string `json:"id" jsonschema:"the template UUID"`
}

type PatchWorkoutTemplateArgs struct {
	ID                   string   `json:"id" jsonschema:"the template UUID"`
	Sport                *string  `json:"sport,omitempty" jsonschema:"optional new sport"`
	Name                 *string  `json:"name,omitempty" jsonschema:"optional new name"`
	Description          *string  `json:"description,omitempty" jsonschema:"optional new description"`
	EstimatedDurationSec *int     `json:"estimated_duration_sec,omitempty" jsonschema:"optional new advisory duration in seconds"`
	Steps                []wtStep `json:"steps,omitempty" jsonschema:"optional: when supplied, replaces the whole step program"`
	IdempotencyKey       string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteWorkoutTemplateArgs struct {
	ID             string `json:"id" jsonschema:"the template UUID to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleCreateWorkoutTemplate(ctx context.Context, c *apiClient, args CreateWorkoutTemplateArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Sport                string   `json:"sport"`
		Name                 string   `json:"name"`
		Description          *string  `json:"description,omitempty"`
		EstimatedDurationSec *int     `json:"estimated_duration_sec,omitempty"`
		Steps                []wtStep `json:"steps"`
	}{args.Sport, args.Name, args.Description, args.EstimatedDurationSec, args.Steps})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_workout_template", args)
	status, respBody, err := c.Post(ctx, "/workout-templates", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListWorkoutTemplates(ctx context.Context, c *apiClient, args ListWorkoutTemplatesArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Sport != "" {
		q.Set("sport", args.Sport)
	}
	status, body, err := c.Get(ctx, "/workout-templates", q)
	return toToolResult(status, body, err)
}

func handleGetWorkoutTemplate(ctx context.Context, c *apiClient, args GetWorkoutTemplateArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/workout-templates/"+url.PathEscape(args.ID), nil)
	return toToolResult(status, body, err)
}

func handlePatchWorkoutTemplate(ctx context.Context, c *apiClient, args PatchWorkoutTemplateArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Sport != nil {
		payload["sport"] = *args.Sport
	}
	if args.Name != nil {
		payload["name"] = *args.Name
	}
	if args.Description != nil {
		payload["description"] = *args.Description
	}
	if args.EstimatedDurationSec != nil {
		payload["estimated_duration_sec"] = *args.EstimatedDurationSec
	}
	if args.Steps != nil {
		payload["steps"] = args.Steps
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	status, respBody, err := c.Patch(ctx, "/workout-templates/"+url.PathEscape(args.ID), body, "")
	return toToolResult(status, respBody, err)
}

func handleDeleteWorkoutTemplate(ctx context.Context, c *apiClient, args DeleteWorkoutTemplateArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_workout_template", args)
	status, respBody, err := c.Delete(ctx, "/workout-templates/"+url.PathEscape(args.ID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerWorkoutTemplateTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "create_workout_template",
		Description: "Create a reusable, structured workout template — a sport plus an ordered program of steps " +
			"(warmup / intervals with target zones / cooldown) and repeat groups. This is the library the training " +
			"plan references and the Garmin watch push compiles. Steps are validated; repeat groups are single-level.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreateWorkoutTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleCreateWorkoutTemplate(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_workout_templates",
		Description: "List workout templates, optionally filtered by sport (run | bike | swim | strength | other).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListWorkoutTemplatesArgs) (*mcp.CallToolResult, any, error) {
		return handleListWorkoutTemplates(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workout_template",
		Description: "Fetch a single workout template by id, including its full step program.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetWorkoutTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleGetWorkoutTemplate(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "patch_workout_template",
		Description: "Update a workout template. Any of sport / name / description / estimated_duration_sec / steps " +
			"may be supplied; omitted fields are unchanged. A supplied `steps` array replaces the whole program.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchWorkoutTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchWorkoutTemplate(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_workout_template",
		Description: "Delete a workout template by id. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteWorkoutTemplateArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteWorkoutTemplate(ctx, c, args), nil, nil
	})
}
