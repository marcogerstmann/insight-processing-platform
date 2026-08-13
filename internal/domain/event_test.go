package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewDomainEventDeterministicID(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	t.Run("same event type and subject id reproduce the same EventID", func(t *testing.T) {
		a := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now, nil)
		b := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now.Add(time.Hour), nil)
		if a.EventID != b.EventID {
			t.Fatalf("EventID = %q, want %q (redelivery must reuse the same id)", b.EventID, a.EventID)
		}
	})

	t.Run("different subject id changes the EventID", func(t *testing.T) {
		a := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now, nil)
		b := NewDomainEvent(InsightCreated, "tenant-1", "insight-2", now, nil)
		if a.EventID == b.EventID {
			t.Fatalf("EventID unexpectedly equal for different subject ids: %q", a.EventID)
		}
	})

	t.Run("different event type changes the EventID", func(t *testing.T) {
		a := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now, nil)
		b := NewDomainEvent(InsightEnriched, "tenant-1", "insight-1", now, nil)
		if a.EventID == b.EventID {
			t.Fatalf("EventID unexpectedly equal for different event types: %q", a.EventID)
		}
	})

	t.Run("version starts at 1", func(t *testing.T) {
		ev := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now, nil)
		if ev.Version != 1 {
			t.Fatalf("Version = %d, want 1", ev.Version)
		}
	})
}

func TestDomainEventJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	type payload struct {
		InsightID string `json:"insight_id"`
	}

	original := NewDomainEvent(InsightCreated, "tenant-1", "insight-1", now, payload{InsightID: "insight-1"})

	first, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DomainEvent
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("JSON round-trip not stable:\n first = %s\nsecond = %s", first, second)
	}

	if decoded.EventID != original.EventID || decoded.EventType != original.EventType ||
		decoded.Version != original.Version || decoded.TenantID != original.TenantID ||
		!decoded.OccurredAt.Equal(original.OccurredAt) {
		t.Fatalf("decoded envelope fields = %+v, want match for %+v", decoded, original)
	}
}

func TestNewInsightEventConstructors(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	insight := Insight{ID: "insight-1", TenantID: "tenant-1", Source: "readwise"}

	t.Run("InsightCreated carries id and source", func(t *testing.T) {
		ev := NewInsightCreatedEvent(insight, now)
		if ev.EventType != InsightCreated || ev.TenantID != "tenant-1" {
			t.Fatalf("got %+v", ev)
		}
		payload, ok := ev.Payload.(InsightCreatedPayload)
		if !ok || payload.InsightID != "insight-1" || payload.Source != "readwise" {
			t.Fatalf("payload = %+v, ok=%v", ev.Payload, ok)
		}
	})

	t.Run("InsightEnriched carries tags, nil enrichment yields none", func(t *testing.T) {
		ev := NewInsightEnrichedEvent(insight, now)
		payload, ok := ev.Payload.(InsightEnrichedPayload)
		if !ok || len(payload.Tags) != 0 {
			t.Fatalf("expected no tags for nil enrichment, got %+v ok=%v", payload, ok)
		}

		enriched := insight
		enriched.Enrichment = &Enrichment{Tags: []string{"stoicism"}}
		ev2 := NewInsightEnrichedEvent(enriched, now)
		payload2 := ev2.Payload.(InsightEnrichedPayload)
		if len(payload2.Tags) != 1 || payload2.Tags[0] != "stoicism" {
			t.Fatalf("expected tags=[stoicism], got %v", payload2.Tags)
		}
	})
}
