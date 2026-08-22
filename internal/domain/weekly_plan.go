package domain

import (
	"errors"
	"strings"
	"time"
)

// PlanStatus tracks a WeeklyPlan through its async lifecycle. Submission
// (PLAN 1/IPP-103) only ever writes PlanStatusPending; PLAN 4 (IPP-106) is
// what moves it to PlanStatusReady or PlanStatusFailed once the planning
// worker reports back.
type PlanStatus string

const (
	PlanStatusPending PlanStatus = "pending"
	PlanStatusReady   PlanStatus = "ready"
	PlanStatusFailed  PlanStatus = "failed"
)

// maxFocusSentenceLength keeps the focus submission to one sentence, not an
// essay, per IPP-103's acceptance criteria.
const maxFocusSentenceLength = 280

var (
	ErrEmptyFocusSentence   = errors.New("focus sentence is required")
	ErrFocusSentenceTooLong = errors.New("focus sentence too long")
)

// WeeklyPlan is a user's request to plan their week from a tag and a focus
// sentence (PLAN 1/IPP-103). Persisted with Status = pending; PLAN 4
// (IPP-106) fills in Actions or FailureReason once the planning worker
// reports back via SetReady/SetFailed.
//
// Actions is nested on the plan item rather than stored as separate rows
// (IPP-106's implementation notes): there are at most five, and they are
// never read except together with their plan.
type WeeklyPlan struct {
	ID            string
	TenantID      string
	Tag           string
	FocusSentence string
	Status        PlanStatus
	CreatedAt     time.Time
	Actions       []Action
	FailureReason string
}

// Action is one LLM-drafted, citation-validated step in a ready WeeklyPlan.
// SupportingInsightIDs already passed PLAN 3's hallucination check
// (services/ai/application/action_generation.py) before this ever reaches
// SetReady — this type stores that verified claim, it doesn't re-verify it.
type Action struct {
	Title                string
	Why                  string
	SupportingInsightIDs []string
}

// ResolvedInsight is one of an Action's supporting insights, resolved to a
// short summary for display — analogous to RelatedInsight's denormalized
// Text, but resolved at read time (application/weeklyplan.Service.Get)
// rather than write time: SetReady's caller only ever supplies ids, never
// insight text, so there is no write-time point to denormalize at.
type ResolvedInsight struct {
	InsightID string
	Text      string
}

// ResolvedAction is an Action with its citations resolved to the insights
// they refer to — GET .../weekly-plans/:planID's shape. Never persisted.
type ResolvedAction struct {
	Title              string
	Why                string
	SupportingInsights []ResolvedInsight
}

// PlanDetail is a WeeklyPlan with its actions' citations resolved — the
// read side's answer to "what should I act on, and why" that PLAN 4 exists
// to serve.
type PlanDetail struct {
	Plan    WeeklyPlan
	Actions []ResolvedAction
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
