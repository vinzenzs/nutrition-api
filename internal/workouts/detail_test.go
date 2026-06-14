package workouts_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// A run body carrying scalar + zone columns and two nested splits. external_id
// is garmin so it flows the reconcile path; with no planned candidate it lands
// as a standalone completed row with its children.
func runWithSplitsBody(extID string, start time.Time, splits string) string {
	return fmt.Sprintf(`{
        "external_id":%q,"source":"garmin","sport":"run",
        "started_at":%q,"ended_at":%q,
        "kcal_burned":600,"avg_hr":150,
        "elevation_gain_m":120.0,"normalized_power_w":245,"intensity_factor":0.82,
        "max_hr":176,"aerobic_te":3.4,
        "secs_in_zone_1":300,"secs_in_zone_3":1500,"secs_in_zone_5":100,
        "humidity_pct":72.0,"wind_speed_mps":3.5,
        "splits":%s
    }`, extID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339), splits)
}

const twoSplits = `[
    {"split_index":0,"distance_m":3000.0,"duration_s":900.0,"avg_hr":148,"avg_speed_mps":3.333,"elevation_gain_m":60.0},
    {"split_index":1,"distance_m":3000.0,"duration_s":900.0,"avg_hr":152,"avg_speed_mps":3.305,"elevation_gain_m":60.0}
]`

// ----- 6.3 nested write, persistence, list/get divergence -----

func TestDetail_PostReturnsScalarZoneAndSplits(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:d1", at(7), twoSplits))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	require.NotNil(t, w.ElevationGainM)
	assert.InDelta(t, 120.0, *w.ElevationGainM, 0.001)
	require.NotNil(t, w.IntensityFactor)
	assert.InDelta(t, 0.82, *w.IntensityFactor, 0.001, "2dp precision preserved (not Round1'd)")
	require.NotNil(t, w.SecsInZone3)
	assert.Equal(t, 1500, *w.SecsInZone3)
	require.NotNil(t, w.HumidityPct)
	assert.InDelta(t, 72.0, *w.HumidityPct, 0.001)

	require.Len(t, w.Splits, 2)
	assert.Equal(t, 0, w.Splits[0].SplitIndex)
	require.NotNil(t, w.Splits[0].AvgSpeedMPS)
	assert.InDelta(t, 3.333, *w.Splits[0].AvgSpeedMPS, 0.0001, "3dp speed precision preserved")
	assert.Equal(t, 1, w.Splits[1].SplitIndex)

	// Single-get returns the same nested detail, ordered by index.
	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, got.Splits, 2)
	assert.Equal(t, 0, got.Splits[0].SplitIndex)
	assert.Equal(t, 1, got.Splits[1].SplitIndex)
}

func TestDetail_StrengthSetsRoundTrip(t *testing.T) {
	f := setup(t)
	body := fmt.Sprintf(`{
        "external_id":"garmin:lift1","source":"garmin","sport":"strength",
        "started_at":%q,"ended_at":%q,"kcal_burned":320,
        "sets":[
            {"set_index":0,"exercise_name":"BENCH_PRESS","exercise_category":"BENCH_PRESS","reps":12,"weight_kg":40.0,"duration_s":45.0},
            {"set_index":1,"exercise_name":"BENCH_PRESS","exercise_category":"BENCH_PRESS","reps":10,"weight_kg":42.5,"duration_s":40.0}
        ]
    }`, at(17).Format(time.RFC3339), at(18).Format(time.RFC3339))
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	require.Len(t, w.Sets, 2)
	assert.Equal(t, 0, w.Sets[0].SetIndex)
	require.NotNil(t, w.Sets[0].WeightKg)
	assert.InDelta(t, 40.0, *w.Sets[0].WeightKg, 0.001)
	require.NotNil(t, w.Sets[1].WeightKg)
	assert.InDelta(t, 42.5, *w.Sets[1].WeightKg, 0.001, "2dp weight precision preserved")
	assert.Empty(t, w.Splits, "strength session carries no splits")
}

func TestDetail_ResyncReplacesChildren(t *testing.T) {
	f := setup(t)
	// First sync: three splits.
	threeSplits := `[
        {"split_index":0,"distance_m":2000.0},
        {"split_index":1,"distance_m":2000.0},
        {"split_index":2,"distance_m":2000.0}
    ]`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:rs", at(7), threeSplits))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.Len(t, w.Splits, 3)

	// Re-sync the same external_id with two splits → replaced, not accumulated.
	rec = doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:rs", at(7), twoSplits))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resync := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, w.ID, resync.ID, "same row updated")
	require.Len(t, resync.Splits, 2, "children replaced, not duplicated")

	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Len(t, got.Splits, 2)
}

