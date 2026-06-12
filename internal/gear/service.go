package gear

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrExternalIDRequired     = errors.New("external_id_required")
	ErrGearTypeInvalid        = errors.New("gear_type_invalid")
	ErrDisplayNameRequired    = errors.New("display_name_required")
	ErrTotalDistanceMInvalid  = errors.New("total_distance_m_invalid")
	ErrTotalActivitiesInvalid = errors.New("total_activities_invalid")
)

// Service orchestrates gear CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and upserts a gear record by external_id. created=true on
// INSERT.
func (s *Service) Upsert(ctx context.Context, g *Gear) (*Gear, bool, error) {
	g.ExternalID = strings.TrimSpace(g.ExternalID)
	g.DisplayName = strings.TrimSpace(g.DisplayName)
	if g.ExternalID == "" {
		return nil, false, ErrExternalIDRequired
	}
	if !ValidType(string(g.GearType)) {
		return nil, false, ErrGearTypeInvalid
	}
	if g.DisplayName == "" {
		return nil, false, ErrDisplayNameRequired
	}
	if g.TotalDistanceM != nil && *g.TotalDistanceM < 0 {
		return nil, false, ErrTotalDistanceMInvalid
	}
	if g.TotalActivities != nil && *g.TotalActivities < 0 {
		return nil, false, ErrTotalActivitiesInvalid
	}
	created, err := s.repo.Upsert(ctx, g)
	if err != nil {
		return nil, false, err
	}
	out, err := s.repo.GetByID(ctx, g.ID)
	if err != nil {
		return nil, false, err
	}
	return out, created, nil
}

// Get returns the gear record by backend id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Gear, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns gear rows, optionally filtered by retirement state.
func (s *Service) List(ctx context.Context, retired *bool) ([]*Gear, error) {
	return s.repo.List(ctx, retired)
}
