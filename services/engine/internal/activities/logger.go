package activities

import (
	"encoding/json"
	"fmt"
	"log"
	
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
