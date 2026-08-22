package weeklyplan

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	appweeklyplan "github.com/marcogerstmann/insight-processing-platform/internal/application/weeklyplan"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

type Handler struct {
	svc appweeklyplan.Service
}

func NewHandler(svc appweeklyplan.Service) *Handler {
	return &Handler{svc: svc}
}

// Create is a user route: the tenant comes from the JWT, the tag comes from
// the request body — same rule as everywhere else (see router.go). The
// :tenantID path segment exists only to match the rest of the API's URL
// shape and is never trusted for scoping.
//
// This is deliberately async (IPP-103's implementation notes): the plan is
// persisted as pending and WeeklyPlanRequested is published for the
// planning worker to pick up later, so the request returns 202 rather than
// blocking on an LLM call.
func (h *Handler) Create(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)

	var req CreateWeeklyPlanRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
		return
	}

	plan := mapCreateRequestToDomain(tenantID, req)
	if err := plan.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Submit(c.Request.Context(), plan); err != nil {
		if errors.Is(err, ports.ErrUnknownTag) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tag"})
			return
		}
		slog.ErrorContext(c.Request.Context(), "failed to submit weekly plan",
			"tenant_id", tenantID, "plan_id", plan.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusAccepted, mapPlanToDTO(plan))
}
