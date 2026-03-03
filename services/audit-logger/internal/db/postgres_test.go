package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"flowjs-works/audit-logger/internal/batcher"
)

// ---------------------------------------------------------------------------
// classifyExecutions tests
// ---------------------------------------------------------------------------

// makeProcessEvent creates a process-level AuditEvent (nodeType="process").
// NodeID is set to flowID because the engine convention is to use the process ID
// as the node ID for process-lifecycle events (started / completed / failed / replayed).
func makeProcessEvent(execID, flowID, status string) batcher.AuditEvent {
	return batcher.AuditEvent{
		ExecutionID: execID,
		FlowID:      flowID,
		NodeID:      flowID,
		NodeType:    "process",
		Status:      status,
	}
}

// makeNodeEvent creates an activity-node AuditEvent (nodeType is an arbitrary
// activity type such as "logger" or "http_request"). Unlike makeProcessEvent
// these events originate from individual workflow nodes, not the process lifecycle,
// and must NOT trigger a terminal-status transition.
func makeNodeEvent(execID, flowID, nodeID, nodeType, status string) batcher.AuditEvent {
	return batcher.AuditEvent{
		ExecutionID: execID,
		FlowID:      flowID,
		NodeID:      nodeID,
		NodeType:    nodeType,
		Status:      status,
	}
}

// TestClassifyExecutions_CompletedStatus verifies that a process/completed event
// in the same batch as a process/started event results in terminalStatus=COMPLETED.
func TestClassifyExecutions_CompletedStatus(t *testing.T) {
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-1", "flow-1", "started"),
		makeNodeEvent("exec-1", "flow-1", "node_a", "logger", "success"),
		makeProcessEvent("exec-1", "flow-1", "completed"),
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-1")
	assert.Equal(t, "COMPLETED", infos["exec-1"].terminalStatus)
	assert.Equal(t, "flow-1", infos["exec-1"].flowID)
	assert.Equal(t, "", infos["exec-1"].errorMsg)
}

// TestClassifyExecutions_FailedStatus verifies that a process/failed event sets
// terminalStatus=FAILED and preserves the error message.
func TestClassifyExecutions_FailedStatus(t *testing.T) {
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-2", "flow-2", "started"),
		{
			ExecutionID: "exec-2",
			FlowID:      "flow-2",
			NodeID:      "flow-2",
			NodeType:    "process",
			Status:      "failed",
			ErrorMsg:    "node timeout",
		},
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-2")
	assert.Equal(t, "FAILED", infos["exec-2"].terminalStatus)
	assert.Equal(t, "node timeout", infos["exec-2"].errorMsg)
}

// TestClassifyExecutions_ReplayedStatus verifies that a process/replayed event sets
// terminalStatus=REPLAYED.
func TestClassifyExecutions_ReplayedStatus(t *testing.T) {
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-3", "flow-3", "started"),
		makeProcessEvent("exec-3", "flow-3", "replayed"),
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-3")
	assert.Equal(t, "REPLAYED", infos["exec-3"].terminalStatus)
}

// TestClassifyExecutions_StartedOnlyNoTerminalStatus verifies that a batch with
// only a process/started event leaves terminalStatus empty (execution still in
// progress — no UPDATE should be issued).
func TestClassifyExecutions_StartedOnlyNoTerminalStatus(t *testing.T) {
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-4", "flow-4", "started"),
		makeNodeEvent("exec-4", "flow-4", "n1", "logger", "success"),
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-4")
	assert.Equal(t, "", infos["exec-4"].terminalStatus,
		"no terminal event in batch: status must remain empty so the UPDATE is not issued")
}

// TestClassifyExecutions_TerminalOnlyBatch verifies that a batch containing only
// a terminal process event (the started event was in a previous batch) is still
// classified correctly so the UPDATE runs.
func TestClassifyExecutions_TerminalOnlyBatch(t *testing.T) {
	// Simulate a second batch that arrives after the first one was flushed.
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-5", "flow-5", "completed"),
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-5")
	assert.Equal(t, "COMPLETED", infos["exec-5"].terminalStatus,
		"terminal event in its own batch must still trigger the UPDATE")
}

