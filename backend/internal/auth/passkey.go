package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/octavioturra/secure-slot/backend/pkg/response"
	"github.com/redis/go-redis/v9"
)

type PasskeyHandler struct {
	webauthn *webauthn.WebAuthn
	users    UserRepository
	passkeys PasskeyRepository
	redis    *redis.Client
}

func NewPasskeyHandler(appURL string, users UserRepository, passkeys PasskeyRepository, redisClient *redis.Client) (*PasskeyHandler, error) {
	u, err := url.Parse(appURL)
	if err != nil {
		return nil, fmt.Errorf("parse app url: %w", err)
	}
	rpID := u.Hostname()
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "SecureSlot",
		RPID:          rpID,
		RPOrigins:     []string{origin},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	return &PasskeyHandler{
		webauthn: wa,
		users:    users,
		passkeys: passkeys,
		redis:    redisClient,
	}, nil
}

// RegisterBegin starts the passkey registration ceremony. Requires JWT.
func (h *PasskeyHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	creds, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load credentials")
		return
	}

	options, session, err := h.webauthn.BeginRegistration(toWebAuthnUser(user, creds))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to begin registration")
		return
	}

	if err := h.storeSession(r, fmt.Sprintf("passkey:reg:%s", user.ID), session); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store session")
		return
	}

	response.OK(w, options)
}

// RegisterComplete finishes the passkey registration ceremony. Requires JWT.
func (h *PasskeyHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	sessionKey := fmt.Sprintf("passkey:reg:%s", user.ID)
	session, err := h.loadSession(r, sessionKey)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_SESSION", "session expired or invalid")
		return
	}

	creds, _ := h.passkeys.GetByUserID(r.Context(), user.ID)
	credential, err := h.webauthn.FinishRegistration(toWebAuthnUser(user, creds), *session, r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "REGISTRATION_FAILED", "failed to finish registration")
		return
	}

	if err := h.redis.Del(r.Context(), sessionKey).Err(); err != nil {
		// non-fatal: session will expire on its own
	}

	newCred := PasskeyCredential{
		ID:           uuid.New(),
		UserID:       user.ID,
		CredentialID: base64.RawURLEncoding.EncodeToString(credential.ID),
		PublicKey:    credential.PublicKey,
		SignCount:    credential.Authenticator.SignCount,
		CreatedAt:    time.Now(),
	}
	if err := h.passkeys.Create(r.Context(), newCred); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store credential")
		return
	}

	response.OK(w, map[string]string{"status": "ok"})
}

// AuthBegin starts the passkey authentication ceremony. Requires JWT.
func (h *PasskeyHandler) AuthBegin(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	creds, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load credentials")
		return
	}

	options, session, err := h.webauthn.BeginLogin(toWebAuthnUser(user, creds))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to begin authentication")
		return
	}

	if err := h.storeSession(r, fmt.Sprintf("passkey:auth:%s", user.ID), session); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store session")
		return
	}

	response.OK(w, options)
}

// AuthComplete finishes the passkey authentication ceremony. Requires JWT.
func (h *PasskeyHandler) AuthComplete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	sessionKey := fmt.Sprintf("passkey:auth:%s", user.ID)
	session, err := h.loadSession(r, sessionKey)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_SESSION", "session expired or invalid")
		return
	}

	creds, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load credentials")
		return
	}

	credential, err := h.webauthn.FinishLogin(toWebAuthnUser(user, creds), *session, r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "AUTH_FAILED", "passkey authentication failed")
		return
	}

	_ = h.redis.Del(r.Context(), sessionKey)

	credID := base64.RawURLEncoding.EncodeToString(credential.ID)
	if err := h.passkeys.UpdateSignCount(r.Context(), credID, credential.Authenticator.SignCount); err != nil {
		// non-fatal but log-worthy in production
	}

	response.OK(w, map[string]string{"verified": "true"})
}

func (h *PasskeyHandler) storeSession(r *http.Request, key string, session *webauthn.SessionData) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return h.redis.Set(r.Context(), key, data, 5*time.Minute).Err()
}

func (h *PasskeyHandler) loadSession(r *http.Request, key string) (*webauthn.SessionData, error) {
	data, err := h.redis.Get(r.Context(), key).Bytes()
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// webAuthnUser adapts our User + credentials to the webauthn.User interface.
type webAuthnUser struct {
	user        *User
	credentials []webauthn.Credential
}

func toWebAuthnUser(user *User, creds []PasskeyCredential) *webAuthnUser {
	waCreds := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		id, err := base64.RawURLEncoding.DecodeString(c.CredentialID)
		if err != nil {
			continue
		}
		waCreds = append(waCreds, webauthn.Credential{
			ID:        id,
			PublicKey: c.PublicKey,
			Authenticator: webauthn.Authenticator{
				SignCount: c.SignCount,
			},
		})
	}
	return &webAuthnUser{user: user, credentials: waCreds}
}

func (u *webAuthnUser) WebAuthnID() []byte {
	return u.user.ID[:]
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.user.Email
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	if u.user.DisplayName != "" {
		return u.user.DisplayName
	}
	return u.user.Email
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}
