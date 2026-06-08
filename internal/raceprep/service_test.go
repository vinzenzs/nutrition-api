package raceprep_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/raceprep"
)

// fixedToday is the simulated "now" used across these tests: 2026-06-07 UTC.
// Race dates in tests are chosen relative to this so race_date_in_past flips
// deterministically.
var fixedToday = time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

func TestPlanCarbLoad_DefaultsProduceFourEntries(t *testing.T) {
	race := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          race,
		BodyWeightKg:      70,
		DaysBefore:        3,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 2,
	}, fixedToday)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "2026-07-24", out.RaceDate)
	assert.Equal(t, 70.0, out.BodyWeightKg)
	assert.Equal(t, 3, out.Params.DaysBefore)
	assert.Equal(t, 10.0, out.Params.CarbsPerKgPerDay)
	assert.Equal(t, 2.0, out.Params.RaceDayCarbsPerKg)

	require.Len(t, out.Schedule, 4)
	// Ordered ascending by date.
	assert.Equal(t, "2026-07-21", out.Schedule[0].Date)
	assert.Equal(t, 3, out.Schedule[0].DaysBefore)
	assert.Equal(t, 700.0, out.Schedule[0].TargetCarbsG)
	assert.Equal(t, "carb-load day 1 of 3", out.Schedule[0].Rationale)

	assert.Equal(t, "2026-07-22", out.Schedule[1].Date)
	assert.Equal(t, 2, out.Schedule[1].DaysBefore)
	assert.Equal(t, 700.0, out.Schedule[1].TargetCarbsG)
	assert.Equal(t, "carb-load day 2 of 3", out.Schedule[1].Rationale)

	assert.Equal(t, "2026-07-23", out.Schedule[2].Date)
	assert.Equal(t, 1, out.Schedule[2].DaysBefore)
	assert.Equal(t, 700.0, out.Schedule[2].TargetCarbsG)
	assert.Equal(t, "carb-load day 3 of 3", out.Schedule[2].Rationale)

	// Race day.
	assert.Equal(t, "2026-07-24", out.Schedule[3].Date)
	assert.Equal(t, 0, out.Schedule[3].DaysBefore)
	assert.Equal(t, 140.0, out.Schedule[3].TargetCarbsG)
	assert.Equal(t, "race morning, pre-race meal ~3-4h before start", out.Schedule[3].Rationale)
}

func TestPlanCarbLoad_DaysBeforeZeroReturnsRaceDayOnly(t *testing.T) {
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg:      70,
		DaysBefore:        0,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 2,
	}, fixedToday)
	require.NoError(t, err)
	require.Len(t, out.Schedule, 1)
	assert.Equal(t, 0, out.Schedule[0].DaysBefore)
	assert.Equal(t, "2026-07-24", out.Schedule[0].Date)
	assert.Equal(t, 140.0, out.Schedule[0].TargetCarbsG)
}

func TestPlanCarbLoad_DaysBeforeSevenProducesEightEntries(t *testing.T) {
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg:      70,
		DaysBefore:        7,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 2,
	}, fixedToday)
	require.NoError(t, err)
	require.Len(t, out.Schedule, 8)
	assert.Equal(t, "2026-07-17", out.Schedule[0].Date)
	assert.Equal(t, "2026-07-24", out.Schedule[7].Date)
	assert.Equal(t, 0, out.Schedule[7].DaysBefore)
}

func TestPlanCarbLoad_RaceDayCarbsPerKgZeroProducesZeroTarget(t *testing.T) {
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg:      70,
		DaysBefore:        3,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 0,
	}, fixedToday)
	require.NoError(t, err)
	raceDay := out.Schedule[len(out.Schedule)-1]
	assert.Equal(t, 0, raceDay.DaysBefore)
	assert.Equal(t, 0.0, raceDay.TargetCarbsG)
	assert.Contains(t, raceDay.Rationale, "race morning")
}

func TestPlanCarbLoad_CustomParamsExactMath(t *testing.T) {
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg:      80,
		DaysBefore:        4,
		CarbsPerKgPerDay:  12,
		RaceDayCarbsPerKg: 3,
	}, fixedToday)
	require.NoError(t, err)
	require.Len(t, out.Schedule, 5)
	// Load days: 80 * 12 = 960
	for i := 0; i < 4; i++ {
		assert.Equal(t, 960.0, out.Schedule[i].TargetCarbsG)
	}
	// Race day: 80 * 3 = 240
	assert.Equal(t, 240.0, out.Schedule[4].TargetCarbsG)
}

func TestPlanCarbLoad_RaceDateTodayIsAccepted(t *testing.T) {
	out, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate:          fixedToday,
		BodyWeightKg:      70,
		DaysBefore:        3,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 2,
	}, fixedToday)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-07", out.Schedule[len(out.Schedule)-1].Date)
}

// ----- bounds rejection -----

func TestPlanCarbLoad_BodyWeightUnderMinRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 25, DaysBefore: 3, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrBodyWeightKgInvalid)
}

func TestPlanCarbLoad_BodyWeightOverMaxRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 250, DaysBefore: 3, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrBodyWeightKgInvalid)
}

func TestPlanCarbLoad_DaysBeforeNegativeRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: -1, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrDaysBeforeInvalid)
}

func TestPlanCarbLoad_DaysBeforeOverMaxRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: 8, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrDaysBeforeInvalid)
}

func TestPlanCarbLoad_CarbsPerKgUnderMinRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: 3, CarbsPerKgPerDay: 0.5, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrCarbsPerKgPerDayInvalid)
}

func TestPlanCarbLoad_CarbsPerKgOverMaxRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: 3, CarbsPerKgPerDay: 25, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrCarbsPerKgPerDayInvalid)
}

func TestPlanCarbLoad_RaceDayCarbsUnderMinRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: 3, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: -1,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrRaceDayCarbsPerKgInvalid)
}

func TestPlanCarbLoad_RaceDayCarbsOverMaxRejected(t *testing.T) {
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg: 70, DaysBefore: 3, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 11,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrRaceDayCarbsPerKgInvalid)
}

func TestPlanCarbLoad_RaceDateInPastRejected(t *testing.T) {
	yesterday := fixedToday.AddDate(0, 0, -1)
	_, err := raceprep.PlanCarbLoad(raceprep.CarbLoadParams{
		RaceDate: yesterday,
		BodyWeightKg: 70, DaysBefore: 3, CarbsPerKgPerDay: 10, RaceDayCarbsPerKg: 2,
	}, fixedToday)
	assert.ErrorIs(t, err, raceprep.ErrRaceDateInPast)
}

func TestService_PlanUsesInjectedClock(t *testing.T) {
	svc := raceprep.NewService(
		func() time.Time { return fixedToday },
		time.UTC,
	)
	out, err := svc.Plan(raceprep.CarbLoadParams{
		RaceDate:          time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		BodyWeightKg:      70,
		DaysBefore:        3,
		CarbsPerKgPerDay:  10,
		RaceDayCarbsPerKg: 2,
	})
	require.NoError(t, err)
	require.Len(t, out.Schedule, 4)
}
