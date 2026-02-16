package activities

import (
	"fmt"
	
	"flowjs-works/engine/internal/models"
	"github.com/dop251/goja"
)

// ScriptActivity executes JavaScript code using Goja
type ScriptActivity struct{}

// Name returns the activity type name
func (a *ScriptActivity) Name() string {
	return "script_ts"
}

// Execute runs JavaScript code with the resolved input data
func (a *ScriptActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
	// Get the script from config
	scriptCode, ok := config["script"]
	if !ok {
		return nil, fmt.Errorf("script not found in config")
	}
	
	scriptStr, ok := scriptCode.(string)
	if !ok {
		return nil, fmt.Errorf("script must be a string")
	}
	
	if scriptStr == "" {
		return nil, fmt.Errorf("script cannot be empty")
	}
	
	// Create a new Goja runtime
	vm := goja.New()
	
	// Set the input object in the JavaScript environment
	err := vm.Set("input", input)
	if err != nil {
		return nil, fmt.Errorf("failed to set input in JS environment: %w", err)
	}
	
	// Execute the script and capture errors
	result, err := vm.RunString(scriptStr)
	
	if err != nil {
		// Handle JS syntax errors and runtime errors
		return nil, fmt.Errorf("JavaScript execution error: %w", err)
	}
	
	// Convert the result to a Go map
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		// If script doesn't return anything, return empty output
		return map[string]interface{}{}, nil
	}
	
	// Export the result to Go interface
	exportedResult := result.Export()
	
	// Try to convert to map[string]interface{}
	switch v := exportedResult.(type) {
	case map[string]interface{}:
		return v, nil
	case nil:
		return map[string]interface{}{}, nil
	default:
		// If the result is not a map, wrap it in an output field
		return map[string]interface{}{
			"result": v,
		}, nil
	}
}
