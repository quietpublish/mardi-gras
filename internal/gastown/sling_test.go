package gastown

import "testing"

func TestSlingMultipleEmpty(t *testing.T) {
	err := SlingMultiple(nil)
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleWithFormulaEmpty(t *testing.T) {
	err := SlingMultipleWithFormula(nil, "shiny")
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleEmptySlice(t *testing.T) {
	err := SlingMultiple([]string{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestSlingMultipleWithFormulaEmptySlice(t *testing.T) {
	err := SlingMultipleWithFormula([]string{}, "shiny")
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}
