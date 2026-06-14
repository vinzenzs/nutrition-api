package trainingphases_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/idempotency"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type handlerFix struct {
	r          *gin.Engine
	templates  *trainingphases.TemplatesRepo
	phases     *trainingphases.PhasesRepo
	authNeeded bool
}

func setupHandlers(t *testing.T, withAuth bool) *handlerFix {
	t.Helper()
	pool := storetest.NewPool(t)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	tplSvc := trainingphases.NewTemplatesService(tplRepo)
	phRepo := trainingphases.NewPhasesRepo(pool)
	phSvc := trainingphases.NewPhasesService(phRepo, tplRepo)
	idRepo := idempotency.NewRepo(pool)

	r := gin.New()
	rg := r.Group("/")
	if withAuth {
		rg.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
		rg.Use(idempotency.Middleware(idRepo, time.Hour))
	}
	trainingphases.NewTemplatesHandlers(tplSvc).Register(rg)
	trainingphases.NewPhasesHandlers(phSvc).Register(rg)
	return &handlerFix{r: r, templates: tplRepo, phases: phRepo, authNeeded: withAuth}
}

func (f *handlerFix) do(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if f.authNeeded {
		req.Header.Set("Authorization", "Bearer "+mobileToken)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

// ----- templates -----

func TestTemplatePut_HappyPath(t *testing.T) {
	f := setupHandlers(t, false)
	body := `{"kcal":{"min":2090,"max":2310},"protein_g":{"min":150,"max":190},"notes":"weekday easy"}`
	rec := f.do(http.MethodPut, "/goal-templates/weekday-easy", body, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Template *trainingphases.Template `json:"template"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "weekday-easy", resp.Template.Name)
	require.NotNil(t, resp.Template.Kcal)
	assert.InDelta(t, 2090, *resp.Template.Kcal.Min, 0.001)
}

func TestTemplatePut_IdempotencyKeyRejected(t *testing.T) {
	f := setupHandlers(t, true)
	body := `{"kcal":{"min":2090,"max":2310}}`
	rec := f.do(http.MethodPut, "/goal-templates/weekday-easy", body, map[string]string{
		"Idempotency-Key": "key-1",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "idempotency_unsupported_for_put")
}

func TestTemplatePut_InvalidGoalValue(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodPut, "/goal-templates/bad", `{"kcal":{"min":-50}}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "goal_value_invalid")
}

func TestTemplatePut_InvertedRange(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodPut, "/goal-templates/bad", `{"kcal":{"min":2500,"max":2300}}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "goal_range_invalid")
}

func TestTemplateGet_NotFound(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodGet, "/goal-templates/missing", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "template_not_found")
}

func TestTemplateList(t *testing.T) {
	f := setupHandlers(t, false)
	for _, n := range []string{"easy", "hard", "race"} {
		_ = f.do(http.MethodPut, "/goal-templates/"+n, `{"kcal":{"min":2000}}`, nil)
	}
	rec := f.do(http.MethodGet, "/goal-templates", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Templates []*trainingphases.Template `json:"templates"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Templates, 3)
	assert.Equal(t, "easy", resp.Templates[0].Name)
}

func TestTemplateDelete_HappyPath(t *testing.T) {
	f := setupHandlers(t, false)
	_ = f.do(http.MethodPut, "/goal-templates/easy", `{"kcal":{"min":2000}}`, nil)
	rec := f.do(http.MethodDelete, "/goal-templates/easy", "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTemplateDelete_RefusedWhenReferenced(t *testing.T) {
	f := setupHandlers(t, false)
	// Create template + phase referencing it.
	tplRec := f.do(http.MethodPut, "/goal-templates/easy", `{"kcal":{"min":2000}}`, nil)
	require.Equal(t, http.StatusOK, tplRec.Code)
	var tplResp struct {
		Template *trainingphases.Template `json:"template"`
	}
	require.NoError(t, json.Unmarshal(tplRec.Body.Bytes(), &tplResp))

	phBody := fmt.Sprintf(`{"name":"build-1","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","default_template_id":%q}`, tplResp.Template.ID.String())
	require.Equal(t, http.StatusCreated, f.do(http.MethodPost, "/phases", phBody, nil).Code)

	rec := f.do(http.MethodDelete, "/goal-templates/easy", "", nil)
	require.Equal(t, http.StatusConflict, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "template_in_use")
	assert.Contains(t, body, "build-1")
}

// ----- phases -----

func TestPhasePost_HappyPath(t *testing.T) {
	f := setupHandlers(t, false)
	body := `{"name":"build-block-2","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","notes":"weeks 5-8"}`
	rec := f.do(http.MethodPost, "/phases", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var resp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "build-block-2", resp.Phase.Name)
	assert.Equal(t, trainingphases.PhaseTypeBuild, resp.Phase.Type)
	assert.Nil(t, resp.Phase.DefaultTemplateID)
	assert.Nil(t, resp.Phase.DefaultTemplateName)
}

func TestPhasePost_RejectsInvertedDates(t *testing.T) {
	f := setupHandlers(t, false)
	body := `{"name":"bad","type":"build","start_date":"2026-08-01","end_date":"2026-07-01"}`
	rec := f.do(http.MethodPost, "/phases", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "date_range_invalid")
}

func TestPhasePost_RejectsInvalidType(t *testing.T) {
	f := setupHandlers(t, false)
	body := `{"name":"x","type":"kettlebell","start_date":"2026-07-01","end_date":"2026-07-28"}`
	rec := f.do(http.MethodPost, "/phases", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "phase_type_invalid")
}

func TestPhasePost_RejectsMissingTemplate(t *testing.T) {
	f := setupHandlers(t, false)
	body := fmt.Sprintf(
		`{"name":"x","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","default_template_id":%q}`,
		uuid.New().String())
	rec := f.do(http.MethodPost, "/phases", body, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "template_not_found")
}

func TestPhaseGet_NotFound(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodGet, "/phases/"+uuid.New().String(), "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "phase_not_found")
}

func TestPhaseList_RangeRequired(t *testing.T) {
	f := setupHandlers(t, false)
	rec := f.do(http.MethodGet, "/phases", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "range_required")
}

func TestPhasePatch_TemplateAssignment(t *testing.T) {
	f := setupHandlers(t, false)
	// Create phase + template, then patch the phase to point at the template.
	postRec := f.do(http.MethodPost, "/phases",
		`{"name":"x","type":"build","start_date":"2026-07-01","end_date":"2026-07-28"}`, nil)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var postResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &postResp))

	tplRec := f.do(http.MethodPut, "/goal-templates/easy", `{"kcal":{"min":2000}}`, nil)
	require.Equal(t, http.StatusOK, tplRec.Code)
	var tplResp struct {
		Template *trainingphases.Template `json:"template"`
	}
	require.NoError(t, json.Unmarshal(tplRec.Body.Bytes(), &tplResp))

	patchBody := fmt.Sprintf(`{"default_template_id":%q}`, tplResp.Template.ID.String())
	patchRec := f.do(http.MethodPatch, "/phases/"+postResp.Phase.ID.String(), patchBody, nil)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var patchResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patchResp))
	require.NotNil(t, patchResp.Phase.DefaultTemplateID)
	assert.Equal(t, tplResp.Template.ID, *patchResp.Phase.DefaultTemplateID)
	require.NotNil(t, patchResp.Phase.DefaultTemplateName)
	assert.Equal(t, "easy", *patchResp.Phase.DefaultTemplateName)
}

