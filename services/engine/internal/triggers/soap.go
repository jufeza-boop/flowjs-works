// Package triggers manages the lifecycle of active flow triggers.
// soap.go implements the SOAP/HTTP trigger for the flowjs-works engine.
package triggers

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"flowjs-works/engine/internal/models"
)

// soapTrigger registers a dynamic HTTP route on the engine's shared mux so
// that external SOAP callers can invoke a flow via XML/HTTP (SOAP 1.1 and 1.2).
//
// Like the REST trigger, it delegates to the globalSOAPRegistry instead of
// owning its own http.Server. The registry must be mounted by the engine
// server at /soap/ during startup via GetSOAPRegistryHandler().
//
// Behaviour on each inbound request:
//  1. A request with the "wsdl" query key returns the stored WSDL document
//     (or HTTP 404 when no WSDL was configured).
//  2. Only HTTP POST is accepted; all other methods receive a SOAP Fault with
//     HTTP 405.
//  3. The incoming XML is parsed as a SOAP envelope; the inner XML of the
//     Body element is forwarded to the executor as trigger_data["body"]. The
//     SOAPAction header (or the HTTP method when SOAPAction is absent) is
//     forwarded as trigger_data["method"]; HTTP headers are included under
//     trigger_data["headers"].
//  4. On success a minimal SOAP envelope containing the execution ID is
//     returned; on execution failure a SOAP Fault with HTTP 500 is returned.
type soapTrigger struct {
	executor  Executor
	processID string
	path      string
	wsdl      string
}

func newSOAPTrigger(executor Executor) *soapTrigger {
	return &soapTrigger{executor: executor}
}

// Start validates the SOAP config and registers the HTTP handler in the shared
// SOAP registry.
func (t *soapTrigger) Start(_ context.Context, proc *models.Process) error {
	path, wsdl, err := soapTriggerConfig(proc.Trigger.Config)
	if err != nil {
		return fmt.Errorf("soap_trigger: %w", err)
	}

	t.processID = proc.Definition.ID
	t.path = path
	t.wsdl = wsdl

	// procCopy is a value copy so the handler closure does not hold a
	// reference to the caller's pointer. This matches the pattern used by
	// the cron, REST, and RabbitMQ triggers and prevents surprises if the
	// caller modifies proc after Deploy returns.
	procCopy := *proc
	globalSOAPRegistry.register(path, t.buildHandler(&procCopy))
	log.Printf("soap_trigger: registered POST %s for process %q", path, proc.Definition.ID)
	return nil
}

// buildHandler returns the http.HandlerFunc for this SOAP endpoint.
func (t *soapTrigger) buildHandler(proc *models.Process) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Serve the WSDL document when the caller appends ?wsdl.
		if _, wsdlReq := r.URL.Query()["wsdl"]; wsdlReq {
			if t.wsdl == "" {
				http.Error(w, "no WSDL configured for this endpoint", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			_, _ = w.Write([]byte(t.wsdl))
			return
		}

		if r.Method != http.MethodPost {
			writeSoapFault(w, http.StatusMethodNotAllowed, "Client",
				fmt.Sprintf("Method %s not allowed; SOAP endpoints only accept POST", r.Method))
			return
		}

		// Parse the SOAP envelope. encoding/xml matches on local name only
		// when no namespace URI is specified in the struct tag, making this
		// compatible with both SOAP 1.1 and 1.2 envelopes.
		var env soapRequestEnvelope
		if err := xml.NewDecoder(r.Body).Decode(&env); err != nil {
			writeSoapFault(w, http.StatusBadRequest, "Client",
				fmt.Sprintf("invalid SOAP envelope: %v", err))
			return
		}

		// SOAPAction is the SOAP 1.1 convention for identifying the operation.
		// Fall back to the HTTP method string when the header is absent.
		soapAction := strings.Trim(r.Header.Get("SOAPAction"), `"`)
		if soapAction == "" {
			soapAction = r.Method
		}

		headers := make(map[string]interface{}, len(r.Header))
		for k, vv := range r.Header {
			if len(vv) > 0 {
				// Only the first value is forwarded, consistent with how the
				// REST trigger exposes headers. Multi-value headers (e.g. Set-Cookie)
				// are uncommon on inbound SOAP requests; callers needing all values
				// can inspect the raw SOAP envelope via trigger_data["body"].
				headers[k] = vv[0]
			}
		}

		triggerData := map[string]interface{}{
			"method":  soapAction,
			"headers": headers,
			"body":    string(env.Body.Content),
		}

		execCtx, execErr := t.executor.Execute(proc, triggerData)
		if execErr != nil {
			log.Printf("soap_trigger: execution error for %q: %v", t.processID, execErr)
			writeSoapFault(w, http.StatusInternalServerError, "Server", execErr.Error())
			return
		}

		writeSoapSuccess(w, execCtx.ExecutionID)
	}
}

