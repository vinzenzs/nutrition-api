package summary

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/numfmt"
)

// rounded returns t with every nutrient field rounded to 1 dp for response
// presentation. Status math elsewhere uses the unrounded value.
func (t Totals) rounded() Totals {
	return Totals{
		Kcal:          numfmt.Round1(t.Kcal),
		ProteinG:      numfmt.Round1(t.ProteinG),
		CarbsG:        numfmt.Round1(t.CarbsG),
		FatG:          numfmt.Round1(t.FatG),
		FiberG:        numfmt.Round1(t.FiberG),
		SugarG:        numfmt.Round1(t.SugarG),
		SaltG:         numfmt.Round1(t.SaltG),
		IronMg:        numfmt.Round1Ptr(t.IronMg),
		CalciumMg:     numfmt.Round1Ptr(t.CalciumMg),
		VitaminDMcg:   numfmt.Round1Ptr(t.VitaminDMcg),
		VitaminB12Mcg: numfmt.Round1Ptr(t.VitaminB12Mcg),
		VitaminCMg:    numfmt.Round1Ptr(t.VitaminCMg),
		MagnesiumMg:   numfmt.Round1Ptr(t.MagnesiumMg),
		PotassiumMg:   numfmt.Round1Ptr(t.PotassiumMg),
		ZincMg:        numfmt.Round1Ptr(t.ZincMg),
	}
}

// Totals holds summed effective nutriments over a window. Macros are scalar
// floats (zero-default semantics). Micros are pointers so unsupplied days /
// nutrients can be omitted from the JSON instead of fake-zeroed.
type Totals struct {
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
	FiberG   float64 `json:"fiber_g"`
	SugarG   float64 `json:"sugar_g"`
	SaltG    float64 `json:"salt_g"`

	IronMg        *float64 `json:"iron_mg,omitempty"`
	CalciumMg     *float64 `json:"calcium_mg,omitempty"`
	VitaminDMcg   *float64 `json:"vitamin_d_mcg,omitempty"`
	VitaminB12Mcg *float64 `json:"vitamin_b12_mcg,omitempty"`
	VitaminCMg    *float64 `json:"vitamin_c_mg,omitempty"`
	MagnesiumMg   *float64 `json:"magnesium_mg,omitempty"`
	PotassiumMg   *float64 `json:"potassium_mg,omitempty"`
	ZincMg        *float64 `json:"zinc_mg,omitempty"`
}

// AdherenceEntry reports how close a single nutrient came to its goal. Actual
// is nullable to honestly distinguish "no data at all" (no_data status) from
// "logged 0 grams" (numeric actual with under/on/over status).
type AdherenceEntry struct {
	Actual   *float64    `json:"actual"`
	Target   goals.Range `json:"target"`
	DeltaPct *float64    `json:"delta_pct,omitempty"`
	Status   string      `json:"status"` // under | on | over | no_data
}

// Adherence is a per-nutrient breakdown. After unify-adherence-shape, every
// configured goal produces exactly one entry — empty days produce rows with
// status="no_data" and actual=null instead of being omitted.
type Adherence map[string]AdherenceEntry

// Daily is the response shape for GET /summary/daily.
type Daily struct {
	Date       string             `json:"date"`
	TZ         string             `json:"tz"`
	MealType   *string            `json:"meal_type,omitempty"`
	Totals     Totals             `json:"totals"`
	Entries    []*meals.MealEntry `json:"entries"`
	Adherence  Adherence          `json:"adherence,omitempty"`
	GoalSource string             `json:"goal_source,omitempty"`
	PhaseName  string             `json:"phase_name,omitempty"`
}

// RangeDay is one day of a range summary. Exactly one of Totals or ByMealType
// is populated, controlled by the group_by mode on the range request.
type RangeDay struct {
	Date       string             `json:"date"`
	Totals     *Totals            `json:"totals,omitempty"`
	ByMealType map[string]Totals  `json:"by_meal_type,omitempty"`
	Adherence  Adherence          `json:"adherence,omitempty"`
	GoalSource string             `json:"goal_source,omitempty"`
	PhaseName  string             `json:"phase_name,omitempty"`
}

// Range is the response shape for GET /summary/range.
type Range struct {
	From    string     `json:"from"`
	To      string     `json:"to"`
	TZ      string     `json:"tz"`
	GroupBy *string    `json:"group_by,omitempty"`
	Days    []RangeDay `json:"days"`
}

