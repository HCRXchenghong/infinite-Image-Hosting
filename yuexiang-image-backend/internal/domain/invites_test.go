package domain

import (
	"errors"
	"testing"
	"time"
)

func TestInviteCampaignValidateRedeem(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	campaign := InviteCampaign{
		Code:                 "seed-plus",
		PlanSlug:             "plus",
		GrantDays:            30,
		TotalLimit:           2,
		PerUserLimit:         1,
		NewUsersOnly:         true,
		RequireEmailVerified: true,
		StartsAt:             now.Add(-time.Hour),
		EndsAt:               now.Add(time.Hour),
		Status:               InviteActive,
	}

	err := campaign.ValidateRedeem(InviteRedeemContext{
		Now:           now,
		IsNewUser:     true,
		EmailVerified: true,
		Usage: InviteUsageSnapshot{
			TotalRedeemed: 1,
		},
	})
	if err != nil {
		t.Fatalf("expected redeem to pass, got %v", err)
	}

	err = campaign.ValidateRedeem(InviteRedeemContext{
		Now:           now,
		IsNewUser:     false,
		EmailVerified: true,
	})
	if !errors.Is(err, ErrInviteRequiresNew) {
		t.Fatalf("expected new user error, got %v", err)
	}
}
