package workouts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/nutrition-api/internal/store"
)

// Validation errors map 1:1 to the API error codes documented in the
// workouts capability spec.
var (
	ErrSourceInvalid          = errors.New("source_invalid")
	ErrSportInvalid           = errors.New("sport_invalid")
	ErrWindowInvalid          = errors.New("window_invalid")
	ErrStartedAtFarFuture     = errors.New("started_at_too_far_future")
	ErrKcalBurnedInvalid      = errors.New("kcal_burned_invalid")
	ErrAvgHRInvalid           = errors.New("avg_hr_invalid")
	ErrTSSInvalid             = errors.New("tss_invalid")
	ErrRPEInvalid             = errors.New("rpe_invalid")
	ErrGIDistressScoreInvalid = errors.New("gi_distress_score_invalid")
	ErrDistanceMInvalid       = errors.New("distance_m_invalid")
	ErrAvgPowerWInvalid       = errors.New("avg_power_w_invalid")
	ErrTemperatureCInvalid    = errors.New("temperature_c_invalid")
	ErrSweatLossMLInvalid     = errors.New("sweat_loss_ml_invalid")
	ErrSessionGroupInvalid    = errors.New("session_group_invalid")
	ErrStatusInvalid          = errors.New("status_invalid")
)

// Reconciliation (fulfill/unfulfill) sentinel errors — map 1:1 to API codes.
var (
	ErrFulfillTargetNotPlanned   = errors.New("planned_workout_required")
	ErrFulfillSourceNotCompleted = errors.New("completed_workout_required")
	ErrFulfillSportMismatch      = errors.New("sport_mismatch")
	ErrNotFulfilled              = errors.New("workout_not_fulfilled")
)

// plannedMaxFuture bounds how far ahead a planned workout's started_at may be.
// Completed workouts keep the tighter 24h guard.
const plannedMaxFuture = 365 * 24 * time.Hour

// RPE + GI distress score bounds. Exposed so handler error responses can echo
// `range: {min, max}` back to clients without re-encoding the rule.
const (
	RPEMin             = 1
	RPEMax             = 10
	GIDistressScoreMin = 1
	GIDistressScoreMax = 5
)

// Temperature bounds (°C) and the session-group length cap. Exposed so the
// handler can echo `range: {min, max}` for temperature the way rpe/gi do.
const (
	TemperatureCMin    = -40
	TemperatureCMax    = 60
	SessionGroupMaxLen = 255
)

// Service orchestrates workout CRUD over the repo. pool backs the
// reconciliation transaction (candidate match + merge/insert as one unit); loc
// is the local timezone used for same-calendar-day matching (design D6).
type Service struct {
	repo *Repo
	pool *pgxpool.Pool
	loc  *time.Location
}

// NewService wires the repo, the pool (for the reconciliation transaction), and
// the local timezone name (e.g. "Europe/Vienna"). An empty or invalid tz falls
// back to UTC.
func NewService(repo *Repo, pool *pgxpool.Pool, localTZ string) *Service {
	loc, err := time.LoadLocation(localTZ)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	return &Service{repo: repo, pool: pool, loc: loc}
}

// CreateInput is the payload for POST /workouts.
type CreateInput struct {
	ExternalID      *string
	Source          string
	Sport           string
	Status          string
	Name            *string
	StartedAt       time.Time
	EndedAt         time.Time
	KcalBurned      *float64
	AvgHR           *int
	TSS             *float64
	RPE             *int
	GIDistressScore *int
	DistanceM       *float64
	AvgPowerW       *int
	TemperatureC    *float64
	SweatLossML     *float64
	SessionGroup    *string
	Notes           *string
}

// Upsert validates input and applies the UPSERT-by-external_id semantics, with
// reconciliation for first-sight Garmin imports (merge into a matching open
// planned workout — see reconcileUpsert). The returned bool is true when a new
// row was inserted; false when an existing row was updated or merged.
func (s *Service) Upsert(ctx context.Context, in CreateInput) (*Workout, bool, error) {
	w, err := s.buildWorkout(in)
	if err != nil {
		return nil, false, err
	}
	return s.reconcileUpsert(ctx, w)
}

