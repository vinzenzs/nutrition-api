package workouts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vinzenzs/kazper/internal/store"
)

// ErrNotFound is returned when a workout does not exist.
var ErrNotFound = errors.New("workout not found")

// Repo persists workouts.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

const selectCols = `id, external_id, source, sport, status, name, started_at, ended_at, kcal_burned, avg_hr, tss, rpe, gi_distress_score, distance_m, avg_power_w, temperature_c, sweat_loss_ml, session_group, template_id, plan_slot_id, garmin_workout_id, garmin_schedule_id, needs_link, notes, created_at, updated_at, elevation_gain_m, elevation_loss_m, normalized_power_w, intensity_factor, avg_cadence, avg_stride_m, max_hr, aerobic_te, anaerobic_te, secs_in_zone_1, secs_in_zone_2, secs_in_zone_3, secs_in_zone_4, secs_in_zone_5, humidity_pct, wind_speed_mps`

// Upsert inserts a new workout row, or updates an existing row when
// external_id collides with the partial unique index. Returns `created=true`
// when the row was inserted (the caller maps this to HTTP 201) and `false`
// when an existing row was updated (HTTP 200).
//
// The partial unique index `workouts_external_id_uidx ON (external_id) WHERE
// external_id IS NOT NULL` is critical: rows with NULL external_id never
// conflict, so manual workouts always INSERT.
func (r *Repo) Upsert(ctx context.Context, w *Workout) (created bool, err error) {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	now := time.Now().UTC()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	w.UpdatedAt = now
	// Default status so direct repo callers (and the DB CHECK) never see an
	// empty string; the service already defaults it, this protects the rest.
	if w.Status == "" {
		w.Status = StatusCompleted
	}

	const q = `
        INSERT INTO workouts (
            id, external_id, source, sport, name,
            started_at, ended_at,
            kcal_burned, avg_hr, tss,
            rpe, gi_distress_score,
            distance_m, avg_power_w, temperature_c, sweat_loss_ml, session_group,
            notes, status, needs_link,
            created_at, updated_at,
            elevation_gain_m, elevation_loss_m, normalized_power_w, intensity_factor,
            avg_cadence, avg_stride_m, max_hr, aerobic_te, anaerobic_te,
            secs_in_zone_1, secs_in_zone_2, secs_in_zone_3, secs_in_zone_4, secs_in_zone_5,
            humidity_pct, wind_speed_mps
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7,
            $8, $9, $10,
            $11, $12,
            $13, $14, $15, $16, $17,
            $18, $19, $22,
            $20, $21,
            $23, $24, $25, $26,
            $27, $28, $29, $30, $31,
            $32, $33, $34, $35, $36,
            $37, $38
        )
        ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO UPDATE SET
            source             = EXCLUDED.source,
            sport              = EXCLUDED.sport,
            name               = EXCLUDED.name,
            started_at         = EXCLUDED.started_at,
            ended_at           = EXCLUDED.ended_at,
            kcal_burned        = EXCLUDED.kcal_burned,
            avg_hr             = EXCLUDED.avg_hr,
            tss                = EXCLUDED.tss,
            rpe                = EXCLUDED.rpe,
            gi_distress_score  = EXCLUDED.gi_distress_score,
            distance_m         = EXCLUDED.distance_m,
            avg_power_w        = EXCLUDED.avg_power_w,
            temperature_c      = EXCLUDED.temperature_c,
            sweat_loss_ml      = EXCLUDED.sweat_loss_ml,
            session_group      = EXCLUDED.session_group,
            notes              = EXCLUDED.notes,
            status             = EXCLUDED.status,
            updated_at         = EXCLUDED.updated_at,
            elevation_gain_m   = EXCLUDED.elevation_gain_m,
            elevation_loss_m   = EXCLUDED.elevation_loss_m,
            normalized_power_w = EXCLUDED.normalized_power_w,
            intensity_factor   = EXCLUDED.intensity_factor,
            avg_cadence        = EXCLUDED.avg_cadence,
            avg_stride_m       = EXCLUDED.avg_stride_m,
            max_hr             = EXCLUDED.max_hr,
            aerobic_te         = EXCLUDED.aerobic_te,
            anaerobic_te       = EXCLUDED.anaerobic_te,
            secs_in_zone_1     = EXCLUDED.secs_in_zone_1,
            secs_in_zone_2     = EXCLUDED.secs_in_zone_2,
            secs_in_zone_3     = EXCLUDED.secs_in_zone_3,
            secs_in_zone_4     = EXCLUDED.secs_in_zone_4,
            secs_in_zone_5     = EXCLUDED.secs_in_zone_5,
            humidity_pct       = EXCLUDED.humidity_pct,
            wind_speed_mps     = EXCLUDED.wind_speed_mps
        RETURNING id, created_at = $20 AS inserted
    `
	row := r.q.QueryRow(ctx, q,
		w.ID, w.ExternalID, string(w.Source), string(w.Sport), w.Name,
		w.StartedAt, w.EndedAt,
		w.KcalBurned, w.AvgHR, w.TSS,
		w.RPE, w.GIDistressScore,
		w.DistanceM, w.AvgPowerW, w.TemperatureC, w.SweatLossML, w.SessionGroup,
		w.Notes, string(w.Status),
		w.CreatedAt, w.UpdatedAt,
		w.NeedsLink,
		w.ElevationGainM, w.ElevationLossM, w.NormalizedPowerW, w.IntensityFactor,
		w.AvgCadence, w.AvgStrideM, w.MaxHR, w.AerobicTE, w.AnaerobicTE,
		w.SecsInZone1, w.SecsInZone2, w.SecsInZone3, w.SecsInZone4, w.SecsInZone5,
		w.HumidityPct, w.WindSpeedMPS,
	)
	var (
		returnedID uuid.UUID
		inserted   bool
	)
	if err := row.Scan(&returnedID, &inserted); err != nil {
		return false, fmt.Errorf("upsert workout: %w", err)
	}
	// On UPDATE the existing row's id wins; reflect it back onto the caller's
	// pointer so the response carries the persisted id.
	w.ID = returnedID
	if !inserted {
		// Re-read so the caller sees authoritative timestamps + any fields the
		// caller didn't supply but the existing row had.
		fresh, err := r.GetByID(ctx, returnedID)
		if err != nil {
			return false, fmt.Errorf("re-read after update: %w", err)
		}
		*w = *fresh
	}
	return inserted, nil
}

