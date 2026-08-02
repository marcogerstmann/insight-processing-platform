package domain

import (
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	tests := map[string]struct {
		in     string
		want   Tag
		wantOK bool
	}{
		"lowercases":             {in: "Delegation", want: "delegation", wantOK: true},
		"trims outer whitespace": {in: "  delegation  ", want: "delegation", wantOK: true},
		"collapses internal whitespace": {
			in: "delegation   skills", want: "delegation-skills", wantOK: true,
		},
		"strips punctuation": {in: "delegation!!", want: "delegation", wantOK: true},
		"punctuation between words collapses to one hyphen": {
			in: "delegation, skills", want: "delegation-skills", wantOK: true,
		},
		"keeps existing hyphens":   {in: "well-being", want: "well-being", wantOK: true},
		"empty string dropped":     {in: "", wantOK: false},
		"whitespace-only dropped":  {in: "   ", wantOK: false},
		"punctuation-only dropped": {in: "!!!", wantOK: false},
		"over-length dropped": {
			in: strings.Repeat("a", maxTagLength+1), wantOK: false,
		},
		"at max length kept": {
			in: strings.Repeat("a", maxTagLength), want: Tag(strings.Repeat("a", maxTagLength)), wantOK: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := NormalizeTag(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got tag %q)", ok, tc.wantOK, got)
			}
			if ok && got != tc.want {
				t.Fatalf("tag = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := map[string]struct {
		in   []string
		want []string
	}{
		"dedupes case-insensitively": {
			in:   []string{"Delegation", "delegation", "DELEGATION"},
			want: []string{"delegation"},
		},
		"drops empties and blanks": {
			in:   []string{"delegation", "", "   ", "leadership"},
			want: []string{"delegation", "leadership"},
		},
		"caps at MaxTagsPerInsight": {
			in:   []string{"a", "b", "c", "d", "e", "f", "g"},
			want: []string{"a", "b", "c", "d", "e"},
		},
		"preserves first-seen order": {
			in:   []string{"leadership", "delegation", "leadership"},
			want: []string{"leadership", "delegation"},
		},
		"nil input yields empty slice": {
			in:   nil,
			want: []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := NormalizeTags(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("tags[%d] = %q, want %q (got %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}
