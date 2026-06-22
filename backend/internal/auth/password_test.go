package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal the original password")
	}

	matched, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if !matched {
		t.Fatal("expected password to match")
	}

	matched, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error for wrong password: %v", err)
	}
	if matched {
		t.Fatal("expected wrong password not to match")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	matched, err := VerifyPassword("password", "not-an-argon2id-hash")
	if err == nil {
		t.Fatal("expected malformed hash error")
	}
	if matched {
		t.Fatal("malformed hash must not match")
	}
}
