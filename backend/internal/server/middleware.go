package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/octavioturra/secure-slot/backend/pkg/response"
	"golang.org/x/time/rate"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; font-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

type RateLimiterConfig struct {
	RequestsPerSecond rate.Limit
	Burst             int
}

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiterMiddleware struct {
	cfg      RateLimiterConfig
	mu       sync.Mutex
	limiters map[string]*ipLimiter
}

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiterMiddleware {
	m := &RateLimiterMiddleware{
		cfg:      cfg,
		limiters: make(map[string]*ipLimiter),
	}
	go m.cleanup()
	return m
}

func (m *RateLimiterMiddleware) cleanup() {
	for range time.Tick(time.Minute) {
		m.mu.Lock()
		for ip, l := range m.limiters {
			if time.Since(l.lastSeen) > 5*time.Minute {
				delete(m.limiters, ip)
			}
		}
		m.mu.Unlock()
	}
}

func (m *RateLimiterMiddleware) getLimiter(ip string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.limiters[ip]
	if !ok {
		l = &ipLimiter{limiter: rate.NewLimiter(m.cfg.RequestsPerSecond, m.cfg.Burst)}
		m.limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter
}

func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !m.getLimiter(ip).Allow() {
			response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const claimsKey contextKey = "claims"

var publicPrefixes = []string{
	"/health",
	"/api/v1/auth/",
}

func isPublicRoute(path string) bool {
	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// OTP and submit endpoints are public (Nicoly flow)
	// Pattern: /api/v1/slots/{id}/otp  and  /api/v1/slots/{id}/submit
	if strings.HasSuffix(path, "/otp") || strings.HasSuffix(path, "/submit") {
		return true
	}
	return false
}

func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicRoute(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			}, jwt.WithExpirationRequired())
			if err != nil || !token.Valid {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, token.Claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves JWT claims from a request context.
func ClaimsFromContext(ctx context.Context) (jwt.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(jwt.Claims)
	return c, ok
}
