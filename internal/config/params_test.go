package config

import (
	"os"
	"testing"
	"time"
)

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{"PasswordMinLength", PasswordMinLength, 3},
		{"BcryptCost", BcryptCost, 12},
		{"CookieName", CookieName, "auth_token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.actual)
			}
		})
	}
}

func TestTokenExpiration_Duration(t *testing.T) {
	expected := 24 * time.Hour

	if TokenExpiration != expected {
		t.Errorf("expected TokenExpiration = %v (24h), got %v", expected, TokenExpiration)
	}

	if TokenExpiration.Seconds() != 24*60*60 {
		t.Errorf("expected TokenExpiration = 86400 seconds, got %v", TokenExpiration.Seconds())
	}
}

func TestJWTSecretKey_Default(t *testing.T) {
	original := JWTSecretKey
	defer func() { JWTSecretKey = original }()

	if original != "default-secret-key-change-me" {
		t.Errorf("expected default JWTSecretKey = 'default-secret-key-change-me', got %q", original)
	}
}

func TestGetEnvParams_EmptyEnv_Ignored(t *testing.T) {
	original := JWTSecretKey
	defer func() { JWTSecretKey = original }()

	os.Unsetenv("JWT_KEY")
	os.Setenv("JWT_KEY", "")

	GetEnvParams()

	if JWTSecretKey != "default-secret-key-change-me" {
		t.Errorf("expected JWTSecretKey unchanged after empty env, got %q", JWTSecretKey)
	}
}

func TestGetEnvParams_NoEnv_PreservesDefault(t *testing.T) {
	original := JWTSecretKey
	defer func() { JWTSecretKey = original }()

	os.Unsetenv("JWT_KEY")

	GetEnvParams()

	if JWTSecretKey != "default-secret-key-change-me" {
		t.Errorf("expected JWTSecretKey stays default after unset env, got %q", JWTSecretKey)
	}
}

func TestGetEnvParams_JWTSecretKey(t *testing.T) {
	original := JWTSecretKey
	defer func() { JWTSecretKey = original }()

	os.Unsetenv("JWT_KEY")
	os.Setenv("JWT_KEY", "production-key-abc123")

	GetEnvParams()

	if JWTSecretKey != "production-key-abc123" {
		t.Errorf("expected JWTSecretKey = 'production-key-abc123', got %q", JWTSecretKey)
	}
}

func TestGetEnvParams_PreserveExistingWithEmptyEnv(t *testing.T) {
	original := JWTSecretKey
	defer func() { JWTSecretKey = original }()

	os.Setenv("JWT_KEY", "initial-key")

	GetEnvParams()

	if JWTSecretKey != "initial-key" {
		t.Fatalf("setup failed: expected JWTSecretKey = 'initial-key', got %q", JWTSecretKey)
	}

	os.Setenv("JWT_KEY", "")

	GetEnvParams()

	if JWTSecretKey != "initial-key" {
		t.Errorf("expected JWTSecretKey unchanged after setting empty env, got %q", JWTSecretKey)
	}
}

func TestJWTSecretKey_Flow(t *testing.T) {
	if JWTSecretKey != "default-secret-key-change-me" {
		t.Fatalf("initial JWTSecretKey = %q, want default", JWTSecretKey)
	}

	os.Setenv("JWT_KEY", "production-key-123")

	GetEnvParams()

	if JWTSecretKey != "production-key-123" {
		t.Errorf("after env, JWTSecretKey = %q, want 'production-key-123'", JWTSecretKey)
	}

	os.Setenv("JWT_KEY", "")

	GetEnvParams()

	if JWTSecretKey != "production-key-123" {
		t.Errorf(
			"expected JWTSecretKey = 'production-key-123' after empty env, got %q",
			JWTSecretKey,
		)
	}

	os.Setenv("JWT_KEY", "new-valid-key")

	GetEnvParams()

	if JWTSecretKey != "new-valid-key" {
		t.Errorf("after new env, JWTSecretKey = %q, want 'new-valid-key'", JWTSecretKey)
	}

	os.Unsetenv("JWT_KEY")

	GetEnvParams()

	if JWTSecretKey != "new-valid-key" {
		t.Errorf("expected JWTSecretKey unchanged after unset env, got %q", JWTSecretKey)
	}
}
