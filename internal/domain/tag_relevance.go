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
	// so the score itself stays in [0,1]. Relationship density (REL 5) will
	// join this list as a fourth weighted component once it exists.
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
)

// TagScoreComponents is the normalized (0-1) contribution of each relevance
// signal, exposed alongside the final score so callers can explain why a
// tag ranks where it does.
type TagScoreComponents struct {
	Count     float64
	Recency   float64
	Freshness float64
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
