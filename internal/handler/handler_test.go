package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olelishna/go-shop-diploma/internal/client/accrual"
	"github.com/olelishna/go-shop-diploma/internal/compress"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/model"
	"github.com/olelishna/go-shop-diploma/internal/repository"
	"github.com/olelishna/go-shop-diploma/internal/service/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupDBStorage(t *testing.T) *repository.DBStorage {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping DB tests")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	config.FlagDatabaseDSN = dsn

	storage, err := repository.NewDBStorage(pool)
	require.NoError(t, err)

	ctx := context.Background()

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")

	return storage
}

func doRequest(
	t *testing.T,
	r *chi.Mux,
	method, path, body string,
	headers ...string,
) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request

	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	if len(headers) > 0 {
		req.Header.Set(headers[0], headers[1])
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}

func newRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()

	r.Get("/", func(res http.ResponseWriter, req *http.Request) {
		res.Write([]byte("hi"))
	})

	r.Route("/api/user", func(r chi.Router) {
		r.Use(
			middleware.CleanPath,
			middleware.Recoverer,
			logger.MiddlewareLogger,
			compress.MiddlewareGzip,
		)

		// Public routes
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)

		// Secure routes
		r.Route("/orders", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Post("/", h.UploadOrder)
			r.Get("/", h.GetOrders)
		})

		r.Route("/balance", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Get("/", h.GetBalance)
			r.Post("/withdraw", h.Withdraw)
		})

		r.Route("/withdrawals", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Get("/", h.GetWithdrawals)
		})
	})

	return r
}

func TestHandler_Register_Success(t *testing.T) {
	db := setupDBStorage(t)

	accrualClient := accrual.NewMockAccrualClientInterface(t)

	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqBody := map[string]string{
		"login":    "newuser",
		"password": "strongpass123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := doRequest(t, r, "POST", "/api/user/register", string(jsonBody))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Registration successful")
}

