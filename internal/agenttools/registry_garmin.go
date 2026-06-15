package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Garmin desktop-coach tools — login/MFA, watch scheduling, workout-library
// management, blob export/download (base64-in-JSON, NOT multipart at this
// layer), activity control, and history backfill. Ported from internal/mcpserver
// onto the shared registry (unify-mcp-tool-registry). Arg structs and
// descriptions are byte-identical to the prior bespoke registrations so the
// announced schema is unchanged; Build funcs reproduce the REST mapping exactly.
//
// garmin_login and garmin_submit_mfa set OmitIdempotencyKey: the bespoke
// handlers dispatched them with an empty key because an interactive login is
// not a replayable write — re-login after a failed MFA must restart, not replay
// a stored response.

func init() { registerMCPDomain(garminSpecs()) }

// ----- login (add-garmin-mcp-login) -----

type GarminLoginArgs struct{}

type GarminSubmitMFAArgs struct {
	Code string `json:"code" jsonschema:"the 6-digit MFA code from the user's authenticator app or email"`
}

// ----- scheduling (push the plan to the watch) -----

type GarminScheduleWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"a planned workout's UUID (must have a template)"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminScheduleTemplateArgs struct {
	TemplateID     string `json:"template_id" jsonschema:"a workout template's UUID to schedule as a standalone session (e.g. yoga/mobility)"`
	Date           string `json:"date" jsonschema:"the calendar day YYYY-MM-DD to schedule it on"`
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

func garminSpecs() []Spec {
	return []Spec{
		{
			Name: "garmin_login",
			Description: "Start re-linking the user's Garmin account (renews the ~yearly-expiring Garmin token). " +
				"Takes no arguments — the bridge holds the credentials. If the result is `{\"needs_mfa\": true}`, " +
				"ask the user for the 6-digit code from their authenticator app, then call `garmin_submit_mfa` with it. " +
				"A `{\"logged_in\": true}` result means no code was needed and re-linking is already complete. " +
				"A `503 garmin_disabled` result means the Garmin integration is not configured on this server.",
			SchemaType:         GarminLoginArgs{},
			Tier:               TierWriteAuto,
			OmitIdempotencyKey: true, // interactive login is not a replayable write
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "POST", Path: "/garmin/login"}, nil
			},
		},
		{
			Name: "garmin_submit_mfa",
			Description: "Complete a Garmin re-link by submitting the 6-digit MFA code the user read from their " +
				"authenticator. Call this only after `garmin_login` returned `{\"needs_mfa\": true}`. A " +
				"`{\"logged_in\": true}` result means the token was renewed; an error (e.g. `mfa_invalid`) means the " +
				"code was wrong or expired — call `garmin_login` again to restart.",
			SchemaType:         GarminSubmitMFAArgs{},
			Tier:               TierWriteAuto,
			OmitIdempotencyKey: true, // single-use code; re-submit must not replay
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminSubmitMFAArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					Code string `json:"code"`
				}{a.Code})
				return HTTPCall{Method: "POST", Path: "/garmin/login/mfa", Body: body}, nil
			},
		},
		{
			Name: "garmin_schedule_workout",
			Description: "Push one planned workout to the Garmin watch: compiles its template into a structured Garmin " +
				"workout, schedules it on the workout's date, and stores the Garmin ids. Re-pushing replaces the prior " +
				"calendar entry. The workout must be planned and have a template. 503 garmin_disabled when the bridge is off.",
			SchemaType: GarminScheduleWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminScheduleWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					WorkoutID string `json:"workout_id"`
				}{a.WorkoutID})
				return HTTPCall{Method: "POST", Path: "/garmin/schedule/workout", Body: body}, nil
			},
		},
		{
			Name: "garmin_schedule_template",
			Description: "Schedule a standalone workout template to a date on the Garmin watch in one call — the path for " +
				"ad-hoc sessions that aren't part of a materialized plan (e.g. yoga, mobility). Creates an ad-hoc planned " +
				"workout from the template (its sport is preserved), compiles and schedules it on the date, and stores the " +
				"Garmin ids. Unschedule it later with garmin_unschedule_workout on the returned workout id. " +
				"503 garmin_disabled when the bridge is off.",
			SchemaType: GarminScheduleTemplateArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminScheduleTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					TemplateID string `json:"template_id"`
					Date       string `json:"date"`
				}{a.TemplateID, a.Date})
				return HTTPCall{Method: "POST", Path: "/garmin/schedule/template", Body: body}, nil
			},
		},
		{
			Name: "garmin_unschedule_workout",
			Description: "Remove a workout from the Garmin calendar AND delete its structured workout object from the " +
				"library (the full teardown — closes the library-orphan leak), then clear its stored Garmin ids. No-op " +
				"success if it was never scheduled. To delete only a stray library object (without touching a calendar " +
				"entry), use garmin_delete_workout instead.",
			SchemaType: GarminUnscheduleWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminUnscheduleWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/garmin/schedule/workout/" + url.PathEscape(a.WorkoutID)}, nil
			},
		},
		{
			Name: "garmin_schedule_plan",
			Description: "Push every planned workout in a plan scope (all, week, or range) to the watch in one call. " +
				"Per-workout failures are reported alongside successes, not fatal.",
			SchemaType: GarminSchedulePlanArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminSchedulePlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(map[string]any{"plan_id": a.PlanID, "scope": a.Scope, "week": a.Week, "from": a.From, "to": a.To})
				return HTTPCall{Method: "POST", Path: "/garmin/schedule/plan", Body: body}, nil
			},
		},
		{
			Name:        "garmin_list_scheduled",
			Description: "List the workouts scheduled on the Garmin calendar in a date range, for reconciliation.",
			SchemaType:  GarminListScheduledArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminListScheduledArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/garmin/calendar", Query: q}, nil
			},
		},
		{
			Name: "garmin_delete_workout",
			Description: "Delete a workout's structured object from the Garmin library (reconciliation cleanup for an " +
				"orphan you found). Clears the stored garmin_workout_id but leaves any calendar entry — use " +
				"garmin_unschedule_workout for the full teardown. Idempotent: success even if already gone.",
			SchemaType: GarminDeleteWorkoutArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminDeleteWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/garmin/workout/" + url.PathEscape(a.WorkoutID)}, nil
			},
		},
		{
			Name: "garmin_list_workouts",
			Description: "List the structured workouts in the Garmin library (optional start/limit pagination). Use to " +
				"reconcile what's actually on the watch against what the plan thinks is scheduled.",
			SchemaType: GarminListWorkoutsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminListWorkoutsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Start != nil {
					q.Set("start", strconv.Itoa(*a.Start))
				}
				if a.Limit != nil {
					q.Set("limit", strconv.Itoa(*a.Limit))
				}
				return HTTPCall{Method: "GET", Path: "/garmin/workouts", Query: q}, nil
			},
		},
		{
			Name:        "garmin_get_workout",
			Description: "Fetch one structured workout from the Garmin library by its Garmin workout object id.",
			SchemaType:  GarminGetWorkoutArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminGetWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/garmin/workout/" + url.PathEscape(a.GarminWorkoutID)}, nil
			},
		},
		{
			Name: "garmin_push_hydration",
			Description: "Push logged hydration back TO Garmin for a date — the ONLY write from this system to Garmin, " +
				"and opt-in: invoke it deliberately (e.g. \"sync today's water to my watch\"); nothing pushes " +
				"automatically. value_ml is the day's TOTAL (Garmin sets/replaces the day, it does not append), so read " +
				"the day's total from the hydration summary first and pass that.",
			SchemaType: GarminPushHydrationArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminPushHydrationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					ValueML float64 `json:"value_ml"`
					Date    string  `json:"date"`
				}{a.ValueML, a.Date})
				return HTTPCall{Method: "POST", Path: "/garmin/hydration", Body: body}, nil
			},
		},
		{
			Name: "garmin_export_activity",
			Description: "Export an activity's file (FIT by default; gpx/tcx/kml/csv) as a base64 blob inside a JSON " +
				"envelope {activity_id, format, filename, content_base64}. Decode content_base64 to save the file. " +
				"Read-only; upload is not supported.",
			SchemaType: GarminExportActivityArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminExportActivityArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Format != "" {
					q.Set("format", a.Format)
				}
				return HTTPCall{Method: "GET", Path: "/garmin/activity/" + url.PathEscape(a.ActivityID) + "/export", Query: q}, nil
			},
		},
		{
			Name:        "garmin_get_activity_gear",
			Description: "Read the gear (shoes/bike) Garmin has linked to a specific activity.",
			SchemaType:  GarminGetActivityGearArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminGetActivityGearArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/garmin/activity/" + url.PathEscape(a.ActivityID) + "/gear"}, nil
			},
		},
		{
			Name: "garmin_download_workout",
			Description: "Download a structured workout's file (FIT by default) as a base64 blob inside a JSON envelope " +
				"{garmin_workout_id, format, filename, content_base64}. The structured-workout analogue of " +
				"garmin_export_activity. Read-only.",
			SchemaType: GarminDownloadWorkoutArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminDownloadWorkoutArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Format != "" {
					q.Set("format", a.Format)
				}
				return HTTPCall{Method: "GET", Path: "/garmin/workout/" + url.PathEscape(a.GarminWorkoutID) + "/download", Query: q}, nil
			},
		},
		{
			Name: "garmin_upload_activity",
			Description: "Upload a FIT activity file TO Garmin (base64-encoded). A write from this system to Garmin — " +
				"invoke only on explicit request; nothing uploads automatically.",
			SchemaType: GarminUploadActivityArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminUploadActivityArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					Filename      string `json:"filename"`
					ContentBase64 string `json:"content_base64"`
				}{a.Filename, a.ContentBase64})
				return HTTPCall{Method: "POST", Path: "/garmin/activity/upload", Body: body}, nil
			},
		},
		{
			Name:        "garmin_rename_activity",
			Description: "Rename a Garmin activity (e.g. set a descriptive title like \"Evening Z2 ride\").",
			SchemaType:  GarminRenameActivityArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminRenameActivityArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					Name string `json:"name"`
				}{a.Name})
				return HTTPCall{Method: "PATCH", Path: "/garmin/activity/" + url.PathEscape(a.ActivityID), Body: body}, nil
			},
		},
		{
			Name: "garmin_delete_activity",
			Description: "Delete a Garmin activity. Idempotent — an already-absent activity is a no-op success. A " +
				"destructive write; invoke only on explicit request.",
			SchemaType: GarminDeleteActivityArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminDeleteActivityArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/garmin/activity/" + url.PathEscape(a.ActivityID)}, nil
			},
		},
		{
			Name: "garmin_backfill",
			Description: "Backfill the Garmin sync over a historical date range [from, to] (YYYY-MM-DD), re-syncing each " +
				"day so older activities gain the detail the rolling daily window missed. Bounded (a max-days cap), " +
				"paced (a delay between days), oldest-first and resumable, and idempotent — re-running a range is safe. " +
				"Returns a per-day summary plus a roll-up (days_total/days_ok/days_failed); 207 if some days failed.",
			SchemaType: GarminBackfillArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GarminBackfillArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, _ := json.Marshal(struct {
					From string `json:"from"`
					To   string `json:"to"`
				}{a.From, a.To})
				return HTTPCall{Method: "POST", Path: "/garmin/backfill", Body: body}, nil
			},
		},
	}
}