func TestDetail_ResyncWithoutSplitsLeavesChildrenUntouched(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:keep", at(7), twoSplits))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())
	require.Len(t, w.Splits, 2)

	// A re-sync that carries no splits key at all must not wipe existing children
	// (a missing Garmin detail fetch should not clobber good data).
	body := fmt.Sprintf(`{"external_id":"garmin:keep","source":"garmin","sport":"run","started_at":%q,"ended_at":%q,"kcal_burned":610}`,
		at(7).Format(time.RFC3339), at(8).Format(time.RFC3339))
	rec = doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Len(t, got.Splits, 2, "absent splits key left children in place")
}

func TestDetail_ListCarriesScalarZoneButNotNested(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:l1", at(7), twoSplits))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	from := reconDay.Add(-24 * time.Hour).Format(time.RFC3339)
	to := reconDay.Add(36 * time.Hour).Format(time.RFC3339)
	rec = doReq(t, f.r, http.MethodGet, "/workouts?from="+from+"&to="+to, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Raw-body assertions: scalar/zone present, nested arrays absent.
	body := rec.Body.String()
	assert.Contains(t, body, "elevation_gain_m")
	assert.Contains(t, body, "secs_in_zone_3")
	assert.NotContains(t, body, `"splits"`, "list response omits nested detail")
	assert.NotContains(t, body, `"sets"`)

	var out struct {
		Workouts []workouts.Workout `json:"workouts"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Workouts, 1)
	assert.Empty(t, out.Workouts[0].Splits)
	require.NotNil(t, out.Workouts[0].SecsInZone3)
}

func TestDetail_BulkInvalidChildFailsOnlyItsItem(t *testing.T) {
	f := setup(t)
	// item 0 valid (two splits); item 1 has a duplicate split_index (invalid).
	good := runWithSplitsBody("garmin:bulkok", at(7), twoSplits)
	bad := runWithSplitsBody("garmin:bulkbad", at(9),
		`[{"split_index":0},{"split_index":0}]`)
	body := fmt.Sprintf(`{"workouts":[%s,%s]}`, good, bad)
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 2)
	assert.Equal(t, true, out.Results[0]["created"], "valid item persisted")
	assert.Equal(t, "split_invalid", out.Results[1]["error"], "invalid item carries its own error")

	// The valid item's children landed; the bad item never persisted.
	assert.Equal(t, 1, countDay(t, f, "completed"))
}

// ----- 6.4 reconcile-seam: detail attaches to the surviving reconciled row -----

func TestDetail_ReconcileAttachesToPlannedRowAndReplacesOnResync(t *testing.T) {
	f := setup(t)
	planned, planSlotID, _ := seedPlannedFromSlot(t, f, "run", at(18))

	// A garmin run with nested detail on the same local day → merges into the
	// planned row (planned→completed in place), not a second inserted row.
	rec := doReq(t, f.r, http.MethodPost, "/workouts", runWithSplitsBody("garmin:rec", at(7), twoSplits))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	merged := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, planned.ID, merged.ID, "merged into the planned row")
	require.NotNil(t, merged.PlanSlotID)
	assert.Equal(t, planSlotID, *merged.PlanSlotID, "plan link retained")
	require.Len(t, merged.Splits, 2, "detail attached to the surviving row")
	assert.Equal(t, 1, countDay(t, f, ""), "no duplicate row")

	// Scalar/zone columns landed on the reconciled row too.
	got, code := getWorkout(t, f, planned.ID)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.ElevationGainM)
	assert.InDelta(t, 120.0, *got.ElevationGainM, 0.001)
	require.NotNil(t, got.SecsInZone3)
	assert.Equal(t, 1500, *got.SecsInZone3)
	require.Len(t, got.Splits, 2)

	// Re-sync of the same activity replaces the children in place on the
	// reconciled row — no duplication across the reconcile + re-sync seam.
	rec = doReq(t, f.r, http.MethodPost, "/workouts",
		runWithSplitsBody("garmin:rec", at(7), `[{"split_index":0,"distance_m":6000.0}]`))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resync := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, planned.ID, resync.ID)
	require.Len(t, resync.Splits, 1, "children replaced in place on the reconciled row")
	assert.Equal(t, 1, countDay(t, f, ""), "still a single row")
}
