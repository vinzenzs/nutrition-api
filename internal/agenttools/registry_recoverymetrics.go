package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Recovery-metrics tools — the daily recovery snapshot (sleep, HRV, resting HR,
// stress, body battery, training readiness) the desktop coach logs/reads to
// reason about whether today's deficit / training load is tolerable. Ported
// from internal/mcpserver onto the shared registry (unify-mcp-tool-registry).
// The arg structs are byte-identical to the prior bespoke registrations so the
// announced schema is unchanged.

func init() { registerMCPDomain(recoveryMetricsSpecs()) }

// LogRecoveryMetricsArgs is the input to log_recovery_metrics.
type LogRecoveryMetricsArgs struct {
	Date               string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	SleepSeconds       *int     `json:"sleep_seconds,omitempty" jsonschema:"total sleep duration in seconds (> 0)"`
	SleepScore         *int     `json:"sleep_score,omitempty" jsonschema:"sleep score 0..100"`
	HRVMs              *float64 `json:"hrv_ms,omitempty" jsonschema:"overnight heart-rate variability in milliseconds (> 0)"`
	RestingHR          *int     `json:"resting_hr,omitempty" jsonschema:"resting heart rate in bpm (> 0)"`
	StressAvg          *int     `json:"stress_avg,omitempty" jsonschema:"average daily stress 0..100"`
	BodyBatteryCharged *int     `json:"body_battery_charged,omitempty" jsonschema:"body battery charged over the day 0..100"`
	BodyBatteryDrained *int     `json:"body_battery_drained,omitempty" jsonschema:"body battery drained over the day 0..100"`
	TrainingReadiness  *int     `json:"training_readiness,omitempty" jsonschema:"training readiness score 0..100"`
	IdempotencyKey     string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListRecoveryMetricsArgs is the input to list_recovery_metrics.
type ListRecoveryMetricsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

// GetRecoveryMetricsArgs is the input to get_recovery_metrics.
type GetRecoveryMetricsArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

// DeleteRecoveryMetricsArgs is the input to delete_recovery_metrics.
type DeleteRecoveryMetricsArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func recoveryMetricsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_recovery_metrics",
			Description: "Record (or replace) the daily recovery snapshot for a date — sleep, HRV, resting HR, " +
				"stress, body battery, training readiness. One snapshot per calendar day, keyed by `date`; " +
				"re-posting the same date overwrites it. Every metric is optional. This is the recovery context " +
				"for deciding whether today's deficit / training load is tolerable.",
			SchemaType: LogRecoveryMetricsArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogRecoveryMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Date               string   `json:"date"`
					SleepSeconds       *int     `json:"sleep_seconds,omitempty"`
					SleepScore         *int     `json:"sleep_score,omitempty"`
					HRVMs              *float64 `json:"hrv_ms,omitempty"`
					RestingHR          *int     `json:"resting_hr,omitempty"`
					StressAvg          *int     `json:"stress_avg,omitempty"`
					BodyBatteryCharged *int     `json:"body_battery_charged,omitempty"`
					BodyBatteryDrained *int     `json:"body_battery_drained,omitempty"`
					TrainingReadiness  *int     `json:"training_readiness,omitempty"`
				}{
					Date:               a.Date,
					SleepSeconds:       a.SleepSeconds,
					SleepScore:         a.SleepScore,
					HRVMs:              a.HRVMs,
					RestingHR:          a.RestingHR,
					StressAvg:          a.StressAvg,
					BodyBatteryCharged: a.BodyBatteryCharged,
					BodyBatteryDrained: a.BodyBatteryDrained,
					TrainingReadiness:  a.TrainingReadiness,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/recovery-metrics", Body: body}, nil
			},
		},
		{
			Name: "list_recovery_metrics",
			Description: "List daily recovery snapshots whose date falls in the inclusive [from, to] window " +
				"(YYYY-MM-DD, max 92 days). Use for trend questions ('how has my HRV trended this week?').",
			SchemaType: ListRecoveryMetricsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListRecoveryMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/recovery-metrics", Query: q}, nil
			},
		},
		{
			Name:        "get_recovery_metrics",
			Description: "Fetch the recovery snapshot for a single date (YYYY-MM-DD).",
			SchemaType:  GetRecoveryMetricsArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetRecoveryMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.Date == "" {
					return HTTPCall{}, fmt.Errorf("date is required")
				}
				return HTTPCall{Method: "GET", Path: "/recovery-metrics/" + url.PathEscape(a.Date)}, nil
			},
		},
		{
			Name:        "delete_recovery_metrics",
			Description: "Delete the recovery snapshot for a date. Returns an empty result on success.",
			SchemaType:  DeleteRecoveryMetricsArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteRecoveryMetricsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.Date == "" {
					return HTTPCall{}, fmt.Errorf("date is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/recovery-metrics/" + url.PathEscape(a.Date)}, nil
			},
		},
	}
}
