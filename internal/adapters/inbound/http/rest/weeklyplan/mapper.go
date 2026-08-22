package weeklyplan

import (
	"time"

	"github.com/google/uuid"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func newID() string {
	return uuid.New().String()
}

func mapCreateRequestToDomain(tenantID string, req CreateWeeklyPlanRequestDTO) domain.WeeklyPlan {
	return domain.WeeklyPlan{
		ID:            newID(),
		TenantID:      tenantID,
		Tag:           req.Tag,
		FocusSentence: req.FocusSentence,
		Status:        domain.PlanStatusPending,
		CreatedAt:     time.Now().UTC(),
	}
}

func mapPlanToDTO(p domain.WeeklyPlan) ResponseDTO {
	return ResponseDTO{ID: p.ID, Status: string(p.Status)}
}
