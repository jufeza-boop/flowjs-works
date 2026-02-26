package activities

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSMBActivity_Name verifies the activity type identifier.
func TestSMBActivity_Name(t *testing.T) {
	a := &SMBActivity{}
	assert.Equal(t, "smb", a.Name())
}

// TestSMBActivity_MissingServer ensures an error when 'server' is absent.
func TestSMBActivity_MissingServer(t *testing.T) {
	a := &SMBActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"share":  "shared",
		"method": "get",
		"folder": "/files",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server")
}

// TestSMBActivity_MissingShare ensures an error when 'share' is absent.
func TestSMBActivity_MissingShare(t *testing.T) {
	a := &SMBActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "fileserver",
		"method": "get",
		"folder": "/files",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "share")
}

// TestSMBActivity_MissingMethod ensures an error when 'method' is absent.
func TestSMBActivity_MissingMethod(t *testing.T) {
	a := &SMBActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "fileserver",
		"share":  "shared",
		"folder": "/files",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestSMBActivity_InvalidMethod ensures a descriptive error for unknown method values.
func TestSMBActivity_InvalidMethod(t *testing.T) {
	a := &SMBActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server": "fileserver",
		"share":  "shared",
		"method": "delete",
		"folder": "/files",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestSMBActivity_InvalidRegex ensures malformed regex_filter is rejected before any network I/O.
func TestSMBActivity_InvalidRegex(t *testing.T) {
	a := &SMBActivity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"server":       "fileserver",
		"share":        "shared",
		"method":       "get",
		"folder":       "/files",
		"regex_filter": "[invalid",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regex_filter")
}

// TestSMBActivity_IntegrationGet is an integration test skipped unless
// FLOWJS_RUN_EXTERNAL_TESTS=1 is set.
func TestSMBActivity_IntegrationGet(t *testing.T) {
	if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
		t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
	}
	a := &SMBActivity{}
	out, err := a.Execute(nil, map[string]interface{}{
		"server": os.Getenv("FLOWJS_SMB_SERVER"),
		"share":  os.Getenv("FLOWJS_SMB_SHARE"),
		"method": "get",
		"folder": ".",
		"auth": map[string]interface{}{
			"user":     os.Getenv("FLOWJS_SMB_USER"),
			"password": os.Getenv("FLOWJS_SMB_PASSWORD"),
		},
	}, nil)
	require.NoError(t, err)
	assert.NotNil(t, out["files_downloaded"])
}
