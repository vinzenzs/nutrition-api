package trainingphases

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/goals"
)

// Template-name and -value validation errors. Handler-facing.
var (
	ErrTemplateNameInvalid  = errors.New("template_name_invalid")
	ErrTemplateNameTooLong  = errors.New("template_name_too_long")
)

// MaxTemplateNameLength matches the migration's CHECK constraint.
const MaxTemplateNameLength = 128

// TemplatesService runs validation and orchestrates writes to TemplatesRepo.
type TemplatesService struct {
	repo *TemplatesRepo
}

func NewTemplatesService(repo *TemplatesRepo) *TemplatesService {
	return &TemplatesService{repo: repo}
}

// Upsert validates the name and the goal-bound fields, then stores the
// template. Returns the freshly-stored row (so handlers can render the
// response without a follow-up GET).
func (s *TemplatesService) Upsert(ctx context.Context, name string, t *Template) (*Template, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, ErrTemplateNameInvalid
	}
	if len(cleanName) > MaxTemplateNameLength {
		return nil, ErrTemplateNameTooLong
	}
	t.Name = cleanName
	if err := goals.ValidateGoals(t.AsGoals()); err != nil {
		return nil, err
	}
	return s.repo.Upsert(ctx, t)
}

// GetByName returns a stored template or ErrTemplateNotFound.
func (s *TemplatesService) GetByName(ctx context.Context, name string) (*Template, error) {
	return s.repo.GetByName(ctx, name)
}

// GetByID is used by the goals resolver to look up a phase's template.
func (s *TemplatesService) GetByID(ctx context.Context, id uuid.UUID) (*Template, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns every template ordered by name.
func (s *TemplatesService) List(ctx context.Context) ([]*Template, error) {
	return s.repo.List(ctx)
}

// Delete removes a template by name. Returns ErrTemplateNotFound on a miss
// and *InUseError when FK-RESTRICT trips.
func (s *TemplatesService) Delete(ctx context.Context, name string) error {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return ErrTemplateNameInvalid
	}
	return s.repo.Delete(ctx, cleanName)
}
