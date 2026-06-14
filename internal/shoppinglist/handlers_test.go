package shoppinglist_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/shoppinglist"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r            *gin.Engine
	pool         *pgxpool.Pool
	productsRepo *products.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	productsRepo := products.NewRepo(pool)
	svc := shoppinglist.NewService(shoppinglist.NewRepo(pool))
	svc.SetProductsRepo(productsRepo)
	r := gin.New()
	shoppinglist.NewHandlers(svc).Register(r.Group("/"))
	return &fixture{r: r, pool: pool, productsRepo: productsRepo}
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func count(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM shopping_items").Scan(&n))
	return n
}

func makeRecipe(t *testing.T, repo *products.Repo) uuid.UUID {
	t.Helper()
	p := &products.Product{Name: "Lasagne recipe", Source: products.SourceRecipe}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

type listResp struct {
	Items []shoppinglist.Item `json:"items"`
}

func TestBulkCreateOrderPreserved(t *testing.T) {
	f := setup(t)
	body := `{"items":[
		{"name":"Zwiebeln","quantity_text":"3"},
		{"name":"Staudensellerie","quantity_text":"100 g"},
		{"name":"Hackfleisch"}
	]}`
	w := do(t, f.r, http.MethodPost, "/shopping/items", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp listResp
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 3)
	assert.Equal(t, "Zwiebeln", resp.Items[0].Name)
	assert.Equal(t, "Staudensellerie", resp.Items[1].Name)
	assert.Equal(t, "Hackfleisch", resp.Items[2].Name)
	assert.False(t, resp.Items[0].Checked)

	// List preserves order too.
	lw := do(t, f.r, http.MethodGet, "/shopping/items", "")
	var ll listResp
	require.NoError(t, json.Unmarshal(lw.Body.Bytes(), &ll))
	require.Len(t, ll.Items, 3)
	assert.Equal(t, "Zwiebeln", ll.Items[0].Name)
	assert.Equal(t, "Hackfleisch", ll.Items[2].Name)
}

func TestBulkAtomicityBadIndex(t *testing.T) {
	f := setup(t)
	body := `{"items":[{"name":"ok1"},{"name":"ok2"},{"name":""},{"name":"ok4"},{"name":"ok5"}]}`
	w := do(t, f.r, http.MethodPost, "/shopping/items", body)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var e struct {
		Error string `json:"error"`
		Index int    `json:"index"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	assert.Equal(t, "name_invalid", e.Error)
	assert.Equal(t, 2, e.Index, "offending index reported")
	assert.Equal(t, 0, count(t, f.pool), "zero rows on a failed batch")
}

func TestEmptyBatchRejected(t *testing.T) {
	f := setup(t)
	w := do(t, f.r, http.MethodPost, "/shopping/items", `{"items":[]}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "items_required")
}

func TestUnknownProductRejected(t *testing.T) {
	f := setup(t)
	body := fmt.Sprintf(`{"items":[{"name":"X","recipe_product_id":"%s"}]}`, uuid.New())
	w := do(t, f.r, http.MethodPost, "/shopping/items", body)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "product_not_found")
	assert.Equal(t, 0, count(t, f.pool))
}

func TestDefaultVsIncludeChecked(t *testing.T) {
	f := setup(t)
	do(t, f.r, http.MethodPost, "/shopping/items", `{"items":[{"name":"a"},{"name":"b"},{"name":"c"},{"name":"d"},{"name":"e"}]}`)
	var all listResp
	json.Unmarshal(do(t, f.r, http.MethodGet, "/shopping/items", "").Body.Bytes(), &all)
	// Check two of them.
	for _, i := range []int{1, 3} {
		w := do(t, f.r, http.MethodPatch, "/shopping/items/"+all.Items[i].ID.String(), `{"checked":true}`)
		require.Equal(t, http.StatusOK, w.Code)
	}
	// Default hides checked.
	var def listResp
	json.Unmarshal(do(t, f.r, http.MethodGet, "/shopping/items", "").Body.Bytes(), &def)
	assert.Len(t, def.Items, 3)
	for _, it := range def.Items {
		assert.False(t, it.Checked)
	}
	// include_checked appends checked last.
	var inc listResp
	json.Unmarshal(do(t, f.r, http.MethodGet, "/shopping/items?include_checked=true", "").Body.Bytes(), &inc)
	require.Len(t, inc.Items, 5)
	assert.False(t, inc.Items[2].Checked)
	assert.True(t, inc.Items[3].Checked, "checked items come last")
	assert.True(t, inc.Items[4].Checked)
}

