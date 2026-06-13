package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GarminLoginArgs is intentionally empty: the bridge holds the credentials, so
// starting the login takes no arguments (design D3 of add-garmin-mcp-login).
type GarminLoginArgs struct{}

// GarminSubmitMFAArgs carries only the ephemeral 6-digit code — the single
// secret that ever transits the agent on this path (never the password/token).
type GarminSubmitMFAArgs struct {
	Code string `json:"code" jsonschema:"the 6-digit MFA code from the user's authenticator app or email"`
}

func handleGarminLogin(ctx context.Context, c *apiClient, _ GarminLoginArgs) *mcp.CallToolResult {
	// One HTTP call, no body, no idempotency key: starting an interactive login
	// is not a replayable write.
	status, body, err := c.Post(ctx, "/garmin/login", nil, nil, "")
	return toToolResult(status, body, err)
}

func handleGarminSubmitMFA(ctx context.Context, c *apiClient, args GarminSubmitMFAArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Code string `json:"code"`
	}{Code: args.Code})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	status, respBody, err := c.Post(ctx, "/garmin/login/mfa", nil, body, "")
	return toToolResult(status, respBody, err)
}

// ----- scheduling (push the plan to the watch) -----

type GarminScheduleWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"a planned workout's UUID (must have a template)"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminUnscheduleWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"the workout UUID to remove from the Garmin calendar"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type GarminSchedulePlanArgs struct {
	PlanID         string  `json:"plan_id" jsonschema:"the training-plan UUID"`
	Scope          string  `json:"scope" jsonschema:"all, week, or range"`
	Week           *int    `json:"week,omitempty" jsonschema:"required when scope is week: the week ordinal"`
	From           *string `json:"from,omitempty" jsonschema:"required when scope is range: inclusive start YYYY-MM-DD"`
	To             *string `json:"to,omitempty" jsonschema:"required when scope is range: inclusive end YYYY-MM-DD"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type GarminListScheduledArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound YYYY-MM-DD"`
}

