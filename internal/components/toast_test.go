package components

import (
	"testing"
	"time"
)

func TestToastActiveBeforeExpiry(t *testing.T) {
	toast, _ := ShowToast("hello", ToastInfo, 5*time.Second)
	if !toast.Active() {
		t.Fatal("toast should be active before expiry")
	}
}

func TestToastInactiveAfterExpiry(t *testing.T) {
	toast := Toast{
		Message:   "expired",
		Level:     ToastInfo,
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	if toast.Active() {
		t.Fatal("toast should be inactive after expiry")
	}
}

func TestToastInactiveWhenEmpty(t *testing.T) {
	toast := Toast{
		Message:   "",
		ExpiresAt: time.Now().Add(5 * time.Second),
	}
	if toast.Active() {
		t.Fatal("toast with empty message should not be active")
	}
}

func TestShowToastSetsExpiry(t *testing.T) {
	before := time.Now()
	toast, _ := ShowToast("test", ToastSuccess, 3*time.Second)
	after := time.Now()

	expectedMin := before.Add(3 * time.Second)
	expectedMax := after.Add(3 * time.Second)

	if toast.ExpiresAt.Before(expectedMin) || toast.ExpiresAt.After(expectedMax) {
		t.Fatalf("ExpiresAt %v not in expected range [%v, %v]", toast.ExpiresAt, expectedMin, expectedMax)
	}
}

func TestToastViewEmptyWhenInactive(t *testing.T) {
	toast := Toast{
		Message:   "",
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	if got := toast.View(80); got != "" {
		t.Fatalf("inactive toast View() = %q, want empty string", got)
	}
}
