package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/octavioturra/secure-slot/backend/internal/auth"
	"github.com/octavioturra/secure-slot/backend/internal/config"
	"github.com/octavioturra/secure-slot/backend/pkg/response"
	"golang.org/x/time/rate"
)

type Server struct {
	cfg            *config.Config
	router         chi.Router
	authHandler    *auth.Handler
	passkeyHandler *auth.PasskeyHandler
}

func New(cfg *config.Config, authHandler *auth.Handler, passkeyHandler *auth.PasskeyHandler) *Server {
	s := &Server{cfg: cfg, authHandler: authHandler, passkeyHandler: passkeyHandler}
	s.router = s.buildRouter()
	return s
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	rl := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: rate.Limit(50),
		Burst:             100,
	})

	r.Use(SecurityHeaders)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(rl.Handler)
	r.Use(JWTAuth(s.cfg.JWTSecret))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		response.OK(w, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", s.authHandler.Login)
			r.Get("/callback", s.authHandler.Callback)
			r.Post("/refresh", s.authHandler.Refresh)
			r.Post("/passkey/register/begin", s.passkeyHandler.RegisterBegin)
			r.Post("/passkey/register/complete", s.passkeyHandler.RegisterComplete)
			r.Post("/passkey/auth/begin", s.passkeyHandler.AuthBegin)
			r.Post("/passkey/auth/complete", s.passkeyHandler.AuthComplete)
		})
	})

	return r
}
