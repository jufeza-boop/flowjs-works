package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"flowjs-works/engine/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestExecutor returns an executor with audit logging disabled (no NATS required).
func newTestExecutor(t *testing.T) *ProcessExecutor {
	t.Helper()
	exec, err := NewProcessExecutor("") // empty URL → audit disabled
	require.NoError(t, err)
	return exec
}

// buildProcess is a test helper that creates a minimal process JSON from its parts.
func buildProcess(id string, nodes []models.Node) []byte {
	process := models.Process{
		Definition: models.Definition{
			ID:      id,
			Version: "1.0.0",
			Name:    id,
		},
		Trigger: models.Trigger{
			ID:   "trg_01",
			Type: "http_webhook",
		},
		Nodes: nodes,
	}
	data, _ := json.Marshal(process)
	return data
}

// ---------------------------------------------------------------------------
// Trigger payload propagation
// ---------------------------------------------------------------------------

// TestExecute_TriggerDataStoredInContext verifies that trigger data is stored and
// accessible in the execution context after execution.
func TestExecute_TriggerDataStoredInContext(t *testing.T) {
	exec := newTestExecutor(t)

	triggerData := map[string]interface{}{
		"body": map[string]interface{}{
			"email": "user@example.com",
		},
	}

	process := buildProcess("p1", []models.Node{
		{
			ID:   "log_1",
			Type: "logger",
			InputMapping: map[string]interface{}{
				"message": "$.trigger.body.email",
			},
			Config: map[string]interface{}{"level": "info"},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, triggerData)
	require.NoError(t, err)

	// Trigger data must be present in the returned context
	emailVal, err := ctx.GetValue("$.trigger.body.email")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", emailVal)
}

// TestExecute_NodeOutputStoredInContext verifies that after a node runs its output
// is stored in the execution context under $.nodes.<id>.output.
func TestExecute_NodeOutputStoredInContext(t *testing.T) {
	exec := newTestExecutor(t)

	triggerData := map[string]interface{}{
		"body": map[string]interface{}{"name": "Alice"},
	}

	process := buildProcess("p2", []models.Node{
		{
			ID:   "log_name",
			Type: "logger",
			InputMapping: map[string]interface{}{
				"message": "$.trigger.body.name",
			},
			Config: map[string]interface{}{"level": "info"},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, triggerData)
	require.NoError(t, err)

	// Node output must contain the "logged" field set to true
	outputVal, err := ctx.GetValue("$.nodes.log_name.output")
	require.NoError(t, err)

	outputMap, ok := outputVal.(map[string]interface{})
	require.True(t, ok, "output should be a map")
	assert.Equal(t, true, outputMap["logged"])
}

// TestExecute_NodeStatusStoredInContext verifies that the status "success" is
// recorded for a node that finishes without error.
func TestExecute_NodeStatusStoredInContext(t *testing.T) {
	exec := newTestExecutor(t)

	process := buildProcess("p3", []models.Node{
		{
			ID:     "log_1",
			Type:   "logger",
			Config: map[string]interface{}{"level": "info"},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, map[string]interface{}{})
	require.NoError(t, err)

	statusVal, err := ctx.GetValue("$.nodes.log_1.status")
	require.NoError(t, err)
	assert.Equal(t, "success", statusVal)
}

// ---------------------------------------------------------------------------
// Multi-node payload propagation (chaining)
// ---------------------------------------------------------------------------

// TestExecute_NodeOutputPropagatedToNextNode verifies that a second node can
// reference the output of the first node via input_mapping.
func TestExecute_NodeOutputPropagatedToNextNode(t *testing.T) {
	exec := newTestExecutor(t)

	triggerData := map[string]interface{}{
		"body": map[string]interface{}{"greeting": "hello-world"},
	}

	process := buildProcess("p4", []models.Node{
		{
			ID:   "node_first",
			Type: "logger",
			InputMapping: map[string]interface{}{
				"message": "$.trigger.body.greeting",
			},
			Config: map[string]interface{}{"level": "info"},
		},
		{
			ID:   "node_second",
			Type: "logger",
			InputMapping: map[string]interface{}{
				// Reference the full output object from the first node
				"message": "$.nodes.node_first.output",
			},
			Config: map[string]interface{}{"level": "info"},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, triggerData)
	require.NoError(t, err)

	// Both nodes must have status "success"
	status1, err := ctx.GetValue("$.nodes.node_first.status")
	require.NoError(t, err)
	assert.Equal(t, "success", status1)

	status2, err := ctx.GetValue("$.nodes.node_second.status")
	require.NoError(t, err)
	assert.Equal(t, "success", status2)

	// The second node's output "message" field must reflect the first node's output
	output2, err := ctx.GetValue("$.nodes.node_second.output")
	require.NoError(t, err)
	output2Map, ok := output2.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, output2Map["logged"])
}

// TestExecute_ScriptNodeTransformsPropagated verifies that a script node can
// transform trigger data and subsequent nodes can consume the result.
func TestExecute_ScriptNodeTransformsPropagated(t *testing.T) {
	exec := newTestExecutor(t)

	triggerData := map[string]interface{}{
		"body": map[string]interface{}{
			"name": "Bob",
			"age":  float64(25),
		},
	}

	process := buildProcess("p5", []models.Node{
		{
			ID:   "transform",
			Type: "script_ts",
			InputMapping: map[string]interface{}{
				"name": "$.trigger.body.name",
				"age":  "$.trigger.body.age",
			},
			Script: `(function() { return { greeting: "Hello, " + input.name + "!", isAdult: input.age >= 18 }; })()`,
		},
		{
			ID:   "log_result",
			Type: "logger",
			InputMapping: map[string]interface{}{
				"message": "$.nodes.transform.output",
			},
			Config: map[string]interface{}{"level": "info"},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, triggerData)
	require.NoError(t, err)

	// Script output must be stored in context
	scriptOutput, err := ctx.GetValue("$.nodes.transform.output")
	require.NoError(t, err)
	scriptOutputMap, ok := scriptOutput.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Hello, Bob!", scriptOutputMap["greeting"])
	assert.Equal(t, true, scriptOutputMap["isAdult"])

	// Logger node must have run successfully using the script output
	logStatus, err := ctx.GetValue("$.nodes.log_result.status")
	require.NoError(t, err)
	assert.Equal(t, "success", logStatus)
}

// ---------------------------------------------------------------------------
// Error / edge cases
// ---------------------------------------------------------------------------

// TestExecute_MalformedJSON verifies that malformed JSON input returns an error.
func TestExecute_MalformedJSON(t *testing.T) {
	exec := newTestExecutor(t)

	_, err := exec.ExecuteFromJSON([]byte(`{ this is not valid json`), map[string]interface{}{})
	assert.Error(t, err)
}

// TestExecute_UnknownActivityType verifies that referencing an unknown node type
// causes the execution to fail with a descriptive error.
func TestExecute_UnknownActivityType(t *testing.T) {
	exec := newTestExecutor(t)

	process := buildProcess("p6", []models.Node{
		{ID: "bad_node", Type: "nonexistent_activity"},
	})

	ctx, err := exec.ExecuteFromJSON(process, map[string]interface{}{})
	assert.Error(t, err)
	assert.ErrorContains(t, err, "nonexistent_activity")
	// Context is still returned with the error status
	require.NotNil(t, ctx)
	assert.Equal(t, "error", ctx.Nodes["bad_node"]["status"])
}

// TestExecute_InputMappingReferencesNonExistentNode verifies that referencing a node
// that has not yet produced output causes the execution to fail clearly.
func TestExecute_InputMappingReferencesNonExistentNode(t *testing.T) {
	exec := newTestExecutor(t)

	process := buildProcess("p7", []models.Node{
		{
			ID:   "node_a",
			Type: "logger",
			InputMapping: map[string]interface{}{
				// References a node that does not exist
				"message": "$.nodes.ghost_node.output",
			},
		},
	})

	_, err := exec.ExecuteFromJSON(process, map[string]interface{}{})
	assert.Error(t, err)
}

// TestExecute_EmptyNodeList verifies that a process with no nodes completes without error.
func TestExecute_EmptyNodeList(t *testing.T) {
	exec := newTestExecutor(t)

	process := buildProcess("p8", []models.Node{})

	ctx, err := exec.ExecuteFromJSON(process, map[string]interface{}{"event": "ping"})
	require.NoError(t, err)
	require.NotNil(t, ctx)
}

// TestExecute_HttpRequestActivityRegistered verifies that the "http_request" activity type
// is registered in the activity registry (regression for the "unknown activity type: http_request" bug).
// It also verifies that an unreachable URL does NOT abort the flow — the error is captured in
// the node output under the "error" key instead of being propagated as a fatal execution error.
func TestExecute_HttpRequestActivityRegistered(t *testing.T) {
	exec := newTestExecutor(t)

	// A node with type "http_request" and an unreachable host: the flow must complete
	// successfully (no Go error returned) and the error must be captured in the output.
	process := buildProcess("p9", []models.Node{
		{
			ID:   "http_node",
			Type: "http_request",
			Config: map[string]interface{}{
				"url":    "http://localhost:19999", // nothing listening on this port
				"method": "GET",
			},
		},
	})

	ctx, err := exec.ExecuteFromJSON(process, map[string]interface{}{})

	// The flow must NOT return a fatal error
	require.NoError(t, err)
	require.NotNil(t, ctx)

	// The node status must be "success" (error is captured in output, not propagated)
	statusVal, getErr := ctx.GetValue("$.nodes.http_node.status")
	require.NoError(t, getErr)
	assert.Equal(t, "success", statusVal)

	// The output must contain an "error" field describing the network problem
	outputVal, getErr := ctx.GetValue("$.nodes.http_node.output")
	require.NoError(t, getErr)
	outputMap, ok := outputVal.(map[string]interface{})
	require.True(t, ok, "output should be a map")
	assert.NotEmpty(t, outputMap["error"], "unreachable URL error should be captured in output.error")
}

// TestExecute_HttpRequestMissingURL verifies that omitting the url in config is still a
// fatal configuration error (not a network error, so the flow fails early).
func TestExecute_HttpRequestMissingURL(t *testing.T) {
	exec := newTestExecutor(t)

	process := buildProcess("p10", []models.Node{
		{
			ID:     "http_node",
			Type:   "http_request",
			Config: map[string]interface{}{},
		},
	})

	_, err := exec.ExecuteFromJSON(process, map[string]interface{}{})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown activity type")
}

// TestExecute_CatFactGETFlow validates an end-to-end HTTP GET flow against the
// public catfact API. This is an external integration test and is skipped by
// default unless FLOWJS_RUN_EXTERNAL_TESTS=1.
func TestExecute_CatFactGETFlow(t *testing.T) {
	if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
		t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
	}

	exec := newTestExecutor(t)

	processPath := filepath.Join("..", "..", "test-catfact-process.json")
	triggerPath := filepath.Join("..", "..", "test-catfact-trigger.json")

	processJSON, err := os.ReadFile(processPath)
	require.NoError(t, err)

	triggerJSON, err := os.ReadFile(triggerPath)
	require.NoError(t, err)

	triggerData := make(map[string]interface{})
	err = json.Unmarshal(triggerJSON, &triggerData)
	require.NoError(t, err)

	ctx, err := exec.ExecuteFromJSON(processJSON, triggerData)
	require.NoError(t, err)

	statusVal, err := ctx.GetValue("$.nodes.get_cat_fact.status")
	require.NoError(t, err)
	assert.Equal(t, "success", statusVal)

	bodyVal, err := ctx.GetValue("$.nodes.get_cat_fact.output.body")
	require.NoError(t, err)

	bodyMap, ok := bodyVal.(map[string]interface{})
	require.True(t, ok, "response body should be a map")

	fact, ok := bodyMap["fact"].(string)
	require.True(t, ok, "response body should include fact as string")
	assert.NotEmpty(t, fact)
}

func TestTransition_SuccessPath(t *testing.T) {
exec := newTestExecutor(t)
process := models.Process{
Definition: models.Definition{ID: "trans-p1", Version: "1.0.0", Name: "trans-p1"},
Trigger:    models.Trigger{ID: "trg", Type: "manual"},
Nodes: []models.Node{
{ID: "n1", Type: "logger", Config: map[string]interface{}{"level": "info"}},
{ID: "n2", Type: "logger", Config: map[string]interface{}{"level": "info"}},
},
Transitions: []models.Transition{
{From: "n1", To: "n2", Type: "success"},
},
}
data, _ := json.Marshal(process)
ctx, err := exec.ExecuteFromJSON(data, map[string]interface{}{})
require.NoError(t, err)
s1, _ := ctx.GetValue("$.nodes.n1.status")
s2, _ := ctx.GetValue("$.nodes.n2.status")
assert.Equal(t, "success", s1)
assert.Equal(t, "success", s2)
}

func TestTransition_ErrorPath(t *testing.T) {
exec := newTestExecutor(t)
process := models.Process{
Definition: models.Definition{ID: "trans-p2", Version: "1.0.0", Name: "trans-p2"},
Trigger:    models.Trigger{ID: "trg", Type: "manual"},
Nodes: []models.Node{
{ID: "bad", Type: "nonexistent_activity"},
{ID: "on_error", Type: "logger", Config: map[string]interface{}{"level": "error"}},
},
Transitions: []models.Transition{
{From: "bad", To: "on_error", Type: "error"},
},
}
data, _ := json.Marshal(process)
ctx, err := exec.ExecuteFromJSON(data, map[string]interface{}{})
require.NoError(t, err)
s1, _ := ctx.GetValue("$.nodes.bad.status")
assert.Equal(t, "error", s1)
s2, _ := ctx.GetValue("$.nodes.on_error.status")
assert.Equal(t, "success", s2)
}

func TestTransition_ConditionTrue(t *testing.T) {
exec := newTestExecutor(t)
process := models.Process{
Definition: models.Definition{ID: "trans-p3", Version: "1.0.0", Name: "trans-p3"},
Trigger:    models.Trigger{ID: "trg", Type: "manual"},
Nodes: []models.Node{
{
ID: "script_node", Type: "script_ts",
Script: "(function(){ return { value: 42 }; })()",
},
{ID: "on_true", Type: "logger", Config: map[string]interface{}{"level": "info"}},
{ID: "on_false", Type: "logger", Config: map[string]interface{}{"level": "info"}},
},
Transitions: []models.Transition{
{From: "script_node", To: "on_true", Type: "condition", Condition: "$.nodes.script_node.output.value === 42"},
{From: "script_node", To: "on_false", Type: "nocondition"},
},
}
data, _ := json.Marshal(process)
ctx, err := exec.ExecuteFromJSON(data, map[string]interface{}{})
require.NoError(t, err)
s1, _ := ctx.GetValue("$.nodes.on_true.status")
assert.Equal(t, "success", s1)
_, errFalse := ctx.GetValue("$.nodes.on_false.status")
assert.Error(t, errFalse, "on_false node should not have been executed")
}

func TestTransition_NoConditionFallback(t *testing.T) {
exec := newTestExecutor(t)
process := models.Process{
Definition: models.Definition{ID: "trans-p4", Version: "1.0.0", Name: "trans-p4"},
Trigger:    models.Trigger{ID: "trg", Type: "manual"},
Nodes: []models.Node{
{
ID: "script_node", Type: "script_ts",
Script: "(function(){ return { value: 99 }; })()",
},
{ID: "on_true", Type: "logger", Config: map[string]interface{}{"level": "info"}},
{ID: "on_false", Type: "logger", Config: map[string]interface{}{"level": "info"}},
},
Transitions: []models.Transition{
{From: "script_node", To: "on_true", Type: "condition", Condition: "$.nodes.script_node.output.value === 42"},
{From: "script_node", To: "on_false", Type: "nocondition"},
},
}
data, _ := json.Marshal(process)
ctx, err := exec.ExecuteFromJSON(data, map[string]interface{}{})
require.NoError(t, err)
_, errTrue := ctx.GetValue("$.nodes.on_true.status")
assert.Error(t, errTrue, "on_true node should not have been executed")
s2, _ := ctx.GetValue("$.nodes.on_false.status")
assert.Equal(t, "success", s2)
}
