package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"github.com/octavioturra/secure-slot/backend/pkg/response"
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
		return nil, fmt.Errorf("parse appURL: %w", err)
	}
	rpid := u.Hostname()
	rpOrigin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "SecureSlot",
		RPID:          rpid,
		RPOrigins:     []string{rpOrigin},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn config: %w", err)
	}
	return &PasskeyHandler{webauthn: wa, users: users, passkeys: passkeys, redis: redisClient}, nil
}

// RegisterBegin starts the passkey registration ceremony. Requires a valid JWT.
func (h *PasskeyHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	existing, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load credentials")
		return
	}

	wa := newWebAuthnUser(user, existing)
	options, session, err := h.webauthn.BeginRegistration(wa)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "begin registration failed")
		return
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "session serialization failed")
		return
	}

	key := fmt.Sprintf("passkey:reg:%s", user.ID.String())
	if err := h.redis.Set(r.Context(), key, sessionJSON, 5*time.Minute).Err(); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store session")
		return
	}

	response.OK(w, options)
}

// RegisterComplete finalises the passkey registration ceremony. Requires a valid JWT.
func (h *PasskeyHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	key := fmt.Sprintf("passkey:reg:%s", user.ID.String())
	sessionJSON, err := h.redis.GetDel(r.Context(), key).Bytes()
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_SESSION", "registration session not found or expired")
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "session deserialization failed")
		return
	}

	wa := newWebAuthnUser(user, nil)
	credential, err := h.webauthn.FinishRegistration(wa, session, r)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "REGISTRATION_FAILED", "invalid registration response")
		return
	}

	cred := PasskeyCredential{
		UserID:       user.ID,
		CredentialID: base64.RawURLEncoding.EncodeToString(credential.ID),
		PublicKey:    credential.PublicKey,
		SignCount:    credential.Authenticator.SignCount,
	}
	if err := h.passkeys.Create(r.Context(), cred); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist credential")
		return
	}

	response.OK(w, map[string]string{"status": "ok"})
}

// AuthBegin starts the passkey authentication ceremony. Requires a valid JWT.
func (h *PasskeyHandler) AuthBegin(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	existing, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load credentials")
		return
	}

	descriptors := make([]protocol.CredentialDescriptor, len(existing))
	for i, c := range existing {
		rawID, _ := base64.RawURLEncoding.DecodeString(c.CredentialID)
		descriptors[i] = protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: rawID,
		}
	}

	wa := newWebAuthnUser(user, existing)
	options, session, err := h.webauthn.BeginLogin(wa, webauthn.WithAllowedCredentials(descriptors))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "begin authentication failed")
		return
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "session serialization failed")
		return
	}

	key := fmt.Sprintf("passkey:auth:%s", user.ID.String())
	if err := h.redis.Set(r.Context(), key, sessionJSON, 5*time.Minute).Err(); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store session")
		return
	}

	response.OK(w, options)
}

// AuthComplete finalises the passkey authentication ceremony. Requires a valid JWT.
func (h *PasskeyHandler) AuthComplete(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing claims")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user not found")
		return
	}

	key := fmt.Sprintf("passkey:auth:%s", user.ID.String())
	sessionJSON, err := h.redis.GetDel(r.Context(), key).Bytes()
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_SESSION", "authentication session not found or expired")
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionJSON, &session); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "session deserialization failed")
		return
	}

	existing, err := h.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load credentials")
		return
	}

	wa := newWebAuthnUser(user, existing)
	credential, err := h.webauthn.FinishLogin(wa, session, r)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "AUTH_FAILED", "passkey verification failed")
		return
	}

	credID := base64.RawURLEncoding.EncodeToString(credential.ID)
	if err := h.passkeys.UpdateSignCount(r.Context(), credID, credential.Authenticator.SignCount); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update sign count")
		return
	}

	response.OK(w, map[string]string{"verified": "true"})
}

// webAuthnUser adapts our User type to the webauthn.User interface.
type webAuthnUser struct {
	user        *User
	credentials []webauthn.Credential
}

func newWebAuthnUser(user *User, stored []PasskeyCredential) *webAuthnUser {
	creds := make([]webauthn.Credential, 0, len(stored))
	for _, c := range stored {
		rawID, _ := base64.RawURLEncoding.DecodeString(c.CredentialID)
		creds = append(creds, webauthn.Credential{
			ID:        rawID,
			PublicKey: c.PublicKey,
			Authenticator: webauthn.Authenticator{
				SignCount: c.SignCount,
			},
		})
	}
	return &webAuthnUser{user: user, credentials: creds}
}

func (u *webAuthnUser) WebAuthnID() []byte              { return u.user.ID[:] }
func (u *webAuthnUser) WebAuthnName() string             { return u.user.Email }
func (u *webAuthnUser) WebAuthnDisplayName() string      { return u.user.DisplayName }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