// PlannedSlotInput is the data the training-plan materializer supplies to
// create/refresh one planned workout from a plan slot.
type PlannedSlotInput struct {
	PlanSlotID   uuid.UUID
	TemplateID   uuid.UUID
	Sport        string
	Name         *string
	StartedAt    time.Time
	EndedAt      time.Time
	SessionGroup *string
}

// UpsertPlannedFromSlot creates (or refreshes) the planned workout for a plan
// slot, keyed on plan_slot_id via the partial-unique index. It runs against the
// supplied Querier so the materializer can call it inside a transaction.
//
// The `WHERE workouts.status = 'planned'` guard on the UPDATE is load-bearing:
// once a future reconciliation flips a slot's workout to 'completed' (keeping
// its plan_slot_id), re-materializing must NOT clobber it back to 'planned'.
// When the guard blocks the update, RETURNING yields no row; we then return the
// existing (completed) row unchanged so the caller can report it was skipped.
//
// This path is disjoint from the external_id Upsert: planned-from-plan rows
// carry plan_slot_id and never an external_id, so the two upserts never collide.
func (r *Repo) UpsertPlannedFromSlot(ctx context.Context, q store.Querier, in PlannedSlotInput) (*Workout, error) {
	const sql = `
        INSERT INTO workouts (
            id, source, sport, name, status,
            started_at, ended_at, template_id, plan_slot_id, session_group,
            created_at, updated_at
        ) VALUES (
            gen_random_uuid(), 'manual', $1, $2, 'planned',
            $3, $4, $5, $6, $7,
            now(), now()
        )
        ON CONFLICT (plan_slot_id) WHERE plan_slot_id IS NOT NULL DO UPDATE SET
            sport         = EXCLUDED.sport,
            name          = EXCLUDED.name,
            status        = 'planned',
            template_id   = EXCLUDED.template_id,
            started_at    = EXCLUDED.started_at,
            ended_at      = EXCLUDED.ended_at,
            session_group = EXCLUDED.session_group,
            updated_at    = now()
        WHERE workouts.status = 'planned'
        RETURNING ` + selectCols
	row := q.QueryRow(ctx, sql,
		in.Sport, in.Name, in.StartedAt, in.EndedAt, in.TemplateID, in.PlanSlotID, in.SessionGroup,
	)
	w, err := scanWorkout(row)
	if errors.Is(err, ErrNotFound) {
		// The conflict row exists but is 'completed' (guard blocked the update).
		// Return it unchanged so materialize reports a skip, not a failure.
		return r.getByPlanSlotID(ctx, q, in.PlanSlotID)
	}
	return w, err
}

