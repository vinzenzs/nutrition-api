package hydration

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/nutrition-api/internal/workouts"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrQuantityInvalid = errors.New("quantity_ml_invalid")
	ErrLoggedAtFuture  = errors.New("logged_at_too_far_future")
	ErrNoteTooLong     = errors.New("note_too_long")
	ErrWorkoutNotFound = errors.New("workout_not_found")
)

const maxNoteLen = 500

// Service orchestrates hydration entry CRUD over the repo.
type Service struct {
	repo         *Repo
	workoutsRepo *workouts.Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// SetWorkoutsRepo wires the workouts repo for workout_id link validation.
// Optional — callers that don't set it skip the existence check.
func (s *Service) SetWorkoutsRepo(r *workouts.Repo) { s.workoutsRepo = r }

func (s *Service) validateWorkoutID(ctx context.Context, id uuid.UUID) error {
	if s.workoutsRepo == nil {
		return nil
	}
	if _, err := s.workoutsRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			return ErrWorkoutNotFound
		}
		return err
	}
	return nil
}

// CreateInput is the payload for POST /hydration.
type CreateInput struct {
	QuantityMl float64
	LoggedAt   time.Time
	Note       *string
	WorkoutID  *uuid.UUID
}

// Create validates and inserts a hydration entry.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Entry, error) {
	if err := validateQuantity(in.QuantityMl); err != nil {
		return nil, err
	}
	if err := validateLoggedAt(in.LoggedAt); err != nil {
		return nil, err
	}
	if err := validateNote(in.Note); err != nil {
		return nil, err
	}
	if in.WorkoutID != nil {
		if err := s.validateWorkoutID(ctx, *in.WorkoutID); err != nil {
			return nil, err
		}
	}
	e := &Entry{
		LoggedAt:   in.LoggedAt,
		QuantityMl: in.QuantityMl,
		Note:       in.Note,
		WorkoutID:  in.WorkoutID,
	}
	if err := s.repo.Insert(ctx, e); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, e.ID)
}

// Get returns a hydration entry by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Entry, error) {
	return s.repo.GetByID(ctx, id)
}

// PatchInput is the editable subset on PATCH /hydration/{id}.
//
// WorkoutID tri-state matches the meals patch shape: nil pointer = no change;
// non-nil pointer = set. ClearWorkoutID = true clears (handler converts the
// empty-string sentinel into this flag).
type PatchInput struct {
	QuantityMl     *float64
	LoggedAt       *time.Time
	Note           *string
	WorkoutID      *uuid.UUID
	ClearWorkoutID bool
}

// Patch validates and applies a partial update.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*Entry, error) {
	if in.QuantityMl != nil {
		if err := validateQuantity(*in.QuantityMl); err != nil {
			return nil, err
		}
	}
	if in.LoggedAt != nil {
		if err := validateLoggedAt(*in.LoggedAt); err != nil {
			return nil, err
		}
	}
	if err := validateNote(in.Note); err != nil {
		return nil, err
	}
	if in.WorkoutID != nil && !in.ClearWorkoutID {
		if err := s.validateWorkoutID(ctx, *in.WorkoutID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Patch(ctx, id, PatchParams{
		QuantityMl:     in.QuantityMl,
		LoggedAt:       in.LoggedAt,
		Note:           in.Note,
		WorkoutID:      in.WorkoutID,
		ClearWorkoutID: in.ClearWorkoutID,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a hydration entry.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List proxies to the repo.
func (s *Service) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	return s.repo.List(ctx, from, to)
}

// ----- validators -----

func validateQuantity(q float64) error {
	if math.IsNaN(q) || math.IsInf(q, 0) || q <= 0 {
		return ErrQuantityInvalid
	}
	return nil
}

func validateLoggedAt(ts time.Time) error {
	if ts.After(time.Now().Add(24 * time.Hour)) {
		return ErrLoggedAtFuture
	}
	return nil
}

func validateNote(n *string) error {
	if n == nil {
		return nil
	}
	if len(*n) > maxNoteLen {
		return ErrNoteTooLong
	}
	return nil
}
