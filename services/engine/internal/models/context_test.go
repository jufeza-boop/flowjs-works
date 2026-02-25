package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExecutionContext(t *testing.T) {
	ctx := NewExecutionContext("exec-123")

	assert.Equal(t, "exec-123", ctx.ExecutionID)
	assert.NotNil(t, ctx.Trigger)
	assert.NotNil(t, ctx.Nodes)
	assert.Empty(t, ctx.Trigger)
	assert.Empty(t, ctx.Nodes)
}

func TestSetTriggerData(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	triggerData := map[string]interface{}{
		"body": map[string]interface{}{
			"email": "user@example.com",
			"name":  "John",
		},
		"headers": map[string]interface{}{
			"content-type": "application/json",
		},
	}

	ctx.SetTriggerData(triggerData)

	assert.Equal(t, triggerData, ctx.Trigger)
}

func TestSetNodeOutput(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	output := map[string]interface{}{
		"logged":  true,
		"level":   "info",
		"message": "hello",
	}

	ctx.SetNodeOutput("node_1", output)

	assert.Equal(t, output, ctx.Nodes["node_1"]["output"])
}

func TestSetNodeStatus(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	ctx.SetNodeStatus("node_1", "success")

	assert.Equal(t, "success", ctx.Nodes["node_1"]["status"])
}

func TestSetNodeOutputAndStatusIndependent(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	ctx.SetNodeStatus("node_1", "success")
	ctx.SetNodeOutput("node_1", map[string]interface{}{"result": 42})

	assert.Equal(t, "success", ctx.Nodes["node_1"]["status"])
	assert.Equal(t, map[string]interface{}{"result": 42}, ctx.Nodes["node_1"]["output"])
}

// TestGetValue_TriggerRoot verifies that $.trigger resolves to the full trigger map.
func TestGetValue_TriggerRoot(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetTriggerData(map[string]interface{}{
		"body": map[string]interface{}{"email": "a@b.com"},
	})

	val, err := ctx.GetValue("$.trigger")
	require.NoError(t, err)
	require.NotNil(t, val)

	triggerMap, ok := val.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, triggerMap, "body")
}

// TestGetValue_TriggerNestedField verifies that $.trigger.body.email resolves correctly.
func TestGetValue_TriggerNestedField(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetTriggerData(map[string]interface{}{
		"body": map[string]interface{}{
			"email": "user@example.com",
			"age":   float64(30),
		},
	})

	val, err := ctx.GetValue("$.trigger.body.email")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", val)
}

// TestGetValue_NodeOutput verifies that $.nodes.nodeId.output resolves correctly.
func TestGetValue_NodeOutput(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	output := map[string]interface{}{
		"logged":  true,
		"message": "test-msg",
	}
	ctx.SetNodeOutput("log_node", output)

	val, err := ctx.GetValue("$.nodes.log_node.output")
	require.NoError(t, err)
	assert.Equal(t, output, val)
}

// TestGetValue_NodeStatus verifies that $.nodes.nodeId.status resolves correctly.
func TestGetValue_NodeStatus(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetNodeStatus("log_node", "success")

	val, err := ctx.GetValue("$.nodes.log_node.status")
	require.NoError(t, err)
	assert.Equal(t, "success", val)
}

// TestGetValue_InvalidPath verifies that an invalid path returns an error.
func TestGetValue_InvalidPath(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	_, err := ctx.GetValue("$.trigger.nonexistent.field")
	assert.Error(t, err)
}

// TestGetValue_NonExistentNode verifies that referencing a non-existent node returns an error.
func TestGetValue_NonExistentNode(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	_, err := ctx.GetValue("$.nodes.missing_node.output")
	assert.Error(t, err)
}

// TestGetValue_WithoutDollarPrefix verifies that path without $. prefix still works.
func TestGetValue_WithoutDollarPrefix(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetTriggerData(map[string]interface{}{
		"body": map[string]interface{}{"name": "Alice"},
	})

	val, err := ctx.GetValue("trigger.body.name")
	require.NoError(t, err)
	assert.Equal(t, "Alice", val)
}

