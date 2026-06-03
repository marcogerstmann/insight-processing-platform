package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/rest/insight"
)

func NewRouter(insightHandler *insight.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/tenants/:tenantID")
	{
		v1.GET("/insights", insightHandler.ListByTenantID)
	}

	return r
}
