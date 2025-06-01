package auth

import (
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
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
			if err := CheckPasswordHash(test.hashedPassword, test.password); (err == nil) != test.result {
				t.Error(fmt.Errorf("expected: %v; error: %w", test.result, err))
			}
		})
	}
}

func TestJWTManagement(t *testing.T) {
	user := uuid.New()
	tokenSecret := "this token is secret"
	badTokenSecret := "this token is super mega secret and wrong"
	validDuration := 8 * time.Hour
	invalidDuration := -validDuration

	tests := []struct {
		name                   string
		userID                 uuid.UUID
		tokenSecret            string
		alternativeTokenSecret string
		expiresIn              time.Duration
		createTokenError       bool
		validateTokenError     bool
		userValidationError    bool
	}{
		{
			name:                   "Assert correct creation and validation of JWT",
			userID:                 user,
			tokenSecret:            tokenSecret,
			alternativeTokenSecret: tokenSecret,
			expiresIn:              validDuration,
			createTokenError:       false,
			validateTokenError:     false,
			userValidationError:    false,
		},
		{
			name:                   "Assert empty tokenSecret",
			userID:                 user,
			tokenSecret:            "",
			alternativeTokenSecret: "",
			expiresIn:              validDuration,
			createTokenError:       true,
			validateTokenError:     true,
			userValidationError:    true,
		},
		{
			name:                   "Assert bad token secret to validate JWT",
			userID:                 user,
			tokenSecret:            tokenSecret,
			alternativeTokenSecret: badTokenSecret,
			expiresIn:              validDuration,
			createTokenError:       false,
			validateTokenError:     true,
			userValidationError:    true,
		},
		{
			name:                   "Assert invalid expiration",
			userID:                 user,
			tokenSecret:            tokenSecret,
			alternativeTokenSecret: tokenSecret,
			expiresIn:              invalidDuration,
			createTokenError:       false,
			validateTokenError:     true,
			userValidationError:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokenString, err := MakeJWT(test.userID, test.tokenSecret, test.expiresIn)
			if (err != nil) != test.createTokenError {
				t.Errorf("can't create JWT: %v", err)
			}

			recoveredUserID, err := ValidateJWT(tokenString, test.alternativeTokenSecret)
			if (err != nil) != test.validateTokenError {
				t.Errorf("can't parse JWT: %v", err)
			}

			if (test.userID != recoveredUserID) != test.userValidationError {
				t.Errorf("expected test.userID == recoveredUserID to be %v, but it isn't", test.userValidationError)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tokenString := "+pK7C2P"

	validAuthHeader1 := fmt.Sprintf("Bearer %v", tokenString)
	validHeader1 := http.Header{
		"Authorization": []string{validAuthHeader1},
	}

	validAuthHeader2 := fmt.Sprintf("    Bearer %v\t", tokenString)
	validHeader2 := http.Header{
		"Authorization": []string{validAuthHeader2},
	}

	invalidAuthHeader := fmt.Sprintf("Bearer: %v", tokenString)
	invalidHeader := http.Header{
		"Authorization": []string{invalidAuthHeader},
	}

	tests := []struct {
		name             string
		tokenString      string
		header           http.Header
		parseHeaderError bool
		incorrectToken   bool
	}{
		{
			name:             "valid token string 1",
			tokenString:      tokenString,
			header:           validHeader1,
			parseHeaderError: false,
			incorrectToken:   false,
		},
		{
			name:             "valid token string 2",
			tokenString:      tokenString,
			header:           validHeader2,
			parseHeaderError: false,
			incorrectToken:   false,
		},
		{
			name:             "invalid header",
			tokenString:      tokenString,
			header:           invalidHeader,
			parseHeaderError: true,
			incorrectToken:   true,
		},
		{
			name:             "invalid token string",
			tokenString:      "bad token string",
			header:           validHeader1,
			parseHeaderError: false,
			incorrectToken:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bearerToken, err := GetBearerToken(test.header)
			if (err != nil) != test.parseHeaderError {
				t.Error("found unexpected mismatch when parsing header")
			}
			if (bearerToken != test.tokenString) != test.incorrectToken {
				t.Errorf("got %q when expecting %q", bearerToken, test.tokenString)
			}
		})
	}
}
