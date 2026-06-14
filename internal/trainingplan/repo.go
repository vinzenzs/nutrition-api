package trainingplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vinzenzs/kazper/internal/store"
)

var (
	// ErrPlanNotFound / ErrWeekNotFound / ErrSlotNotFound map to 404s.
	ErrPlanNotFound = errors.New("training plan not found")
	ErrWeekNotFound = errors.New("plan week not found")
	ErrSlotNotFound = errors.New("plan slot not found")
	// ErrTemplateMissing is returned when a slot references an unknown template
	// (FK violation on insert/update).
	ErrTemplateMissing = errors.New("template_not_found")
	// ErrWeekOrdinalTaken is returned when (plan_id, ordinal) already exists.
	ErrWeekOrdinalTaken = errors.New("week_ordinal_taken")
)

// Repo persists training plans, weeks, and slots.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const (
	planCols = `id, name, race_id, to_char(start_date, 'YYYY-MM-DD') AS start_date, notes, created_at, updated_at`
	weekCols = `id, plan_id, ordinal, phase_id, notes, created_at, updated_at`
	slotCols = `id, plan_week_id, weekday, ordinal, template_id, to_char(time_of_day, 'HH24:MI:SS') AS time_of_day, target_overrides, created_at, updated_at`
)

// ----- plan -----

func (r *Repo) CreatePlan(ctx context.Context, p *Plan) (*Plan, error) {
	row := r.q.QueryRow(ctx, `
        INSERT INTO training_plans (name, race_id, start_date, notes, created_at, updated_at)
        VALUES ($1, $2, $3::date, $4, now(), now())
        RETURNING `+planCols,
		p.Name, p.RaceID, p.StartDate, p.Notes,
	)
	return scanPlan(row)
}

func (r *Repo) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	row := r.q.QueryRow(ctx, `SELECT `+planCols+` FROM training_plans WHERE id = $1`, id)
	return scanPlan(row)
}

func (r *Repo) ListPlans(ctx context.Context) ([]*Plan, error) {
	rows, err := r.q.Query(ctx, `SELECT `+planCols+` FROM training_plans ORDER BY start_date DESC`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()
	var out []*Plan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdatePlan writes the mutable plan columns from p (load-modify-write).
func (r *Repo) UpdatePlan(ctx context.Context, p *Plan) (*Plan, error) {
	row := r.q.QueryRow(ctx, `
        UPDATE training_plans
        SET name = $2, race_id = $3, start_date = $4::date, notes = $5, updated_at = now()
        WHERE id = $1
        RETURNING `+planCols,
		p.ID, p.Name, p.RaceID, p.StartDate, p.Notes,
	)
	return scanPlan(row)
}

func (r *Repo) DeletePlan(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM training_plans WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}
	return nil
}

// Load returns the full nested tree: plan → weeks (ordinal asc) → slots
// (weekday, ordinal asc).
func (r *Repo) Load(ctx context.Context, id uuid.UUID) (*Plan, error) {
	p, err := r.GetPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	weekRows, err := r.q.Query(ctx, `SELECT `+weekCols+` FROM plan_weeks WHERE plan_id = $1 ORDER BY ordinal ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("load weeks: %w", err)
	}
	defer weekRows.Close()
	weeks := []PlanWeek{}
	byID := map[uuid.UUID]int{}
	for weekRows.Next() {
		w, err := scanWeek(weekRows)
		if err != nil {
			return nil, err
		}
		w.Slots = []PlanSlot{}
		weeks = append(weeks, *w)
		byID[w.ID] = len(weeks) - 1
	}
	if err := weekRows.Err(); err != nil {
		return nil, err
	}
	// Slots across the whole plan in one query, attached to their week.
	slotRows, err := r.q.Query(ctx, `
        SELECT `+slotCols+`
        FROM plan_slots
        WHERE plan_week_id IN (SELECT id FROM plan_weeks WHERE plan_id = $1)
        ORDER BY weekday ASC, ordinal ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("load slots: %w", err)
	}
	defer slotRows.Close()
	for slotRows.Next() {
		s, err := scanSlot(slotRows)
		if err != nil {
			return nil, err
		}
		if idx, ok := byID[s.PlanWeekID]; ok {
			weeks[idx].Slots = append(weeks[idx].Slots, *s)
		}
	}
	if err := slotRows.Err(); err != nil {
		return nil, err
	}
	p.Weeks = weeks
	return p, nil
}

// ----- week -----

func (r *Repo) CreateWeek(ctx context.Context, w *PlanWeek) (*PlanWeek, error) {
	row := r.q.QueryRow(ctx, `
        INSERT INTO plan_weeks (plan_id, ordinal, phase_id, notes, created_at, updated_at)
        VALUES ($1, $2, $3, $4, now(), now())
        RETURNING `+weekCols,
		w.PlanID, w.Ordinal, w.PhaseID, w.Notes,
	)
	out, err := scanWeek(row)
	return out, mapWriteErr(err)
}

func (r *Repo) GetWeek(ctx context.Context, id uuid.UUID) (*PlanWeek, error) {
	row := r.q.QueryRow(ctx, `SELECT `+weekCols+` FROM plan_weeks WHERE id = $1`, id)
	w, err := scanWeek(row)
	if errors.Is(err, ErrPlanNotFound) {
		return nil, ErrWeekNotFound
	}
	return w, err
}

func (r *Repo) UpdateWeek(ctx context.Context, w *PlanWeek) (*PlanWeek, error) {
	row := r.q.QueryRow(ctx, `
        UPDATE plan_weeks
        SET ordinal = $2, phase_id = $3, notes = $4, updated_at = now()
        WHERE id = $1
        RETURNING `+weekCols,
		w.ID, w.Ordinal, w.PhaseID, w.Notes,
	)
	out, err := scanWeek(row)
	if errors.Is(err, ErrPlanNotFound) {
		return nil, ErrWeekNotFound
	}
	return out, mapWriteErr(err)
}

func (r *Repo) DeleteWeek(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM plan_weeks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete week: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWeekNotFound
	}
	return nil
}

