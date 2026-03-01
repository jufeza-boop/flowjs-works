package activities

import (
"context"
"database/sql"
"fmt"
"time"

_ "github.com/go-sql-driver/mysql"
_ "github.com/lib/pq"

fmodels "flowjs-works/engine/internal/models"
)

// SQLActivity implements the `sql` node type.
// config fields:
//   engine:   "postgres" | "mysql" (required)
//   dsn:      full DSN string OR individual host/port/database/user/password fields
//   query:    SQL query string (required)
//   params:   []interface{} query parameters
//   timeout:  int seconds (default 30)
type SQLActivity struct{}

func (a *SQLActivity) Name() string { return "sql" }

func (a *SQLActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
engine, ok := config["engine"].(string)
if !ok || engine == "" {
return nil, fmt.Errorf("sql activity: missing required config field 'engine'")
}
query, ok := config["query"].(string)
if !ok || query == "" {
return nil, fmt.Errorf("sql activity: missing required config field 'query'")
}

dsn := buildDSN(engine, config)

timeoutSec := 30
if tv, ok := config["timeout"]; ok {
switch v := tv.(type) {
case int:
timeoutSec = v
case float64:
timeoutSec = int(v)
}
}

var params []interface{}
if p, ok := config["params"].([]interface{}); ok {
params = p
}

var driverName string
switch engine {
case "postgres":
driverName = "postgres"
case "mysql":
driverName = "mysql"
default:
return nil, fmt.Errorf("sql activity: unsupported engine %q", engine)
}

db, err := sql.Open(driverName, dsn)
if err != nil {
return nil, fmt.Errorf("sql activity: failed to open DB: %w", err)
}
defer db.Close()

deadline := time.Duration(timeoutSec) * time.Second
ctx2, cancel := context.WithTimeout(context.Background(), deadline)
defer cancel()

rows, err := db.QueryContext(ctx2, query, params...)
if err != nil {
return nil, fmt.Errorf("sql activity: query failed: %w", err)
}
defer rows.Close()

cols, err := rows.Columns()
if err != nil {
return nil, fmt.Errorf("sql activity: failed to get columns: %w", err)
}

var result []map[string]interface{}
for rows.Next() {
vals := make([]interface{}, len(cols))
ptrs := make([]interface{}, len(cols))
for i := range vals {
ptrs[i] = &vals[i]
}
if err := rows.Scan(ptrs...); err != nil {
return nil, fmt.Errorf("sql activity: failed to scan row: %w", err)
}
row := make(map[string]interface{}, len(cols))
for i, col := range cols {
row[col] = vals[i]
}
result = append(result, row)
}
if err := rows.Err(); err != nil {
return nil, fmt.Errorf("sql activity: rows error: %w", err)
}

if result == nil {
result = []map[string]interface{}{}
}

return map[string]interface{}{
"rows":          result,
"rows_affected": len(result),
}, nil
}

func buildDSN(engine string, config map[string]interface{}) string {
if dsn, ok := config["dsn"].(string); ok && dsn != "" {
return dsn
}
// Also support secrets of type connection_string whose value field is named "connection_string".
if dsn, ok := config["connection_string"].(string); ok && dsn != "" {
return dsn
}
host, _ := config["host"].(string)
port, _ := config["port"].(string)
database, _ := config["database"].(string)
user, _ := config["user"].(string)
password, _ := config["password"].(string)
if host == "" {
host = "localhost"
}
switch engine {
case "postgres":
if port == "" {
port = "5432"
}
return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", host, port, database, user, password)
case "mysql":
if port == "" {
port = "3306"
}
return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
}
return ""
}
