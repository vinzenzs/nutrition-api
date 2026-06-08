package trainingphases

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/goals"
)

// PhaseLookupAdapter wraps *PhasesRepo to satisfy goals.PhaseLookup. The
// adapter projects this package's Phase struct down to the goals.PhaseRef
// shape (only the fields the resolver needs). Kept here, not in goals, so
// goals doesn't need to import trainingphases (which would create a cycle —
// trainingphases already imports goals for the Range type).
type PhaseLookupAdapter struct {
	repo *PhasesRepo
}

// NewPhaseLookupAdapter wraps a *PhasesRepo.
func NewPhaseLookupAdapter(repo *PhasesRepo) *PhaseLookupAdapter {
	return &PhaseLookupAdapter{repo: repo}
}

// PhaseFor returns the most-recently-updated phase covering `date`, mapped
// to a goals.PhaseRef. Returns (nil, nil) when no phase covers the date —
// this differs from the underlying repo's ErrPhaseNotFound to give the
// resolver a clean "no match" signal without sentinel-error juggling.
func (a *PhaseLookupAdapter) PhaseFor(ctx context.Context, date time.Time) (*goals.PhaseRef, error) {
	p, err := a.repo.PhaseFor(ctx, date)
	if err != nil {
		if errors.Is(err, ErrPhaseNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return phaseToRef(p), nil
}

// PhasesIntersecting returns every phase intersecting [from, to] mapped to
// goals.PhaseRef.
func (a *PhaseLookupAdapter) PhasesIntersecting(ctx context.Context, from, to time.Time) ([]*goals.PhaseRef, error) {
	ps, err := a.repo.ListIntersecting(ctx, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]*goals.PhaseRef, 0, len(ps))
	for _, p := range ps {
		out = append(out, phaseToRef(p))
	}
	return out, nil
}

func phaseToRef(p *Phase) *goals.PhaseRef {
	if p == nil {
		return nil
	}
	return &goals.PhaseRef{
		Name:              p.Name,
		StartDate:         p.StartDate,
		EndDate:           p.EndDate,
		DefaultTemplateID: p.DefaultTemplateID,
		UpdatedAt:         p.UpdatedAt,
	}
}

// TemplateLookupAdapter wraps *TemplatesRepo to satisfy goals.TemplateLookup.
type TemplateLookupAdapter struct {
	repo *TemplatesRepo
}

func NewTemplateLookupAdapter(repo *TemplatesRepo) *TemplateLookupAdapter {
	return &TemplateLookupAdapter{repo: repo}
}

// TemplateGoals fetches one template by id and returns its bounds as
// *goals.Goals. Returns (nil, nil) when the template doesn't exist — same
// "no match means nil" convention as PhaseLookupAdapter.PhaseFor.
func (a *TemplateLookupAdapter) TemplateGoals(ctx context.Context, id uuid.UUID) (*goals.Goals, error) {
	t, err := a.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return t.AsGoals(), nil
}

// TemplateGoalsByIDs batch-fetches templates and returns their bounds.
// Missing IDs are silently omitted from the map.
func (a *TemplateLookupAdapter) TemplateGoalsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*goals.Goals, error) {
	ts, err := a.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*goals.Goals, len(ts))
	for id, t := range ts {
		out[id] = t.AsGoals()
	}
	return out, nil
}
