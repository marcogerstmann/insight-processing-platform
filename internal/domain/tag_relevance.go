package domain

import (
	"math"
	"time"
)

const (
	// tagRelevanceCountWeight, tagRelevanceRecencyWeight and
	// tagRelevanceFreshnessWeight split relevance between "how much has this
	// tag been used", "was it just used", and "has its usage as a whole
	// stayed recent rather than being a pile of old insights". They sum to 1
	// so the score itself stays in [0,1]. Relationship density (REL 5) is
	// layered on afterward by TagRelevanceScoreWithDensity rather than
	// joining this weighted sum directly — see tagRelevanceDensityWeight.
	tagRelevanceCountWeight     = 0.3
	tagRelevanceRecencyWeight   = 0.35
	tagRelevanceFreshnessWeight = 0.35

	// tagRelevanceCountSaturation is the insight count at which the count
	// component reaches half of its maximum. Diminishing returns: a tag's
	// 50th insight barely moves the score more than its 5th.
	tagRelevanceCountSaturation = 5.0

	// tagRelevanceHalfLife is how long it takes a single insight's
	// contribution to recency/freshness to decay to half its original
	// weight. Picked to roughly match a weekly tag-cloud cadence: an
	// insight from a week ago should already be fading.
	tagRelevanceHalfLife = 7 * 24 * time.Hour

	// tagRelevanceDensityWeight (REL 5) is how much of the score
	// TagRelevanceScoreWithDensity hands to relationship density, taken as
	// a slice out of TagRelevanceScore's already-normalized [0,1] output
	// rather than a fourth term alongside Count/Recency/Freshness — that
	// keeps TagRelevanceScore itself, and its weights above, untouched.
	tagRelevanceDensityWeight = 0.2

	// tagRelevanceDensitySaturation is the average relationships-per-insight
	// at which the density component reaches half of its maximum. Picked,
	// not measured: two links per insight already reads as "well-connected"
	// for a personal knowledge base's scale.
	tagRelevanceDensitySaturation = 2.0
)

// TagScoreComponents is the normalized (0-1) contribution of each relevance
// signal, exposed alongside the final score so callers can explain why a
// tag ranks where it does. Density is only populated by
// TagRelevanceScoreWithDensity — TagRelevanceScore leaves it at zero.
type TagScoreComponents struct {
	Count     float64
	Recency   float64
	Freshness float64
	Density   float64
}

// TagRelevanceScore ranks a tag by how active and current its usage is, not
// just how many insights carry it. It is a pure function: given the
// creation time of every insight tagged with it and the current time
// (injected, never read from a clock), it deterministically returns a score
// in [0,1] plus the component breakdown that produced it.
func TagRelevanceScore(insightTimestamps []time.Time, now time.Time) (float64, TagScoreComponents) {
	if len(insightTimestamps) == 0 {
		return 0, TagScoreComponents{}
	}

	count := float64(len(insightTimestamps))
	countComponent := count / (count + tagRelevanceCountSaturation)

	mostRecent := insightTimestamps[0]
	var freshnessSum float64
	for _, ts := range insightTimestamps {
		if ts.After(mostRecent) {
			mostRecent = ts
		}
		freshnessSum += tagRelevanceDecay(now.Sub(ts))
	}

	components := TagScoreComponents{
		Count:     countComponent,
		Recency:   tagRelevanceDecay(now.Sub(mostRecent)),
		Freshness: freshnessSum / count,
	}
	score := tagRelevanceCountWeight*components.Count +
		tagRelevanceRecencyWeight*components.Recency +
		tagRelevanceFreshnessWeight*components.Freshness

	return score, components
}

// tagRelevanceDecay is the exponential falloff applied to a single
// insight's age: 1.0 when brand new, halving every tagRelevanceHalfLife.
func tagRelevanceDecay(age time.Duration) float64 {
	if age < 0 {
		age = 0
	}
	return math.Exp(-math.Ln2 * age.Hours() / tagRelevanceHalfLife.Hours())
}

// TagRelevanceScoreWithDensity is TagRelevanceScore plus a relationship-
// density component (REL 5/IPP-101): well-connected topics — insights with
// relationships between them — rank higher, not just often- or recently-
// used ones. avgRelationshipsPerInsight is the tag's insights' average
// relationship-edge count (both directions), computed by the caller from
// RelationshipRepository.
func TagRelevanceScoreWithDensity(insightTimestamps []time.Time, now time.Time, avgRelationshipsPerInsight float64) (float64, TagScoreComponents) {
	score, components := TagRelevanceScore(insightTimestamps, now)
	if len(insightTimestamps) == 0 {
		return score, components
	}

	components.Density = tagRelevanceDensityComponent(avgRelationshipsPerInsight)
	score = score*(1-tagRelevanceDensityWeight) + tagRelevanceDensityWeight*components.Density
	return score, components
}

// tagRelevanceDensityComponent normalizes an average relationship count
// into [0,1], the same diminishing-returns saturation curve
// TagRelevanceScore's count component uses.
func tagRelevanceDensityComponent(avgRelationshipsPerInsight float64) float64 {
	return avgRelationshipsPerInsight / (avgRelationshipsPerInsight + tagRelevanceDensitySaturation)
}
