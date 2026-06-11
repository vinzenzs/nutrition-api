package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRace_PostsBodyWithIdempotencyKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 201, `{"id":"r1"}`)
	dur := 90
	handleCreateRace(context.Background(), c, CreateRaceArgs{
		Name:     "Allgäu Sprint",
		RaceDate: "2026-07-24",
		Legs: []RaceLegArg{
			{Ordinal: 1, Discipline: "swim", ExpectedDurationMin: &dur},
			{Ordinal: 2, Discipline: "bike", ExpectedDurationMin: &dur},
		},
	})
	rec := (*records)[0]
	assert.Equal(t, "POST", rec.method)
	assert.Equal(t, "/races", rec.path)
	assert.NotEmpty(t, rec.idemKey, "write tool must set an idempotency key")
	assert.Contains(t, rec.body, `"name":"Allgäu Sprint"`)
	assert.Contains(t, rec.body, `"discipline":"bike"`)
}

func TestCreateRace_ExplicitIdempotencyKeyWins(t *testing.T) {
	c, records := newRacePrepRecorder(t, 201, `{}`)
	handleCreateRace(context.Background(), c, CreateRaceArgs{
		Name:           "X",
		RaceDate:       "2026-07-24",
		IdempotencyKey: "explicit-key",
	})
	assert.Equal(t, "explicit-key", (*records)[0].idemKey)
	// The body must not carry the idempotency_key field.
	assert.NotContains(t, (*records)[0].body, "idempotency_key")
}

func TestPlanRaceFueling_ForwardsParamsNoIdempotency(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{}`)
	sweat := 900.0
	handlePlanRaceFueling(context.Background(), c, PlanRaceFuelingArgs{
		ID:               "race-123",
		BodyWeightKg:     70,
		SweatRateMlPerHr: &sweat,
	})
	rec := (*records)[0]
	assert.Equal(t, "GET", rec.method)
	assert.Equal(t, "/races/race-123/fueling-plan", rec.path)
	assert.Contains(t, rec.rawQuery, "body_weight_kg=70")
	assert.Contains(t, rec.rawQuery, "sweat_rate_ml_per_hr=900")
	assert.Empty(t, rec.idemKey, "read tool must not set an idempotency key")
}

func TestPlanRaceFueling_OmitsSweatRateWhenAbsent(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{}`)
	handlePlanRaceFueling(context.Background(), c, PlanRaceFuelingArgs{
		ID:           "race-123",
		BodyWeightKg: 70,
	})
	rec := (*records)[0]
	assert.Contains(t, rec.rawQuery, "body_weight_kg=70")
	assert.False(t, strings.Contains(rec.rawQuery, "sweat_rate_ml_per_hr"),
		"sweat rate must be omitted when not supplied")
}

func TestUpdateRace_PatchesWithIdempotencyKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{}`)
	name := "Renamed"
	handleUpdateRace(context.Background(), c, UpdateRaceArgs{ID: "r1", Name: &name})
	rec := (*records)[0]
	assert.Equal(t, "PATCH", rec.method)
	assert.Equal(t, "/races/r1", rec.path)
	assert.NotEmpty(t, rec.idemKey)
	assert.Contains(t, rec.body, `"name":"Renamed"`)
}
