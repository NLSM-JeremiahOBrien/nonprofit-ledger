package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword: expected true for correct password")
	}

	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword: expected false for incorrect password")
	}
}

func TestPasswordHashIsSelfDescribing(t *testing.T) {
	hash, err := HashPassword("some-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if hash[:10] != "$argon2id$" {
		t.Fatalf("expected PHC-style argon2id prefix, got %q", hash)
	}
}

func TestPasswordHashUsesRandomSalt(t *testing.T) {
	hash1, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	hash2, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if hash1 == hash2 {
		t.Fatal("expected two hashes of the same password to differ (random salt)")
	}
}

func TestPasswordVerifyRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-valid-hash", "anything") {
		t.Fatal("expected VerifyPassword to return false for malformed hash, not panic or true")
	}
	if VerifyPassword("", "anything") {
		t.Fatal("expected VerifyPassword to return false for empty hash")
	}
}
