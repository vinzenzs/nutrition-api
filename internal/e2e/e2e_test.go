// Package e2e contains a single end-to-end happy-path test that boots the
// full HTTP server (auth + idempotency + products + meals + summary) against a
// testcontainers-managed Postgres and a stubbed OFF transport.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/auth"
	"github.com/vinzenzs/nutrition-api/internal/goals"
	"github.com/vinzenzs/nutrition-api/internal/idempotency"
	"github.com/vinzenzs/nutrition-api/internal/meals"
	"github.com/vinzenzs/nutrition-api/internal/off"
	"github.com/vinzenzs/nutrition-api/internal/products"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
	"github.com/vinzenzs/nutrition-api/internal/summary"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const (
	mobileToken = "mobile-token-aaaaaaaaaaaaaa"
	agentToken  = "agent-token-bbbbbbbbbbbbbbbb"
)

// stubOFFTransport returns a recorded fixture for any path that contains the
// given barcode. Used in lieu of hitting the real OFF endpoint.
type stubOFFTransport struct {
	barcode string
	body    []byte
}

func (s *stubOFFTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func bootServer(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)

	fixtureBody, err := os.ReadFile("../../testdata/off/3017624010701.json")
	require.NoError(t, err)
	transport := &stubOFFTransport{barcode: "3017624010701", body: fixtureBody}

	offClient, err := off.New(off.Config{
		HTTPClient: &http.Client{Transport: transport},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	pRepo := products.NewRepo(pool)
	mRepo := meals.NewRepo(pool)
	pSvc := products.NewService(pool, pRepo, offClient)
	mSvc := meals.NewService(pool, mRepo, pRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo, nil, nil)
	sSvc := summary.NewService(pool, mRepo, resolver)
	idRepo := idempotency.NewRepo(pool)

	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/")
	api.Use(auth.Middleware(auth.Config{MobileToken: mobileToken, AgentToken: agentToken}))
	api.Use(idempotency.Middleware(idRepo, time.Hour))
	products.NewHandlers(pSvc).Register(api)
	meals.NewHandlers(mSvc).Register(api)
	summary.NewHandlers(sSvc, "UTC", slog.New(slog.NewTextHandler(io.Discard, nil))).Register(api)
	return r
}

type client struct {
	t      *testing.T
	r      *gin.Engine
	token  string
	idKey  string // optional
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.idKey != "" {
		req.Header.Set(idempotency.HeaderName, c.idKey)
	}
	rec := httptest.NewRecorder()
	c.r.ServeHTTP(rec, req)
	return rec
}

func TestE2E_FullHappyPath(t *testing.T) {
	r := bootServer(t)
	mobile := &client{t: t, r: r, token: mobileToken}
	agent := &client{t: t, r: r, token: agentToken}

	ctx := context.Background()
	_ = ctx

	// ---------- 1. Lookup new barcode (cache miss → OFF fetch) ----------
	rec := mobile.do(http.MethodPost, "/products/lookup/3017624010701", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var product products.Product
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &product))
	assert.Equal(t, "Nutella", product.Name)
	assert.Equal(t, products.SourceOFF, product.Source)

	// ---------- 2. Log a meal entry using that product ----------
	logMealReq := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":100,"logged_at":"2026-06-06T08:00:00Z","meal_type":"breakfast"}`,
		product.ID,
	)
	mobile.idKey = "meal-1"
	rec = mobile.do(http.MethodPost, "/meals", logMealReq)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var meal meals.MealEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meal))
	mobile.idKey = ""

	// ---------- 3. Daily summary shows it (UTC) ----------
	rec = mobile.do(http.MethodGet, "/summary/daily?date=2026-06-06&tz=UTC", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var day summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &day))
	require.Len(t, day.Entries, 1)
	// 539 kcal/100g * 100g / 100 = 539 kcal
	assert.InDelta(t, 539.0, day.Totals.Kcal, 0.1)

	// ---------- 4. Patch quantity → daily summary updates ----------
	patch := `{"quantity_g":200}`
	rec = mobile.do(http.MethodPatch, "/meals/"+meal.ID.String(), patch)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = mobile.do(http.MethodGet, "/summary/daily?date=2026-06-06&tz=UTC", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var dayPatched summary.Daily
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dayPatched))
	assert.InDelta(t, 1078.0, dayPatched.Totals.Kcal, 0.5)

	// ---------- 5. Freeform log via the agent ----------
	freeform := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"protein_g":1.1,"carbs_g":22.8,"fat_g":0.3},
        "quantity_g":120,
        "logged_at":"2026-06-06T15:00:00Z",
        "save_as_product": true
    }`
	agent.idKey = "freeform-1"
	rec = agent.do(http.MethodPost, "/meals/freeform", freeform)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	agent.idKey = ""

	// ---------- 6. Range summary includes both days ----------
	rec = mobile.do(http.MethodGet, "/summary/range?from=2026-06-01&to=2026-06-07&tz=UTC", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rng summary.Range
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rng))
	require.Len(t, rng.Days, 7)
	var jun6 *summary.RangeDay
	for i := range rng.Days {
		if rng.Days[i].Date == "2026-06-06" {
			jun6 = &rng.Days[i]
			break
		}
	}
	require.NotNil(t, jun6, "June 6 should appear in range")
	// 1078 (nutella) + 89 * 120 / 100 = 106.8 banana = 1184.8 total
	assert.InDelta(t, 1184.8, jun6.Totals.Kcal, 1.0)

	// ---------- 7. Idempotent replay of a meal log returns the same id ----------
	replayBody := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":300,"logged_at":"2026-06-06T09:00:00Z"}`,
		product.ID,
	)
	mobile.idKey = "meal-replay"
	first := mobile.do(http.MethodPost, "/meals", replayBody)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	var firstMeal meals.MealEntry
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstMeal))

	second := mobile.do(http.MethodPost, "/meals", replayBody)
	require.Equal(t, http.StatusCreated, second.Code, second.Body.String())
	var secondMeal meals.MealEntry
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondMeal))
	assert.Equal(t, firstMeal.ID, secondMeal.ID, "replay should return same id")

	// ---------- 8. Conflicting body with same key returns 409 ----------
	conflictBody := fmt.Sprintf(
		`{"product_id":%q,"quantity_g":999,"logged_at":"2026-06-06T09:00:00Z"}`,
		product.ID,
	)
	conflict := mobile.do(http.MethodPost, "/meals", conflictBody)
	assert.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	assert.JSONEq(t, `{"error":"idempotency_key_conflict"}`, conflict.Body.String())
	mobile.idKey = ""
}

// TestE2E_FreeformReplayReturnsSameMealID exercises the canonical idempotency
// path on POST /meals/freeform: two byte-identical calls with the same
// explicit Idempotency-Key return 201 and the same meal id. This is the test
// the original MCP-driven session never actually ran (harden-write-paths
// section 5.7 in tasks.md).
func TestE2E_FreeformReplayReturnsSameMealID(t *testing.T) {
	r := bootServer(t)
	agent := &client{t: t, r: r, token: agentToken, idKey: "freeform-replay-key"}

	body := `{
        "name":"banana",
        "nutriments_per_100g":{"kcal":89,"protein_g":1.1,"carbs_g":22.8,"fat_g":0.3},
        "quantity_g":120,
        "logged_at":"2026-06-07T10:00:00Z"
    }`

	first := agent.do(http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	var firstMeal meals.MealEntry
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstMeal))

	second := agent.do(http.MethodPost, "/meals/freeform", body)
	require.Equal(t, http.StatusCreated, second.Code, second.Body.String())
	var secondMeal meals.MealEntry
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondMeal))

	assert.Equal(t, firstMeal.ID, secondMeal.ID, "freeform replay must return the same meal id")
}
