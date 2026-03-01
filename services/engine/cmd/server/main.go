// Package main is the HTTP server entry point for the flowjs-works engine.
// It exposes a REST API that the Designer UI calls to execute DSL flows,
// run live node tests, check service health, and manage secrets.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"flowjs-works/engine/internal/engine"
	"flowjs-works/engine/internal/models"
	"flowjs-works/engine/internal/secrets"
	procstore "flowjs-works/engine/internal/store"
	"flowjs-works/engine/internal/triggers"

	_ "github.com/lib/pq"
)

// validProcessIDRe ensures process IDs only contain URL-safe alphanumeric
// characters, hyphens, and underscores, to prevent path traversal or injection.
var validProcessIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,255}$`)

// flowResponse is the shared response shape returned by /v1/flow, /replay, and /replay-from.
type flowResponse struct {
	ExecutionID string                            `json:"execution_id"`
	Nodes       map[string]map[string]interface{} `json:"nodes"`
	Error       string                            `json:"error,omitempty"`
}

// writeFlowResponse writes an execution result to w using the shared flowResponse shape.
// On execution error it sets HTTP 422 Unprocessable Entity.
func writeFlowResponse(w http.ResponseWriter, ctx *models.ExecutionContext, execErr error) {
	resp := flowResponse{Nodes: map[string]map[string]interface{}{}}
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
}

func main() {
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	httpAddr := envOrDefault("HTTP_ADDR", ":9090")
	requestTimeout := parseDurationEnv("REQUEST_TIMEOUT", 60*time.Second)

	executor, err := engine.NewProcessExecutor(natsURL)
	if err != nil {
		log.Fatalf("engine-server: failed to create executor: %v", err)
	}
	defer executor.Close()

	// Trigger manager handles deploy/stop lifecycle for all trigger types.
	triggerMgr := triggers.NewManager(executor)
	defer triggerMgr.StopAll()

	// Optional: connect to the config DB for secrets management and process storage.
	// When DATABASE_URL is not set the secrets and process endpoints return 503.
	var secretStore *secrets.SecretStore
	var processStore *procstore.ProcessStore
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, dbErr := sql.Open("postgres", dbURL)
		if dbErr != nil {
			log.Printf("engine-server: config DB unavailable: %v", dbErr)
		} else {
			aesKey := aesKeyFromEnv("SECRETS_AES_KEY")
			ss, storeErr := secrets.NewSecretStore(db, aesKey)
			if storeErr != nil {
				log.Printf("engine-server: failed to create secret store: %v", storeErr)
			} else {
				secretStore = ss
				executor.SetSecretResolver(ss)
				log.Printf("engine-server: DB-backed secret store enabled")
			}
			processStore = procstore.NewProcessStore(db)
			log.Printf("engine-server: DB-backed process store enabled")
		}
	}

	mux := http.NewServeMux()
	registerRoutes(mux, executor, secretStore, processStore, triggerMgr)

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

func registerRoutes(mux *http.ServeMux, executor *engine.ProcessExecutor, store *secrets.SecretStore, procStore *procstore.ProcessStore, triggerMgr *triggers.Manager) {
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
		writeFlowResponse(w, ctx, execErr)
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

	// ── Secrets API ─────────────────────────────────────────────────────────

	// GET /api/v1/secrets — list secret metadata (no values)
	// POST /api/v1/secrets — create or update a secret
	mux.HandleFunc("/api/v1/secrets", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			jsonError(w, "secrets store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			list, err := store.List(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if list == nil {
				list = []secrets.SecretMeta{}
			}
			jsonOK(w, list)

		case http.MethodPost:
			var input secrets.SecretInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
				return
			}
			if err := store.Upsert(r.Context(), input); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": input.ID, "status": "saved"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// DELETE /api/v1/secrets/{secretId}
	mux.HandleFunc("/api/v1/secrets/", func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			jsonError(w, "secrets store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		secretID := strings.TrimPrefix(r.URL.Path, "/api/v1/secrets/")
		if secretID == "" {
			jsonError(w, "secret id is required", http.StatusBadRequest)
			return
		}
		if err := store.Delete(r.Context(), secretID); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// ── Process Management API ───────────────────────────────────────────────

	// GET  /api/v1/processes        — list all processes (optionally ?status=draft|deployed|stopped)
	// POST /api/v1/processes        — create or update a process (upsert by definition.id)
	mux.HandleFunc("/api/v1/processes", func(w http.ResponseWriter, r *http.Request) {
		if procStore == nil {
			jsonError(w, "process store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			statusFilter := r.URL.Query().Get("status")
			list, err := procStore.List(r.Context(), statusFilter)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if list == nil {
				list = []procstore.ProcessSummary{}
			}
			jsonOK(w, list)

		case http.MethodPost:
			var proc models.Process
			if err := json.NewDecoder(r.Body).Decode(&proc); err != nil {
				jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
				return
			}
			if proc.Definition.ID == "" {
				jsonError(w, "definition.id is required", http.StatusBadRequest)
				return
			}
			rec, err := procStore.Upsert(r.Context(), &proc)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(rec)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// GET    /api/v1/processes/{processId}  — retrieve full DSL
	// DELETE /api/v1/processes/{processId}  — delete process
	mux.HandleFunc("/api/v1/processes/", func(w http.ResponseWriter, r *http.Request) {
		if procStore == nil {
			jsonError(w, "process store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
			return
		}
		// Strip prefix and split off optional sub-resource (deploy / stop / replay / replay-from)
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/processes/")
		parts := strings.SplitN(rest, "/", 3)
		processID := parts[0]
		if processID == "" {
			jsonError(w, "process id is required", http.StatusBadRequest)
			return
		}
		if !validProcessIDRe.MatchString(processID) {
			jsonError(w, "process id must contain only alphanumeric characters, hyphens, and underscores", http.StatusBadRequest)
			return
		}

		// ── sub-resource routing ─────────────────────────────────────────
		if len(parts) >= 2 && parts[1] != "" {
			switch parts[1] {
			case "deploy":
				handleDeploy(w, r, processID, procStore, triggerMgr, executor)
			case "stop":
				handleStop(w, r, processID, procStore, triggerMgr, executor)
			case "replay":
				handleReplay(w, r, processID, procStore, executor)
			case "replay-from":
				if len(parts) < 3 || parts[2] == "" {
					jsonError(w, "node id is required for replay-from", http.StatusBadRequest)
					return
				}
				handleReplayFrom(w, r, processID, parts[2], procStore, executor)
			default:
				jsonError(w, fmt.Sprintf("unknown sub-resource: %q", parts[1]), http.StatusNotFound)
			}
			return
		}

		// ── base resource ────────────────────────────────────────────────
		switch r.Method {
		case http.MethodGet:
			rec, err := procStore.Get(r.Context(), processID)
			if err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rec)

		case http.MethodDelete:
			// Stop the trigger first if running.
			if triggerMgr.IsRunning(processID) {
				_ = triggerMgr.Stop(processID)
			}
			if err := procStore.Delete(r.Context(), processID); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Mount the REST trigger registry so deployed REST-triggered processes
	// receive inbound HTTP calls at /triggers/{path}.
	mux.Handle("/triggers/", triggers.GetRegistryHandler())

	// Mount the SOAP trigger registry so deployed SOAP-triggered processes
	// receive inbound SOAP/XML calls at /soap/{path}.
	mux.Handle("/soap/", triggers.GetSOAPRegistryHandler())
}

// handleDeploy starts the trigger for a process and updates its status to "deployed".
func handleDeploy(w http.ResponseWriter, r *http.Request, processID string, procStore *procstore.ProcessStore, triggerMgr *triggers.Manager, executor *engine.ProcessExecutor) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rec, err := procStore.Get(r.Context(), processID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	proc, err := rec.ParseDSL()
	if err != nil {
		jsonError(w, fmt.Sprintf("parse DSL: %v", err), http.StatusInternalServerError)
		return
	}
	if err := triggerMgr.Deploy(proc); err != nil {
		executor.SendLifecycleAuditLog(processID, proc.Trigger.Type, "deployed", err.Error())
		jsonError(w, fmt.Sprintf("deploy trigger: %v", err), http.StatusBadRequest)
		return
	}
	if err := procStore.UpdateStatus(r.Context(), processID, "deployed"); err != nil {
		log.Printf("engine-server: warning: update status for %q: %v", processID, err)
	}
	executor.SendLifecycleAuditLog(processID, proc.Trigger.Type, "deployed", "")
	jsonOK(w, map[string]string{
		"process_id": processID,
		"status":     "deployed",
		"message":    fmt.Sprintf("%s trigger started", proc.Trigger.Type),
	})
}

// handleStop deactivates the trigger for a process and updates its status to "stopped".
func handleStop(w http.ResponseWriter, r *http.Request, processID string, procStore *procstore.ProcessStore, triggerMgr *triggers.Manager, executor *engine.ProcessExecutor) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Capture the trigger type before stopping so audit logs carry full context.
	triggerType := triggerMgr.TriggerType(processID)
	if err := triggerMgr.Stop(processID); err != nil {
		executor.SendLifecycleAuditLog(processID, triggerType, "stopped", err.Error())
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := procStore.UpdateStatus(r.Context(), processID, "stopped"); err != nil {
		log.Printf("engine-server: warning: update status for %q: %v", processID, err)
	}
	executor.SendLifecycleAuditLog(processID, triggerType, "stopped", "")
	jsonOK(w, map[string]string{
		"process_id": processID,
		"status":     "stopped",
	})
}

// handleReplay executes a stored process using new trigger data (full re-run).
func handleReplay(w http.ResponseWriter, r *http.Request, processID string, procStore *procstore.ProcessStore, executor *engine.ProcessExecutor) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if procStore == nil {
		jsonError(w, "process store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
		return
	}
	rec, err := procStore.Get(r.Context(), processID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	proc, err := rec.ParseDSL()
	if err != nil {
		jsonError(w, fmt.Sprintf("parse DSL: %v", err), http.StatusInternalServerError)
		return
	}

	var reqRaw struct {
		TriggerData json.RawMessage `json:"trigger_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqRaw); err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	// Accept any JSON value for trigger_data; default to {} if absent or null.
	// Return 400 if trigger_data is present but not a JSON object.
	var triggerData map[string]interface{}
	if len(reqRaw.TriggerData) > 0 {
		if err := json.Unmarshal(reqRaw.TriggerData, &triggerData); err != nil || triggerData == nil {
			jsonError(w, "trigger_data must be a JSON object (got non-object value)", http.StatusBadRequest)
			return
		}
	}
	if triggerData == nil {
		triggerData = map[string]interface{}{}
	}

	ctx, execErr := executor.Execute(proc, triggerData)
	writeFlowResponse(w, ctx, execErr)
}

