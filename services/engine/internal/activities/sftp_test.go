package activities

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSFTPActivity_Name verifies the activity type identifier.
func TestSFTPActivity_Name(t *testing.T) {
	a := &SFTPActivity{}
	assert.Equal(t, "sftp", a.Name())
}

// TestSFTPActivity_MissingServer ensures an error is returned when 'server' is absent.
func TestSFTPActivity_MissingServer(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"method": "get",
		"folder": "/files",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server")
}

// TestSFTPActivity_MissingMethod ensures an error is returned when 'method' is absent.
func TestSFTPActivity_MissingMethod(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "sftp.example.com",
		"folder": "/files",
		"auth":   map[string]interface{}{"user": "u", "password": "p"},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestSFTPActivity_InvalidMethod ensures a descriptive error for unknown method values.
func TestSFTPActivity_InvalidMethod(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "sftp.example.com",
		"folder": "/files",
		"method": "delete",
		"auth":   map[string]interface{}{"user": "u", "password": "p"},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestSFTPActivity_MissingFolder ensures an error is returned when 'folder' is absent.
func TestSFTPActivity_MissingFolder(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "sftp.example.com",
		"method": "get",
		"auth":   map[string]interface{}{"user": "u", "password": "p"},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "folder")
}

// TestSFTPActivity_MissingAuth ensures an error is returned when no auth method is provided.
func TestSFTPActivity_MissingAuth(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "sftp.example.com",
		"method": "get",
		"folder": "/files",
		// no auth
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth")
}

// TestSFTPActivity_InvalidRegex ensures malformed regex_filter is rejected.
func TestSFTPActivity_InvalidRegex(t *testing.T) {
	a := &SFTPActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server":       "sftp.example.com",
		"method":       "get",
		"folder":       "/files",
		"regex_filter": "[invalid",
		"auth":         map[string]interface{}{"user": "u", "password": "p"},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regex_filter")
}

// TestSFTPActivity_IntegrationGet is an integration test skipped unless
// FLOWJS_RUN_EXTERNAL_TESTS=1 is set.
func TestSFTPActivity_IntegrationGet(t *testing.T) {
	if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
		t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
	}
	a := &SFTPActivity{}
	out, err := a.Execute(nil, map[string]interface{}{
		"server": "sftp.example.com",
		"port":   22,
		"method": "get",
		"folder": "/upload",
		"auth":   map[string]interface{}{"user": "demo", "password": "demo"},
	}, nil)
	require.NoError(t, err)
	assert.NotNil(t, out["files_downloaded"])
}
