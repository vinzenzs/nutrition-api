package agenttools

import (
	"encoding/json"
	"net/url"
)

// Fitness-metrics tools — daily fitness snapshots (VO2max, race predictions,
// acute/chronic training load). Ported from internal/mcpserver onto the shared
// registry (unify-mcp-tool-registry). The arg structs and descriptions are
// byte-identical to the prior bespoke registrations so the announced schema is
// unchanged.

func init() { registerMCPDomain(fitnessMetricsSpecs()) }

// LogFitnessMetricsArgs is the input to log_fitness_metrics.
type LogFitnessMetricsArgs struct {
	Date                     string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	VO2MaxRunning            *float64 `json:"vo2max_running,omitempty" jsonschema:"running VO2max (> 0)"`
	VO2MaxCycling            *float64 `json:"vo2max_cycling,omitempty" jsonschema:"cycling VO2max (> 0)"`
	RacePredictor5kSeconds   *int     `json:"race_predictor_5k_seconds,omitempty" jsonschema:"predicted 5k time in SECONDS (> 0)"`
	RacePredictor10kSeconds  *int     `json:"race_predictor_10k_seconds,omitempty" jsonschema:"predicted 10k time in SECONDS (> 0)"`
	RacePredictorHalfSeconds *int     `json:"race_predictor_half_seconds,omitempty" jsonschema:"predicted half-marathon time in SECONDS (> 0)"`
	RacePredictorFullSeconds *int     `json:"race_predictor_full_seconds,omitempty" jsonschema:"predicted marathon time in SECONDS (> 0)"`
	AcuteLoad                *float64 `json:"acute_load,omitempty" jsonschema:"acute (7-day) training load (>= 0)"`
	ChronicLoad              *float64 `json:"chronic_load,omitempty" jsonschema:"chronic (28-day) training load (>= 0)"`
	IdempotencyKey           string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListFitnessMetricsArgs is the input to list_fitness_metrics.
type ListFitnessMetricsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

// GetFitnessMetricsArgs is the input to get_fitness_metrics.
type GetFitnessMetricsArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

// DeleteFitnessMetricsArgs is the input to delete_fitness_metrics.
type DeleteFitnessMetricsArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func fitnessMetricsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_fitness_metrics",
			Description: "Record (or replace) the daily fitness snapshot for a date — VO2max (run/bike), race " +
				"predictions, acute/chronic training load. One snapshot per calendar day, keyed by `date`. " +
				"Race predictions are SECONDS (format h:mm:ss yourself). Acute:chronic ratio = acute_load / " +
				"chronic_load (compute it; it isn't stored).",
			SchemaType: LogFitnessMetricsArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogFitnessMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Date                     string   `json:"date"`
					VO2MaxRunning            *float64 `json:"vo2max_running,omitempty"`
					VO2MaxCycling            *float64 `json:"vo2max_cycling,omitempty"`
					RacePredictor5kSeconds   *int     `json:"race_predictor_5k_seconds,omitempty"`
					RacePredictor10kSeconds  *int     `json:"race_predictor_10k_seconds,omitempty"`
					RacePredictorHalfSeconds *int     `json:"race_predictor_half_seconds,omitempty"`
					RacePredictorFullSeconds *int     `json:"race_predictor_full_seconds,omitempty"`
					AcuteLoad                *float64 `json:"acute_load,omitempty"`
					ChronicLoad              *float64 `json:"chronic_load,omitempty"`
				}{
					Date:                     a.Date,
					VO2MaxRunning:            a.VO2MaxRunning,
					VO2MaxCycling:            a.VO2MaxCycling,
					RacePredictor5kSeconds:   a.RacePredictor5kSeconds,
					RacePredictor10kSeconds:  a.RacePredictor10kSeconds,
					RacePredictorHalfSeconds: a.RacePredictorHalfSeconds,
					RacePredictorFullSeconds: a.RacePredictorFullSeconds,
					AcuteLoad:                a.AcuteLoad,
					ChronicLoad:              a.ChronicLoad,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/fitness-metrics", Body: body}, nil
			},
		},
		{
			Name: "list_fitness_metrics",
			Description: "List daily fitness snapshots whose date falls in the inclusive [from, to] window " +
				"(YYYY-MM-DD, max 92 days). Use for fitness-trend questions (VO2max progression, load ramp).",
			SchemaType: ListFitnessMetricsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListFitnessMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/fitness-metrics", Query: q}, nil
			},
		},
		{
			Name:        "get_fitness_metrics",
			Description: "Fetch the fitness snapshot for a single date (YYYY-MM-DD).",
			SchemaType:  GetFitnessMetricsArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetFitnessMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/fitness-metrics/" + url.PathEscape(a.Date)}, nil
			},
		},
		{
			Name:        "delete_fitness_metrics",
			Description: "Delete the fitness snapshot for a date. Returns an empty result on success.",
			SchemaType:  DeleteFitnessMetricsArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteFitnessMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/fitness-metrics/" + url.PathEscape(a.Date)}, nil
			},
		},
	}
}
