package ifpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	BaseURL       string
	PartnerAppID  string
	ClientID      string
	ClientSecret  string
	PrivateKeyPEM string
	HTTPClient    *http.Client
}

type PaymentCreateRequest struct {
	PaymentMethod string         `json:"payment_method"`
	SubMethod     string         `json:"sub_method"`
	OrderID       string         `json:"order_id"`
	Amount        int64          `json:"amount"`
	Currency      string         `json:"currency"`
	Description   string         `json:"description,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type PaymentResponse struct {
	PaymentID     string `json:"payment_id"`
	PaymentMethod string `json:"payment_method"`
	SubMethod     string `json:"sub_method"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	QRSessionID   string `json:"qr_session_id,omitempty"`
	ReviewID      string `json:"review_id,omitempty"`
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
}

type OAuthUserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nickname      string `json:"nickname"`
}

func (c Client) ExchangeOAuthCode(ctx context.Context, code, redirectURI string) (OAuthTokenResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return OAuthTokenResponse{
			AccessToken: "ifpay_dev_access_" + uuid.NewString(),
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}, nil
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase(c.BaseURL)+"/api/ifpay/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return OAuthTokenResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return OAuthTokenResponse{}, fmt.Errorf("ifpay oauth token returned status %d", resp.StatusCode)
	}
	var out OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return OAuthTokenResponse{}, err
	}
	return out, nil
}

func (c Client) FetchUserInfo(ctx context.Context, accessToken string) (OAuthUserInfo, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return OAuthUserInfo{
			Subject:       "ifpay_dev_" + uuid.NewString(),
			Email:         "ifpay-user-" + uuid.NewString()[:8] + "@oauth.local",
			EmailVerified: true,
			Name:          "IF-Pay 用户",
		}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase(c.BaseURL)+"/api/ifpay/oauth/userinfo", nil)
	if err != nil {
		return OAuthUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return OAuthUserInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return OAuthUserInfo{}, fmt.Errorf("ifpay userinfo returned status %d", resp.StatusCode)
	}
	var out OAuthUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return OAuthUserInfo{}, err
	}
	return out, nil
}

func (c Client) CreatePayment(ctx context.Context, accessToken string, req PaymentCreateRequest) (PaymentResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.PrivateKeyPEM) == "" {
		return PaymentResponse{
			PaymentID:     "ifpay_dev_" + uuid.NewString(),
			PaymentMethod: "ifpay",
			SubMethod:     req.SubMethod,
			Status:        "created",
			Amount:        req.Amount,
			Currency:      req.Currency,
		}, nil
	}
	body, err := json.Marshal(req)
	if err != nil {
		return PaymentResponse{}, err
	}
	path := "/api/ifpay/v1/payments"
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce, err := GenerateNonce(18)
	if err != nil {
		return PaymentResponse{}, err
	}
	digest := BuildDigest(body)
	canonical := CanonicalMessage(http.MethodPost, path, timestamp, nonce, digest)
	signature, err := SignRSASHA256(c.PrivateKeyPEM, canonical)
	if err != nil {
		return PaymentResponse{}, err
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase(c.BaseURL)+path, bytes.NewReader(body))
	if err != nil {
		return PaymentResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set(HeaderAppID, c.PartnerAppID)
	httpReq.Header.Set(HeaderTimestamp, timestamp)
	httpReq.Header.Set(HeaderNonce, nonce)
	httpReq.Header.Set(HeaderDigest, digest)
	httpReq.Header.Set(HeaderSignature, signature)
	httpReq.Header.Set(HeaderIdempotency, "yx-img-pay-"+req.OrderID)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return PaymentResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return PaymentResponse{}, fmt.Errorf("ifpay returned status %d", resp.StatusCode)
	}
	var out PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PaymentResponse{}, err
	}
	return out, nil
}

func apiBase(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/api")
}
