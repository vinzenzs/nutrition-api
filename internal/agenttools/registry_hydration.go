package agenttools

import (
	"encoding/json"
	"net/url"
)

// Hydration MCP tools — fluid-intake logging, listing, patching, deletion, and
// the volume-only daily summary. Ported from internal/mcpserver onto the shared
// registry (unify-mcp-tool-registry). These are MCP-only; the descriptions and
// arg structs are byte-identical to the prior bespoke registrations so the
// announced schema is unchanged.

func init() { registerMCPDomain(hydrationSpecs()) }

// LogHydrationArgs is the input to log_hydration.
type LogHydrationArgs struct {
	QuantityMl     float64 `json:"quantity_ml" jsonschema:"volume drunk in millilitres; must be greater than zero"`
	LoggedAt       string  `json:"logged_at" jsonschema:"when the drink was consumed, RFC 3339 timestamp"`
	Note           string  `json:"note,omitempty" jsonschema:"optional free-text beverage context (e.g. 'water', 'iced coffee', 'electrolytes')"`
	WorkoutID      string  `json:"workout_id,omitempty" jsonschema:"optional UUID of an existing workout to link this hydration entry to. The link is metadata; workout_fueling_summary aggregates by logged_at time-window matching, not by this tag."`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args. To log the same drink twice, pass a distinct key."`
}

// ListHydrationArgs is the input to list_hydration.
type ListHydrationArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound on logged_at"`
	To   string `json:"to" jsonschema:"exclusive RFC 3339 upper bound on logged_at; max 92 days from 'from'"`
}

// PatchHydrationArgs is the input to patch_hydration.
type PatchHydrationArgs struct {
	ID         string   `json:"id" jsonschema:"the id of the hydration entry to update"`
	QuantityMl *float64 `json:"quantity_ml,omitempty" jsonschema:"new volume in millilitres; must be greater than zero if supplied"`
	LoggedAt   *string  `json:"logged_at,omitempty" jsonschema:"new RFC 3339 timestamp"`
	Note       *string  `json:"note,omitempty" jsonschema:"new beverage note"`
	// WorkoutID supports the empty-string sentinel: \"<uuid>\" sets, \"\" clears, missing leaves unchanged.
	WorkoutID      *string `json:"workout_id,omitempty" jsonschema:"new workout link: \"<uuid>\" sets, \"\" clears, omit to leave unchanged"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteHydrationArgs is the input to delete_hydration.
type DeleteHydrationArgs struct {
	ID             string `json:"id" jsonschema:"the id of the hydration entry to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DailyHydrationSummaryArgs is the input to daily_hydration_summary.
type DailyHydrationSummaryArgs struct {
	Date string `json:"date" jsonschema:"calendar date in YYYY-MM-DD"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func hydrationSpecs() []Spec {
	return []Spec{
		{
			Name: "log_hydration",
			Description: "Record a volume of fluid the user drank at a specific time. The optional `note` " +
				"carries beverage context (e.g. 'water', 'iced coffee', 'electrolytes'). Use this for ANY " +
				"drink — water, coffee, sports drinks. For beverages with nutriments (Coke, juice), " +
				"additionally log via log_meal_freeform with the macros.",
			SchemaType: LogHydrationArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogHydrationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					QuantityMl float64 `json:"quantity_ml"`
					LoggedAt   string  `json:"logged_at"`
					Note       string  `json:"note,omitempty"`
					WorkoutID  string  `json:"workout_id,omitempty"`
				}{
					QuantityMl: a.QuantityMl,
					LoggedAt:   a.LoggedAt,
					Note:       a.Note,
					WorkoutID:  a.WorkoutID,
				})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/hydration", Body: body}, nil
			},
		},
		{
			Name: "list_hydration",
			Description: "List hydration entries whose logged_at falls within the half-open [from, to) " +
				"RFC 3339 window. Window is capped at 92 days. Use daily_hydration_summary instead when " +
				"you want a one-day total without paging through individual entries.",
			SchemaType: ListHydrationArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListHydrationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/hydration", Query: q}, nil
			},
		},
		{
			Name:        "patch_hydration",
			Description: "Partially update an existing hydration entry. Only supplied fields are changed.",
			SchemaType:  PatchHydrationArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PatchHydrationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if a.QuantityMl != nil {
					payload["quantity_ml"] = *a.QuantityMl
				}
				if a.LoggedAt != nil {
					payload["logged_at"] = *a.LoggedAt
				}
				if a.Note != nil {
					payload["note"] = *a.Note
				}
				if a.WorkoutID != nil {
					payload["workout_id"] = *a.WorkoutID
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/hydration/" + url.PathEscape(a.ID), Body: body}, nil
			},
		},
		{
			Name:        "delete_hydration",
			Description: "Delete a hydration entry. Returns an empty result on success.",
			SchemaType:  DeleteHydrationArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteHydrationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/hydration/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "daily_hydration_summary",
			Description: "Return the total ml and per-entry list for one calendar day. This is the " +
				"volume-only summary — separate from daily_summary, which is the nutrient-only summary. " +
				"Combine both when the user asks 'how did I do today?'",
			SchemaType: DailyHydrationSummaryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DailyHydrationSummaryArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/summary/hydration/daily", Query: q}, nil
			},
		},
	}
}