// ----- slot -----

func (r *Repo) CreateSlot(ctx context.Context, s *PlanSlot) (*PlanSlot, error) {
	overrides, err := marshalOverrides(s.TargetOverrides)
	if err != nil {
		return nil, err
	}
	row := r.q.QueryRow(ctx, `
        INSERT INTO plan_slots (plan_week_id, weekday, ordinal, template_id, time_of_day, target_overrides, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5::time, $6::jsonb, now(), now())
        RETURNING `+slotCols,
		s.PlanWeekID, s.Weekday, s.Ordinal, s.TemplateID, s.TimeOfDay, overrides,
	)
	out, err := scanSlot(row)
	return out, mapWriteErr(err)
}

func (r *Repo) GetSlot(ctx context.Context, id uuid.UUID) (*PlanSlot, error) {
	row := r.q.QueryRow(ctx, `SELECT `+slotCols+` FROM plan_slots WHERE id = $1`, id)
	s, err := scanSlot(row)
	if errors.Is(err, ErrPlanNotFound) {
		return nil, ErrSlotNotFound
	}
	return s, err
}

func (r *Repo) UpdateSlot(ctx context.Context, s *PlanSlot) (*PlanSlot, error) {
	overrides, err := marshalOverrides(s.TargetOverrides)
	if err != nil {
		return nil, err
	}
	row := r.q.QueryRow(ctx, `
        UPDATE plan_slots
        SET weekday = $2, ordinal = $3, template_id = $4, time_of_day = $5::time, target_overrides = $6::jsonb, updated_at = now()
        WHERE id = $1
        RETURNING `+slotCols,
		s.ID, s.Weekday, s.Ordinal, s.TemplateID, s.TimeOfDay, overrides,
	)
	out, err := scanSlot(row)
	if errors.Is(err, ErrPlanNotFound) {
		return nil, ErrSlotNotFound
	}
	return out, mapWriteErr(err)
}

func (r *Repo) DeleteSlot(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM plan_slots WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete slot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSlotNotFound
	}
	return nil
}

