// Package main is the entry point for the audit-logger microservice.
// It reads configuration from environment variables, sets up the NATS subscriber,
// batcher and PostgreSQL persistence layer, and exposes a small HTTP API for
// querying execution history from the Designer frontend.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"flowjs-works/audit-logger/internal/batcher"
	"flowjs-works/audit-logger/internal/db"
	"flowjs-works/audit-logger/internal/subscriber"
)

func main() {
	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	pgDSN := envOrDefault("POSTGRES_DSN",
		"host=localhost port=5432 user=admin password=flowjs_pass dbname=flowjs_audit sslmode=disable")
	httpAddr := envOrDefault("HTTP_ADDR", ":8080")

	// Connect to PostgreSQL.
	dbClient, err := db.New(pgDSN)
	if err != nil {
		log.Fatalf("audit-logger: %v", err)
	}
	defer dbClient.Close()

	// Create batcher that persists via dbClient.
	b := batcher.New(batcher.DefaultMaxBatchSize, batcher.DefaultFlushInterval, func(events []batcher.AuditEvent) error {
		if err := dbClient.BatchInsertLogs(events); err != nil {
			log.Printf("audit-logger: batch insert failed: %v", err)
			return err
		}
		log.Printf("audit-logger: persisted batch of %d events", len(events))
		return nil
	})
	defer b.Stop()

	// Subscribe to NATS.
	sub, err := subscriber.New(natsURL, b)
	if err != nil {
		log.Fatalf("audit-logger: could not connect to NATS: %v", err)
	}
	if err := sub.Start(); err != nil {
		log.Fatalf("audit-logger: could not subscribe to NATS: %v", err)
	}
	defer sub.Stop()

	// HTTP API for the Designer frontend.
	rawDB, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("audit-logger: open raw db for http: %v", err)
	}
	defer rawDB.Close()

	mux := http.NewServeMux()
	registerRoutes(mux, rawDB)

	server := &http.Server{
		Addr:         httpAddr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("audit-logger: HTTP API listening on %s", httpAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("audit-logger: HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("audit-logger: shutting down")
}

// ---------------------------------------------------------------------------
// HTTP route registration
// ---------------------------------------------------------------------------

func registerRoutes(mux *http.ServeMux, rawDB *sql.DB) {
	// GET /health — liveness probe
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := rawDB.Ping(); err != nil {
			jsonError(w, "database unreachable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		jsonOK(w, map[string]string{"status": "ok", "service": "audit-logger"})
	})

	// GET /executions — list all execution headers (most recent first)
	mux.HandleFunc("/executions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := rawDB.Query(`
			SELECT execution_id, flow_id, COALESCE(version,''), status,
			       COALESCE(correlation_id,''), start_time,
			       COALESCE(trigger_type,''), COALESCE(main_error_message,'')
			FROM executions
			ORDER BY start_time DESC
			LIMIT 200`)
		if err != nil {
			jsonError(w, "query executions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type ExecutionRow struct {
			ExecutionID      string `json:"execution_id"`
			FlowID           string `json:"flow_id"`
			Version          string `json:"version"`
			Status           string `json:"status"`
			CorrelationID    string `json:"correlation_id"`
			StartTime        string `json:"start_time"`
			TriggerType      string `json:"trigger_type"`
			MainErrorMessage string `json:"main_error_message"`
		}
		var results []ExecutionRow
		for rows.Next() {
			var exec ExecutionRow
			var startTime time.Time
			if err := rows.Scan(
				&exec.ExecutionID, &exec.FlowID, &exec.Version, &exec.Status,
				&exec.CorrelationID, &startTime, &exec.TriggerType, &exec.MainErrorMessage,
			); err != nil {
				jsonError(w, "scan execution: "+err.Error(), http.StatusInternalServerError)
				return
			}
			exec.StartTime = startTime.Format(time.RFC3339)
			results = append(results, exec)
		}
		if results == nil {
			results = []ExecutionRow{}
		}
		jsonOK(w, results)
	})

	// GET /executions/{id}/logs — list activity logs for a specific execution
	mux.HandleFunc("/executions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Parse execution_id from path: /executions/{id}/logs
		path := r.URL.Path
		var executionID string
		if _, err := fmt.Sscanf(path, "/executions/%36s", &executionID); err != nil || executionID == "" {
			http.Error(w, "missing execution_id", http.StatusBadRequest)
			return
		}
		// Trim trailing /logs if present
		if len(executionID) > 5 && executionID[len(executionID)-5:] == "/logs" {
			executionID = executionID[:len(executionID)-5]
		}

		rows, err := rawDB.Query(`
			SELECT log_id, node_id, COALESCE(node_type,''), status,
			       input_data, output_data, error_details,
			       COALESCE(duration_ms,0), created_at
			FROM activity_logs
			WHERE execution_id = $1
			ORDER BY created_at ASC`, executionID)
		if err != nil {
			jsonError(w, "query activity_logs: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type LogRow struct {
			LogID        int64           `json:"log_id"`
			NodeID       string          `json:"node_id"`
			NodeType     string          `json:"node_type"`
			Status       string          `json:"status"`
			InputData    json.RawMessage `json:"input_data"`
			OutputData   json.RawMessage `json:"output_data"`
			ErrorDetails json.RawMessage `json:"error_details"`
			DurationMs   int             `json:"duration_ms"`
			CreatedAt    string          `json:"created_at"`
		}
		var results []LogRow
		for rows.Next() {
			var lr LogRow
			var (
				inputRaw  []byte
				outputRaw []byte
				errorRaw  []byte
				createdAt time.Time
			)
			if err := rows.Scan(
				&lr.LogID, &lr.NodeID, &lr.NodeType, &lr.Status,
				&inputRaw, &outputRaw, &errorRaw, &lr.DurationMs, &createdAt,
			); err != nil {
				jsonError(w, "scan log: "+err.Error(), http.StatusInternalServerError)
				return
			}
			lr.CreatedAt = createdAt.Format(time.RFC3339)
			lr.InputData = nullableJSON(inputRaw)
			lr.OutputData = nullableJSON(outputRaw)
			lr.ErrorDetails = nullableJSON(errorRaw)
			results = append(results, lr)
		}
		if results == nil {
			results = []LogRow{}
		}
		jsonOK(w, results)
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// corsMiddleware adds CORS headers so the designer frontend (different origin) can call the API.
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

// nullableJSON returns a null JSON token when the raw bytes are nil or empty.
func nullableJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
