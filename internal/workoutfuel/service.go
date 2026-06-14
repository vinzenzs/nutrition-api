package workoutfuel

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrNameRequired    = errors.New("name_required")
	ErrEmptyEntry      = errors.New("empty_entry")
	ErrQuantityInvalid = errors.New("quantity_ml_invalid")
	ErrCarbsInvalid    = errors.New("carbs_g_invalid")
	ErrSodiumInvalid   = errors.New("sodium_mg_invalid")
	ErrPotassInvalid   = errors.New("potassium_mg_invalid")
	ErrCaffeineInvalid = errors.New("caffeine_mg_invalid")
	ErrLoggedAtFuture  = errors.New("logged_at_too_far_future")
	ErrNoteTooLong     = errors.New("note_too_long")
	ErrWorkoutNotFound = errors.New("workout_not_found")
)

const maxNoteLen = 500

// Service orchestrates workout-fuel entry CRUD over the repo.
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

// CreateInput is the payload for POST /workout-fuel.
type CreateInput struct {
	Name        string
	LoggedAt    time.Time
	QuantityMl  *float64
	CarbsG      *float64
	SodiumMg    *float64
	PotassiumMg *float64
	CaffeineMg  *float64
	Note        *string
	WorkoutID   *uuid.UUID
}

// Create validates and inserts a workout-fuel entry.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Entry, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrNameRequired
	}
	if err := validateQuantitative(in.QuantityMl, in.CarbsG, in.SodiumMg, in.PotassiumMg, in.CaffeineMg); err != nil {
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
		Name:        strings.TrimSpace(in.Name),
		LoggedAt:    in.LoggedAt,
		QuantityMl:  in.QuantityMl,
		CarbsG:      in.CarbsG,
		SodiumMg:    in.SodiumMg,
		PotassiumMg: in.PotassiumMg,
		CaffeineMg:  in.CaffeineMg,
		Note:        in.Note,
		WorkoutID:   in.WorkoutID,
	}
	if err := s.repo.Insert(ctx, e); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, e.ID)
}

// Get returns a workout-fuel entry by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Entry, error) {
	return s.repo.GetByID(ctx, id)
}

// PatchInput is the editable subset on PATCH /workout-fuel/{id}.
//
// Every quantitative field is tri-state via a (Value *float64, Clear bool)
// pair: nil + Clear=false means "leave alone", non-nil sets, Clear sets NULL.
// WorkoutID follows the meals/hydration empty-string-clear pattern.
type PatchInput struct {
	Name             *string
	LoggedAt         *time.Time
	QuantityMl       *float64
	ClearQuantityMl  bool
	CarbsG           *float64
	ClearCarbsG      bool
	SodiumMg         *float64
	ClearSodiumMg    bool
	PotassiumMg      *float64
	ClearPotassiumMg bool
	CaffeineMg       *float64
	ClearCaffeineMg  bool
	Note             *string
	ClearNote        bool
	WorkoutID        *uuid.UUID
	ClearWorkoutID   bool
}

// Patch validates and applies a partial update.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*Entry, error) {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return nil, ErrNameRequired
	}
	if in.QuantityMl != nil {
		if err := validateQuantityMl(*in.QuantityMl); err != nil {
			return nil, err
		}
	}
	if in.CarbsG != nil {
		if err := validateNonNegative(*in.CarbsG, ErrCarbsInvalid); err != nil {
			return nil, err
		}
	}
	if in.SodiumMg != nil {
		if err := validateNonNegative(*in.SodiumMg, ErrSodiumInvalid); err != nil {
			return nil, err
		}
	}
	if in.PotassiumMg != nil {
		if err := validateNonNegative(*in.PotassiumMg, ErrPotassInvalid); err != nil {
			return nil, err
		}
	}
	if in.CaffeineMg != nil {
		if err := validateNonNegative(*in.CaffeineMg, ErrCaffeineInvalid); err != nil {
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

	// Re-validate the merged state: refuse if the result would have all five
	// quantitative fields null. Fetch current row, project the patch on top,
	// then check.
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	mergedQml := projectFloat(current.QuantityMl, in.QuantityMl, in.ClearQuantityMl)
	mergedCarbs := projectFloat(current.CarbsG, in.CarbsG, in.ClearCarbsG)
	mergedSodium := projectFloat(current.SodiumMg, in.SodiumMg, in.ClearSodiumMg)
	mergedPotass := projectFloat(current.PotassiumMg, in.PotassiumMg, in.ClearPotassiumMg)
	mergedCaff := projectFloat(current.CaffeineMg, in.CaffeineMg, in.ClearCaffeineMg)
	if mergedQml == nil && mergedCarbs == nil && mergedSodium == nil && mergedPotass == nil && mergedCaff == nil {
		return nil, ErrEmptyEntry
	}

	if err := s.repo.Patch(ctx, id, PatchParams{
		Name:             in.Name,
		LoggedAt:         in.LoggedAt,
		QuantityMl:       in.QuantityMl,
		ClearQuantityMl:  in.ClearQuantityMl,
		CarbsG:           in.CarbsG,
		ClearCarbsG:      in.ClearCarbsG,
		SodiumMg:         in.SodiumMg,
		ClearSodiumMg:    in.ClearSodiumMg,
		PotassiumMg:      in.PotassiumMg,
		ClearPotassiumMg: in.ClearPotassiumMg,
		CaffeineMg:       in.CaffeineMg,
		ClearCaffeineMg:  in.ClearCaffeineMg,
		Note:             in.Note,
		ClearNote:        in.ClearNote,
		WorkoutID:        in.WorkoutID,
		ClearWorkoutID:   in.ClearWorkoutID,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes a workout-fuel entry.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// List proxies to the repo.
func (s *Service) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	return s.repo.List(ctx, from, to)
}

// ----- validators -----

// projectFloat applies a patch field to a current value:
//
//	clear=true → nil (set to NULL)
//	patch!=nil → patch (set to new value)
//	otherwise  → current (no change)
func projectFloat(current, patch *float64, clear bool) *float64 {
	switch {
	case clear:
		return nil
	case patch != nil:
		return patch
	default:
		return current
	}
}

func validateQuantitative(qml, carbs, sodium, potass, caff *float64) error {
	if qml != nil {
		if err := validateQuantityMl(*qml); err != nil {
			return err
		}
	}
	if carbs != nil {
		if err := validateNonNegative(*carbs, ErrCarbsInvalid); err != nil {
			return err
		}
	}
	if sodium != nil {
		if err := validateNonNegative(*sodium, ErrSodiumInvalid); err != nil {
			return err
		}
	}
	if potass != nil {
		if err := validateNonNegative(*potass, ErrPotassInvalid); err != nil {
			return err
		}
	}
	if caff != nil {
		if err := validateNonNegative(*caff, ErrCaffeineInvalid); err != nil {
			return err
		}
	}
	if qml == nil && carbs == nil && sodium == nil && potass == nil && caff == nil {
		return ErrEmptyEntry
	}
	return nil
}

func validateQuantityMl(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return ErrQuantityInvalid
	}
	return nil
}

func validateNonNegative(v float64, code error) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return code
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
