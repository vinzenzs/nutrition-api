package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// engineWithHealthz mirrors what Run() registers around the framework defaults:
// the JSON NoRoute / NoMethod responders plus the healthz endpoint that we
// reuse below to drive the "wrong method on a known route" case.
func engineWithHealthz() *gin.Engine {
	r := BuildEngine()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	return r
}

func TestEngine_NoRouteReturnsJSON404(t *testing.T) {
	r := engineWithHealthz()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/this/does/not/exist", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"not_found"}`, rec.Body.String())
}

func TestEngine_NoMethodReturnsJSON405(t *testing.T) {
	r := engineWithHealthz()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/healthz", nil))

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error":"method_not_allowed"}`, rec.Body.String())
}