// getByPlanSlotID fetches the workout owning a plan slot, against the given Querier.
func (r *Repo) getByPlanSlotID(ctx context.Context, q store.Querier, planSlotID uuid.UUID) (*Workout, error) {
	row := q.QueryRow(ctx, `SELECT `+selectCols+` FROM workouts WHERE plan_slot_id = $1`, planSlotID)
	return scanWorkout(row)
}

// GetByID returns a single workout row.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*Workout, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM workouts WHERE id = $1`, id)
	return scanWorkout(row)
}

// GetByExternalID returns the workout owning external_id. ErrNotFound if none —
// used by reconciliation to distinguish a first-sight import from a re-sync.
func (r *Repo) GetByExternalID(ctx context.Context, externalID string) (*Workout, error) {
	row := r.q.QueryRow(ctx, `SELECT `+selectCols+` FROM workouts WHERE external_id = $1`, externalID)
	return scanWorkout(row)
}

// FindOpenPlanned returns open planned workouts (status='planned',
// external_id IS NULL) of the given sport whose started_at falls on the same
// LOCAL calendar day as `start`, comparing in the IANA timezone `tz`. The
// reconciliation candidate query (design D2/D6).
func (r *Repo) FindOpenPlanned(ctx context.Context, sport string, start time.Time, tz string) ([]*Workout, error) {
	const q = `SELECT ` + selectCols + ` FROM workouts
        WHERE status = 'planned' AND external_id IS NULL AND sport = $1
          AND (started_at AT TIME ZONE $3)::date = ($2 AT TIME ZONE $3)::date
        ORDER BY started_at ASC`
	rows, err := r.q.Query(ctx, q, sport, start, tz)
	if err != nil {
		return nil, fmt.Errorf("find open planned: %w", err)
	}
	defer rows.Close()
	var out []*Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Merge fulfills the planned workout `plannedID` in place with the actuals from
// activity `a`: it sets external_id/source/status=completed and the actual
// metrics + window, COALESCE-ing name/session_group/notes so a planned label
// survives when the activity carries none, clears needs_link, and retains
// template_id/plan_slot_id (the prescription). Returns the merged row.
func (r *Repo) Merge(ctx context.Context, plannedID uuid.UUID, a *Workout) (*Workout, error) {
	const q = `UPDATE workouts SET
            external_id        = $2,
            source             = $3,
            status             = 'completed',
            name               = COALESCE($4, name),
            started_at         = $5,
            ended_at           = $6,
            kcal_burned        = $7,
            avg_hr             = $8,
            tss                = $9,
            distance_m         = $10,
            avg_power_w        = $11,
            temperature_c      = $12,
            sweat_loss_ml      = $13,
            session_group      = COALESCE($14, session_group),
            notes              = COALESCE($15, notes),
            elevation_gain_m   = $16,
            elevation_loss_m   = $17,
            normalized_power_w = $18,
            intensity_factor   = $19,
            avg_cadence        = $20,
            avg_stride_m       = $21,
            max_hr             = $22,
            aerobic_te         = $23,
            anaerobic_te       = $24,
            secs_in_zone_1     = $25,
            secs_in_zone_2     = $26,
            secs_in_zone_3     = $27,
            secs_in_zone_4     = $28,
            secs_in_zone_5     = $29,
            humidity_pct       = $30,
            wind_speed_mps     = $31,
            needs_link         = false,
            updated_at         = now()
        WHERE id = $1
        RETURNING ` + selectCols
	row := r.q.QueryRow(ctx, q,
		plannedID, a.ExternalID, string(a.Source), a.Name,
		a.StartedAt, a.EndedAt,
		a.KcalBurned, a.AvgHR, a.TSS,
		a.DistanceM, a.AvgPowerW, a.TemperatureC, a.SweatLossML,
		a.SessionGroup, a.Notes,
		a.ElevationGainM, a.ElevationLossM, a.NormalizedPowerW, a.IntensityFactor,
		a.AvgCadence, a.AvgStrideM, a.MaxHR, a.AerobicTE, a.AnaerobicTE,
		a.SecsInZone1, a.SecsInZone2, a.SecsInZone3, a.SecsInZone4, a.SecsInZone5,
		a.HumidityPct, a.WindSpeedMPS,
	)
	return scanWorkout(row)
}

// RestorePlanned reverses a merge (unfulfill): clears external_id, the actual
// metrics and needs_link, sets source back to 'manual' and status to 'planned',
// retaining template_id/plan_slot_id. Returns the restored row. ErrNotFound if
// no row matches.
func (r *Repo) RestorePlanned(ctx context.Context, id uuid.UUID) (*Workout, error) {
	const q = `UPDATE workouts SET
            external_id        = NULL,
            source             = 'manual',
            status             = 'planned',
            kcal_burned        = NULL,
            avg_hr             = NULL,
            tss                = NULL,
            distance_m         = NULL,
            avg_power_w        = NULL,
            temperature_c      = NULL,
            sweat_loss_ml      = NULL,
            elevation_gain_m   = NULL,
            elevation_loss_m   = NULL,
            normalized_power_w = NULL,
            intensity_factor   = NULL,
            avg_cadence        = NULL,
            avg_stride_m       = NULL,
            max_hr             = NULL,
            aerobic_te         = NULL,
            anaerobic_te       = NULL,
            secs_in_zone_1     = NULL,
            secs_in_zone_2     = NULL,
            secs_in_zone_3     = NULL,
            secs_in_zone_4     = NULL,
            secs_in_zone_5     = NULL,
            humidity_pct       = NULL,
            wind_speed_mps     = NULL,
            needs_link         = false,
            updated_at         = now()
        WHERE id = $1
        RETURNING ` + selectCols
	row := r.q.QueryRow(ctx, q, id)
	return scanWorkout(row)
}

// PatchParams carries the optional mutable fields. nil pointers mean "leave
// the existing value unchanged". For the two nullable integer rehearsal
// signals (rpe, gi_distress_score) we expose a tri-state: nil + ClearX=false
// means "leave unchanged", non-nil pointer means "set to that value",
// nil + ClearX=true means "clear to NULL". The handler decodes JSON `null`
// into ClearX=true (numeric-null-clears convention — see design.md decision #6).
type PatchParams struct {
	Name       *string
	Notes      *string
	KcalBurned *float64
	AvgHR      *int
	TSS        *float64

	RPE                  *int
	ClearRPE             bool
	GIDistressScore      *int
	ClearGIDistressScore bool

	// Ingestion metrics share the same tri-state as the rehearsal signals:
	// non-nil pointer sets, nil + ClearX=true clears to NULL, nil +
	// ClearX=false leaves unchanged.
	DistanceM         *float64
	ClearDistanceM    bool
	AvgPowerW         *int
	ClearAvgPowerW    bool
	TemperatureC      *float64
	ClearTemperatureC bool
	SweatLossML       *float64
	ClearSweatLossML  bool
	SessionGroup      *string
	ClearSessionGroup bool

	// Status is NOT NULL, so it has no clear flag — a non-nil pointer sets it.
	Status *string
}

// HasUpdates reports whether at least one mutable field is being changed.
func (p PatchParams) HasUpdates() bool {
	return p.Name != nil || p.Notes != nil || p.KcalBurned != nil || p.AvgHR != nil || p.TSS != nil ||
		p.RPE != nil || p.ClearRPE ||
		p.GIDistressScore != nil || p.ClearGIDistressScore ||
		p.DistanceM != nil || p.ClearDistanceM ||
		p.AvgPowerW != nil || p.ClearAvgPowerW ||
		p.TemperatureC != nil || p.ClearTemperatureC ||
		p.SweatLossML != nil || p.ClearSweatLossML ||
		p.SessionGroup != nil || p.ClearSessionGroup ||
		p.Status != nil
}

// Patch applies a partial update over the mutable subset. Returns ErrNotFound
// if no row matches.
func (r *Repo) Patch(ctx context.Context, id uuid.UUID, p PatchParams) error {
	sets := []string{"updated_at = now()"}
	args := []any{id}
	next := 2
	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", next))
		args = append(args, *p.Name)
		next++
	}
	if p.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", next))
		args = append(args, *p.Notes)
		next++
	}
	if p.KcalBurned != nil {
		sets = append(sets, fmt.Sprintf("kcal_burned = $%d", next))
		args = append(args, *p.KcalBurned)
		next++
	}
	if p.AvgHR != nil {
		sets = append(sets, fmt.Sprintf("avg_hr = $%d", next))
		args = append(args, *p.AvgHR)
		next++
	}
	if p.TSS != nil {
		sets = append(sets, fmt.Sprintf("tss = $%d", next))
		args = append(args, *p.TSS)
		next++
	}
	if p.ClearRPE {
		sets = append(sets, "rpe = NULL")
	} else if p.RPE != nil {
		sets = append(sets, fmt.Sprintf("rpe = $%d", next))
		args = append(args, *p.RPE)
		next++
	}
	if p.ClearGIDistressScore {
		sets = append(sets, "gi_distress_score = NULL")
	} else if p.GIDistressScore != nil {
		sets = append(sets, fmt.Sprintf("gi_distress_score = $%d", next))
		args = append(args, *p.GIDistressScore)
		next++
	}
	if p.ClearDistanceM {
		sets = append(sets, "distance_m = NULL")
	} else if p.DistanceM != nil {
		sets = append(sets, fmt.Sprintf("distance_m = $%d", next))
		args = append(args, *p.DistanceM)
		next++
	}
	if p.ClearAvgPowerW {
		sets = append(sets, "avg_power_w = NULL")
	} else if p.AvgPowerW != nil {
		sets = append(sets, fmt.Sprintf("avg_power_w = $%d", next))
		args = append(args, *p.AvgPowerW)
		next++
	}
	if p.ClearTemperatureC {
		sets = append(sets, "temperature_c = NULL")
	} else if p.TemperatureC != nil {
		sets = append(sets, fmt.Sprintf("temperature_c = $%d", next))
		args = append(args, *p.TemperatureC)
		next++
	}
	if p.ClearSweatLossML {
		sets = append(sets, "sweat_loss_ml = NULL")
	} else if p.SweatLossML != nil {
		sets = append(sets, fmt.Sprintf("sweat_loss_ml = $%d", next))
		args = append(args, *p.SweatLossML)
		next++
	}
	if p.ClearSessionGroup {
		sets = append(sets, "session_group = NULL")
	} else if p.SessionGroup != nil {
		sets = append(sets, fmt.Sprintf("session_group = $%d", next))
		args = append(args, *p.SessionGroup)
		next++
	}
	if p.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", next))
		args = append(args, *p.Status)
		next++
	}
	if len(sets) == 1 {
		// Nothing to update — just confirm the row exists so the caller can
		// distinguish "noop on existing" from "404 on missing".
		if _, err := r.GetByID(ctx, id); err != nil {
			return err
		}
		return nil
	}
	q := "UPDATE workouts SET " + strings.Join(sets, ", ") + " WHERE id = $1"
	tag, err := r.q.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetGarminIDs sets (or clears, when nil) the two Garmin scheduling ids on a
