package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestWeeklyPlan_Validate_HappyPath(t *testing.T) {
	p := WeeklyPlan{FocusSentence: "Read more about distributed systems this week."}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestWeeklyPlan_Validate_EmptyFocusSentence_Rejected(t *testing.T) {
	p := WeeklyPlan{FocusSentence: "   "}
	if err := p.Validate(); !errors.Is(err, ErrEmptyFocusSentence) {
		t.Fatalf("Validate err = %v, want ErrEmptyFocusSentence", err)
	}
}

func TestWeeklyPlan_Validate_OversizedFocusSentence_Rejected(t *testing.T) {
	p := WeeklyPlan{FocusSentence: strings.Repeat("a", maxFocusSentenceLength+1)}
	if err := p.Validate(); !errors.Is(err, ErrFocusSentenceTooLong) {
		t.Fatalf("Validate err = %v, want ErrFocusSentenceTooLong", err)
	}
}
