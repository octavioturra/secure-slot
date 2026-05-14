package slot

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SlotStatus string

const (
	SlotPending SlotStatus = "pending"
	SlotFilled  SlotStatus = "filled"
	SlotOpened  SlotStatus = "opened"
	SlotExpired SlotStatus = "expired"
)

type Slot struct {
	ID                uuid.UUID
	OwnerID           uuid.UUID
	Label             string
	Status            SlotStatus
	RecipientEmail    string
	PublicKey         string
	WrappedPrivateKey string
	WrapIV            string
	CredentialID      string
	EncryptedAESKey   *string
	PayloadIV         *string
	Ciphertext        *string
	VaultKeyID        *string
	TTLHours          int
	ExpiresAt         time.Time
	CreatedAt         time.Time
	SubmittedAt       *time.Time
	OpenedAt          *time.Time
	DeletedAt         *time.Time
}

type Repository interface {
	Create(ctx context.Context, slot Slot) error
	Get(ctx context.Context, id uuid.UUID) (*Slot, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]Slot, error)
}
