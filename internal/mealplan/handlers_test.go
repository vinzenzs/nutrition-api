package mealplan_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/mealplan"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

type fixture struct {
	r            *gin.Engine
	pool         *pgxpool.Pool
	productsRepo *products.Repo
}

func wire(pool *pgxpool.Pool) *mealplan.Service {
	productsRepo := products.NewRepo(pool)
	mealsSvc := meals.NewService(pool, meals.NewRepo(pool), productsRepo)
	svc := mealplan.NewService(pool, mealplan.NewRepo(pool))
	svc.SetProductsRepo(productsRepo)
	svc.SetMealsService(mealsSvc)
	return svc
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	mealplan.NewHandlers(wire(pool)).Register(r.Group("/"))
	return &fixture{r: r, pool: pool, productsRepo: products.NewRepo(pool)}
}

func setupWithMiddleware(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	r.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	r.Use(idempotency.Middleware(idempotency.NewRepo(pool), time.Hour))
	mealplan.NewHandlers(wire(pool)).Register(r.Group("/"))
	return &fixture{r: r, pool: pool, productsRepo: products.NewRepo(pool)}
}

func makeProduct(t *testing.T, repo *products.Repo) uuid.UUID {
	t.Helper()
	kcal, protein := 539.0, 6.3
	serving := 400.0
	p := &products.Product{
		Name:         "Lasagne",
		Source:       products.SourceManual,
		ServingSizeG: &serving,
		Nutriments:   products.Nutriments{KcalPer100g: &kcal, ProteinGPer100g: &protein},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func do(t *testing.T, r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func countMeals(t *testing.T, pool *pgxpool.Pool, productID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM meal_entries WHERE product_id = $1", productID).Scan(&n))
	return n
}

func createPlan(t *testing.T, f *fixture, pid uuid.UUID, date, slot string, qty *float64) mealplan.PlannedMeal {
	t.Helper()
	q := ""
	if qty != nil {
		q = fmt.Sprintf(`, "quantity_g": %v`, *qty)
	}
	body := fmt.Sprintf(`{"plan_date":"%s","slot":"%s","product_id":"%s"%s}`, date, slot, pid, q)
	w := do(t, f.r, http.MethodPost, "/plan", body, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var pm mealplan.PlannedMeal
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pm))
	return pm
}

func TestCRUDRoundTrip(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	q := 450.0
	pm := createPlan(t, f, pid, "2026-06-12", "dinner", &q)
	assert.Equal(t, "planned", pm.Status)
	assert.Equal(t, "Lasagne", pm.ProductName)
	require.NotNil(t, pm.QuantityG)
	assert.Equal(t, 450.0, *pm.QuantityG)

	w := do(t, f.r, http.MethodGet, "/plan/"+pm.ID.String(), "", nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = do(t, f.r, http.MethodDelete, "/plan/"+pm.ID.String(), "", nil)
	require.Equal(t, http.StatusNoContent, w.Code)
	w = do(t, f.r, http.MethodGet, "/plan/"+pm.ID.String(), "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRangeOrderingAndInclusivity(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	createPlan(t, f, pid, "2026-06-13", "dinner", nil)
	createPlan(t, f, pid, "2026-06-12", "lunch", nil)
	createPlan(t, f, pid, "2026-06-12", "breakfast", nil)
	createPlan(t, f, pid, "2026-06-15", "dinner", nil) // outside range

	w := do(t, f.r, http.MethodGet, "/plan?from=2026-06-12&to=2026-06-14", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		PlannedMeals []mealplan.PlannedMeal `json:"planned_meals"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.PlannedMeals, 3, "2026-06-15 must be excluded")
	// Ordered: 06-12 breakfast, 06-12 lunch, 06-13 dinner.
	assert.Equal(t, "2026-06-12", resp.PlannedMeals[0].PlanDate)
	assert.Equal(t, "breakfast", resp.PlannedMeals[0].Slot)
	assert.Equal(t, "lunch", resp.PlannedMeals[1].Slot)
	assert.Equal(t, "2026-06-13", resp.PlannedMeals[2].PlanDate)
}

func TestUnknownProductRejected(t *testing.T) {
	f := setup(t)
	body := fmt.Sprintf(`{"plan_date":"2026-06-12","slot":"dinner","product_id":"%s"}`, uuid.New())
	w := do(t, f.r, http.MethodPost, "/plan", body, nil)
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "product_not_found")
}

func TestTwoDinnersLegal(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	createPlan(t, f, pid, "2026-06-12", "dinner", nil)
	createPlan(t, f, pid, "2026-06-12", "dinner", nil)
	w := do(t, f.r, http.MethodGet, "/plan?from=2026-06-12&to=2026-06-12", "", nil)
	var resp struct {
		PlannedMeals []mealplan.PlannedMeal `json:"planned_meals"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.PlannedMeals, 2)
}

func TestEatenHappyPath(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	q := 450.0
	pm := createPlan(t, f, pid, "2026-06-12", "dinner", &q)

	w := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Plan mealplan.PlannedMeal   `json:"plan"`
		Meal map[string]interface{} `json:"meal"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "eaten", resp.Plan.Status)
	require.NotNil(t, resp.Plan.MealEntryID)
	assert.Equal(t, 450.0, resp.Meal["quantity_g"])
	assert.Equal(t, "dinner", resp.Meal["meal_type"])
	assert.Equal(t, 1, countMeals(t, f.pool, pid))
}

func TestEatenUsesServingDefaultWhenNoQuantity(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo) // serving 400
	pm := createPlan(t, f, pid, "2026-06-12", "lunch", nil)
	w := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Meal map[string]interface{} `json:"meal"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 400.0, resp.Meal["quantity_g"], "falls back to product serving size")
}

func TestDoubleEatenConflict(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	pm := createPlan(t, f, pid, "2026-06-12", "dinner", nil)
	require.Equal(t, http.StatusOK, do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", nil).Code)

	// Genuine second tap (no idempotency key) → conflict, no second meal.
	w := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", nil)
	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "plan_entry_already_eaten")
	assert.Equal(t, 1, countMeals(t, f.pool, pid))
}

func TestFutureLoggedAtRejectedAtomically(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	pm := createPlan(t, f, pid, "2026-06-12", "dinner", nil)
	future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	w := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten",
		fmt.Sprintf(`{"logged_at":"%s"}`, future), nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	// Neither write occurred: no meal, plan still planned.
	assert.Equal(t, 0, countMeals(t, f.pool, pid))
	g := do(t, f.r, http.MethodGet, "/plan/"+pm.ID.String(), "", nil)
	assert.Contains(t, g.Body.String(), `"status":"planned"`)
}

func TestSkippedCanBeUnskippedAndEatenTerminal(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	pm := createPlan(t, f, pid, "2026-06-12", "dinner", nil)
	id := pm.ID.String()

	// planned → skipped → planned (un-skip).
	require.Equal(t, http.StatusOK, do(t, f.r, http.MethodPatch, "/plan/"+id, `{"status":"skipped"}`, nil).Code)
	w := do(t, f.r, http.MethodPatch, "/plan/"+id, `{"status":"planned"}`, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"planned"`)

	// Eat it, then any PATCH away from eaten is a conflict.
	require.Equal(t, http.StatusOK, do(t, f.r, http.MethodPost, "/plan/"+id+"/eaten", "", nil).Code)
	w = do(t, f.r, http.MethodPatch, "/plan/"+id, `{"status":"planned"}`, nil)
	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "plan_entry_eaten_terminal")
}

// Task 3.3: idempotency replay returns the original response without a second meal.
func TestEatenIdempotencyReplay(t *testing.T) {
	f := setupWithMiddleware(t)
	pid := makeProduct(t, f.productsRepo)
	auth := map[string]string{"Authorization": "Bearer " + mobileToken}
	q := 300.0
	body := fmt.Sprintf(`{"plan_date":"2026-06-12","slot":"dinner","product_id":"%s","quantity_g":%v}`, pid, q)
	cw := do(t, f.r, http.MethodPost, "/plan", body, auth)
	require.Equal(t, http.StatusCreated, cw.Code, cw.Body.String())
	var pm mealplan.PlannedMeal
	require.NoError(t, json.Unmarshal(cw.Body.Bytes(), &pm))

	key := map[string]string{"Authorization": "Bearer " + mobileToken, "Idempotency-Key": "eaten-once"}
	w1 := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", key)
	require.Equal(t, http.StatusOK, w1.Code, w1.Body.String())
	w2 := do(t, f.r, http.MethodPost, "/plan/"+pm.ID.String()+"/eaten", "", key)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.JSONEq(t, w1.Body.String(), w2.Body.String(), "replay returns the original response")
	assert.Equal(t, 1, countMeals(t, f.pool, pid), "replay must not create a second meal entry")
}
