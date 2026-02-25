package activities

import (
"fmt"
"os"

"flowjs-works/engine/internal/models"
)

// FileActivity implements the `file` node type.
// config fields:
//   operation: "create" | "read" | "delete"
//   path:      file path (required)
//   content:   string content (for create)
//   mode:      "overwrite" (default) | "append" (for create)
type FileActivity struct{}

func (a *FileActivity) Name() string { return "file" }

func (a *FileActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
operation, ok := config["operation"].(string)
if !ok || operation == "" {
return nil, fmt.Errorf("file activity: missing required config field 'operation'")
}
path, ok := config["path"].(string)
if !ok || path == "" {
return nil, fmt.Errorf("file activity: missing required config field 'path'")
}

switch operation {
case "create":
content := ""
if c, ok := config["content"].(string); ok {
content = c
}
mode := "overwrite"
if m, ok := config["mode"].(string); ok {
mode = m
}
flag := os.O_CREATE | os.O_WRONLY
if mode == "append" {
flag |= os.O_APPEND
} else {
flag |= os.O_TRUNC
}
f, err := os.OpenFile(path, flag, 0644)
if err != nil {
return nil, fmt.Errorf("file activity: failed to open file %q: %w", path, err)
}
defer f.Close()
if _, err := f.WriteString(content); err != nil {
return nil, fmt.Errorf("file activity: failed to write file %q: %w", path, err)
}
return map[string]interface{}{"created": true, "path": path}, nil

case "read":
data, err := os.ReadFile(path)
if err != nil {
return nil, fmt.Errorf("file activity: failed to read file %q: %w", path, err)
}
return map[string]interface{}{"content": string(data)}, nil

case "delete":
if err := os.Remove(path); err != nil {
return nil, fmt.Errorf("file activity: failed to delete file %q: %w", path, err)
}
return map[string]interface{}{"deleted": true}, nil

default:
return nil, fmt.Errorf("file activity: unknown operation %q (use create, read, delete)", operation)
}
}
