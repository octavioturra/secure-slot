package auth

import (
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

// Login generates a random state, stores it in Redis with a 10-minute TTL, and
// redirects the browser to the Keycloak authorization endpoint.
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

// Callback verifies the OAuth2 state, exchanges the code for an ID token, upserts
// the user, and returns a JWT session token.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	key := fmt.Sprintf("oauth:state:%s", state)
	if err := h.redis.GetDel(r.Context(), key).Err(); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_STATE", "invalid or expired state parameter")
		return
	}

	idClaims, err := h.oidc.Exchange(r.Context(), code)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized domain") {
			response.Error(w, http.StatusForbidden, "FORBIDDEN", "unauthorized domain")
			return
		}
		response.Error(w, http.StatusBadRequest, "OIDC_ERROR", "authentication failed")
		return
	}

	user, err := h.users.Upsert(r.Context(), idClaims.Email, idClaims.Domain, idClaims.DisplayName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist user")
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

// Refresh issues a fresh JWT for the already-authenticated user (claims come from the JWT middleware).
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
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
