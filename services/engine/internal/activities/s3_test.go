package activities

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestS3Activity_Name verifies the activity type identifier.
func TestS3Activity_Name(t *testing.T) {
	a := &S3Activity{}
	assert.Equal(t, "s3", a.Name())
}

// TestS3Activity_MissingBucket ensures an error when 'bucket' is absent.
func TestS3Activity_MissingBucket(t *testing.T) {
	a := &S3Activity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"region": "us-east-1",
		"method": "get",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket")
}

// TestS3Activity_MissingRegion ensures an error when 'region' is absent.
func TestS3Activity_MissingRegion(t *testing.T) {
	a := &S3Activity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"bucket": "my-bucket",
		"method": "get",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")
}

// TestS3Activity_MissingMethod ensures an error when 'method' is absent.
func TestS3Activity_MissingMethod(t *testing.T) {
	a := &S3Activity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"bucket": "my-bucket",
		"region": "us-east-1",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestS3Activity_InvalidMethod ensures a descriptive error for unknown method values.
func TestS3Activity_InvalidMethod(t *testing.T) {
	a := &S3Activity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"bucket": "my-bucket",
		"region": "us-east-1",
		"method": "delete",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method")
}

// TestS3Activity_InvalidRegex ensures malformed regex_filter is rejected.
func TestS3Activity_InvalidRegex(t *testing.T) {
	a := &S3Activity{}
	_, err := a.Execute(nil, map[string]interface{}{
		"bucket":       "my-bucket",
		"region":       "us-east-1",
		"method":       "get",
		"regex_filter": "[invalid",
		"auth": map[string]interface{}{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regex_filter")
}

// TestS3Activity_IntegrationGet is an integration test skipped unless
// FLOWJS_RUN_EXTERNAL_TESTS=1 is set.
func TestS3Activity_IntegrationGet(t *testing.T) {
	if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
		t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
	}
	a := &S3Activity{}
	out, err := a.Execute(nil, map[string]interface{}{
		"bucket": os.Getenv("FLOWJS_S3_BUCKET"),
		"region": os.Getenv("FLOWJS_S3_REGION"),
		"method": "get",
		"folder": "",
	}, nil)
	require.NoError(t, err)
	assert.NotNil(t, out["files_downloaded"])
}
