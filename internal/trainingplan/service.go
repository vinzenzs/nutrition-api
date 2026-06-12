package trainingplan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vinzenzs/nutrition-api/internal/store"
	"github.com/vinzenzs/nutrition-api/internal/workouts"
	"github.com/vinzenzs/nutrition-api/internal/workouttemplates"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrNameRequired          = errors.New("name_required")
	ErrStartDateInvalid      = errors.New("start_date_invalid")
	ErrOrdinalInvalid        = errors.New("ordinal_invalid")
	ErrWeekdayInvalid        = errors.New("weekday_invalid")
	ErrTimeOfDayInvalid      = errors.New("time_of_day_invalid")
	ErrTemplateRequired      = errors.New("template_id_required")
	ErrScopeInvalid          = errors.New("scope_invalid")
	ErrOverrideIntentInvalid = errors.New("override_intent_invalid")
	ErrOverrideDuplicate     = errors.New("override_intent_duplicate")
	ErrOverrideTargetInvalid = errors.New("override_target_invalid")
)

const dateLayout = "2006-01-02"

// defaultStartHour is the local start time materialize uses when a slot has no
// explicit time_of_day; later same-day slots stack one hour apart by ordinal.
const defaultStartHour = 6

// fallbackDurationSec is the planned-workout length when a template carries no
// estimated_duration_sec.
const fallbackDurationSec = 3600

// Service orchestrates plan/week/slot CRUD plus materialize.
type Service struct {
	repo          *Repo
	pool          *pgxpool.Pool
	workoutsRepo  *workouts.Repo
	templatesRepo *workouttemplates.Repo
	loc           *time.Location
}

// NewService wires the plan repo, the pool (for the materialize transaction),
// the workouts repo (slot upsert target), the workout-templates repo (effective
// program resolution), and the default timezone the planned start times resolve
// in.
func NewService(repo *Repo, pool *pgxpool.Pool, workoutsRepo *workouts.Repo, templatesRepo *workouttemplates.Repo, tz string) *Service {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return &Service{repo: repo, pool: pool, workoutsRepo: workoutsRepo, templatesRepo: templatesRepo, loc: loc}
}

// ----- plan -----

type PlanInput struct {
	Name      string
	RaceID    *uuid.UUID
	StartDate string
	Notes     *string
}

func (s *Service) CreatePlan(ctx context.Context, in PlanInput) (*Plan, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrNameRequired
	}
	if !validDate(in.StartDate) {
		return nil, ErrStartDateInvalid
	}
	return s.repo.CreatePlan(ctx, &Plan{Name: strings.TrimSpace(in.Name), RaceID: in.RaceID, StartDate: in.StartDate, Notes: in.Notes})
}

func (s *Service) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	return s.repo.Load(ctx, id)
}
func (s *Service) ListPlans(ctx context.Context) ([]*Plan, error) { return s.repo.ListPlans(ctx) }
func (s *Service) DeletePlan(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePlan(ctx, id)
}

// PlanPatch carries partial plan updates with tri-state for the nullables.
type PlanPatch struct {
	Name      *string
	StartDate *string
	SetRace   bool
	RaceID    *uuid.UUID
	SetNotes  bool
	Notes     *string
}

func (s *Service) PatchPlan(ctx context.Context, id uuid.UUID, p PlanPatch) (*Plan, error) {
	plan, err := s.repo.GetPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" {
			return nil, ErrNameRequired
		}
		plan.Name = strings.TrimSpace(*p.Name)
	}
	if p.StartDate != nil {
		if !validDate(*p.StartDate) {
			return nil, ErrStartDateInvalid
		}
		plan.StartDate = *p.StartDate
	}
	if p.SetRace {
		plan.RaceID = p.RaceID
	}
	if p.SetNotes {
		plan.Notes = p.Notes
	}
	return s.repo.UpdatePlan(ctx, plan)
}

// ----- week -----

type WeekInput struct {
	Ordinal int
	PhaseID *uuid.UUID
	Notes   *string
}

