package devices_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/devices"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := devices.NewService(devices.NewRepo(pool))
	r := gin.New()
	devices.NewHandlers(svc).Register(r.Group("/"))
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

func decode(t *testing.T, b []byte) devices.Device {
	t.Helper()
	var d devices.Device
	require.NoError(t, json.Unmarshal(b, &d))
	return d
}

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/devices",
		`{"external_id":"dev-1","display_name":"Fenix 7","model":"fenix7","battery_pct":86.0}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	d := decode(t, rec.Body.Bytes())
	require.NotNil(t, d.BatteryPct)
	assert.InDelta(t, 86.0, *d.BatteryPct, 0.001)

	rec2 := do(t, r, http.MethodPost, "/devices", `{"external_id":"dev-1","display_name":"Fenix 7","battery_pct":41.0}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	d2 := decode(t, rec2.Body.Bytes())
	assert.Equal(t, d.ID, d2.ID)
	require.NotNil(t, d2.BatteryPct)
	assert.InDelta(t, 41.0, *d2.BatteryPct, 0.001)
	assert.Nil(t, d2.Model, "full-replace nulls omitted fields")

	list := do(t, r, http.MethodGet, "/devices", "")
	var out struct {
		Devices []devices.Device `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	assert.Len(t, out.Devices, 1)
}

func TestUpsert_Validation(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"display_name":"x"}`: "external_id_required",
		`{"external_id":"d"}`:  "display_name_required",
		`{"external_id":"d","display_name":"x","battery_pct":150}`: "battery_pct_invalid",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/devices", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String(), body)
	}
}

func TestList_OrderedAndGet404(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/devices", `{"external_id":"d1","display_name":"Zephyr"}`).Code)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/devices", `{"external_id":"d2","display_name":"Alpha"}`).Code)
	list := do(t, r, http.MethodGet, "/devices", "")
	var out struct {
		Devices []devices.Device `json:"devices"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	require.Len(t, out.Devices, 2)
	assert.Equal(t, "Alpha", out.Devices[0].DisplayName, "ordered by display_name asc")

	miss := do(t, r, http.MethodGet, "/devices/11111111-1111-1111-1111-111111111111", "")
	require.Equal(t, http.StatusNotFound, miss.Code)
	assert.JSONEq(t, `{"error":"device_not_found"}`, miss.Body.String())
}

func TestUnitIsolationAndRounding(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/devices", `{"external_id":"d","display_name":"x","battery_pct":86.44}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "kcal")
	assert.NotContains(t, body, "total_ml")
	d := decode(t, rec.Body.Bytes())
	require.NotNil(t, d.BatteryPct)
	assert.InDelta(t, 86.4, *d.BatteryPct, 0.001, "battery rounded at the boundary")
}