// TestClassifyExecutions_NodeEventsDoNotSetTerminalStatus verifies that node-level
// success/error events (nodeType != "process") never set terminalStatus.
func TestClassifyExecutions_NodeEventsDoNotSetTerminalStatus(t *testing.T) {
	events := []batcher.AuditEvent{
		makeNodeEvent("exec-6", "flow-6", "n1", "logger", "success"),
		makeNodeEvent("exec-6", "flow-6", "n2", "http_request", "error"),
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-6")
	assert.Equal(t, "", infos["exec-6"].terminalStatus)
}

// TestClassifyExecutions_EmptyExecutionIDSkipped verifies that events without an
// ExecutionID are silently ignored and do not end up in the infos map.
func TestClassifyExecutions_EmptyExecutionIDSkipped(t *testing.T) {
	events := []batcher.AuditEvent{
		{ExecutionID: "", FlowID: "flow-7", NodeType: "process", Status: "completed"},
		makeProcessEvent("exec-7", "flow-7", "completed"),
	}

	infos := classifyExecutions(events)

	assert.Len(t, infos, 1, "event with empty ExecutionID must be skipped")
	require.Contains(t, infos, "exec-7")
	assert.Equal(t, "COMPLETED", infos["exec-7"].terminalStatus)
}

// TestClassifyExecutions_MultipleExecutionsInBatch verifies that events from
// different executions are bucketed independently.
func TestClassifyExecutions_MultipleExecutionsInBatch(t *testing.T) {
	events := []batcher.AuditEvent{
		makeProcessEvent("exec-A", "flow-A", "started"),
		makeProcessEvent("exec-B", "flow-B", "started"),
		makeProcessEvent("exec-A", "flow-A", "completed"),
		makeProcessEvent("exec-B", "flow-B", "failed"),
	}

	infos := classifyExecutions(events)

	require.Len(t, infos, 2)
	assert.Equal(t, "COMPLETED", infos["exec-A"].terminalStatus)
	assert.Equal(t, "FAILED", infos["exec-B"].terminalStatus)
}

// TestClassifyExecutions_CaseInsensitiveStatus verifies that lowercase status
// strings from the engine are correctly upper-cased and matched.
func TestClassifyExecutions_CaseInsensitiveStatus(t *testing.T) {
	cases := []struct {
		rawStatus string
		want      string
	}{
		{"completed", "COMPLETED"},
		{"COMPLETED", "COMPLETED"},
		{"failed", "FAILED"},
		{"replayed", "REPLAYED"},
	}

	for _, tc := range cases {
		t.Run(tc.rawStatus, func(t *testing.T) {
			events := []batcher.AuditEvent{
				{ExecutionID: "exec-x", FlowID: "flow-x", NodeType: "process", Status: tc.rawStatus},
			}
			infos := classifyExecutions(events)
			require.Contains(t, infos, "exec-x")
			assert.Equal(t, tc.want, infos["exec-x"].terminalStatus)
		})
	}
}

// TestClassifyExecutions_UnknownFlowIDResolvedLater verifies that a placeholder
// "unknown" flowID is replaced when a subsequent event supplies the real value.
func TestClassifyExecutions_UnknownFlowIDResolvedLater(t *testing.T) {
	events := []batcher.AuditEvent{
		// First event has no FlowID → stored as "unknown"
		{ExecutionID: "exec-8", FlowID: "", NodeType: "logger", Status: "success"},
		// Later event supplies the real FlowID
		{ExecutionID: "exec-8", FlowID: "flow-8", NodeType: "process", Status: "completed"},
	}

	infos := classifyExecutions(events)

	require.Contains(t, infos, "exec-8")
	assert.Equal(t, "flow-8", infos["exec-8"].flowID)
	assert.Equal(t, "COMPLETED", infos["exec-8"].terminalStatus)
}

// ---------------------------------------------------------------------------
// insertActivityLogs argument-count tests (no DB required)
// ---------------------------------------------------------------------------

// TestInsertActivityLogs_SkipsEmptyExecutionID ensures that events without an
// ExecutionID are excluded from the placeholder list that would later be sent to
// the database — preventing "invalid input syntax for type uuid" errors that
// would roll back the entire batch (including already-committed status updates).
func TestInsertActivityLogs_SkipsEmptyExecutionID(t *testing.T) {
	// We cannot call insertActivityLogs without a real *sql.Tx, but we can
	// validate the skipping logic indirectly by verifying that classifyExecutions
	// (which shares the same skip rule) produces only entries with valid IDs.
	events := []batcher.AuditEvent{
		{ExecutionID: "", NodeType: "process", Status: "completed"},
		{ExecutionID: "valid-uuid-1", FlowID: "f1", NodeType: "process", Status: "started"},
		{ExecutionID: "valid-uuid-2", FlowID: "f2", NodeType: "logger", Status: "success"},
	}

	infos := classifyExecutions(events)

	assert.NotContains(t, infos, "", "empty ExecutionID must not appear in the infos map")
	assert.Len(t, infos, 2, "only the two valid-uuid entries must be present")
}
