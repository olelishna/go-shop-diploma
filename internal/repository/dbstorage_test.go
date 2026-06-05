package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *DBStorage {
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

	storage, err := NewDBStorage(pool)
	require.NoError(t, err)

	return storage
}

func TestDBStorage_Create(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")

	id, err := storage.Create(ctx, "creatoruser1", "hash1")
	require.NoError(t, err)
	assert.NotZero(t, id)

	id2, err := storage.Create(ctx, "creatoruser1", "hash2")
	assert.Zero(t, id2)
	assert.ErrorIs(t, err, ErrNonUnique)
	assert.Contains(t, err.Error(), "login creatoruser1 already exists")
}

func TestDBStorage_FindByLogin(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")

	userName := "findbyloginuser1"

	_, err := storage.Pool.Exec(ctx,
		"INSERT INTO users (login, password_hash, created_at) VALUES ($1, $2, $3)",
		userName, "somehash", time.Now(),
	)
	require.NoError(t, err)

	user, err := storage.FindByLogin(ctx, userName)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, userName, user.Login)

	user2, err := storage.FindByLogin(ctx, "notexist")
	require.NoError(t, err)
	assert.Nil(t, user2)
}

func TestDBStorage_FindUserByOrderNumber(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID1, err := storage.Create(ctx, "finduser1", "hash1")
	require.NoError(t, err)

	userID2, err := storage.Create(ctx, "finduser2", "hash2")
	require.NoError(t, err)

	orderNumber := "FIND-ORD-1"
	err = storage.SaveOrder(ctx, userID1, orderNumber)
	require.NoError(t, err)

	foundUserID, err := storage.FindUserByOrderNumber(ctx, orderNumber)
	require.NoError(t, err)
	assert.Equal(t, userID1, foundUserID)

	err = storage.SaveOrder(ctx, userID2, orderNumber)
	assert.ErrorIs(t, err, ErrNonUnique)
	assert.Contains(t, err.Error(), "already uploaded by another user")

	foundUserID2, err := storage.FindUserByOrderNumber(ctx, "NOT-EXIST-ORD")
	require.NoError(t, err)
	assert.Zero(t, foundUserID2)
}

func TestDBStorage_SaveOrder(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "saveruser1", "hash2")
	otherUserID, _ := storage.Create(ctx, "saveruser2", "hash3")

	orderNumber := "SAVE-ORD-1"

	err := storage.SaveOrder(ctx, userID, orderNumber)
	require.NoError(t, err)

	err = storage.SaveOrder(ctx, otherUserID, orderNumber)
	assert.ErrorIs(t, err, ErrNonUnique)
	assert.Contains(t, err.Error(), "already uploaded by another user")
}

func TestDBStorage_GetOrders(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "getterorder", "hash4")

	orderNumber1 := "GET-ORD-1"
	orderNumber2 := "GET-ORD-2"

	_, err := storage.Pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status, accrual, uploaded_at) VALUES
		 ($1, $2, 'NEW', NULL, NOW() - INTERVAL '10 seconds'),
		 ($1, $3, 'PROCESSED', 10.5, NOW() - INTERVAL '5 seconds')`,
		userID, orderNumber1, orderNumber2,
	)
	require.NoError(t, err)

	orders, err := storage.GetOrders(ctx, userID)
	require.NoError(t, err)
	require.Len(t, orders, 2)

	assert.Equal(t, orderNumber2, orders[0].Number)
	assert.Equal(t, float64(10.5), *orders[0].Accrual)

	assert.Equal(t, orderNumber1, orders[1].Number)
	assert.Nil(t, orders[1].Accrual)
}

func TestDBStorage_GetBalance(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")

	userID, _ := storage.Create(ctx, "getbalancerzero", "hash5")

	resp, err := storage.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, model.BalanceResponse{Current: 0, Withdrawn: 0}, resp)

	var userID2 int64

	err = storage.Pool.QueryRow(
		ctx,
		`INSERT INTO users (login, password_hash, created_at, current_balance, total_withdrawn) 
				VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		"getbalancerfull",
		"hash",
		time.Now().UTC(),
		123.45,
		20.0,
	).Scan(&userID2)
	require.NoError(t, err)

	resp2, err := storage.GetBalance(ctx, userID2)
	require.NoError(t, err)
	assert.Equal(t, model.BalanceResponse{Current: 123.45, Withdrawn: 20.0}, resp2)
}

