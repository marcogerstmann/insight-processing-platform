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

func mapActionDTOsToDomain(dtos []ActionDTO) []domain.Action {
	actions := make([]domain.Action, len(dtos))
	for i, dto := range dtos {
		actions[i] = domain.Action{
			Title:                dto.Title,
			Why:                  dto.Why,
			SupportingInsightIDs: dto.SupportingInsightIDs,
		}
	}
	return actions
}

func mapPlanDetailToDTO(detail domain.PlanDetail) PlanDetailDTO {
	actions := make([]ResolvedActionDTO, len(detail.Actions))
	for i, a := range detail.Actions {
		supporting := make([]ResolvedInsightDTO, len(a.SupportingInsights))
		for j, s := range a.SupportingInsights {
			supporting[j] = ResolvedInsightDTO{InsightID: s.InsightID, Text: s.Text}
		}
		actions[i] = ResolvedActionDTO{Title: a.Title, Why: a.Why, SupportingInsights: supporting}
	}

	return PlanDetailDTO{
		ID:            detail.Plan.ID,
		Tag:           detail.Plan.Tag,
		FocusSentence: detail.Plan.FocusSentence,
		Status:        string(detail.Plan.Status),
		CreatedAt:     detail.Plan.CreatedAt,
		FailureReason: detail.Plan.FailureReason,
		Actions:       actions,
	}
}

func mapPlansToListDTO(plans []domain.WeeklyPlan) ListPlansResponseDTO {
	items := make([]PlanListItemDTO, len(plans))
	for i, p := range plans {
		items[i] = PlanListItemDTO{
			ID:            p.ID,
			Tag:           p.Tag,
			FocusSentence: p.FocusSentence,
			Status:        string(p.Status),
			CreatedAt:     p.CreatedAt,
		}
	}
	return ListPlansResponseDTO{Items: items}
}
