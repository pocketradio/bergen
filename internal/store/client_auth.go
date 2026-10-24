package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pocketradio/bergen/internal/domain"
)

// ErrClientCredentialNotFound indicates that a credential is absent or not owned by the requested user.
var ErrClientCredentialNotFound = errors.New("client credential not found")

// ClientCredentialRecord contains the persisted identity and revocation state needed for authentication.
type ClientCredentialRecord struct {
	ID        domain.ID
	UserID    domain.UserID
	CreatedAt time.Time
	RevokedAt *time.Time
}

// ClientCredentialDigest is the fixed-size SHA-256 verifier persisted instead of a usable client secret.
type ClientCredentialDigest [32]byte

// ClientAuthStore persists client users and credentials in PostgreSQL.
type ClientAuthStore struct {
	pool *pgxpool.Pool
}

// NewClientAuthStore creates a client-authentication store backed by the supplied PostgreSQL pool.
func NewClientAuthStore(pool *pgxpool.Pool) *ClientAuthStore {
	return &ClientAuthStore{pool: pool}
}

// CreateUser inserts a user with the supplied opaque ID.
func (s *ClientAuthStore) CreateUser(ctx context.Context, id domain.UserID) error {
	if id == "" {
		return errors.New("create user: ID is required")
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO bergen.users (id) VALUES ($1)`, id); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// CreateClientCredential inserts a credential digest for its owning user without accepting the usable secret.
func (s *ClientAuthStore) CreateClientCredential(ctx context.Context, id domain.ID, userID domain.UserID, digest ClientCredentialDigest) error {
	if id == "" || userID == "" {
		return errors.New("create client credential: credential ID and user ID are required")
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO bergen.client_credentials (id, user_id, credential_digest)
		VALUES ($1, $2, $3)
	`, id, userID, digest[:]); err != nil {
		return fmt.Errorf("create client credential: %w", err)
	}
	return nil
}

// FindClientCredentialByDigest returns the matching credential's owner and revocation state.
func (s *ClientAuthStore) FindClientCredentialByDigest(ctx context.Context, digest ClientCredentialDigest) (ClientCredentialRecord, error) {
	var record ClientCredentialRecord
	var revokedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, created_at, revoked_at
		FROM bergen.client_credentials
		WHERE credential_digest = $1
	`, digest[:]).Scan(&record.ID, &record.UserID, &record.CreatedAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClientCredentialRecord{}, ErrClientCredentialNotFound
	}
	if err != nil {
		return ClientCredentialRecord{}, fmt.Errorf("find client credential: %w", err)
	}
	if revokedAt.Valid {
		revoked := revokedAt.Time
		record.RevokedAt = &revoked
	}
	return record, nil
}

// RevokeClientCredential marks an owner's credential revoked and preserves its first revocation time on retries.
func (s *ClientAuthStore) RevokeClientCredential(ctx context.Context, id domain.ID, userID domain.UserID, revokedAt time.Time) error {
	if id == "" || userID == "" || revokedAt.IsZero() {
		return errors.New("revoke client credential: credential ID, user ID, and revocation time are required")
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE bergen.client_credentials
		SET revoked_at = COALESCE(revoked_at, $3)
		WHERE id = $1 AND user_id = $2
		RETURNING id
	`, id, userID, revokedAt.UTC()).Scan(new(string))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrClientCredentialNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke client credential: %w", err)
	}
	return nil
}