func TestCheckUncheckStampsCheckedAt(t *testing.T) {
	f := setup(t)
	var created listResp
	json.Unmarshal(do(t, f.r, http.MethodPost, "/shopping/items", `{"items":[{"name":"milk"}]}`).Body.Bytes(), &created)
	id := created.Items[0].ID.String()

	var checked shoppinglist.Item
	json.Unmarshal(do(t, f.r, http.MethodPatch, "/shopping/items/"+id, `{"checked":true}`).Body.Bytes(), &checked)
	assert.True(t, checked.Checked)
	require.NotNil(t, checked.CheckedAt)

	var unchecked shoppinglist.Item
	json.Unmarshal(do(t, f.r, http.MethodPatch, "/shopping/items/"+id, `{"checked":false}`).Body.Bytes(), &unchecked)
	assert.False(t, unchecked.Checked)
	assert.Nil(t, unchecked.CheckedAt, "checked_at cleared on uncheck")
}

func TestClearCheckedReportsCount(t *testing.T) {
	f := setup(t)
	var created listResp
	json.Unmarshal(do(t, f.r, http.MethodPost, "/shopping/items",
		`{"items":[{"name":"a"},{"name":"b"},{"name":"c"},{"name":"d"},{"name":"e"}]}`).Body.Bytes(), &created)
	for i := 0; i < 4; i++ {
		do(t, f.r, http.MethodPatch, "/shopping/items/"+created.Items[i].ID.String(), `{"checked":true}`)
	}
	w := do(t, f.r, http.MethodDelete, "/shopping/items?checked=true", "")
	require.Equal(t, http.StatusOK, w.Code)
	var dr struct {
		Deleted int `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dr))
	assert.Equal(t, 4, dr.Deleted)
	assert.Equal(t, 1, count(t, f.pool))
}

func TestUnqualifiedBulkDeleteRejected(t *testing.T) {
	f := setup(t)
	do(t, f.r, http.MethodPost, "/shopping/items", `{"items":[{"name":"a"}]}`)
	w := do(t, f.r, http.MethodDelete, "/shopping/items", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "checked_qualifier_required")
	assert.Equal(t, 1, count(t, f.pool), "nothing deleted")
}

func TestProvenanceSetNullOnProductDelete(t *testing.T) {
	f := setup(t)
	pid := makeRecipe(t, f.productsRepo)
	body := fmt.Sprintf(`{"items":[{"name":"Staudensellerie","recipe_product_id":"%s","plan_date":"2026-06-13"}]}`, pid)
	cw := do(t, f.r, http.MethodPost, "/shopping/items", body)
	require.Equal(t, http.StatusCreated, cw.Code, cw.Body.String())
	var created listResp
	json.Unmarshal(cw.Body.Bytes(), &created)
	require.NotNil(t, created.Items[0].RecipeProductID)
	require.NotNil(t, created.Items[0].PlanDate)
	assert.Equal(t, "2026-06-13", *created.Items[0].PlanDate)

	// Delete the product directly → FK ON DELETE SET NULL.
	_, err := f.pool.Exec(context.Background(), "DELETE FROM products WHERE id = $1", pid)
	require.NoError(t, err)

	var after listResp
	json.Unmarshal(do(t, f.r, http.MethodGet, "/shopping/items", "").Body.Bytes(), &after)
	require.Len(t, after.Items, 1, "item survives product deletion")
	assert.Nil(t, after.Items[0].RecipeProductID, "provenance set null")
}

func TestSingleDeleteAndNotFound(t *testing.T) {
	f := setup(t)
	var created listResp
	json.Unmarshal(do(t, f.r, http.MethodPost, "/shopping/items", `{"items":[{"name":"a"}]}`).Body.Bytes(), &created)
	id := created.Items[0].ID.String()
	require.Equal(t, http.StatusNoContent, do(t, f.r, http.MethodDelete, "/shopping/items/"+id, "").Code)
	w := do(t, f.r, http.MethodPatch, "/shopping/items/"+id, `{"checked":true}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "shopping_item_not_found")
}
