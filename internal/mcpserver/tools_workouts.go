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
	ExternalID      *string  `json:"external_id,omitempty" jsonschema:"writer-supplied dedup key, e.g. 'garmin:1234567'. Garmin and other sourced writers SHOULD set this. For agent-driven manual entries, leave it unset."`
	Source          string   `json:"source" jsonschema:"provenance: 'garmin' | 'manual' | 'other'. Use 'manual' for agent-driven entries."`
	Sport           string   `json:"sport" jsonschema:"'run' | 'bike' | 'swim' | 'strength' | 'other'"`
	Status          string   `json:"status,omitempty" jsonschema:"'completed' (default) for a session that happened, or 'planned' for a scheduled future session. Planned workouts may be future-dated (up to 1 year); completed ones cannot be more than 24h ahead."`
	Name            *string  `json:"name,omitempty" jsonschema:"optional human-readable label, e.g. 'Morning Z2 ride'"`
	StartedAt       string   `json:"started_at" jsonschema:"RFC 3339 timestamp the workout started"`
	EndedAt         string   `json:"ended_at" jsonschema:"RFC 3339 timestamp the workout ended; must be after started_at"`
	KcalBurned      *float64 `json:"kcal_burned,omitempty" jsonschema:"calories burned during the session; positive number"`
	AvgHR           *int     `json:"avg_hr,omitempty" jsonschema:"average heart rate in bpm; positive integer"`
	TSS             *float64 `json:"tss,omitempty" jsonschema:"Training Stress Score; the intensity signal. Non-negative."`
	RPE             *int     `json:"rpe,omitempty" jsonschema:"Borg CR-10 perceived effort, integer 1..10. Per session — logged after the workout. Nullable; omit for non-rehearsal rides (Z1 spins, gym sessions, etc.)."`
	GIDistressScore *int     `json:"gi_distress_score,omitempty" jsonschema:"GI distress severity, integer 1..5 (1 = no distress, 5 = severe / couldn't continue). Per session — the rehearsal-outcome signal that lets you iterate fueling strategy. Nullable."`
	DistanceM       *float64 `json:"distance_m,omitempty" jsonschema:"session distance in METRES (> 0). Nullable; omit if the source did not measure it."`
	AvgPowerW       *int     `json:"avg_power_w,omitempty" jsonschema:"average power in WATTS (positive integer). Nullable."`
	TemperatureC    *float64 `json:"temperature_c,omitempty" jsonschema:"ambient temperature in °C, range -40..60. Heat context for fueling. Nullable."`
	SweatLossML     *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"estimated sweat loss in MILLILITRES (> 0). Personalises fluid targets. Nullable."`
	SessionGroup    *string  `json:"session_group,omitempty" jsonschema:"free-text key linking the legs of a brick/multisport session — set the SAME key on every leg (e.g. the Garmin parent activity id) so they can be fetched together. Nullable."`
	Notes           *string  `json:"notes,omitempty" jsonschema:"free-text notes (e.g. how the fueling went, which product caused issues at minute N)"`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. Note: writers normally rely on external_id for dedup, not this header."`
}

