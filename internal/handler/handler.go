// Package handler implements handlers
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/olelishna/go-shop-diploma/internal/client/accrual"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/model"
	"github.com/olelishna/go-shop-diploma/internal/repository"
	"github.com/olelishna/go-shop-diploma/internal/service/auth"
	"github.com/olelishna/go-shop-diploma/internal/service/helper"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	updateOrdersWorkerCount = 2
	accrualRequestDelayTime = 100 * time.Millisecond
	dbRequestDelayTime      = 100 * time.Millisecond
)

// Handler object.
type Handler struct {
	DB            *repository.DBStorage
	AccrualClient *accrual.Client
	locks         sync.Map
}

// NewHandler function.
func NewHandler(ctx context.Context, db *repository.DBStorage, accClient *accrual.Client) *Handler {
	handler := &Handler{
		DB:            db,
		AccrualClient: accClient,
	}

	for w := 1; w <= updateOrdersWorkerCount; w++ {
		go handler.processOrders(ctx, w)
	}

	return handler
}

// Register регистрация пользователя.
//
//	@Summary		Register new user
//	@Description	Registers a new user with bcrypt-hashed password and sets auth cookie
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.RegisterRequest	true	"User registration request"
//	@Success		200		{object}	map[string]string		"Registration successful"
//	@Failure		400		{object}	model.ErrorResponse		"Invalid JSON or missing fields"
//	@Failure		409		{object}	model.ErrorResponse		"Login already taken"
//	@Router			/api/user/register [post]
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

	userId, err := h.DB.Create(ctx, regReq.Login, string(hashed))
	if err != nil {
		if errors.Is(err, repository.ErrNonUnique) {
			helper.SendJSONError(res, "Login already taken", http.StatusConflict)
		} else {
			logger.Log.Error(err.Error(), zap.String("event", "user creation"))
			helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)
		}

		return
	}

	token, err := auth.GenerateJWT(userId)
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
//
//	@Summary		Authenticate user
//	@Description	Logs in existing user and sets auth cookie using bcrypt
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.AuthRequest	true	"User login request"
//	@Success		200		{object}	map[string]string	"Authentication successful"
//	@Failure		400		{object}	model.ErrorResponse	"Invalid JSON or missing fields"
//	@Failure		401		{object}	model.ErrorResponse	"Invalid login/password"
//	@Router			/api/user/login [post]
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

	token, err := auth.GenerateJWT(user.ID)
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

// UploadOrder загрузка пользователем номера заказа для расчёта.
//
//	@Summary		Upload order number for accrual calculation
//	@Description	Uploads order number (digits only, Luhn-validated) with plain text body.
//	@Description	Requires JWT token in cookie or Authorization header.
//	@Tags			Orders
//	@Accept			plain
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Param			orderNumber	body	string	true	"Order number (digits only)"
//	@Success		202			"Accepted for processing"
//	@Success		200			"Order already uploaded by same user"
//	@Failure		400			{object}	model.ErrorResponse	"Invalid Content-Type, empty body, or non-digit characters"
//	@Failure		401			{object}	model.ErrorResponse	"Unauthorized (invalid or missing JWT)"
//	@Failure		409			{object}	model.ErrorResponse	"Order already uploaded by another user"
//	@Failure		422			{object}	model.ErrorResponse	"Invalid order number (Luhn check failed)"
//	@Router			/api/user/orders [post]
func (h *Handler) UploadOrder(res http.ResponseWriter, req *http.Request) {
	userID, ok := auth.GetUserIdFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	if req.Header.Get("Content-Type") != "text/plain" {
		helper.SendJSONError(
			res,
			"Invalid Content-Type, expected text/plain",
			http.StatusBadRequest,
		)

		return
	}

	body := make([]byte, 1024*1024)

	n, err := req.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		helper.SendJSONError(res, "Failed to read body", http.StatusBadRequest)

		return
	}

	orderNumber := strings.TrimSpace(string(body[:n]))
	if orderNumber == "" {
		helper.SendJSONError(res, "Empty order number", http.StatusBadRequest)

		return
	}

	for _, ch := range orderNumber {
		if !unicode.IsDigit(ch) {
			helper.SendJSONError(
				res,
				"Order number must contain digits only",
				http.StatusBadRequest,
			)

			return
		}
	}

	if !helper.LuhnValid(orderNumber) {
		helper.SendJSONError(res, "Invalid order number", http.StatusUnprocessableEntity)

		return
	}

	ctx := req.Context()

	existingUserID, err := h.DB.FindUserByOrderNumber(ctx, orderNumber)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "upload order"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	if existingUserID != 0 {
		if existingUserID == userID {
			res.WriteHeader(http.StatusOK)

			return
		}

		helper.SendJSONError(
			res,
			"Order number already uploaded by another user",
			http.StatusConflict,
		)

		return
	}

	err = h.DB.SaveOrder(ctx, userID, orderNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNonUnique) {
			helper.SendJSONError(res, err.Error(), http.StatusConflict)
		} else {
			logger.Log.Error(err.Error(), zap.String("event", "save order"))
			helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)
		}

		return
	}

	res.WriteHeader(http.StatusAccepted)
}

