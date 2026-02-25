package activities

import (
"fmt"
"time"

"flowjs-works/engine/internal/models"
"github.com/dop251/goja"
)

// ScriptActivity executes JavaScript/TypeScript code using Goja (registered as "script_ts")
type ScriptActivity struct{}

func (a *ScriptActivity) Name() string { return "script_ts" }

func (a *ScriptActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
return executeScript(input, config, ctx)
}

// CodeActivity is an alias for ScriptActivity registered as "code"
type CodeActivity struct{}

func (a *CodeActivity) Name() string { return "code" }

func (a *CodeActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
return executeScript(input, config, ctx)
}

// executeScript runs JS code with timeout support.
func executeScript(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
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

timeoutMs := 5000
if tmVal, ok := config["timeout_ms"]; ok {
switch v := tmVal.(type) {
case int:
timeoutMs = v
case float64:
timeoutMs = int(v)
}
}

vm := goja.New()
if err := vm.Set("input", input); err != nil {
return nil, fmt.Errorf("failed to set input in JS environment: %w", err)
}

timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
vm.Interrupt("timeout")
})
defer timer.Stop()

result, err := vm.RunString(scriptStr)
if err != nil {
return nil, fmt.Errorf("JavaScript execution error: %w", err)
}

if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
return map[string]interface{}{}, nil
}

exportedResult := result.Export()
switch v := exportedResult.(type) {
case map[string]interface{}:
return v, nil
case nil:
return map[string]interface{}{}, nil
default:
return map[string]interface{}{"result": v}, nil
}
}
