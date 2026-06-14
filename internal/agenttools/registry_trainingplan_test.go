package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The training-plan domain contributes exactly these thirteen MCP tools (plan
// CRUD, week CRUD, slot CRUD, materialize, and the program read), with the tiers
// the bespoke registrations implied (reads → TierRead; mutations → write).
func TestTrainingPlan_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"create_training_plan":      TierWriteAuto,
		"list_training_plans":       TierRead,
		"get_training_plan":         TierRead,
		"patch_training_plan":       TierWriteAuto,
		"delete_training_plan":      TierWriteAuto,
		"add_plan_week":             TierWriteAuto,
		"patch_plan_week":           TierWriteAuto,
		"delete_plan_week":          TierWriteAuto,
		"add_plan_slot":             TierWriteAuto,
		"patch_plan_slot":           TierWriteAuto,
		"delete_plan_slot":          TierWriteAuto,
		"materialize_training_plan": TierWriteAuto,
		"get_workout_program":       TierRead,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// create_training_plan → POST /training-plans; the body is marshalled from a
// map with raw pointer values, so absent optional pointers (race_id, notes)
// serialize as JSON null — NOT omitted — matching the bespoke handler. The
// idempotency_key is never a body field.
func TestTrainingPlan_Create(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["create_training_plan"].Build(json.RawMessage(
		`{"name":"18-week build","start_date":"2026-03-23","race_id":"race-1","notes":"A race","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/training-plans", call.Path)
	assert.Empty(t, call.Query)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "18-week build", body["name"])
	assert.Equal(t, "2026-03-23", body["start_date"])
	assert.Equal(t, "race-1", body["race_id"])
	assert.Equal(t, "A race", body["notes"])
	assert.NotContains(t, body, "idempotency_key")
}

// When the optional pointers are omitted, the bespoke handler still emits them
// as JSON null (map marshal of nil *string), so the keys are PRESENT with a null
// value. Replicating that exactly preserves the byte-stable contract.
func TestTrainingPlan_CreateNullOptionals(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["create_training_plan"].Build(json.RawMessage(
		`{"name":"plan","start_date":"2026-03-23"}`))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	v, ok := body["race_id"]
	require.True(t, ok, "race_id key present even when absent (nil pointer → null)")
	assert.Nil(t, v)
	n, ok := body["notes"]
	require.True(t, ok, "notes key present even when absent (nil pointer → null)")
	assert.Nil(t, n)
}

// list_training_plans → GET /training-plans, no query.
func TestTrainingPlan_List(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_training_plans"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/training-plans", call.Path)
	assert.Empty(t, call.Query)
	assert.Nil(t, call.Body)
}

// get_training_plan → GET /training-plans/{id} with the id path-escaped.
func TestTrainingPlan_Get(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_training_plan"].Build(json.RawMessage(`{"id":"plan abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/training-plans/plan%20abc", call.Path)
}

// patch_training_plan → PATCH /training-plans/{id}; only supplied fields in the
// body, id consumed for the path.
func TestTrainingPlan_Patch(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_training_plan"].Build(json.RawMessage(
		`{"id":"p1","name":"renamed","start_date":"2026-04-06"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/training-plans/p1", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "renamed", body["name"])
	assert.Equal(t, "2026-04-06", body["start_date"])
	assert.NotContains(t, body, "id")
	assert.NotContains(t, body, "race_id")
	assert.NotContains(t, body, "notes")
}

// patch with no editable fields produces an empty JSON object body, mirroring
// the bespoke handler's unconditional map marshal.
func TestTrainingPlan_PatchEmptyBody(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_training_plan"].Build(json.RawMessage(`{"id":"p1"}`))
	require.NoError(t, err)
	assert.Equal(t, "{}", string(call.Body))
}

// delete_training_plan → DELETE /training-plans/{id}.
func TestTrainingPlan_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_training_plan"].Build(json.RawMessage(`{"id":"p9"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/training-plans/p9", call.Path)
}

// add_plan_week → POST /training-plans/{plan_id}/weeks; body carries ordinal plus
// phase_id/notes (null when absent, per the bespoke map marshal). plan_id is the
// path, not a body field; idempotency_key dropped.
func TestTrainingPlan_AddWeek(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["add_plan_week"].Build(json.RawMessage(
		`{"plan_id":"p1","ordinal":3,"phase_id":"ph1","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/training-plans/p1/weeks", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, float64(3), body["ordinal"])
	assert.Equal(t, "ph1", body["phase_id"])
	assert.NotContains(t, body, "plan_id")
	assert.NotContains(t, body, "idempotency_key")
	// notes absent → null (key present), matching the bespoke map marshal.
	n, ok := body["notes"]
	require.True(t, ok)
	assert.Nil(t, n)
}

// patch_plan_week → PATCH /training-plans/{plan_id}/weeks/{week_id}; only supplied
// fields in the body.
func TestTrainingPlan_PatchWeek(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_plan_week"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2","ordinal":4}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/training-plans/p1/weeks/w2", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, float64(4), body["ordinal"])
	assert.NotContains(t, body, "phase_id")
	assert.NotContains(t, body, "notes")
	assert.NotContains(t, body, "plan_id")
	assert.NotContains(t, body, "week_id")
}

// delete_plan_week → DELETE /training-plans/{plan_id}/weeks/{week_id}.
func TestTrainingPlan_DeleteWeek(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_plan_week"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/training-plans/p1/weeks/w2", call.Path)
}

// add_plan_slot → POST /training-plans/{plan_id}/weeks/{week_id}/slots; the body
// always carries weekday/ordinal/template_id/time_of_day (time_of_day null when
// absent) and target_overrides only when supplied. plan_id/week_id are path,
// idempotency_key dropped.
func TestTrainingPlan_AddSlot(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["add_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2","weekday":2,"ordinal":0,"template_id":"t1",` +
			`"time_of_day":"06:30","target_overrides":[{"intent":"interval",` +
			`"target":{"kind":"pace","low_sec_per_km":435,"high_sec_per_km":435}}],` +
			`"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/training-plans/p1/weeks/w2/slots", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, float64(2), body["weekday"])
	assert.Equal(t, float64(0), body["ordinal"])
	assert.Equal(t, "t1", body["template_id"])
	assert.Equal(t, "06:30", body["time_of_day"])
	assert.Contains(t, body, "target_overrides")
	assert.NotContains(t, body, "plan_id")
	assert.NotContains(t, body, "week_id")
	assert.NotContains(t, body, "idempotency_key")

	overrides, ok := body["target_overrides"].([]any)
	require.True(t, ok)
	require.Len(t, overrides, 1)
	ov := overrides[0].(map[string]any)
	assert.Equal(t, "interval", ov["intent"])
	tgt := ov["target"].(map[string]any)
	assert.Equal(t, "pace", tgt["kind"])
	assert.Equal(t, float64(435), tgt["low_sec_per_km"])
}

// add_plan_slot omits target_overrides when not supplied (conditional add) but
// always emits time_of_day as null when absent, matching the bespoke handler.
func TestTrainingPlan_AddSlotOmitsOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["add_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2","weekday":2,"ordinal":0,"template_id":"t1"}`))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "target_overrides")
	tod, ok := body["time_of_day"]
	require.True(t, ok, "time_of_day present even when absent (nil pointer → null)")
	assert.Nil(t, tod)
}

// patch_plan_slot → PATCH /training-plans/{plan_id}/slots/{slot_id}; the slot
// lives under /slots/ (NOT under its week). Only supplied fields in the body.
func TestTrainingPlan_PatchSlot(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","slot_id":"s3","weekday":4,"template_id":"t2"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/training-plans/p1/slots/s3", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, float64(4), body["weekday"])
	assert.Equal(t, "t2", body["template_id"])
	assert.NotContains(t, body, "ordinal")
	assert.NotContains(t, body, "time_of_day")
	assert.NotContains(t, body, "target_overrides")
	assert.NotContains(t, body, "plan_id")
	assert.NotContains(t, body, "slot_id")
}

// patch_plan_slot with an empty target_overrides list forwards the (present)
// empty array — the wholesale-replace "clear all overrides" sentinel — because
// the pointer is non-nil. This is the tri-state semantic the handler relied on.
func TestTrainingPlan_PatchSlotClearsOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","slot_id":"s3","target_overrides":[]}`))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	v, ok := body["target_overrides"]
	require.True(t, ok, "empty list sentinel must be forwarded (clears all overrides)")
	arr, ok := v.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 0)
}

