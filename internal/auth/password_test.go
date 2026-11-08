package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword returned false for correct password")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	hash, err := HashPassword("right password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	for _, candidate := range []string{"", "wrong", "right password!", "Right Password"} {
		ok, err := VerifyPassword(hash, candidate)
		if err != nil {
			t.Fatalf("VerifyPassword(%q): %v", candidate, err)
		}
		if ok {
			t.Fatalf("VerifyPassword(%q) unexpectedly true", candidate)
		}
	}
}