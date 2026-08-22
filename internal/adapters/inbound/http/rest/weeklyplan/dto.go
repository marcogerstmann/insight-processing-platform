package weeklyplan

type CreateWeeklyPlanRequestDTO struct {
	Tag           string `json:"tag"`
	FocusSentence string `json:"focus_sentence"`
}

type ResponseDTO struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
