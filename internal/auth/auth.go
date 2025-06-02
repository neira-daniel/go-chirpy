package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const cost = 10

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hashedPassword), nil
}

func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	if tokenSecret == "" {
		return "", errors.New("got empty tokenSecret")
	}

	now := time.Now().UTC()
	claims := &jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("parsing JWT: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return uuid.Nil, errors.New("unknown claims type, can't proceed")
	}

	userIDString, err := claims.GetSubject()
	if err != nil {
		return uuid.Nil, fmt.Errorf("getting subject field from JWT: %w", err)
	}

	userID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil, fmt.Errorf("transforming user id of type string into uuid type: %w", err)
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("Authorization header not found")
	}

	fields := strings.Fields(authHeader)
	if len(fields) < 2 || strings.ToLower(fields[0]) != "bearer" {
		return "", errors.New("Received malformed authorization header")
	}

	return fields[1], nil
}

func MakeRefreshToken() (string, error) {
	key := make([]byte, 32)
	// we don't check the values returned by Read
	// if it doesn't work, it will panic the program anyway before returning any of
	// the values
	// also: it's bad practice to try to recover from a crypto panic
	rand.Read(key)
	return hex.EncodeToString(key), nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("Authorization header not found")
	}

	fields := strings.Fields(authHeader)
	if len(fields) < 2 || strings.ToLower(fields[0]) != "apikey" {
		return "", errors.New("Received malformed authorization header")
	}

	return fields[1], nil
}
