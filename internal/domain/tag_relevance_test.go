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

func TestTagRelevanceScoreWithDensity(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	timestamps := []time.Time{now}

	t.Run("empty tag scores zero regardless of density", func(t *testing.T) {
		score, components := TagRelevanceScoreWithDensity(nil, now, 10)
		if score != 0 {
			t.Fatalf("score = %v, want 0", score)
		}
		if components != (TagScoreComponents{}) {
			t.Fatalf("components = %+v, want zero value", components)
		}
	})

	t.Run("zero relationships pulls the score below TagRelevanceScore's own baseline", func(t *testing.T) {
		baseScore, _ := TagRelevanceScore(timestamps, now)
		score, components := TagRelevanceScoreWithDensity(timestamps, now, 0)
		if components.Density != 0 {
			t.Fatalf("Density = %v, want 0 for no relationships", components.Density)
		}
		if score >= baseScore {
			t.Fatalf("score = %v, want < baseScore = %v (density=0 must pull the score down)", score, baseScore)
		}
	})

	t.Run("more relationships raise the score and the density component, all else equal", func(t *testing.T) {
		lowScore, lowComponents := TagRelevanceScoreWithDensity(timestamps, now, 1)
		highScore, highComponents := TagRelevanceScoreWithDensity(timestamps, now, 10)

		if highComponents.Density <= lowComponents.Density {
			t.Fatalf("highComponents.Density = %v, want > lowComponents.Density = %v", highComponents.Density, lowComponents.Density)
		}
		if highScore <= lowScore {
			t.Fatalf("highScore = %v, want > lowScore = %v", highScore, lowScore)
		}
		// Only Density should move between the two calls — Count/Recency/
		// Freshness come from TagRelevanceScore, untouched by density.
		if lowComponents.Count != highComponents.Count || lowComponents.Recency != highComponents.Recency || lowComponents.Freshness != highComponents.Freshness {
			t.Fatalf("non-density components changed: low=%+v high=%+v", lowComponents, highComponents)
		}
	})

	t.Run("density component saturates toward 1 as relationships grow, never reaching it", func(t *testing.T) {
		_, components := TagRelevanceScoreWithDensity(timestamps, now, 1000)
		if components.Density <= 0.99 || components.Density >= 1 {
			t.Fatalf("Density = %v, want in (0.99, 1)", components.Density)
		}
	})

	t.Run("TagRelevanceScore itself never populates Density", func(t *testing.T) {
		_, components := TagRelevanceScore(timestamps, now)
		if components.Density != 0 {
			t.Fatalf("Density = %v, want 0 (only TagRelevanceScoreWithDensity sets it)", components.Density)
		}
	})
}
