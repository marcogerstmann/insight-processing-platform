package domain

import (
	"errors"
	"strings"
	"time"
)

// PlanStatus tracks a WeeklyPlan through its async lifecycle. Submission
// (PLAN 1/IPP-103) only ever writes PlanStatusPending; later stories move it
// forward once the LLM planning work completes.
type PlanStatus string

const PlanStatusPending PlanStatus = "pending"

// maxFocusSentenceLength keeps the focus submission to one sentence, not an
// essay, per IPP-103's acceptance criteria.
const maxFocusSentenceLength = 280

var (
	ErrEmptyFocusSentence   = errors.New("focus sentence is required")
	ErrFocusSentenceTooLong = errors.New("focus sentence too long")
)

// WeeklyPlan is a user's request to plan their week from a tag and a focus
// sentence (PLAN 1/IPP-103). Persisted with Status = pending; a later story
// picks it up from the WeeklyPlanRequested event and fills in the plan.
type WeeklyPlan struct {
	ID            string
	TenantID      string
	Tag           string
	FocusSentence string
	Status        PlanStatus
	CreatedAt     time.Time
}

// Validate checks the fields that don't require a database round trip: the
// focus sentence's presence and length. Whether Tag actually exists for the
// tenant is WeeklyPlanRepository.Create's job — only it can check that.
func (p WeeklyPlan) Validate() error {
	if strings.TrimSpace(p.FocusSentence) == "" {
		return ErrEmptyFocusSentence
	}
	if len(p.FocusSentence) > maxFocusSentenceLength {
		return ErrFocusSentenceTooLong
	}
	return nil
}
