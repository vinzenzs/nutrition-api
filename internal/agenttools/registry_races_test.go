package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The races domain contributes exactly these six MCP tools, with the tiers the
// bespoke registrations implied (reads → TierRead; mutations → write-auto).
func TestRaces_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"create_race":       TierWriteAuto,
		"list_races":        TierRead,
		"get_race":          TierRead,
		"update_race":       TierWriteAuto,
		"delete_race":       TierWriteAuto,
		"plan_race_fueling": TierRead,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// create_race → POST /races with name/race_date/legs; the idempotency_key is
// dropped from the REST body.
func TestRaces_Create(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_race"].Build(json.RawMessage(
		`{"name":"Allgäu Sprint","race_date":"2026-07-24",` +
			`"legs":[{"ordinal":1,"discipline":"swim","expected_duration_min":90},` +
			`{"ordinal":2,"discipline":"bike","expected_duration_min":90}],` +
			`"idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/races", call.Path)
	assert.Empty(t, call.Query)

	assert.Contains(t, string(call.Body), `"name":"Allgäu Sprint"`)
	assert.Contains(t, string(call.Body), `"discipline":"bike"`)
	// idempotency_key must not be forwarded in the REST body.
	assert.NotContains(t, string(call.Body), "idempotency_key")
}

// create omits race_type/location/notes/legs when not supplied (omitempty), but
// always emits name/race_date — matching the bespoke marshal struct.
func TestRaces_CreateOmitsOptionals(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_race"].Build(json.RawMessage(`{"name":"X","race_date":"2026-07-24"}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "X", body["name"])
	assert.Equal(t, "2026-07-24", body["race_date"])
	assert.NotContains(t, body, "race_type")
	assert.NotContains(t, body, "location")
	assert.NotContains(t, body, "notes")
	assert.NotContains(t, body, "legs")
}

// list_races → GET /races, no query.
func TestRaces_List(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_races"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/races", call.Path)
}

// get_race → GET /races/{id}; missing id errors.
func TestRaces_Get(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["get_race"].Build(json.RawMessage(`{"id":"r1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/races/r1", call.Path)

	_, err = specs["get_race"].Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}

// update_race → PATCH /races/{id}; only supplied scalar fields in the body, id
// stripped, missing id errors.
func TestRaces_Update(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["update_race"].Build(json.RawMessage(`{"id":"r1","name":"Renamed"}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/races/r1", call.Path)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "Renamed", body["name"])
	assert.NotContains(t, body, "race_date")
	assert.NotContains(t, body, "legs")
	assert.NotContains(t, body, "id")
	assert.NotContains(t, body, "idempotency_key")

	_, err = specs["update_race"].Build(json.RawMessage(`{"name":"x"}`))
	assert.Error(t, err)
}

// update_race with a legs array REPLACES legs; an empty array clears them.
func TestRaces_UpdateLegsReplace(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["update_race"].Build(json.RawMessage(
		`{"id":"r1","legs":[{"ordinal":1,"discipline":"run"}]}`))
	require.NoError(t, err)
	assert.Contains(t, string(call.Body), `"discipline":"run"`)

	clear, err := specs["update_race"].Build(json.RawMessage(`{"id":"r1","legs":[]}`))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(clear.Body, &body))
	legs, ok := body["legs"]
	require.True(t, ok, "empty legs array must be present in the body to clear")
	assert.Empty(t, legs)
}

// delete_race → DELETE /races/{id}; missing id errors.
func TestRaces_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["delete_race"].Build(json.RawMessage(`{"id":"r9"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/races/r9", call.Path)

	_, err = specs["delete_race"].Build(json.RawMessage(`{}`))
	assert.Error(t, err)
}

// plan_race_fueling → GET /races/{id}/fueling-plan with body_weight_kg and an
// optional sweat_rate_ml_per_hr query param; missing id errors.
func TestRaces_PlanFueling(t *testing.T) {
	specs := ByName(MCPRegistry())

	call, err := specs["plan_race_fueling"].Build(json.RawMessage(
		`{"id":"race-123","body_weight_kg":70,"sweat_rate_ml_per_hr":900}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/races/race-123/fueling-plan", call.Path)
	assert.Equal(t, "70", call.Query.Get("body_weight_kg"))
	assert.Equal(t, "900", call.Query.Get("sweat_rate_ml_per_hr"))

	// sweat rate omitted when not supplied.
	noSweat, err := specs["plan_race_fueling"].Build(json.RawMessage(
		`{"id":"race-123","body_weight_kg":70}`))
	require.NoError(t, err)
	assert.Equal(t, "70", noSweat.Query.Get("body_weight_kg"))
	assert.False(t, noSweat.Query.Has("sweat_rate_ml_per_hr"),
		"sweat rate must be omitted when not supplied")

	_, err = specs["plan_race_fueling"].Build(json.RawMessage(`{"body_weight_kg":70}`))
	assert.Error(t, err)
}
