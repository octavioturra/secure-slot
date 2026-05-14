package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"sub"`
	Email  string    `json:"email"`
	Domain string    `json:"hd"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret        []byte
	expiryMinutes int
}

func NewJWTService(secret string, expiryMinutes int) *JWTService {
	return &JWTService{secret: []byte(secret), expiryMinutes: expiryMinutes}
}

func (s *JWTService) Issue(user User) (string, time.Time, error) {
	expiresAt := time.Now().Add(time.Duration(s.expiryMinutes) * time.Minute)
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Domain: user.Domain,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *JWTService) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("unexpected claims type")
	}
	return claims, nil
}

type contextKey string

const claimsContextKey contextKey = "auth_claims"

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsContextKey).(*Claims)
	return c, ok
}
