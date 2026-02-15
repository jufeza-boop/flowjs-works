package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	
	"flowjs-works/engine/internal/engine"
)

func main() {
	// Parse command line flags
	processFile := flag.String("process", "", "Path to the process JSON file")
	triggerFile := flag.String("trigger", "", "Path to the trigger data JSON file (optional)")
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS server URL for audit logging")
	flag.Parse()
	
	// Use example data if no process file specified
	var processJSON []byte
	var err error
	
	if *processFile == "" {
		log.Println("No process file specified, using embedded example")
		processJSON = []byte(exampleProcess)
	} else {
		processJSON, err = os.ReadFile(*processFile)
		if err != nil {
			log.Fatalf("Failed to read process file: %v", err)
		}
	}
	
	// Load trigger data
	triggerData := make(map[string]interface{})
	if *triggerFile != "" {
		triggerJSON, err := os.ReadFile(*triggerFile)
		if err != nil {
			log.Fatalf("Failed to read trigger file: %v", err)
		}
		if err := json.Unmarshal(triggerJSON, &triggerData); err != nil {
			log.Fatalf("Failed to parse trigger JSON: %v", err)
		}
	} else {
		// Default trigger data
		triggerData = map[string]interface{}{
			"body": map[string]interface{}{
				"message": "Hello from flowjs-works Runner!",
				"user_id": 12345,
			},
			"headers": map[string]interface{}{
				"content-type": "application/json",
				"date":         "2026-02-15T17:31:50Z",
			},
		}
	}
	
	// Create executor
	executor, err := engine.NewProcessExecutor(*natsURL)
	if err != nil {
		log.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Close()
	
	// Execute the process
	ctx, err := executor.ExecuteFromJSON(processJSON, triggerData)
	if err != nil {
		log.Fatalf("Process execution failed: %v", err)
	}
	
	// Print the final context
	fmt.Println("\n========== EXECUTION RESULT ==========")
	contextJSON, err := ctx.ToJSON()
	if err != nil {
		log.Printf("Failed to convert context to JSON: %v", err)
	} else {
		fmt.Println(contextJSON)
	}
	fmt.Println("======================================")
}

// exampleProcess is a simple embedded example for testing
const exampleProcess = `{
  "definition": {
    "id": "hello-world-process",
    "version": "1.0.0",
    "name": "Hello World Process",
    "description": "A simple process to test the runner",
    "settings": {
      "persistence": "full",
      "timeout": 30000,
      "error_strategy": "stop_and_rollback"
    }
  },
  "trigger": {
    "id": "trg_01",
    "type": "http_webhook",
    "config": {
      "path": "/v1/hello",
      "method": "POST"
    }
  },
  "nodes": [
    {
      "id": "log_input",
      "type": "logger",
      "description": "Log the incoming trigger data",
      "input_mapping": {
        "message": "$.trigger.body"
      },
      "config": {
        "level": "info"
      }
    },
    {
      "id": "log_user",
      "type": "logger",
      "description": "Log the user ID",
      "input_mapping": {
        "message": "$.trigger.body.user_id"
      },
      "config": {
        "level": "info"
      }
    }
  ],
  "transitions": []
}`
