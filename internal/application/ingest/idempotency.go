package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

func buildIdempotencyKey(ev domain.IngestEvent) string {
	h := fmt.Sprintf("%s|%s|%s|%s",
		ev.TenantID,
		ev.Source,
		ev.EventType,
		ev.Highlight.ID,
	)

	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:])
}
