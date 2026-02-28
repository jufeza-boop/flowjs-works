// Package db handles PostgreSQL connectivity and batch persistence for the audit logger.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver

	"flowjs-works/audit-logger/internal/batcher"
)

// Client wraps a PostgreSQL connection and provides batch insert operations.
type Client struct {
	db *sql.DB
}

// New opens a connection to PostgreSQL and verifies it with a ping.
// It retries up to maxRetries times with an exponential back-off.
func New(dsn string) (*Client, error) {
	const maxRetries = 5
	var (
		db  *sql.DB
		err error
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			log.Printf("audit-logger: connected to PostgreSQL (attempt %d)", attempt)
			return &Client{db: db}, nil
		}
		wait := time.Duration(attempt*attempt) * time.Second
		log.Printf("audit-logger: postgres not ready (attempt %d/%d): %v — retrying in %s",
			attempt, maxRetries, err, wait)
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("audit-logger: could not connect to PostgreSQL after %d attempts: %w", maxRetries, err)
}

// Close closes the underlying database connection pool.
func (c *Client) Close() {
	if c.db != nil {
		_ = c.db.Close()
	}
}

// BatchInsertLogs persists a slice of AuditEvents as rows in activity_logs.
// Each event requires a matching row in executions; this function upserts
// the execution header before inserting the activity rows.
func (c *Client) BatchInsertLogs(events []batcher.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Upsert execution headers first so the FK constraint is satisfied.
	if err = upsertExecutions(tx, events); err != nil {
		return err
	}

	// Build a single multi-row INSERT for activity_logs.
	if err = insertActivityLogs(tx, events); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// upsertExecutions ensures that every execution_id referenced by the batch
// has a corresponding row in the executions table.
func upsertExecutions(tx *sql.Tx, events []batcher.AuditEvent) error {
	// Deduplicate execution IDs within this batch and record their flow_id.
	// The first event seen for each execution_id wins.
	seen := make(map[string]string) // executionID → flowID
	for _, e := range events {
		if e.ExecutionID == "" {
			continue
		}
		if _, exists := seen[e.ExecutionID]; !exists {
			flowID := e.FlowID
			if flowID == "" {
				flowID = "unknown"
			}
			seen[e.ExecutionID] = flowID
		}
	}

	stmt, err := tx.Prepare(`
		INSERT INTO executions (execution_id, flow_id, status, start_time)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (execution_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("prepare upsert executions: %w", err)
	}
	defer stmt.Close()

	for id, flowID := range seen {
		if _, err := stmt.Exec(id, flowID, "STARTED"); err != nil {
			return fmt.Errorf("upsert execution %s: %w", id, err)
		}
	}
	return nil
}

// insertActivityLogs inserts all events in a single parameterised multi-row INSERT.
func insertActivityLogs(tx *sql.Tx, events []batcher.AuditEvent) error {
	const cols = 7 // execution_id, node_id, node_type, status, input_data, output_data, error_details
	placeholders := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*cols)

	for i, e := range events {
		base := i * cols
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
		))

		inputJSON, err := marshalJSONB(e.InputData)
		if err != nil {
			return err
		}
		outputJSON, err := marshalJSONB(e.OutputData)
		if err != nil {
			return err
		}
		var errorJSON []byte
		if e.ErrorMsg != "" {
			errorJSON, err = json.Marshal(map[string]string{"message": e.ErrorMsg})
			if err != nil {
				return fmt.Errorf("marshal error details: %w", err)
			}
		}

		args = append(args,
			e.ExecutionID,
			e.NodeID,
			e.NodeType,
			strings.ToUpper(e.Status),
			inputJSON,
			outputJSON,
			errorJSON,
		)
	}

	query := fmt.Sprintf(
		`INSERT INTO activity_logs
			(execution_id, node_id, node_type, status, input_data, output_data, error_details)
		 VALUES %s`,
		strings.Join(placeholders, ","),
	)

	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("batch insert activity_logs: %w", err)
	}
	return nil
}

// marshalJSONB converts a map to a JSON byte slice suitable for a JSONB column.
// Returns nil when the map is nil or empty (stores SQL NULL).
func marshalJSONB(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal jsonb: %w", err)
	}
	return b, nil
}
