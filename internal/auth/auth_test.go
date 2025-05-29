package auth

import (
	"fmt"
	"log"
	"testing"
)

func TestPasswordManagement(t *testing.T) {
	password1 := "we are checking"
	hashedPassword1, err := HashPassword(password1)
	if err != nil {
		log.Fatalf("can't hash password %q", password1)
	}

	password2 := "we are checking"
	hashedPassword2, err := HashPassword(password2)
	if err != nil {
		log.Fatalf("can't hash password %q", password2)
	}

	password3 := "thisFANTASTICpassword"
	hashedPassword3, err := HashPassword(password3)
	if err != nil {
		log.Fatalf("can't hash password %q", password3)
	}

	tests := []struct {
		name           string
		password       string
		hashedPassword string
		result         bool
	}{
		{
			name:           "Assert valid password: 1",
			password:       password1,
			hashedPassword: hashedPassword1,
			result:         true,
		},
		{
			name:           "Assert valid password: 2",
			password:       password1,
			hashedPassword: hashedPassword2,
			result:         true,
		},
		{
			name:           "Assert invalid password",
			password:       password2,
			hashedPassword: hashedPassword3,
			result:         false,
		},
		{
			name:           "Assert invalid hash",
			password:       password3,
			hashedPassword: "not a valid hash",
			result:         false,
		},
		{
			name:           "Assert empty password",
			password:       "",
			hashedPassword: hashedPassword3,
			result:         false,
		},
		{
			name:           "Assert empty password and hash",
			password:       "",
			hashedPassword: "",
			result:         false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fmt.Printf("pw=%q, hash=%q\n", test.password, test.hashedPassword)
			if err := CheckPasswordHash(test.hashedPassword, test.password); (err == nil) != test.result {
				t.Error(fmt.Errorf("expected: %v; error: %w", test.result, err))
			}
		})
	}
}
