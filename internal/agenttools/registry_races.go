package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Race CRUD + per-leg fuelling plan. A race is the durable structure the desktop
// coach reuses: a `{name, race_date, race_type?, location?, notes?}` owning an
// ordered set of legs `{ordinal, discipline, distance_m?, expected_duration_min?,
// intensity?}`. Ported from internal/mcpserver (tools_races.go) onto the shared
// registry (unify-mcp-tool-registry). These are MCP-only (not chat-exposed); the
// descriptions and arg structs are byte-identical to the prior bespoke
// registrations so the announced schema is unchanged.

func init() { registerMCPDomain(racesSpecs()) }

// RaceLegArg is one leg in a race create/update call.
type RaceLegArg struct {
	Ordinal             int      `json:"ordinal" jsonschema:"1-based order of this leg within the race; unique per race"`
	Discipline          string   `json:"discipline" jsonschema:"one of swim|bike|run|transition|other"`
	DistanceM           *float64 `json:"distance_m,omitempty" jsonschema:"optional leg distance in metres; > 0"`
	ExpectedDurationMin *int     `json:"expected_duration_min,omitempty" jsonschema:"optional expected duration in minutes; > 0. Drives the fuelling plan — legs without it get no fuelling."`
	Intensity           *string  `json:"intensity,omitempty" jsonschema:"optional free annotation (easy|moderate|hard|race or a zone)"`
}

// CreateRaceArgs is the input for create_race.
type CreateRaceArgs struct {
	Name           string       `json:"name" jsonschema:"race name, e.g. 'Allgäu Triathlon Sprint'"`
	RaceDate       string       `json:"race_date" jsonschema:"race date in YYYY-MM-DD"`
	RaceType       *string      `json:"race_type,omitempty" jsonschema:"optional free annotation: sprint|olympic|70.3|ironman|…"`
	Location       *string      `json:"location,omitempty" jsonschema:"optional location"`
	Notes          *string      `json:"notes,omitempty" jsonschema:"optional free-text notes"`
	Legs           []RaceLegArg `json:"legs,omitempty" jsonschema:"ordered legs of the race"`
	IdempotencyKey string       `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted a stable key is derived from the other args"`
}

// GetRaceArgs / ListRacesArgs / DeleteRaceArgs.
type GetRaceArgs struct {
	ID string `json:"id" jsonschema:"race UUID"`
}

type ListRacesArgs struct{}

type DeleteRaceArgs struct {
	ID             string `json:"id" jsonschema:"race UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// UpdateRaceArgs is the input for update_race. A non-nil Legs slice REPLACES
// all legs wholesale; omit it to leave legs unchanged.
type UpdateRaceArgs struct {
	ID             string        `json:"id" jsonschema:"race UUID"`
	Name           *string       `json:"name,omitempty" jsonschema:"new race name"`
	RaceDate       *string       `json:"race_date,omitempty" jsonschema:"new race date YYYY-MM-DD"`
	RaceType       *string       `json:"race_type,omitempty" jsonschema:"new race type annotation"`
	Location       *string       `json:"location,omitempty" jsonschema:"new location"`
	Notes          *string       `json:"notes,omitempty" jsonschema:"new notes"`
	Legs           *[]RaceLegArg `json:"legs,omitempty" jsonschema:"if present, REPLACES all legs (empty array clears them); omit to leave unchanged"`
	IdempotencyKey string        `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// PlanRaceFuelingArgs is the input for plan_race_fueling (read-only).
type PlanRaceFuelingArgs struct {
	ID               string   `json:"id" jsonschema:"race UUID"`
	BodyWeightKg     float64  `json:"body_weight_kg" jsonschema:"athlete body weight in kilograms, 30..200"`
	SweatRateMlPerHr *float64 `json:"sweat_rate_ml_per_hr,omitempty" jsonschema:"optional measured sweat rate in ml/hr; personalises fluid and sodium (else a flagged 600 ml/hr default is used)"`
}

func racesSpecs() []Spec {
	return []Spec{
		{
			Name: "create_race",
			Description: "Create a persistent race with its ordered legs. A race is `{name, race_date, " +
				"race_type?, location?, notes?}` owning legs `{ordinal, discipline, distance_m?, " +
				"expected_duration_min?, intensity?}`. Disciplines: swim|bike|run|transition|other; ordinals " +
				"must be unique. This stores the durable race structure the agent reuses — compute the per-leg " +
				"fuelling plan separately with plan_race_fueling. For race-week carb-loading use plan_carb_load.",
			SchemaType: CreateRaceArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateRaceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Name     string       `json:"name"`
					RaceDate string       `json:"race_date"`
					RaceType *string      `json:"race_type,omitempty"`
					Location *string      `json:"location,omitempty"`
					Notes    *string      `json:"notes,omitempty"`
					Legs     []RaceLegArg `json:"legs,omitempty"`
				}{a.Name, a.RaceDate, a.RaceType, a.Location, a.Notes, a.Legs})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/races", Body: body}, nil
			},
		},
		{
			Name:        "list_races",
			Description: "List all stored races with their legs, ordered by race date. Read-only.",
			SchemaType:  ListRacesArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/races"}, nil
			},
		},
		{
			Name:        "get_race",
			Description: "Fetch one race with its legs by id. Read-only.",
			SchemaType:  GetRaceArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetRaceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "GET", Path: "/races/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "update_race",
			Description: "Update a race's scalar fields, and optionally replace its legs. Supplying a `legs` " +
				"array REPLACES all legs wholesale (an empty array clears them); omit `legs` to leave them " +
				"unchanged.",
			SchemaType: UpdateRaceArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UpdateRaceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				body, err := json.Marshal(struct {
					Name     *string       `json:"name,omitempty"`
					RaceDate *string       `json:"race_date,omitempty"`
					RaceType *string       `json:"race_type,omitempty"`
					Location *string       `json:"location,omitempty"`
					Notes    *string       `json:"notes,omitempty"`
					Legs     *[]RaceLegArg `json:"legs,omitempty"`
				}{a.Name, a.RaceDate, a.RaceType, a.Location, a.Notes, a.Legs})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/races/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_race",
			Description: "Delete a race by id; its legs are removed too.",
			SchemaType:  DeleteRaceArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteRaceArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/races/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "plan_race_fueling",
			Description: "Compute the deterministic per-leg in-event fuelling plan for a stored race. Returns, " +
				"per leg, carbs (g/hr + total), sodium (mg/hr + total) and fluid (ml/hr + total), plus a race " +
				"total. Carbs band by total race duration (<75 min → 0, 75–150 → 60, ≥150 → 90 g/hr) and scale " +
				"by discipline (swim/transition 0, bike full, run 0.7, other 0.8). Fluid/sodium derive from " +
				"sweat_rate_ml_per_hr when supplied, else a flagged 600 ml/hr & 600 mg/hr default. This is a " +
				"baseline to adjust for weather, gut tolerance and course — read-only, no idempotency-key.",
			SchemaType: PlanRaceFuelingArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PlanRaceFuelingArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				q := url.Values{}
				q.Set("body_weight_kg", strconv.FormatFloat(a.BodyWeightKg, 'f', -1, 64))
				if a.SweatRateMlPerHr != nil {
					q.Set("sweat_rate_ml_per_hr", strconv.FormatFloat(*a.SweatRateMlPerHr, 'f', -1, 64))
				}
				return HTTPCall{Method: "GET", Path: "/races/" + url.PathEscape(a.ID) + "/fueling-plan", Query: q}, nil
			},
		},
	}
}
