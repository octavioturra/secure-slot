package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/octavioturra/secure-slot/backend/internal/config"
	"github.com/octavioturra/secure-slot/backend/pkg/response"
	"golang.org/x/time/rate"
)

type Server struct {
	cfg    *config.Config
	router chi.Router
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
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
		// Routes will be registered here as features are implemented.
	})

	return r
}