func (s *Service) CreateWeek(ctx context.Context, planID uuid.UUID, in WeekInput) (*PlanWeek, error) {
	if in.Ordinal < 1 {
		return nil, ErrOrdinalInvalid
	}
	if _, err := s.repo.GetPlan(ctx, planID); err != nil {
		return nil, err
	}
	return s.repo.CreateWeek(ctx, &PlanWeek{PlanID: planID, Ordinal: in.Ordinal, PhaseID: in.PhaseID, Notes: in.Notes})
}

type WeekPatch struct {
	Ordinal  *int
	SetPhase bool
	PhaseID  *uuid.UUID
	SetNotes bool
	Notes    *string
}

func (s *Service) PatchWeek(ctx context.Context, weekID uuid.UUID, p WeekPatch) (*PlanWeek, error) {
	w, err := s.repo.GetWeek(ctx, weekID)
	if err != nil {
		return nil, err
	}
	if p.Ordinal != nil {
		if *p.Ordinal < 1 {
			return nil, ErrOrdinalInvalid
		}
		w.Ordinal = *p.Ordinal
	}
	if p.SetPhase {
		w.PhaseID = p.PhaseID
	}
	if p.SetNotes {
		w.Notes = p.Notes
	}
	return s.repo.UpdateWeek(ctx, w)
}

func (s *Service) DeleteWeek(ctx context.Context, weekID uuid.UUID) error {
	return s.repo.DeleteWeek(ctx, weekID)
}

// ----- slot -----

type SlotInput struct {
	Weekday         int
	Ordinal         int
	TemplateID      uuid.UUID
	TimeOfDay       *string
	TargetOverrides []SlotTargetOverride
}

func (s *Service) CreateSlot(ctx context.Context, weekID uuid.UUID, in SlotInput) (*PlanSlot, error) {
	if in.Weekday < 0 || in.Weekday > 6 {
		return nil, ErrWeekdayInvalid
	}
	if in.TemplateID == uuid.Nil {
		return nil, ErrTemplateRequired
	}
	if in.TimeOfDay != nil && !validTime(*in.TimeOfDay) {
		return nil, ErrTimeOfDayInvalid
	}
	if err := validateOverrides(in.TargetOverrides); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetWeek(ctx, weekID); err != nil {
		return nil, err
	}
	return s.repo.CreateSlot(ctx, &PlanSlot{
		PlanWeekID: weekID, Weekday: in.Weekday, Ordinal: in.Ordinal,
		TemplateID: in.TemplateID, TimeOfDay: in.TimeOfDay, TargetOverrides: in.TargetOverrides,
	})
}

type SlotPatch struct {
	Weekday      *int
	Ordinal      *int
	TemplateID   *uuid.UUID
	SetTime      bool
	TimeOfDay    *string
	SetOverrides bool
	// TargetOverrides is the replacement list when SetOverrides is true; an
	// empty/nil slice clears all overrides.
	TargetOverrides []SlotTargetOverride
}

func (s *Service) PatchSlot(ctx context.Context, slotID uuid.UUID, p SlotPatch) (*PlanSlot, error) {
	sl, err := s.repo.GetSlot(ctx, slotID)
	if err != nil {
		return nil, err
	}
	if p.Weekday != nil {
		if *p.Weekday < 0 || *p.Weekday > 6 {
			return nil, ErrWeekdayInvalid
		}
		sl.Weekday = *p.Weekday
	}
	if p.Ordinal != nil {
		sl.Ordinal = *p.Ordinal
	}
	if p.TemplateID != nil {
		if *p.TemplateID == uuid.Nil {
			return nil, ErrTemplateRequired
		}
		sl.TemplateID = *p.TemplateID
	}
	if p.SetTime {
		if p.TimeOfDay != nil && !validTime(*p.TimeOfDay) {
			return nil, ErrTimeOfDayInvalid
		}
		sl.TimeOfDay = p.TimeOfDay
	}
	if p.SetOverrides {
		if err := validateOverrides(p.TargetOverrides); err != nil {
			return nil, err
		}
		sl.TargetOverrides = p.TargetOverrides
	}
	return s.repo.UpdateSlot(ctx, sl)
}