// PlannedWorkoutsInScope returns the ids of planned workouts materialized from
// a plan, narrowed to a scope: a single week (by ordinal), a date range (by
// started_at), or all. Used by the Garmin-scheduling plan push to loop the
// single-workout path. Joins workouts to the plan via plan_slot_id.
func (r *Repo) PlannedWorkoutsInScope(ctx context.Context, planID uuid.UUID, kind string, week *int, from, to *string) ([]uuid.UUID, error) {
	q := `
        SELECT w.id
        FROM workouts w
        JOIN plan_slots s ON s.id = w.plan_slot_id
        JOIN plan_weeks wk ON wk.id = s.plan_week_id
        WHERE wk.plan_id = $1 AND w.status = 'planned'`
	args := []any{planID}
	switch kind {
	case "week":
		q += ` AND wk.ordinal = $2`
		args = append(args, *week)
	case "range":
		q += ` AND w.started_at >= $2::date AND w.started_at < ($3::date + INTERVAL '1 day')`
		args = append(args, *from, *to)
	case "all":
		// no extra filter
	default:
		return nil, ErrScopeInvalid
	}
	q += ` ORDER BY w.started_at ASC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("planned workouts in scope: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// materializeSlot is one slot joined with its week ordinal and template detail,
// everything Materialize needs to compute a planned workout.
type materializeSlot struct {
	SlotID              uuid.UUID
	WeekOrdinal         int
	Weekday             int
	SlotOrdinal         int
	TimeOfDay           *string
	TemplateID          uuid.UUID
	TemplateSport       string
	TemplateName        string
	TemplateDurationSec *int
}

// loadMaterializeSlots returns every slot of a plan joined with its week ordinal
// and template detail, ordered by (week, weekday, slot). Runs against the given
// Querier so it can share the materialize transaction.
func (r *Repo) loadMaterializeSlots(ctx context.Context, q store.Querier, planID uuid.UUID) ([]materializeSlot, error) {
	rows, err := q.Query(ctx, `
        SELECT s.id, w.ordinal, s.weekday, s.ordinal,
               to_char(s.time_of_day, 'HH24:MI:SS'),
               t.id, t.sport, t.name, t.estimated_duration_sec
        FROM plan_slots s
        JOIN plan_weeks w ON w.id = s.plan_week_id
        JOIN workout_templates t ON t.id = s.template_id
        WHERE w.plan_id = $1
        ORDER BY w.ordinal ASC, s.weekday ASC, s.ordinal ASC`, planID)
	if err != nil {
		return nil, fmt.Errorf("load materialize slots: %w", err)
	}
	defer rows.Close()
	var out []materializeSlot
	for rows.Next() {
		var m materializeSlot
		if err := rows.Scan(&m.SlotID, &m.WeekOrdinal, &m.Weekday, &m.SlotOrdinal,
			&m.TimeOfDay, &m.TemplateID, &m.TemplateSport, &m.TemplateName, &m.TemplateDurationSec); err != nil {
			return nil, fmt.Errorf("scan materialize slot: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ----- scanning + error mapping -----

type scanner interface {
	Scan(dest ...any) error
}

func scanPlan(s scanner) (*Plan, error) {
	var p Plan
	err := s.Scan(&p.ID, &p.Name, &p.RaceID, &p.StartDate, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlanNotFound
		}
		return nil, fmt.Errorf("scan plan: %w", err)
	}
	return &p, nil
}

func scanWeek(s scanner) (*PlanWeek, error) {
	var w PlanWeek
	err := s.Scan(&w.ID, &w.PlanID, &w.Ordinal, &w.PhaseID, &w.Notes, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlanNotFound // remapped by callers to ErrWeekNotFound
		}
		return nil, fmt.Errorf("scan week: %w", err)
	}
	return &w, nil
}

func scanSlot(s scanner) (*PlanSlot, error) {
	var sl PlanSlot
	var overrides []byte
	err := s.Scan(&sl.ID, &sl.PlanWeekID, &sl.Weekday, &sl.Ordinal, &sl.TemplateID, &sl.TimeOfDay, &overrides, &sl.CreatedAt, &sl.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlanNotFound // remapped by callers to ErrSlotNotFound
		}
		return nil, fmt.Errorf("scan slot: %w", err)
	}
	if len(overrides) > 0 {
		if err := json.Unmarshal(overrides, &sl.TargetOverrides); err != nil {
			return nil, fmt.Errorf("scan slot target_overrides: %w", err)
		}
	}
	return &sl, nil
}

// marshalOverrides returns the JSONB bytes for a slot's overrides, or nil (SQL
// NULL) when there are none.
func marshalOverrides(o []SlotTargetOverride) ([]byte, error) {
	if len(o) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("marshal slot target_overrides: %w", err)
	}
	return b, nil
}

// mapWriteErr turns Postgres FK / unique violations into typed sentinels.
func mapWriteErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503": // foreign_key_violation — a missing template (or phase/plan)
			return ErrTemplateMissing
		case "23505": // unique_violation — (plan_id, ordinal)
			return ErrWeekOrdinalTaken
		}
	}
	return err
}
