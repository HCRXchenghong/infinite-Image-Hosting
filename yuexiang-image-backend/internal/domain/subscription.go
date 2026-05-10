package domain

import "time"

type SubscriptionStatus string

const (
	SubscriptionActive   SubscriptionStatus = "active"
	SubscriptionPastDue  SubscriptionStatus = "past_due"
	SubscriptionRetained SubscriptionStatus = "retained"
	SubscriptionDeleted  SubscriptionStatus = "deleted"
)

type Subscription struct {
	ID              string             `json:"id"`
	UserID          string             `json:"user_id"`
	PlanSlug        string             `json:"plan_slug"`
	Status          SubscriptionStatus `json:"status"`
	StartsAt        time.Time          `json:"starts_at"`
	EndsAt          time.Time          `json:"ends_at"`
	RetentionEndsAt time.Time          `json:"retention_ends_at"`
}

func NewSubscription(id, userID, planSlug string, startsAt time.Time, duration time.Duration) Subscription {
	endsAt := startsAt.Add(duration)
	return Subscription{
		ID:              id,
		UserID:          userID,
		PlanSlug:        planSlug,
		Status:          SubscriptionActive,
		StartsAt:        startsAt,
		EndsAt:          endsAt,
		RetentionEndsAt: endsAt.Add(30 * 24 * time.Hour),
	}
}

func (s Subscription) EffectiveStatus(now time.Time) SubscriptionStatus {
	if s.Status == SubscriptionDeleted {
		return SubscriptionDeleted
	}
	if now.Before(s.EndsAt) || now.Equal(s.EndsAt) {
		return SubscriptionActive
	}
	if now.Before(s.RetentionEndsAt) || now.Equal(s.RetentionEndsAt) {
		return SubscriptionRetained
	}
	return SubscriptionDeleted
}

func (s Subscription) CanUpload(now time.Time) bool {
	return s.EffectiveStatus(now) == SubscriptionActive
}
