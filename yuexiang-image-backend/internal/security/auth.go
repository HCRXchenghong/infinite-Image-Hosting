package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrTokenMalformed = errors.New("token is malformed")
	ErrTokenExpired   = errors.New("token is expired")
	ErrTokenInvalid   = errors.New("token signature is invalid")
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateOpaqueToken(prefix string, bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 32
	}
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashSecret(secret, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(secret))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func SignUserToken(secret, userID string, ttl time.Duration, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", ErrTokenInvalid
	}
	nonce, err := GenerateOpaqueToken("", 12)
	if err != nil {
		return "", err
	}
	exp := now.Add(ttl).Unix()
	payload := fmt.Sprintf("%s.%d.%s", userID, exp, nonce)
	signature := signTokenPayload(secret, payload)
	return "yx_session_" + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature, nil
}

func VerifyUserToken(secret, token string, now time.Time) (string, error) {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "Bearer ")
	if !strings.HasPrefix(token, "yx_session_") {
		return "", ErrTokenMalformed
	}
	body := strings.TrimPrefix(token, "yx_session_")
	parts := strings.Split(body, ".")
	if len(parts) != 2 {
		return "", ErrTokenMalformed
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrTokenMalformed
	}
	payload := string(payloadBytes)
	if !hmac.Equal([]byte(signTokenPayload(secret, payload)), []byte(parts[1])) {
		return "", ErrTokenInvalid
	}
	payloadParts := strings.Split(payload, ".")
	if len(payloadParts) != 3 {
		return "", ErrTokenMalformed
	}
	expiresAt, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return "", ErrTokenMalformed
	}
	if now.Unix() > expiresAt {
		return "", ErrTokenExpired
	}
	return payloadParts[0], nil
}

func SignAdminToken(secret, adminID string, ttl time.Duration, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", ErrTokenInvalid
	}
	nonce, err := GenerateOpaqueToken("", 12)
	if err != nil {
		return "", err
	}
	exp := now.Add(ttl).Unix()
	payload := fmt.Sprintf("%s.%d.%s", adminID, exp, nonce)
	signature := signTokenPayload(secret, payload)
	return "yx_admin_" + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + signature, nil
}

func VerifyAdminToken(secret, token string, now time.Time) (string, error) {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "Bearer ")
	if !strings.HasPrefix(token, "yx_admin_") {
		return "", ErrTokenMalformed
	}
	body := strings.TrimPrefix(token, "yx_admin_")
	parts := strings.Split(body, ".")
	if len(parts) != 2 {
		return "", ErrTokenMalformed
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrTokenMalformed
	}
	payload := string(payloadBytes)
	if !hmac.Equal([]byte(signTokenPayload(secret, payload)), []byte(parts[1])) {
		return "", ErrTokenInvalid
	}
	payloadParts := strings.Split(payload, ".")
	if len(payloadParts) != 3 {
		return "", ErrTokenMalformed
	}
	expiresAt, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return "", ErrTokenMalformed
	}
	if now.Unix() > expiresAt {
		return "", ErrTokenExpired
	}
	return payloadParts[0], nil
}

func GenerateTOTPSetup(email, issuer string) (secret string, provisioningURL string, qrCodeDataURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
	})
	if err != nil {
		return "", "", "", err
	}
	imageValue, err := key.Image(240, 240)
	if err != nil {
		return "", "", "", err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageValue); err != nil {
		return "", "", "", err
	}
	return key.Secret(), key.URL(), "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func ValidateTOTP(secret, code string) bool {
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

func signTokenPayload(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
