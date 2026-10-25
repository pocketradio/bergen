package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pocketradio/bergen/internal/domain"
	"github.com/pocketradio/bergen/internal/ports"
)

// TestClientAuthStorePersistsAndRevokesCredentials verifies user ownership, credential lookup, overlap, and safe revocation retries in PostgreSQL.
func TestClientAuthStorePersistsAndRevokesCredentials(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientStore, pool := newClientAuthTestStore(t, ctx)
	userID := newClientAuthTestUserID(t)
	otherUserID := newClientAuthTestUserID(t)
	credentialID := newClientAuthTestID(t)
	secondCredentialID := newClientAuthTestID(t)
	firstDigest := ClientCredentialDigest(sha256.Sum256([]byte("first synthetic credential for persistence test")))
	secondDigest := ClientCredentialDigest(sha256.Sum256([]byte("second synthetic credential for persistence test")))
	cleanupClientAuthTestUsers(t, pool, userID, otherUserID)

	if err := clientStore.CreateUser(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if err := clientStore.CreateUser(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	if err := clientStore.CreateClientCredential(ctx, credentialID, userID, firstDigest); err != nil {
		t.Fatal(err)
	}
	if err := clientStore.CreateClientCredential(ctx, secondCredentialID, userID, secondDigest); err != nil {
		t.Fatalf("allow a second active credential for rotation: %v", err)
	}

	record, err := clientStore.FindClientCredentialByDigest(ctx, firstDigest)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != credentialID || record.UserID != userID || record.RevokedAt != nil || record.CreatedAt.IsZero() {
		t.Fatalf("unexpected credential record: %+v", record)
	}

	if err := clientStore.RevokeClientCredential(ctx, credentialID, otherUserID, time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)); !errors.Is(err, ErrClientCredentialNotFound) {
		t.Fatalf("revoking another user's credential error = %v, want not found", err)
	}

	firstRevocation := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	secondRevocation := firstRevocation.Add(time.Hour)
	if err := clientStore.RevokeClientCredential(ctx, credentialID, userID, firstRevocation); err != nil {
		t.Fatal(err)
	}
	if err := clientStore.RevokeClientCredential(ctx, credentialID, userID, secondRevocation); err != nil {
		t.Fatalf("repeated revocation should be safe: %v", err)
	}

	revoked, err := clientStore.FindClientCredentialByDigest(ctx, firstDigest)
	if err != nil {
		t.Fatal(err)
	}
	if revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(firstRevocation) {
		t.Fatalf("revocation time = %v, want original time %v", revoked.RevokedAt, firstRevocation)
	}

	missingDigest := ClientCredentialDigest(sha256.Sum256([]byte("missing synthetic credential")))
	if _, err := clientStore.FindClientCredentialByDigest(ctx, missingDigest); !errors.Is(err, ErrClientCredentialNotFound) {
		t.Fatalf("missing credential error = %v, want not found", err)
	}
}

// TestClientAuthStoreEnforcesCredentialConstraints verifies PostgreSQL rejects duplicate digests and orphan credentials.
func TestClientAuthStoreEnforcesCredentialConstraints(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientStore, pool := newClientAuthTestStore(t, ctx)
	userID := newClientAuthTestUserID(t)
	cleanupClientAuthTestUsers(t, pool, userID)
	if err := clientStore.CreateUser(ctx, userID); err != nil {
		t.Fatal(err)
	}

	digest := ClientCredentialDigest(sha256.Sum256([]byte("unique synthetic credential for constraints test")))
	if err := clientStore.CreateClientCredential(ctx, newClientAuthTestID(t), userID, digest); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		userID domain.UserID
		digest ClientCredentialDigest
		code   string
	}{
		{name: "duplicate digest", userID: userID, digest: digest, code: "23505"},
		{name: "missing owner", userID: newClientAuthTestUserID(t), digest: ClientCredentialDigest(sha256.Sum256([]byte("orphan synthetic credential"))), code: "23503"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := clientStore.CreateClientCredential(ctx, newClientAuthTestID(t), test.userID, test.digest)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != test.code {
				t.Fatalf("insert error = %v, want PostgreSQL code %s", err, test.code)
			}
		})
	}
}

// newClientAuthTestStore migrates the configured PostgreSQL test database and returns a store and pool.
func newClientAuthTestStore(t *testing.T, ctx context.Context) (*ClientAuthStore, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("BERGEN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set BERGEN_TEST_DATABASE_URL for PostgreSQL integration")
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	return NewClientAuthStore(pool), pool
}

// newClientAuthTestID creates a collision-resistant opaque test identifier.
func newClientAuthTestID(t *testing.T) domain.ID {
	t.Helper()
	id, err := (ports.RandomIDGen{}).NewID()
	if err != nil {
		t.Fatal(err)
	}
	return domain.ID(id)
}

// newClientAuthTestUserID creates a collision-resistant opaque user identifier for PostgreSQL tests.
func newClientAuthTestUserID(t *testing.T) domain.UserID {
	return domain.UserID(newClientAuthTestID(t))
}

// cleanupClientAuthTestUsers removes only the uniquely identified rows created by this PostgreSQL test.
func cleanupClientAuthTestUsers(t *testing.T, pool *pgxpool.Pool, userIDs ...domain.UserID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, userID := range userIDs {
			if _, err := pool.Exec(ctx, `DELETE FROM bergen.client_credentials WHERE user_id = $1`, userID); err != nil {
				t.Errorf("remove test credentials for %s: %v", userID, err)
			}
			if _, err := pool.Exec(ctx, `DELETE FROM bergen.users WHERE id = $1`, userID); err != nil {
				t.Errorf("remove test user %s: %v", userID, err)
			}
		}
	})
}
