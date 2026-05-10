package domain

import (
	"errors"
	"time"
)

var (
	ErrInviteInactive        = errors.New("invite campaign is inactive")
	ErrInviteExpired         = errors.New("invite campaign is expired")
	ErrInviteRequiresNew     = errors.New("invite requires a new user")
	ErrInviteTotalLimit      = errors.New("invite total redemption limit reached")
	ErrInviteUserLimit       = errors.New("invite user redemption limit reached")
	ErrInviteEmailLimit      = errors.New("invite email redemption limit reached")
	ErrInviteIPLimit         = errors.New("invite ip redemption limit reached")
	ErrInviteDeviceLimit     = errors.New("invite device redemption limit reached")
	ErrInviteEmailUnverified = errors.New("invite requires verified email")
	ErrInviteOAuthRequired   = errors.New("invite requires oauth binding")
)

type InviteCampaignStatus string

const (
	InviteActive InviteCampaignStatus = "active"
	InvitePaused InviteCampaignStatus = "paused"
	InviteEnded  InviteCampaignStatus = "ended"
)

type InviteCampaign struct {
	ID                   string               `json:"id"`
	Code                 string               `json:"code"`
	Name                 string               `json:"name"`
	PlanSlug             string               `json:"plan_slug"`
	GrantDays            int                  `json:"grant_days"`
	TotalLimit           int                  `json:"total_limit"`
	PerUserLimit         int                  `json:"per_user_limit"`
	PerEmailLimit        int                  `json:"per_email_limit"`
	PerIPLimit           int                  `json:"per_ip_limit"`
	PerDeviceLimit       int                  `json:"per_device_limit"`
	NewUsersOnly         bool                 `json:"new_users_only"`
	RequireEmailVerified bool                 `json:"require_email_verified"`
	RequireOAuthBinding  bool                 `json:"require_oauth_binding"`
	RequireAdminApproval bool                 `json:"require_admin_approval"`
	AllowStacking        bool                 `json:"allow_stacking"`
	StartsAt             time.Time            `json:"starts_at"`
	EndsAt               time.Time            `json:"ends_at"`
	Status               InviteCampaignStatus `json:"status"`
	Notes                string               `json:"notes"`
}

type InviteUsageSnapshot struct {
	TotalRedeemed  int
	UserRedeemed   int
	EmailRedeemed  int
	IPRedeemed     int
	DeviceRedeemed int
}

type InviteRedeemContext struct {
	Now           time.Time
	IsNewUser     bool
	EmailVerified bool
	OAuthBound    bool
	Usage         InviteUsageSnapshot
}

func (c InviteCampaign) ValidateRedeem(ctx InviteRedeemContext) error {
	if c.Status != InviteActive {
		return ErrInviteInactive
	}
	if ctx.Now.Before(c.StartsAt) || ctx.Now.After(c.EndsAt) {
		return ErrInviteExpired
	}
	if c.NewUsersOnly && !ctx.IsNewUser {
		return ErrInviteRequiresNew
	}
	if c.RequireEmailVerified && !ctx.EmailVerified {
		return ErrInviteEmailUnverified
	}
	if c.RequireOAuthBinding && !ctx.OAuthBound {
		return ErrInviteOAuthRequired
	}
	if c.TotalLimit > 0 && ctx.Usage.TotalRedeemed >= c.TotalLimit {
		return ErrInviteTotalLimit
	}
	if c.PerUserLimit > 0 && ctx.Usage.UserRedeemed >= c.PerUserLimit {
		return ErrInviteUserLimit
	}
	if c.PerEmailLimit > 0 && ctx.Usage.EmailRedeemed >= c.PerEmailLimit {
		return ErrInviteEmailLimit
	}
	if c.PerIPLimit > 0 && ctx.Usage.IPRedeemed >= c.PerIPLimit {
		return ErrInviteIPLimit
	}
	if c.PerDeviceLimit > 0 && ctx.Usage.DeviceRedeemed >= c.PerDeviceLimit {
		return ErrInviteDeviceLimit
	}
	return nil
}
