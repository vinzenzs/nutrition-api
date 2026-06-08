package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientID is one of the resolved client identities set on the request context.
type ClientID string

const (
	ClientMobile ClientID = "mobile"
	ClientAgent  ClientID = "agent"
)

const clientContextKey = "auth.client_id"

// Middleware returns a Gin middleware that requires Authorization: Bearer <token>
// matching one of the two configured tokens. On success the resolved client id
// is stored on the request context; on failure the request is aborted with 401.
func Middleware(cfg Config) gin.HandlerFunc {
	mobile := []byte(cfg.MobileToken)
	agent := []byte(cfg.AgentToken)

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abort(c, http.StatusUnauthorized, "auth_required")
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			abort(c, http.StatusUnauthorized, "auth_required")
			return
		}
		token := []byte(strings.TrimPrefix(header, prefix))

		switch {
		case subtle.ConstantTimeCompare(token, mobile) == 1:
			c.Set(clientContextKey, ClientMobile)
		case subtle.ConstantTimeCompare(token, agent) == 1:
			c.Set(clientContextKey, ClientAgent)
		default:
			abort(c, http.StatusUnauthorized, "auth_invalid")
			return
		}
		c.Next()
	}
}

// ClientFromContext returns the client id set by the auth middleware, or empty
// string if the request has not been authenticated.
func ClientFromContext(c *gin.Context) ClientID {
	v, ok := c.Get(clientContextKey)
	if !ok {
		return ""
	}
	id, _ := v.(ClientID)
	return id
}

func abort(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}
