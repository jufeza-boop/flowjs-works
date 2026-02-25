package engine

import (
"context"
"encoding/json"
"fmt"
"log"
"regexp"
"time"

"flowjs-works/engine/internal/activities"
"flowjs-works/engine/internal/models"
"flowjs-works/engine/internal/secrets"

"github.com/dop251/goja"
"github.com/google/uuid"
nats "github.com/nats-io/nats.go"
)

// ProcessExecutor executes a workflow process
type ProcessExecutor struct {
activityRegistry *activities.ActivityRegistry
natsConn         *nats.Conn
auditEnabled     bool
secretResolver   secrets.SecretResolver
}

// NewProcessExecutor creates a new process executor
func NewProcessExecutor(natsURL string) (*ProcessExecutor, error) {
executor := &ProcessExecutor{
activityRegistry: activities.NewActivityRegistry(),
auditEnabled:     natsURL != "",
secretResolver:   &secrets.NoopResolver{},
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
var process models.Process
if err := json.Unmarshal(jsonData, &process); err != nil {
return nil, fmt.Errorf("failed to parse process JSON: %w", err)
}
return e.Execute(&process, triggerData)
}

// Execute executes a process with the given trigger data
func (e *ProcessExecutor) Execute(process *models.Process, triggerData map[string]interface{}) (*models.ExecutionContext, error) {
executionID := uuid.New().String()
log.Printf("Starting execution %s for process %s (v%s)", executionID, process.Definition.ID, process.Definition.Version)

ctx := models.NewExecutionContext(executionID)
ctx.SetTriggerData(triggerData)

// Sequential mode: backward-compatible when no transitions and no Next fields
if isSequentialMode(process) {
for _, node := range process.Nodes {
nodeCopy := node
if err := e.executeNode(&nodeCopy, ctx); err != nil {
return ctx, fmt.Errorf("node %s failed: %w", node.ID, err)
}
}
log.Printf("Execution %s completed successfully", executionID)
return ctx, nil
}

// Transition-based routing
nodeMap := make(map[string]*models.Node, len(process.Nodes))
for i := range process.Nodes {
nodeMap[process.Nodes[i].ID] = &process.Nodes[i]
}

transMap := make(map[string][]models.Transition)
toSet := make(map[string]bool)
for _, t := range process.Transitions {
transMap[t.From] = append(transMap[t.From], t)
toSet[t.To] = true
}

// Start nodes: nodes never appearing as To in any transition
var startNodes []string
for _, node := range process.Nodes {
if !toSet[node.ID] {
startNodes = append(startNodes, node.ID)
}
}

visited := make(map[string]bool)
for _, startID := range startNodes {
if err := e.executeChain(startID, nodeMap, transMap, ctx, visited); err != nil {
return ctx, err
}
}

log.Printf("Execution %s completed successfully", executionID)
return ctx, nil
}

// isSequentialMode returns true when no transitions and no Next fields are defined.
func isSequentialMode(process *models.Process) bool {
if len(process.Transitions) > 0 {
return false
}
for _, node := range process.Nodes {
if len(node.Next) > 0 {
return false
}
}
return true
}

func (e *ProcessExecutor) executeChain(nodeID string, nodeMap map[string]*models.Node, transMap map[string][]models.Transition, ctx *models.ExecutionContext, visited map[string]bool) error {
if visited[nodeID] {
return fmt.Errorf("cycle detected: node %s", nodeID)
}
visited[nodeID] = true

node := nodeMap[nodeID]
nodeErr := e.executeNode(node, ctx)
transitions := transMap[nodeID]

if nodeErr != nil {
var errorTrans []models.Transition
for _, t := range transitions {
if t.Type == "error" {
errorTrans = append(errorTrans, t)
}
}
if len(errorTrans) == 0 {
return nodeErr
}
for _, t := range errorTrans {
if err := e.executeChain(t.To, nodeMap, transMap, ctx, visited); err != nil {
return err
}
}
return nil
}

// Collect transitions by type
var condTrans, noCondTrans, successTrans []models.Transition
for _, t := range transitions {
switch t.Type {
case "condition":
condTrans = append(condTrans, t)
case "nocondition":
noCondTrans = append(noCondTrans, t)
case "success":
successTrans = append(successTrans, t)
}
}

if len(condTrans) > 0 || len(noCondTrans) > 0 {
for _, t := range condTrans {
if evaluateCondition(t.Condition, ctx) {
return e.executeChain(t.To, nodeMap, transMap, ctx, visited)
}
}
for _, t := range noCondTrans {
if err := e.executeChain(t.To, nodeMap, transMap, ctx, visited); err != nil {
return err
}
}
return nil
}

for _, t := range successTrans {
if err := e.executeChain(t.To, nodeMap, transMap, ctx, visited); err != nil {
return err
}
}
return nil
}

var jsonPathRe = regexp.MustCompile(`\$\.[a-zA-Z0-9_.\[\]]+`)

func evaluateCondition(expr string, ctx *models.ExecutionContext) bool {
replaced := jsonPathRe.ReplaceAllStringFunc(expr, func(token string) string {
val, err := ctx.GetValue(token)
if err != nil {
return "undefined"
}
switch v := val.(type) {
case string:
b, _ := json.Marshal(v)
return string(b)
case float64:
if v == float64(int64(v)) {
return fmt.Sprintf("%d", int64(v))
}
return fmt.Sprintf("%g", v)
case bool:
if v {
return "true"
}
return "false"
case nil:
return "null"
default:
b, _ := json.Marshal(v)
return string(b)
}
})
vm := goja.New()
result, err := vm.RunString(replaced)
if err != nil {
return false
}
return result.ToBoolean()
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
ctx.SetNodeStatus(node.ID, "error")
e.sendAuditLog(ctx.ExecutionID, node.ID, node.Type, "error", nil, nil, err.Error())
return fmt.Errorf("failed to resolve input mapping: %w", err)
}
} else {
input = make(map[string]interface{})
}

