package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
	restraindrop "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/raindrop"
	restreadwise "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/readwise"
	restrelationship "github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/relationship"
)

// NewRouter builds the REST engine. allowedOrigins enables browser CORS for
// those origins; pass nil in environments where CORS is handled upstream (AWS
// API Gateway), and the Vite dev origin from the local runner.
func NewRouter(insightHandler *insight.Handler, readwiseHandler *restreadwise.Handler, raindropHandler *restraindrop.Handler, relationshipHandler *restrelationship.Handler, authValidator *auth.CognitoValidator, allowedOrigins []string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Registered globally (before the route group's auth middleware) so it can
	// answer token-less preflight OPTIONS requests. Only wired when origins are
	// supplied — see corsMiddleware's doc comment.
	if len(allowedOrigins) > 0 {
		r.Use(corsMiddleware(allowedOrigins))
	}

	v1 := r.Group("/v1")
	v1.Use(authValidator.Middleware())
	{
		// auth.RequireUser() per route, not on the group: the agent-only
		// route below needs auth.RequireScope(...) instead — group-level
		// RequireUser would 403 the AI service's own machine token before
		// RequireScope ever ran.
		v1.GET("/insights", auth.RequireUser(), insightHandler.ListByTenantID)
		v1.POST("/insights", auth.RequireUser(), insightHandler.Create)
		v1.GET("/tags", auth.RequireUser(), insightHandler.ListTags)
		v1.POST("/readwise/import", auth.RequireUser(), readwiseHandler.Import)
		v1.POST("/raindrop/import", auth.RequireUser(), raindropHandler.Import)

		// Agent-only (REL 4, IPP-100): tenant and from-insight come from the
		// URL, not a JWT claim — the AI service's machine token has no
		// tenant of its own (see auth.CognitoValidator.Middleware).
		v1.POST("/tenants/:tenantID/insights/:insightID/relationships", auth.RequireScope(auth.ScopeAgentWrite), relationshipHandler.Create)
	}

	return r
}
