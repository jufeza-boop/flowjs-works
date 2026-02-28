package triggers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"flowjs-works/engine/internal/models"
)

// restTrigger registers a dynamic HTTP route on the engine's main mux so that
// external callers can invoke the flow via HTTP.
//
// Because the engine runs a single http.Server, the REST trigger uses the
// shared ServeMux stored in the global restRegistry instead of starting its own
// HTTP server. The registry is populated at server startup.
type restTrigger struct {
	executor  Executor
	processID string
	path      string
	method    string
}

func newRESTTrigger(executor Executor) *restTrigger {
	return &restTrigger{executor: executor}
}

// Start validates the REST config and registers the route in the shared registry.
func (t *restTrigger) Start(ctx context.Context, proc *models.Process) error {
	path, method, err := restTriggerConfig(proc.Trigger.Config)
	if err != nil {
		return fmt.Errorf("rest_trigger: %w", err)
	}

	t.processID = proc.Definition.ID
	t.path = path
	t.method = method

	procCopy := *proc
	globalRESTRegistry.register(path, method, func(w http.ResponseWriter, r *http.Request) {
		body := map[string]interface{}{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		// Build trigger data matching the REST trigger output shape in the DSL.
		headers := map[string]interface{}{}
		for k, vv := range r.Header {
			if len(vv) > 0 {
				headers[k] = vv[0]
			}
		}
		triggerData := map[string]interface{}{
			"method":  r.Method,
			"headers": headers,
			"body":    body,
			"auth":    r.Header.Get("Authorization"),
		}

		execCtx, execErr := t.executor.Execute(&procCopy, triggerData)
		if execErr != nil {
			log.Printf("rest_trigger: execution error for %q: %v", t.processID, execErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": execErr.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"execution_id": execCtx.ExecutionID,
			"nodes":        execCtx.Nodes,
		})
	})

	log.Printf("rest_trigger: registered %s %s for process %q", method, path, proc.Definition.ID)
	return nil
}

// Stop deregisters the route from the shared registry.
func (t *restTrigger) Stop() error {
	if t.path != "" {
		globalRESTRegistry.deregister(t.path, t.method)
		log.Printf("rest_trigger: deregistered %s %s for process %q", t.method, t.path, t.processID)
	}
	return nil
}

func (t *restTrigger) Type() string { return "rest" }

// restTriggerConfig extracts path and method from trigger config.
func restTriggerConfig(config map[string]interface{}) (path, method string, err error) {
	if config == nil {
		return "", "", fmt.Errorf("trigger config is nil; expected {\"path\":\"...\",\"method\":\"...\"}")
	}
	path, _ = config["path"].(string)
	if path == "" {
		return "", "", fmt.Errorf("trigger config missing required field \"path\"")
	}
	method, _ = config["method"].(string)
	if method == "" {
		method = http.MethodPost // sensible default
	}
	return path, method, nil
}

// ---------------------------------------------------------------------------
// Global REST route registry
// ---------------------------------------------------------------------------

// restRegistryImpl is a mutex-protected map of dynamically registered REST
// trigger handlers. It is safe for concurrent use by multiple goroutines.
type restRegistryImpl struct {
	mu       sync.RWMutex
	handlers map[string]http.HandlerFunc
}

func newRESTRegistry() *restRegistryImpl {
	return &restRegistryImpl{handlers: make(map[string]http.HandlerFunc)}
}

var globalRESTRegistry = newRESTRegistry()

func (r *restRegistryImpl) register(path, method string, h http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[registryKey(path, method)] = h
}

func (r *restRegistryImpl) deregister(path, method string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, registryKey(path, method))
}

// ServeHTTP dispatches incoming requests to the registered handler for the
// given method+path combination. It is intended to be used inside a catch-all
// HTTP route like /triggers/{path}.
//
// The /triggers prefix is stripped from the URL path before the lookup so that
// a DSL trigger configured with path "/v1/rest" is reachable at
// /triggers/v1/rest without needing to duplicate the prefix in the DSL.
func (r *restRegistryImpl) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Strip the mount-point prefix so the key matches the DSL path.
	lookupPath := strings.TrimPrefix(req.URL.Path, "/triggers")
	if lookupPath == "" {
		lookupPath = "/"
	}
	r.mu.RLock()
	key := registryKey(lookupPath, req.Method)
	h, ok := r.handlers[key]
	if !ok {
		// Fall back to method-agnostic lookup registered under POST.
		h, ok = r.handlers[registryKey(lookupPath, "POST")]
	}
	r.mu.RUnlock()

	if !ok {
		http.Error(w, fmt.Sprintf("no REST trigger registered for %s %s", req.Method, req.URL.Path), http.StatusNotFound)
		return
	}
	h(w, req)
}

// GetRegistryHandler returns the shared REST registry as an http.Handler.
// Call this once and mount it on the mux under /triggers/.
func GetRegistryHandler() http.Handler {
	return globalRESTRegistry
}

func registryKey(path, method string) string {
	return method + " " + path
}

// TimeoutMiddleware is a small helper used by REST trigger handlers.
func TimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.TimeoutHandler(next, timeout, `{"error":"request timeout"}`)
}
