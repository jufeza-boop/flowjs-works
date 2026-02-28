// Package store provides DB-backed persistence for process definitions.
// It manages the lifecycle status (draft | deployed | stopped) of every flow.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"flowjs-works/engine/internal/models"
)

// ProcessRecord is a row from the processes table in the config DB.
type ProcessRecord struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DSL         json.RawMessage `json:"dsl"`
	Status      string          `json:"status"` // draft | deployed | stopped
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ProcessSummary is a lightweight view used in listing endpoints.
type ProcessSummary struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	TriggerType string    `json:"trigger_type"` // e.g. "rest", "soap", "cron"
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProcessStore persists and retrieves flow DSLs from the config database.
type ProcessStore struct {
	db *sql.DB
}

// NewProcessStore creates a store backed by db. The caller owns the connection.
func NewProcessStore(db *sql.DB) *ProcessStore {
	return &ProcessStore{db: db}
}

// Upsert inserts or updates a process definition. Status is preserved when the row
// already exists; a new row always starts as "draft".
func (s *ProcessStore) Upsert(ctx context.Context, proc *models.Process) (*ProcessRecord, error) {
	dslBytes, err := json.Marshal(proc)
	if err != nil {
		return nil, fmt.Errorf("process_store: marshal DSL: %w", err)
	}

	query := `
		INSERT INTO processes (id, version, name, description, dsl, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'draft', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		  SET version     = EXCLUDED.version,
		      name        = EXCLUDED.name,
		      description = EXCLUDED.description,
		      dsl         = EXCLUDED.dsl,
		      updated_at  = NOW()
		RETURNING id, version, name, description, dsl, status, created_at, updated_at`

	row := s.db.QueryRowContext(ctx, query,
		proc.Definition.ID,
		proc.Definition.Version,
		proc.Definition.Name,
		proc.Definition.Description,
		dslBytes,
	)
	return scanRecord(row)
}

// Get returns the full process record for id, or an error if not found.
func (s *ProcessStore) Get(ctx context.Context, id string) (*ProcessRecord, error) {
	query := `
		SELECT id, version, name, description, dsl, status, created_at, updated_at
		FROM processes WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	rec, err := scanRecord(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("process_store: process %q not found", id)
		}
		return nil, fmt.Errorf("process_store: get %q: %w", id, err)
	}
	return rec, nil
}

// List returns summaries of all processes, optionally filtered by status.
// An empty statusFilter returns all rows.
func (s *ProcessStore) List(ctx context.Context, statusFilter string) ([]ProcessSummary, error) {
	var (
		rows *sql.Rows
		err  error
	)
	const baseCols = `id, version, name, status, COALESCE(dsl->'trigger'->>'type', '') AS trigger_type, updated_at`
	if statusFilter != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+baseCols+` FROM processes WHERE status = $1 ORDER BY updated_at DESC`,
			statusFilter)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT `+baseCols+` FROM processes ORDER BY updated_at DESC`)
	}
	if err != nil {
		return nil, fmt.Errorf("process_store: list: %w", err)
	}
	defer rows.Close()

	var result []ProcessSummary
	for rows.Next() {
		var s ProcessSummary
		if err := rows.Scan(&s.ID, &s.Version, &s.Name, &s.Status, &s.TriggerType, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("process_store: scan summary: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// Delete removes a process from the store. It is a no-op when the id does not exist.
func (s *ProcessStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM processes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("process_store: delete %q: %w", id, err)
	}
	return nil
}

// UpdateStatus sets the status column for id (draft | deployed | stopped).
func (s *ProcessStore) UpdateStatus(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE processes SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("process_store: update status %q → %q: %w", id, status, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("process_store: process %q not found", id)
	}
	return nil
}

// ParseDSL deserialises the stored JSON back into a models.Process.
func (r *ProcessRecord) ParseDSL() (*models.Process, error) {
	var proc models.Process
	if err := json.Unmarshal(r.DSL, &proc); err != nil {
		return nil, fmt.Errorf("process_store: parse DSL for %q: %w", r.ID, err)
	}
	return &proc, nil
}

// scanRecord reads one row returned by Upsert / Get.
func scanRecord(row *sql.Row) (*ProcessRecord, error) {
	var rec ProcessRecord
	err := row.Scan(
		&rec.ID,
		&rec.Version,
		&rec.Name,
		&rec.Description,
		&rec.DSL,
		&rec.Status,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
