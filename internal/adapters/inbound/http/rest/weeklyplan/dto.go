package weeklyplan

import "time"

type CreateWeeklyPlanRequestDTO struct {
	Tag           string `json:"tag"`
	FocusSentence string `json:"focus_sentence"`
}

type ResponseDTO struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ActionDTO struct {
	Title                string   `json:"title"`
	Why                  string   `json:"why"`
	SupportingInsightIDs []string `json:"supporting_insight_ids"`
}

// SubmitPlanResultRequestDTO is PUT .../result's body (agent scope, PLAN
// 4/IPP-106): the planning worker's outcome for one plan. Actions is
// meaningful only when Status is "ready"; FailureReason only when "failed"
// — the same pending -> {ready, failed} fork PLAN 3's generation step itself
// ends in.
type SubmitPlanResultRequestDTO struct {
	Status        string      `json:"status"`
	Actions       []ActionDTO `json:"actions"`
	FailureReason string      `json:"failure_reason"`
}

type ResolvedInsightDTO struct {
	InsightID string `json:"insight_id"`
	Text      string `json:"text"`
}

type ResolvedActionDTO struct {
	Title              string               `json:"title"`
	Why                string               `json:"why"`
	SupportingInsights []ResolvedInsightDTO `json:"supporting_insights"`
}

type PlanDetailDTO struct {
	ID            string              `json:"id"`
	Tag           string              `json:"tag"`
	FocusSentence string              `json:"focus_sentence"`
	Status        string              `json:"status"`
	CreatedAt     time.Time           `json:"created_at"`
	FailureReason string              `json:"failure_reason,omitempty"`
	Actions       []ResolvedActionDTO `json:"actions"`
}

type PlanListItemDTO struct {
	ID            string    `json:"id"`
	Tag           string    `json:"tag"`
	FocusSentence string    `json:"focus_sentence"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type ListPlansResponseDTO struct {
	Items []PlanListItemDTO `json:"items"`
}