// GetOrders получение списка загруженных пользователем номеров заказов, статусов их обработки и
// информации о начислениях.
//
//	@Summary		Get uploaded orders with status and accrual info
//	@Description	Returns list of orders for authenticated user. Supports cookie or Authorization header.
//	@Tags			Orders
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Success		200	{array}	model.OrderResponse	"List of orders"
//	@Success		204	"No orders found"
//	@Failure		401	{object}	model.ErrorResponse	"Unauthorized (invalid or missing JWT)"
//	@Failure		500	{object}	model.ErrorResponse	"Internal server error"
//	@Router			/api/user/orders [get]
func (h *Handler) GetOrders(res http.ResponseWriter, req *http.Request) {
	userID, ok := auth.GetUserIdFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	orders, err := h.DB.GetOrders(req.Context(), userID)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "get orders"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	if len(orders) == 0 {
		res.WriteHeader(http.StatusNoContent)

		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(res).Encode(orders); err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "upload order"))

		return
	}
}

// GetBalance получение текущего баланса счёта баллов лояльности пользователя.
//
//	@Summary		Get current loyalty points balance
//	@Description	Returns current and withdrawn balance for authenticated user. Supports cookie or Authorization header.
//	@Tags			Balance
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Success		200	{object}	model.BalanceResponse	"User balance"
//	@Failure		401	{object}	model.ErrorResponse		"Unauthorized (invalid or missing JWT)"
//	@Failure		500	{object}	model.ErrorResponse		"Internal server error"
//	@Router			/api/user/balance [get]
func (h *Handler) GetBalance(res http.ResponseWriter, req *http.Request) {
	userID, ok := auth.GetUserIdFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	balanceResp, err := h.DB.GetBalance(req.Context(), userID)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "get balance"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(res).Encode(balanceResp); err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "get balance"))

		return
	}
}

// Withdraw запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа.
//
//	@Summary		Request withdrawal of points for new order
//	@Description	Requests points withdrawal (validated sum and order number). Requires JWT token.
//	@Tags			Balance
//	@Accept			json
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Param			request	body	model.WithdrawRequest	true	"Withdrawal request"
//	@Success		200		"Success"
//	@Failure		401		{object}	model.ErrorResponse	"Unauthorized (invalid or missing JWT)"
//	@Failure		402		{object}	model.ErrorResponse	"Insufficient balance"
//	@Failure		422		{object}	model.ErrorResponse	"Invalid request (sum ≤ 0 or invalid order number)"
//	@Router			/api/user/balance/withdraw [post]
func (h *Handler) Withdraw(res http.ResponseWriter, req *http.Request) {
	userID, ok := auth.GetUserIdFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	var wReq model.WithdrawRequest
	if err := json.NewDecoder(req.Body).Decode(&wReq); err != nil {
		helper.SendJSONError(res, "Invalid JSON format", http.StatusUnprocessableEntity)

		return
	}

	if wReq.Sum <= 0 {
		helper.SendJSONError(res, "Sum must be positive", http.StatusUnprocessableEntity)

		return
	}

	if !helper.IsValidOrderNumber(wReq.Order) {
		helper.SendJSONError(res, "Invalid order number", http.StatusUnprocessableEntity)

		return
	}

	err := h.DB.Withdraw(req.Context(), userID, wReq)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			helper.SendJSONError(res, err.Error(), http.StatusPaymentRequired)

			return
		} else if errors.Is(err, repository.ErrNonUnique) {
			helper.SendJSONError(res, err.Error(), http.StatusUnprocessableEntity)

			return
		}

		logger.Log.Error(err.Error(), zap.String("event", "withdraw"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)

		return
	}

	res.WriteHeader(http.StatusOK)
}

