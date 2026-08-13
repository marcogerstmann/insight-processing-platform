package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// EventType names a domain fact, not a queue or transport detail.
type EventType string

const (
	InsightCreated   EventType = "InsightCreated"
	InsightEnriched  EventType = "InsightEnriched"
	KnowledgeUpdated EventType = "KnowledgeUpdated"
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
