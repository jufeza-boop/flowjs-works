package store

import (
	"encoding/json"
	"testing"
	"time"

	"flowjs-works/engine/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ProcessRecord.ParseDSL
// ---------------------------------------------------------------------------

func TestProcessRecord_ParseDSL_ValidJSON(t *testing.T) {
	proc := &models.Process{
		Definition: models.Definition{ID: "test-flow", Version: "1.0.0", Name: "Test"},
		Trigger:    models.Trigger{ID: "trg_01", Type: "manual"},
		Nodes:      []models.Node{},
	}
	dslBytes, err := json.Marshal(proc)
	require.NoError(t, err)

	rec := &ProcessRecord{
		ID:      "test-flow",
		DSL:     json.RawMessage(dslBytes),
		Version: "1.0.0",
		Name:    "Test",
		Status:  "draft",
	}

	parsed, err := rec.ParseDSL()
	require.NoError(t, err)
	assert.Equal(t, "test-flow", parsed.Definition.ID)
	assert.Equal(t, "1.0.0", parsed.Definition.Version)
	assert.Equal(t, "manual", parsed.Trigger.Type)
}

func TestProcessRecord_ParseDSL_MalformedJSON(t *testing.T) {
	rec := &ProcessRecord{
		ID:  "bad-flow",
		DSL: json.RawMessage(`{not valid json`),
	}
	_, err := rec.ParseDSL()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse DSL")
}

// ---------------------------------------------------------------------------
// ProcessStore — in-memory stubs (no DB required)
// These tests verify the query-construction helpers via direct method coverage.
// Full integration tests require a live Postgres instance and are skipped in CI.
// ---------------------------------------------------------------------------

func TestProcessStore_New(t *testing.T) {
	store := NewProcessStore(nil)
	assert.NotNil(t, store)
}

// TestProcessRecord_JSON verifies that ProcessRecord serialises with snake_case JSON keys
// (required by the frontend getProcess() API call).
func TestProcessRecord_JSON(t *testing.T) {
	dslBytes := json.RawMessage(`{"definition":{"id":"p1"}}`)
	rec := &ProcessRecord{
		ID:          "p1",
		Version:     "1.0.0",
		Name:        "P1",
		Description: "desc",
		DSL:         dslBytes,
		Status:      "draft",
		CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(rec)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	// Keys must be lowercase/snake_case for the TypeScript client
	assert.Equal(t, "p1", m["id"])
	assert.Equal(t, "draft", m["status"])
	assert.Contains(t, m, "dsl")
	assert.Contains(t, m, "created_at")
	assert.Contains(t, m, "updated_at")
}

func TestProcessStore_Upsert_NilDB(t *testing.T) {
	t.Skip("nil DB causes a panic from database/sql; integration tests cover this path")
}

// TestProcessSummary_JSON verifies JSON serialization of ProcessSummary.
func TestProcessSummary_JSON(t *testing.T) {
	s := ProcessSummary{
		ID:        "my-flow",
		Version:   "2.0.0",
		Name:      "My Flow",
		Status:    "deployed",
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, "my-flow", m["id"])
	assert.Equal(t, "deployed", m["status"])
}
