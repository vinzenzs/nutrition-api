package personalrecords

import (
	"context"
	"errors"
	"strings"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrExternalIDRequired = errors.New("external_id_required")
	ErrPRTypeRequired     = errors.New("pr_type_required")
	ErrValueInvalid       = errors.New("value_invalid")
	ErrUnitRequired       = errors.New("unit_required")
	ErrAchievedAtRequired = errors.New("achieved_at_required")
)

// Service orchestrates personal-record CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and upserts a record by external_id. created=true on INSERT.
func (s *Service) Upsert(ctx context.Context, pr *PersonalRecord) (*PersonalRecord, bool, error) {
	pr.ExternalID = strings.TrimSpace(pr.ExternalID)
	pr.PRType = strings.TrimSpace(pr.PRType)
	pr.Unit = strings.TrimSpace(pr.Unit)
	if pr.ExternalID == "" {
		return nil, false, ErrExternalIDRequired
	}
	if pr.PRType == "" {
		return nil, false, ErrPRTypeRequired
	}
	// value is required and non-negative: a nil pointer means the field was
	// absent (distinct from an explicit 0, which is valid-but-degenerate).
	if pr.Value == nil || *pr.Value < 0 {
		return nil, false, ErrValueInvalid
	}
	if pr.Unit == "" {
		return nil, false, ErrUnitRequired
	}
	if pr.AchievedAt.IsZero() {
		return nil, false, ErrAchievedAtRequired
	}
	created, err := s.repo.Upsert(ctx, pr)
	if err != nil {
		return nil, false, err
	}
	out, err := s.repo.GetByID(ctx, pr.ID)
	if err != nil {
		return nil, false, err
	}
	return out, created, nil
}

// List returns records, optionally filtered by pr_type.
func (s *Service) List(ctx context.Context, prType *string) ([]*PersonalRecord, error) {
	return s.repo.List(ctx, prType)
}
