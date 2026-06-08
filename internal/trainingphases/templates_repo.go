package trainingphases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/store"
)

// Sentinel errors.
var (
	ErrTemplateNotFound = errors.New("template not found")
	// ErrTemplateInUse is returned by Delete when one or more phases reference
	// the template via default_template_id (FK-RESTRICT). The handler converts
	// this to 409 with the referencing phases listed.
	ErrTemplateInUse = errors.New("template in use by one or more phases")
)

// ReferencingPhase is the {id, name} pair the 409 template_in_use response
// carries for each phase blocking a template delete.
type ReferencingPhase struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// TemplatesRepo persists goal_templates rows.
type TemplatesRepo struct {
	q store.Querier
}

func NewTemplatesRepo(q store.Querier) *TemplatesRepo {
	return &TemplatesRepo{q: q}
}

// goal_templates column projection (id, name, notes, then 30 nutrient bound
// columns, then timestamps). Kept inline rather than reaching into goals'
// private goalColumns slice; the column shape is stable and the duplication
// is bounded to this one file.
const templatesSelectCols = `
    id, name, notes,
    kcal_min, kcal_max,
    protein_g_min, protein_g_max,
    carbs_g_min, carbs_g_max,
    fat_g_min, fat_g_max,
    fiber_g_min, fiber_g_max,
    sugar_g_min, sugar_g_max,
    salt_g_min, salt_g_max,
    iron_mg_min, iron_mg_max,
    calcium_mg_min, calcium_mg_max,
    vitamin_d_mcg_min, vitamin_d_mcg_max,
    vitamin_b12_mcg_min, vitamin_b12_mcg_max,
    vitamin_c_mg_min, vitamin_c_mg_max,
    magnesium_mg_min, magnesium_mg_max,
    potassium_mg_min, potassium_mg_max,
    zinc_mg_min, zinc_mg_max,
    created_at, updated_at
`

// Upsert writes the template by name, replacing every nutrient bound and
// notes wholesale (PUT full-replace semantics). Returns the freshly-stored
// template (so the handler can render the response without a second GET).
func (r *TemplatesRepo) Upsert(ctx context.Context, t *Template) (*Template, error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	const q = `
        INSERT INTO goal_templates (
            id, name, notes,
            kcal_min, kcal_max,
            protein_g_min, protein_g_max,
            carbs_g_min, carbs_g_max,
            fat_g_min, fat_g_max,
            fiber_g_min, fiber_g_max,
            sugar_g_min, sugar_g_max,
            salt_g_min, salt_g_max,
            iron_mg_min, iron_mg_max,
            calcium_mg_min, calcium_mg_max,
            vitamin_d_mcg_min, vitamin_d_mcg_max,
            vitamin_b12_mcg_min, vitamin_b12_mcg_max,
            vitamin_c_mg_min, vitamin_c_mg_max,
            magnesium_mg_min, magnesium_mg_max,
            potassium_mg_min, potassium_mg_max,
            zinc_mg_min, zinc_mg_max,
            updated_at
        ) VALUES (
            $1, $2, $3,
            $4, $5,
            $6, $7,
            $8, $9,
            $10, $11,
            $12, $13,
            $14, $15,
            $16, $17,
            $18, $19,
            $20, $21,
            $22, $23,
            $24, $25,
            $26, $27,
            $28, $29,
            $30, $31,
            $32, $33,
            now()
        )
        ON CONFLICT (name) DO UPDATE SET
            notes               = EXCLUDED.notes,
            kcal_min            = EXCLUDED.kcal_min,
            kcal_max            = EXCLUDED.kcal_max,
            protein_g_min       = EXCLUDED.protein_g_min,
            protein_g_max       = EXCLUDED.protein_g_max,
            carbs_g_min         = EXCLUDED.carbs_g_min,
            carbs_g_max         = EXCLUDED.carbs_g_max,
            fat_g_min           = EXCLUDED.fat_g_min,
            fat_g_max           = EXCLUDED.fat_g_max,
            fiber_g_min         = EXCLUDED.fiber_g_min,
            fiber_g_max         = EXCLUDED.fiber_g_max,
            sugar_g_min         = EXCLUDED.sugar_g_min,
            sugar_g_max         = EXCLUDED.sugar_g_max,
            salt_g_min          = EXCLUDED.salt_g_min,
            salt_g_max          = EXCLUDED.salt_g_max,
            iron_mg_min         = EXCLUDED.iron_mg_min,
            iron_mg_max         = EXCLUDED.iron_mg_max,
            calcium_mg_min      = EXCLUDED.calcium_mg_min,
            calcium_mg_max      = EXCLUDED.calcium_mg_max,
            vitamin_d_mcg_min   = EXCLUDED.vitamin_d_mcg_min,
            vitamin_d_mcg_max   = EXCLUDED.vitamin_d_mcg_max,
            vitamin_b12_mcg_min = EXCLUDED.vitamin_b12_mcg_min,
            vitamin_b12_mcg_max = EXCLUDED.vitamin_b12_mcg_max,
            vitamin_c_mg_min    = EXCLUDED.vitamin_c_mg_min,
            vitamin_c_mg_max    = EXCLUDED.vitamin_c_mg_max,
            magnesium_mg_min    = EXCLUDED.magnesium_mg_min,
            magnesium_mg_max    = EXCLUDED.magnesium_mg_max,
            potassium_mg_min    = EXCLUDED.potassium_mg_min,
            potassium_mg_max    = EXCLUDED.potassium_mg_max,
            zinc_mg_min         = EXCLUDED.zinc_mg_min,
            zinc_mg_max         = EXCLUDED.zinc_mg_max,
            updated_at          = now()
    `
	_, err := r.q.Exec(ctx, q,
		t.ID, t.Name, t.Notes,
		rangeMin(t.Kcal), rangeMax(t.Kcal),
		rangeMin(t.ProteinG), rangeMax(t.ProteinG),
		rangeMin(t.CarbsG), rangeMax(t.CarbsG),
		rangeMin(t.FatG), rangeMax(t.FatG),
		rangeMin(t.FiberG), rangeMax(t.FiberG),
		rangeMin(t.SugarG), rangeMax(t.SugarG),
		rangeMin(t.SaltG), rangeMax(t.SaltG),
		rangeMin(t.IronMg), rangeMax(t.IronMg),
		rangeMin(t.CalciumMg), rangeMax(t.CalciumMg),
		rangeMin(t.VitaminDMcg), rangeMax(t.VitaminDMcg),
		rangeMin(t.VitaminB12Mcg), rangeMax(t.VitaminB12Mcg),
		rangeMin(t.VitaminCMg), rangeMax(t.VitaminCMg),
		rangeMin(t.MagnesiumMg), rangeMax(t.MagnesiumMg),
		rangeMin(t.PotassiumMg), rangeMax(t.PotassiumMg),
		rangeMin(t.ZincMg), rangeMax(t.ZincMg),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert template: %w", err)
	}
	return r.GetByName(ctx, t.Name)
}

