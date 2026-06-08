package workouts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to the API error codes documented in the
// workouts capability spec.
var (
	ErrSourceInvalid        = errors.New("source_invalid")
	ErrSportInvalid         = errors.New("sport_invalid")
	ErrWindowInvalid        = errors.New("window_invalid")
	ErrStartedAtFarFuture   = errors.New("started_at_too_far_future")
	ErrKcalBurnedInvalid    = errors.New("kcal_burned_invalid")
	ErrAvgHRInvalid         = errors.New("avg_hr_invalid")
	ErrTSSInvalid           = errors.New("tss_invalid")
)

// Service orchestrates workout CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput is the payload for POST /workouts.
type CreateInput struct {
	ExternalID *string
	Source     string
	Sport      string
	Name       *string
	StartedAt  time.Time
	EndedAt    time.Time
	KcalBurned *float64
	AvgHR      *int
	TSS        *float64
	Notes      *string
}

// Upsert validates input and applies the UPSERT-by-external_id semantics. The
// returned bool is true when a new row was inserted; false when an existing
// row was updated.
func (s *Service) Upsert(ctx context.Context, in CreateInput) (*Workout, bool, error) {
	w, err := s.buildWorkout(in)
	if err != nil {
		return nil, false, err
	}
	created, err := s.repo.Upsert(ctx, w)
	if err != nil {
		return nil, false, err
	}
	return w, created, nil
}

// Get returns a single workout by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Workout, error) {
	return s.repo.GetByID(ctx, id)
}

// PatchInput is the editable subset of PATCH /workouts/{id}.
type PatchInput struct {
	Name       *string
	Notes      *string
	KcalBurned *float64
	AvgHR      *int
	TSS        *float64
}

// Patch validates and applies the partial update.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*Workout, error) {
	if in.KcalBurned != nil {
		if err := validateKcalBurned(*in.KcalBurned); err != nil {
			return nil, err
		}
	}
	if in.AvgHR != nil {
		if err := validateAvgHR(*in.AvgHR); err != nil {
			return nil, err
		}
	}
	if in.TSS != nil {
		if err := validateTSS(*in.TSS); err != nil {
			return nil, err
		}
	}
	params := PatchParams{
		Name:       in.Name,
		Notes:      in.Notes,
		KcalBurned: in.KcalBurned,
		AvgHR:      in.AvgHR,
		TSS:        in.TSS,
	}
	if err := s.repo.Patch(ctx, id, params); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a workout.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ListWindow returns workouts whose started_at falls within [from, to].
func (s *Service) ListWindow(ctx context.Context, from, to time.Time) ([]*Workout, error) {
	return s.repo.List(ctx, from, to)
}

// BulkItemResult carries the per-item outcome of a BulkUpsert call.
type BulkItemResult struct {
	Index   int
	ID      uuid.UUID
	Created bool
	Err     error
}

// BulkUpsert validates and upserts each item independently. Partial failure
// is allowed: each item's outcome is reported via its BulkItemResult.
func (s *Service) BulkUpsert(ctx context.Context, items []CreateInput) []BulkItemResult {
	results := make([]BulkItemResult, len(items))
	for i, in := range items {
		w, err := s.buildWorkout(in)
		if err != nil {
			results[i] = BulkItemResult{Index: i, Err: err}
			continue
		}
		created, err := s.repo.Upsert(ctx, w)
		if err != nil {
			results[i] = BulkItemResult{Index: i, Err: err}
			continue
		}
		results[i] = BulkItemResult{Index: i, ID: w.ID, Created: created}
	}
	return results
}

func (s *Service) buildWorkout(in CreateInput) (*Workout, error) {
	if !ValidSource(in.Source) {
		return nil, ErrSourceInvalid
	}
	if !ValidSport(in.Sport) {
		return nil, ErrSportInvalid
	}
	if in.StartedAt.IsZero() || in.EndedAt.IsZero() || !in.EndedAt.After(in.StartedAt) {
		return nil, ErrWindowInvalid
	}
	if in.StartedAt.After(time.Now().Add(24 * time.Hour)) {
		return nil, ErrStartedAtFarFuture
	}
	if in.KcalBurned != nil {
		if err := validateKcalBurned(*in.KcalBurned); err != nil {
			return nil, err
		}
	}
	if in.AvgHR != nil {
		if err := validateAvgHR(*in.AvgHR); err != nil {
			return nil, err
		}
	}
	if in.TSS != nil {
		if err := validateTSS(*in.TSS); err != nil {
			return nil, err
		}
	}
	return &Workout{
		ExternalID: in.ExternalID,
		Source:     Source(in.Source),
		Sport:      Sport(in.Sport),
		Name:       in.Name,
		StartedAt:  in.StartedAt,
		EndedAt:    in.EndedAt,
		KcalBurned: in.KcalBurned,
		AvgHR:      in.AvgHR,
		TSS:        in.TSS,
		Notes:      in.Notes,
	}, nil
}

func validateKcalBurned(v float64) error {
	if v <= 0 {
		return ErrKcalBurnedInvalid
	}
	return nil
}

func validateAvgHR(v int) error {
	if v <= 0 {
		return ErrAvgHRInvalid
	}
	return nil
}

func validateTSS(v float64) error {
	if v < 0 {
		return ErrTSSInvalid
	}
	return nil
}
