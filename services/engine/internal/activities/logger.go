package activities

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"flowjs-works/engine/internal/models"
)

// LoggerActivity logs messages to the console
type LoggerActivity struct{}

// Name returns the activity type name
func (a *LoggerActivity) Name() string {
	return "logger"
}

// Execute logs the input data
func (a *LoggerActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
	// Get the log level from config (default to "info")
	level := "info"
	if levelVal, ok := config["level"]; ok {
		if levelStr, ok := levelVal.(string); ok {
			level = levelStr
		}
	}
	
	// Get the message from input
	message := ""
	if msgVal, ok := input["message"]; ok {
		switch v := msgVal.(type) {
		case string:
			message = v
		default:
			// Convert to JSON for complex objects
			jsonBytes, err := json.Marshal(msgVal)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal message: %w", err)
			}
			message = string(jsonBytes)
		}
	} else {
		// If no specific message, log the entire input
		jsonBytes, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}
		message = string(jsonBytes)
	}
	
	// Log the message
	log.Printf("[%s] %s", level, message)
	
	// Return the logged data as output
	return map[string]interface{}{
		"logged":  true,
		"level":   level,
		"message": message,
	}, nil
}

// LogActivity is a variant of LoggerActivity registered as "log".
// It accepts levels ERROR/WARNING/INFO/DEBUG (case-insensitive) and normalizes to uppercase.
// It falls back to config["message"] if input["message"] is missing.
type LogActivity struct{}

func (a *LogActivity) Name() string { return "log" }

func (a *LogActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
level := "INFO"
if levelVal, ok := config["level"]; ok {
if levelStr, ok := levelVal.(string); ok {
level = strings.ToUpper(levelStr)
}
}

message := ""
if msgVal, ok := input["message"]; ok {
switch v := msgVal.(type) {
case string:
message = v
default:
jsonBytes, err := json.Marshal(msgVal)
if err != nil {
return nil, fmt.Errorf("failed to marshal message: %w", err)
}
message = string(jsonBytes)
}
} else if cfgMsg, ok := config["message"]; ok {
if s, ok := cfgMsg.(string); ok {
message = s
}
} else {
jsonBytes, err := json.Marshal(input)
if err != nil {
return nil, fmt.Errorf("failed to marshal input: %w", err)
}
message = string(jsonBytes)
}

log.Printf("[%s] %s", level, message)
return map[string]interface{}{
"logged":  true,
"level":   level,
"message": message,
}, nil
}
