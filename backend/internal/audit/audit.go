package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const (
	ActionSlotCreated    = "slot.created"
	ActionSlotOpened     = "slot.opened"
	ActionSlotDeleted    = "slot.deleted"
	ActionSubmitReceived = "submit.received"
	ActionOTPRequested   = "otp.requested"
)

type AuditEntry struct {
	ID        uuid.UUID
	Action    string
	SlotID    *uuid.UUID
	Actor     string
	IP        string
	UserAgent string
	Timestamp time.Time
}

type Logger interface {
	Log(ctx context.Context, entry AuditEntry) error
}
