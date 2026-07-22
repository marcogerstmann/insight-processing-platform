package insight

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	appinsight "github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
)

type Handler struct {
	svc appinsight.Service
}

func NewHandler(svc appinsight.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListByTenantID(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)

	insights, err := h.svc.ListByTenantID(c.Request.Context(), tenantID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to list insights", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapInsightsToDTO(tenantID, insights))
}

func (h *Handler) Create(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)

	var req CreateInsightRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	insight := mapCreateRequestToDomain(tenantID, req)
	res, err := h.svc.Process(c.Request.Context(), insight)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to create insight", "tenant_id", tenantID, "insight_id", insight.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	status := http.StatusOK
	if res.Inserted {
		status = http.StatusCreated
	}
	c.JSON(status, CreateInsightResponseDTO{
		Inserted: res.Inserted,
		Insight:  mapInsightToDTO(insight),
	})
}
