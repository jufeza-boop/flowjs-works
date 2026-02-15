package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExecutionContext holds the state during process execution
type ExecutionContext struct {
	ExecutionID string                            `json:"execution_id"`
	Trigger     map[string]interface{}            `json:"trigger"`
	Nodes       map[string]map[string]interface{} `json:"nodes"`
}

// NewExecutionContext creates a new execution context
func NewExecutionContext(executionID string) *ExecutionContext {
	return &ExecutionContext{
		ExecutionID: executionID,
		Trigger:     make(map[string]interface{}),
		Nodes:       make(map[string]map[string]interface{}),
	}
}

// SetTriggerData stores the trigger payload
func (ctx *ExecutionContext) SetTriggerData(data map[string]interface{}) {
	ctx.Trigger = data
}

// SetNodeOutput stores the output of a node execution
func (ctx *ExecutionContext) SetNodeOutput(nodeID string, output map[string]interface{}) {
	if ctx.Nodes[nodeID] == nil {
		ctx.Nodes[nodeID] = make(map[string]interface{})
	}
	ctx.Nodes[nodeID]["output"] = output
}

// SetNodeStatus stores the status of a node execution
func (ctx *ExecutionContext) SetNodeStatus(nodeID string, status string) {
	if ctx.Nodes[nodeID] == nil {
		ctx.Nodes[nodeID] = make(map[string]interface{})
	}
	ctx.Nodes[nodeID]["status"] = status
}

// GetValue retrieves a value using a simplified JSONPath syntax
// Supports paths like:
//   - $.trigger.body
//   - $.trigger.headers.date
//   - $.nodes.nodeId.output
//   - $.nodes.nodeId.status
func (ctx *ExecutionContext) GetValue(path string) (interface{}, error) {
	// Remove leading $. if present
	path = strings.TrimPrefix(path, "$.")
	
	// Split the path into parts
	parts := strings.Split(path, ".")
	
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}
	
	// Start with the root context
	var current interface{} = map[string]interface{}{
		"trigger": ctx.Trigger,
		"nodes":   ctx.Nodes,
	}
	
	// Traverse the path
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s at part %s", path, part)
			}
			current = val
		case map[string]map[string]interface{}:
			// Handle nested map structure (e.g., nodes map)
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s at part %s", path, part)
			}
			// Convert to interface{} to continue traversal
			current = interface{}(val)
		default:
			return nil, fmt.Errorf("cannot traverse path %s: not a map at part %s (type: %T)", path, part, current)
		}
	}
	
	return current, nil
}

// ResolveInputMapping resolves all input mappings for a node
func (ctx *ExecutionContext) ResolveInputMapping(inputMapping map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for key, value := range inputMapping {
		switch v := value.(type) {
		case string:
			// If the value is a string starting with $, treat it as a path
			if strings.HasPrefix(v, "$") {
				resolved, err := ctx.GetValue(v)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve %s: %w", v, err)
				}
				result[key] = resolved
			} else {
				result[key] = v
			}
		default:
			// For non-string values, use them as-is
			result[key] = v
		}
	}
	
	return result, nil
}

// ToJSON converts the context to JSON string
func (ctx *ExecutionContext) ToJSON() (string, error) {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
