package ifpay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

const (
	HeaderAppID        = "X-IFPAY-App-Id"
	HeaderTimestamp    = "X-IFPAY-Timestamp"
	HeaderNonce        = "X-IFPAY-Nonce"
	HeaderSerial       = "X-IFPAY-Serial"
	HeaderSignature    = "X-IFPAY-Signature"
	HeaderDigest       = "Digest"
	HeaderIdempotency  = "Idempotency-Key"
	DigestPrefixSHA256 = "SHA-256="
)

func GenerateNonce(length int) (string, error) {
	if length <= 0 {
		length = 18
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func BuildDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return DigestPrefixSHA256 + base64.StdEncoding.EncodeToString(sum[:])
}

func VerifyDigest(expected string, body []byte) bool {
	return strings.EqualFold(strings.TrimSpace(expected), BuildDigest(body))
}

func CanonicalMessage(method, requestPath, timestamp, nonce, digest string) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		strings.TrimSpace(requestPath),
		strings.TrimSpace(timestamp),
		strings.TrimSpace(nonce),
		strings.TrimSpace(digest),
	}, "\n")
}

func SignRSASHA256(privateKeyPEM, canonical string) (string, error) {
	key, err := parseRSAPrivateKeyPEM(privateKeyPEM)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func VerifyRSASHA256(publicKeyPEM, canonical, signature string) error {
	key, err := parseRSAPublicKeyPEM(publicKeyPEM)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	sum := sha256.Sum256([]byte(canonical))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], raw)
}

func parseRSAPublicKeyPEM(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, fmt.Errorf("invalid rsa public key pem")
	}
	publicAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		publicKey, ok := publicAny.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not rsa")
		}
		return publicKey, nil
	}
	cert, certErr := x509.ParseCertificate(block.Bytes)
	if certErr != nil {
		return nil, err
	}
	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("certificate key is not rsa")
	}
	return publicKey, nil
}

func parseRSAPrivateKeyPEM(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, fmt.Errorf("invalid rsa private key pem")
	}
	if privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not rsa")
		}
		return rsaKey, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
