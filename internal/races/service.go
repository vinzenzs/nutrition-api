package races

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/kazper/internal/store"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrNameRequired         = errors.New("race_name_required")
	ErrRaceDateInvalid      = errors.New("race_date_invalid")
	ErrLegOrdinalDuplicate  = errors.New("leg_ordinal_duplicate")
	ErrLegDisciplineInvalid = errors.New("leg_discipline_invalid")
	ErrLegDurationInvalid   = errors.New("leg_expected_duration_min_invalid")
	ErrLegDistanceInvalid   = errors.New("leg_distance_m_invalid")
	ErrNotesTooLong         = errors.New("notes_too_long")

	ErrBodyWeightRequired = errors.New("body_weight_kg_required")
	ErrBodyWeightRange    = errors.New("body_weight_kg_out_of_range")
	ErrSweatRateRange     = errors.New("sweat_rate_out_of_range")
)

const (
	bodyWeightKgMin = 30.0
	bodyWeightKgMax = 200.0
	sweatRateMlMax  = 5000.0
	maxNameLen      = 200
	maxNotesLen     = 2000
)

// Service orchestrates race CRUD and fuelling-plan computation. It holds the
// pool so race+legs writes are atomic; the repo runs against pool or tx.
type Service struct {
	pool *pgxpool.Pool
	repo *Repo
}

func NewService(pool *pgxpool.Pool, repo *Repo) *Service {
	return &Service{pool: pool, repo: repo}
}

// LegInput is a leg supplied on create/update.
type LegInput struct {
	Ordinal             int
	Discipline          Discipline
	DistanceM           *float64
	ExpectedDurationMin *int
	Intensity           *string
}

// CreateInput is the payload for POST /races.
type CreateInput struct {
	Name     string
	RaceDate string
	RaceType *string
	Location *string
	Notes    *string
	Legs     []LegInput
}

// Create validates and persists a race with its legs atomically.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Race, error) {
	if err := validateName(in.Name); err != nil {
		return nil, err
	}
	if err := validateNotes(in.Notes); err != nil {
		return nil, err
	}
	date, err := parseRaceDate(in.RaceDate)
	if err != nil {
		return nil, err
	}
	if err := validateLegs(in.Legs); err != nil {
		return nil, err
	}

	race := &Race{
		Name:     strings.TrimSpace(in.Name),
		RaceType: in.RaceType,
		Location: in.Location,
		Notes:    in.Notes,
	}
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := NewRepo(tx)
		if err := r.InsertRace(ctx, race, date); err != nil {
			return err
		}
		return insertLegs(ctx, r, race.ID, in.Legs)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetRace(ctx, race.ID)
}

// Get returns a race with legs.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Race, error) {
	return s.repo.GetRace(ctx, id)
}

// List returns all races with legs.
func (s *Service) List(ctx context.Context) ([]*Race, error) {
	return s.repo.ListRaces(ctx)
}

// UpdateInput is the editable subset on PATCH /races/{id}. Nil scalar pointers
// leave the field unchanged; a non-nil Legs slice replaces all legs wholesale.
type UpdateInput struct {
	Name     *string
	RaceDate *string
	RaceType *string
	Location *string
	Notes    *string
	Legs     *[]LegInput
}

// Update validates and applies a partial update, optionally replacing legs.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Race, error) {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return nil, ErrNameRequired
	}
	if in.Name != nil && len(*in.Name) > maxNameLen {
		return nil, ErrNameRequired
	}
	if err := validateNotes(in.Notes); err != nil {
		return nil, err
	}
	var datePtr *time.Time
	if in.RaceDate != nil {
		d, err := parseRaceDate(*in.RaceDate)
		if err != nil {
			return nil, err
		}
		datePtr = &d
	}
	if in.Legs != nil {
		if err := validateLegs(*in.Legs); err != nil {
			return nil, err
		}
	}

	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := NewRepo(tx)
		if err := r.UpdateRace(ctx, id, UpdateRaceParams{
			Name:     trimPtr(in.Name),
			RaceDate: datePtr,
			RaceType: in.RaceType,
			Location: in.Location,
			Notes:    in.Notes,
		}); err != nil {
			return err
		}
		if in.Legs != nil {
			if err := r.DeleteLegsForRace(ctx, id); err != nil {
				return err
			}
			return insertLegs(ctx, r, id, *in.Legs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetRace(ctx, id)
}

// Delete removes a race (legs cascade).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteRace(ctx, id)
}

// PlanFueling computes the per-leg fuelling plan for a stored race.
func (s *Service) PlanFueling(ctx context.Context, id uuid.UUID, p FuelingParams) (*FuelingPlan, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	race, err := s.repo.GetRace(ctx, id)
	if err != nil {
		return nil, err
	}
	return ComputeFueling(race, p), nil
}

// ----- validators -----

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrNameRequired
	}
	if len(name) > maxNameLen {
		return ErrNameRequired
	}
	return nil
}

func validateNotes(notes *string) error {
	if notes != nil && len(*notes) > maxNotesLen {
		return ErrNotesTooLong
	}
	return nil
}

func parseRaceDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(dateLayout, strings.TrimSpace(s), time.UTC)
	if err != nil {
		return time.Time{}, ErrRaceDateInvalid
	}
	return t, nil
}

func validateLegs(legs []LegInput) error {
	seen := map[int]bool{}
	for _, leg := range legs {
		if seen[leg.Ordinal] {
			return ErrLegOrdinalDuplicate
		}
		seen[leg.Ordinal] = true
		if !leg.Discipline.valid() {
			return ErrLegDisciplineInvalid
		}
		if leg.ExpectedDurationMin != nil && *leg.ExpectedDurationMin <= 0 {
			return ErrLegDurationInvalid
		}
		if leg.DistanceM != nil {
			d := *leg.DistanceM
			if math.IsNaN(d) || math.IsInf(d, 0) || d <= 0 {
				return ErrLegDistanceInvalid
			}
		}
	}
	return nil
}

func insertLegs(ctx context.Context, r *Repo, raceID uuid.UUID, legs []LegInput) error {
	for _, in := range legs {
		leg := &RaceLeg{
			Ordinal:             in.Ordinal,
			Discipline:          in.Discipline,
			DistanceM:           in.DistanceM,
			ExpectedDurationMin: in.ExpectedDurationMin,
			Intensity:           in.Intensity,
		}
		if err := r.InsertLeg(ctx, raceID, leg); err != nil {
			return err
		}
	}
	return nil
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