// reconcileUpsert persists a built workout. For a first-sight Garmin import
// (source='garmin', external_id set and not already stored) it tries to fulfill
// a single matching open planned workout in one transaction (design D2):
// exactly one candidate → merge; zero → standalone insert; ≥2 → standalone
// insert flagged needs_link. Everything else (manual rows, or a re-sync whose
// external_id is already stored) takes the plain external_id UPSERT path.
func (s *Service) reconcileUpsert(ctx context.Context, w *Workout) (*Workout, bool, error) {
	if w.Source != SourceGarmin || w.ExternalID == nil {
		created, err := s.repo.Upsert(ctx, w)
		if err != nil {
			return nil, false, err
		}
		return w, created, nil
	}

	var (
		result  *Workout
		created bool
	)
	txErr := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		tr := NewRepo(tx)
		// Re-sync: the activity is already stored → plain UPSERT updates it in
		// place; reconciliation does not re-run.
		if _, err := tr.GetByExternalID(ctx, *w.ExternalID); err == nil {
			c, err := tr.Upsert(ctx, w)
			if err != nil {
				return err
			}
			result, created = w, c
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		// First sight → candidate match on sport + local calendar day.
		cands, err := tr.FindOpenPlanned(ctx, string(w.Sport), w.StartedAt, s.loc.String())
		if err != nil {
			return err
		}
		switch len(cands) {
		case 1:
			merged, err := tr.Merge(ctx, cands[0].ID, w)
			if err != nil {
				return err
			}
			result, created = merged, false
			return nil
		case 0:
			c, err := tr.Upsert(ctx, w)
			if err != nil {
				return err
			}
			result, created = w, c
			return nil
		default:
			w.NeedsLink = true
			c, err := tr.Upsert(ctx, w)
			if err != nil {
				return err
			}
			result, created = w, c
			return nil
		}
	})
	if txErr != nil {
		return nil, false, txErr
	}
	return result, created, nil
}

