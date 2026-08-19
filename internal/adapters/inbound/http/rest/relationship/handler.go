package relationship

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Handler struct {
	repo ports.RelationshipRepository
}

func NewHandler(repo ports.RelationshipRepository) *Handler {
	return &Handler{repo: repo}
}

// Create is an agent-only route (see router.go's RequireScope wiring): the
// tenant and from-insight come from the URL, not a JWT claim, because the
// AI service's machine token carries no tenant of its own.
func (h *Handler) Create(c *gin.Context) {
	tenantID := c.Param("tenantID")
	fromInsightID := c.Param("insightID")

	var req CreateRelationshipRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
		return
	}

	rel := mapCreateRequestToDomain(tenantID, fromInsightID, req)
	if err := rel.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Put(c.Request.Context(), rel); err != nil {
		if errors.Is(err, ports.ErrInsightNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown insight"})
			return
		}
		slog.ErrorContext(c.Request.Context(), "failed to persist relationship",
			"tenant_id", tenantID, "from_insight_id", fromInsightID, "to_insight_id", req.ToInsightID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapRelationshipToDTO(rel))
}

// ListByInsightID is a user route (see router.go): the tenant comes from
// the JWT (auth.TenantIDKey), same as every other user route — the
// :tenantID path segment exists only to mirror Create's URL shape and is
// never trusted for scoping.
func (h *Handler) ListByInsightID(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)
	insightID := c.Param("insightID")

	related, err := h.repo.ListByInsightID(c.Request.Context(), tenantID, insightID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to list relationships", "tenant_id", tenantID, "insight_id", insightID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapRelatedInsightsToDTO(insightID, related))
}
