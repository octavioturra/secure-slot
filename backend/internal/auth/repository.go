package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	Upsert(ctx context.Context, email, domain, displayName string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}

type PasskeyRepository interface {
	Create(ctx context.Context, cred PasskeyCredential) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]PasskeyCredential, error)
	GetByCredentialID(ctx context.Context, credID string) (*PasskeyCredential, error)
	UpdateSignCount(ctx context.Context, credID string, count uint32) error
}

type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) Upsert(ctx context.Context, email, domain, displayName string) (*User, error) {
	const q = `
		INSERT INTO users (email, domain, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO UPDATE
			SET last_login_at = NOW(),
			    display_name  = EXCLUDED.display_name
		RETURNING id, email, domain, display_name, created_at, last_login_at`

	var u User
	var dn *string
	err := r.pool.QueryRow(ctx, q, email, domain, displayName).
		Scan(&u.ID, &u.Email, &u.Domain, &dn, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	if dn != nil {
		u.DisplayName = *dn
	}
	return &u, nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `SELECT id, email, domain, display_name, created_at, last_login_at FROM users WHERE id = $1`
	var u User
	var dn *string
	err := r.pool.QueryRow(ctx, q, id).
		Scan(&u.ID, &u.Email, &u.Domain, &dn, &u.CreatedAt, &u.LastLoginAt)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if dn != nil {
		u.DisplayName = *dn
	}
	return &u, nil
}

type PostgresPasskeyRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPasskeyRepository(pool *pgxpool.Pool) *PostgresPasskeyRepository {
	return &PostgresPasskeyRepository{pool: pool}
}

func (r *PostgresPasskeyRepository) Create(ctx context.Context, cred PasskeyCredential) error {
	const q = `
		INSERT INTO passkey_credentials (id, user_id, credential_id, public_key, sign_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.pool.Exec(ctx, q,
		cred.ID, cred.UserID, cred.CredentialID, cred.PublicKey, cred.SignCount, cred.CreatedAt)
	if err != nil {
		return fmt.Errorf("create passkey: %w", err)
	}
	return nil
}

func (r *PostgresPasskeyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]PasskeyCredential, error) {
	const q = `SELECT id, user_id, credential_id, public_key, sign_count, created_at, last_used_at
		FROM passkey_credentials WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get passkeys by user: %w", err)
	}
	defer rows.Close()

	var creds []PasskeyCredential
	for rows.Next() {
		var c PasskeyCredential
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey, &c.SignCount, &c.CreatedAt, &c.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan passkey: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (r *PostgresPasskeyRepository) GetByCredentialID(ctx context.Context, credID string) (*PasskeyCredential, error) {
	const q = `SELECT id, user_id, credential_id, public_key, sign_count, created_at, last_used_at
		FROM passkey_credentials WHERE credential_id = $1`
	var c PasskeyCredential
	err := r.pool.QueryRow(ctx, q, credID).
		Scan(&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey, &c.SignCount, &c.CreatedAt, &c.LastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("get passkey by credential id: %w", err)
	}
	return &c, nil
}

func (r *PostgresPasskeyRepository) UpdateSignCount(ctx context.Context, credID string, count uint32) error {
	const q = `UPDATE passkey_credentials SET sign_count = $1, last_used_at = $2 WHERE credential_id = $3`
	_, err := r.pool.Exec(ctx, q, count, time.Now(), credID)
	if err != nil {
		return fmt.Errorf("update sign count: %w", err)
	}
	return nil
}
