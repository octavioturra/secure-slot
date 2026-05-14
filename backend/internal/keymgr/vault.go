package keymgr

import (
	"context"
	"encoding/base64"
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

type VaultKeyManager struct {
	client  *vault.Client
	keyName string
}

func NewVaultKeyManager(addr, token, keyName string) (*VaultKeyManager, error) {
	cfg := vault.DefaultConfig()
	cfg.Address = addr

	client, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	client.SetToken(token)

	return &VaultKeyManager{client: client, keyName: keyName}, nil
}

func (v *VaultKeyManager) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString(plaintext)

	secret, err := v.client.Logical().WriteWithContext(ctx,
		fmt.Sprintf("transit/encrypt/%s", v.keyName),
		map[string]any{"plaintext": encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("vault encrypt: %w", err)
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("vault encrypt: unexpected response shape")
	}
	return []byte(ciphertext), nil
}

func (v *VaultKeyManager) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	secret, err := v.client.Logical().WriteWithContext(ctx,
		fmt.Sprintf("transit/decrypt/%s", v.keyName),
		map[string]any{"ciphertext": string(ciphertext)},
	)
	if err != nil {
		return nil, fmt.Errorf("vault decrypt: %w", err)
	}

	encoded, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("vault decrypt: unexpected response shape")
	}

	plaintext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("vault decrypt: base64 decode: %w", err)
	}
	return plaintext, nil
}
