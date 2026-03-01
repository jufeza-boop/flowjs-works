package secrets

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Minimal in-memory mock for SecretDB
// ---------------------------------------------------------------------------

type row struct {
	id           string
	name         string
	stype        string
	encryptedVal []byte
	metadata     []byte
}

type mockDB struct {
	rows   []row
	execOK bool
}

func newMockDB() *mockDB { return &mockDB{execOK: true} }

func (m *mockDB) ExecContext(_ context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !m.execOK {
		return nil, assert.AnError
	}
	// Simple upsert simulation
	if len(args) >= 5 {
		id := args[0].(string)
		for i, r := range m.rows {
			if r.id == id {
				m.rows[i] = row{
					id:           id,
					name:         args[1].(string),
					stype:        args[2].(string),
					encryptedVal: args[3].([]byte),
					metadata:     []byte(args[4].(string)),
				}
				return nil, nil
			}
		}
		m.rows = append(m.rows, row{
			id:           id,
			name:         args[1].(string),
			stype:        args[2].(string),
			encryptedVal: args[3].([]byte),
			metadata:     []byte(args[4].(string)),
		})
	} else if len(args) == 1 {
		// DELETE
		id := args[0].(string)
		updated := m.rows[:0]
		for _, r := range m.rows {
			if r.id != id {
				updated = append(updated, r)
			}
		}
		m.rows = updated
	}
	return nil, nil
}

func (m *mockDB) QueryContext(_ context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// We cannot easily construct real *sql.Rows without a real DB.
	// Tests that need rows use the SecretStore methods via the encrypt/decrypt helpers
	// directly rather than the DB scanning path.
	return nil, nil
}

// ---------------------------------------------------------------------------
// AES-256-GCM encryption round-trip
// ---------------------------------------------------------------------------

func newTestStore(t *testing.T) *SecretStore {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	s, err := NewSecretStore(newMockDB(), key)
	require.NoError(t, err)
	return s
}

func TestNewSecretStore_InvalidKeyLength(t *testing.T) {
	_, err := NewSecretStore(newMockDB(), []byte("short"))
	assert.ErrorContains(t, err, "32 bytes")
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	plaintext := []byte(`{"username":"admin","password":"s3cr3t"}`)

	ciphertext, err := s.encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	recovered, err := s.decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, recovered)
}

func TestEncryptDecrypt_DifferentNonceEachTime(t *testing.T) {
	s := newTestStore(t)
	plain := []byte("same plaintext")

	ct1, err := s.encrypt(plain)
	require.NoError(t, err)
	ct2, err := s.encrypt(plain)
	require.NoError(t, err)
	// Each encryption must produce a different ciphertext (different nonce)
	assert.NotEqual(t, ct1, ct2)
}

func TestDecrypt_TruncatedData(t *testing.T) {
	s := newTestStore(t)
	_, err := s.decrypt([]byte("short"))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Upsert / List / Delete integration through encrypt helpers
// ---------------------------------------------------------------------------

func TestUpsert_MissingID(t *testing.T) {
	s := newTestStore(t)
	err := s.Upsert(context.Background(), SecretInput{Name: "x", Type: SecretTypeToken, Value: map[string]interface{}{"token": "abc"}})
	assert.ErrorContains(t, err, "id is required")
}

func TestUpsert_MissingName(t *testing.T) {
	s := newTestStore(t)
	err := s.Upsert(context.Background(), SecretInput{ID: "sec_1", Type: SecretTypeToken, Value: map[string]interface{}{"token": "abc"}})
	assert.ErrorContains(t, err, "name is required")
}

func TestUpsert_NilValue(t *testing.T) {
	s := newTestStore(t)
	// nil map marshals to JSON "null" — should not error
	err := s.Upsert(context.Background(), SecretInput{ID: "sec_1", Name: "x", Type: SecretTypeToken, Value: nil})
	require.NoError(t, err)
}

func TestUpsert_StoresEncryptedValue(t *testing.T) {
	mdb := newMockDB()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	s, err := NewSecretStore(mdb, key)
	require.NoError(t, err)

	payload := map[string]interface{}{"username": "admin", "password": "p@ss"}
	err = s.Upsert(context.Background(), SecretInput{
		ID: "sec_pg", Name: "Postgres", Type: SecretTypeConnectionString, Value: payload,
	})
	require.NoError(t, err)
	require.Len(t, mdb.rows, 1)

	// Encrypted value must not contain the plaintext password
	plainJSON, _ := json.Marshal(payload)
	assert.NotContains(t, string(mdb.rows[0].encryptedVal), string(plainJSON))

	// But we can decrypt it back
	recovered, err := s.decrypt(mdb.rows[0].encryptedVal)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(recovered, &got))
	assert.Equal(t, "admin", got["username"])
	assert.Equal(t, "p@ss", got["password"])
}

// ---------------------------------------------------------------------------
// SecretType constants
// ---------------------------------------------------------------------------

func TestSecretTypeConstants(t *testing.T) {
	assert.Equal(t, SecretType("basic_auth"), SecretTypeBasicAuth)
	assert.Equal(t, SecretType("token"), SecretTypeToken)
	assert.Equal(t, SecretType("certificate"), SecretTypeCertificate)
	assert.Equal(t, SecretType("connection_string"), SecretTypeConnectionString)
	assert.Equal(t, SecretType("aws_credentials"), SecretTypeAWSCredentials)
	assert.Equal(t, SecretType("ssh_key"), SecretTypeSSHKey)
	assert.Equal(t, SecretType("amqp_url"), SecretTypeAMQPURL)
}

// ---------------------------------------------------------------------------
// NoopResolver (existing, must still pass)
// ---------------------------------------------------------------------------

func TestNoopResolver_AlwaysEmpty(t *testing.T) {
	r := &NoopResolver{}
	result, err := r.Resolve(context.Background(), "any-ref")
	require.NoError(t, err)
	assert.Empty(t, result)
}
