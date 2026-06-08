package goals

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// GoalSource identifies which goal set produced an adherence row in a summary.
type GoalSource string

const (
	GoalSourceDefault       GoalSource = "default"
	GoalSourceOverride      GoalSource = "override"
	GoalSourcePhaseTemplate GoalSource = "phase_template"
	GoalSourceNone          GoalSource = "none"
)

// PhaseRef is a minimal projection of a training_phases row, carrying only
// what the resolver needs to interpret it. Defined in this package (not in
// trainingphases) so the resolver doesn't have to import its consumer —
// which it can't, since trainingphases imports `goals` for the Range type.
// The trainingphases package implements PhaseLookup by mapping its own
// Phase struct down to this shape.
type PhaseRef struct {
	Name              string
	StartDate         time.Time
	EndDate           time.Time
	DefaultTemplateID *uuid.UUID
	UpdatedAt         time.Time
}

// PhaseLookup is the slice of training_phases functionality the resolver
// depends on. PhaseFor returns the most-recently-updated phase covering
// `date`, or (nil, nil) when none does. PhasesIntersecting returns every
// phase whose [start_date, end_date] intersects [from, to], ordered by
// start_date ASC then updated_at DESC (so range processing can pick the
// winner per day in memory).
type PhaseLookup interface {
	PhaseFor(ctx context.Context, date time.Time) (*PhaseRef, error)
	PhasesIntersecting(ctx context.Context, from, to time.Time) ([]*PhaseRef, error)
}

// TemplateLookup is the slice of goal_templates functionality the resolver
// depends on. Returns goals (already shaped) for use in adherence.
type TemplateLookup interface {
	TemplateGoals(ctx context.Context, id uuid.UUID) (*Goals, error)
	TemplateGoalsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Goals, error)
}

// Resolver resolves "today's effective goals" from the four-step chain:
// per-date override > phase template > singleton default > none.
type Resolver struct {
	defaults  *Repo
	overrides *OverridesRepo
	phases    PhaseLookup
	templates TemplateLookup
}

// NewResolver constructs the resolver. `phases` and `templates` may be nil —
// in that case the resolver skips the phase step and falls back to the
// override > default > none chain (preserving pre-add-training-phases behaviour).
// In production both are wired; in tests that don't care about phases, both
// stay nil.
func NewResolver(defaults *Repo, overrides *OverridesRepo, phases PhaseLookup, templates TemplateLookup) *Resolver {
	return &Resolver{defaults: defaults, overrides: overrides, phases: phases, templates: templates}
}

// EffectiveFor returns the goal set that applies to `date`, the source
// label, and (when source == phase_template) the resolved phase name. If
// no goals apply, returns (nil, "none", "", nil).
func (r *Resolver) EffectiveFor(ctx context.Context, date time.Time) (*Goals, GoalSource, string, error) {
	override, err := r.overrides.GetOverride(ctx, date)
	if err != nil && !errors.Is(err, ErrOverrideNotFound) {
		return nil, GoalSourceNone, "", err
	}
	if override != nil {
		return override, GoalSourceOverride, "", nil
	}
	if r.phases != nil && r.templates != nil {
		phase, err := r.phases.PhaseFor(ctx, date)
		if err != nil {
			return nil, GoalSourceNone, "", err
		}
		if phase != nil && phase.DefaultTemplateID != nil {
			g, err := r.templates.TemplateGoals(ctx, *phase.DefaultTemplateID)
			if err != nil {
				return nil, GoalSourceNone, "", err
			}
			if g != nil {
				return g, GoalSourcePhaseTemplate, phase.Name, nil
			}
		}
	}
	def, err := r.defaults.Get(ctx)
	if err != nil {
		return nil, GoalSourceNone, "", err
	}
	if def == nil {
		return nil, GoalSourceNone, "", nil
	}
	return def, GoalSourceDefault, "", nil
}

// EffectiveForRange returns one entry per calendar date in [from, to]. The
// default singleton is fetched once; overrides, phases, and the templates
// referenced by those phases are each fetched in a single batch query.
func (r *Resolver) EffectiveForRange(
	ctx context.Context, from, to time.Time,
) (map[string]*Goals, map[string]GoalSource, map[string]string, error) {
	def, err := r.defaults.Get(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	overrides, err := r.overrides.List(ctx, from, to)
	if err != nil {
		return nil, nil, nil, err
	}
	overridesByDate := make(map[string]*Goals, len(overrides))
	for _, o := range overrides {
		overridesByDate[o.Date.Format("2006-01-02")] = o.Goals
	}

	// Fetch phases intersecting the window, plus the templates they
	// reference. Both are no-ops when the resolver has no PhaseLookup wired.
	var phases []*PhaseRef
	templatesByID := map[uuid.UUID]*Goals{}
	if r.phases != nil && r.templates != nil {
		phases, err = r.phases.PhasesIntersecting(ctx, from, to)
		if err != nil {
			return nil, nil, nil, err
		}
		ids := make([]uuid.UUID, 0, len(phases))
		for _, p := range phases {
			if p.DefaultTemplateID != nil {
				ids = append(ids, *p.DefaultTemplateID)
			}
		}
		if len(ids) > 0 {
			templatesByID, err = r.templates.TemplateGoalsByIDs(ctx, ids)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	effective := map[string]*Goals{}
	sources := map[string]GoalSource{}
	phaseNames := map[string]string{}
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if override, ok := overridesByDate[key]; ok {
			effective[key] = override
			sources[key] = GoalSourceOverride
			continue
		}
		// Find the most-recently-updated phase covering d, with a non-nil
		// template that we successfully fetched. Phases are sorted by
		// (start_date ASC, updated_at DESC); the per-day pick uses
		// updated_at as the tiebreaker.
		var winner *PhaseRef
		var winnerGoals *Goals
		for _, p := range phases {
			if d.Before(dayOnly(p.StartDate)) || d.After(dayOnly(p.EndDate)) {
				continue
			}
			if p.DefaultTemplateID == nil {
				continue
			}
			g, ok := templatesByID[*p.DefaultTemplateID]
			if !ok || g == nil {
				continue
			}
			if winner == nil || p.UpdatedAt.After(winner.UpdatedAt) {
				winner = p
				winnerGoals = g
			}
		}
		if winner != nil {
			effective[key] = winnerGoals
			sources[key] = GoalSourcePhaseTemplate
			phaseNames[key] = winner.Name
			continue
		}
		if def != nil {
			effective[key] = def
			sources[key] = GoalSourceDefault
			continue
		}
		effective[key] = nil
		sources[key] = GoalSourceNone
	}
	return effective, sources, phaseNames, nil
}

func dayOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
