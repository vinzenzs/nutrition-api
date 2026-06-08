package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mealRecord struct {
	method    string
	path      string
	body      []byte
	idemKey   string
}

func newMealRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]mealRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []mealRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, mealRecord{
			method:  r.Method,
			path:    r.URL.Path,
			body:    body,
			idemKey: r.Header.Get("Idempotency-Key"),
		})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, &records
}

func TestLogMeal_PostsToMealsWithIdempotencyKey(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	args := LogMealArgs{
		ProductID: "abc",
		QuantityG: 100,
		LoggedAt:  "2026-06-06T12:00:00Z",
		MealType:  "lunch",
	}
	r := handleLogMeal(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/meals", rec.path)
	assert.JSONEq(t,
		`{"product_id":"abc","quantity_g":100,"logged_at":"2026-06-06T12:00:00Z","meal_type":"lunch"}`,
		string(rec.body))
	assert.NotEmpty(t, rec.idemKey, "derived idempotency key should be present")
}

func TestLogMeal_ExplicitIdempotencyKeyIsForwarded(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	args := LogMealArgs{
		ProductID:      "abc",
		QuantityG:      100,
		LoggedAt:       "2026-06-06T12:00:00Z",
		IdempotencyKey: "explicit-key",
	}
	_ = handleLogMeal(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-key", (*recs)[0].idemKey)
}

func TestLogMeal_SameArgsTwiceProducesSameDerivedKey(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m1"}`)
	args := LogMealArgs{ProductID: "abc", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z"}
	_ = handleLogMeal(context.Background(), c, args)
	_ = handleLogMeal(context.Background(), c, args)
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey)
}

func TestLogMealFreeform_PostsToFreeformEndpoint(t *testing.T) {
	c, recs := newMealRecorder(t, 201, `{"id":"m2"}`)
	kcal := 89.0
	args := LogMealFreeformArgs{
		Name:              "banana",
		NutrimentsPer100g: FreeformNutriments{Kcal: &kcal},
		QuantityG:         120,
		LoggedAt:          "2026-06-06T10:00:00Z",
		SaveAsProduct:     true,
	}
	r := handleLogMealFreeform(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, "/meals/freeform", rec.path)
	assert.Contains(t, string(rec.body), `"name":"banana"`)
	assert.Contains(t, string(rec.body), `"save_as_product":true`)
}

func TestPatchMeal_OnlySuppliedFieldsAreSent(t *testing.T) {
	c, recs := newMealRecorder(t, 200, `{"id":"m1"}`)
	qty := 200.0
	args := PatchMealArgs{
		MealID:    "abc",
		QuantityG: &qty,
	}
	_ = handlePatchMeal(context.Background(), c, args)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPatch, rec.method)
	assert.Equal(t, "/meals/abc", rec.path)
	assert.JSONEq(t, `{"quantity_g":200}`, string(rec.body))
}

func TestDeleteMeal_204ReturnsEmptySuccessResult(t *testing.T) {
	c, recs := newMealRecorder(t, 204, "")
	r := handleDeleteMeal(context.Background(), c, DeleteMealArgs{MealID: "abc"})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	tc, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Empty(t, tc.Text)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodDelete, (*recs)[0].method)
	assert.Equal(t, "/meals/abc", (*recs)[0].path)
}

func TestDeleteMeal_404ReturnsIsError(t *testing.T) {
	c, _ := newMealRecorder(t, 404, `{"error":"meal_not_found"}`)
	r := handleDeleteMeal(context.Background(), c, DeleteMealArgs{MealID: "abc"})
	assert.True(t, r.IsError)
}

func TestLogMeal_404ProductNotFoundForwarded(t *testing.T) {
	c, _ := newMealRecorder(t, 404, `{"error":"product_not_found"}`)
	r := handleLogMeal(context.Background(), c, LogMealArgs{
		ProductID: "missing", QuantityG: 100, LoggedAt: "2026-06-06T12:00:00Z",
	})
	assert.True(t, r.IsError)
}
