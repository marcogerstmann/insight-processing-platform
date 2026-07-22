package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/insight"
)

func NewRouter(insightHandler *insight.Handler, authValidator *auth.CognitoValidator) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/v1/tenants/:tenantID")
	v1.Use(authValidator.Middleware())
	{
		v1.GET("/insights", insightHandler.ListByTenantID)
		v1.POST("/insights", insightHandler.Create)
	}

	return r
}
