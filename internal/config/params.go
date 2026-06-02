package config

import (
	"os"
	"time"
)

const (
	// PasswordMinLength minimum length of passwords.
	PasswordMinLength = 3
	BcryptCost        = 12
	CookieName        = "auth_token"
	TokenExpiration   = 24 * time.Hour
)

var JWTSecretKey = "default-secret-key-change-me"

// GetEnvParams func to get default params from env.
func GetEnvParams() {
	if envJWTSecretKey := os.Getenv("JWT_KEY"); envJWTSecretKey != "" {
		JWTSecretKey = envJWTSecretKey
	}
}
