package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	_ "github.com/vinzenzs/kazper/docs"
	"github.com/vinzenzs/kazper/internal/httpserver"
)

func TestRegisterSwagger_DebugMode_AlwaysOn(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.DebugMode)
	t.Cleanup(func() { gin.SetMode(prev) })

	r := gin.New()
	httpserver.RegisterSwagger(r, false)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if rr.Code == http.StatusNotFound {
		t.Fatalf("debug mode: /swagger/doc.json should be served, got 404")
	}
}

func TestRegisterSwagger_ReleaseMode_OptIn(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(prev) })

	r := gin.New()
	httpserver.RegisterSwagger(r, true)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if rr.Code == http.StatusNotFound {
		t.Fatalf("release mode with SWAGGER_ENABLED=true: /swagger/doc.json should be served, got 404")
	}
}

func TestRegisterSwagger_ReleaseMode_DefaultOff(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(prev) })

	r := gin.New()
	httpserver.RegisterSwagger(r, false)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("release mode without SWAGGER_ENABLED: /swagger/doc.json should return 404, got %d", rr.Code)
	}
}