// Service computes summaries.
type Service struct {
	pool          *pgxpool.Pool
	mealsRepo     *meals.Repo
	goalsResolver *goals.Resolver

	// bodyWeightRepo is wired via SetBodyWeightRepo by callers that want the
	// protein-distribution endpoint to resolve weight from stored entries.
	// Optional — callers that don't set it must pass an explicit
	// body_weight_kg override on every request.
	bodyWeightRepo *bodyweight.Repo
}

func NewService(pool *pgxpool.Pool, mealsRepo *meals.Repo, resolver *goals.Resolver) *Service {
	return &Service{pool: pool, mealsRepo: mealsRepo, goalsResolver: resolver}
}

// DailyParams scopes a daily summary request.
type DailyParams struct {
	Date     time.Time
	Loc      *time.Location
	MealType *meals.MealType // nil means "all meal types"
}

// DailyFor computes the daily summary including (optionally) a meal_type
// filter and a goals-derived adherence block. When MealType is set, the
// returned Daily echoes it back and omits adherence.
func (s *Service) DailyFor(ctx context.Context, p DailyParams) (*Daily, error) {
	dayStart := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), 0, 0, 0, 0, p.Loc)
	dayEnd := dayStart.Add(24 * time.Hour)

	entries, err := s.mealsRepo.List(ctx, meals.ListParams{
		From:     dayStart.UTC(),
		To:       dayEnd.UTC(),
		MealType: p.MealType,
	})
	if err != nil {
		return nil, fmt.Errorf("list daily entries: %w", err)
	}
	if entries == nil {
		entries = []*meals.MealEntry{}
	}
	totals := sumEntries(entries)

	out := &Daily{
		Date:    p.Date.Format("2006-01-02"),
		TZ:      p.Loc.String(),
		Totals:  totals.rounded(),
		Entries: entries,
	}
	if p.MealType != nil {
		mt := string(*p.MealType)
		out.MealType = &mt
		return out, nil
	}

	// Resolve which goal set applies to this date: override > phase template > default.
	effective, source, phaseName, err := s.goalsResolver.EffectiveFor(ctx, p.Date)
	if err != nil {
		return nil, err
	}
	out.GoalSource = string(source)
	if source == goals.GoalSourcePhaseTemplate {
		out.PhaseName = phaseName
	}

	// Adherence runs against UNROUNDED totals so the status decision is
	// honest at the borderline; newEntry rounds the actual/target/delta_pct
	// fields it returns for presentation.
	adherence := computeAdherenceFor(totals, effective, len(entries) > 0)
	if len(adherence) > 0 {
		out.Adherence = adherence
	}
	return out, nil
}

// RangeParams scopes a range summary request.
type RangeParams struct {
	From    time.Time
	To      time.Time
	Loc     *time.Location
	GroupBy string // "" or "meal_type"
}

