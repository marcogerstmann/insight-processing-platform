package domain

import (
	"strings"
	"unicode"
)

// MaxTagsPerInsight caps how many tags an enrichment keeps, regardless of
// how many the LLM returns.
const MaxTagsPerInsight = 5

// maxTagLength drops tags that are implausibly long (e.g. the model
// returning a sentence instead of a tag).
const maxTagLength = 40

// Tag is a normalized, storable tag: lowercase, hyphenated, punctuation-free.
type Tag string

// NormalizeTag deterministically normalizes a raw tag string: trims,
// lowercases, strips punctuation, collapses whitespace, and joins words
// with hyphens. It reports false if the result is empty or too long.
//
// TRADE-OFF: no stemming/lemmatization, so "delegating" and "delegation"
// stay distinct tags. Add a consolidation pass only if real data shows
// synonym drift that this cheap normalization doesn't catch.
func NormalizeTag(raw string) (Tag, bool) {
	lowered := strings.ToLower(strings.TrimSpace(raw))

	var b strings.Builder
	for _, r := range lowered {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		}
	}

	normalized := strings.Join(strings.Fields(b.String()), "-")
	if normalized == "" || len(normalized) > maxTagLength {
		return "", false
	}
	return Tag(normalized), true
}

// NormalizeTags normalizes raw tags, drops invalid/duplicate ones, and caps
// the result at MaxTagsPerInsight.
func NormalizeTags(raw []string) []string {
	seen := make(map[Tag]bool, len(raw))
	result := make([]string, 0, MaxTagsPerInsight)

	for _, r := range raw {
		if len(result) >= MaxTagsPerInsight {
			break
		}
		tag, ok := NormalizeTag(r)
		if !ok || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, string(tag))
	}

	return result
}
