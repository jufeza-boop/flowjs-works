// Package triggers manages the lifecycle of active flow triggers.
// Each deployed process owns exactly one TriggerHandler that starts/stops
// according to the trigger type defined in its DSL.
package triggers

import (
	"context"
	"fmt"
	"log"
	"sync"

	"flowjs-works/engine/internal/models"
)

// Executor is the subset of engine.ProcessExecutor used by triggers.
// Keeping a narrow interface avoids an import cycle.
type Executor interface {
	Execute(process *models.Process, triggerData map[string]interface{}) (*models.ExecutionContext, error)
}

// TriggerHandler is the lifecycle interface every trigger must implement.
type TriggerHandler interface {
	// Start activates the trigger. For cron and queue-based triggers this
	// starts background goroutines; for REST/MCP it registers routes.
	Start(ctx context.Context, proc *models.Process) error
	// Stop deactivates the trigger and releases all resources.
	Stop() error
	// Type returns the DSL trigger type string (e.g. "cron").
	Type() string
}

// Manager maintains a registry of running triggers, keyed by process ID.
// It is safe for concurrent use.
type Manager struct {
	executor Executor
	running  map[string]TriggerHandler
	mu       sync.Mutex
}

// NewManager creates a Manager that will use executor to run flows when a
// trigger fires.
func NewManager(executor Executor) *Manager {
	return &Manager{
		executor: executor,
		running:  make(map[string]TriggerHandler),
	}
}

// Deploy starts the appropriate trigger for proc. If the process is already
// deployed, it is stopped first and then restarted (hot-reload semantics).
func (m *Manager) Deploy(proc *models.Process) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any existing handler for this process.
	if h, ok := m.running[proc.Definition.ID]; ok {
		log.Printf("triggers: redeploying %q — stopping previous %s trigger", proc.Definition.ID, h.Type())
		if err := h.Stop(); err != nil {
			log.Printf("triggers: warning: stop previous %q trigger: %v", proc.Definition.ID, err)
		}
		delete(m.running, proc.Definition.ID)
	}

	handler, err := m.newHandler(proc)
	if err != nil {
		return fmt.Errorf("triggers: create handler for %q: %w", proc.Definition.ID, err)
	}

	if err := handler.Start(context.Background(), proc); err != nil {
		return fmt.Errorf("triggers: start %s trigger for %q: %w", proc.Trigger.Type, proc.Definition.ID, err)
	}

	m.running[proc.Definition.ID] = handler
	log.Printf("triggers: deployed %s trigger for process %q", proc.Trigger.Type, proc.Definition.ID)
	return nil
}

// Stop deactivates the trigger for processID.
func (m *Manager) Stop(processID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.running[processID]
	if !ok {
		return fmt.Errorf("triggers: process %q is not currently deployed", processID)
	}
	if err := h.Stop(); err != nil {
		return fmt.Errorf("triggers: stop %s trigger for %q: %w", h.Type(), processID, err)
	}
	delete(m.running, processID)
	log.Printf("triggers: stopped trigger for process %q", processID)
	return nil
}

// IsRunning reports whether a trigger is active for processID.
func (m *Manager) IsRunning(processID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[processID]
	return ok
}

// TriggerType returns the trigger type string for a currently-deployed process,
// or an empty string if the process is not running.
func (m *Manager) TriggerType(processID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.running[processID]; ok {
		return h.Type()
	}
	return ""
}

// StopAll deactivates every running trigger. Useful during shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, h := range m.running {
		if err := h.Stop(); err != nil {
			log.Printf("triggers: warning: stop %q: %v", id, err)
		}
	}
	m.running = make(map[string]TriggerHandler)
}

// newHandler selects the correct TriggerHandler implementation for proc.
func (m *Manager) newHandler(proc *models.Process) (TriggerHandler, error) {
	switch proc.Trigger.Type {
	case "cron":
		return newCronTrigger(m.executor), nil
	case "rabbitmq":
		return newRabbitMQTrigger(m.executor), nil
	case "mcp":
		return newMCPTrigger(m.executor), nil
	case "rest":
		return newRESTTrigger(m.executor), nil
	case "soap":
		return newSOAPTrigger(m.executor), nil
	case "manual":
		return &manualTrigger{}, nil
	default:
		return nil, fmt.Errorf("unsupported trigger type: %q", proc.Trigger.Type)
	}
}

// ---------------------------------------------------------------------------
// manualTrigger — no-op; the flow is started via the /api/v1/processes/{id}/run endpoint.
// ---------------------------------------------------------------------------

type manualTrigger struct{}

func (t *manualTrigger) Start(_ context.Context, _ *models.Process) error { return nil }
func (t *manualTrigger) Stop() error                                       { return nil }
func (t *manualTrigger) Type() string                                      { return "manual" }
