package readwise

import (
	"testing"
	"time"
)

func TestMapReadwiseDTOToDomain_HighlightedAt(t *testing.T) {
	receivedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	highlightedAt := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		dto  webhookDTO
		want time.Time
	}{
		"uses highlighted_at when present": {
			dto:  webhookDTO{ID: 1, Text: "hi", EventType: "readwise.highlight.created", HighlightedAt: &highlightedAt, Updated: updated},
			want: highlightedAt,
		},
		"falls back to updated when highlighted_at is missing": {
			dto:  webhookDTO{ID: 1, Text: "hi", EventType: "readwise.highlight.created", Updated: updated},
			want: updated,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ev, err := mapReadwiseDTOToDomain(tc.dto, receivedAt, "tenant-1")
			if err != nil {
				t.Fatalf("mapReadwiseDTOToDomain: %v", err)
			}
			if !ev.Highlight.HighlightedAt.Equal(tc.want) {
				t.Fatalf("HighlightedAt = %v, want %v", ev.Highlight.HighlightedAt, tc.want)
			}
		})
	}
}
