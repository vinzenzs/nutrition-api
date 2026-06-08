package energy

import (
	"context"
	"errors"
	"time"

	"github.com/vinzenzs/nutrition-api/internal/bodyweight"
)

const rollingWindowDays = 7

// resolveComposition picks FFM_kg per the four-tier rule documented in the
// energy-availability spec:
//
//  1. lean_mass_kg param (explicit)
//  2. body_fat_pct param + body weight
//  3. body_fat_pct from most recent in-window weight entry + body weight
//  4. body weight × 0.85 (fallback; flagged loudly)
//
// Returns ErrWeightDataMissing only when *no* body-weight data exists AND no
// explicit lean_mass_kg was supplied. Tier 1 with no stored weight is honored
// (BodyWeightKg stays nil in the response).
func (s *Service) resolveComposition(ctx context.Context, p AvailabilityParams) (*Composition, error) {
	bwKg, bwSource, err := s.resolveBodyWeight(ctx, p.From, p.To)
	if err != nil {
		return nil, err
	}

	// Tier 1: explicit lean mass wins over everything.
	if p.LeanMassKg != nil {
		return &Composition{
			FFMKg:            *p.LeanMassKg,
			Source:           SourceExplicitLeanMass,
			BodyWeightKg:     bwKg,
			BodyWeightSource: bwSource,
		}, nil
	}

	// Tiers 2-4 all need body weight.
	if bwKg == nil {
		return nil, ErrWeightDataMissing
	}

	// Tier 2: explicit body_fat_pct.
	if p.BodyFatPct != nil {
		return &Composition{
			FFMKg:            *bwKg * (1 - *p.BodyFatPct/100),
			Source:           SourceExplicitBodyFat,
			BodyWeightKg:     bwKg,
			BodyWeightSource: bwSource,
		}, nil
	}

	// Tier 3: stored body_fat_pct from the most-recent in-window entry.
	if bf, err := s.latestInWindowBodyFat(ctx, p.From, p.To); err != nil {
		return nil, err
	} else if bf != nil {
		return &Composition{
			FFMKg:            *bwKg * (1 - *bf/100),
			Source:           SourceStoredBodyFat,
			BodyWeightKg:     bwKg,
			BodyWeightSource: bwSource,
		}, nil
	}

	// Tier 4: 85% fallback. Loud — composition_estimated flag set.
	return &Composition{
		FFMKg:                *bwKg * 0.85,
		Source:               SourceEstimated85pct,
		BodyWeightKg:         bwKg,
		BodyWeightSource:     bwSource,
		CompositionEstimated: true,
	}, nil
}

// resolveBodyWeight applies the three-tier body-weight rule:
//
//  1. rolling-7d-avg using entries in [to-7d, to)
//  2. mean of entries in [from, to) (shorter-window fallback)
//  3. weight of the most-recent entry strictly before `from`
//
// Returns (nil, nil, nil) — no error and nil weight — when no entries exist
// anywhere; the caller decides whether that's terminal (Tier 2-4 of FFM) or
// fine (Tier 1 of FFM, explicit lean mass).
func (s *Service) resolveBodyWeight(ctx context.Context, from, to time.Time) (*float64, *string, error) {
	// Tier 1: rolling 7-day avg ending at `to`. Use entries in [to-7d, to).
	rollingStart := to.Add(-time.Duration(rollingWindowDays) * 24 * time.Hour)
	if rollingStart.Before(from) {
		// Window shorter than 7 days — Tier 2 will pick this up; skip tier 1.
	} else {
		rolling, err := s.bodyWeight.List(ctx, rollingStart, to)
		if err != nil {
			return nil, nil, err
		}
		if len(rolling) > 0 {
			avg := meanWeight(rolling)
			src := BodyWeightSourceRolling7d
			return &avg, &src, nil
		}
	}

	// Tier 2: mean of entries in [from, to).
	inWindow, err := s.bodyWeight.List(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	if len(inWindow) > 0 {
		avg := meanWeight(inWindow)
		src := BodyWeightSourceInWindow
		return &avg, &src, nil
	}

	// Tier 3: last-before-window.
	latest, err := s.bodyWeight.LatestBefore(ctx, from)
	if err != nil {
		if errors.Is(err, bodyweight.ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	src := BodyWeightSourceLastBefore
	return &latest.WeightKg, &src, nil
}

// latestInWindowBodyFat returns the body_fat_pct value of the most-recent
// in-window body-weight entry that has it set. Returns (nil, nil) when no
// entry carries the field — distinct from a DB error.
func (s *Service) latestInWindowBodyFat(ctx context.Context, from, to time.Time) (*float64, error) {
	entries, err := s.bodyWeight.List(ctx, from, to)
	if err != nil {
		return nil, err
	}
	// List orders ASC by logged_at — walk backwards for "most recent."
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].BodyFatPct != nil {
			return entries[i].BodyFatPct, nil
		}
	}
	return nil, nil
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