// validateOverrides enforces: each intent is a known template intent, no intent
// repeats, and each target passes the workout-templates Target validator.
func validateOverrides(overrides []SlotTargetOverride) error {
	seen := make(map[string]bool, len(overrides))
	for _, o := range overrides {
		if !workouttemplates.ValidIntent(o.Intent) {
			return ErrOverrideIntentInvalid
		}
		if seen[o.Intent] {
			return ErrOverrideDuplicate
		}
		seen[o.Intent] = true
		t := o.Target
		if err := workouttemplates.ValidateTarget(&t); err != nil {
			return ErrOverrideTargetInvalid
		}
	}
	return nil
}

func (s *Service) DeleteSlot(ctx context.Context, slotID uuid.UUID) error {
	return s.repo.DeleteSlot(ctx, slotID)
}

// ----- materialize -----

// Scope selects which slots a materialize call expands.
type Scope struct {
	Kind string // "week" | "range" | "all"
	Week *int
	From *string
	To   *string
}

// Materialize expands the in-scope slots into planned workouts (idempotent,
// slot-keyed) inside one transaction, and returns the affected workouts.
func (s *Service) Materialize(ctx context.Context, planID uuid.UUID, scope Scope) ([]*workouts.Workout, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	plan, err := s.repo.GetPlan(ctx, planID)
	if err != nil {
		return nil, err
	}
	start, err := time.ParseInLocation(dateLayout, plan.StartDate, s.loc)
	if err != nil {
		return nil, ErrStartDateInvalid
	}

	var out []*workouts.Workout
	txErr := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		slots, err := s.repo.loadMaterializeSlots(ctx, tx, planID)
		if err != nil {
			return err
		}
		// Count slots per (week, weekday) so multi-slot days get a session_group.
		dayCount := map[string]int{}
		for _, sl := range slots {
			dayCount[dayKey(sl.WeekOrdinal, sl.Weekday)]++
		}
		for _, sl := range slots {
			date := start.AddDate(0, 0, (sl.WeekOrdinal-1)*7+sl.Weekday)
			if !inScope(scope, sl.WeekOrdinal, date) {
				continue
			}
			started := s.startTime(date, sl)
			durSec := fallbackDurationSec
			if sl.TemplateDurationSec != nil && *sl.TemplateDurationSec > 0 {
				durSec = *sl.TemplateDurationSec
			}
			ended := started.Add(time.Duration(durSec) * time.Second)

			var sessionGroup *string
			if dayCount[dayKey(sl.WeekOrdinal, sl.Weekday)] > 1 {
				g := fmt.Sprintf("plan:%s:w%d:d%d", planID, sl.WeekOrdinal, sl.Weekday)
				sessionGroup = &g
			}
			name := sl.TemplateName
			w, err := s.workoutsRepo.UpsertPlannedFromSlot(ctx, tx, workouts.PlannedSlotInput{
				PlanSlotID:   sl.SlotID,
				TemplateID:   sl.TemplateID,
				Sport:        sl.TemplateSport,
				Name:         &name,
				StartedAt:    started.UTC(),
				EndedAt:      ended.UTC(),
				SessionGroup: sessionGroup,
			})
			if err != nil {
				return err
			}
			out = append(out, w)
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

// startTime resolves a slot's local start instant: its explicit time_of_day, or
// the default hour stacked one hour per slot ordinal.
func (s *Service) startTime(date time.Time, sl materializeSlot) time.Time {
	y, m, d := date.Date()
	if sl.TimeOfDay != nil {
		if t, err := time.Parse("15:04:05", *sl.TimeOfDay); err == nil {
			return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, s.loc)
		}
	}
	return time.Date(y, m, d, defaultStartHour+sl.SlotOrdinal, 0, 0, 0, s.loc)
}

func dayKey(weekOrdinal, weekday int) string {
	return fmt.Sprintf("%d:%d", weekOrdinal, weekday)
}

func inScope(scope Scope, weekOrdinal int, date time.Time) bool {
	switch scope.Kind {
	case "all":
		return true
	case "week":
		return scope.Week != nil && weekOrdinal == *scope.Week
	case "range":
		ds := date.Format(dateLayout)
		return ds >= *scope.From && ds <= *scope.To
	}
	return false
}

// PlannedWorkoutsInScope validates the scope and returns the planned workout ids
// the Garmin scheduling push should act on (delegates to the repo join).
func (s *Service) PlannedWorkoutsInScope(ctx context.Context, planID uuid.UUID, scope Scope) ([]uuid.UUID, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetPlan(ctx, planID); err != nil {
		return nil, err
	}
	return s.repo.PlannedWorkoutsInScope(ctx, planID, scope.Kind, scope.Week, scope.From, scope.To)
}

// ----- effective program -----

// EffectiveProgram resolves a workout's program: the steps of its template with
// the workout's slot target overrides applied (matched by step intent). A
// workout with no template returns its metadata and an empty step list. Returns
// workouts.ErrNotFound when the workout is missing.
func (s *Service) EffectiveProgram(ctx context.Context, workoutID uuid.UUID) (*Program, error) {
	w, err := s.workoutsRepo.GetByID(ctx, workoutID)
	if err != nil {
		return nil, err
	}
	prog := &Program{WorkoutID: w.ID, Sport: string(w.Sport), Name: w.Name, Steps: []workouttemplates.Step{}}
	if w.TemplateID == nil {
		return prog, nil
	}
	tmpl, err := s.templatesRepo.GetByID(ctx, w.TemplateID.String())
	if err != nil {
		return nil, err
	}
	var overrides []SlotTargetOverride
	if w.PlanSlotID != nil {
		if sl, err := s.repo.GetSlot(ctx, *w.PlanSlotID); err == nil {
			overrides = sl.TargetOverrides
		}
	}
	prog.Steps = applyOverrides(tmpl.Steps, overrides)
	return prog, nil
}

// applyOverrides returns a copy of steps with each step's target replaced when
// its intent has a matching override; repeat groups are recursed one level.
func applyOverrides(steps []workouttemplates.Step, overrides []SlotTargetOverride) []workouttemplates.Step {
	if len(steps) == 0 {
		return []workouttemplates.Step{}
	}
	byIntent := make(map[string]workouttemplates.Target, len(overrides))
	for _, o := range overrides {
		byIntent[o.Intent] = o.Target
	}
	out := make([]workouttemplates.Step, len(steps))
	for i, st := range steps {
		out[i] = applyOverrideToStep(st, byIntent)
	}
	return out
}

func applyOverrideToStep(st workouttemplates.Step, byIntent map[string]workouttemplates.Target) workouttemplates.Step {
	if st.Type == workouttemplates.NodeRepeat {
		inner := make([]workouttemplates.Step, len(st.Steps))
		for i, child := range st.Steps {
			inner[i] = applyOverrideToStep(child, byIntent)
		}
		st.Steps = inner
		return st
	}
	if t, ok := byIntent[st.Intent]; ok {
		tt := t
		st.Target = &tt
	}
	return st
}

// ----- validators -----

func validateScope(s Scope) error {
	switch s.Kind {
	case "all":
		return nil
	case "week":
		if s.Week == nil || *s.Week < 1 {
			return ErrScopeInvalid
		}
		return nil
	case "range":
		if s.From == nil || s.To == nil || !validDate(*s.From) || !validDate(*s.To) || *s.To < *s.From {
			return ErrScopeInvalid
		}
		return nil
	}
	return ErrScopeInvalid
}

func validDate(d string) bool {
	_, err := time.Parse(dateLayout, d)
	return err == nil
}

func validTime(t string) bool {
	if _, err := time.Parse("15:04:05", t); err == nil {
		return true
	}
	_, err := time.Parse("15:04", t)
	return err == nil
}