// RangeFor computes per-day totals across [from, to]. When GroupBy is
// "meal_type", each day reports per-meal-type totals (and no adherence).
func (s *Service) RangeFor(ctx context.Context, p RangeParams) (*Range, error) {
	startOfFrom := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	endOfTo := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc).Add(24 * time.Hour)

	entries, err := s.mealsRepo.List(ctx, meals.ListParams{From: startOfFrom.UTC(), To: endOfTo.UTC()})
	if err != nil {
		return nil, fmt.Errorf("list range entries: %w", err)
	}

	buckets := map[string][]*meals.MealEntry{}
	for _, e := range entries {
		local := e.LoggedAt.In(p.Loc)
		key := local.Format("2006-01-02")
		buckets[key] = append(buckets[key], e)
	}

	// Pre-fetch effective goals once for every day in the window if we'll
	// need adherence (i.e. not grouped). Resolution chain per day: per-date
	// override > phase template > singleton default.
	var (
		effective  map[string]*goals.Goals
		sources    map[string]goals.GoalSource
		phaseNames map[string]string
	)
	if p.GroupBy == "" {
		effective, sources, phaseNames, err = s.goalsResolver.EffectiveForRange(ctx, p.From, p.To)
		if err != nil {
			return nil, err
		}
	}

	out := &Range{
		From: p.From.Format("2006-01-02"),
		To:   p.To.Format("2006-01-02"),
		TZ:   p.Loc.String(),
	}
	if p.GroupBy != "" {
		gb := p.GroupBy
		out.GroupBy = &gb
	}

	for d := startOfFrom; !d.After(time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		dayEntries := buckets[key]

		day := RangeDay{Date: key}
		if p.GroupBy == "meal_type" {
			if len(dayEntries) == 0 {
				out.Days = append(out.Days, day)
				continue
			}
			byMT := map[string]Totals{}
			for _, mt := range []meals.MealType{meals.Breakfast, meals.Lunch, meals.Dinner, meals.Snack} {
				sub := filterByMealType(dayEntries, mt)
				if len(sub) == 0 {
					continue
				}
				byMT[string(mt)] = sumEntries(sub)
			}
			if len(byMT) > 0 {
				day.ByMealType = byMT
			}
		} else {
			totals := sumEntries(dayEntries)
			// Adherence sees unrounded totals; the response carries rounded ones.
			adherence := computeAdherenceFor(totals, effective[key], len(dayEntries) > 0)
			rounded := totals.rounded()
			day.Totals = &rounded
			if len(adherence) > 0 {
				day.Adherence = adherence
			}
			day.GoalSource = string(sources[key])
			if sources[key] == goals.GoalSourcePhaseTemplate {
				day.PhaseName = phaseNames[key]
			}
		}
		out.Days = append(out.Days, day)
	}

	return out, nil
}

func filterByMealType(entries []*meals.MealEntry, mt meals.MealType) []*meals.MealEntry {
	var out []*meals.MealEntry
	for _, e := range entries {
		if e.MealType != nil && *e.MealType == mt {
			out = append(out, e)
		}
	}
	return out
}

// computeAdherenceFor is the pure version that takes a goals object directly.
// After unify-adherence-shape, every configured goal produces exactly one
// entry; empty days produce rows with status="no_data".
func computeAdherenceFor(t Totals, g *goals.Goals, hasMeals bool) Adherence {
	if g == nil {
		return nil
	}
	out := Adherence{}

	// Macros are always-numeric in Totals (zero-default). On a day with no
	// logged meals at all, we promote that 0 to actual=nil + no_data so the
	// adherence row honestly says "no input"; on a day with meals where the
	// macro happened to sum to 0, the actual is the literal 0.
	addMacro := func(key string, actual float64, r *goals.Range) {
		if r == nil {
			return
		}
		if !hasMeals {
			out[key] = newEntry(nil, *r)
			return
		}
		a := actual
		out[key] = newEntry(&a, *r)
	}
	addMacro("kcal", t.Kcal, g.Kcal)
	addMacro("protein_g", t.ProteinG, g.ProteinG)
	addMacro("carbs_g", t.CarbsG, g.CarbsG)
	addMacro("fat_g", t.FatG, g.FatG)
	addMacro("fiber_g", t.FiberG, g.FiberG)
	addMacro("sugar_g", t.SugarG, g.SugarG)
	addMacro("salt_g", t.SaltG, g.SaltG)

	// Micros stay nullable in Totals — a nil actual means "no contributing
	// meal had a non-null column value for this nutrient," which is the
	// no_data case regardless of whether the day had other meals logged.
	addMicro := func(key string, actual *float64, r *goals.Range) {
		if r == nil {
			return
		}
		out[key] = newEntry(actual, *r)
	}
	addMicro("iron_mg", t.IronMg, g.IronMg)
	addMicro("calcium_mg", t.CalciumMg, g.CalciumMg)
	addMicro("vitamin_d_mcg", t.VitaminDMcg, g.VitaminDMcg)
	addMicro("vitamin_b12_mcg", t.VitaminB12Mcg, g.VitaminB12Mcg)
	addMicro("vitamin_c_mg", t.VitaminCMg, g.VitaminCMg)
	addMicro("magnesium_mg", t.MagnesiumMg, g.MagnesiumMg)
	addMicro("potassium_mg", t.PotassiumMg, g.PotassiumMg)
	addMicro("zinc_mg", t.ZincMg, g.ZincMg)

	return out
}

