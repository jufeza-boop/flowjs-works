package models

import "encoding/json"

// =============================================================================
// flowjs-works — Core DSL Go Models
// =============================================================================
// This file mirrors the TypeScript DSL in apps/designer/src/types/dsl.ts.
// Keep both in sync when adding new node or trigger types.
// =============================================================================

// ── Top-Level Process ───────────────────────────────────────────────────────

// Process represents the complete workflow definition
type Process struct {
	Definition  Definition   `json:"definition"`
	Trigger     Trigger      `json:"trigger"`
	Nodes       []Node       `json:"nodes"`
	Transitions []Transition `json:"transitions"`
}

// Definition contains metadata about the process
type Definition struct {
	ID          string          `json:"id"`
	Version     string          `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Settings    ProcessSettings `json:"settings"`
}

// ProcessSettings defines execution behavior
type ProcessSettings struct {
	Persistence   string `json:"persistence"`    // full | minimal | none
	Timeout       int    `json:"timeout"`
	ErrorStrategy string `json:"error_strategy"` // stop_and_rollback | continue | retry
}

// ── Trigger ─────────────────────────────────────────────────────────────────

// Trigger defines how the process is initiated.
// Supported types: cron, rest, soap, rabbitmq, mcp, manual.
type Trigger struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// ── Node ────────────────────────────────────────────────────────────────────

// Node represents a single execution step in the workflow.
// Supported types: http, sftp, s3, smb, mail, rabbitmq, sql, code, log, transform, file.
type Node struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Description  string                 `json:"description,omitempty"`
	InputMapping map[string]interface{} `json:"input_mapping,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	SecretRef    string                 `json:"secret_ref,omitempty"`
	Script       string                 `json:"script,omitempty"`
	Next         []string               `json:"next,omitempty"`
	RetryPolicy  *RetryPolicy           `json:"retry_policy,omitempty"`
}

// RetryPolicy defines retry behavior for a node
type RetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Interval    string `json:"interval"`
	Type        string `json:"type"` // fixed | exponential
}

// ── Transition ──────────────────────────────────────────────────────────────

// Transition defines directional flow between nodes.
// Supported types: success, error, condition, nocondition.
type Transition struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`                // success | error | condition | nocondition
	Condition string `json:"condition,omitempty"`
}

// ── Execution Result ────────────────────────────────────────────────────────

// NodeExecution represents the result of executing a node
type NodeExecution struct {
	NodeID string                 `json:"node_id"`
	Status string                 `json:"status"` // success, error, warning
	Output map[string]interface{} `json:"output"`
	Error  string                 `json:"error,omitempty"`
}

// UnmarshalJSON custom unmarshaling to handle the process structure
func (p *Process) UnmarshalJSON(data []byte) error {
	type Alias Process
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	return json.Unmarshal(data, &aux)
}
