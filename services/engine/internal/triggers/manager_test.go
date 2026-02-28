package triggers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowjs-works/engine/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cronTickWaitDuration is the time the cron test waits after starting the
// scheduler. It must be longer than the cron interval (1s) so at least one
// execution fires before Stop() is called.
const cronTickWaitDuration = 1200 * time.Millisecond

type mockExecutor struct {
	executions []map[string]interface{}
	err        error
}

func (m *mockExecutor) Execute(_ *models.Process, triggerData map[string]interface{}) (*models.ExecutionContext, error) {
	m.executions = append(m.executions, triggerData)
	ctx := models.NewExecutionContext("test-exec-id")
	return ctx, m.err
}

// ---------------------------------------------------------------------------
// Manager tests
// ---------------------------------------------------------------------------

func TestManager_DeployManualTrigger(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	proc := buildProcess("p1", "manual", nil)
	require.NoError(t, mgr.Deploy(proc))
	assert.True(t, mgr.IsRunning("p1"))
}

func TestManager_StopUnknownProcess(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)
	err := mgr.Stop("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not currently deployed")
}

func TestManager_DeployUnsupportedTrigger(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	proc := buildProcess("p2", "graphql", nil)
	err := mgr.Deploy(proc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported trigger type")
}

func TestManager_StopAll(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	require.NoError(t, mgr.Deploy(buildProcess("m1", "manual", nil)))
	require.NoError(t, mgr.Deploy(buildProcess("m2", "manual", nil)))

	mgr.StopAll()
	assert.False(t, mgr.IsRunning("m1"))
	assert.False(t, mgr.IsRunning("m2"))
}

func TestManager_RedeployStopsOld(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	proc := buildProcess("p3", "manual", nil)
	require.NoError(t, mgr.Deploy(proc))
	require.True(t, mgr.IsRunning("p3"))
	// Re-deploy: should not error and should still be running.
	require.NoError(t, mgr.Deploy(proc))
	assert.True(t, mgr.IsRunning("p3"))
}

func TestManager_TriggerType(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	// Not deployed → empty string
	assert.Equal(t, "", mgr.TriggerType("p-unknown"))

	proc := buildProcess("p-manual", "manual", nil)
	require.NoError(t, mgr.Deploy(proc))
	assert.Equal(t, "manual", mgr.TriggerType("p-manual"))

	// After stop, should return empty string again
	require.NoError(t, mgr.Stop("p-manual"))
	assert.Equal(t, "", mgr.TriggerType("p-manual"))
}

// ---------------------------------------------------------------------------
// Cron trigger tests
// ---------------------------------------------------------------------------

func TestCronTrigger_MissingExpression(t *testing.T) {
	exec := &mockExecutor{}
	tr := newCronTrigger(exec)
	proc := buildProcess("c1", "cron", map[string]interface{}{})
	err := tr.Start(context.Background(), proc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expression")
}

func TestCronTrigger_InvalidExpression(t *testing.T) {
	exec := &mockExecutor{}
	tr := newCronTrigger(exec)
	proc := buildProcess("c2", "cron", map[string]interface{}{"expression": "not-a-cron"})
	err := tr.Start(context.Background(), proc)
	assert.Error(t, err)
}

func TestCronTrigger_StartStop(t *testing.T) {
	exec := &mockExecutor{}
	tr := newCronTrigger(exec)
	// Use "every second" to verify the scheduler can be started and stopped cleanly.
	proc := buildProcess("c3", "cron", map[string]interface{}{"expression": "* * * * * *"})
	require.NoError(t, tr.Start(context.Background(), proc))
	// Give the scheduler time to fire at least once.
	time.Sleep(cronTickWaitDuration)
	require.NoError(t, tr.Stop())
	assert.GreaterOrEqual(t, len(exec.executions), 1, "expected at least one execution")
}

// ---------------------------------------------------------------------------
// RabbitMQ trigger config tests (no live broker required)
// ---------------------------------------------------------------------------

func TestRabbitMQTriggerConfig_Missing(t *testing.T) {
	_, _, _, err := rabbitmqTriggerConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "url_amqp")
}

func TestRabbitMQTriggerConfig_MissingQueue(t *testing.T) {
	_, _, _, err := rabbitmqTriggerConfig(map[string]interface{}{"url_amqp": "amqp://localhost"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue")
}

func TestRabbitMQTriggerConfig_Valid(t *testing.T) {
	url, q, vh, err := rabbitmqTriggerConfig(map[string]interface{}{
		"url_amqp": "amqp://guest:guest@localhost:5672",
		"queue":    "my-queue",
		"vhost":    "/",
	})
	require.NoError(t, err)
	assert.Equal(t, "amqp://guest:guest@localhost:5672", url)
	assert.Equal(t, "my-queue", q)
	assert.Equal(t, "/", vh)
}

// ---------------------------------------------------------------------------
// REST trigger tests
// ---------------------------------------------------------------------------

func TestRESTTriggerConfig_MissingPath(t *testing.T) {
	_, _, err := restTriggerConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestRESTTriggerConfig_DefaultMethod(t *testing.T) {
	path, method, err := restTriggerConfig(map[string]interface{}{"path": "/hook"})
	require.NoError(t, err)
	assert.Equal(t, "/hook", path)
	assert.Equal(t, "POST", method)
}

// ---------------------------------------------------------------------------
// MCP trigger tests
// ---------------------------------------------------------------------------

func TestMCPTrigger_StartStop(t *testing.T) {
	exec := &mockExecutor{}
	tr := newMCPTrigger(exec)
	proc := buildProcess("mcp1", "mcp", map[string]interface{}{
		"version": "2024-11-05",
		"addr":    ":19091", // random high port to avoid conflicts
	})
	require.NoError(t, tr.Start(context.Background(), proc))
	time.Sleep(50 * time.Millisecond) // let server start
	require.NoError(t, tr.Stop())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildProcess(id, triggerType string, triggerConfig map[string]interface{}) *models.Process {
	return &models.Process{
		Definition: models.Definition{ID: id, Version: "1.0.0", Name: id},
		Trigger: models.Trigger{
			ID:     "trg_01",
			Type:   triggerType,
			Config: triggerConfig,
		},
		Nodes: []models.Node{},
	}
}

// ---------------------------------------------------------------------------
// REST trigger lifecycle tests
// ---------------------------------------------------------------------------

// TestRESTTrigger_StartStop verifies that deploying a REST trigger registers an
// HTTP handler and that stopping it deregisters the handler (returns 404).
func TestRESTTrigger_StartStop(t *testing.T) {
	exec := &mockExecutor{}
	tr := newRESTTrigger(exec)

	// dslPath is what goes in the DSL config (no mount prefix).
	// reqPath is the full URL the caller uses (mirrors production: /triggers<dslPath>).
	const dslPath = "/test-rest-start-stop"
	const reqPath = "/triggers" + dslPath
	proc := buildProcess("rest-ss", "rest", map[string]interface{}{
		"path":   dslPath,
		"method": "POST",
	})

	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	// A test server backed by the shared REST registry should handle the path.
	srv := httptest.NewServer(GetRegistryHandler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+reqPath, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, exec.executions, 1)

	// After stop, the same path must return 404.
	require.NoError(t, tr.Stop())
	resp2, err := http.Post(srv.URL+reqPath, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

// TestRESTTrigger_TriggerDataShape verifies the trigger data payload built from
// an incoming HTTP request matches the DSL-specified output shape.
func TestRESTTrigger_TriggerDataShape(t *testing.T) {
	exec := &mockExecutor{}
	tr := newRESTTrigger(exec)

	const dslPath = "/test-rest-data-shape"
	const reqPath = "/triggers" + dslPath
	proc := buildProcess("rest-ds", "rest", map[string]interface{}{
		"path":   dslPath,
		"method": "POST",
	})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetRegistryHandler())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+reqPath, strings.NewReader(`{"key":"val"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, exec.executions, 1)
	td := exec.executions[0]
	assert.Equal(t, "POST", td["method"])
	assert.NotNil(t, td["headers"])
	assert.NotNil(t, td["body"])
	assert.Equal(t, "Bearer tok", td["auth"])
}

// TestRESTTrigger_ExecutionError verifies that when the executor returns an
// error the REST trigger responds with HTTP 422 and a JSON error body.
func TestRESTTrigger_ExecutionError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("flow exploded")}
	tr := newRESTTrigger(exec)

	const dslPath = "/test-rest-exec-error"
	proc := buildProcess("rest-err", "rest", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetRegistryHandler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/triggers"+dslPath, "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// SOAP trigger config tests
// ---------------------------------------------------------------------------

func TestSOAPTriggerConfig_NilConfig(t *testing.T) {
	_, _, err := soapTriggerConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestSOAPTriggerConfig_MissingPath(t *testing.T) {
	_, _, err := soapTriggerConfig(map[string]interface{}{"wsdl": "<wsdl/>"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestSOAPTriggerConfig_Valid(t *testing.T) {
	path, wsdl, err := soapTriggerConfig(map[string]interface{}{"path": "/svc"})
	require.NoError(t, err)
	assert.Equal(t, "/svc", path)
	assert.Equal(t, "", wsdl)
}

func TestSOAPTriggerConfig_WithWSDL(t *testing.T) {
	const doc = `<?xml version="1.0"?><definitions/>`
	path, wsdl, err := soapTriggerConfig(map[string]interface{}{
		"path": "/svc",
		"wsdl": doc,
	})
	require.NoError(t, err)
	assert.Equal(t, "/svc", path)
	assert.Equal(t, doc, wsdl)
}

// ---------------------------------------------------------------------------
// SOAP trigger lifecycle tests
// ---------------------------------------------------------------------------

// TestSOAPTrigger_StartStop checks that Start registers and Stop deregisters
// the SOAP endpoint in the shared registry.
func TestSOAPTrigger_StartStop(t *testing.T) {
	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-start-stop"
	const reqPath = "/soap" + dslPath
	proc := buildProcess("soap-ss", "soap", map[string]interface{}{"path": dslPath})

	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	soapReq := soapEnvelopeFixture("<ping/>")
	resp, err := http.Post(srv.URL+reqPath, "text/xml", strings.NewReader(soapReq))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Verify the SOAP Body inner-XML was forwarded to the executor.
	require.Len(t, exec.executions, 1)
	assert.Contains(t, exec.executions[0]["body"], "ping")

	require.NoError(t, tr.Stop())
	resp2, err := http.Post(srv.URL+reqPath, "text/xml", strings.NewReader(soapReq))
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

// TestSOAPTrigger_WSDLNotConfigured verifies a 404 when ?wsdl is requested but
// no WSDL document was set in the trigger config.
func TestSOAPTrigger_WSDLNotConfigured(t *testing.T) {
	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-wsdl-missing"
	proc := buildProcess("soap-wsdl-nil", "soap", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/soap" + dslPath + "?wsdl")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestSOAPTrigger_WSDLServed verifies that a configured WSDL document is
// returned with the correct Content-Type when ?wsdl is requested.
func TestSOAPTrigger_WSDLServed(t *testing.T) {
	const wsdlDoc = `<?xml version="1.0"?><definitions name="TestService"/>`

	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-wsdl-served"
	proc := buildProcess("soap-wsdl-ok", "soap", map[string]interface{}{
		"path": dslPath,
		"wsdl": wsdlDoc,
	})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/soap" + dslPath + "?wsdl")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")
}

// TestSOAPTrigger_NonPOSTMethod verifies that non-POST methods receive a SOAP
// Fault with HTTP 405.
func TestSOAPTrigger_NonPOSTMethod(t *testing.T) {
	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-non-post"
	proc := buildProcess("soap-nonpost", "soap", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/soap" + dslPath)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")
}

// TestSOAPTrigger_InvalidXML verifies that a malformed XML body receives a
// SOAP Fault with HTTP 400.
func TestSOAPTrigger_InvalidXML(t *testing.T) {
	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-invalid-xml"
	proc := buildProcess("soap-bad-xml", "soap", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/soap"+dslPath, "text/xml", strings.NewReader("not xml at all <<<"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")
}

// TestSOAPTrigger_SuccessfulExecution verifies the full happy-path: a valid
// SOAP request triggers the executor and returns a SOAP success envelope.
func TestSOAPTrigger_SuccessfulExecution(t *testing.T) {
	exec := &mockExecutor{}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-success"
	proc := buildProcess("soap-ok", "soap", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	body := soapEnvelopeFixture("<invoiceRequest><id>42</id></invoiceRequest>")
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/soap"+dslPath, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"urn:invoiceService#getInvoice"`)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")

	// Exactly one execution must have been triggered.
	require.Len(t, exec.executions, 1)
	td := exec.executions[0]

	// method must be the stripped SOAPAction value.
	assert.Equal(t, "urn:invoiceService#getInvoice", td["method"])
	// body must contain the inner XML of the SOAP Body.
	assert.Contains(t, td["body"], "invoiceRequest")
	assert.NotNil(t, td["headers"])
}

// TestSOAPTrigger_ExecutionError verifies that when the executor returns an
// error the SOAP trigger responds with a SOAP Fault and HTTP 500.
func TestSOAPTrigger_ExecutionError(t *testing.T) {
	exec := &mockExecutor{err: fmt.Errorf("downstream system unavailable")}
	tr := newSOAPTrigger(exec)

	const dslPath = "/test-soap-exec-error"
	proc := buildProcess("soap-exec-err", "soap", map[string]interface{}{"path": dslPath})
	require.NoError(t, tr.Start(context.Background(), proc))
	t.Cleanup(func() { _ = tr.Stop() })

	srv := httptest.NewServer(GetSOAPRegistryHandler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/soap"+dslPath, "text/xml", strings.NewReader(soapEnvelopeFixture("<op/>")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/xml")
}

// TestManager_DeploySOAPTrigger verifies the Manager correctly routes the
// "soap" trigger type and that the process lifecycle (deploy → running → stop)
// works end-to-end.
func TestManager_DeploySOAPTrigger(t *testing.T) {
	exec := &mockExecutor{}
	mgr := NewManager(exec)

	proc := buildProcess("soap-mgr", "soap", map[string]interface{}{
		"path": "/test-manager-soap-deploy",
	})
	require.NoError(t, mgr.Deploy(proc))
	assert.True(t, mgr.IsRunning("soap-mgr"))
	assert.Equal(t, "soap", mgr.TriggerType("soap-mgr"))

	require.NoError(t, mgr.Stop("soap-mgr"))
	assert.False(t, mgr.IsRunning("soap-mgr"))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// soapEnvelopeFixture wraps inner XML in a minimal SOAP 1.1 envelope.
func soapEnvelopeFixture(bodyContent string) string {
	return `<?xml version="1.0" encoding="utf-8"?>` +
		`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<soap:Body>` + bodyContent + `</soap:Body>` +
		`</soap:Envelope>`
}