// GetWithdrawals получение информации о выводе средств с накопительного счёта пользователем.
//
//	@Summary		Get withdrawal history
//	@Description	Returns list of withdrawals for authenticated user. Supports cookie or Authorization header.
//	@Tags			Balance
//	@Produce		json
//
//	@Security		ApiKeyAuth
//
//	@Success		200	{array}	model.WithdrawalResponse	"Withdrawal history"
//	@Success		204	"No withdrawals found"
//	@Failure		401	{object}	model.ErrorResponse	"Unauthorized (invalid or missing JWT)"
//	@Failure		500	{object}	model.ErrorResponse	"Internal server error"
//	@Router			/api/user/withdrawals [get]
func (h *Handler) GetWithdrawals(res http.ResponseWriter, req *http.Request) {
	userID, ok := auth.GetUserIdFromContext(req.Context())
	if !ok {
		helper.SendJSONError(res, "Unauthorized", http.StatusUnauthorized)

		return
	}

	withdrawals, err := h.DB.GetWithdrawals(req.Context(), userID)
	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "get withdrawals"))
		helper.SendJSONError(res, "Internal server error", http.StatusInternalServerError)
	}

	if len(withdrawals) == 0 {
		res.WriteHeader(http.StatusNoContent)

		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(res).Encode(withdrawals); err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "get withdrawals"))

		return
	}
}

func (h *Handler) processOrders(ctx context.Context, id int) {
	logger.Log.Info("Starting processing orders worker", zap.Int("id", id))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("Shutting down worker", zap.Int("id", id))

			return
		default:
		}

		orders, err := h.DB.FetchPendingOrders(ctx)
		if err != nil {
			logger.Log.Error(
				fmt.Sprintf("failed to fetch pending orders: %v", err),
				zap.String("event", "process orders"),
			)

			continue
		}

		for _, order := range orders {
			h.processOrder(ctx, order)

			time.Sleep(accrualRequestDelayTime)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(dbRequestDelayTime):
		}

		logger.Log.Info("Worker job is done", zap.Int("id", id))
	}
}

func (h *Handler) processOrder(ctx context.Context, order model.PendingOrderResponse) {
	mu := h.getLock(order.Number)
	mu.Lock()
	defer mu.Unlock()

	// Double-check: статус мог измениться между fetch и блокировкой
	current, err := h.DB.GetOrderStatusByID(ctx, order.ID)
	if err != nil {
		logger.Log.Error(
			fmt.Sprintf("failed to reload order %s after lock: %v", order.Number, err),
			zap.String("event", "process order"),
		)

		return
	}

	if current.Status == model.StatusProcessed || current.Status == model.StatusInvalid {
		logger.Log.Debug(
			fmt.Sprintf("order %s already final (status: %s), skip", order.Number, current.Status),
			zap.String("event", "process order"),
		)

		return
	}

	resp, err := h.AccrualClient.GetAccrual(ctx, order.Number)
	if err != nil {
		logger.Log.Warn(
			fmt.Sprintf("loyalty request failed for order %s: %v", order.Number, err),
			zap.String("event", "process orders"),
		)

		err = h.DB.UpdateOrderStatus(ctx, order.ID, model.StatusProcessing)
		if err != nil {
			logger.Log.Error(err.Error(), zap.String("event", "process order"))
		}

		return
	}

	switch resp.Status {
	case model.StatusProcessed:
		err = h.DB.ApplyAccrual(ctx, order.ID, order.UserID, resp.Accrual)
	case model.StatusInvalid:
		err = h.DB.UpdateOrderStatus(ctx, order.ID, model.StatusInvalid)
	case model.StatusProcessing:
		err = h.DB.UpdateOrderStatus(ctx, order.ID, model.StatusProcessing)
	case model.StatusNew:
		err = h.DB.UpdateOrderStatus(ctx, order.ID, model.StatusProcessing)
	default:
		logger.Log.Warn(
			fmt.Sprintf("unknown status %s for order %s", resp.Status, order.Number),
			zap.String("event", "process order"),
		)

		err = h.DB.UpdateOrderStatus(ctx, order.ID, model.StatusProcessing)
	}

	if err != nil {
		logger.Log.Error(err.Error(), zap.String("event", "process order"))
	}
}

func (h *Handler) getLock(orderNumber string) *sync.Mutex {
	if v, ok := h.locks.Load(orderNumber); ok {
		return v.(*sync.Mutex)
	}

	v, _ := h.locks.LoadOrStore(orderNumber, &sync.Mutex{})

	return v.(*sync.Mutex)
}
