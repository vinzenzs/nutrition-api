package goals

import (
	"context"
	"errors"
	"time"
)

// GoalSource identifies which goal set produced an adherence row in a summary.
type GoalSource string

const (
	GoalSourceDefault  GoalSource = "default"
	GoalSourceOverride GoalSource = "override"
	GoalSourceNone     GoalSource = "none"
)

// Resolver resolves "today's effective goals" from the default singleton plus
// the per-date override table. Wraps both repos so callers don't juggle two
// dependencies.
type Resolver struct {
	defaults  *Repo
	overrides *OverridesRepo
}

func NewResolver(defaults *Repo, overrides *OverridesRepo) *Resolver {
	return &Resolver{defaults: defaults, overrides: overrides}
}

// EffectiveFor returns the goal set that applies to `date` and the source
// label. Override beats default; if neither exists, returns (nil, "none", nil).
func (r *Resolver) EffectiveFor(ctx context.Context, date time.Time) (*Goals, GoalSource, error) {
	override, err := r.overrides.GetOverride(ctx, date)
	if err != nil && !errors.Is(err, ErrOverrideNotFound) {
		return nil, GoalSourceNone, err
	}
	if override != nil {
		return override, GoalSourceOverride, nil
	}
	def, err := r.defaults.Get(ctx)
	if err != nil {
		return nil, GoalSourceNone, err
	}
	if def == nil {
		return nil, GoalSourceNone, nil
	}
	return def, GoalSourceDefault, nil
}

// EffectiveForRange returns one entry per calendar date in [from, to] (both
// local-date values). The default singleton is fetched once and shared; all
// overrides in the window are fetched in a single query.
func (r *Resolver) EffectiveForRange(ctx context.Context, from, to time.Time) (map[string]*Goals, map[string]GoalSource, error) {
	def, err := r.defaults.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	overrides, err := r.overrides.List(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	byDate := make(map[string]*Goals, len(overrides))
	for _, o := range overrides {
		byDate[o.Date.Format("2006-01-02")] = o.Goals
	}

	effective := map[string]*Goals{}
	sources := map[string]GoalSource{}
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	for d := fromDay; !d.After(toDay); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if override, ok := byDate[key]; ok {
			effective[key] = override
			sources[key] = GoalSourceOverride
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
	return effective, sources, nil
}