func TestHandler_Register_Validation(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	t.Run("Empty body", func(t *testing.T) {
		w := doRequest(t, r, "POST", "/api/user/register", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Missing password", func(t *testing.T) {
		reqBody := map[string]string{"login": "user"}
		jsonBody, _ := json.Marshal(reqBody)
		w := doRequest(t, r, "POST", "/api/user/register", string(jsonBody))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Short password", func(t *testing.T) {
		reqBody := map[string]string{"login": "user", "password": "12"}
		jsonBody, _ := json.Marshal(reqBody)
		w := doRequest(t, r, "POST", "/api/user/register", string(jsonBody))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_Register_DuplicateLogin(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqBody := map[string]string{"login": "dupuser", "password": "pass123456"}
	jsonBody, _ := json.Marshal(reqBody)

	_ = doRequest(t, r, "POST", "/api/user/register", string(jsonBody))
	w := doRequest(t, r, "POST", "/api/user/register", string(jsonBody))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandler_Login_Success(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqBody := map[string]string{"login": "loginuser", "password": "pass123456"}
	jsonBody, _ := json.Marshal(reqBody)
	_ = doRequest(t, r, "POST", "/api/user/register", string(jsonBody))
	w := doRequest(t, r, "POST", "/api/user/login", string(jsonBody))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Authentication successful")
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqBody := map[string]string{"login": "notexist", "password": "wrong"}
	jsonBody, _ := json.Marshal(reqBody)
	w := doRequest(t, r, "POST", "/api/user/login", string(jsonBody))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_UploadOrder_Success(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuth := map[string]string{"login": "orderuser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	orderNumber := "4532015112830366"

	req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	req.Header.Set("Content-Type", "text/plain")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code, "Expected 202 Accepted")
}

func TestHandler_UploadOrder_DuplicateSameUser(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuth := map[string]string{"login": "dupuser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	orderNumber := "4532015112830366"

	req1 := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	req1.Header.Set("Content-Type", "text/plain")

	for _, c := range cookies {
		req1.AddCookie(c)
	}

	req2 := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	req2.Header.Set("Content-Type", "text/plain")

	for _, c := range cookies {
		req2.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req1)
	assert.Equal(t, http.StatusAccepted, w.Code, "Expected 202 Accepted")

	n := httptest.NewRecorder()
	r.ServeHTTP(n, req2)
	assert.Equal(t, http.StatusOK, n.Code)
}

func TestHandler_UploadOrder_DifferentUser(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuthA := map[string]string{"login": "userA", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuthA)
	authRecA := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRecA.Code)

	cookiesA := authRecA.Result().Cookies()

	reqAuthB := map[string]string{"login": "userB", "password": "pass123456"}
	jsonAuthB, _ := json.Marshal(reqAuthB)
	authRecB := doRequest(t, r, "POST", "/api/user/register", string(jsonAuthB))
	require.Equal(t, http.StatusOK, authRecB.Code)

	cookiesB := authRecB.Result().Cookies()

	orderNumber := "4532015112830366"

	reqA := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	reqA.Header.Set("Content-Type", "text/plain")

	for _, c := range cookiesA {
		reqA.AddCookie(c)
	}

	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	require.Equal(t, http.StatusAccepted, wA.Code)

	reqB := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	reqB.Header.Set("Content-Type", "text/plain")

	for _, c := range cookiesB {
		reqB.AddCookie(c)
	}

	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	assert.Equal(t, http.StatusConflict, wB.Code)
}

func TestHandler_UploadOrder_InvalidOrder(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqAuth := map[string]string{"login": "orduser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))

	cookies := authRec.Result().Cookies()

	t.Run("Empty body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(""))

		for _, c := range cookies {
			req.AddCookie(c)
		}

		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Non-digit characters", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader("ORD-123"))

		for _, c := range cookies {
			req.AddCookie(c)
		}

		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Invalid Luhn", func(t *testing.T) {
		req := httptest.NewRequest(
			"POST",
			"/api/user/orders",
			strings.NewReader("1234567890123456"),
		)

		for _, c := range cookies {
			req.AddCookie(c)
		}

		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestHandler_GetOrders(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)

	r := newRouter(h)

	reqAuth := map[string]string{"login": "ordersuser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))

	cookies := authRec.Result().Cookies()

	orderNumbers := []string{"4532015112830366", "6011111111111117"}
	for _, num := range orderNumbers {
		req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(num))

		for _, c := range cookies {
			req.AddCookie(c)
		}

		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusAccepted, w.Code, "Failed to upload order %s", num)
	}

	req := httptest.NewRequest("GET", "/api/user/orders", nil)

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []model.OrderResponse

	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestHandler_GetBalance(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuth := map[string]string{"login": "baluser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))

	cookies := authRec.Result().Cookies()

	req := httptest.NewRequest("GET", "/api/user/balance", nil)

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var balance model.BalanceResponse
	err := json.NewDecoder(w.Body).Decode(&balance)
	require.NoError(t, err)
	assert.Equal(t, float64(0), balance.Current)
}

func TestHandler_Withdraw_InsufficientBalance(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuth := map[string]string{"login": "withdrawuser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	reqBody := map[string]any{
		"order": "4532015112830366",
		"sum":   10.0,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(
		"POST",
		"/api/user/balance/withdraw",
		strings.NewReader(string(jsonBody)),
	)
	req.Header.Set("Content-Type", "application/json")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient balance")
}

func TestHandler_Withdraw_Validation(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	reqAuth := map[string]string{"login": "withdrawvaliduser", "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	t.Run("Negative sum", func(t *testing.T) {
		reqBody := map[string]any{"order": "ORD-1", "sum": -5.0}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(
			"POST",
			"/api/user/balance/withdraw",
			strings.NewReader(string(jsonBody)),
		)
		req.Header.Set("Content-Type", "application/json")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("Zero sum", func(t *testing.T) {
		reqBody := map[string]any{"order": "ORD-2", "sum": 0.0}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(
			"POST",
			"/api/user/balance/withdraw",
			strings.NewReader(string(jsonBody)),
		)
		req.Header.Set("Content-Type", "application/json")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("Invalid order (non-digit)", func(t *testing.T) {
		reqBody := map[string]any{"order": "ORD-123", "sum": 5.0}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(
			"POST",
			"/api/user/balance/withdraw",
			strings.NewReader(string(jsonBody)),
		)
		req.Header.Set("Content-Type", "application/json")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("Invalid Luhn checksum", func(t *testing.T) {
		reqBody := map[string]any{"order": "1234567890123456", "sum": 5.0}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(
			"POST",
			"/api/user/balance/withdraw",
			strings.NewReader(string(jsonBody)),
		)
		req.Header.Set("Content-Type", "application/json")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestHandler_GetWithdrawals(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	login := "withdrawalsuser"

	reqAuth := map[string]string{"login": login, "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	query := `SELECT id FROM users WHERE login = $1`

	var userId int64

	err := db.Pool.QueryRow(context.Background(), query, login).Scan(&userId)
	require.NoError(t, err)

	_, err = db.Pool.Exec(
		context.Background(),
		`INSERT INTO withdrawals (user_id, order_number, amount, processed_at) VALUES ($1, $2, $3, NOW())`,
		userId,
		"4532015112830366",
		50.0,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/withdrawals", nil)

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var withdrawals []model.WithdrawalResponse
	err = json.NewDecoder(w.Body).Decode(&withdrawals)
	require.NoError(t, err)
	assert.Len(t, withdrawals, 1)
	assert.Equal(t, "4532015112830366", withdrawals[0].Order)
	assert.Equal(t, float64(50.0), withdrawals[0].Sum)
}

func TestHandler_ProcessOrders(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)

	orderNumber := "4532015112830366"

	response := model.AccrualResponse{
		Order:   orderNumber,
		Status:  model.StatusProcessed,
		Accrual: new(float64(700.00)),
	}

	accrualClient.EXPECT().GetAccrual(mock.Anything, orderNumber).Return(&response, nil)

	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	login := "processuser"

	reqAuth := map[string]string{"login": login, "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	query := `SELECT id FROM users WHERE login = $1`

	var userId int64

	err := db.Pool.QueryRow(context.Background(), query, login).Scan(&userId)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	req.Header.Set("Content-Type", "text/plain")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	query2 := `SELECT id, status, uploaded_at FROM orders WHERE number = $1`

	var (
		orderId     int64
		orderStatus string
		uploadedAt  time.Time
	)

	err = db.Pool.QueryRow(context.Background(), query2, orderNumber).
		Scan(&orderId, &orderStatus, &uploadedAt)
	require.NoError(t, err)

	h.processOrder(context.Background(), model.PendingOrderResponse{
		ID:         orderId,
		Number:     orderNumber,
		Status:     orderStatus,
		UserID:     userId,
		UploadedAt: uploadedAt,
	})

	reqGet := httptest.NewRequest("GET", "/api/user/orders", nil)

	for _, c := range cookies {
		reqGet.AddCookie(c)
	}

	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)

	assert.Equal(t, http.StatusOK, wGet.Code)

	var resp []model.OrderResponse
	err = json.NewDecoder(wGet.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)

	orderResp := resp[0]
	assert.Equal(t, orderNumber, orderResp.Number)
	assert.Equal(t, model.StatusProcessed, orderResp.Status)
	assert.NotNil(t, orderResp.Accrual)
	assert.Greater(t, *orderResp.Accrual, float64(0))
}

func TestHandler_ProcessOrders_AccrualStatus(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)

	orderNumber := "4532015112830366"

	response := model.AccrualResponse{
		Order:   orderNumber,
		Status:  model.StatusInvalid,
		Accrual: nil,
	}

	accrualClient.EXPECT().GetAccrual(mock.Anything, orderNumber).Return(&response, nil)

	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	login := "processinvalid"

	reqAuth := map[string]string{"login": login, "password": "pass123456"}
	jsonAuth, _ := json.Marshal(reqAuth)
	authRec := doRequest(t, r, "POST", "/api/user/register", string(jsonAuth))
	require.Equal(t, http.StatusOK, authRec.Code)

	cookies := authRec.Result().Cookies()

	query := `SELECT id FROM users WHERE login = $1`

	var userId int64

	err := db.Pool.QueryRow(context.Background(), query, login).Scan(&userId)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/user/orders", strings.NewReader(orderNumber))
	req.Header.Set("Content-Type", "text/plain")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	query2 := `SELECT id, status, uploaded_at FROM orders WHERE number = $1`

	var (
		orderId     int64
		orderStatus string
		uploadedAt  time.Time
	)

	err = db.Pool.QueryRow(context.Background(), query2, orderNumber).
		Scan(&orderId, &orderStatus, &uploadedAt)
	require.NoError(t, err)

	h.processOrder(context.Background(), model.PendingOrderResponse{
		ID:         orderId,
		Number:     orderNumber,
		Status:     orderStatus,
		UserID:     userId,
		UploadedAt: uploadedAt,
	})

	reqGet := httptest.NewRequest("GET", "/api/user/orders", nil)

	for _, c := range cookies {
		reqGet.AddCookie(c)
	}

	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)

	var resp []model.OrderResponse
	err = json.NewDecoder(wGet.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, model.StatusInvalid, resp[0].Status)
	assert.Nil(t, resp[0].Accrual)
}

func TestHandler_AuthRequired(t *testing.T) {
	db := setupDBStorage(t)
	accrualClient := accrual.NewMockAccrualClientInterface(t)
	h := NewHandler(context.Background(), db, accrualClient)
	r := newRouter(h)

	t.Run("Upload order without auth", func(t *testing.T) {
		req := httptest.NewRequest(
			"POST",
			"/api/user/orders",
			strings.NewReader("1234567890123456"),
		)
		req.Header.Set("Content-Type", "text/plain")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Get balance without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/user/balance", nil)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Withdraw without auth", func(t *testing.T) {
		reqBody := map[string]any{"order": "ORD-1", "sum": 10.0}
		jsonBody, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(
			"POST",
			"/api/user/balance/withdraw",
			strings.NewReader(string(jsonBody)),
		)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
