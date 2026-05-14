package keymgr

import (
	"context"
	"encoding/base64"
)

// MockKeyManager is an identity key manager for tests — no real crypto.
type MockKeyManager struct{}

func (m *MockKeyManager) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return []byte(base64.StdEncoding.EncodeToString(plaintext)), nil
}

func (m *MockKeyManager) Decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(ciphertext))
}
