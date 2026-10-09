// Package auth implements JWT related functions
package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/service/helper"
)

type contextKey string

const userLoginId contextKey = "user_id"

// GenerateJWT func to generate JWT token.
func GenerateJWT(userId int64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userId, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.TokenExpiration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(config.JWTSecretKey))
}

// SetAuthCookie set auth cookie.
func SetAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     config.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // true при HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(config.TokenExpiration.Seconds()),
	})
}

// MiddlewareAuth check auth.
func MiddlewareAuth(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""

		// Сначала проверяем Authorization header (для Swagger UI и API-тестов)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Если токен не получен из Authorization, пробуем получить из cookie
		if tokenString == "" {
			cookie, err := r.Cookie(config.CookieName)
			if err == nil {
				tokenString = cookie.Value
			}
		}

		if tokenString == "" {
			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		claims := &jwt.RegisteredClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				return []byte(config.JWTSecretKey), nil
			},
		)
		if err != nil || !token.Valid {
			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		userIdS := claims.Subject
		if userIdS == "" {
			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		userId, err := strconv.ParseInt(userIdS, 10, 64)
		if err != nil {
			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		ctx := context.WithValue(r.Context(), userLoginId, userId)
		next.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(fn)
}

// GetUserIdFromContext get user ID from context.
func GetUserIdFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userLoginId).(int64)

	return id, ok
}
