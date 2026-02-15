package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"flowjs-works/engine/internal/activities"
	"flowjs-works/engine/internal/models"
	
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// ProcessExecutor executes a workflow process
type ProcessExecutor struct {
	activityRegistry *activities.ActivityRegistry
	natsConn         *nats.Conn
	auditEnabled     bool
}

// NewProcessExecutor creates a new process executor
func NewProcessExecutor(natsURL string) (*ProcessExecutor, error) {
	executor := &ProcessExecutor{
		activityRegistry: activities.NewActivityRegistry(),
		auditEnabled:     natsURL != "",
	}
	
	// Connect to NATS if URL is provided
	if executor.auditEnabled {
		nc, err := nats.Connect(natsURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to NATS at %s: %v. Audit logging disabled.", natsURL, err)
			executor.auditEnabled = false
		} else {
			executor.natsConn = nc
			log.Printf("Connected to NATS at %s for audit logging", natsURL)
		}
	}
	
	return executor, nil
}

// Close closes the NATS connection
func (e *ProcessExecutor) Close() {
	if e.natsConn != nil {
		e.natsConn.Close()
	}
}

// ExecuteFromJSON parses a JSON DSL and executes the process
func (e *ProcessExecutor) ExecuteFromJSON(jsonData []byte, triggerData map[string]interface{}) (*models.ExecutionContext, error) {
	// Parse the process definition
	var process models.Process
	if err := json.Unmarshal(jsonData, &process); err != nil {
		return nil, fmt.Errorf("failed to parse process JSON: %w", err)
	}
	
	// Execute the process
	return e.Execute(&process, triggerData)
}

// Execute executes a process with the given trigger data
func (e *ProcessExecutor) Execute(process *models.Process, triggerData map[string]interface{}) (*models.ExecutionContext, error) {
	// Generate execution ID
	executionID := uuid.New().String()
	
	log.Printf("Starting execution %s for process %s (v%s)", executionID, process.Definition.ID, process.Definition.Version)
	
	// Create execution context
	ctx := models.NewExecutionContext(executionID)
	ctx.SetTriggerData(triggerData)
	
	// Execute nodes sequentially (for now)
	// In the future, this could be enhanced to support parallel execution
	for _, node := range process.Nodes {
		if err := e.executeNode(&node, ctx); err != nil {
			// Mark node as failed
			ctx.SetNodeStatus(node.ID, "error")
			
			// Send audit log
			e.sendAuditLog(executionID, node.ID, "error", nil, err.Error())
			
			// According to specs, stop on error
			return ctx, fmt.Errorf("node %s failed: %w", node.ID, err)
		}
	}
	
	log.Printf("Execution %s completed successfully", executionID)
	return ctx, nil
}

// executeNode executes a single node
func (e *ProcessExecutor) executeNode(node *models.Node, ctx *models.ExecutionContext) error {
	log.Printf("Executing node %s (type: %s)", node.ID, node.Type)
	
	startTime := time.Now()
	
	// Resolve input mapping
	var input map[string]interface{}
	var err error
	
	if node.InputMapping != nil {
		input, err = ctx.ResolveInputMapping(node.InputMapping)
		if err != nil {
			return fmt.Errorf("failed to resolve input mapping: %w", err)
		}
	} else {
		input = make(map[string]interface{})
	}
	
	// Prepare config for activity execution
	config := node.Config
	if config == nil {
		config = make(map[string]interface{})
	}
	
	// For script_ts nodes, add the script to config if it exists
	if node.Type == "script_ts" && node.Script != "" {
		config["script"] = node.Script
	}
	
	// Get the activity implementation
	activity, ok := e.activityRegistry.Get(node.Type)
	if !ok {
		return fmt.Errorf("unknown activity type: %s", node.Type)
	}
	
	// Execute the activity with retry logic
	var output map[string]interface{}
	maxAttempts := 1
	
	if node.RetryPolicy != nil {
		maxAttempts = node.RetryPolicy.MaxAttempts
	}
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err = activity.Execute(input, config, ctx)
		
		if err == nil {
			// Success
			break
		}
		
		if attempt < maxAttempts {
			log.Printf("Node %s attempt %d/%d failed: %v. Retrying...", node.ID, attempt, maxAttempts, err)
			// Simple retry with fixed delay (could be enhanced with exponential backoff)
			time.Sleep(2 * time.Second)
		}
	}
	
	duration := time.Since(startTime)
	
	if err != nil {
		// All attempts failed
		ctx.SetNodeStatus(node.ID, "error")
		e.sendAuditLog(ctx.ExecutionID, node.ID, "error", nil, err.Error())
		return err
	}
	
	// Success
	ctx.SetNodeOutput(node.ID, output)
	ctx.SetNodeStatus(node.ID, "success")
	
	log.Printf("Node %s completed successfully in %v", node.ID, duration)
	
	// Send audit log
	e.sendAuditLog(ctx.ExecutionID, node.ID, "success", output, "")
	
	return nil
}

// sendAuditLog sends an audit message to NATS
func (e *ProcessExecutor) sendAuditLog(executionID, nodeID, status string, output map[string]interface{}, errorMsg string) {
	if !e.auditEnabled || e.natsConn == nil {
		return
	}
	
	auditMsg := map[string]interface{}{
		"execution_id": executionID,
		"node_id":      nodeID,
		"status":       status,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"output":       output,
	}
	
	if errorMsg != "" {
		auditMsg["error"] = errorMsg
	}
	
	msgBytes, err := json.Marshal(auditMsg)
	if err != nil {
		log.Printf("Failed to marshal audit message: %v", err)
		return
	}
	
	if err := e.natsConn.Publish("audit.logs", msgBytes); err != nil {
		log.Printf("Failed to publish audit log: %v", err)
	}
}
