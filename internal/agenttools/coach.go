package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// coachReadSpecs are the aggregate grounding reads the coach calls before giving
// training or recovery advice (the training/recovery siblings of
// get_daily_context). Backed by the /context/* endpoints from
// add-coach-context-endpoints.
func coachReadSpecs() []Spec {
	return []Spec{
		{
			Name:        "get_training_context",
			Description: "Read recent training in one call: the current plan phase, latest fitness (VO2max, acute/chronic load + ACWR, training status), a recent-load summary with recent completed workouts (lookback_days, default 14), and upcoming planned workouts (lookahead_days, default 7). Call this FIRST before any training advice.",
			Schema:      `{"type":"object","properties":{"date":{"type":"string","description":"YYYY-MM-DD; defaults to today"},"tz":{"type":"string","description":"IANA timezone; optional"},"lookback_days":{"type":"integer"},"lookahead_days":{"type":"integer"}}}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					Date          string `json:"date"`
					TZ            string `json:"tz"`
					LookbackDays  int    `json:"lookback_days"`
					LookaheadDays int    `json:"lookahead_days"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Date != "" {
					q.Set("date", a.Date)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.LookbackDays > 0 {
					q.Set("lookback_days", fmt.Sprintf("%d", a.LookbackDays))
				}
				if a.LookaheadDays > 0 {
					q.Set("lookahead_days", fmt.Sprintf("%d", a.LookaheadDays))
				}
				return HTTPCall{Method: "GET", Path: "/context/training", Query: q}, nil
			},
		},
		{
			Name:        "get_recovery_context",
			Description: "Read recovery readiness in one call: the latest recovery snapshot (sleep, HRV, resting HR, body battery, training readiness) plus the recent trend over `days` (default 7). Call this before advising on a hard session or a rest day.",
			Schema:      `{"type":"object","properties":{"date":{"type":"string","description":"YYYY-MM-DD; defaults to today"},"tz":{"type":"string","description":"IANA timezone; optional"},"days":{"type":"integer"}}}`,
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					Date string `json:"date"`
					TZ   string `json:"tz"`
					Days int    `json:"days"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Date != "" {
					q.Set("date", a.Date)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				if a.Days > 0 {
					q.Set("days", fmt.Sprintf("%d", a.Days))
				}
				return HTTPCall{Method: "GET", Path: "/context/recovery", Query: q}, nil
			},
		},
	}
}

// coachWriteConfirmSpecs are the consequential coaching writes — training,
// goal-override, logging, and destructive — that pause for human confirmation
// before dispatch. Each carries a Format formatter so the confirmation card
// shows an honest, code-composed preview (D6).
func coachWriteConfirmSpecs() []Spec {
	return []Spec{
		{
			Name:        "log_workout",
			Description: "Record a workout (completed by default, or status=planned for a scheduled session). source is 'manual' for coach-entered sessions. started_at/ended_at are RFC3339. Optionally set sport, name, kcal_burned, tss, rpe (1-10), distance_m, notes.",
			Schema:      `{"type":"object","properties":{"source":{"type":"string","enum":["manual","garmin","other"]},"sport":{"type":"string","enum":["run","bike","swim","strength","other"]},"status":{"type":"string","enum":["completed","planned"]},"name":{"type":"string"},"started_at":{"type":"string"},"ended_at":{"type":"string"},"kcal_burned":{"type":"number"},"tss":{"type":"number"},"rpe":{"type":"integer"},"distance_m":{"type":"number"},"notes":{"type":"string"}},"required":["source","sport","started_at","ended_at"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/workouts", in)
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					Sport  string `json:"sport"`
					Status string `json:"status"`
					Name   string `json:"name"`
					Date   string `json:"started_at"`
				}
				_ = json.Unmarshal(in, &a)
				verb := "Log"
				if a.Status == "planned" {
					verb = "Schedule"
				}
				label := strings.TrimSpace(a.Sport + " workout")
				if a.Name != "" {
					label = a.Name
				}
				if d := dateOnly(a.Date); d != "" {
					return fmt.Sprintf("%s %s on %s", verb, label, d)
				}
				return fmt.Sprintf("%s %s", verb, label)
			},
		},
		{
			Name:        "patch_workout",
			Description: "Update an existing workout by id — corrected metrics (kcal_burned, tss, avg_hr, distance_m), name/notes, rpe (1-10), or promote status planned→completed. Only supplied fields change.",
			Schema:      `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"notes":{"type":"string"},"kcal_burned":{"type":"number"},"tss":{"type":"number"},"rpe":{"type":"integer"},"distance_m":{"type":"number"},"status":{"type":"string","enum":["planned","completed"]}},"required":["id"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return PathParamPassthrough(in, "id", "PATCH", "/workouts/")
			},
			Format: func(in json.RawMessage) string { return "Update workout " + shortID(idField(in)) },
		},
		{
			Name:        "delete_workout",
			Description: "Delete a workout by id. Irreversible.",
			Schema:      `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				id := idField(in)
				if id == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/workouts/" + url.PathEscape(id)}, nil
			},
			Format: func(in json.RawMessage) string { return "Delete workout " + shortID(idField(in)) },
		},
		{
			Name:        "log_weight",
			Description: "Record a body-weight entry. weight_kg required; logged_at RFC3339 (defaults to now). Optionally body_fat_pct, muscle_mass_kg.",
			Schema:      `{"type":"object","properties":{"weight_kg":{"type":"number"},"logged_at":{"type":"string"},"body_fat_pct":{"type":"number"},"muscle_mass_kg":{"type":"number"}},"required":["weight_kg"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/weight", in)
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					WeightKg *float64 `json:"weight_kg"`
				}
				_ = json.Unmarshal(in, &a)
				if a.WeightKg != nil {
					return fmt.Sprintf("Log weight %.1f kg", *a.WeightKg)
				}
				return "Log weight"
			},
		},
		{
			Name:        "log_hydration",
			Description: "Record a hydration (fluid intake) entry in millilitres. quantity_ml required; logged_at RFC3339 (defaults to now).",
			Schema:      `{"type":"object","properties":{"quantity_ml":{"type":"number"},"logged_at":{"type":"string"}},"required":["quantity_ml"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/hydration", in)
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					QuantityMl *float64 `json:"quantity_ml"`
				}
				_ = json.Unmarshal(in, &a)
				if a.QuantityMl != nil {
					return fmt.Sprintf("Log %.0f ml hydration", *a.QuantityMl)
				}
				return "Log hydration"
			},
		},
		{
			Name:        "log_meal_freeform",
			Description: "Log a meal from a free-text description; the server estimates nutriments. Use when the user describes what they ate rather than picking a library recipe. text required; logged_at RFC3339 (defaults to now), optional quantity_g, meal_type.",
			Schema:      `{"type":"object","properties":{"text":{"type":"string"},"quantity_g":{"type":"number"},"logged_at":{"type":"string"},"meal_type":{"type":"string","enum":["breakfast","lunch","dinner","snack"]}},"required":["text"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return Passthrough("POST", "/meals/freeform", in)
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					Text string `json:"text"`
				}
				_ = json.Unmarshal(in, &a)
				if t := strings.TrimSpace(a.Text); t != "" {
					return "Log meal: " + truncate(t, 60)
				}
				return "Log meal"
			},
		},
		{
			Name:        "set_daily_goal_override",
			Description: "Set the nutrition goal override for a specific date (full-replace for that day; wins over the phase template). date required (YYYY-MM-DD); supply the goal ranges to apply, e.g. {\"kcal\":{\"min\":2800},\"carbs_g\":{\"min\":600}}.",
			Schema:      `{"type":"object","properties":{"date":{"type":"string"},"kcal":{"type":"object"},"protein_g":{"type":"object"},"carbs_g":{"type":"object"},"fat_g":{"type":"object"}},"required":["date"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return PathParamPassthrough(in, "date", "PUT", "/goals/overrides/")
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					Date string `json:"date"`
				}
				_ = json.Unmarshal(in, &a)
				if a.Date != "" {
					return "Set goal override for " + a.Date
				}
				return "Set goal override"
			},
		},
		{
			Name:        "delete_daily_goal_override",
			Description: "Remove the nutrition goal override for a specific date, reverting that day to the phase template / default. date required (YYYY-MM-DD).",
			Schema:      `{"type":"object","properties":{"date":{"type":"string"}},"required":["date"]}`,
			Tier:        TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a struct {
					Date string `json:"date"`
				}
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.Date == "" {
					return HTTPCall{}, fmt.Errorf("date is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/goals/overrides/" + url.PathEscape(a.Date)}, nil
			},
			Format: func(in json.RawMessage) string {
				var a struct {
					Date string `json:"date"`
				}
				_ = json.Unmarshal(in, &a)
				if a.Date != "" {
					return "Clear goal override for " + a.Date
				}
				return "Clear goal override"
			},
		},
	}
}

// idField pulls the "id" string from a tool input, "" if absent.
func idField(in json.RawMessage) string {
	var a struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(in, &a)
	return a.ID
}

// shortID renders a compact form of a uuid-ish id for previews.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	if id == "" {
		return "(unknown)"
	}
	return id
}

// dateOnly returns the YYYY-MM-DD prefix of an RFC3339 timestamp, or "".
func dateOnly(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
