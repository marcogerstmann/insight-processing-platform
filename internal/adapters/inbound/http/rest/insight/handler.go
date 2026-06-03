package insight

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports/outbound"
)

type Handler struct {
	repo outbound.InsightRepository
	log  *slog.Logger
}

func NewHandler(repo outbound.InsightRepository, log *slog.Logger) *Handler {
	return &Handler{repo: repo, log: log}
}

func (h *Handler) ListByTenantID(c *gin.Context) {
	tenantID := strings.TrimSpace(c.Param("tenantID"))
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantID is required"})
		return
	}

	insights, err := h.repo.ListByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "failed to list insights", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapInsightsToDTO(tenantID, insights))
}
