package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

func handleCreateRace(ctx context.Context, c *apiClient, args CreateRaceArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name     string       `json:"name"`
		RaceDate string       `json:"race_date"`
		RaceType *string      `json:"race_type,omitempty"`
		Location *string      `json:"location,omitempty"`
		Notes    *string      `json:"notes,omitempty"`
		Legs     []RaceLegArg `json:"legs,omitempty"`
	}{args.Name, args.RaceDate, args.RaceType, args.Location, args.Notes, args.Legs})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "create_race", args)
	status, respBody, err := c.Post(ctx, "/races", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleUpdateRace(ctx context.Context, c *apiClient, args UpdateRaceArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name     *string       `json:"name,omitempty"`
		RaceDate *string       `json:"race_date,omitempty"`
		RaceType *string       `json:"race_type,omitempty"`
		Location *string       `json:"location,omitempty"`
		Notes    *string       `json:"notes,omitempty"`
		Legs     *[]RaceLegArg `json:"legs,omitempty"`
	}{args.Name, args.RaceDate, args.RaceType, args.Location, args.Notes, args.Legs})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "update_race", args)
	status, respBody, err := c.Patch(ctx, "/races/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func handlePlanRaceFueling(ctx context.Context, c *apiClient, args PlanRaceFuelingArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("body_weight_kg", strconv.FormatFloat(args.BodyWeightKg, 'f', -1, 64))
	if args.SweatRateMlPerHr != nil {
		q.Set("sweat_rate_ml_per_hr", strconv.FormatFloat(*args.SweatRateMlPerHr, 'f', -1, 64))
	}
	status, body, err := c.Get(ctx, "/races/"+url.PathEscape(args.ID)+"/fueling-plan", q)
	return toToolResult(status, body, err)
}

func registerRaceTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "create_race",
		Description: "Create a persistent race with its ordered legs. A race is `{name, race_date, " +
			"race_type?, location?, notes?}` owning legs `{ordinal, discipline, distance_m?, " +
			"expected_duration_min?, intensity?}`. Disciplines: swim|bike|run|transition|other; ordinals " +
			"must be unique. This stores the durable race structure the agent reuses — compute the per-leg " +
			"fuelling plan separately with plan_race_fueling. For race-week carb-loading use plan_carb_load.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args CreateRaceArgs) (*mcp.CallToolResult, any, error) {
		return handleCreateRace(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_races",
		Description: "List all stored races with their legs, ordered by race date. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListRacesArgs) (*mcp.CallToolResult, any, error) {
		status, body, err := c.Get(ctx, "/races", nil)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_race",
		Description: "Fetch one race with its legs by id. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetRaceArgs) (*mcp.CallToolResult, any, error) {
		status, body, err := c.Get(ctx, "/races/"+url.PathEscape(args.ID), nil)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_race",
		Description: "Update a race's scalar fields, and optionally replace its legs. Supplying a `legs` " +
			"array REPLACES all legs wholesale (an empty array clears them); omit `legs` to leave them " +
			"unchanged.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UpdateRaceArgs) (*mcp.CallToolResult, any, error) {
		return handleUpdateRace(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_race",
		Description: "Delete a race by id; its legs are removed too.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteRaceArgs) (*mcp.CallToolResult, any, error) {
		key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_race", args)
		status, body, err := c.Delete(ctx, "/races/"+url.PathEscape(args.ID), key)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "plan_race_fueling",
		Description: "Compute the deterministic per-leg in-event fuelling plan for a stored race. Returns, " +
			"per leg, carbs (g/hr + total), sodium (mg/hr + total) and fluid (ml/hr + total), plus a race " +
			"total. Carbs band by total race duration (<75 min → 0, 75–150 → 60, ≥150 → 90 g/hr) and scale " +
			"by discipline (swim/transition 0, bike full, run 0.7, other 0.8). Fluid/sodium derive from " +
			"sweat_rate_ml_per_hr when supplied, else a flagged 600 ml/hr & 600 mg/hr default. This is a " +
			"baseline to adjust for weather, gut tolerance and course — read-only, no idempotency-key.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args PlanRaceFuelingArgs) (*mcp.CallToolResult, any, error) {
		return handlePlanRaceFueling(ctx, c, args), nil, nil
	})
}