func TestDBStorage_Withdraw(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM withdrawals")

	userID, err := storage.Create(ctx, "withdrawer", "hash")
	require.NoError(t, err)

	_, err = storage.Pool.Exec(ctx,
		`UPDATE users SET current_balance = $1, total_withdrawn = $2 WHERE id = $3`,
		100.0, 0.0, userID,
	)
	require.NoError(t, err)

	req := model.WithdrawRequest{
		Order: "ORD-W-1",
		Sum:   30.0,
	}

	err = storage.Withdraw(ctx, userID, req)
	require.NoError(t, err)

	ws, err := storage.GetWithdrawals(ctx, userID)
	require.NoError(t, err)
	require.Len(t, ws, 1)
	assert.Equal(t, "ORD-W-1", ws[0].Order)
	assert.Equal(t, float64(30.0), ws[0].Sum)

	bal, err := storage.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, float64(70.0), bal.Current)
	assert.Equal(t, float64(30.0), bal.Withdrawn)

	err = storage.Withdraw(ctx, userID, req)
	assert.ErrorIs(t, err, ErrNonUnique)

	req2 := model.WithdrawRequest{Order: "ORD-W-2", Sum: 100.0}
	err = storage.Withdraw(ctx, userID, req2)
	assert.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestDBStorage_GetOrderStatusByID(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "statususer", "hash")
	_, _ = storage.Pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status) VALUES ($1, $2, $3)`,
		userID, "ORD-STATUS-1", model.StatusNew,
	)

	var orderID int64

	err := storage.Pool.QueryRow(ctx, `SELECT id FROM orders WHERE number = $1`, "ORD-STATUS-1").
		Scan(&orderID)
	require.NoError(t, err)

	statusResp, err := storage.GetOrderStatusByID(ctx, orderID)
	require.NoError(t, err)
	require.NotNil(t, statusResp)
	assert.Equal(t, model.StatusNew, statusResp.Status)

	_, err = storage.GetOrderStatusByID(ctx, 9999)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDBStorage_UpdateOrderStatus(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "statusupdater", "hash")

	orderNumber := "ORD-UPDATE"

	_, err := storage.Pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status) VALUES ($1, $2, $3)`,
		userID, orderNumber, model.StatusNew,
	)
	require.NoError(t, err)

	var orderID int64
	err = storage.Pool.QueryRow(ctx, `SELECT id FROM orders WHERE number = $1`, orderNumber).
		Scan(&orderID)
	require.NoError(t, err)

	err = storage.UpdateOrderStatus(ctx, orderID, model.StatusProcessing)
	require.NoError(t, err)

	statusResp, err := storage.GetOrderStatusByID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusProcessing, statusResp.Status)
}

func TestDBStorage_FetchPendingOrders(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "pendinguser", "hash")

	oldTime := time.Now().Add(-10 * time.Second).UTC()

	_, err := storage.Pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status, uploaded_at) VALUES
		 ($1, $2, $3, $4),
		 ($1, $5, $6, $4)`,
		userID, "P-1", model.StatusNew, oldTime, "P-2", model.StatusProcessing,
	)
	require.NoError(t, err)

	_, err = storage.Pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status, uploaded_at) VALUES ($1, $2, $3, NOW())`,
		userID, "P-3", model.StatusNew,
	)
	require.NoError(t, err)

	var orders []model.PendingOrderResponse

	for order, err := range storage.FetchPendingOrders(ctx) {
		require.NoError(t, err)

		orders = append(orders, order)
	}

	require.NoError(t, err)
	require.Len(t, orders, 2)

	numbers := map[string]bool{}
	for _, o := range orders {
		numbers[o.Number] = true
	}
	assert.True(t, numbers["P-1"], "P-1 должен быть в списке")
	assert.True(t, numbers["P-2"], "P-2 должен быть в списке")
	assert.False(t, numbers["P-3"], "P-3 не должен быть в списке")
}

func TestDBStorage_ApplyAccrual(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "accrualuser", "hash")

	orderNumber := "ORD-ACCRUAL"

	var orderID int64

	err := storage.Pool.QueryRow(ctx,
		`INSERT INTO orders (user_id, number, status) VALUES ($1, $2, $3) RETURNING id`,
		userID, orderNumber, model.StatusNew,
	).Scan(&orderID)

	require.NoError(t, err)

	err = storage.ApplyAccrual(ctx, orderID, userID, new(float64(25.75)))
	require.NoError(t, err)

	statusResp, err := storage.GetOrderStatusByID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusProcessed, statusResp.Status)

	bal, err := storage.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, float64(25.75), bal.Current)

	var storedAccrual *float64
	err = storage.Pool.QueryRow(ctx, `SELECT accrual FROM orders WHERE id = $1`, orderID).
		Scan(&storedAccrual)

	require.NoError(t, err)
	assert.Equal(t, float64(25.75), *storedAccrual)
}

func TestDBStorage_SaveOrder_DuplicateSameUser(t *testing.T) {
	ctx := context.Background()
	storage := setupTestDB(t)

	_, _ = storage.Pool.Exec(ctx, "DELETE FROM users")
	_, _ = storage.Pool.Exec(ctx, "DELETE FROM orders")

	userID, _ := storage.Create(ctx, "dupuser", "hash")
	orderNumber := "ORD-DUP"

	err := storage.SaveOrder(ctx, userID, orderNumber)
	require.NoError(t, err)

	err = storage.SaveOrder(ctx, userID, orderNumber)
	require.NoError(t, err)
}
