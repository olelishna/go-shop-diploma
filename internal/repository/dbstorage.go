// Package repository implements db storage
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/model"
)

const (
	QueryTimeOut = 5 * time.Second
)

var ErrNonUnique = errors.New("data conflict")

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
func (r *DBStorage) Create(ctx context.Context, login, passwordHash string) error {
	_, err := r.pool.Exec(
		ctx,
		"INSERT INTO users (login, password_hash, created_at) VALUES ($1, $2, $3)",
		login,
		passwordHash,
		time.Now().UTC(),
	)
	if err != nil {
		pgErr, ok := errors.AsType[*pgconn.PgError](err)
		if ok && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%w: login %s already exists", ErrNonUnique, login)
		}

		return err
	}

	return nil
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
