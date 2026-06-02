package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/olelishna/go-shop-diploma/internal/logger"
)

func TestAppSmoke(t *testing.T) {
	r := newRouterMock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if rec.Body.String() != "hi" {
		t.Errorf("Expected body 'hi', got %q", rec.Body.String())
	}
}

func newRouterMock() *chi.Mux {
	r := chi.NewRouter()

	r.Use(logger.MiddlewareLogger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	return r
}
