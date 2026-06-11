package mcpserver

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatePlannedMeal_PostsBodyWithKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 201, `{"id":"p1"}`)
	q := 450.0
	handleCreatePlannedMeal(context.Background(), c, CreatePlannedMealArgs{
		PlanDate:  "2026-06-12",
		Slot:      "dinner",
		ProductID: "prod-1",
		QuantityG: &q,
	})
	rec := (*records)[0]
	assert.Equal(t, "POST", rec.method)
	assert.Equal(t, "/plan", rec.path)
	assert.NotEmpty(t, rec.idemKey)
	assert.Contains(t, rec.body, `"slot":"dinner"`)
	assert.Contains(t, rec.body, `"product_id":"prod-1"`)
	assert.NotContains(t, rec.body, "idempotency_key")
}

func TestListPlannedMeals_ForwardsRangeNoKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{"planned_meals":[]}`)
	handleListPlannedMealsForTest(t, c)
	rec := (*records)[0]
	assert.Equal(t, "GET", rec.method)
	assert.Equal(t, "/plan", rec.path)
	assert.Contains(t, rec.rawQuery, "from=2026-06-12")
	assert.Contains(t, rec.rawQuery, "to=2026-06-14")
	assert.Empty(t, rec.idemKey)
}

func TestMarkPlannedMealEaten_PostsToEatenWithKey(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{"plan":{},"meal":{}}`)
	handleMarkPlannedMealEaten(context.Background(), c, MarkPlannedMealEatenArgs{ID: "p1"})
	rec := (*records)[0]
	assert.Equal(t, "POST", rec.method)
	assert.Equal(t, "/plan/p1/eaten", rec.path)
	assert.NotEmpty(t, rec.idemKey)
}

func TestUpdatePlannedMeal_PatchesStatus(t *testing.T) {
	c, records := newRacePrepRecorder(t, 200, `{}`)
	skipped := "skipped"
	handleUpdatePlannedMeal(context.Background(), c, UpdatePlannedMealArgs{ID: "p1", Status: &skipped})
	rec := (*records)[0]
	assert.Equal(t, "PATCH", rec.method)
	assert.Equal(t, "/plan/p1", rec.path)
	assert.Contains(t, rec.body, `"status":"skipped"`)
	assert.False(t, strings.Contains(rec.body, "plan_date"), "omitted fields must not be sent")
}

// list has no dedicated handler func; exercise the one GET it issues.
func handleListPlannedMealsForTest(t *testing.T, c *apiClient) {
	t.Helper()
	q := url.Values{}
	q.Set("from", "2026-06-12")
	q.Set("to", "2026-06-14")
	_, _, _ = c.Get(context.Background(), "/plan", q)
}
