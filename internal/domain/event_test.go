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