// workout row. Used by the scheduling orchestration after a push/unschedule.
// Returns ErrNotFound if no row matched.
func (r *Repo) SetGarminIDs(ctx context.Context, id uuid.UUID, garminWorkoutID, garminScheduleID *string) error {
	tag, err := r.q.Exec(ctx,
		`UPDATE workouts SET garmin_workout_id = $2, garmin_schedule_id = $3, updated_at = now() WHERE id = $1`,
		id, garminWorkoutID, garminScheduleID,
	)
	if err != nil {
		return fmt.Errorf("set garmin ids: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a workout. Returns ErrNotFound if no row matched.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.q.Exec(ctx, `DELETE FROM workouts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// List returns workouts whose started_at falls within [from, to] inclusive,
// ordered by started_at ascending. When sessionGroup is non-nil it further
// narrows the result to rows whose session_group equals that key exactly —
// used to fetch the legs of one brick/multisport session together. When status
// is non-nil it narrows to rows with that lifecycle status (planned|completed).
func (r *Repo) List(ctx context.Context, from, to time.Time, sessionGroup, status *string) ([]*Workout, error) {
	q := `SELECT ` + selectCols + ` FROM workouts WHERE started_at >= $1 AND started_at <= $2`
	args := []any{from, to}
	next := 3
	if sessionGroup != nil {
		q += fmt.Sprintf(` AND session_group = $%d`, next)
		args = append(args, *sessionGroup)
		next++
	}
	if status != nil {
		q += fmt.Sprintf(` AND status = $%d`, next)
		args = append(args, *status)
		next++
	}
	q += ` ORDER BY started_at ASC`
	rows, err := r.q.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	defer rows.Close()
	var out []*Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ReplaceChildren replaces a workout's nested splits/sets in place against the
// supplied Querier (pool or pgx.Tx). It is nil-aware so a re-sync that carries
// no detail for one child kind leaves that kind untouched: a nil slice means
// "this payload had no opinion" (skip), while a non-nil slice (including empty)
// means "this is the authoritative set" (delete existing, insert the new).
// This is what keeps the replace-on-resync semantics from clobbering good data
// when a Garmin detail fetch is missing on a later sync.
func (r *Repo) ReplaceChildren(ctx context.Context, q store.Querier, workoutID uuid.UUID, splits []Split, sets []Set) error {
	if splits != nil {
		if _, err := q.Exec(ctx, `DELETE FROM workout_splits WHERE workout_id = $1`, workoutID); err != nil {
			return fmt.Errorf("clear splits: %w", err)
		}
		for _, s := range splits {
			if _, err := q.Exec(ctx, `
                INSERT INTO workout_splits (
                    id, workout_id, split_index,
                    distance_m, duration_s, avg_hr, avg_power_w, avg_speed_mps, elevation_gain_m
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				uuid.New(), workoutID, s.SplitIndex,
				s.DistanceM, s.DurationS, s.AvgHR, s.AvgPowerW, s.AvgSpeedMPS, s.ElevationGainM,
			); err != nil {
				return fmt.Errorf("insert split: %w", err)
			}
		}
	}
	if sets != nil {
		if _, err := q.Exec(ctx, `DELETE FROM workout_sets WHERE workout_id = $1`, workoutID); err != nil {
			return fmt.Errorf("clear sets: %w", err)
		}
		for _, s := range sets {
			if _, err := q.Exec(ctx, `
                INSERT INTO workout_sets (
                    id, workout_id, set_index,
                    exercise_name, exercise_category, reps, weight_kg, duration_s
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.New(), workoutID, s.SetIndex,
				s.ExerciseName, s.ExerciseCategory, s.Reps, s.WeightKg, s.DurationS,
			); err != nil {
				return fmt.Errorf("insert set: %w", err)
			}
		}
	}
	return nil
}

// DeleteChildren removes all split/set rows for a workout (used when unfulfill
// reverts a reconciled row to planned — the imported detail no longer applies).
func (r *Repo) DeleteChildren(ctx context.Context, q store.Querier, workoutID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM workout_splits WHERE workout_id = $1`, workoutID); err != nil {
		return fmt.Errorf("delete splits: %w", err)
	}
	if _, err := q.Exec(ctx, `DELETE FROM workout_sets WHERE workout_id = $1`, workoutID); err != nil {
		return fmt.Errorf("delete sets: %w", err)
	}
	return nil
}

// loadChildren populates w.Splits and w.Sets from the child tables, each ordered
// by its index. Used by the single-get path only; the list query never calls it.
func (r *Repo) loadChildren(ctx context.Context, q store.Querier, w *Workout) error {
	srows, err := q.Query(ctx, `
        SELECT split_index, distance_m, duration_s, avg_hr, avg_power_w, avg_speed_mps, elevation_gain_m
        FROM workout_splits WHERE workout_id = $1 ORDER BY split_index ASC`, w.ID)
	if err != nil {
		return fmt.Errorf("load splits: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var s Split
		if err := srows.Scan(&s.SplitIndex, &s.DistanceM, &s.DurationS, &s.AvgHR, &s.AvgPowerW, &s.AvgSpeedMPS, &s.ElevationGainM); err != nil {
			return fmt.Errorf("scan split: %w", err)
		}
		w.Splits = append(w.Splits, s)
	}
	if err := srows.Err(); err != nil {
		return err
	}
	trows, err := q.Query(ctx, `
        SELECT set_index, exercise_name, exercise_category, reps, weight_kg, duration_s
        FROM workout_sets WHERE workout_id = $1 ORDER BY set_index ASC`, w.ID)
	if err != nil {
		return fmt.Errorf("load sets: %w", err)
	}
	defer trows.Close()
	for trows.Next() {
		var s Set
		if err := trows.Scan(&s.SetIndex, &s.ExerciseName, &s.ExerciseCategory, &s.Reps, &s.WeightKg, &s.DurationS); err != nil {
			return fmt.Errorf("scan set: %w", err)
		}
		w.Sets = append(w.Sets, s)
	}
	return trows.Err()
}

// GetByIDWithChildren returns a single workout with its nested splits/sets
// detail loaded (the single-get response shape). ErrNotFound if no row matches.
func (r *Repo) GetByIDWithChildren(ctx context.Context, id uuid.UUID) (*Workout, error) {
	w, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.loadChildren(ctx, r.q, w); err != nil {
		return nil, err
	}
	return w, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkout(s scanner) (*Workout, error) {
	var (
		w         Workout
		sourceStr string
		sportStr  string
		statusStr string
	)
	err := s.Scan(
		&w.ID, &w.ExternalID, &sourceStr, &sportStr, &statusStr, &w.Name,
		&w.StartedAt, &w.EndedAt,
		&w.KcalBurned, &w.AvgHR, &w.TSS,
		&w.RPE, &w.GIDistressScore,
		&w.DistanceM, &w.AvgPowerW, &w.TemperatureC, &w.SweatLossML, &w.SessionGroup,
		&w.TemplateID, &w.PlanSlotID,
		&w.GarminWorkoutID, &w.GarminScheduleID,
		&w.NeedsLink,
		&w.Notes,
		&w.CreatedAt, &w.UpdatedAt,
		&w.ElevationGainM, &w.ElevationLossM, &w.NormalizedPowerW, &w.IntensityFactor,
		&w.AvgCadence, &w.AvgStrideM, &w.MaxHR, &w.AerobicTE, &w.AnaerobicTE,
		&w.SecsInZone1, &w.SecsInZone2, &w.SecsInZone3, &w.SecsInZone4, &w.SecsInZone5,
		&w.HumidityPct, &w.WindSpeedMPS,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan workout: %w", err)
	}
	w.Source = Source(sourceStr)
	w.Sport = Sport(sportStr)
	w.Status = Status(statusStr)
	return &w, nil
}
