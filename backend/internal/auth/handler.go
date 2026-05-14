package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/octavioturra/secure-slot/backend/pkg/response"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	oidc  *OIDCClient
	jwt   *JWTService
	users UserRepository
	redis *redis.Client
}

func NewHandler(oidc *OIDCClient, jwt *JWTService, users UserRepository, redis *redis.Client) *Handler {
	return &Handler{oidc: oidc, jwt: jwt, users: users, redis: redis}
}

// Login generates a random state, stores it in Redis, and redirects to the OIDC provider.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate state")
		return
	}
	state := hex.EncodeToString(b)

	key := fmt.Sprintf("oauth:state:%s", state)
	if err := h.redis.Set(r.Context(), key, "1", 10*time.Minute).Err(); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store state")
		return
	}

	http.Redirect(w, r, h.oidc.AuthCodeURL(state), http.StatusFound)
}

// Callback verifies the OAuth2 state, exchanges the code, upserts the user, and returns a JWT.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	key := fmt.Sprintf("oauth:state:%s", state)
	n, err := h.redis.Del(r.Context(), key).Result()
	if err != nil || n == 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_STATE", "invalid or expired state")
		return
	}

	claims, err := h.oidc.Exchange(r.Context(), code)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized domain") {
			response.Error(w, http.StatusForbidden, "FORBIDDEN", "unauthorized domain")
			return
		}
		response.Error(w, http.StatusBadRequest, "OAUTH_ERROR", "authentication failed")
		return
	}

	user, err := h.users.Upsert(r.Context(), claims.Email, claims.Domain, claims.DisplayName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to provision user")
		return
	}

	token, expiresAt, err := h.jwt.Issue(*user)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue token")
		return
	}

	response.OK(w, map[string]any{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

// Refresh validates the current Bearer JWT and issues a new one.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header")
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := h.jwt.Validate(tokenStr)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
		return
	}

	user, err := h.users.GetByID(context.Background(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not found")
		return
	}

	token, expiresAt, err := h.jwt.Issue(*user)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue token")
		return
	}

	response.OK(w, map[string]any{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}
