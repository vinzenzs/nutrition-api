package races

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a race does not exist.
var ErrNotFound = errors.New("race not found")

// Repo persists races and their legs against a store.Querier (pool or tx).
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const raceCols = `id, name, race_date, race_type, location, notes, created_at, updated_at`
const legCols = `id, ordinal, discipline, distance_m, expected_duration_min, intensity`

// InsertRace inserts the race row (legs are inserted separately, in the same tx).
// The supplied Race's ID/CreatedAt/UpdatedAt are set on success.
func (r *Repo) InsertRace(ctx context.Context, race *Race, raceDate time.Time) error {
	if race.ID == uuid.Nil {
		race.ID = uuid.New()
	}
	now := time.Now().UTC()
	const q = `
        INSERT INTO races (id, name, race_date, race_type, location, notes, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`
	if _, err := r.q.Exec(ctx, q,
		race.ID, race.Name, raceDate, race.RaceType, race.Location, race.Notes, now,
	); err != nil {
		return fmt.Errorf("insert race: %w", err)
	}
	race.RaceDate = raceDate.Format(dateLayout)
	race.CreatedAt = now
	race.UpdatedAt = now
	return nil
}

// InsertLeg inserts one leg for a race. The leg's ID is set on success.
func (r *Repo) InsertLeg(ctx context.Context, raceID uuid.UUID, leg *RaceLeg) error {
	if leg.ID == uuid.Nil {
		leg.ID = uuid.New()
	}
	const q = `
        INSERT INTO race_legs (id, race_id, ordinal, discipline, distance_m, expected_duration_min, intensity)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := r.q.Exec(ctx, q,
		leg.ID, raceID, leg.Ordinal, string(leg.Discipline),
		leg.DistanceM, leg.ExpectedDurationMin, leg.Intensity,
	); err != nil {
		return fmt.Errorf("insert race leg: %w", err)
	}
	return nil
}

// GetRace returns a race with its legs ordered by ordinal. ErrNotFound if absent.
func (r *Repo) GetRace(ctx context.Context, id uuid.UUID) (*Race, error) {
	row := r.q.QueryRow(ctx, `SELECT `+raceCols+` FROM races WHERE id = $1`, id)
	race, err := scanRace(row)
	if err != nil {
		return nil, err
	}
	legs, err := r.legsForRace(ctx, id)
	if err != nil {
		return nil, err
	}
	race.Legs = legs
	return race, nil
}

// ListRaces returns all races (each with its legs) ordered by race_date ASC.
func (r *Repo) ListRaces(ctx context.Context) ([]*Race, error) {
	rows, err := r.q.Query(ctx, `SELECT `+raceCols+` FROM races ORDER BY race_date ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list races: %w", err)
	}
	defer rows.Close()
	var out []*Race
	for rows.Next() {
		race, err := scanRace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, race)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, race := range out {
		legs, err := r.legsForRace(ctx, race.ID)
		if err != nil {
			return nil, err
		}
		race.Legs = legs
	}
	return out, nil
}

func (r *Repo) legsForRace(ctx context.Context, raceID uuid.UUID) ([]*RaceLeg, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+legCols+` FROM race_legs WHERE race_id = $1 ORDER BY ordinal ASC`, raceID)
	if err != nil {
		return nil, fmt.Errorf("list race legs: %w", err)
	}
	defer rows.Close()
	legs := []*RaceLeg{}
	for rows.Next() {
		var leg RaceLeg
		var disc string
		if err := rows.Scan(&leg.ID, &leg.Ordinal, &disc, &leg.DistanceM, &leg.ExpectedDurationMin, &leg.Intensity); err != nil {
			return nil, fmt.Errorf("scan race leg: %w", err)
		}
		leg.Discipline = Discipline(disc)
		legs = append(legs, &leg)
	}
	return legs, rows.Err()
}

// UpdateRaceParams holds the scalar race fields to update. A nil pointer leaves
// the column unchanged.
type UpdateRaceParams struct {
	Name     *string
	RaceDate *time.Time
	RaceType *string
	Location *string
	Notes    *string
}

// UpdateRace updates the scalar fields. ErrNotFound if no row matches.
func (r *Repo) UpdateRace(ctx context.Context, id uuid.UUID, p UpdateRaceParams) error {
	const q = `
        UPDATE races SET
            name      = COALESCE($2, name),
            race_date = COALESCE($3, race_date),
            race_type = CASE WHEN $4::boolean THEN $5 ELSE race_type END,
            location  = CASE WHEN $6::boolean THEN $7 ELSE location END,
            notes     = CASE WHEN $8::boolean THEN $9 ELSE notes END,
            updated_at = now()
        WHERE id = $1`
	tag, err := r.q.Exec(ctx, q, id,
		p.Name, p.RaceDate,
		p.RaceType != nil, p.RaceType,
		p.Location != nil, p.Location,
		p.Notes != nil, p.Notes,
	)
	if err != nil {
		return fmt.Errorf("update race: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteLegsForRace removes all legs of a race (used by ReplaceLegs, in a tx).
func (r *Repo) DeleteLegsForRace(ctx context.Context, raceID uuid.UUID) error {
	if _, err := r.q.Exec(ctx, `DELETE FROM race_legs WHERE race_id = $1`, raceID); err != nil {
		return fmt.Errorf("delete race legs: %w", err)
	}
	return nil
}

// DeleteRace removes a race; legs cascade. ErrNotFound if no row matched.
func (r *Repo) DeleteRace(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM races WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete race: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRace(s interface{ Scan(...any) error }) (*Race, error) {
	var race Race
	var raceDate time.Time
	err := s.Scan(&race.ID, &race.Name, &raceDate, &race.RaceType, &race.Location, &race.Notes, &race.CreatedAt, &race.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan race: %w", err)
	}
	race.RaceDate = raceDate.Format(dateLayout)
	race.Legs = []*RaceLeg{}
	return &race, nil
}
