package triggers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"flowjs-works/engine/internal/models"
)

// mcpTrigger exposes an HTTP endpoint that accepts Model Context Protocol
// JSON-RPC 2.0 requests and translates them into flow executions.
//
// MCP (Model Context Protocol) is the Anthropic standard for LLM tool calls.
// The trigger listens on addr (default :9091) at /mcp/{processId}.
type mcpTrigger struct {
	executor  Executor
	server    *http.Server
	processID string
}

func newMCPTrigger(executor Executor) *mcpTrigger {
	return &mcpTrigger{executor: executor}
}

// Start registers the MCP JSON-RPC handler on an internal HTTP server.
func (t *mcpTrigger) Start(ctx context.Context, proc *models.Process) error {
	addr, err := mcpAddr(proc.Trigger.Config)
	if err != nil {
		return fmt.Errorf("mcp_trigger: %w", err)
	}

	t.processID = proc.Definition.ID
	procCopy := *proc

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp/"+proc.Definition.ID, t.buildHandler(&procCopy))
	// Health / capabilities endpoint required by MCP clients.
	mux.HandleFunc("/mcp/"+proc.Definition.ID+"/capabilities", t.capabilitiesHandler(&procCopy))

	t.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("mcp_trigger: server error for %q: %v", t.processID, err)
		}
	}()

	log.Printf("mcp_trigger: listening on %s/mcp/%s for process %q", addr, proc.Definition.ID, proc.Definition.ID)
	return nil
}

// buildHandler returns an http.HandlerFunc for the MCP JSON-RPC endpoint.
func (t *mcpTrigger) buildHandler(proc *models.Process) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req mcpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeMCPError(w, nil, -32700, "Parse error: "+err.Error())
			return
		}
		if req.JSONRPC != "2.0" {
			writeMCPError(w, req.ID, -32600, "Invalid Request: jsonrpc must be \"2.0\"")
			return
		}

		triggerData := map[string]interface{}{
			"tool_request": map[string]interface{}{
				"method":    req.Method,
				"params":    req.Params,
				"arguments": req.Params, // alias for compatibility
			},
			"client_context": map[string]interface{}{
				"jsonrpc": req.JSONRPC,
				"id":      req.ID,
			},
		}

		execCtx, execErr := t.executor.Execute(proc, triggerData)
		if execErr != nil {
			log.Printf("mcp_trigger: execution error for %q: %v", t.processID, execErr)
			writeMCPError(w, req.ID, -32000, execErr.Error())
			return
		}

		// Return the final node outputs as the MCP result.
		result := map[string]interface{}{"nodes": execCtx.Nodes}
		writeMCPResult(w, req.ID, result)
	}
}

// capabilitiesHandler returns the MCP capabilities document for this flow.
func (t *mcpTrigger) capabilitiesHandler(proc *models.Process) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caps := map[string]interface{}{}
		if proc.Trigger.Config != nil {
			if c, ok := proc.Trigger.Config["capabilities"]; ok {
				caps, _ = c.(map[string]interface{})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"version":      proc.Trigger.Config["version"],
			"capabilities": caps,
		})
	}
}

// Stop gracefully shuts down the MCP HTTP server.
func (t *mcpTrigger) Stop() error {
	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := t.server.Shutdown(ctx)
		t.server = nil
		return err
	}
	return nil
}

func (t *mcpTrigger) Type() string { return "mcp" }

// mcpAddr returns the addr the MCP server should bind to, defaulting to :9091.
func mcpAddr(config map[string]interface{}) (string, error) {
	if config == nil {
		return ":9091", nil
	}
	if raw, ok := config["addr"]; ok {
		if addr, ok := raw.(string); ok && addr != "" {
			return addr, nil
		}
	}
	return ":9091", nil
}

// ---------------------------------------------------------------------------
// MCP JSON-RPC helpers
// ---------------------------------------------------------------------------

// mcpRequest is a minimal MCP / JSON-RPC 2.0 request envelope.
type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

func writeMCPResult(w http.ResponseWriter, id, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeMCPError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
