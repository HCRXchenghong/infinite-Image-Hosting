package ifpay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestSignAndVerifyRSASHA256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))

	body := []byte(`{"payment_method":"ifpay","sub_method":"wechat","order_id":"ord_1","amount":1200}`)
	digest := BuildDigest(body)
	canonical := CanonicalMessage("POST", "/api/ifpay/v1/payments", "1777777777", "nonce", digest)
	signature, err := SignRSASHA256(privatePEM, canonical)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if err := VerifyRSASHA256(publicPEM, canonical, signature); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if VerifyDigest(digest, []byte(`{}`)) {
		t.Fatalf("digest should reject modified body")
	}
}

func TestAPIBaseNormalizesOptionalAPISuffix(t *testing.T) {
	cases := map[string]string{
		"https://pay.example.com":      "https://pay.example.com",
		"https://pay.example.com/":     "https://pay.example.com",
		"https://pay.example.com/api":  "https://pay.example.com",
		"https://pay.example.com/api/": "https://pay.example.com",
	}
	for input, want := range cases {
		if got := apiBase(input); got != want {
			t.Fatalf("apiBase(%q) = %q, want %q", input, got, want)
		}
	}
}
