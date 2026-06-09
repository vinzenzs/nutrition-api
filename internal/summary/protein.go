package summary

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/numfmt"
)

// MPS literature value (Phillips 2014 / Loucks 2007 / Stokes 2018 all converge):
// per-meal protein dose that maximises muscle protein synthesis is
// ~0.3 g protein per kg body weight. Hard-coded in v1; a future
// `mps_threshold_g_per_kg` query parameter is a non-breaking follow-up if
// real use shows the friction.
const mpsThresholdGPerKg = 0.3

// Body-weight resolution constants — exported so the response and the spec
// can refer to the same canonical strings.
const (
	BodyWeightSourceExplicit       = "explicit"
	BodyWeightSourceRolling7dAvg   = "rolling_7d_avg"
	BodyWeightSourceLastBeforeDate = "last_before_date"
)

// Validation errors for the protein-distribution path.
var (
	ErrWeightDataMissing  = errors.New("weight_data_missing")
	ErrBodyWeightInvalid  = errors.New("body_weight_kg_invalid")
)

// ProteinMeal is one row in the response — one per `meal_entries` row on the
// queried date. `MealType` is nullable because the column itself is. The
// gap is *int (nullable) so `null` round-trips on the first meal of the day.
type ProteinMeal struct {
	LoggedAt                time.Time `json:"logged_at"`
	LoggedAtHour            int       `json:"logged_at_hour"`
	MealType                *string   `json:"meal_type,omitempty"`
	ProteinG                float64   `json:"protein_g"`
	MPSEffective            bool      `json:"mps_effective"`
	GapMinutesSincePrevious *int      `json:"gap_minutes_since_previous"`
}

// ProteinDistribution is the response shape for GET /summary/protein-distribution.
type ProteinDistribution struct {
	Date                  string        `json:"date"`
	TZ                    string        `json:"tz"`
	BodyWeightKg          float64       `json:"body_weight_kg"`
	BodyWeightSource      string        `json:"body_weight_source"`
	MPSThresholdG         float64       `json:"mps_threshold_g"`
	TotalProteinG         float64       `json:"total_protein_g"`
	MealCount             int           `json:"meal_count"`
	MPSEffectiveMealCount int           `json:"mps_effective_meal_count"`
	Meals                 []ProteinMeal `json:"meals"`
}

// ProteinDistributionParams scopes a protein-distribution request. The
// handler validates Date / Loc / BodyWeightKgOverride; this layer assumes
// them in range. BodyWeightKgOverride is nil when the user didn't pass it.
type ProteinDistributionParams struct {
	Date                 time.Time
	Loc                  *time.Location
	BodyWeightKgOverride *float64
}

// SetBodyWeightRepo wires the body-weight repo so the protein-distribution
// endpoint can resolve the user's weight at the queried date. Optional setter
// for the same reason `meals.Service.SetWorkoutsRepo` is — keeps the existing
// constructor signature stable while letting one new endpoint depend on the
// new repo. Callers that don't set it cannot call ProteinDistributionFor (it
// returns weight_data_missing on every call without an explicit override).
func (s *Service) SetBodyWeightRepo(r *bodyweight.Repo) {
	s.bodyWeightRepo = r
}

