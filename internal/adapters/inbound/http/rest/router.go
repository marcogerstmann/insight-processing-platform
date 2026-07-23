package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
)

// NewRouter builds the REST engine. allowedOrigins enables browser CORS for
// those origins; pass nil in environments where CORS is handled upstream (AWS
// API Gateway), and the Vite dev origin from the local runner.
func NewRouter(insightHandler *insight.Handler, authValidator *auth.CognitoValidator, allowedOrigins []string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Registered globally (before the route group's auth middleware) so it can
	// answer token-less preflight OPTIONS requests. Only wired when origins are
	// supplied — see corsMiddleware's doc comment.
	if len(allowedOrigins) > 0 {
		r.Use(corsMiddleware(allowedOrigins))
	}

	v1 := r.Group("/v1/tenants/:tenantID")
	v1.Use(authValidator.Middleware())
	{
		v1.GET("/insights", insightHandler.ListByTenantID)
		v1.POST("/insights", insightHandler.Create)
	}

	return r
}
