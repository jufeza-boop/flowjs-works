// Package main is the HTTP server entry point for the flowjs-works engine.
// It exposes a REST API that the Designer UI calls to execute DSL flows,
// run live node tests, and check service health.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"flowjs-works/engine/internal/engine"
	"flowjs-works/engine/internal/models"
)

func main() {
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	httpAddr := envOrDefault("HTTP_ADDR", ":9090")
	requestTimeout := parseDurationEnv("REQUEST_TIMEOUT", 60*time.Second)

	executor, err := engine.NewProcessExecutor(natsURL)
	if err != nil {
		log.Fatalf("engine-server: failed to create executor: %v", err)
	}
	defer executor.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, executor)

	server := &http.Server{
		Addr:         httpAddr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  requestTimeout,
		WriteTimeout: requestTimeout,
	}

	log.Printf("engine-server: HTTP API listening on %s", httpAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("engine-server: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Route registration
// ---------------------------------------------------------------------------

func registerRoutes(mux *http.ServeMux, executor *engine.ProcessExecutor) {
	// GET /health — liveness probe
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonOK(w, map[string]string{"status": "ok", "service": "engine"})
	})

	// POST /v1/flow — execute a complete DSL flow
	mux.HandleFunc("/v1/flow", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			DSL         models.Process         `json:"dsl"`
			TriggerData map[string]interface{} `json:"trigger_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		if req.TriggerData == nil {
			req.TriggerData = map[string]interface{}{}
		}

		ctx, execErr := executor.Execute(&req.DSL, req.TriggerData)

		type response struct {
			ExecutionID string                 `json:"execution_id"`
			Nodes       map[string]map[string]interface{} `json:"nodes"`
			Error       string                 `json:"error,omitempty"`
		}

		resp := response{Nodes: map[string]map[string]interface{}{}}
		if ctx != nil {
			resp.ExecutionID = ctx.ExecutionID
			resp.Nodes = ctx.Nodes
		}
		if execErr != nil {
			resp.Error = execErr.Error()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		jsonOK(w, resp)
	})

	// POST /v1/test — live test a single script/mapping node
	mux.HandleFunc("/v1/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			InputMapping map[string]string      `json:"input_mapping"`
			Script       string                 `json:"script"`
			InputPayload map[string]interface{} `json:"input_payload"`
			// NodeType lets the UI specify which DSL activity to run (e.g. "log", "http", "sql").
			// Falls back to "script_ts" when Script is non-empty, or "logger" otherwise.
			NodeType string                 `json:"node_type"`
			// Config is the node's configuration forwarded verbatim to the activity.
			Config map[string]interface{} `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		// Determine which activity to run
		nodeType := req.NodeType
		switch {
		case nodeType != "":
			// Use whatever the UI asked for
		case req.Script != "":
			nodeType = "script_ts"
		default:
			nodeType = "logger"
		}

		// Build effective config: start from the request config (node's own config),
		// then overlay defaults so mandatory fields are always present.
		effectiveConfig := map[string]interface{}{"level": "info"}
		for k, v := range req.Config {
			effectiveConfig[k] = v
		}

		// Convert input_mapping from map[string]string to map[string]interface{}
		inputMappingIface := make(map[string]interface{}, len(req.InputMapping))
		for k, v := range req.InputMapping {
			inputMappingIface[k] = v
		}

		process := &models.Process{
			Definition: models.Definition{ID: "live-test", Version: "1.0.0", Name: "live-test"},
			Trigger:    models.Trigger{ID: "trg_test", Type: "manual"},
			Nodes: []models.Node{
				{
					ID:           "test_node",
					Type:         nodeType,
					InputMapping: inputMappingIface,
					Script:       req.Script,
					Config:       effectiveConfig,
				},
			},
		}

		ctx, execErr := executor.Execute(process, req.InputPayload)
		if execErr != nil {
			jsonError(w, execErr.Error(), http.StatusUnprocessableEntity)
			return
		}

		output := map[string]interface{}{}
		if nodeData, ok := ctx.Nodes["test_node"]; ok {
			if out, ok := nodeData["output"]; ok {
				if outMap, ok := out.(map[string]interface{}); ok {
					output = outMap
				}
			}
		}

		jsonOK(w, map[string]interface{}{"output": output})
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// corsMiddleware adds CORS headers so the Designer frontend can call this API.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseDurationEnv reads a duration from an environment variable, defaulting to def on parse error.
func parseDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("engine-server: invalid %s=%q, using default %s", key, v, def)
		return def
	}
	return d
}
