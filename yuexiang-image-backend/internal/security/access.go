package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Decision string

const (
	DecisionAllow   Decision = "allow"
	DecisionProtect Decision = "protect"
	DecisionDeny    Decision = "deny"
)

type ImageAccessPolicy struct {
	AllowedDomains       []string
	BlockedDomains       []string
	AllowEmptyReferer    bool
	SigningSecret        string
	DefaultTokenLifetime time.Duration
}

type ImageAccessRequest struct {
	PublicID  string
	Referer   string
	Token     string
	ExpiresAt int64
	Now       time.Time
	Private   bool
}

type ImageAccessResult struct {
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason"`
}

func (p ImageAccessPolicy) Evaluate(req ImageAccessRequest) ImageAccessResult {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	if req.Private {
		if !p.VerifyToken(req.PublicID, req.ExpiresAt, req.Token, now) {
			return ImageAccessResult{Decision: DecisionProtect, Reason: "private_image_token_invalid_or_expired"}
		}
		return ImageAccessResult{Decision: DecisionAllow, Reason: "private_image_token_valid"}
	}
	host := refererHost(req.Referer)
	if host == "" {
		if p.AllowEmptyReferer {
			return ImageAccessResult{Decision: DecisionAllow, Reason: "empty_referer_allowed"}
		}
		return ImageAccessResult{Decision: DecisionProtect, Reason: "empty_referer_blocked"}
	}
	if matchDomain(host, p.BlockedDomains) {
		return ImageAccessResult{Decision: DecisionProtect, Reason: "blocked_referer"}
	}
	if len(p.AllowedDomains) == 0 || matchDomain(host, p.AllowedDomains) {
		return ImageAccessResult{Decision: DecisionAllow, Reason: "referer_allowed"}
	}
	return ImageAccessResult{Decision: DecisionProtect, Reason: "referer_not_allowed"}
}

func (p ImageAccessPolicy) SignToken(publicID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, []byte(p.SigningSecret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s:%d", publicID, expiresAt)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (p ImageAccessPolicy) VerifyToken(publicID string, expiresAt int64, token string, now time.Time) bool {
	if p.SigningSecret == "" || token == "" || expiresAt <= now.Unix() {
		return false
	}
	expected := p.SignToken(publicID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(token))
}

func refererHost(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func matchDomain(host string, domains []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
