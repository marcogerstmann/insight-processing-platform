package domain

import (
	"errors"
	"time"
)

// RelationType is IPP-99's fixed enum, kept in sync with the AI service's
// RelationType (services/ai/src/ipp_ai/domain/relationship.py).
type RelationType string

const (
	RelationSupports    RelationType = "supports"
	RelationContradicts RelationType = "contradicts"
	RelationExtends     RelationType = "extends"
	RelationExampleOf   RelationType = "example_of"
	RelationSameTopic   RelationType = "same_topic"
)

func (t RelationType) Valid() bool {
	switch t {
	case RelationSupports, RelationContradicts, RelationExtends, RelationExampleOf, RelationSameTopic:
		return true
	}
	return false
}

var (
	ErrSelfLink             = errors.New("relationship cannot link an insight to itself")
	ErrUnknownRelationType  = errors.New("unknown relation type")
	ErrConfidenceOutOfRange = errors.New("confidence must be within [0,1]")
)

// Relationship is a discovered edge between two insights within a tenant —
// the persisted counterpart to the AI service's in-memory judgement
// (REL 3/IPP-99). REL 4/IPP-100 is what turns it into a stored,
// bidirectional edge (see ports.RelationshipRepository).
type Relationship struct {
	TenantID      string
	FromInsightID string
	ToInsightID   string
	Type          RelationType
	Confidence    float64
	Rationale     string
	DiscoveredAt  time.Time
}

// Validate checks the fields that don't require a database round trip:
// self-links, the relation type enum, and the confidence range. Whether
// FromInsightID/ToInsightID actually exist in the tenant is
// RelationshipRepository.Put's job — only it can check that.
func (r Relationship) Validate() error {
	if r.FromInsightID == r.ToInsightID {
		return ErrSelfLink
	}
	if !r.Type.Valid() {
		return ErrUnknownRelationType
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return ErrConfidenceOutOfRange
	}
	return nil
}
