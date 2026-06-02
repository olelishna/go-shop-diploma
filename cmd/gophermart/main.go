// Package main implements app
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/olelishna/go-shop-diploma/docs"
	"github.com/olelishna/go-shop-diploma/internal/client/accrual"
	"github.com/olelishna/go-shop-diploma/internal/compress"
	"github.com/olelishna/go-shop-diploma/internal/config"
	"github.com/olelishna/go-shop-diploma/internal/handler"
	"github.com/olelishna/go-shop-diploma/internal/logger"
	"github.com/olelishna/go-shop-diploma/internal/repository"
	"github.com/olelishna/go-shop-diploma/internal/service/auth"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	ShutdownTimeout = 15 * time.Second
	RequestTimeout  = 60 * time.Second
)

var errDSNReq = errors.New("database DSN is required")

//	@title			Gophermart API
//	@version		1.0
//	@description	This is a go diploma.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	Svistunova Olga
//	@contact.email	svistunovaoliya@yandex.ru

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath	/

//	@securityDefinitions.basic	BasicAuth

func main() {
	config.ParseFlags()
	config.GetEnvParams()

	var err error

	err = logger.Init(config.FlagLogLevel)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	err = run(ctx)
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal(err.Error(), zap.String("event", "run server"))
		}
	}
}

func run(ctx context.Context) error {
	logger.Log.Info("Running server at", zap.String("addr", config.FlagRunAddr))

	if config.FlagDatabaseDSN == "" {
		return errDSNReq
	}

	pool, err := pgxpool.New(ctx, config.FlagDatabaseDSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	dbs, err := repository.NewDBStorage(pool)
	if err != nil {
		return err
	}

	client := accrual.NewClient(config.FlagAccrualSystemAddress)

	hand := handler.NewHandler(ctx, dbs, client)

	r := chi.NewRouter()

	r.Use(middleware.Timeout(RequestTimeout))

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

		r.Post("/register", hand.Register)
		r.Post("/login", hand.Login)

		// Secure routes

		r.Route("/orders", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Post("/", hand.UploadOrder)
			r.Get("/", hand.GetOrders)
		})

		r.Route("/balance", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Get("/", hand.GetBalance)
			r.Post("/withdraw", hand.Withdraw)
		})

		r.Route("/withdrawals", func(r chi.Router) {
			r.Use(auth.MiddlewareAuth)
			r.Get("/", hand.GetWithdrawals)
		})
	})

	uRes, _ := url.JoinPath(config.FlagBaseURLResult, "swagger/doc.json")

	// Swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL(uRes),
	))

	nCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, os.Kill)
	defer stop()

	httpServer := http.Server{
		Addr:    config.FlagRunAddr,
		Handler: r,
	}

	group, gCtx := errgroup.WithContext(nCtx)
	group.Go(httpServer.ListenAndServe)
	group.Go(func() error {
		<-gCtx.Done()

		logger.Log.Info(gCtx.Err().Error())
		stop()

		logger.Log.Info("going to shutdown server")

		tCtx, cancelFn := context.WithTimeout(gCtx, ShutdownTimeout)
		defer cancelFn()

		err := httpServer.Shutdown(tCtx)

		if err != nil {
			logger.Log.Error(err.Error(), zap.String("event", "shutdown server"))
		} else {
			logger.Log.Info("server shutdown was successful")
		}

		return err
	})

	err = group.Wait()
	if err != nil {
		return err
	}

	return nil
}
