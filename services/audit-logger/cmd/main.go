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
	"strconv"
	"strings"
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

	// Create batcher that persists via dbClient.
	b := batcher.New(batcher.DefaultMaxBatchSize, batcher.DefaultFlushInterval, func(events []batcher.AuditEvent) error {
		if err := dbClient.BatchInsertLogs(events); err != nil {
			log.Printf("audit-logger: batch insert failed: %v", err)
			return err
		}
		log.Printf("audit-logger: persisted batch of %d events", len(events))
		return nil
	})

	// Subscribe to NATS.
	// All defers are registered *after* every log.Fatalf call so that gocritic
	// exitAfterDefer is not triggered (os.Exit skips deferred functions).
	// Resources created before a fatal path are closed explicitly on that path.
	sub, err := subscriber.New(natsURL, b)
	if err != nil {
		b.Stop()
		dbClient.Close()
		log.Fatalf("audit-logger: could not connect to NATS: %v", err)
	}
	if err := sub.Start(); err != nil {
		b.Stop()
		dbClient.Close()
		log.Fatalf("audit-logger: could not subscribe to NATS: %v", err)
	}
	// HTTP API for the Designer frontend.
	rawDB, err := sql.Open("postgres", pgDSN)
	if err != nil {
		sub.Stop()
		b.Stop()
		dbClient.Close()
		log.Fatalf("audit-logger: open raw db for http: %v", err)
	}
	defer sub.Stop()
	defer b.Stop()
	defer dbClient.Close()
	defer func() {
		if err := rawDB.Close(); err != nil {
			log.Printf("audit-logger: close raw db: %v", err)
		}
	}()

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

// registerRoutes wires all HTTP handlers onto mux. Each handler is extracted
// into its own function to keep cyclomatic complexity below the project limit.
func registerRoutes(mux *http.ServeMux, rawDB *sql.DB) {
	mux.HandleFunc("/health", healthHandler(rawDB))
	mux.HandleFunc("/executions", listExecutionsHandler(rawDB))
	mux.HandleFunc("/executions/", executionDetailHandler(rawDB))
}

// healthHandler returns a liveness-probe handler.
func healthHandler(rawDB *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := rawDB.Ping(); err != nil {
			jsonError(w, "database unreachable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		jsonOK(w, map[string]string{"status": "ok", "service": "audit-logger"})
	}
}

// parsePagination reads ?limit and ?offset from the query string and applies
// safe bounds (max 200 for limit, non-negative for offset).
func parsePagination(q map[string][]string) (limit, offset int) {
	limit = 50
	if s := q["limit"]; len(s) > 0 {
		if n, err := strconv.Atoi(s[0]); err == nil && n > 0 {
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}
	if s := q["offset"]; len(s) > 0 {
		if n, err := strconv.Atoi(s[0]); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// buildWhereClause constructs the SQL WHERE fragment and positional args for the
// optional status and full-text search filters.
func buildWhereClause(statusFilter, searchFilter string) (string, []interface{}) {
	var parts []string
	var args []interface{}

	if statusFilter != "" {
		args = append(args, statusFilter)
		parts = append(parts, fmt.Sprintf("e.status = $%d", len(args)))
	}
	if searchFilter != "" {
		args = append(args, searchFilter)
		parts = append(parts, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM activity_logs al WHERE al.execution_id = e.execution_id"+
				" AND (al.input_data::text ILIKE '%%' || $%d || '%%'"+
				" OR al.output_data::text ILIKE '%%' || $%d || '%%'))",
			len(args), len(args),
		))
	}

	if len(parts) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(parts, " AND "), args
}

// listExecutionsHandler returns a handler that lists execution headers with
// optional filtering and pagination.
func listExecutionsHandler(rawDB *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		limit, offset := parsePagination(q)
		whereSQL, args := buildWhereClause(q.Get("status"), q.Get("search"))

		// Total matching count for X-Total-Count header.
		var total int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM executions e %s", whereSQL)
		if err := rawDB.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			jsonError(w, "count executions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		paginatedArgs := make([]interface{}, len(args)+2)
		copy(paginatedArgs, args)
		paginatedArgs[len(args)] = limit
		paginatedArgs[len(args)+1] = offset
		dataQuery := fmt.Sprintf(`
			SELECT e.execution_id, e.flow_id, COALESCE(e.version,''), e.status,
			       COALESCE(e.correlation_id,''), e.start_time,
			       COALESCE(e.trigger_type,''), COALESCE(e.main_error_message,'')
			FROM executions e
			%s
			ORDER BY e.start_time DESC
			LIMIT $%d OFFSET $%d`, whereSQL, len(paginatedArgs)-1, len(paginatedArgs))

		rows, err := rawDB.QueryContext(r.Context(), dataQuery, paginatedArgs...)
		if err != nil {
			jsonError(w, "query executions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := rows.Close(); err != nil {
				log.Printf("audit-logger: close executions rows: %v", err)
			}
		}()

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

		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		jsonOK(w, results)
	}
}

// executionDetailHandler handles /executions/{id}/logs and /executions/{id}/trigger-data.
func executionDetailHandler(rawDB *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, "/executions/")
		var executionID, subResource string
		if idx := strings.Index(rest, "/"); idx >= 0 {
			executionID = rest[:idx]
			subResource = strings.ToLower(rest[idx+1:])
		} else {
			executionID = rest
			subResource = "logs"
		}
		if executionID == "" {
			http.Error(w, "missing execution_id", http.StatusBadRequest)
			return
		}

		switch subResource {
		case "logs", "":
			serveExecutionLogs(w, r, rawDB, executionID)
		case "trigger-data":
			serveExecutionTriggerData(w, r, rawDB, executionID)
		default:
			jsonError(w, fmt.Sprintf("unknown sub-resource: %q", subResource), http.StatusNotFound)
		}
	}
}

// serveExecutionLogs writes the activity-log rows for a given execution.
func serveExecutionLogs(w http.ResponseWriter, r *http.Request, rawDB *sql.DB, executionID string) {
	rows, err := rawDB.QueryContext(r.Context(), `
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
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("audit-logger: close activity_logs rows: %v", err)
		}
	}()

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
		var inputRaw, outputRaw, errorRaw []byte
		var createdAt time.Time
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
}

// serveExecutionTriggerData writes the original trigger payload for a given execution.
func serveExecutionTriggerData(w http.ResponseWriter, r *http.Request, rawDB *sql.DB, executionID string) {
	var inputRaw []byte
	err := rawDB.QueryRowContext(r.Context(), `
		SELECT COALESCE(input_data->'trigger', '{}')
		FROM activity_logs
		WHERE execution_id = $1
		  AND node_type = 'process'
		  AND status = 'STARTED'
		ORDER BY created_at ASC
		LIMIT 1`, executionID).Scan(&inputRaw)
	if err == sql.ErrNoRows {
		jsonError(w, "trigger data not found for execution "+executionID, http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query trigger data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	payload := inputRaw
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	if _, writeErr := w.Write(payload); writeErr != nil {
		log.Printf("audit-logger: write trigger-data response: %v", writeErr)
	}
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
