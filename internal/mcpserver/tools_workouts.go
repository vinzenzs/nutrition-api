package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LogWorkoutArgs is the input for the log_workout tool. Every nullable column
// is a pointer so absent → null in the POST body.
type LogWorkoutArgs struct {
	ExternalID *string  `json:"external_id,omitempty" jsonschema:"writer-supplied dedup key, e.g. 'garmin:1234567'. Garmin and other sourced writers SHOULD set this. For agent-driven manual entries, leave it unset."`
	Source     string   `json:"source" jsonschema:"provenance: 'garmin' | 'manual' | 'other'. Use 'manual' for agent-driven entries."`
	Sport      string   `json:"sport" jsonschema:"'run' | 'bike' | 'swim' | 'strength' | 'other'"`
	Name       *string  `json:"name,omitempty" jsonschema:"optional human-readable label, e.g. 'Morning Z2 ride'"`
	StartedAt  string   `json:"started_at" jsonschema:"RFC 3339 timestamp the workout started"`
	EndedAt    string   `json:"ended_at" jsonschema:"RFC 3339 timestamp the workout ended; must be after started_at"`
	KcalBurned *float64 `json:"kcal_burned,omitempty" jsonschema:"calories burned during the session; positive number"`
	AvgHR      *int     `json:"avg_hr,omitempty" jsonschema:"average heart rate in bpm; positive integer"`
	TSS        *float64 `json:"tss,omitempty" jsonschema:"Training Stress Score; the intensity signal. Non-negative."`
	Notes      *string  `json:"notes,omitempty" jsonschema:"free-text notes (e.g. how the fueling went)"`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. Note: writers normally rely on external_id for dedup, not this header."`
}

type ListWorkoutsArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on started_at"`
	To   string `json:"to" jsonschema:"inclusive RFC 3339 upper bound on started_at; max 92 days from 'from'"`
}

type GetWorkoutArgs struct {
	ID string `json:"id" jsonschema:"the workout id"`
}

