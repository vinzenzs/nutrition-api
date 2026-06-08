package httpserver

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterSwagger mounts the Swagger UI at /swagger/*any when documentation
// is enabled. Documentation is enabled when explicitly opted in via
// `enabled=true` (the SWAGGER_ENABLED env var) OR when Gin is not in release
// mode (so local development gets it for free).
func RegisterSwagger(r *gin.Engine, enabled bool) {
	if !ShouldEnableSwagger(enabled) {
		return
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// ShouldEnableSwagger reports whether the Swagger UI should be registered
// given the explicit opt-in flag and the current Gin mode.
func ShouldEnableSwagger(enabled bool) bool {
	return enabled || gin.Mode() != gin.ReleaseMode
}