type ListWorkoutsArgs struct {
	From         string  `json:"from" jsonschema:"inclusive RFC 3339 lower bound on started_at"`
	To           string  `json:"to" jsonschema:"inclusive RFC 3339 upper bound on started_at; max 92 days from 'from'"`
	SessionGroup *string `json:"session_group,omitempty" jsonschema:"optional: narrow to the legs of one brick/multisport session (exact-match on the session_group key). The from/to window is still required."`
	Status       *string `json:"status,omitempty" jsonschema:"optional: filter by lifecycle status 'planned' or 'completed'. The from/to window is still required."`
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
	// RPE + GI use json.RawMessage so the wrapper can encode three states:
	// absent (leave unchanged), integer (set), JSON null (clear to NULL).
	// Use the helper fields RPE/GIDistressScore for set, ClearRPE/ClearGI for null-clear.
	RPE                  *int `json:"rpe,omitempty" jsonschema:"Borg CR-10 perceived effort 1..10. Per session, set after the workout. Omit to leave unchanged."`
	ClearRPE             bool `json:"clear_rpe,omitempty" jsonschema:"set true to clear rpe to null (retract a previously logged value). Mutually exclusive with rpe."`
	GIDistressScore      *int `json:"gi_distress_score,omitempty" jsonschema:"GI distress severity 1..5. Per session. Omit to leave unchanged."`
	ClearGIDistressScore bool `json:"clear_gi_distress_score,omitempty" jsonschema:"set true to clear gi_distress_score to null. Mutually exclusive with gi_distress_score."`

	// Ingestion metrics — same tri-state: value sets, Clear* clears to null, omit leaves unchanged.
	DistanceM         *float64 `json:"distance_m,omitempty" jsonschema:"corrected distance in METRES (> 0). Omit to leave unchanged."`
	ClearDistanceM    bool     `json:"clear_distance_m,omitempty" jsonschema:"set true to clear distance_m to null."`
	AvgPowerW         *int     `json:"avg_power_w,omitempty" jsonschema:"corrected average power in WATTS (positive integer). Omit to leave unchanged."`
	ClearAvgPowerW    bool     `json:"clear_avg_power_w,omitempty" jsonschema:"set true to clear avg_power_w to null."`
	TemperatureC      *float64 `json:"temperature_c,omitempty" jsonschema:"corrected ambient temperature in °C (-40..60). Omit to leave unchanged."`
	ClearTemperatureC bool     `json:"clear_temperature_c,omitempty" jsonschema:"set true to clear temperature_c to null."`
	SweatLossML       *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"corrected estimated sweat loss in MILLILITRES (> 0). Omit to leave unchanged."`
	ClearSweatLossML  bool     `json:"clear_sweat_loss_ml,omitempty" jsonschema:"set true to clear sweat_loss_ml to null."`
	SessionGroup      *string  `json:"session_group,omitempty" jsonschema:"set the brick/multisport group key. Omit to leave unchanged."`
	ClearSessionGroup bool     `json:"clear_session_group,omitempty" jsonschema:"set true to un-group this leg (clear session_group to null)."`
	Status            *string  `json:"status,omitempty" jsonschema:"set the lifecycle status: 'planned' or 'completed'. Typical use: promote a planned session to completed once it happens. Omit to leave unchanged."`

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

type FulfillWorkoutArgs struct {
	PlannedID      string `json:"planned_id" jsonschema:"the PLANNED workout id to fulfill (the merge target that holds the plan_slot_id)"`
	CompletedID    string `json:"completed_id" jsonschema:"the standalone COMPLETED activity id to merge into the planned workout"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type UnfulfillWorkoutArgs struct {
	ID             string `json:"id" jsonschema:"the fulfilled workout id to revert back to planned"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

func handleLogWorkout(ctx context.Context, c *apiClient, args LogWorkoutArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		ExternalID      *string  `json:"external_id,omitempty"`
		Source          string   `json:"source"`
		Sport           string   `json:"sport"`
		Status          string   `json:"status,omitempty"`
		Name            *string  `json:"name,omitempty"`
		StartedAt       string   `json:"started_at"`
		EndedAt         string   `json:"ended_at"`
		KcalBurned      *float64 `json:"kcal_burned,omitempty"`
		AvgHR           *int     `json:"avg_hr,omitempty"`
		TSS             *float64 `json:"tss,omitempty"`
		RPE             *int     `json:"rpe,omitempty"`
		GIDistressScore *int     `json:"gi_distress_score,omitempty"`
		DistanceM       *float64 `json:"distance_m,omitempty"`
		AvgPowerW       *int     `json:"avg_power_w,omitempty"`
		TemperatureC    *float64 `json:"temperature_c,omitempty"`
		SweatLossML     *float64 `json:"sweat_loss_ml,omitempty"`
		SessionGroup    *string  `json:"session_group,omitempty"`
		Notes           *string  `json:"notes,omitempty"`
	}{
		ExternalID:      args.ExternalID,
		Source:          args.Source,
		Sport:           args.Sport,
		Status:          args.Status,
		Name:            args.Name,
		StartedAt:       args.StartedAt,
		EndedAt:         args.EndedAt,
		KcalBurned:      args.KcalBurned,
		AvgHR:           args.AvgHR,
		TSS:             args.TSS,
		RPE:             args.RPE,
		GIDistressScore: args.GIDistressScore,
		DistanceM:       args.DistanceM,
		AvgPowerW:       args.AvgPowerW,
		TemperatureC:    args.TemperatureC,
		SweatLossML:     args.SweatLossML,
		SessionGroup:    args.SessionGroup,
		Notes:           args.Notes,
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
	if args.SessionGroup != nil {
		q.Set("session_group", *args.SessionGroup)
	}
	if args.Status != nil {
		q.Set("status", *args.Status)
	}
	statusCode, body, err := c.Get(ctx, "/workouts", q)
	return toToolResult(statusCode, body, err)
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
	// RPE tri-state: ClearRPE wins (encodes as JSON null → backend clears);
	// otherwise integer if set; otherwise field absent from payload.
	if args.ClearRPE {
		payload["rpe"] = nil
	} else if args.RPE != nil {
		payload["rpe"] = *args.RPE
	}
	if args.ClearGIDistressScore {
		payload["gi_distress_score"] = nil
	} else if args.GIDistressScore != nil {
		payload["gi_distress_score"] = *args.GIDistressScore
	}
	// Ingestion-metric tri-state: Clear* wins (JSON null → backend clears),
	// otherwise value if set, otherwise field absent from payload.
	if args.ClearDistanceM {
		payload["distance_m"] = nil
	} else if args.DistanceM != nil {
		payload["distance_m"] = *args.DistanceM
	}
	if args.ClearAvgPowerW {
		payload["avg_power_w"] = nil
	} else if args.AvgPowerW != nil {
		payload["avg_power_w"] = *args.AvgPowerW
	}
	if args.ClearTemperatureC {
		payload["temperature_c"] = nil
	} else if args.TemperatureC != nil {
		payload["temperature_c"] = *args.TemperatureC
	}
	if args.ClearSweatLossML {
		payload["sweat_loss_ml"] = nil
	} else if args.SweatLossML != nil {
		payload["sweat_loss_ml"] = *args.SweatLossML
	}
	if args.ClearSessionGroup {
		payload["session_group"] = nil
	} else if args.SessionGroup != nil {
		payload["session_group"] = *args.SessionGroup
	}
	if args.Status != nil {
		payload["status"] = *args.Status
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

func handleFulfillWorkout(ctx context.Context, c *apiClient, args FulfillWorkoutArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		CompletedID string `json:"completed_id"`
	}{CompletedID: args.CompletedID})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "fulfill_workout", args)
	status, respBody, err := c.Post(ctx, "/workouts/"+url.PathEscape(args.PlannedID)+"/fulfill", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleUnfulfillWorkout(ctx context.Context, c *apiClient, args UnfulfillWorkoutArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "unfulfill_workout", args)
	status, respBody, err := c.Post(ctx, "/workouts/"+url.PathEscape(args.ID)+"/unfulfill", nil, nil, key)
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
			"tools will handle the gap. " +
			"`rpe` and `gi_distress_score` are the rehearsal-outcome signals — log them after fueling-rehearsal " +
			"workouts (e.g. long Z2 rides in race-prep blocks). RPE is Borg CR-10 perceived effort, integer 1..10. " +
			"GI distress is 1=no distress through 5=severe / had to stop. Both are nullable and only meaningful for " +
			"sessions you're actively iterating fueling on; skip for everyday Z1 spins and gym work. " +
			"Ingestion metrics (all nullable, units fixed): `distance_m` metres, `avg_power_w` watts, " +
			"`temperature_c` °C, `sweat_loss_ml` millilitres. `session_group` links the legs of a brick/multisport " +
			"session — set the SAME key on every leg (e.g. the Garmin parent activity id) so they can be fetched " +
			"together; omit for single-sport sessions.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleLogWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_workouts",
		Description: "List workouts whose started_at falls within the RFC 3339 window. Window is capped at 92 days. " +
			"Use this when answering 'what did I train this week?' or aggregating fueling-relevant workouts. " +
			"Pass `session_group` to fetch just the legs of one brick/multisport session (the window stays required).",
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
			"`avg_hr`, `tss`, `rpe`, `gi_distress_score`, `distance_m`, `avg_power_w`, `temperature_c`, " +
			"`sweat_loss_ml`, `session_group`. IMMUTABLE (delete + re-create if these are wrong): " +
			"`sport`, `started_at`, `ended_at`, `source`, `external_id`. " +
			"Typical post-ride flow: set `rpe` (Borg CR-10 perceived effort, 1..10) and `gi_distress_score` " +
			"(1=no distress, 5=severe) on the workout you just rehearsed fueling on. Other typical uses: " +
			"capture how the fueling went in `notes`, supply a missing kcal estimate, correct TSS after an FTP " +
			"change, or un-group a mis-linked brick leg (`clear_session_group: true`). " +
			"Every nullable field is tri-state: omit to leave unchanged, set a value to overwrite, OR set the " +
			"matching `clear_*` flag to retract a value (clears to null on the backend).",
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
		Name: "fulfill_workout",
		Description: "Manually merge a completed activity into a planned workout — the explicit version of the " +
			"automatic reconciliation that runs on Garmin sync. Use this for cases the auto-match declined: a session " +
			"done on a different calendar day than planned, or a day with two same-sport planned workouts where the " +
			"import was flagged `needs_link`. Pass `planned_id` (the scheduled session, which holds the plan_slot_id) " +
			"and `completed_id` (the standalone imported activity). The planned row survives with the activity's " +
			"actuals and external_id and flips to completed; the standalone row is removed. Both must be the same sport.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args FulfillWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleFulfillWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "unfulfill_workout",
		Description: "Reverse a fulfilled workout back to planned — undo a wrong merge (auto or manual). Clears the " +
			"workout's external_id and actual metrics and restores status=planned, keeping its template_id and " +
			"plan_slot_id. The activity is not re-fetched; the next Garmin sync re-imports it as a fresh standalone row.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UnfulfillWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleUnfulfillWorkout(ctx, c, args), nil, nil
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
