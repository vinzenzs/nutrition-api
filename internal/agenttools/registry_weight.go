package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Body-weight tracking — log/list/patch/delete measurements plus a smoothed
// rolling-average trend. Ported from internal/mcpserver onto the shared
// registry (unify-mcp-tool-registry). The arg structs, descriptions, and REST
// mappings are byte-identical to the prior bespoke registrations so the
// announced schema and the loopback calls are unchanged.

func init() { registerMCPDomain(weightSpecs()) }

// LogWeightArgs is the input to log_weight.
type LogWeightArgs struct {
	WeightKg       float64  `json:"weight_kg" jsonschema:"body weight in kilograms; must be greater than zero"`
	LoggedAt       string   `json:"logged_at" jsonschema:"when the measurement was taken, RFC 3339 timestamp"`
	BodyFatPct     *float64 `json:"body_fat_pct,omitempty" jsonschema:"optional body-fat percentage, 0..100"`
	MuscleMassKg   *float64 `json:"muscle_mass_kg,omitempty" jsonschema:"optional smart-scale muscle mass in kg (> 0)"`
	BodyWaterPct   *float64 `json:"body_water_pct,omitempty" jsonschema:"optional smart-scale body water percentage, 0..100"`
	BoneMassKg     *float64 `json:"bone_mass_kg,omitempty" jsonschema:"optional smart-scale bone mass in kg (> 0)"`
	BMI            *float64 `json:"bmi,omitempty" jsonschema:"optional body mass index (> 0)"`
	Note           string   `json:"note,omitempty" jsonschema:"optional free-text context (e.g. 'morning, fasted', 'hotel scale', 'post-workout')"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListWeightsArgs is the input to list_weights.
type ListWeightsArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

// PatchWeightArgs is the input to patch_weight.
type PatchWeightArgs struct {
	ID             string   `json:"id" jsonschema:"the id of the body-weight entry to update"`
	WeightKg       *float64 `json:"weight_kg,omitempty" jsonschema:"new weight in kg; must be greater than zero if supplied"`
	BodyFatPct     *float64 `json:"body_fat_pct,omitempty" jsonschema:"new body-fat % (0..100) if supplied"`
	MuscleMassKg   *float64 `json:"muscle_mass_kg,omitempty" jsonschema:"new muscle mass in kg (> 0)"`
	BodyWaterPct   *float64 `json:"body_water_pct,omitempty" jsonschema:"new body water % (0..100)"`
	BoneMassKg     *float64 `json:"bone_mass_kg,omitempty" jsonschema:"new bone mass in kg (> 0)"`
	BMI            *float64 `json:"bmi,omitempty" jsonschema:"new BMI (> 0)"`
	LoggedAt       *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	Note           *string  `json:"note,omitempty" jsonschema:"new note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteWeightArgs is the input to delete_weight.
type DeleteWeightArgs struct {
	ID             string `json:"id" jsonschema:"the id of the body-weight entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// WeightTrendArgs is the input to weight_trend.
type WeightTrendArgs struct {
	From       string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To         string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 366 days from 'from'"`
	WindowDays *int   `json:"window_days,omitempty" jsonschema:"trailing window length in days, 1..30 (default 7)"`
	TZ         string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin); if omitted, the REST server uses DEFAULT_USER_TZ"`
}

func weightSpecs() []Spec {
	return []Spec{
		{
			Name: "log_weight",
			Description: "Record a body-weight measurement, optionally with body-fat %. Multiple " +
				"measurements per day are fine — the trend tool smooths them. Use `note` for context " +
				"that affects readings (post-workout, post-meal, hotel scale, non-morning timing). " +
				"Body-weight feeds the EA computation, race-day fuelling math, and the trend signal.",
			SchemaType: LogWeightArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args LogWeightArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					WeightKg     float64  `json:"weight_kg"`
					LoggedAt     string   `json:"logged_at"`
					BodyFatPct   *float64 `json:"body_fat_pct,omitempty"`
					MuscleMassKg *float64 `json:"muscle_mass_kg,omitempty"`
					BodyWaterPct *float64 `json:"body_water_pct,omitempty"`
					BoneMassKg   *float64 `json:"bone_mass_kg,omitempty"`
					BMI          *float64 `json:"bmi,omitempty"`
					Note         string   `json:"note,omitempty"`
				}{
					WeightKg:     args.WeightKg,
					LoggedAt:     args.LoggedAt,
					BodyFatPct:   args.BodyFatPct,
					MuscleMassKg: args.MuscleMassKg,
					BodyWaterPct: args.BodyWaterPct,
					BoneMassKg:   args.BoneMassKg,
					BMI:          args.BMI,
					Note:         args.Note,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/weight", Body: body}, nil
			},
		},
		{
			Name: "list_weights",
			Description: "List body-weight entries whose logged_at falls within the half-open [from, to) " +
				"RFC 3339 window. Window is capped at 92 days. Use weight_trend instead when you want a " +
				"smoothed trajectory rather than raw entries.",
			SchemaType: ListWeightsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args ListWeightsArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", args.From)
				q.Set("to", args.To)
				return HTTPCall{Method: "GET", Path: "/weight", Query: q}, nil
			},
		},
		{
			Name:        "patch_weight",
			Description: "Partially update an existing body-weight entry. Only supplied fields are changed.",
			SchemaType:  PatchWeightArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args PatchWeightArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if args.WeightKg != nil {
					payload["weight_kg"] = *args.WeightKg
				}
				if args.BodyFatPct != nil {
					payload["body_fat_pct"] = *args.BodyFatPct
				}
				if args.MuscleMassKg != nil {
					payload["muscle_mass_kg"] = *args.MuscleMassKg
				}
				if args.BodyWaterPct != nil {
					payload["body_water_pct"] = *args.BodyWaterPct
				}
				if args.BoneMassKg != nil {
					payload["bone_mass_kg"] = *args.BoneMassKg
				}
				if args.BMI != nil {
					payload["bmi"] = *args.BMI
				}
				if args.LoggedAt != nil {
					payload["logged_at"] = *args.LoggedAt
				}
				if args.Note != nil {
					payload["note"] = *args.Note
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/weight/" + url.PathEscape(args.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_weight",
			Description: "Delete a body-weight entry. Returns an empty result on success.",
			SchemaType:  DeleteWeightArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args DeleteWeightArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/weight/" + url.PathEscape(args.ID)}, nil
			},
		},
		{
			Name: "weight_trend",
			Description: "Return a rolling-average body-weight curve for a date range. `window_days` " +
				"defaults to 7 (suppresses normal daily noise from hydration, glycogen, food in gut). " +
				"Each point carries `sample_count` — a `rolling_avg_kg` from `sample_count: 1` is NOT a " +
				"trend, it's just that one sample. Check `sample_count` before basing decisions on a value.",
			SchemaType: WeightTrendArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args WeightTrendArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", args.From)
				q.Set("to", args.To)
				if args.WindowDays != nil {
					q.Set("window_days", strconv.Itoa(*args.WindowDays))
				}
				if args.TZ != "" {
					q.Set("tz", args.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/weight/trend", Query: q}, nil
			},
		},
	}
}