// Fulfill merges an existing completed activity (completedID) into an existing
// planned workout (plannedID): copies the activity's external_id/source/actuals
// onto the planned row, flips it to completed, removes the redundant standalone
// row, and clears needs_link. The planned row survives so plan_slot_id stays
// the stable identity. Returns the merged row.
func (s *Service) Fulfill(ctx context.Context, plannedID, completedID uuid.UUID) (*Workout, error) {
	var result *Workout
	txErr := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		tr := NewRepo(tx)
		planned, err := tr.GetByID(ctx, plannedID)
		if err != nil {
			return err
		}
		completed, err := tr.GetByID(ctx, completedID)
		if err != nil {
			return err
		}
		if planned.Status != StatusPlanned {
			return ErrFulfillTargetNotPlanned
		}
		if completed.Status != StatusCompleted {
			return ErrFulfillSourceNotCompleted
		}
		if planned.Sport != completed.Sport {
			return ErrFulfillSportMismatch
		}
		// Remove the standalone row first so its external_id frees up before the
		// merge writes the same external_id onto the planned row (the partial
		// unique index would otherwise conflict).
		if err := tr.Delete(ctx, completedID); err != nil {
			return err
		}
		merged, err := tr.Merge(ctx, plannedID, completed)
		if err != nil {
			return err
		}
		result = merged
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// Unfulfill reverses a merge: restores status='planned' and clears the
// external_id and actual metrics, keeping template_id/plan_slot_id. Errors if
// the workout is not a fulfilled planned row (completed with a plan_slot_id).
func (s *Service) Unfulfill(ctx context.Context, id uuid.UUID) (*Workout, error) {
	var result *Workout
	txErr := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		tr := NewRepo(tx)
		w, err := tr.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if w.Status != StatusCompleted || w.PlanSlotID == nil {
			return ErrNotFulfilled
		}
		restored, err := tr.RestorePlanned(ctx, id)
		if err != nil {
			return err
		}
		result = restored
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// Get returns a single workout by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Workout, error) {
	return s.repo.GetByID(ctx, id)
}

// PatchInput is the editable subset of PATCH /workouts/{id}. The two
// rehearsal-signal fields carry tri-state via the ClearX flag: a non-nil
// pointer sets, nil + ClearX=true clears to NULL, nil + ClearX=false leaves
// the field unchanged. The handler decodes JSON `null` into ClearX=true.
type PatchInput struct {
	Name                 *string
	Notes                *string
	KcalBurned           *float64
	AvgHR                *int
	TSS                  *float64
	RPE                  *int
	ClearRPE             bool
	GIDistressScore      *int
	ClearGIDistressScore bool

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

	// Status is never cleared (NOT NULL); a non-nil pointer sets it.
	Status *string
}

// Patch validates and applies the partial update. Range validation runs
// before any DB write — if any field is out of range, no field is written
// (transactional validation per the spec scenario).
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
	if err := validateRPE(in.RPE); err != nil {
		return nil, err
	}
	if err := validateGIDistressScore(in.GIDistressScore); err != nil {
		return nil, err
	}
	if err := validateIngestionMetrics(in.DistanceM, in.AvgPowerW, in.TemperatureC, in.SweatLossML, in.SessionGroup); err != nil {
		return nil, err
	}
	if in.Status != nil && !ValidStatus(*in.Status) {
		return nil, ErrStatusInvalid
	}
	params := PatchParams{
		Name:                 in.Name,
		Notes:                in.Notes,
		KcalBurned:           in.KcalBurned,
		AvgHR:                in.AvgHR,
		TSS:                  in.TSS,
		RPE:                  in.RPE,
		ClearRPE:             in.ClearRPE,
		GIDistressScore:      in.GIDistressScore,
		ClearGIDistressScore: in.ClearGIDistressScore,
		DistanceM:            in.DistanceM,
		ClearDistanceM:       in.ClearDistanceM,
		AvgPowerW:            in.AvgPowerW,
		ClearAvgPowerW:       in.ClearAvgPowerW,
		TemperatureC:         in.TemperatureC,
		ClearTemperatureC:    in.ClearTemperatureC,
		SweatLossML:          in.SweatLossML,
		ClearSweatLossML:     in.ClearSweatLossML,
		SessionGroup:         in.SessionGroup,
		ClearSessionGroup:    in.ClearSessionGroup,
		Status:               in.Status,
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

// ListWindow returns workouts whose started_at falls within [from, to]. When
// sessionGroup is non-nil the result is narrowed to workouts whose session_group
// equals that key (the legs of one brick/multisport session). When status is
// non-nil it narrows to that lifecycle status (planned|completed).
func (s *Service) ListWindow(ctx context.Context, from, to time.Time, sessionGroup, status *string) ([]*Workout, error) {
	return s.repo.List(ctx, from, to, sessionGroup, status)
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
		res, created, err := s.reconcileUpsert(ctx, w)
		if err != nil {
			results[i] = BulkItemResult{Index: i, Err: err}
			continue
		}
		results[i] = BulkItemResult{Index: i, ID: res.ID, Created: created}
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
	// Status defaults to completed when omitted; validate when supplied.
	status := in.Status
	if status == "" {
		status = string(StatusCompleted)
	}
	if !ValidStatus(status) {
		return nil, ErrStatusInvalid
	}
	// Future-date guard is conditioned on status: a completed activity can't be
	// more than 24h ahead, but a planned session may be scheduled up to a year
	// out. Planned sessions in the past are allowed (a plan already underway).
	if Status(status) == StatusCompleted {
		if in.StartedAt.After(time.Now().Add(24 * time.Hour)) {
			return nil, ErrStartedAtFarFuture
		}
	} else {
		if in.StartedAt.After(time.Now().Add(plannedMaxFuture)) {
			return nil, ErrStartedAtFarFuture
		}
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
	if err := validateRPE(in.RPE); err != nil {
		return nil, err
	}
	if err := validateGIDistressScore(in.GIDistressScore); err != nil {
		return nil, err
	}
	if err := validateIngestionMetrics(in.DistanceM, in.AvgPowerW, in.TemperatureC, in.SweatLossML, in.SessionGroup); err != nil {
		return nil, err
	}
	return &Workout{
		ExternalID:      in.ExternalID,
		Source:          Source(in.Source),
		Sport:           Sport(in.Sport),
		Status:          Status(status),
		Name:            in.Name,
		StartedAt:       in.StartedAt,
		EndedAt:         in.EndedAt,
		KcalBurned:      in.KcalBurned,
		AvgHR:           in.AvgHR,
		TSS:             in.TSS,
		RPE:             in.RPE,
		GIDistressScore: in.GIDistressScore,
		DistanceM:       in.DistanceM,
		AvgPowerW:       in.AvgPowerW,
		TemperatureC:    in.TemperatureC,
		SweatLossML:     in.SweatLossML,
		SessionGroup:    in.SessionGroup,
		Notes:           in.Notes,
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

// validateRPE accepts nil (not set) or an integer in [RPEMin, RPEMax].
// Returns ErrRPEInvalid for any out-of-range integer.
func validateRPE(v *int) error {
	if v == nil {
		return nil
	}
	if *v < RPEMin || *v > RPEMax {
		return ErrRPEInvalid
	}
	return nil
}

// validateGIDistressScore accepts nil (not set) or an integer in
// [GIDistressScoreMin, GIDistressScoreMax]. Returns ErrGIDistressScoreInvalid
// for any out-of-range integer.
func validateGIDistressScore(v *int) error {
	if v == nil {
		return nil
	}
	if *v < GIDistressScoreMin || *v > GIDistressScoreMax {
		return ErrGIDistressScoreInvalid
	}
	return nil
}

// validateIngestionMetrics validates the five ingestion fields. Each is nil
// when not supplied (set or clear-to-NULL paths both skip validation here —
// clearing carries no value to check). Distance/power/sweat must be > 0,
// temperature in [TemperatureCMin, TemperatureCMax], session_group non-empty
// (after trimming) and ≤ SessionGroupMaxLen.
func validateIngestionMetrics(distanceM *float64, avgPowerW *int, temperatureC, sweatLossML *float64, sessionGroup *string) error {
	if distanceM != nil && *distanceM <= 0 {
		return ErrDistanceMInvalid
	}
	if avgPowerW != nil && *avgPowerW <= 0 {
		return ErrAvgPowerWInvalid
	}
	if temperatureC != nil && (*temperatureC < TemperatureCMin || *temperatureC > TemperatureCMax) {
		return ErrTemperatureCInvalid
	}
	if sweatLossML != nil && *sweatLossML <= 0 {
		return ErrSweatLossMLInvalid
	}
	if sessionGroup != nil {
		if strings.TrimSpace(*sessionGroup) == "" || len(*sessionGroup) > SessionGroupMaxLen {
			return ErrSessionGroupInvalid
		}
	}
	return nil
}
