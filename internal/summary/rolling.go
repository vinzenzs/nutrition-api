package summary

import (
	"context"
	"fmt"
	"time"

	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/meals"
)

// Window-day bounds for /summary/rolling. Beyond 30 the trailing-trend framing
// breaks down and `range_summary` is the right tool.
const (
	RollingMinWindowDays = 2
	RollingMaxWindowDays = 30
)

// RollingDay is one calendar-day row inside the response. `has_data`
// distinguishes "user logged nothing on this day" from "user logged a meal
// whose computed totals happen to be zero."
type RollingDay struct {
	Date    string `json:"date"`
	Totals  Totals `json:"totals"`
	HasData bool   `json:"has_data"`
}

// Rolling is the response shape for GET /summary/rolling.
//
// The averaging divisor (`days_with_data`) is exposed alongside `total_days`
// so a sparse window is loud — averages reflect what the user actually
// logged, not the total elapsed days. See add-rolling-window-summaries.
type Rolling struct {
	AnchorDate       string       `json:"anchor_date"`
	WindowDays       int          `json:"window_days"`
	TZ               string       `json:"tz"`
	Averages         Totals       `json:"averages"`
	DaysWithData     int          `json:"days_with_data"`
	TotalDays        int          `json:"total_days"`
	Days             []RollingDay `json:"days"`
	Adherence        Adherence    `json:"adherence,omitempty"`
	GoalSource       string       `json:"goal_source,omitempty"`
	PhaseName        string       `json:"phase_name,omitempty"`
}

// RollingParams scopes a rolling summary request. The handler validates
// AnchorDate / WindowDays / Loc; this layer assumes them in range.
type RollingParams struct {
	AnchorDate time.Time
	WindowDays int
	Loc        *time.Location
}

// RollingFor returns the trailing-window aggregate of nutrition totals as of
// p.AnchorDate. The window is [anchor - (window_days - 1), anchor], inclusive
// in p.Loc. Averages are computed over days where at least one meal was
// logged; days with no meals still appear in the response with has_data=false
// but do NOT contribute to the divisor.
func (s *Service) RollingFor(ctx context.Context, p RollingParams) (*Rolling, error) {
	startDate := p.AnchorDate.AddDate(0, 0, -(p.WindowDays - 1))
	startMidnight := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, p.Loc)
	endMidnight := time.Date(p.AnchorDate.Year(), p.AnchorDate.Month(), p.AnchorDate.Day(), 0, 0, 0, 0, p.Loc).Add(24 * time.Hour)

	entries, err := s.mealsRepo.List(ctx, meals.ListParams{From: startMidnight.UTC(), To: endMidnight.UTC()})
	if err != nil {
		return nil, fmt.Errorf("list rolling entries: %w", err)
	}

	buckets := map[string][]*meals.MealEntry{}
	for _, e := range entries {
		key := e.LoggedAt.In(p.Loc).Format("2006-01-02")
		buckets[key] = append(buckets[key], e)
	}

	out := &Rolling{
		AnchorDate: p.AnchorDate.Format("2006-01-02"),
		WindowDays: p.WindowDays,
		TZ:         p.Loc.String(),
		TotalDays:  p.WindowDays,
		Days:       make([]RollingDay, 0, p.WindowDays),
	}

	// Walk the enumerated calendar dates ascending. Use AddDate on the local
	// midnight to handle DST correctly (24h would skip an hour on spring-forward).
	cursor := startMidnight
	var (
		summedTotals Totals
		// Per-nutriment "any contributor" trackers for nullable micros so the
		// window average preserves NULL semantics: if no day in the window
		// supplied iron, the average's iron stays nil — not zero.
		sawIron, sawCa, sawVitD, sawB12, sawVitC, sawMg, sawK, sawZn bool
	)
	for i := 0; i < p.WindowDays; i++ {
		date := cursor.Format("2006-01-02")
		dayEntries := buckets[date]
		dayTotals := sumEntries(dayEntries)
		hasData := len(dayEntries) > 0

		if hasData {
			out.DaysWithData++
			accumulate(&summedTotals, dayTotals, &sawIron, &sawCa, &sawVitD, &sawB12, &sawVitC, &sawMg, &sawK, &sawZn)
		}

		out.Days = append(out.Days, RollingDay{
			Date:    date,
			Totals:  dayTotals.rounded(),
			HasData: hasData,
		})
		cursor = cursor.AddDate(0, 0, 1)
	}

	// Window averages — divisor is days_with_data only.
	var avg Totals
	if out.DaysWithData > 0 {
		avg = divideTotals(summedTotals, out.DaysWithData, sawIron, sawCa, sawVitD, sawB12, sawVitC, sawMg, sawK, sawZn)
	}
	out.Averages = avg.rounded()

	// Adherence resolves against the goal at the anchor (honoring overrides
	// on that exact date). The unrounded `avg` is what gets compared so the
	// borderline decision is honest; newEntry rounds for presentation.
	effective, source, phaseName, err := s.goalsResolver.EffectiveFor(ctx, p.AnchorDate)
	if err != nil {
		return nil, err
	}
	out.GoalSource = string(source)
	if source == goals.GoalSourcePhaseTemplate {
		out.PhaseName = phaseName
	}
	adherence := computeAdherenceFor(avg, effective, out.DaysWithData > 0)
	if len(adherence) > 0 {
		out.Adherence = adherence
	}

	return out, nil
}

