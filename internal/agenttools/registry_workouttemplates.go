package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Workout-template tools — the reusable, structured workout library the desktop
// coach reads and edits. Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry). The arg structs (including the step model wtStep /
// wtSubStep / wtDuration / wtTarget) and descriptions are byte-identical to the
// prior bespoke registrations so the announced schema is unchanged.

func init() { registerMCPDomain(workoutTemplatesSpecs()) }

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

func workoutTemplatesSpecs() []Spec {
	return []Spec{
		{
			Name: "create_workout_template",
			Description: "Create a reusable, structured workout template — a sport plus an ordered program of steps " +
				"(warmup / intervals with target zones / cooldown) and repeat groups. This is the library the training " +
				"plan references and the Garmin watch push compiles. Steps are validated; repeat groups are single-level.",
			SchemaType: CreateWorkoutTemplateArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateWorkoutTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Sport                string   `json:"sport"`
					Name                 string   `json:"name"`
					Description          *string  `json:"description,omitempty"`
					EstimatedDurationSec *int     `json:"estimated_duration_sec,omitempty"`
					Steps                []wtStep `json:"steps"`
				}{a.Sport, a.Name, a.Description, a.EstimatedDurationSec, a.Steps})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/workout-templates", Body: body}, nil
			},
		},
		{
			Name:        "list_workout_templates",
			Description: "List workout templates, optionally filtered by sport (run | bike | swim | strength | other).",
			SchemaType:  ListWorkoutTemplatesArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListWorkoutTemplatesArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Sport != "" {
					q.Set("sport", a.Sport)
				}
				return HTTPCall{Method: "GET", Path: "/workout-templates", Query: q}, nil
			},
		},
		{
			Name:        "get_workout_template",
			Description: "Fetch a single workout template by id, including its full step program.",
			SchemaType:  GetWorkoutTemplateArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetWorkoutTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "GET", Path: "/workout-templates/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "patch_workout_template",
			Description: "Update a workout template. Any of sport / name / description / estimated_duration_sec / steps " +
				"may be supplied; omitted fields are unchanged. A supplied `steps` array replaces the whole program.",
			SchemaType: PatchWorkoutTemplateArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchWorkoutTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				payload := map[string]any{}
				if a.Sport != nil {
					payload["sport"] = *a.Sport
				}
				if a.Name != nil {
					payload["name"] = *a.Name
				}
				if a.Description != nil {
					payload["description"] = *a.Description
				}
				if a.EstimatedDurationSec != nil {
					payload["estimated_duration_sec"] = *a.EstimatedDurationSec
				}
				if a.Steps != nil {
					payload["steps"] = a.Steps
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/workout-templates/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_workout_template",
			Description: "Delete a workout template by id. Returns an empty result on success.",
			SchemaType:  DeleteWorkoutTemplateArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteWorkoutTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/workout-templates/" + url.PathEscape(a.ID)}, nil
			},
		},
	}
}
