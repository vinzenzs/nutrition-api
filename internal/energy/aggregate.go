package energy

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// dateKey is YYYY-MM-DD in the requested TZ; canonical map key for day-bucket
// aggregation.
type dateKey string

const dateLayout = "2006-01-02"

// bucket holds per-day mutable totals before they're folded into a Day.
type bucket struct {
	meals             []*meals.MealEntry
	burnedKcal        float64
	missingWorkoutIDs []uuid.UUID
}

// buildDays returns one Day per calendar day in [from, to) interpreted in
// loc. Meals are bucketed by LoggedAt.In(loc) date; workouts by StartedAt.In(loc)
// date (Loucks rule: a midnight-spanning workout belongs to its *start* day).
//
// Days without any meals or workouts still appear in the response with zeros
// (complete_data: true — there are no missing-burn workouts on a day with no
// workouts).
func buildDays(from, to time.Time, loc *time.Location, mealsAll []*meals.MealEntry, workoutsAll []*workouts.Workout, ffmKg float64) []Day {
	buckets := map[dateKey]*bucket{}
	keyOf := func(t time.Time) dateKey {
		return dateKey(t.In(loc).Format(dateLayout))
	}

	for _, m := range mealsAll {
		k := keyOf(m.LoggedAt)
		b := buckets[k]
		if b == nil {
			b = &bucket{}
			buckets[k] = b
		}
		b.meals = append(b.meals, m)
	}
	for _, w := range workoutsAll {
		k := keyOf(w.StartedAt)
		b := buckets[k]
		if b == nil {
			b = &bucket{}
			buckets[k] = b
		}
		if w.KcalBurned != nil {
			b.burnedKcal += *w.KcalBurned
		} else {
			b.missingWorkoutIDs = append(b.missingWorkoutIDs, w.ID)
		}
	}

	// Enumerate every calendar day in [from, to) in loc — even ones with no
	// meals or workouts get a row.
	startDay := startOfDay(from.In(loc))
	endDay := endOfDay(to.In(loc))
	var days []Day
	for d := startDay; d.Before(endDay); d = d.AddDate(0, 0, 1) {
		k := dateKey(d.Format(dateLayout))
		b := buckets[k]
		days = append(days, dayFromBucket(string(k), b, ffmKg))
	}
	return days
}

// dayFromBucket folds a bucket into the final Day row. nil bucket → all zeros.
func dayFromBucket(date string, b *bucket, ffmKg float64) Day {
	var intake float64
	if b != nil {
		intake = summary.SumEntries(b.meals).Kcal
	}
	var burned float64
	var missing []uuid.UUID
	if b != nil {
		burned = b.burnedKcal
		missing = b.missingWorkoutIDs
	}
	if missing == nil {
		missing = []uuid.UUID{}
	}
	// Stable order on missing ids — workouts.List returns ASC by started_at
	// but we may have multiple workouts per day; sort by uuid string so the
	// response is deterministic across reruns.
	if len(missing) > 1 {
		sort.Slice(missing, func(i, j int) bool {
			return missing[i].String() < missing[j].String()
		})
	}

	ea := computeEA(intake, burned, ffmKg)
	return Day{
		Date:                  date,
		IntakeKcal:            numfmt.Round1(intake),
		ExerciseEnergyKcal:    numfmt.Round1(burned),
		EA:                    numfmt.Round1(ea),
		Band:                  classifyBand(ea),
		MissingBurnWorkoutIDs: missing,
		CompleteData:          len(missing) == 0,
	}
}

// startOfDay returns the local-midnight at or before t.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// endOfDay returns the local-midnight at or after t — used as the open
// upper bound for the day-enumeration loop.
//
// If t already sits exactly at local midnight, the loop should terminate
// AT t; otherwise it should run through the day containing t (the half-open
// window [from, to) includes the calendar day that to falls into when to is
// not at midnight; excludes it when to is exactly at midnight).
func endOfDay(t time.Time) time.Time {
	midnight := startOfDay(t)
	if t.Equal(midnight) {
		return midnight
	}
	return midnight.AddDate(0, 0, 1)
}
