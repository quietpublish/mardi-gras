package ui

import "testing"

func deref(t *testing.T, p *string, field string) string {
	t.Helper()
	if p == nil {
		t.Fatalf("%s color is nil; expected a brand hex", field)
	}
	return *p
}

// TestMardiGrasGlamourStyle verifies the brand theme maps mg's palette onto the
// markdown elements it recolors, and that it doesn't disturb the cloned dark
// base (a value copy, so the package var stays untouched).
func TestMardiGrasGlamourStyle(t *testing.T) {
	c := MardiGrasGlamourStyle()

	cases := []struct {
		field string
		got   *string
		want  string
	}{
		{"H1.Color", c.H1.Color, "#FFD700"},
		{"H1.BackgroundColor", c.H1.BackgroundColor, "#7B2D8E"},
		{"Heading.Color", c.Heading.Color, "#9B59B6"},
		{"H6.Color", c.H6.Color, "#9B59B6"},
		{"Emph.Color", c.Emph.Color, "#F5C518"},
		{"Strong.Color", c.Strong.Color, "#FFD700"},
		{"Link.Color", c.Link.Color, "#F5C518"},
		{"LinkText.Color", c.LinkText.Color, "#F5C518"},
		{"Code.Color", c.Code.Color, "#2ECC71"},
		{"Item.Color", c.Item.Color, "#9B59B6"},
		{"Enumeration.Color", c.Enumeration.Color, "#9B59B6"},
	}
	for _, tc := range cases {
		if got := deref(t, tc.got, tc.field); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}

	if c.H1.Bold == nil || !*c.H1.Bold {
		t.Error("H1 should be bold")
	}
}

// TestMardiGrasGlamourStyleDoesNotMutateBase guards against accidentally
// mutating the shared DarkStyleConfig through a copied pointer: two calls must
// return identical, independent colors.
func TestMardiGrasGlamourStyleDoesNotMutateBase(t *testing.T) {
	a := MardiGrasGlamourStyle()
	b := MardiGrasGlamourStyle()
	if a.H1.Color == b.H1.Color {
		t.Error("each call should allocate fresh color pointers, not share them")
	}
	if *a.H1.Color != *b.H1.Color {
		t.Errorf("repeated calls disagree: %q vs %q", *a.H1.Color, *b.H1.Color)
	}
}
