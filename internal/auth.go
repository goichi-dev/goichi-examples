package internal

import (
	"os"
	"time"

	"github.com/goichi-dev/goichi/middleware"
)

// Secret is read from the JWT_SECRET environment variable. Never hard-code a
// signing secret in source — this falls back to an obvious dev-only value so the
// examples still run locally, but production must set JWT_SECRET.
var Secret = func() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "INSECURE-DEV-ONLY-CHANGE-ME"
}()

// IssueToken signs a one-hour token for username. It reports false when the
// credentials do not match the demo account.
func IssueToken(username, password string) (string, bool) {
	if username != "admin" || password != "password" {
		return "", false
	}
	token, err := middleware.EncodeJWT(middleware.JWTClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}, Secret)
	if err != nil {
		return "", false
	}
	return token, true
}
