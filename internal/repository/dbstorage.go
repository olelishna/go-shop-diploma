// Package repository implements db storage
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/model"
)

const orderBatchSize = 100

var (
	ErrNonUnique           = errors.New("data conflict")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// DBStorage db storage.
type DBStorage struct {
	pool *pgxpool.Pool
}

// NewDBStorage func to create DB connection.
func NewDBStorage(pool *pgxpool.Pool) (*DBStorage, error) {
	err := applyMigrations()
	if err != nil {
		return nil, err
	}

	return &DBStorage{
		pool: pool,
	}, nil
}

func applyMigrations() error {
	logger.Log.Info("start migrations")

	m, err := migrate.New("file://migrations", config.FlagDatabaseDSN)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return err
		}

		logger.Log.Info("database is already up-to-date")
	}

	logger.Log.Info("end migrations")

	return nil
}

// Create func creates new user.
func (r *DBStorage) Create(ctx context.Context, login, passwordHash string) (int64, error) {
	var id int64

	err := r.pool.QueryRow(
		ctx,
		"INSERT INTO users (login, password_hash, created_at) VALUES ($1, $2, $3) RETURNING id",
		login,
		passwordHash,
		time.Now().UTC(),
	).Scan(&id)
	if err != nil {
		pgErr, ok := errors.AsType[*pgconn.PgError](err)
		if ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, fmt.Errorf("%w: login %s already exists", ErrNonUnique, login)
		}

		return 0, err
	}

	return id, nil
}

// FindByLogin finds user by login.
func (r *DBStorage) FindByLogin(ctx context.Context, login string) (*model.User, error) {
	query := `
		SELECT id, login, password_hash, created_at
		FROM users
		WHERE login = $1
	`

	var u model.User

	err := r.pool.QueryRow(ctx, query, login).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &u, nil
}

// FindUserByOrderNumber finds user and order status by order ID.
func (r *DBStorage) FindUserByOrderNumber(ctx context.Context, orderNumber string) (int64, error) {
	var userID int64

	err := r.pool.QueryRow(ctx, `SELECT user_id FROM orders WHERE number = $1`, orderNumber).
		Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

// SaveOrder save new order.
func (r *DBStorage) SaveOrder(ctx context.Context, userID int64, orderNumber string) error {
	query := `INSERT INTO orders (user_id, number, status) VALUES ($1, $2, 'NEW')`

	_, err := r.pool.Exec(ctx, query, userID, orderNumber)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			var conflictUserID int64
			_ = r.pool.QueryRow(ctx, query, orderNumber).Scan(&conflictUserID, nil)

			if conflictUserID == userID {
				return nil
			}

			return fmt.Errorf("%w: order number already uploaded by another user", ErrNonUnique)
		}

		return err
	}

	return nil
}

