package activities

import (
"os"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestSQLActivity_MissingEngine(t *testing.T) {
a := &SQLActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"query": "SELECT 1",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "engine")
}

func TestSQLActivity_MissingQuery(t *testing.T) {
a := &SQLActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"engine": "postgres",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "query")
}

func TestSQLActivity_UnsupportedEngine(t *testing.T) {
a := &SQLActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"engine": "oracle",
"query":  "SELECT 1",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "unsupported engine")
}

// TestBuildDSN_ConnectionStringField verifies that a secret of type connection_string
// (whose injected field is named "connection_string") is accepted by buildDSN.
func TestBuildDSN_ConnectionStringField(t *testing.T) {
config := map[string]interface{}{
"connection_string": "host=db port=5432 dbname=mydb user=sa password=pass sslmode=disable",
}
dsn := buildDSN("postgres", config)
assert.Equal(t, "host=db port=5432 dbname=mydb user=sa password=pass sslmode=disable", dsn)
}

// TestBuildDSN_DSNFieldTakesPriority ensures that the explicit "dsn" field wins over "connection_string".
func TestBuildDSN_DSNFieldTakesPriority(t *testing.T) {
config := map[string]interface{}{
"dsn":               "host=explicit",
"connection_string": "host=fallback",
}
dsn := buildDSN("postgres", config)
assert.Equal(t, "host=explicit", dsn)
}

func TestSQLActivity_PostgresIntegration(t *testing.T) {
if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
}
a := &SQLActivity{}
out, err := a.Execute(nil, map[string]interface{}{
"engine": "postgres",
"dsn":    "host=localhost port=5432 dbname=testdb user=postgres password=postgres sslmode=disable",
"query":  "SELECT 1 AS val",
}, nil)
require.NoError(t, err)
assert.NotNil(t, out["rows"])
}

func TestSQLActivity_MySQLIntegration(t *testing.T) {
if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
}
a := &SQLActivity{}
out, err := a.Execute(nil, map[string]interface{}{
"engine": "mysql",
"dsn":    "root:password@tcp(localhost:3306)/testdb",
"query":  "SELECT 1 AS val",
}, nil)
require.NoError(t, err)
assert.NotNil(t, out["rows"])
}