// Copy node.Config to avoid mutation on secret injection
config := make(map[string]interface{})
for k, v := range node.Config {
config[k] = v
}

// For script nodes, add the script field to config
if (node.Type == "script_ts" || node.Type == "code") && node.Script != "" {
config["script"] = node.Script
}

// Secret injection
if node.SecretRef != "" {
secretData, secretErr := e.secretResolver.Resolve(context.Background(), node.SecretRef)
if secretErr != nil {
ctx.SetNodeStatus(node.ID, "error")
e.sendAuditLog(ctx.ExecutionID, node.ID, node.Type, "error", input, nil, secretErr.Error())
return fmt.Errorf("failed to resolve secret %s: %w", node.SecretRef, secretErr)
}
for k, v := range secretData {
config[k] = v
}
}

// Get the activity implementation
activity, ok := e.activityRegistry.Get(node.Type)
if !ok {
execErr := fmt.Errorf("unknown activity type: %s", node.Type)
ctx.SetNodeStatus(node.ID, "error")
e.sendAuditLog(ctx.ExecutionID, node.ID, node.Type, "error", input, nil, execErr.Error())
return execErr
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
break
}
if attempt < maxAttempts {
log.Printf("Node %s attempt %d/%d failed: %v. Retrying...", node.ID, attempt, maxAttempts, err)
time.Sleep(2 * time.Second)
}
}

duration := time.Since(startTime)

if err != nil {
ctx.SetNodeStatus(node.ID, "error")
e.sendAuditLog(ctx.ExecutionID, node.ID, node.Type, "error", input, nil, err.Error())
return err
}

ctx.SetNodeOutput(node.ID, output)
ctx.SetNodeStatus(node.ID, "success")
log.Printf("Node %s completed successfully in %v", node.ID, duration)
e.sendAuditLog(ctx.ExecutionID, node.ID, node.Type, "success", input, output, "")

return nil
}

// sendAuditLog sends an audit message to NATS
func (e *ProcessExecutor) sendAuditLog(executionID, nodeID, nodeType, status string, input, output map[string]interface{}, errorMsg string) {
if !e.auditEnabled || e.natsConn == nil {
return
}

auditMsg := map[string]interface{}{
"execution_id": executionID,
"node_id":      nodeID,
"node_type":    nodeType,
"status":       status,
"timestamp":    time.Now().UTC().Format(time.RFC3339),
"input":        input,
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