// add_plan_slot forwards duration_overrides when supplied (and omits them when
// not), alongside target_overrides — the two lists are independent.
func TestTrainingPlan_AddSlotForwardsDurationOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["add_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2","weekday":2,"ordinal":0,"template_id":"t1",` +
			`"duration_overrides":[{"intent":"active","duration":{"kind":"time","seconds":4800}}]}`))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "target_overrides")
	dov, ok := body["duration_overrides"].([]any)
	require.True(t, ok)
	require.Len(t, dov, 1)
	entry := dov[0].(map[string]any)
	assert.Equal(t, "active", entry["intent"])
	dur := entry["duration"].(map[string]any)
	assert.Equal(t, "time", dur["kind"])
	assert.Equal(t, float64(4800), dur["seconds"])
}

func TestTrainingPlan_AddSlotOmitsDurationOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["add_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","week_id":"w2","weekday":2,"ordinal":0,"template_id":"t1"}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.NotContains(t, body, "duration_overrides")
}

// patch_plan_slot forwards an empty duration_overrides list (the wholesale-clear
// sentinel) because the pointer is non-nil — same tri-state as target_overrides.
func TestTrainingPlan_PatchSlotClearsDurationOverrides(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["patch_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","slot_id":"s3","duration_overrides":[]}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	v, ok := body["duration_overrides"]
	require.True(t, ok, "empty list sentinel must be forwarded (clears all duration overrides)")
	arr, ok := v.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 0)
}

// delete_plan_slot → DELETE /training-plans/{plan_id}/slots/{slot_id}.
func TestTrainingPlan_DeleteSlot(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_plan_slot"].Build(json.RawMessage(
		`{"plan_id":"p1","slot_id":"s3"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/training-plans/p1/slots/s3", call.Path)
}

