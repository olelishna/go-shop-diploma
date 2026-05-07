// Package auth implements JWT related functions
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/service/helper"
)

type contextKey string

const userLoginKey contextKey = "user_login"

// GenerateJWT func to generate JWT token.
func GenerateJWT(login string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   login,
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
		cookie, err := r.Cookie(config.CookieName)
		if err != nil {
			if errors.Is(err, http.ErrNoCookie) {
				helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

				return
			}

			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		tokenString := cookie.Value

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

		login := claims.Subject
		if login == "" {
			helper.SendJSONError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		ctx := context.WithValue(r.Context(), userLoginKey, login)
		next.ServeHTTP(w, r.WithContext(ctx))
	}

	return http.HandlerFunc(fn)
}

// GetUserLoginFromContext get login from context.
func GetUserLoginFromContext(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(userLoginKey).(string)

	return login, ok
}
