package triggers

import (
	"context"
	"fmt"
	"log"
	"time"

	"flowjs-works/engine/internal/models"

	"github.com/robfig/cron/v3"
)

// cronTrigger fires the process on a cron schedule.
// It relies on robfig/cron v3, which is thread-safe by design.
type cronTrigger struct {
	executor  Executor
	scheduler *cron.Cron
}

func newCronTrigger(executor Executor) *cronTrigger {
	return &cronTrigger{
		executor: executor,
	}
}

// Start parses the cron expression from the trigger config and schedules the job.
func (t *cronTrigger) Start(ctx context.Context, proc *models.Process) error {
	expr, err := cronExpression(proc.Trigger.Config)
	if err != nil {
		return fmt.Errorf("cron_trigger: %w", err)
	}

	// Keep a local copy so the closure does not reference the outer variable.
	procCopy := *proc
	t.scheduler = cron.New(cron.WithSeconds())

	_, addErr := t.scheduler.AddFunc(expr, func() {
		triggerData := map[string]interface{}{
			"datetime": time.Now().UTC().Format(time.RFC3339),
		}
		if _, execErr := t.executor.Execute(&procCopy, triggerData); execErr != nil {
			log.Printf("cron_trigger: execution error for %q: %v", procCopy.Definition.ID, execErr)
		}
	})
	if addErr != nil {
		return fmt.Errorf("cron_trigger: add cron job: %w", addErr)
	}

	t.scheduler.Start()
	log.Printf("cron_trigger: scheduled %q with expression %q", proc.Definition.ID, expr)
	return nil
}

// Stop halts the scheduler and waits for any in-flight job to complete.
func (t *cronTrigger) Stop() error {
	if t.scheduler != nil {
		ctx := t.scheduler.Stop()
		// Wait until the running job finishes (or context is done).
		select {
		case <-ctx.Done():
		case <-time.After(30 * time.Second):
			log.Printf("cron_trigger: timed out waiting for job to finish")
		}
		t.scheduler = nil
	}
	return nil
}

func (t *cronTrigger) Type() string { return "cron" }

// cronExpression extracts the "expression" field from the trigger config.
func cronExpression(config map[string]interface{}) (string, error) {
	if config == nil {
		return "", fmt.Errorf("trigger config is nil; expected {\"expression\":\"...\"}")
	}
	raw, ok := config["expression"]
	if !ok {
		return "", fmt.Errorf("trigger config missing required field \"expression\"")
	}
	expr, ok := raw.(string)
	if !ok || expr == "" {
		return "", fmt.Errorf("trigger config field \"expression\" must be a non-empty string")
	}
	return expr, nil
}
