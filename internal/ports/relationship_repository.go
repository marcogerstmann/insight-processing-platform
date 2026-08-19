package ports

import (
	"context"
	"errors"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

// ErrInsightNotFound is returned by RelationshipRepository.Put when either
// side of the edge doesn't exist in rel.TenantID's partition.
var ErrInsightNotFound = errors.New("insight not found")

type RelationshipRepository interface {
	// Put stores rel as a bidirectional edge, upserting on
	// (FromInsightID, ToInsightID) so re-posting the same edge updates it
	// rather than duplicating.
	Put(ctx context.Context, rel domain.Relationship) error

	// ListByInsightID returns insightID's edges, sorted by confidence
	// descending, regardless of which side they were originally
	// discovered from.
	ListByInsightID(ctx context.Context, tenantID, insightID string) ([]domain.RelatedInsight, error)
}
