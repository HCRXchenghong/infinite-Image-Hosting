package domain

import (
	"testing"
	"time"
)

func TestSubscriptionRetentionWindow(t *testing.T) {
	start := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	sub := NewSubscription("sub_1", "user_1", "pro", start, 24*time.Hour)

	if !sub.CanUpload(start.Add(12 * time.Hour)) {
		t.Fatalf("active subscription should upload")
	}
	if sub.CanUpload(start.Add(25 * time.Hour)) {
		t.Fatalf("retained subscription must not upload")
	}
	if status := sub.EffectiveStatus(start.Add(20 * 24 * time.Hour)); status != SubscriptionRetained {
		t.Fatalf("expected retained, got %s", status)
	}
	if status := sub.EffectiveStatus(start.Add(40 * 24 * time.Hour)); status != SubscriptionDeleted {
		t.Fatalf("expected deleted, got %s", status)
	}
}
