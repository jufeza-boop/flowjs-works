package activities

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPActivity_Name(t *testing.T) {
	a := &HTTPActivity{}
	assert.Equal(t, "http", a.Name())
}

func TestHTTPActivity_MissingURL(t *testing.T) {
	a := &HTTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

// TestHTTPActivity_TokenSecretInjectsBearer verifies that a token secret injected
// into config via secret_ref sets the Authorization: Bearer header.
func TestHTTPActivity_TokenSecretInjectsBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &HTTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"url":   srv.URL,
		"token": "my-secret-token",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-secret-token", gotAuth)
}

// TestHTTPActivity_BasicAuthSecretInjectsHeader verifies that user+password from a
// basic_auth secret injected into config sets the Authorization: Basic header.
func TestHTTPActivity_BasicAuthSecretInjectsHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &HTTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"url":      srv.URL,
		"user":     "alice",
		"password": "s3cr3t",
	}, nil)
	require.NoError(t, err)
	assert.Contains(t, gotAuth, "Basic ")
}

// TestHTTPActivity_ExplicitAuthHeaderOverridesSecret verifies that an Authorization
// header provided in config["headers"] takes priority over a token secret.
func TestHTTPActivity_ExplicitAuthHeaderOverridesSecret(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := &HTTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"url":   srv.URL,
		"token": "injected-token",
		"headers": map[string]interface{}{
			"Authorization": "Bearer explicit-token",
		},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer explicit-token", gotAuth)
}
