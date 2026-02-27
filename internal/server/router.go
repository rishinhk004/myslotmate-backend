package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"myslotmate-backend/internal/auth"
	"myslotmate-backend/internal/firebase"
)

// NewRouter builds the HTTP router with all routes and middleware wired.
func NewRouter(fbApp *firebase.App) http.Handler {
	r := chi.NewRouter()

	// Generic middleware.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/rishi", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello Rishi"))
	})

	// Auth routes.
	authHandler := auth.NewHandler(fbApp.Auth)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/signUp", authHandler.VerifyIDToken)
	})

	return r
}
