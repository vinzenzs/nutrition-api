package workouttemplates

import (
	"context"
	"errors"
	"strings"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrSportInvalid       = errors.New("sport_invalid")
	ErrNameRequired       = errors.New("name_required")
	ErrEstimatedInvalid   = errors.New("estimated_duration_sec_invalid")
	ErrStepsEmpty         = errors.New("steps_empty")
	ErrStepTypeInvalid    = errors.New("step_type_invalid")
	ErrIntentInvalid      = errors.New("intent_invalid")
	ErrDurationInvalid    = errors.New("duration_invalid")
	ErrTargetInvalid      = errors.New("target_invalid")
	ErrTargetRangeInvalid = errors.New("target_range_invalid")
	ErrRepeatInvalid      = errors.New("repeat_invalid")
	ErrRepeatNested       = errors.New("repeat_nested")
)

// Service orchestrates template CRUD with structured-step validation.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new template.
func (s *Service) Create(ctx context.Context, t *Template) (*Template, error) {
	if err := validateTemplate(t); err != nil {
		return nil, err
	}
	t.Name = strings.TrimSpace(t.Name)
	return s.repo.Create(ctx, t)
}

// Get returns one template by id.
func (s *Service) Get(ctx context.Context, id string) (*Template, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns templates, optionally filtered by sport.
func (s *Service) List(ctx context.Context, sport string) ([]*Template, error) {
	if sport != "" && !validSport(sport) {
		return nil, ErrSportInvalid
	}
	return s.repo.List(ctx, sport)
}

// PatchInput is a partial update. A nil pointer / false Set flag means "leave
// unchanged"; for the nullable fields a true Set flag with a nil value clears
// the column (design D5).
type PatchInput struct {
	Name  *string
	Sport *string
	Steps *[]Step

	SetDescription bool
	Description    *string

	SetEstimated         bool
	EstimatedDurationSec *int
}

// Patch loads the template, applies the supplied fields, validates the result,
// and writes it back. Returns ErrNotFound when the id is unknown.
func (s *Service) Patch(ctx context.Context, id string, in PatchInput) (*Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if in.Sport != nil {
		t.Sport = *in.Sport
	}
	if in.Steps != nil {
		t.Steps = *in.Steps
	}
	if in.SetDescription {
		t.Description = in.Description
	}
	if in.SetEstimated {
		t.EstimatedDurationSec = in.EstimatedDurationSec
	}
	if err := validateTemplate(t); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, t)
}

// Delete removes a template by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ----- validation -----

func validateTemplate(t *Template) error {
	if !validSport(t.Sport) {
		return ErrSportInvalid
	}
	if strings.TrimSpace(t.Name) == "" {
		return ErrNameRequired
	}
	if t.EstimatedDurationSec != nil && *t.EstimatedDurationSec <= 0 {
		return ErrEstimatedInvalid
	}
	if len(t.Steps) == 0 {
		return ErrStepsEmpty
	}
	for i := range t.Steps {
		if err := validateNode(t.Steps[i], false); err != nil {
			return err
		}
	}
	return nil
}

// validateNode validates one step node. nested=true forbids further repeats
// (single-level repeat groups only).
func validateNode(n Step, nested bool) error {
	switch n.Type {
	case NodeStep:
		return validateSingleStep(n)
	case NodeRepeat:
		if nested {
			return ErrRepeatNested
		}
		if n.Count < 2 {
			return ErrRepeatInvalid
		}
		if len(n.Steps) == 0 {
			return ErrRepeatInvalid
		}
		for i := range n.Steps {
			// A repeat's children must be single steps; a nested repeat trips
			// ErrRepeatNested via the nested=true flag.
			if err := validateNode(n.Steps[i], true); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrStepTypeInvalid
	}
}

func validateSingleStep(n Step) error {
	switch n.Intent {
	case IntentWarmup, IntentActive, IntentInterval, IntentRecovery, IntentRest, IntentCooldown:
	default:
		return ErrIntentInvalid
	}
	if err := validateDuration(n.Duration); err != nil {
		return err
	}
	return validateTarget(n.Target)
}

func validateDuration(d *Duration) error {
	if d == nil {
		return ErrDurationInvalid
	}
	switch d.Kind {
	case DurationTime:
		if d.Seconds == nil || *d.Seconds <= 0 {
			return ErrDurationInvalid
		}
	case DurationDistance:
		if d.Meters == nil || *d.Meters <= 0 {
			return ErrDurationInvalid
		}
	case DurationLapButton, DurationOpen:
		// no quantity
	default:
		return ErrDurationInvalid
	}
	return nil
}

func validateTarget(t *Target) error {
	if t == nil {
		return ErrTargetInvalid
	}
	switch t.Kind {
	case TargetNone:
		return nil
	case TargetHRZone, TargetPowerZone:
		return validateRange(t.Low, t.High, 1, 5)
	case TargetHRBpm, TargetPowerW, TargetRPE:
		return validatePositiveRange(t.Low, t.High)
	case TargetPace:
		return validatePositiveRange(t.LowSecPerKM, t.HighSecPerKM)
	default:
		return ErrTargetInvalid
	}
}

// validateRange enforces both bounds (when present) within [lo, hi] and low <= high.
func validateRange(low, high *int, lo, hi int) error {
	for _, v := range []*int{low, high} {
		if v != nil && (*v < lo || *v > hi) {
			return ErrTargetRangeInvalid
		}
	}
	if low != nil && high != nil && *low > *high {
		return ErrTargetRangeInvalid
	}
	return nil
}

// validatePositiveRange enforces positive bounds (when present) and low <= high.
func validatePositiveRange(low, high *int) error {
	for _, v := range []*int{low, high} {
		if v != nil && *v <= 0 {
			return ErrTargetRangeInvalid
		}
	}
	if low != nil && high != nil && *low > *high {
		return ErrTargetRangeInvalid
	}
	return nil
}