// GetOrders get users orders.
func (r *DBStorage) GetOrders(ctx context.Context, userID int64) ([]model.OrderResponse, error) {
	query := `
        SELECT number, status, COALESCE(accrual, 0.0) AS accrual, uploaded_at
        FROM orders
        WHERE user_id = $1
        ORDER BY uploaded_at DESC
    `

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	defer rows.Close()

	var orders []model.OrderResponse

	for rows.Next() {
		var (
			number     string
			status     string
			accrual    *float64
			uploadedAt time.Time
		)

		err = rows.Scan(&number, &status, &accrual, &uploadedAt)
		if err != nil {
			return nil, err
		}

		resp := model.OrderResponse{
			Number:     number,
			Status:     status,
			UploadedAt: uploadedAt.Format(time.RFC3339),
		}

		if status == model.StatusProcessed && accrual != nil && *accrual > 0 {
			resp.Accrual = accrual
		}

		orders = append(orders, resp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetBalance get current balance of user.
func (r *DBStorage) GetBalance(ctx context.Context, userID int64) (model.BalanceResponse, error) {
	var current, withdrawn float64

	query := `SELECT current_balance, total_withdrawn FROM users WHERE id = $1`

	err := r.pool.QueryRow(ctx, query, userID).Scan(&current, &withdrawn)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			current, withdrawn = 0, 0
		} else {
			return model.BalanceResponse{}, err
		}
	}

	resp := model.BalanceResponse{Current: current, Withdrawn: withdrawn}

	return resp, nil
}

// Withdraw do withdraw.
func (r *DBStorage) Withdraw(ctx context.Context, userID int64, wReq model.WithdrawRequest) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var currentBalance float64

	err = tx.QueryRow(ctx, `SELECT current_balance FROM users WHERE id = $1 FOR UPDATE`, userID).
		Scan(&currentBalance)
	if err != nil {
		return err
	}

	if currentBalance < wReq.Sum {
		return fmt.Errorf("%w: insufficient funds", ErrInsufficientBalance)
	}

	insertWithdrawal := `INSERT INTO withdrawals (user_id, order_number, amount) VALUES ($1, $2, $3)`

	_, err = tx.Exec(ctx, insertWithdrawal, userID, wReq.Order, wReq.Sum)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			return fmt.Errorf("%w: order number already withdrawn", ErrNonUnique)
		}

		return err
	}

	updateBalance := `
        UPDATE users
        SET current_balance = current_balance - $1,
            total_withdrawn = total_withdrawn + $1
        WHERE id = $2
    `

	_, err = tx.Exec(ctx, updateBalance, wReq.Sum, userID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetWithdrawals get withdrawals of the user.
func (r *DBStorage) GetWithdrawals(
	ctx context.Context,
	userID int64,
) ([]model.WithdrawalResponse, error) {
	query := `
        SELECT order_number, amount, processed_at
        FROM withdrawals
        WHERE user_id = $1
        ORDER BY processed_at DESC
    `

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var withdrawals []model.WithdrawalResponse

	for rows.Next() {
		var (
			orderNumber string
			amount      float64
			processedAt time.Time
		)

		err = rows.Scan(&orderNumber, &amount, &processedAt)
		if err != nil {
			return nil, err
		}

		withdrawals = append(withdrawals, model.WithdrawalResponse{
			Order:       orderNumber,
			Sum:         amount,
			ProcessedAt: processedAt.Format(time.RFC3339),
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}

// FetchPendingOrders get all unprocessed orders to check status.
func (r *DBStorage) FetchPendingOrders(ctx context.Context) ([]model.PendingOrderResponse, error) {
	query := `
        SELECT id, number, status, user_id, uploaded_at
        FROM orders
        WHERE status IN ($1, $2)
          AND uploaded_at < NOW() - INTERVAL '5 seconds'
        ORDER BY uploaded_at
        LIMIT $3
        FOR UPDATE SKIP LOCKED
    `

	rows, err := r.pool.Query(ctx, query, model.StatusNew, model.StatusProcessing, orderBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.PendingOrderResponse

	for rows.Next() {
		var o model.PendingOrderResponse
		if err := rows.Scan(&o.ID, &o.Number, &o.Status, &o.UserID, &o.UploadedAt); err != nil {
			return nil, err
		}

		orders = append(orders, o)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// UpdateOrderStatus update order status.
func (r *DBStorage) UpdateOrderStatus(ctx context.Context, id int64, s string) error {
	_, err := r.pool.Exec(ctx, `
        UPDATE orders
        SET status = $1
        WHERE id = $2
    `, s, id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %v", err)
	}

	return nil
}

// ApplyAccrual apply accrual.
func (r *DBStorage) ApplyAccrual(
	ctx context.Context,
	orderID int64,
	userID int64,
	accrual float64,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var currentBalance float64

	err = tx.QueryRow(ctx, `SELECT current_balance FROM users WHERE id = $1 FOR UPDATE`, userID).
		Scan(&currentBalance)
	if err != nil {
		return err
	}

	updateOrder := `
        UPDATE orders
        SET status = $1, accrual = $2
        WHERE id = $3
    `

	_, err = tx.Exec(ctx, updateOrder, model.StatusProcessed, accrual, orderID)
	if err != nil {
		return err
	}

	updateUser := `
        UPDATE users
        SET current_balance = current_balance + $1
        WHERE id = $2
    `

	_, err = tx.Exec(ctx, updateUser, accrual, userID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetOrderStatusByID get order status.
func (r *DBStorage) GetOrderStatusByID(
	ctx context.Context,
	id int64,
) (*model.OrderStatusResponse, error) {
	query := `SELECT status FROM orders WHERE id = $1`

	var os model.OrderStatusResponse

	err := r.pool.QueryRow(ctx, query, id).Scan(&os.Status)
	if err != nil {
		return nil, err
	}

	return &os, nil
}