// GetByName returns the template with the given name.
func (r *TemplatesRepo) GetByName(ctx context.Context, name string) (*Template, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+templatesSelectCols+` FROM goal_templates WHERE name = $1`, name)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return t, nil
}

// GetByID returns the template with the given UUID. Used by the goals
// resolver when resolving a phase's default_template_id.
func (r *TemplatesRepo) GetByID(ctx context.Context, id uuid.UUID) (*Template, error) {
	row := r.q.QueryRow(ctx,
		`SELECT `+templatesSelectCols+` FROM goal_templates WHERE id = $1`, id)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("scan template by id: %w", err)
	}
	return t, nil
}

// GetByIDs returns templates for a set of IDs in one round-trip. Used by
// EffectiveForRange to batch-fetch the templates referenced by the phases
// intersecting a window. Missing IDs are silently omitted from the map.
func (r *TemplatesRepo) GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Template, error) {
	out := make(map[uuid.UUID]*Template, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.Query(ctx,
		`SELECT `+templatesSelectCols+` FROM goal_templates WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("list templates by ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out[t.ID] = t
	}
	return out, rows.Err()
}

// List returns every template ordered by name ascending.
func (r *TemplatesRepo) List(ctx context.Context) ([]*Template, error) {
	rows, err := r.q.Query(ctx,
		`SELECT `+templatesSelectCols+` FROM goal_templates ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	var out []*Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a template by name. Returns ErrTemplateNotFound if no row
// matched. Returns ErrTemplateInUse (with the referencing phases attached
// to the returned *InUseError) when the FK-RESTRICT trips.
func (r *TemplatesRepo) Delete(ctx context.Context, name string) error {
	// First look up the id so we can report referencing phases in the
	// in-use error case (and so we can return NotFound cleanly).
	t, err := r.GetByName(ctx, name)
	if err != nil {
		return err
	}
	tag, err := r.q.Exec(ctx, `DELETE FROM goal_templates WHERE id = $1`, t.ID)
	if err != nil {
		// pgx surfaces FK violations as pgconn.PgError with code 23503.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			refs, refsErr := r.referencingPhases(ctx, t.ID)
			if refsErr != nil {
				return fmt.Errorf("delete template (FK violation, refs lookup failed): %w", refsErr)
			}
			return &InUseError{ReferencingPhases: refs}
		}
		return fmt.Errorf("delete template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

// InUseError carries the list of phases blocking a template delete. The
// handler unwraps this to render `{"error":"template_in_use","referencing_phases":[...]}`.
type InUseError struct {
	ReferencingPhases []ReferencingPhase
}

func (e *InUseError) Error() string {
	return ErrTemplateInUse.Error()
}

func (e *InUseError) Unwrap() error {
	return ErrTemplateInUse
}

// referencingPhases returns the {id, name} pairs of every phase whose
// default_template_id equals templateID. Used by Delete to populate the
// 409 response body.
func (r *TemplatesRepo) referencingPhases(ctx context.Context, templateID uuid.UUID) ([]ReferencingPhase, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, name FROM training_phases WHERE default_template_id = $1 ORDER BY name ASC`,
		templateID)
	if err != nil {
		return nil, fmt.Errorf("query referencing phases: %w", err)
	}
	defer rows.Close()
	var out []ReferencingPhase
	for rows.Next() {
		var p ReferencingPhase
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("scan referencing phase: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// scanTemplate scans one row from the templatesSelectCols projection.
func scanTemplate(s scanner) (*Template, error) {
	var (
		t                                                          Template
		kcalMin, kcalMax                                           *float64
		proteinMin, proteinMax, carbsMin, carbsMax, fatMin, fatMax *float64
		fiberMin, fiberMax, sugarMin, sugarMax, saltMin, saltMax   *float64
		ironMin, ironMax                                           *float64
		calciumMin, calciumMax                                     *float64
		vitDMin, vitDMax, vitB12Min, vitB12Max                     *float64
		vitCMin, vitCMax, magnesiumMin, magnesiumMax               *float64
		potassiumMin, potassiumMax, zincMin, zincMax               *float64
	)
	if err := s.Scan(
		&t.ID, &t.Name, &t.Notes,
		&kcalMin, &kcalMax,
		&proteinMin, &proteinMax,
		&carbsMin, &carbsMax,
		&fatMin, &fatMax,
		&fiberMin, &fiberMax,
		&sugarMin, &sugarMax,
		&saltMin, &saltMax,
		&ironMin, &ironMax,
		&calciumMin, &calciumMax,
		&vitDMin, &vitDMax,
		&vitB12Min, &vitB12Max,
		&vitCMin, &vitCMax,
		&magnesiumMin, &magnesiumMax,
		&potassiumMin, &potassiumMax,
		&zincMin, &zincMax,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.Kcal = rangeOrNil(kcalMin, kcalMax)
	t.ProteinG = rangeOrNil(proteinMin, proteinMax)
	t.CarbsG = rangeOrNil(carbsMin, carbsMax)
	t.FatG = rangeOrNil(fatMin, fatMax)
	t.FiberG = rangeOrNil(fiberMin, fiberMax)
	t.SugarG = rangeOrNil(sugarMin, sugarMax)
	t.SaltG = rangeOrNil(saltMin, saltMax)
	t.IronMg = rangeOrNil(ironMin, ironMax)
	t.CalciumMg = rangeOrNil(calciumMin, calciumMax)
	t.VitaminDMcg = rangeOrNil(vitDMin, vitDMax)
	t.VitaminB12Mcg = rangeOrNil(vitB12Min, vitB12Max)
	t.VitaminCMg = rangeOrNil(vitCMin, vitCMax)
	t.MagnesiumMg = rangeOrNil(magnesiumMin, magnesiumMax)
	t.PotassiumMg = rangeOrNil(potassiumMin, potassiumMax)
	t.ZincMg = rangeOrNil(zincMin, zincMax)
	return &t, nil
}

// scanner is the minimal interface both pgx.Row and pgx.Rows satisfy.
type scanner interface {
	Scan(dest ...any) error
}

// rangeOrNil returns nil when both bounds are nil (the row's nutrient is
// unset), otherwise a *goals.Range carrying whichever bounds are present.
func rangeOrNil(min, max *float64) *goals.Range {
	if min == nil && max == nil {
		return nil
	}
	return &goals.Range{Min: min, Max: max}
}

func rangeMin(r *goals.Range) *float64 {
	if r == nil {
		return nil
	}
	return r.Min
}

func rangeMax(r *goals.Range) *float64 {
	if r == nil {
		return nil
	}
	return r.Max
}