// materialize_training_plan → POST /training-plans/{plan_id}/materialize; body
// carries scope plus week/from/to (null when absent, per the bespoke map
// marshal). plan_id is the path; no idempotency key (naturally slot-keyed).
func TestTrainingPlan_Materialize(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["materialize_training_plan"].Build(json.RawMessage(
		`{"plan_id":"p1","scope":"week","week":3}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/training-plans/p1/materialize", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "week", body["scope"])
	assert.Equal(t, float64(3), body["week"])
	assert.NotContains(t, body, "plan_id")
	// from/to absent → null (keys present), per the bespoke map marshal.
	f, ok := body["from"]
	require.True(t, ok)
	assert.Nil(t, f)
	to, ok := body["to"]
	require.True(t, ok)
	assert.Nil(t, to)
}

// materialize range scope forwards from/to.
func TestTrainingPlan_MaterializeRange(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["materialize_training_plan"].Build(json.RawMessage(
		`{"plan_id":"p1","scope":"range","from":"2026-03-23","to":"2026-03-29"}`))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "range", body["scope"])
	assert.Equal(t, "2026-03-23", body["from"])
	assert.Equal(t, "2026-03-29", body["to"])
	w, ok := body["week"]
	require.True(t, ok)
	assert.Nil(t, w)
}

// get_workout_program → GET /workouts/{id}/program (note: under /workouts, not
// /training-plans), with the id path-escaped.
func TestTrainingPlan_GetWorkoutProgram(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["get_workout_program"].Build(json.RawMessage(`{"id":"wk1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/wk1/program", call.Path)
}
