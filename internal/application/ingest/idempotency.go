package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func buildIdempotencyKey(ev domain.IngestEvent) string {
	u := ev.Highlight.UpdatedAt.UTC().Format(time.RFC3339)
	h := fmt.Sprintf("%s|%s|%s|%d|%s",
		ev.TenantID,
		ev.Source,
		ev.EventType,
		ev.Highlight.ID,
		u,
	)

	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:])
}
