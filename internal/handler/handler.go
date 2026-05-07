// Package handler implements handlers
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/model"
	"github.com/olelishna/go-shop-diploma/internal/repository"
	"github.com/olelishna/go-shop-diploma/internal/service/auth"
	"github.com/olelishna/go-shop-diploma/internal/service/helper"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Handler object.
type Handler struct {
	DB *repository.DBStorage
}

// NewHandler function.
func NewHandler(db *repository.DBStorage) *Handler {
	handler := &Handler{
		DB: db,
	}

	return handler
}

// Register регистрация пользователя.
func (h *Handler) Register(res http.ResponseWriter, req *http.Request) {
	var regReq model.RegisterRequest

	err := json.NewDecoder(req.Body).Decode(&regReq)
	if err != nil {
		helper.SendJSONError(res, "Invalid JSON format", http.StatusBadRequest)

		return
	}

	if regReq.Login == "" || regReq.Password == "" {
		helper.SendJSONError(res, "Login and password are required", http.StatusBadRequest)

		return
	}

	if len(regReq.Password) < config.PasswordMinLength {
		helper.SendJSONError(
			res,
			fmt.Sprintf("Password must be at least %d characters", config.PasswordMinLength),
			http.StatusBadRequest,
		)

		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(regReq.Password), config.BcryptCost)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "hash generation"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	ctx := req.Context()

	err = h.DB.Create(ctx, regReq.Login, string(hashed))
	if err != nil {
		if errors.Is(err, repository.ErrNonUnique) {
			helper.SendJSONError(res, "Login already taken", http.StatusConflict)
		} else {
			logger.Log.Error(err.Error(), zap.String("event", "user creation"))
			helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)
		}

		return
	}

	token, err := auth.GenerateJWT(regReq.Login)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "jwt generation"))
		helper.SendJSONError(
			res,
			"Failed to authenticate after registration",
			http.StatusInternalServerError,
		)

		return
	}

	auth.SetAuthCookie(res, token)

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]string{
		"message": "Registration successful, you are authenticated now",
	})
}

// Login аутентификация пользователя.
func (h *Handler) Login(res http.ResponseWriter, req *http.Request) {
	var authReq model.AuthRequest

	if err := json.NewDecoder(req.Body).Decode(&authReq); err != nil {
		helper.SendJSONError(res, "Invalid JSON format", http.StatusBadRequest)

		return
	}

	if authReq.Login == "" || authReq.Password == "" {
		helper.SendJSONError(res, "Login and password are required", http.StatusBadRequest)

		return
	}

	ctx := req.Context()

	user, err := h.DB.FindByLogin(ctx, authReq.Login)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "db user find by login"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	if user == nil {
		helper.SendJSONError(res, "Invalid login/password", http.StatusUnauthorized)

		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(authReq.Password))
	if err != nil {
		helper.SendJSONError(res, "Invalid login/password", http.StatusUnauthorized)

		return
	}

	token, err := auth.GenerateJWT(user.Login)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "JWT error user login"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	auth.SetAuthCookie(res, token)

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]string{
		"message": "Authentication successful",
	})

	return
}

// SendOrders загрузка пользователем номера заказа для расчёта.
func (h *Handler) SendOrders(res http.ResponseWriter, req *http.Request) {
	login, ok := auth.GetUserLoginFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	json.NewEncoder(res).Encode(map[string]string{
		"login": login,
	})
}

// GetOrders получение списка загруженных пользователем номеров заказов, статусов их обработки и
// информации о начислениях.
func (h *Handler) GetOrders(res http.ResponseWriter, req *http.Request) {
	return
}

// GetBalance получение текущего баланса счёта баллов лояльности пользователя.
func (h *Handler) GetBalance(res http.ResponseWriter, req *http.Request) {
	return
}

// Withdraw запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа.
func (h *Handler) Withdraw(res http.ResponseWriter, req *http.Request) {
	return
}

// GetWithdrawals получение информации о выводе средств с накопительного счёта пользователем.
func (h *Handler) GetWithdrawals(res http.ResponseWriter, req *http.Request) {
	return
}
