package security

import (
	"testing"
	"time"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("secret-123456")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "secret-123456") {
		t.Fatalf("expected password to verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatalf("wrong password should not verify")
	}
}

func TestSignAndVerifyUserToken(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	token, err := SignUserToken("secret", "usr_1", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := VerifyUserToken("secret", token, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if userID != "usr_1" {
		t.Fatalf("expected usr_1, got %s", userID)
	}
	if _, err := VerifyUserToken("secret", token, now.Add(2*time.Hour)); err != ErrTokenExpired {
		t.Fatalf("expected expired token, got %v", err)
	}
}
