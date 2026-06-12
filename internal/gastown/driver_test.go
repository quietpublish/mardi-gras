package gastown

import "testing"

// Compile-time assertion lives in gt_driver.go (var _ Driver = GTDriver{}).
// These tests cover the pure, side-effect-free surface of the Gas Town driver;
// the dispatch methods themselves delegate to the existing gt CLI wrappers,
// which are exercised by their own tests.

func TestNewGTDriver(t *testing.T) {
	d := NewGTDriver()
	if d == nil {
		t.Fatal("NewGTDriver returned nil")
	}
	if got := d.Backend(); got != "gastown" {
		t.Errorf("Backend() = %q, want %q", got, "gastown")
	}
}

func TestGTDriverBackend(t *testing.T) {
	if got := (GTDriver{}).Backend(); got != "gastown" {
		t.Errorf("Backend() = %q, want %q", got, "gastown")
	}
}

func TestGTDriverSupports(t *testing.T) {
	d := GTDriver{}
	tests := []struct {
		feature Feature
		want    bool
	}{
		{FeatureVitals, true},
		{FeatureCosts, true},
		{FeaturePatrol, true},
		{FeatureSSE, false},
		{Feature(999), false}, // unknown feature
	}
	for _, tt := range tests {
		if got := d.Supports(tt.feature); got != tt.want {
			t.Errorf("Supports(%d) = %v, want %v", tt.feature, got, tt.want)
		}
	}
}