// TestResolveInputMapping_StaticValues verifies that static (non-$) values are passed through unchanged.
func TestResolveInputMapping_StaticValues(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	mapping := map[string]interface{}{
		"level":   "info",
		"retries": float64(3),
	}

	result, err := ctx.ResolveInputMapping(mapping)
	require.NoError(t, err)
	assert.Equal(t, "info", result["level"])
	assert.Equal(t, float64(3), result["retries"])
}

// TestResolveInputMapping_JSONPathValues verifies that $-prefixed values are resolved from context.
func TestResolveInputMapping_JSONPathValues(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetTriggerData(map[string]interface{}{
		"body": map[string]interface{}{
			"email": "user@example.com",
		},
	})

	mapping := map[string]interface{}{
		"to": "$.trigger.body.email",
	}

	result, err := ctx.ResolveInputMapping(mapping)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", result["to"])
}

// TestResolveInputMapping_NodeOutputReference verifies that a node output can be referenced in a mapping.
func TestResolveInputMapping_NodeOutputReference(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	nodeOutput := map[string]interface{}{
		"logged":  true,
		"message": "processed",
	}
	ctx.SetNodeOutput("step_1", nodeOutput)

	mapping := map[string]interface{}{
		"message": "$.nodes.step_1.output",
	}

	result, err := ctx.ResolveInputMapping(mapping)
	require.NoError(t, err)
	assert.Equal(t, nodeOutput, result["message"])
}

// TestResolveInputMapping_InvalidPath verifies that an invalid $-path returns an error.
func TestResolveInputMapping_InvalidPath(t *testing.T) {
	ctx := NewExecutionContext("exec-1")

	mapping := map[string]interface{}{
		"value": "$.nodes.nonexistent.output",
	}

	_, err := ctx.ResolveInputMapping(mapping)
	assert.Error(t, err)
}

// TestResolveInputMapping_MixedValues verifies that a mapping with both static and dynamic values is resolved correctly.
func TestResolveInputMapping_MixedValues(t *testing.T) {
	ctx := NewExecutionContext("exec-1")
	ctx.SetTriggerData(map[string]interface{}{
		"body": map[string]interface{}{"name": "Bob"},
	})

	mapping := map[string]interface{}{
		"name":  "$.trigger.body.name",
		"level": "warn",
	}

	result, err := ctx.ResolveInputMapping(mapping)
	require.NoError(t, err)
	assert.Equal(t, "Bob", result["name"])
	assert.Equal(t, "warn", result["level"])
}

// TestToJSON verifies that the context serialises to valid JSON.
func TestToJSON(t *testing.T) {
	ctx := NewExecutionContext("exec-json-1")
	ctx.SetTriggerData(map[string]interface{}{"event": "test"})
	ctx.SetNodeOutput("n1", map[string]interface{}{"ok": true})
	ctx.SetNodeStatus("n1", "success")

	jsonStr, err := ctx.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)

	// Verify it's valid JSON that round-trips
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &decoded))
	assert.Equal(t, "exec-json-1", decoded["execution_id"])
}

func TestGetValue_ArrayIndex(t *testing.T) {
ctx := NewExecutionContext("exec-arr-1")
ctx.SetTriggerData(map[string]interface{}{
"body": map[string]interface{}{
"items": []interface{}{"alpha", "beta", "gamma"},
},
})
val, err := ctx.GetValue("$.trigger.body.items[0]")
require.NoError(t, err)
assert.Equal(t, "alpha", val)
}

func TestGetValue_NestedArrayIndex(t *testing.T) {
ctx := NewExecutionContext("exec-arr-2")
ctx.SetTriggerData(map[string]interface{}{
"body": map[string]interface{}{
"users": []interface{}{
map[string]interface{}{"name": "Alice", "age": float64(30)},
map[string]interface{}{"name": "Bob",   "age": float64(25)},
},
},
})
val, err := ctx.GetValue("$.trigger.body.users[1].name")
require.NoError(t, err)
assert.Equal(t, "Bob", val)
}

func TestGetValue_ArrayIndexOutOfRange(t *testing.T) {
ctx := NewExecutionContext("exec-arr-3")
ctx.SetTriggerData(map[string]interface{}{
"body": map[string]interface{}{
"items": []interface{}{"only"},
},
})
_, err := ctx.GetValue("$.trigger.body.items[5]")
assert.Error(t, err)
}
