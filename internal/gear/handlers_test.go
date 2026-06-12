package gear_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/gear"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := gear.NewService(gear.NewRepo(pool))
	r := gin.New()
	gear.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, b []byte) gear.Gear {
	t.Helper()
	var g gear.Gear
	require.NoError(t, json.Unmarshal(b, &g))
	return g
}

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/gear",
		`{"external_id":"gear-abc","gear_type":"shoes","display_name":"Daily Trainers","total_distance_m":780000,"total_activities":120}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	g := decode(t, rec.Body.Bytes())
	require.NotEqual(t, "00000000-0000-0000-0000-000000000000", g.ID.String())
	assert.Equal(t, gear.TypeShoes, g.GearType)
	require.NotNil(t, g.TotalDistanceM)
	assert.InDelta(t, 780000, *g.TotalDistanceM, 0.5)
	assert.False(t, g.Retired)

	// Re-upsert same external_id → 200, updated in place (new distance + retired).
	rec2 := do(t, r, http.MethodPost, "/gear",
		`{"external_id":"gear-abc","gear_type":"shoes","display_name":"Daily Trainers","total_distance_m":812000,"retired":true}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	g2 := decode(t, rec2.Body.Bytes())
	assert.Equal(t, g.ID, g2.ID, "same backend id on update")
	assert.True(t, g2.Retired)
	assert.Nil(t, g2.TotalActivities, "full-replace nulls omitted fields")

	// Exactly one row.
	list := do(t, r, http.MethodGet, "/gear", "")
	var out struct {
		Gear []gear.Gear `json:"gear"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	assert.Len(t, out.Gear, 1)
}

func TestUpsert_ValidationErrors(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"gear_type":"shoes","display_name":"x"}`:                                         "external_id_required",
		`{"external_id":"g","gear_type":"kayak","display_name":"x"}`:                       "gear_type_invalid",
		`{"external_id":"g","gear_type":"shoes"}`:                                          "display_name_required",
		`{"external_id":"g","gear_type":"shoes","display_name":"x","total_distance_m":-1}`: "total_distance_m_invalid",
		`{"external_id":"g","gear_type":"shoes","display_name":"x","total_activities":-1}`: "total_activities_invalid",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/gear", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String(), body)
	}
}

func TestList_RetiredFilterAndOrder(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/gear",
		`{"external_id":"g1","gear_type":"shoes","display_name":"Zephyr","retired":true}`).Code)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/gear",
		`{"external_id":"g2","gear_type":"bike","display_name":"Alpha","retired":false}`).Code)

	all := do(t, r, http.MethodGet, "/gear", "")
	var out struct {
		Gear []gear.Gear `json:"gear"`
	}
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &out))
	require.Len(t, out.Gear, 2)
	assert.Equal(t, "Alpha", out.Gear[0].DisplayName, "ordered by display_name asc")

	retired := do(t, r, http.MethodGet, "/gear?retired=true", "")
	require.NoError(t, json.Unmarshal(retired.Body.Bytes(), &out))
	require.Len(t, out.Gear, 1)
	assert.Equal(t, "Zephyr", out.Gear[0].DisplayName)
}

func TestGet_RoundTripAndNotFound(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/gear",
		`{"external_id":"g","gear_type":"shoes","display_name":"x","total_distance_m":812345.67}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	g := decode(t, rec.Body.Bytes())

	got := do(t, r, http.MethodGet, "/gear/"+g.ID.String(), "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	gg := decode(t, got.Body.Bytes())
	require.NotNil(t, gg.TotalDistanceM)
	assert.InDelta(t, 812345.7, *gg.TotalDistanceM, 0.0001, "distance rounded at the boundary")

	miss := do(t, r, http.MethodGet, "/gear/11111111-1111-1111-1111-111111111111", "")
	require.Equal(t, http.StatusNotFound, miss.Code)
	assert.JSONEq(t, `{"error":"gear_not_found"}`, miss.Body.String())
}

func TestGet_NullableFieldsOmitted(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/gear",
		`{"external_id":"g","gear_type":"shoes","display_name":"x"}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "total_distance_m")
	assert.NotContains(t, body, "total_activities")
	assert.NotContains(t, body, "date_begin")
}
