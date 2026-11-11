package auth_test

import (
	"strings"
	"testing"

	"github.com/groupify/groupify/backend/internal/auth"
)

func TestHashPasswordProducesPHCString(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected PHC prefix: %q", hash)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash must not contain the password")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, _ := auth.HashPassword("same-password-123")
	b, _ := auth.HashPassword("same-password-123")
	if a == b {
		t.Fatal("two hashes of the same password must differ (unique salt)")
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("s3cret-passphrase")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	ok, err := auth.VerifyPassword(hash, "s3cret-passphrase")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !ok {
		t.Fatal("correct password should verify")
	}

	ok, err = auth.VerifyPassword(hash, "s3cret-passphrasE")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok {
		t.Fatal("wrong password must not verify")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, hash := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=0,p=0$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=3,p=4$!!!$aGFzaA",
	} {
		if _, err := auth.VerifyPassword(hash, "anything"); err == nil {
			t.Fatalf("malformed hash %q should error", hash)
		}
	}
}

func TestHashPasswordRejectsOverlongInput(t *testing.T) {
	if _, err := auth.HashPassword(strings.Repeat("x", auth.MaxPasswordLength+1)); err == nil {
		t.Fatal("expected error for overlong password")
	}
}

func TestTokenHashing(t *testing.T) {
	a := auth.HashToken("token-one")
	b := auth.HashToken("token-one")
	c := auth.HashToken("token-two")

	if len(a) != 32 {
		t.Fatalf("sha256 length = %d", len(a))
	}
	if string(a) != string(b) {
		t.Fatal("hashing must be deterministic")
	}
	if string(a) == string(c) {
		t.Fatal("different tokens must hash differently")
	}
}
