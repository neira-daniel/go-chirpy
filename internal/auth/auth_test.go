package auth

import (
	"log"
	"testing"
)

func TestPasswords(t *testing.T) {
	passwords := []string{"password", "wearechecking", "thisFANTASTICpassword"}
	for _, password := range passwords {
		hashedPassword, err := HashPassword(password)
		if err != nil {
			log.Fatalf("can't hash password %q", password)
		}
		if err := CheckPasswordHash(hashedPassword, password); err != nil {
			t.Errorf("hashes don't match for password %q", password)
			return
		}
	}
}

func TestCompareHashesDifferentPasswords(t *testing.T) {
	password1 := "wearechecking"
	hashedPassword1, err := HashPassword(password1)
	if err != nil {
		log.Fatalf("can't hash password %q", password1)
	}

	password2 := "wearenotchecking"
	hashedPassword2, err := HashPassword(password2)
	if err != nil {
		log.Fatalf("can't hash password %q", password1)
	}

	if err := CheckPasswordHash(hashedPassword1, password2); err == nil {
		t.Errorf("hash for %q matches the hash for %q", password1, password2)
		return
	}
	if err := CheckPasswordHash(hashedPassword2, password1); err == nil {
		t.Errorf("hash for %q matches the hash for %q", password2, password1)
		return
	}
}

func TestSalt(t *testing.T) {
	password := "wearechecking"
	hash1, err := HashPassword(password)
	if err != nil {
		log.Fatalf("can't hash password %q", password)
	}
	hash2, err := HashPassword(password)
	if err != nil {
		log.Fatalf("can't hash password %q", password)
	}
	if hash1 == hash2 {
		t.Errorf("salt not working for password %q", password)
		return
	}
}
