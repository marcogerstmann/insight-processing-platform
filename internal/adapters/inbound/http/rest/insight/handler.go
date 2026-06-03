package insight

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	appinsight "github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
)

type Handler struct {
	svc appinsight.Service
	log *slog.Logger
}

func NewHandler(svc appinsight.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) ListByTenantID(c *gin.Context) {
	tenantID := strings.TrimSpace(c.Param("tenantID"))
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantID is required"})
		return
	}

	insights, err := h.svc.ListByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "failed to list insights", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapInsightsToDTO(tenantID, insights))
}