// handleReplayFrom re-executes a stored process starting from a specific node,
// injecting nodeInput as the pre-resolved output of that node.
func handleReplayFrom(w http.ResponseWriter, r *http.Request, processID, nodeID string, procStore *procstore.ProcessStore, executor *engine.ProcessExecutor) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if procStore == nil {
		jsonError(w, "process store not configured (DATABASE_URL missing)", http.StatusServiceUnavailable)
		return
	}
	rec, err := procStore.Get(r.Context(), processID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	proc, err := rec.ParseDSL()
	if err != nil {
		jsonError(w, fmt.Sprintf("parse DSL: %v", err), http.StatusInternalServerError)
		return
	}

	var req struct {
		NodeInput map[string]interface{} `json:"node_input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.NodeInput == nil {
		req.NodeInput = map[string]interface{}{}
	}

	ctx, execErr := executor.ExecuteFromNode(proc, nodeID, req.NodeInput, "")
	writeFlowResponse(w, ctx, execErr)
}


func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

// aesKeyFromEnv reads a 32-byte AES key from the SECRETS_AES_KEY environment
// variable. When the variable is absent or too short, a fixed development key is
// used. Production deployments MUST set SECRETS_AES_KEY to a random 32-byte value.
func aesKeyFromEnv(envKey string) []byte {
	v := os.Getenv(envKey)
	if len(v) >= 32 {
		return []byte(v[:32])
	}
	// Dev fallback — never use in production
	const devKey = "flowjs-dev-key-00000000000000000"
	log.Printf("engine-server: WARNING — using insecure dev AES key; set %s in production", envKey)
	return []byte(devKey[:32])
}
