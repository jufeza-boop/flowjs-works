// Package secrets provides the SecretResolver interface, built-in implementations,
// and a DB-backed secret store with AES-256-GCM encryption.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// SecretType enumerates supported credential categories.
type SecretType string

const (
	SecretTypeBasicAuth        SecretType = "basic_auth"
	SecretTypeToken            SecretType = "token"
	SecretTypeCertificate      SecretType = "certificate"
	SecretTypeConnectionString SecretType = "connection_string"
)

// SecretMeta contains non-sensitive metadata returned by List.
type SecretMeta struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      SecretType `json:"type"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// SecretInput is the payload used to create or update a secret.
type SecretInput struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     SecretType             `json:"type"`
	Value    map[string]interface{} `json:"value"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SecretStore persists secrets encrypted with AES-256-GCM and exposes
// CRUD operations plus the SecretResolver interface for the engine.
type SecretStore struct {
	db  SecretDB
	key []byte // 32-byte AES-256 key
}

// SecretDB is the minimal DB interface required by SecretStore (allows mocking).
type SecretDB interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// NewSecretStore creates a SecretStore backed by the provided DB connection and
// 32-byte AES-256 key. Returns an error if the key length is wrong.
func NewSecretStore(db SecretDB, key []byte) (*SecretStore, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secrets: AES key must be exactly 32 bytes, got %d", len(key))
	}
	return &SecretStore{db: db, key: key}, nil
}

// ---------------------------------------------------------------------------
// CRUD operations
// ---------------------------------------------------------------------------

// Upsert creates or updates a secret. The value is AES-256-GCM encrypted before
// being stored. Secrets must never appear in audit logs.
func (s *SecretStore) Upsert(ctx context.Context, input SecretInput) error {
	if input.ID == "" {
		return fmt.Errorf("secrets: id is required")
	}
	if input.Name == "" {
		return fmt.Errorf("secrets: name is required")
	}

	plain, err := json.Marshal(input.Value)
	if err != nil {
		return fmt.Errorf("secrets: marshal value: %w", err)
	}

	ciphertext, err := s.encrypt(plain)
	if err != nil {
		return fmt.Errorf("secrets: encrypt: %w", err)
	}

	metaJSON, err := json.Marshal(input.Metadata)
	if err != nil {
		return fmt.Errorf("secrets: marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO secrets (id, name, type, encrypted_val, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		  SET name          = EXCLUDED.name,
		      type          = EXCLUDED.type,
		      encrypted_val = EXCLUDED.encrypted_val,
		      metadata      = EXCLUDED.metadata,
		      updated_at    = NOW()
	`, input.ID, input.Name, string(input.Type), ciphertext, string(metaJSON))
	if err != nil {
		return fmt.Errorf("secrets: upsert %s: %w", input.ID, err)
	}
	return nil
}

// List returns metadata for all secrets; the encrypted value is never exposed.
func (s *SecretStore) List(ctx context.Context) ([]SecretMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, created_at, updated_at FROM secrets ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("secrets: list: %w", err)
	}
	defer rows.Close()

	var results []SecretMeta
	for rows.Next() {
		var m SecretMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("secrets: scan row: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("secrets: rows error: %w", err)
	}
	return results, nil
}

// Delete removes a secret by ID. Returns nil when the secret does not exist.
func (s *SecretStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("secrets: delete %s: %w", id, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// SecretResolver implementation
// ---------------------------------------------------------------------------

// Resolve implements the SecretResolver interface. It fetches and decrypts the
// secret identified by ref, returning its key/value pairs for config injection.
// Secrets must never appear in audit logs.
func (s *SecretStore) Resolve(ctx context.Context, ref string) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT encrypted_val FROM secrets WHERE id = $1`, ref)
	if err != nil {
		return nil, fmt.Errorf("secrets: resolve %s: %w", ref, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("secrets: resolve %s: %w", ref, err)
		}
		return nil, fmt.Errorf("secrets: secret not found: %s", ref)
	}

	var ciphertext []byte
	if err := rows.Scan(&ciphertext); err != nil {
		return nil, fmt.Errorf("secrets: scan ciphertext: %w", err)
	}

	plain, err := s.decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt %s: %w", ref, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(plain, &result); err != nil {
		return nil, fmt.Errorf("secrets: unmarshal decrypted value: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// AES-256-GCM helpers
// ---------------------------------------------------------------------------

// encrypt encrypts plaintext using AES-256-GCM. The nonce is prepended to the
// ciphertext so that decrypt can extract it without extra storage.
func (s *SecretStore) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt reverses encrypt. It expects the nonce to be prepended to the ciphertext.
func (s *SecretStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
