package personalrecords_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/personalrecords"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := personalrecords.NewService(personalrecords.NewRepo(pool))
	r := gin.New()
	personalrecords.NewHandlers(svc).Register(r.Group("/"))
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

func decode(t *testing.T, b []byte) personalrecords.PersonalRecord {
	t.Helper()
	var pr personalrecords.PersonalRecord
	require.NoError(t, json.Unmarshal(b, &pr))
	return pr
}

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/personal-records",
		`{"external_id":"pr-1","pr_type":"5k","value":1320,"unit":"s","activity_id":"act-987","achieved_at":"2026-05-20T08:00:00Z"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	pr := decode(t, rec.Body.Bytes())
	require.NotNil(t, pr.Value)
	assert.InDelta(t, 1320, *pr.Value, 0.001)
	require.NotNil(t, pr.ActivityID)
	assert.Equal(t, "act-987", *pr.ActivityID)

	// Beaten PR → 200, updated in place.
	rec2 := do(t, r, http.MethodPost, "/personal-records",
		`{"external_id":"pr-1","pr_type":"5k","value":1295,"unit":"s","achieved_at":"2026-06-01T08:00:00Z"}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	pr2 := decode(t, rec2.Body.Bytes())
	assert.Equal(t, pr.ID, pr2.ID, "same backend id on update")
	require.NotNil(t, pr2.Value)
	assert.InDelta(t, 1295, *pr2.Value, 0.001)
	assert.Nil(t, pr2.ActivityID, "full-replace nulls omitted activity_id")

	list := do(t, r, http.MethodGet, "/personal-records", "")
	var out struct {
		PersonalRecords []personalrecords.PersonalRecord `json:"personal_records"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &out))
	assert.Len(t, out.PersonalRecords, 1)
}

func TestUpsert_ValidationErrors(t *testing.T) {
	r := setup(t)
	cases := map[string]string{
		`{"pr_type":"5k","value":1,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`:                    "external_id_required",
		`{"external_id":"p","value":1,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`:                 "pr_type_required",
		`{"external_id":"p","pr_type":"5k","unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`:            "value_invalid",
		`{"external_id":"p","pr_type":"5k","value":-1,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`: "value_invalid",
		`{"external_id":"p","pr_type":"5k","value":1,"achieved_at":"2026-05-20T08:00:00Z"}`:             "unit_required",
		`{"external_id":"p","pr_type":"5k","value":1,"unit":"s"}`:                                       "achieved_at_required",
	}
	for body, code := range cases {
		rec := do(t, r, http.MethodPost, "/personal-records", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"`+code+`"}`, rec.Body.String(), body)
	}
}

func TestList_TypeFilterAndOrder(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/personal-records",
		`{"external_id":"p1","pr_type":"5k","value":1320,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`).Code)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/personal-records",
		`{"external_id":"p2","pr_type":"10k","value":2790,"unit":"s","achieved_at":"2026-06-01T08:00:00Z"}`).Code)

	all := do(t, r, http.MethodGet, "/personal-records", "")
	var out struct {
		PersonalRecords []personalrecords.PersonalRecord `json:"personal_records"`
	}
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &out))
	require.Len(t, out.PersonalRecords, 2)
	assert.Equal(t, "p2", out.PersonalRecords[0].ExternalID, "ordered by achieved_at desc")

	filtered := do(t, r, http.MethodGet, "/personal-records?pr_type=5k", "")
	require.NoError(t, json.Unmarshal(filtered.Body.Bytes(), &out))
	require.Len(t, out.PersonalRecords, 1)
	assert.Equal(t, "p1", out.PersonalRecords[0].ExternalID)
}

func TestUpsert_ActivityIDOmittedWhenAbsent(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/personal-records",
		`{"external_id":"p","pr_type":"5k","value":1320,"unit":"s","achieved_at":"2026-05-20T08:00:00Z"}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), "activity_id")
}
