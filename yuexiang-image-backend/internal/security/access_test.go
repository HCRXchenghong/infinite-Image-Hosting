package security

import (
	"testing"
	"time"
)

func TestImageAccessPolicyReturnsProtectForHotlink(t *testing.T) {
	policy := ImageAccessPolicy{
		AllowedDomains:    []string{"yuexiang.com"},
		AllowEmptyReferer: false,
		SigningSecret:     "secret",
	}
	result := policy.Evaluate(ImageAccessRequest{
		PublicID: "img_1",
		Referer:  "https://evil.example/post",
		Now:      time.Now(),
	})
	if result.Decision != DecisionProtect {
		t.Fatalf("expected protect decision, got %+v", result)
	}
}

func TestImageAccessPolicyAllowsPrivateSignedURL(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	policy := ImageAccessPolicy{SigningSecret: "secret"}
	expiresAt := now.Add(5 * time.Minute).Unix()
	token := policy.SignToken("img_private", expiresAt)

	result := policy.Evaluate(ImageAccessRequest{
		PublicID:  "img_private",
		Private:   true,
		Token:     token,
		ExpiresAt: expiresAt,
		Now:       now,
	})
	if result.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %+v", result)
	}
}
