package activities

import (
"os"
"path/filepath"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestFileActivity_CreateReadDelete(t *testing.T) {
a := &FileActivity{}
tmpDir := t.TempDir()
path := filepath.Join(tmpDir, "test.txt")

// create
out, err := a.Execute(nil, map[string]interface{}{
"operation": "create",
"path":      path,
"content":   "hello file",
}, nil)
require.NoError(t, err)
assert.Equal(t, true, out["created"])

// read back
out, err = a.Execute(nil, map[string]interface{}{
"operation": "read",
"path":      path,
}, nil)
require.NoError(t, err)
assert.Equal(t, "hello file", out["content"])

// delete
out, err = a.Execute(nil, map[string]interface{}{
"operation": "delete",
"path":      path,
}, nil)
require.NoError(t, err)
assert.Equal(t, true, out["deleted"])

_, statErr := os.Stat(path)
assert.True(t, os.IsNotExist(statErr))
}

func TestFileActivity_ReadNonexistent(t *testing.T) {
a := &FileActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"operation": "read",
"path":      "/tmp/flowjs_nonexistent_test_xyz.txt",
}, nil)
assert.Error(t, err)
}

func TestFileActivity_DeleteNonexistent(t *testing.T) {
a := &FileActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"operation": "delete",
"path":      "/tmp/flowjs_nonexistent_test_xyz.txt",
}, nil)
assert.Error(t, err)
}

func TestFileActivity_UnknownOperation(t *testing.T) {
a := &FileActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"operation": "move",
"path":      "/tmp/x.txt",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "unknown operation")
}

func TestFileActivity_AppendMode(t *testing.T) {
a := &FileActivity{}
tmpDir := t.TempDir()
path := filepath.Join(tmpDir, "append.txt")

_, err := a.Execute(nil, map[string]interface{}{"operation": "create", "path": path, "content": "line1"}, nil)
require.NoError(t, err)
_, err = a.Execute(nil, map[string]interface{}{"operation": "create", "path": path, "content": "line2", "mode": "append"}, nil)
require.NoError(t, err)

out, err := a.Execute(nil, map[string]interface{}{"operation": "read", "path": path}, nil)
require.NoError(t, err)
assert.Equal(t, "line1line2", out["content"])
}
