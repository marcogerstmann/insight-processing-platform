package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// EventType names a domain fact, not a queue or transport detail.
type EventType string

const (
	InsightCreated      EventType = "InsightCreated"
	InsightEnriched     EventType = "InsightEnriched"
	KnowledgeUpdated    EventType = "KnowledgeUpdated"
	WeeklyPlanRequested EventType = "WeeklyPlanRequested"
)

// domainEventVersion is the envelope schema version. It starts at 1; bump it
// only when the envelope shape itself changes, not per event type.
const domainEventVersion = 1

// DomainEvent is the versioned envelope for "something happened" in the
// domain. It carries no AWS/transport concepts — adapters translate it to
// whatever wire format a subscriber needs.
type DomainEvent struct {
	EventID    string    `json:"event_id"`
	EventType  EventType `json:"event_type"`
	Version    int       `json:"version"`
	TenantID   string    `json:"tenant_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    any       `json:"payload"`
}

// NewDomainEvent builds an envelope with a deterministic EventID derived
// from (eventType, subjectID), so a redelivered SQS message re-publishes an
// event with the same id and subscribers can dedupe on it.
func NewDomainEvent(eventType EventType, tenantID, subjectID string, occurredAt time.Time, payload any) DomainEvent {
	return DomainEvent{
		EventID:    deterministicEventID(eventType, subjectID),
		EventType:  eventType,
		Version:    domainEventVersion,
		TenantID:   tenantID,
		OccurredAt: occurredAt,
		Payload:    payload,
	}
}

func deterministicEventID(eventType EventType, subjectID string) string {
	sum := sha256.Sum256([]byte(string(eventType) + "|" + subjectID))
	return hex.EncodeToString(sum[:])
}

// InsightCreatedPayload is the InsightCreated event's payload. Kept small —
// subscribers read the full record themselves if they need more than this
// notification carries; the envelope already carries TenantID/OccurredAt.
type InsightCreatedPayload struct {
	InsightID string `json:"insight_id"`
	Source    string `json:"source"`
}

// InsightEnrichedPayload is the InsightEnriched event's payload.
type InsightEnrichedPayload struct {
	InsightID string   `json:"insight_id"`
	Tags      []string `json:"tags"`
}

// NewInsightCreatedEvent builds the envelope published right after an
// insight is durably written for the first time.
func NewInsightCreatedEvent(insight Insight, occurredAt time.Time) DomainEvent {
	return NewDomainEvent(InsightCreated, insight.TenantID, insight.ID, occurredAt, InsightCreatedPayload{
		InsightID: insight.ID,
		Source:    insight.Source,
	})
}

// NewInsightEnrichedEvent builds the envelope published right after an
// insight's enrichment is durably written.
func NewInsightEnrichedEvent(insight Insight, occurredAt time.Time) DomainEvent {
	var tags []string
	if insight.Enrichment != nil {
		tags = insight.Enrichment.Tags
	}
	return NewDomainEvent(InsightEnriched, insight.TenantID, insight.ID, occurredAt, InsightEnrichedPayload{
		InsightID: insight.ID,
		Tags:      tags,
	})
}

// KnowledgeUpdatedPayload is the KnowledgeUpdated event's payload (REL
// 5/IPP-101): the pair of insights a newly persisted relationship connects.
type KnowledgeUpdatedPayload struct {
	FromInsightID string `json:"from_insight_id"`
	ToInsightID   string `json:"to_insight_id"`
}

// NewKnowledgeUpdatedEvent builds the envelope published right after a
// relationship edge is durably written (RelationshipRepository.Put). The
// subject ID is the edge itself, not either insight alone, so re-posting
// the same edge (Put is idempotent) redelivers the same deterministic
// event ID rather than a fresh one each time.
func NewKnowledgeUpdatedEvent(rel Relationship, occurredAt time.Time) DomainEvent {
	return NewDomainEvent(KnowledgeUpdated, rel.TenantID, rel.FromInsightID+"|"+rel.ToInsightID, occurredAt, KnowledgeUpdatedPayload{
		FromInsightID: rel.FromInsightID,
		ToInsightID:   rel.ToInsightID,
	})
}

// WeeklyPlanRequestedPayload is the WeeklyPlanRequested event's payload
// (PLAN 1/IPP-103): what the async planning worker needs to start, without
// re-reading the plan row.
type WeeklyPlanRequestedPayload struct {
	PlanID        string `json:"plan_id"`
	Tag           string `json:"tag"`
	FocusSentence string `json:"focus_sentence"`
}

// NewWeeklyPlanRequestedEvent builds the envelope published right after a
// weekly plan is durably written. The subject ID is the plan's own id, since
// (unlike relationship edges) a plan submission has no natural composite key
// to dedupe on.
func NewWeeklyPlanRequestedEvent(plan WeeklyPlan, occurredAt time.Time) DomainEvent {
	return NewDomainEvent(WeeklyPlanRequested, plan.TenantID, plan.ID, occurredAt, WeeklyPlanRequestedPayload{
		PlanID:        plan.ID,
		Tag:           plan.Tag,
		FocusSentence: plan.FocusSentence,
	})
}
