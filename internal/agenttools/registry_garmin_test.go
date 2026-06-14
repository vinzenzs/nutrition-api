package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGarmin_RegisteredSurface(t *testing.T) {
	specs := ByName(MCPRegistry())
	want := []string{
		"garmin_login", "garmin_submit_mfa", "garmin_schedule_workout",
		"garmin_unschedule_workout", "garmin_schedule_plan", "garmin_list_scheduled",
		"garmin_delete_workout", "garmin_list_workouts", "garmin_get_workout",
		"garmin_push_hydration", "garmin_export_activity", "garmin_get_activity_gear",
		"garmin_download_workout", "garmin_upload_activity", "garmin_rename_activity",
		"garmin_delete_activity", "garmin_backfill",
	}
	for _, n := range want {
		_, ok := specs[n]
		assert.Truef(t, ok, "garmin tool %q not on the MCP surface", n)
	}
}

func TestGarmin_LoginAndMFA(t *testing.T) {
	specs := ByName(MCPRegistry())

	// garmin_login → POST /garmin/login, no body, and OmitIdempotencyKey set
	// (interactive login is not a replayable write).
	login := specs["garmin_login"]
	call, err := login.Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/login", call.Path)
	assert.Empty(t, call.Body, "login posts no body")
	assert.True(t, login.OmitIdempotencyKey, "login must not carry an idempotency key")

	// garmin_submit_mfa → POST /garmin/login/mfa with {"code":...}, OmitIdempotencyKey.
	mfa := specs["garmin_submit_mfa"]
	call, err = mfa.Build(json.RawMessage(`{"code":"418923"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/login/mfa", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "418923", body["code"])
	assert.True(t, mfa.OmitIdempotencyKey)
}

func TestGarmin_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	// schedule_workout → POST /garmin/schedule/workout, body carries only workout_id.
	call, err := specs["garmin_schedule_workout"].Build(json.RawMessage(`{"workout_id":"w1","idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/schedule/workout", call.Path)
	assert.Contains(t, string(call.Body), `"workout_id":"w1"`)
	assert.NotContains(t, string(call.Body), "idempotency_key")

	// unschedule_workout → DELETE /garmin/schedule/workout/{id}
	call, _ = specs["garmin_unschedule_workout"].Build(json.RawMessage(`{"workout_id":"w1"}`))
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/garmin/schedule/workout/w1", call.Path)

	// schedule_plan → POST /garmin/schedule/plan; absent optional pointers serialize as null.
	call, _ = specs["garmin_schedule_plan"].Build(json.RawMessage(`{"plan_id":"p1","scope":"all"}`))
	assert.Equal(t, "/garmin/schedule/plan", call.Path)
	assert.Contains(t, string(call.Body), `"week":null`)

	// list_scheduled → GET /garmin/calendar?from&to
	call, _ = specs["garmin_list_scheduled"].Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30"}`))
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/garmin/calendar", call.Path)
	assert.Equal(t, "2026-06-01", call.Query.Get("from"))

	// list_workouts → GET /garmin/workouts with optional start/limit
	call, _ = specs["garmin_list_workouts"].Build(json.RawMessage(`{"start":10,"limit":5}`))
	assert.Equal(t, "/garmin/workouts", call.Path)
	assert.Equal(t, "10", call.Query.Get("start"))
	assert.Equal(t, "5", call.Query.Get("limit"))
	call, _ = specs["garmin_list_workouts"].Build(json.RawMessage(`{}`))
	assert.False(t, call.Query.Has("start"))

	// push_hydration → POST /garmin/hydration {value_ml, date}
	call, _ = specs["garmin_push_hydration"].Build(json.RawMessage(`{"value_ml":2400,"date":"2026-06-14"}`))
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/hydration", call.Path)
	assert.Contains(t, string(call.Body), `"value_ml":2400`)

	// export_activity → GET /garmin/activity/{id}/export?format
	call, _ = specs["garmin_export_activity"].Build(json.RawMessage(`{"activity_id":"a1","format":"gpx"}`))
	assert.Equal(t, "/garmin/activity/a1/export", call.Path)
	assert.Equal(t, "gpx", call.Query.Get("format"))
	// no format → key absent
	call, _ = specs["garmin_export_activity"].Build(json.RawMessage(`{"activity_id":"a1"}`))
	assert.False(t, call.Query.Has("format"))

	// download_workout → GET /garmin/workout/{id}/download
	call, _ = specs["garmin_download_workout"].Build(json.RawMessage(`{"garmin_workout_id":"g1"}`))
	assert.Equal(t, "/garmin/workout/g1/download", call.Path)

	// get_activity_gear → GET /garmin/activity/{id}/gear
	call, _ = specs["garmin_get_activity_gear"].Build(json.RawMessage(`{"activity_id":"a1"}`))
	assert.Equal(t, "/garmin/activity/a1/gear", call.Path)

	// upload_activity → POST /garmin/activity/upload {filename, content_base64}
	call, _ = specs["garmin_upload_activity"].Build(json.RawMessage(`{"filename":"ride.fit","content_base64":"QUJD"}`))
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/activity/upload", call.Path)
	assert.Contains(t, string(call.Body), `"filename":"ride.fit"`)
	assert.Contains(t, string(call.Body), `"content_base64":"QUJD"`)

	// rename_activity → PATCH /garmin/activity/{id} {name}
	call, _ = specs["garmin_rename_activity"].Build(json.RawMessage(`{"activity_id":"a1","name":"Evening ride"}`))
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/garmin/activity/a1", call.Path)
	assert.Contains(t, string(call.Body), `"name":"Evening ride"`)
	assert.NotContains(t, string(call.Body), "activity_id")

	// delete_activity → DELETE /garmin/activity/{id}
	call, _ = specs["garmin_delete_activity"].Build(json.RawMessage(`{"activity_id":"a1"}`))
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/garmin/activity/a1", call.Path)

	// get_workout → GET /garmin/workout/{id}
	call, _ = specs["garmin_get_workout"].Build(json.RawMessage(`{"garmin_workout_id":"g1"}`))
	assert.Equal(t, "/garmin/workout/g1", call.Path)

	// delete_workout → DELETE /garmin/workout/{id}
	call, _ = specs["garmin_delete_workout"].Build(json.RawMessage(`{"workout_id":"w1"}`))
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/garmin/workout/w1", call.Path)

	// backfill → POST /garmin/backfill {from, to}
	call, _ = specs["garmin_backfill"].Build(json.RawMessage(`{"from":"2026-01-01","to":"2026-02-01"}`))
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/garmin/backfill", call.Path)
	assert.Contains(t, string(call.Body), `"from":"2026-01-01"`)
}