// Stop deregisters the route from the shared SOAP registry.
func (t *soapTrigger) Stop() error {
	if t.path != "" {
		globalSOAPRegistry.deregister(t.path)
		log.Printf("soap_trigger: deregistered %s for process %q", t.path, t.processID)
	}
	return nil
}

// Type implements TriggerHandler.
func (t *soapTrigger) Type() string { return "soap" }

// soapTriggerConfig extracts and validates SOAP trigger config fields.
// path is required; wsdl is optional (static WSDL document served at ?wsdl).
func soapTriggerConfig(config map[string]interface{}) (path, wsdl string, err error) {
	if config == nil {
		return "", "", fmt.Errorf("trigger config is nil; expected {\"path\":\"...\"}")
	}
	path, _ = config["path"].(string)
	if path == "" {
		return "", "", fmt.Errorf("trigger config missing required field \"path\"")
	}
	wsdl, _ = config["wsdl"].(string) // optional
	return path, wsdl, nil
}

// ---------------------------------------------------------------------------
// Global SOAP route registry
// ---------------------------------------------------------------------------

// soapRegistryImpl is a mutex-protected map of dynamically registered SOAP
// endpoint handlers, keyed by URL path. SOAP always uses POST, so no method
// discrimination is needed. It is safe for concurrent use.
type soapRegistryImpl struct {
	mu       sync.RWMutex
	handlers map[string]http.HandlerFunc
}

func newSOAPRegistry() *soapRegistryImpl {
	return &soapRegistryImpl{handlers: make(map[string]http.HandlerFunc)}
}

var globalSOAPRegistry = newSOAPRegistry()

func (r *soapRegistryImpl) register(path string, h http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[path] = h
}

func (r *soapRegistryImpl) deregister(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, path)
}

// ServeHTTP dispatches the incoming request to the handler registered for
// the request's URL path.
func (r *soapRegistryImpl) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	h, ok := r.handlers[req.URL.Path]
	r.mu.RUnlock()

	if !ok {
		http.Error(w, fmt.Sprintf("no SOAP trigger registered for path %s", req.URL.Path), http.StatusNotFound)
		return
	}
	h(w, req)
}

// GetSOAPRegistryHandler returns the shared SOAP registry as an http.Handler.
// Mount it on the engine mux under /soap/ during server startup.
func GetSOAPRegistryHandler() http.Handler {
	return globalSOAPRegistry
}

// ---------------------------------------------------------------------------
// SOAP XML parsing
// ---------------------------------------------------------------------------

// soapRequestEnvelope is a minimal SOAP 1.1/1.2 request envelope.
// encoding/xml matches on local name only when no namespace URI is specified
// in the struct tag, so this struct is compatible with both SOAP versions.
type soapRequestEnvelope struct {
	XMLName xml.Name        `xml:"Envelope"`
	Body    soapRequestBody `xml:"Body"`
}

// soapRequestBody captures the raw inner XML of the SOAP Body element.
type soapRequestBody struct {
	XMLName xml.Name `xml:"Body"`
	Content []byte   `xml:",innerxml"`
}

// ---------------------------------------------------------------------------
// SOAP response helpers
// ---------------------------------------------------------------------------

const soapFaultEnvelope = `<?xml version="1.0" encoding="utf-8"?>` +
	`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">` +
	`<soap:Body>` +
	`<soap:Fault>` +
	`<faultcode>soap:%s</faultcode>` +
	`<faultstring>%s</faultstring>` +
	`</soap:Fault>` +
	`</soap:Body>` +
	`</soap:Envelope>`

const soapSuccessEnvelope = `<?xml version="1.0" encoding="utf-8"?>` +
	`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">` +
	`<soap:Body>` +
	`<flowResponse>` +
	`<executionId>%s</executionId>` +
	`<status>success</status>` +
	`</flowResponse>` +
	`</soap:Body>` +
	`</soap:Envelope>`

// writeSoapFault writes a SOAP 1.1 Fault envelope with the given HTTP status.
// faultString is XML-escaped before insertion to prevent malformed responses.
// xml.EscapeText writes to a strings.Builder whose Write method never returns
// an error, so the return value is intentionally discarded.
func writeSoapFault(w http.ResponseWriter, statusCode int, faultCode, faultString string) {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(faultString))
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, soapFaultEnvelope, faultCode, escaped.String())
}

// writeSoapSuccess writes a SOAP 1.1 success envelope containing the execution ID.
// See the note on writeSoapFault regarding xml.EscapeText error discarding.
func writeSoapSuccess(w http.ResponseWriter, executionID string) {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(executionID))
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = fmt.Fprintf(w, soapSuccessEnvelope, escaped.String())
}