// ProteinDistributionFor returns the per-meal protein breakdown for a single
// calendar date in loc, annotated with the MPS threshold derived from body
// weight. See `internal/summary/protein.go` and the add-protein-distribution
// proposal for the full algorithm.
func (s *Service) ProteinDistributionFor(ctx context.Context, p ProteinDistributionParams) (*ProteinDistribution, error) {
	if p.BodyWeightKgOverride != nil {
		v := *p.BodyWeightKgOverride
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return nil, ErrBodyWeightInvalid
		}
	}

	bwKg, bwSource, err := s.resolveBodyWeightAtDate(ctx, p.Date, p.Loc, p.BodyWeightKgOverride)
	if err != nil {
		return nil, err
	}

	dayStart := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), 0, 0, 0, 0, p.Loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	entries, err := s.mealsRepo.List(ctx, meals.ListParams{From: dayStart.UTC(), To: dayEnd.UTC()})
	if err != nil {
		return nil, fmt.Errorf("list meals for protein distribution: %w", err)
	}

	mpsThreshold := mpsThresholdGPerKg * bwKg

	out := &ProteinDistribution{
		Date:             p.Date.Format("2006-01-02"),
		TZ:               p.Loc.String(),
		BodyWeightKg:     numfmt.Round1(bwKg),
		BodyWeightSource: bwSource,
		MPSThresholdG:    numfmt.Round1(mpsThreshold),
		MealCount:        len(entries),
		Meals:            make([]ProteinMeal, 0, len(entries)),
	}

	var totalProtein float64
	for i, e := range entries {
		// Per-row protein (not per-day): inline the math; cheap, avoids
		// summary.SumEntries which is per-day.
		protein := proteinForEntry(e)
		totalProtein += protein

		row := ProteinMeal{
			LoggedAt:     e.LoggedAt,
			LoggedAtHour: e.LoggedAt.In(p.Loc).Hour(),
			ProteinG:     numfmt.Round1(protein),
			MPSEffective: protein >= mpsThreshold,
		}
		if e.MealType != nil {
			mt := string(*e.MealType)
			row.MealType = &mt
		}
		if i > 0 {
			gap := int(e.LoggedAt.Sub(entries[i-1].LoggedAt).Minutes())
			row.GapMinutesSincePrevious = &gap
		}
		out.Meals = append(out.Meals, row)
		if row.MPSEffective {
			out.MPSEffectiveMealCount++
		}
	}

	out.TotalProteinG = numfmt.Round1(totalProtein)
	return out, nil
}

// proteinForEntry computes the per-entry protein in grams from the entry's
// effective per-100g nutriments and its quantity.
func proteinForEntry(e *meals.MealEntry) float64 {
	per100g := e.EffectiveNutrimentsPer100g.ProteinGPer100g
	if per100g == nil {
		return 0
	}
	return *per100g * (e.QuantityG / 100.0)
}

// resolveBodyWeightAtDate implements the four-tier resolution rule:
//
//  1. Explicit override → "explicit"
//  2. Rolling 7-day mean of entries in [localMidnight(date-6d), localMidnight(date+1d)) → "rolling_7d_avg"
//  3. Most-recent entry strictly before localMidnight(date) → "last_before_date"
//  4. No data at all → ErrWeightDataMissing
//
// Mirrors the energy package's resolveBodyWeight in spirit but operates on a
// single date rather than a [from, to) window — different semantics, written
// inline rather than hoisted (decision documented in
// openspec/changes/add-protein-distribution/design.md §3).
func (s *Service) resolveBodyWeightAtDate(ctx context.Context, date time.Time, loc *time.Location, override *float64) (float64, string, error) {
	if override != nil {
		return *override, BodyWeightSourceExplicit, nil
	}
	if s.bodyWeightRepo == nil {
		// Defensive: no override, no repo wired → cannot resolve.
		return 0, "", ErrWeightDataMissing
	}

	startMidnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	endMidnight := startMidnight.Add(24 * time.Hour)
	// Rolling window: 7 local days ending at `date` inclusive →
	// [localMidnight(date - 6d), localMidnight(date + 1d)).
	rollingStart := startMidnight.AddDate(0, 0, -6)

	rolling, err := s.bodyWeightRepo.List(ctx, rollingStart.UTC(), endMidnight.UTC())
	if err != nil {
		return 0, "", err
	}
	if len(rolling) > 0 {
		return meanWeight(rolling), BodyWeightSourceRolling7dAvg, nil
	}

	latest, err := s.bodyWeightRepo.LatestBefore(ctx, startMidnight.UTC())
	if err != nil {
		if errors.Is(err, bodyweight.ErrNotFound) {
			return 0, "", ErrWeightDataMissing
		}
		return 0, "", err
	}
	return latest.WeightKg, BodyWeightSourceLastBeforeDate, nil
}

func meanWeight(entries []*bodyweight.Entry) float64 {
	if len(entries) == 0 {
		return 0
	}
	var sum float64
	for _, e := range entries {
		sum += e.WeightKg
	}
	return sum / float64(len(entries))
}
