package triggers

import (
	"context"
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

	proc := buildProcess("p2", "soap", nil)
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
