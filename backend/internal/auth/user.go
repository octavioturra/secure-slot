package auth

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Email       string
	Domain      string
	DisplayName string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

type PasskeyCredential struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	CredentialID string
	PublicKey    []byte
	SignCount    uint32
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}