type PatchWorkoutArgs struct {
	ID         string   `json:"id" jsonschema:"the workout id to update"`
	Name       *string  `json:"name,omitempty" jsonschema:"new label"`
	Notes      *string  `json:"notes,omitempty" jsonschema:"new free-text notes"`
	KcalBurned *float64 `json:"kcal_burned,omitempty" jsonschema:"corrected kcal_burned (positive)"`
	AvgHR      *int     `json:"avg_hr,omitempty" jsonschema:"corrected average heart rate (positive)"`
	TSS        *float64 `json:"tss,omitempty" jsonschema:"corrected TSS (non-negative)"`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type DeleteWorkoutArgs struct {
	ID             string `json:"id" jsonschema:"the workout id to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type WorkoutFuelingSummaryArgs struct {
	WorkoutID     string `json:"workout_id" jsonschema:"the workout id to summarise fueling for"`
	PreWindowMin  *int   `json:"pre_window_min,omitempty" jsonschema:"pre-workout window in minutes (default 240, range 0..720)"`
	PostWindowMin *int   `json:"post_window_min,omitempty" jsonschema:"post-workout window in minutes (default 60, range 0..720)"`
}

func handleLogWorkout(ctx context.Context, c *apiClient, args LogWorkoutArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		ExternalID *string  `json:"external_id,omitempty"`
		Source     string   `json:"source"`
		Sport      string   `json:"sport"`
		Name       *string  `json:"name,omitempty"`
		StartedAt  string   `json:"started_at"`
		EndedAt    string   `json:"ended_at"`
		KcalBurned *float64 `json:"kcal_burned,omitempty"`
		AvgHR      *int     `json:"avg_hr,omitempty"`
		TSS        *float64 `json:"tss,omitempty"`
		Notes      *string  `json:"notes,omitempty"`
	}{
		ExternalID: args.ExternalID,
		Source:     args.Source,
		Sport:      args.Sport,
		Name:       args.Name,
		StartedAt:  args.StartedAt,
		EndedAt:    args.EndedAt,
		KcalBurned: args.KcalBurned,
		AvgHR:      args.AvgHR,
		TSS:        args.TSS,
		Notes:      args.Notes,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_workout", args)
	status, respBody, err := c.Post(ctx, "/workouts", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListWorkouts(ctx context.Context, c *apiClient, args ListWorkoutsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/workouts", q)
	return toToolResult(status, body, err)
}

func handleGetWorkout(ctx context.Context, c *apiClient, args GetWorkoutArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/workouts/"+url.PathEscape(args.ID), nil)
	return toToolResult(status, body, err)
}

func handlePatchWorkout(ctx context.Context, c *apiClient, args PatchWorkoutArgs) *mcp.CallToolResult {
	payload := map[string]any{}
	if args.Name != nil {
		payload["name"] = *args.Name
	}
	if args.Notes != nil {
		payload["notes"] = *args.Notes
	}
	if args.KcalBurned != nil {
		payload["kcal_burned"] = *args.KcalBurned
	}
	if args.AvgHR != nil {
		payload["avg_hr"] = *args.AvgHR
	}
	if args.TSS != nil {
		payload["tss"] = *args.TSS
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "patch_workout", args)
	status, respBody, err := c.Patch(ctx, "/workouts/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handleDeleteWorkout(ctx context.Context, c *apiClient, args DeleteWorkoutArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_workout", args)
	status, respBody, err := c.Delete(ctx, "/workouts/"+url.PathEscape(args.ID), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func handleWorkoutFuelingSummary(ctx context.Context, c *apiClient, args WorkoutFuelingSummaryArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.PreWindowMin != nil {
		q.Set("pre_window_min", strconv.Itoa(*args.PreWindowMin))
	}
	if args.PostWindowMin != nil {
		q.Set("post_window_min", strconv.Itoa(*args.PostWindowMin))
	}
	status, body, err := c.Get(ctx, "/workouts/"+url.PathEscape(args.WorkoutID)+"/fueling", q)
	return toToolResult(status, body, err)
}

func registerWorkoutsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_workout",
		Description: "Record a workout. Most workouts come from the Garmin importer with `source: garmin` " +
			"and an `external_id` (e.g. 'garmin:1234567') — that flow lives outside the agent. Use this tool " +
			"for MANUAL entries: gym sessions without a watch, sweat-rate test windows, untracked workouts the " +
			"user describes after the fact. For manual writes, leave `external_id` null — `external_id` is the " +
			"dedup mechanism; setting it on an agent-driven entry risks colliding with a future Garmin sync. " +
			"`tss` is the intensity signal; supply it if you know it, otherwise leave it null and downstream " +
			"tools will handle the gap.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleLogWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_workouts",
		Description: "List workouts whose started_at falls within the RFC 3339 window. Window is capped at 92 days. " +
			"Use this when answering 'what did I train this week?' or aggregating fueling-relevant workouts.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListWorkoutsArgs) (*mcp.CallToolResult, any, error) {
		return handleListWorkouts(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workout",
		Description: "Fetch a single workout by id. Returns the row with all stored fields.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGetWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "patch_workout",
		Description: "Adjust mutable fields on an existing workout. PATCH-able: `name`, `notes`, `kcal_burned`, " +
			"`avg_hr`, `tss`. IMMUTABLE (delete + re-create if these are wrong): `sport`, `started_at`, " +
			"`ended_at`, `source`, `external_id`. Typical uses: capture how the fueling went (`notes`), " +
			"supply a missing kcal estimate, correct TSS after an FTP change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PatchWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handlePatchWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_workout",
		Description: "Delete a workout. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "workout_fueling_summary",
		Description: "Return pre/intra/post fueling totals for a workout. Three time-anchored buckets " +
			"(pre, intra, post), each carrying THREE separate sub-objects: `nutrition` (kcal + macros + " +
			"nullable micros from meals), `hydration` (total_ml from hydration entries), and " +
			"`workout_fuel` (carbs/sodium/potassium/caffeine/ml from workout-fuel entries — gels, " +
			"electrolyte drinks, salt tabs, caffeine). Aggregation is by `logged_at` time-window matching, " +
			"NOT by the `workout_id` tag on intake rows — an untagged meal logged in the pre-window still " +
			"contributes. Defaults: pre_window_min=240 (4h), post_window_min=60. Both bounded [0, 720].",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args WorkoutFuelingSummaryArgs) (*mcp.CallToolResult, any, error) {
		return handleWorkoutFuelingSummary(ctx, c, args), nil, nil
	})
}
