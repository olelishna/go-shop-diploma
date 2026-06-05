package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/olelishna/go-shop-diploma/internal/config"
)

func TestGenerateJWT_Success(t *testing.T) {
	tokenStr, err := GenerateJWT(123)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("token is empty")
	}

	claims := &jwt.RegisteredClaims{}

	_, err = jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecretKey), nil
	})
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	if claims.Subject != "123" {
		t.Errorf("Expected subject '123', got '%s'", claims.Subject)
	}

	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		t.Error("Expected ExpiresAt and IssuedAt to be set")
	}
}

func TestSetAuthCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	token := "fake_token"
	SetAuthCookie(rec, token)

	cookieHeader := rec.Header().Get("Set-Cookie")
	if cookieHeader == "" {
		t.Fatal("Set-Cookie header is empty")
	}

	expectedParts := []string{
		config.CookieName,
		"fake_token",
		"HttpOnly",
		"SameSite=Strict",
	}
	for _, part := range expectedParts {
		if !strings.Contains(cookieHeader, part) {
			t.Errorf("Cookie header does not contain expected part '%s': %s", part, cookieHeader)
		}
	}

	maxAgeStr := strconv.FormatInt(int64(config.TokenExpiration.Seconds()), 10)
	if !strings.Contains(cookieHeader, "Max-Age="+maxAgeStr) &&
		!strings.Contains(cookieHeader, "expires=") {
		t.Errorf("Cookie header does not contain Max-Age or expires: %s", cookieHeader)
	}
}

func TestMiddlewareAuth_Unauthorized_NoCookie(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestMiddlewareAuth_Unauthorized_InvalidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: "invalid.token.here"})

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestMiddlewareAuth_Unauthorized_ExpiredToken(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:   "123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err := token.SignedString([]byte(config.JWTSecretKey))
	if err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: tokenStr})

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for expired token, got %d", rec.Code)
	}
}

func TestMiddlewareAuth_SuccessfulRequest(t *testing.T) {
	userId := int64(456)

	tokenStr, err := GenerateJWT(userId)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := GetUserIdFromContext(r.Context())
		if !ok {
			t.Error("user_id not found in context")
		}

		if id != userId {
			t.Errorf("Expected user_id=%d, got %d", userId, id)
		}

		called = true
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: config.CookieName, Value: tokenStr})

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestMiddlewareAuth_SuccessfulRequest_AuthHeader(t *testing.T) {
	userId := int64(456)

	tokenStr, err := GenerateJWT(userId)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := GetUserIdFromContext(r.Context())
		if !ok {
			t.Error("user_id not found in context")
		}

		if id != userId {
			t.Errorf("Expected user_id=%d, got %d", userId, id)
		}

		called = true
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler was not called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestMiddlewareAuth_Unauthorized_ButHasAuthorizationHeaderNoBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := MiddlewareAuth(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidScheme token123")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestGetUserIdFromContext_Success(t *testing.T) {
	ctx := context.WithValue(context.Background(), userLoginId, int64(789))
	id, ok := GetUserIdFromContext(ctx)

	if !ok {
		t.Error("Expected ok=true")
	}

	if id != 789 {
		t.Errorf("Expected id=789, got %d", id)
	}
}

func TestGetUserIdFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	id, ok := GetUserIdFromContext(ctx)

	if ok {
		t.Error("Expected ok=false")
	}

	if id != 0 {
		t.Errorf("Expected id=0, got %d", id)
	}
}

func TestGetUserIdFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), userLoginId, "not-an-int")

	id, ok := GetUserIdFromContext(ctx)
	if ok || id != 0 {
		t.Errorf("Expected ok=false and id=0, got ok=%v, id=%d", ok, id)
	}
}