// accumulate folds one day's totals into the running window sum, flipping the
// "saw any contributor" tracker per nullable nutriment so the window average
// preserves NULL when no day supplied a measurement.
func accumulate(acc *Totals, day Totals, sIron, sCa, sVitD, sB12, sVitC, sMg, sK, sZn *bool) {
	acc.Kcal += day.Kcal
	acc.ProteinG += day.ProteinG
	acc.CarbsG += day.CarbsG
	acc.FatG += day.FatG
	acc.FiberG += day.FiberG
	acc.SugarG += day.SugarG
	acc.SaltG += day.SaltG
	addNullable(&acc.IronMg, day.IronMg, sIron)
	addNullable(&acc.CalciumMg, day.CalciumMg, sCa)
	addNullable(&acc.VitaminDMcg, day.VitaminDMcg, sVitD)
	addNullable(&acc.VitaminB12Mcg, day.VitaminB12Mcg, sB12)
	addNullable(&acc.VitaminCMg, day.VitaminCMg, sVitC)
	addNullable(&acc.MagnesiumMg, day.MagnesiumMg, sMg)
	addNullable(&acc.PotassiumMg, day.PotassiumMg, sK)
	addNullable(&acc.ZincMg, day.ZincMg, sZn)
}

func addNullable(acc **float64, day *float64, saw *bool) {
	if day == nil {
		return
	}
	*saw = true
	if *acc == nil {
		v := *day
		*acc = &v
		return
	}
	**acc += *day
}

func divideTotals(t Totals, n int, sIron, sCa, sVitD, sB12, sVitC, sMg, sK, sZn bool) Totals {
	div := float64(n)
	out := Totals{
		Kcal:     t.Kcal / div,
		ProteinG: t.ProteinG / div,
		CarbsG:   t.CarbsG / div,
		FatG:     t.FatG / div,
		FiberG:   t.FiberG / div,
		SugarG:   t.SugarG / div,
		SaltG:    t.SaltG / div,
	}
	out.IronMg = divPtr(t.IronMg, div, sIron)
	out.CalciumMg = divPtr(t.CalciumMg, div, sCa)
	out.VitaminDMcg = divPtr(t.VitaminDMcg, div, sVitD)
	out.VitaminB12Mcg = divPtr(t.VitaminB12Mcg, div, sB12)
	out.VitaminCMg = divPtr(t.VitaminCMg, div, sVitC)
	out.MagnesiumMg = divPtr(t.MagnesiumMg, div, sMg)
	out.PotassiumMg = divPtr(t.PotassiumMg, div, sK)
	out.ZincMg = divPtr(t.ZincMg, div, sZn)
	return out
}

func divPtr(v *float64, div float64, saw bool) *float64 {
	if !saw || v == nil {
		return nil
	}
	out := *v / div
	return &out
}

