package domain

import (
	"testing"
	"time"
)

func TestTagRelevanceScore(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	t.Run("empty tag scores zero", func(t *testing.T) {
		score, components := TagRelevanceScore(nil, now)
		if score != 0 {
			t.Fatalf("score = %v, want 0", score)
		}
		if components != (TagScoreComponents{}) {
			t.Fatalf("components = %+v, want zero value", components)
		}
	})

	t.Run("single insight today maxes recency and freshness", func(t *testing.T) {
		score, components := TagRelevanceScore([]time.Time{now}, now)
		if score <= 0 || score > 1 {
			t.Fatalf("score = %v, want in (0,1]", score)
		}
		if components.Recency != 1 {
			t.Fatalf("Recency = %v, want 1", components.Recency)
		}
		if components.Freshness != 1 {
			t.Fatalf("Freshness = %v, want 1", components.Freshness)
		}
	})

	t.Run("few recent insights outrank many old ones", func(t *testing.T) {
		var manyOld []time.Time
		for i := 0; i < 50; i++ {
			manyOld = append(manyOld, now.Add(-365*24*time.Hour))
		}
		fewRecent := []time.Time{now, now.Add(-time.Hour)}

		oldScore, _ := TagRelevanceScore(manyOld, now)
		recentScore, _ := TagRelevanceScore(fewRecent, now)

		if recentScore <= oldScore {
			t.Fatalf("recentScore = %v, want > oldScore = %v", recentScore, oldScore)
		}
	})

	t.Run("deterministic for identical inputs", func(t *testing.T) {
		timestamps := []time.Time{now.Add(-24 * time.Hour), now.Add(-48 * time.Hour)}
		score1, components1 := TagRelevanceScore(timestamps, now)
		score2, components2 := TagRelevanceScore(timestamps, now)
		if score1 != score2 || components1 != components2 {
			t.Fatalf("scores differ across identical calls: %v/%+v vs %v/%+v", score1, components1, score2, components2)
		}
	})
}
