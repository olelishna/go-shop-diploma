package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

var Log = zap.NewNop()

func Init(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl

	return nil
}

func MiddlewareLogger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			start := time.Now()

			next.ServeHTTP(w, r)

			duration := time.Since(start)

			Log.Info("info",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", duration),
			)
		}()
	}

	return http.HandlerFunc(fn)
}