func TestPhasePatch_ClearsTemplate(t *testing.T) {
	f := setupHandlers(t, false)
	tplRec := f.do(http.MethodPut, "/goal-templates/easy", `{"kcal":{"min":2000}}`, nil)
	var tplResp struct {
		Template *trainingphases.Template `json:"template"`
	}
	require.NoError(t, json.Unmarshal(tplRec.Body.Bytes(), &tplResp))

	postRec := f.do(http.MethodPost, "/phases",
		fmt.Sprintf(`{"name":"x","type":"build","start_date":"2026-07-01","end_date":"2026-07-28","default_template_id":%q}`,
			tplResp.Template.ID.String()), nil)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var postResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &postResp))

	// Empty string sentinel clears the template ref.
	patchRec := f.do(http.MethodPatch, "/phases/"+postResp.Phase.ID.String(),
		`{"default_template_id":""}`, nil)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var patchResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patchResp))
	assert.Nil(t, patchResp.Phase.DefaultTemplateID)
}

func TestPhaseDelete_HappyPath(t *testing.T) {
	f := setupHandlers(t, false)
	postRec := f.do(http.MethodPost, "/phases",
		`{"name":"x","type":"build","start_date":"2026-07-01","end_date":"2026-07-28"}`, nil)
	var postResp struct {
		Phase *trainingphases.Phase `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &postResp))

	rec := f.do(http.MethodDelete, "/phases/"+postResp.Phase.ID.String(), "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestPhaseAuth_RequiresBearerToken(t *testing.T) {
	f := setupHandlers(t, true)
	req := httptest.NewRequest(http.MethodPost, "/phases", nil) // no Authorization header
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
