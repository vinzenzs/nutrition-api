package mcpserver

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDailySummary_BuildsCorrectQuery(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleDailySummary(context.Background(), c, DailySummaryArgs{Date: "2026-06-06", TZ: "Europe/Berlin"})
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/summary/daily", rec.path)
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "2026-06-06", values.Get("date"))
	assert.Equal(t, "Europe/Berlin", values.Get("tz"))
}

func TestDailySummary_OmitsTZWhenEmpty(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleDailySummary(context.Background(), c, DailySummaryArgs{Date: "2026-06-06"})
	assert.NotContains(t, rec.rawQS, "tz=")
}

func TestRangeSummary_BuildsCorrectQuery(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleRangeSummary(context.Background(), c, RangeSummaryArgs{From: "2026-06-01", To: "2026-06-07", TZ: "UTC"})
	assert.Equal(t, "/summary/range", rec.path)
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "2026-06-01", values.Get("from"))
	assert.Equal(t, "2026-06-07", values.Get("to"))
	assert.Equal(t, "UTC", values.Get("tz"))
}

func TestRangeSummary_TooLargeErrorIsForwarded(t *testing.T) {
	c, _ := newRecordingClient(t, 400, `{"error":"range_too_large","max_days":92}`)
	r := handleRangeSummary(context.Background(), c, RangeSummaryArgs{From: "2026-01-01", To: "2026-12-31"})
	assert.True(t, r.IsError)
	tc := extractText(t, r)
	assert.True(t, strings.Contains(tc, "range_too_large"))
}

func TestDailySummary_MealTypeForwardedWhenPresent(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleDailySummary(context.Background(), c, DailySummaryArgs{Date: "2026-06-06", MealType: "breakfast"})
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "breakfast", values.Get("meal_type"))
}

func TestDailySummary_MealTypeOmittedWhenEmpty(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleDailySummary(context.Background(), c, DailySummaryArgs{Date: "2026-06-06"})
	assert.NotContains(t, rec.rawQS, "meal_type=")
}

func TestRangeSummary_GroupByForwardedWhenPresent(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleRangeSummary(context.Background(), c, RangeSummaryArgs{
		From: "2026-06-01", To: "2026-06-07", GroupBy: "meal_type",
	})
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "meal_type", values.Get("group_by"))
}

func TestRangeSummary_GroupByOmittedWhenEmpty(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleRangeSummary(context.Background(), c, RangeSummaryArgs{From: "2026-06-01", To: "2026-06-07"})
	assert.NotContains(t, rec.rawQS, "group_by=")
}

// ----- rolling_summary -----

func TestRollingSummary_BuildsCorrectQuery(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleRollingSummary(context.Background(), c, RollingSummaryArgs{
		AnchorDate: "2026-06-08", WindowDays: 7, TZ: "Europe/Berlin",
	})
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/summary/rolling", rec.path)
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "2026-06-08", values.Get("anchor_date"))
	assert.Equal(t, "7", values.Get("window_days"))
	assert.Equal(t, "Europe/Berlin", values.Get("tz"))
}

func TestRollingSummary_OmitsTZWhenEmpty(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleRollingSummary(context.Background(), c, RollingSummaryArgs{
		AnchorDate: "2026-06-08", WindowDays: 7,
	})
	assert.NotContains(t, rec.rawQS, "tz=")
}

func TestRollingSummary_400Forwarded(t *testing.T) {
	c, _ := newRecordingClient(t, 400, `{"error":"window_days_invalid","range":{"min":2,"max":30}}`)
	r := handleRollingSummary(context.Background(), c, RollingSummaryArgs{
		AnchorDate: "2026-06-08", WindowDays: 1,
	})
	assert.True(t, r.IsError)
	tc := extractText(t, r)
	assert.True(t, strings.Contains(tc, "window_days_invalid"))
}

func TestRollingSummary_SparseWindowPassesThroughVerbatim(t *testing.T) {
	body := `{"anchor_date":"2026-06-08","window_days":7,"days_with_data":2,"total_days":7,"averages":{"kcal":1234.5}}`
	c, _ := newRecordingClient(t, 200, body)
	r := handleRollingSummary(context.Background(), c, RollingSummaryArgs{
		AnchorDate: "2026-06-08", WindowDays: 7,
	})
	assert.False(t, r.IsError)
	assert.Equal(t, body, extractText(t, r),
		"wrapper must pass the body byte-for-byte without injecting any sparse-window warning")
}

// ----- protein_distribution -----

func TestProteinDistribution_BuildsCorrectQuery(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	bw := 72.5
	_ = handleProteinDistribution(context.Background(), c, ProteinDistributionArgs{
		Date: "2026-06-09", TZ: "Europe/Berlin", BodyWeightKg: &bw,
	})
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/summary/protein-distribution", rec.path)
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "2026-06-09", values.Get("date"))
	assert.Equal(t, "Europe/Berlin", values.Get("tz"))
	assert.Equal(t, "72.5", values.Get("body_weight_kg"))
}

func TestProteinDistribution_OmitsOptionalsWhenUnset(t *testing.T) {
	c, rec := newRecordingClient(t, 200, `{}`)
	_ = handleProteinDistribution(context.Background(), c, ProteinDistributionArgs{Date: "2026-06-09"})
	values, err := url.ParseQuery(rec.rawQS)
	assert.NoError(t, err)
	assert.Equal(t, "2026-06-09", values.Get("date"))
	assert.NotContains(t, rec.rawQS, "tz=")
	assert.NotContains(t, rec.rawQS, "body_weight_kg=")
}

func TestProteinDistribution_400Forwarded(t *testing.T) {
	c, _ := newRecordingClient(t, 400, `{"error":"weight_data_missing"}`)
	r := handleProteinDistribution(context.Background(), c, ProteinDistributionArgs{Date: "2026-06-09"})
	assert.True(t, r.IsError)
	tc := extractText(t, r)
	assert.True(t, strings.Contains(tc, "weight_data_missing"))
}

func TestProteinDistribution_BodyPassesThroughVerbatim(t *testing.T) {
	body := `{"date":"2026-06-09","body_weight_kg":72.5,"mps_threshold_g":21.8,"meal_count":4,"mps_effective_meal_count":3}`
	c, _ := newRecordingClient(t, 200, body)
	r := handleProteinDistribution(context.Background(), c, ProteinDistributionArgs{Date: "2026-06-09"})
	assert.False(t, r.IsError)
	assert.Equal(t, body, extractText(t, r))
}
