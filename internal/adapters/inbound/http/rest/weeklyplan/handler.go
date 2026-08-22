package weeklyplan

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/marcogerstmann/insight-processing-platform/internal/adapters/inbound/http/rest/auth"
	appweeklyplan "github.com/marcogerstmann/insight-processing-platform/internal/application/weeklyplan"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
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

// SubmitResult is an agent-only route (see router.go): the planning
// worker's own machine token, not a human's, so tenant and plan both come
// from the URL — same trust boundary as relationship.Create's agent route.
//
// Exactly one outcome per call: "ready" persists the drafted actions,
// "failed" persists a reason. Either way the underlying write is
// conditional on the plan still being pending (IPP-106's AC), so a
// redelivered result — or a result for a plan that doesn't exist — is
// rejected rather than silently overwriting an already-resolved plan.
func (h *Handler) SubmitResult(c *gin.Context) {
	tenantID := c.Param("tenantID")
	planID := c.Param("planID")

	var req SubmitPlanResultRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request_body"})
		return
	}

	var err error
	switch domain.PlanStatus(req.Status) {
	case domain.PlanStatusReady:
		err = h.svc.SetReady(c.Request.Context(), tenantID, planID, mapActionDTOsToDomain(req.Actions))
	case domain.PlanStatusFailed:
		if strings.TrimSpace(req.FailureReason) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failure_reason is required"})
			return
		}
		err = h.svc.SetFailed(c.Request.Context(), tenantID, planID, req.FailureReason)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be \"ready\" or \"failed\""})
		return
	}

	if err != nil {
		if errors.Is(err, ports.ErrPlanNotPending) {
			c.JSON(http.StatusConflict, gin.H{"error": "plan not found or already resolved"})
			return
		}
		slog.ErrorContext(c.Request.Context(), "failed to persist weekly plan result",
			"tenant_id", tenantID, "plan_id", planID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	// AbortWithStatus, not Status: Status alone only records the code on
	// c.Writer and relies on the engine's own post-handler flush to send
	// it, which never runs when a handler is invoked directly (as the
	// tests do) rather than through the full router.
	c.AbortWithStatus(http.StatusNoContent)
}

// Get is a user route (see router.go): the tenant comes from the JWT, same
// as every other user route. Returns the plan with its actions' citations
// resolved to the insights they refer to, so the UI can link straight to
// them instead of showing a bare id.
func (h *Handler) Get(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)
	planID := c.Param("planID")

	detail, err := h.svc.Get(c.Request.Context(), tenantID, planID)
	if err != nil {
		if errors.Is(err, ports.ErrPlanNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		slog.ErrorContext(c.Request.Context(), "failed to get weekly plan",
			"tenant_id", tenantID, "plan_id", planID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapPlanDetailToDTO(detail))
}

// List is a user route (see router.go): the tenant's plans, newest first.
func (h *Handler) List(c *gin.Context) {
	tenantID := c.GetString(auth.TenantIDKey)

	plans, err := h.svc.List(c.Request.Context(), tenantID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to list weekly plans", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, mapPlansToListDTO(plans))
}