func handleGarminScheduleWorkout(ctx context.Context, c *apiClient, args GarminScheduleWorkoutArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		WorkoutID string `json:"workout_id"`
	}{args.WorkoutID})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_schedule_workout", args)
	status, resp, err := c.Post(ctx, "/garmin/schedule/workout", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminUnscheduleWorkout(ctx context.Context, c *apiClient, args GarminUnscheduleWorkoutArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_unschedule_workout", args)
	status, resp, err := c.Delete(ctx, "/garmin/schedule/workout/"+url.PathEscape(args.WorkoutID), key)
	return toToolResult(status, resp, err)
}

func handleGarminSchedulePlan(ctx context.Context, c *apiClient, args GarminSchedulePlanArgs) *mcp.CallToolResult {
	body, err := json.Marshal(map[string]any{"plan_id": args.PlanID, "scope": args.Scope, "week": args.Week, "from": args.From, "to": args.To})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_schedule_plan", args)
	status, resp, err := c.Post(ctx, "/garmin/schedule/plan", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminListScheduled(ctx context.Context, c *apiClient, args GarminListScheduledArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, resp, err := c.Get(ctx, "/garmin/calendar", q)
	return toToolResult(status, resp, err)
}

// ----- workout-library management + blob export (garmin-workout-library-mgmt) -----

type GarminDeleteWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"the backend workout UUID whose Garmin library object should be deleted"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminListWorkoutsArgs struct {
	Start *int `json:"start,omitempty" jsonschema:"optional pagination start offset"`
	Limit *int `json:"limit,omitempty" jsonschema:"optional pagination limit"`
}

type GarminGetWorkoutArgs struct {
	GarminWorkoutID string `json:"garmin_workout_id" jsonschema:"the Garmin workout object id to fetch from the library"`
}

type GarminPushHydrationArgs struct {
	ValueML        float64 `json:"value_ml" jsonschema:"the day's TOTAL hydration in millilitres (sets/replaces the day on Garmin, not a delta)"`
	Date           string  `json:"date" jsonschema:"the calendar day YYYY-MM-DD to set hydration for"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminExportActivityArgs struct {
	ActivityID string `json:"activity_id" jsonschema:"the Garmin activity id to export"`
	Format     string `json:"format,omitempty" jsonschema:"fit (default) | gpx | tcx | kml | csv"`
}

func handleGarminDeleteWorkout(ctx context.Context, c *apiClient, args GarminDeleteWorkoutArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_delete_workout", args)
	status, resp, err := c.Delete(ctx, "/garmin/workout/"+url.PathEscape(args.WorkoutID), key)
	return toToolResult(status, resp, err)
}

func handleGarminListWorkouts(ctx context.Context, c *apiClient, args GarminListWorkoutsArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Start != nil {
		q.Set("start", strconv.Itoa(*args.Start))
	}
	if args.Limit != nil {
		q.Set("limit", strconv.Itoa(*args.Limit))
	}
	status, resp, err := c.Get(ctx, "/garmin/workouts", q)
	return toToolResult(status, resp, err)
}

func handleGarminGetWorkout(ctx context.Context, c *apiClient, args GarminGetWorkoutArgs) *mcp.CallToolResult {
	status, resp, err := c.Get(ctx, "/garmin/workout/"+url.PathEscape(args.GarminWorkoutID), nil)
	return toToolResult(status, resp, err)
}

func handleGarminPushHydration(ctx context.Context, c *apiClient, args GarminPushHydrationArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		ValueML float64 `json:"value_ml"`
		Date    string  `json:"date"`
	}{args.ValueML, args.Date})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_push_hydration", args)
	status, resp, err := c.Post(ctx, "/garmin/hydration", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminExportActivity(ctx context.Context, c *apiClient, args GarminExportActivityArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Format != "" {
		q.Set("format", args.Format)
	}
	status, resp, err := c.Get(ctx, "/garmin/activity/"+url.PathEscape(args.ActivityID)+"/export", q)
	return toToolResult(status, resp, err)
}

// ----- activity-level control operations (add-garmin-misc-mirror) -----

type GarminGetActivityGearArgs struct {
	ActivityID string `json:"activity_id" jsonschema:"the Garmin activity id whose linked gear to read"`
}

type GarminDownloadWorkoutArgs struct {
	GarminWorkoutID string `json:"garmin_workout_id" jsonschema:"the Garmin workout object id to download"`
	Format          string `json:"format,omitempty" jsonschema:"fit (default) | …"`
}

type GarminUploadActivityArgs struct {
	Filename       string `json:"filename" jsonschema:"the FIT file name (e.g. ride.fit)"`
	ContentBase64  string `json:"content_base64" jsonschema:"the FIT file bytes, base64-encoded"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminRenameActivityArgs struct {
	ActivityID     string `json:"activity_id" jsonschema:"the Garmin activity id to rename"`
	Name           string `json:"name" jsonschema:"the new activity name"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminDeleteActivityArgs struct {
	ActivityID     string `json:"activity_id" jsonschema:"the Garmin activity id to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminBackfillArgs struct {
	From           string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD (oldest day to re-sync)"`
	To             string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

func handleGarminBackfill(ctx context.Context, c *apiClient, args GarminBackfillArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		From string `json:"from"`
		To   string `json:"to"`
	}{args.From, args.To})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_backfill", args)
	status, resp, err := c.Post(ctx, "/garmin/backfill", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminGetActivityGear(ctx context.Context, c *apiClient, args GarminGetActivityGearArgs) *mcp.CallToolResult {
	status, resp, err := c.Get(ctx, "/garmin/activity/"+url.PathEscape(args.ActivityID)+"/gear", nil)
	return toToolResult(status, resp, err)
}

func handleGarminDownloadWorkout(ctx context.Context, c *apiClient, args GarminDownloadWorkoutArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Format != "" {
		q.Set("format", args.Format)
	}
	status, resp, err := c.Get(ctx, "/garmin/workout/"+url.PathEscape(args.GarminWorkoutID)+"/download", q)
	return toToolResult(status, resp, err)
}

func handleGarminUploadActivity(ctx context.Context, c *apiClient, args GarminUploadActivityArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Filename      string `json:"filename"`
		ContentBase64 string `json:"content_base64"`
	}{args.Filename, args.ContentBase64})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_upload_activity", args)
	status, resp, err := c.Post(ctx, "/garmin/activity/upload", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminRenameActivity(ctx context.Context, c *apiClient, args GarminRenameActivityArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name string `json:"name"`
	}{args.Name})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_rename_activity", args)
	status, resp, err := c.Patch(ctx, "/garmin/activity/"+url.PathEscape(args.ActivityID), body, key)
	return toToolResult(status, resp, err)
}

func handleGarminDeleteActivity(ctx context.Context, c *apiClient, args GarminDeleteActivityArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_delete_activity", args)
	status, resp, err := c.Delete(ctx, "/garmin/activity/"+url.PathEscape(args.ActivityID), key)
	return toToolResult(status, resp, err)
}

func registerGarminTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_login",
		Description: "Start re-linking the user's Garmin account (renews the ~yearly-expiring Garmin token). " +
			"Takes no arguments — the bridge holds the credentials. If the result is `{\"needs_mfa\": true}`, " +
			"ask the user for the 6-digit code from their authenticator app, then call `garmin_submit_mfa` with it. " +
			"A `{\"logged_in\": true}` result means no code was needed and re-linking is already complete. " +
			"A `503 garmin_disabled` result means the Garmin integration is not configured on this server.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminLoginArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminLogin(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_submit_mfa",
		Description: "Complete a Garmin re-link by submitting the 6-digit MFA code the user read from their " +
			"authenticator. Call this only after `garmin_login` returned `{\"needs_mfa\": true}`. A " +
			"`{\"logged_in\": true}` result means the token was renewed; an error (e.g. `mfa_invalid`) means the " +
			"code was wrong or expired — call `garmin_login` again to restart.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminSubmitMFAArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminSubmitMFA(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_schedule_workout",
		Description: "Push one planned workout to the Garmin watch: compiles its template into a structured Garmin " +
			"workout, schedules it on the workout's date, and stores the Garmin ids. Re-pushing replaces the prior " +
			"calendar entry. The workout must be planned and have a template. 503 garmin_disabled when the bridge is off.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminScheduleWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminScheduleWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_unschedule_workout",
		Description: "Remove a workout from the Garmin calendar AND delete its structured workout object from the " +
			"library (the full teardown — closes the library-orphan leak), then clear its stored Garmin ids. No-op " +
			"success if it was never scheduled. To delete only a stray library object (without touching a calendar " +
			"entry), use garmin_delete_workout instead.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminUnscheduleWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminUnscheduleWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_schedule_plan",
		Description: "Push every planned workout in a plan scope (all, week, or range) to the watch in one call. " +
			"Per-workout failures are reported alongside successes, not fatal.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminSchedulePlanArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminSchedulePlan(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "garmin_list_scheduled",
		Description: "List the workouts scheduled on the Garmin calendar in a date range, for reconciliation.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminListScheduledArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminListScheduled(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_delete_workout",
		Description: "Delete a workout's structured object from the Garmin library (reconciliation cleanup for an " +
			"orphan you found). Clears the stored garmin_workout_id but leaves any calendar entry — use " +
			"garmin_unschedule_workout for the full teardown. Idempotent: success even if already gone.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminDeleteWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminDeleteWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_list_workouts",
		Description: "List the structured workouts in the Garmin library (optional start/limit pagination). Use to " +
			"reconcile what's actually on the watch against what the plan thinks is scheduled.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminListWorkoutsArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminListWorkouts(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "garmin_get_workout",
		Description: "Fetch one structured workout from the Garmin library by its Garmin workout object id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminGetWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminGetWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_push_hydration",
		Description: "Push logged hydration back TO Garmin for a date — the ONLY write from this system to Garmin, " +
			"and opt-in: invoke it deliberately (e.g. \"sync today's water to my watch\"); nothing pushes " +
			"automatically. value_ml is the day's TOTAL (Garmin sets/replaces the day, it does not append), so read " +
			"the day's total from the hydration summary first and pass that.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminPushHydrationArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminPushHydration(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_export_activity",
		Description: "Export an activity's file (FIT by default; gpx/tcx/kml/csv) as a base64 blob inside a JSON " +
			"envelope {activity_id, format, filename, content_base64}. Decode content_base64 to save the file. " +
			"Read-only; upload is not supported.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminExportActivityArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminExportActivity(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "garmin_get_activity_gear",
		Description: "Read the gear (shoes/bike) Garmin has linked to a specific activity.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminGetActivityGearArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminGetActivityGear(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_download_workout",
		Description: "Download a structured workout's file (FIT by default) as a base64 blob inside a JSON envelope " +
			"{garmin_workout_id, format, filename, content_base64}. The structured-workout analogue of " +
			"garmin_export_activity. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminDownloadWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminDownloadWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_upload_activity",
		Description: "Upload a FIT activity file TO Garmin (base64-encoded). A write from this system to Garmin — " +
			"invoke only on explicit request; nothing uploads automatically.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminUploadActivityArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminUploadActivity(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "garmin_rename_activity",
		Description: "Rename a Garmin activity (e.g. set a descriptive title like \"Evening Z2 ride\").",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminRenameActivityArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminRenameActivity(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_delete_activity",
		Description: "Delete a Garmin activity. Idempotent — an already-absent activity is a no-op success. A " +
			"destructive write; invoke only on explicit request.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminDeleteActivityArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminDeleteActivity(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_backfill",
		Description: "Backfill the Garmin sync over a historical date range [from, to] (YYYY-MM-DD), re-syncing each " +
			"day so older activities gain the detail the rolling daily window missed. Bounded (a max-days cap), " +
			"paced (a delay between days), oldest-first and resumable, and idempotent — re-running a range is safe. " +
			"Returns a per-day summary plus a roll-up (days_total/days_ok/days_failed); 207 if some days failed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminBackfillArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminBackfill(ctx, c, args), nil, nil
	})
}
