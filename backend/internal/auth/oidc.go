package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type IDTokenClaims struct {
	Email       string `json:"email"`
	Domain      string `json:"hd"`
	DisplayName string `json:"name"`
}

type OIDCClient struct {
	provider      *oidc.Provider
	config        oauth2.Config
	verifier      *oidc.IDTokenVerifier
	allowedDomain string
}

// NewOIDCClient discovers the provider via Keycloak's well-known endpoint.
// issuerURL = keycloakURL + "/realms/" + realm
func NewOIDCClient(ctx context.Context, keycloakURL, realm, clientID, clientSecret, redirectURL, allowedDomain string) (*OIDCClient, error) {
	issuerURL := fmt.Sprintf("%s/realms/%s", keycloakURL, realm)
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	return &OIDCClient{
		provider:      provider,
		config:        cfg,
		verifier:      verifier,
		allowedDomain: allowedDomain,
	}, nil
}

func (c *OIDCClient) AuthCodeURL(state string) string {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange trades the authorization code for an ID token and extracts claims.
func (c *OIDCClient) Exchange(ctx context.Context, code string) (*IDTokenClaims, error) {
	oauthToken, err := c.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("missing id_token")
	}
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}
	var claims IDTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	// Only validate hosted domain if GOOGLE_ALLOWED_DOMAIN is configured
	if c.allowedDomain != "" && claims.Domain != c.allowedDomain {
		return nil, fmt.Errorf("unauthorized domain: %s", claims.Domain)
	}
	return &claims, nil
}