// newEntry builds one AdherenceEntry. Status uses the unrounded actual against
// the unrounded target (see spec "Status uses unrounded comparison"); the
// returned Actual / Target / DeltaPct fields are rounded to 1 dp for
// presentation.
func newEntry(actual *float64, target goals.Range) AdherenceEntry {
	roundedTarget := goals.Range{
		Min: numfmt.Round1Ptr(target.Min),
		Max: numfmt.Round1Ptr(target.Max),
	}
	entry := AdherenceEntry{
		Actual: numfmt.Round1Ptr(actual),
		Target: roundedTarget,
	}
	if actual == nil {
		entry.Status = "no_data"
		return entry
	}
	a := *actual
	switch {
	case target.Min != nil && a < *target.Min:
		entry.Status = "under"
	case target.Max != nil && a > *target.Max:
		entry.Status = "over"
	default:
		entry.Status = "on"
	}
	if ref := targetReference(target); ref != nil && *ref != 0 {
		pct := numfmt.Round1((a - *ref) / *ref * 100)
		entry.DeltaPct = &pct
	}
	return entry
}

// targetReference picks the comparison anchor for delta_pct: the midpoint of
// min and max when both are set, otherwise whichever bound exists.
func targetReference(t goals.Range) *float64 {
	switch {
	case t.Min != nil && t.Max != nil:
		mid := (*t.Min + *t.Max) / 2
		return &mid
	case t.Min != nil:
		return t.Min
	case t.Max != nil:
		return t.Max
	}
	return nil
}

// SumEntries aggregates meal-entry nutriments into a Totals. Exported so the
// workouts fueling aggregator can reuse the same shape (added by
// add-meal-workout-link).
func SumEntries(entries []*meals.MealEntry) Totals { return sumEntries(entries) }

func sumEntries(entries []*meals.MealEntry) Totals {
	var t Totals
	var (
		hasIron, hasCalcium, hasVitD, hasVitB12 bool
		hasVitC, hasMg, hasK, hasZn             bool
		iron, calcium, vitD, vitB12             float64
		vitC, mg, k, zn                         float64
	)
	for _, e := range entries {
		f := e.QuantityG / 100.0
		n := e.EffectiveNutrimentsPer100g
		t.Kcal += valOrZero(n.KcalPer100g) * f
		t.ProteinG += valOrZero(n.ProteinGPer100g) * f
		t.CarbsG += valOrZero(n.CarbsGPer100g) * f
		t.FatG += valOrZero(n.FatGPer100g) * f
		t.FiberG += valOrZero(n.FiberGPer100g) * f
		t.SugarG += valOrZero(n.SugarGPer100g) * f
		t.SaltG += valOrZero(n.SaltGPer100g) * f
		if n.IronMgPer100g != nil {
			iron += *n.IronMgPer100g * f
			hasIron = true
		}
		if n.CalciumMgPer100g != nil {
			calcium += *n.CalciumMgPer100g * f
			hasCalcium = true
		}
		if n.VitaminDMcgPer100g != nil {
			vitD += *n.VitaminDMcgPer100g * f
			hasVitD = true
		}
		if n.VitaminB12McgPer100g != nil {
			vitB12 += *n.VitaminB12McgPer100g * f
			hasVitB12 = true
		}
		if n.VitaminCMgPer100g != nil {
			vitC += *n.VitaminCMgPer100g * f
			hasVitC = true
		}
		if n.MagnesiumMgPer100g != nil {
			mg += *n.MagnesiumMgPer100g * f
			hasMg = true
		}
		if n.PotassiumMgPer100g != nil {
			k += *n.PotassiumMgPer100g * f
			hasK = true
		}
		if n.ZincMgPer100g != nil {
			zn += *n.ZincMgPer100g * f
			hasZn = true
		}
	}
	if hasIron {
		t.IronMg = &iron
	}
	if hasCalcium {
		t.CalciumMg = &calcium
	}
	if hasVitD {
		t.VitaminDMcg = &vitD
	}
	if hasVitB12 {
		t.VitaminB12Mcg = &vitB12
	}
	if hasVitC {
		t.VitaminCMg = &vitC
	}
	if hasMg {
		t.MagnesiumMg = &mg
	}
	if hasK {
		t.PotassiumMg = &k
	}
	if hasZn {
		t.ZincMg = &zn
	}
	return t
}

func valOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// Sentinel for callers that want to test the error path without bringing up a
// real DB. Currently unused outside tests.
var errGoalsLookup = errors.New("goals lookup failed")

var _ = errGoalsLookup
