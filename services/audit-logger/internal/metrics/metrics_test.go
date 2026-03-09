package metrics_test

import (
	"strings"
	"testing"

	"flowjs-works/audit-logger/internal/metrics"

	"github.com/stretchr/testify/assert"
)

func TestWritePrometheus_ContainsAllMetrics(t *testing.T) {
	metrics.EventsReceived.Store(20)
	metrics.EventsPersisted.Store(18)
	metrics.BatchesFlushed.Store(3)
	metrics.BatchFlushErrors.Store(1)
	metrics.HTTPRequestsTotal.Store(5)
	metrics.Ready.Store(1)

	var sb strings.Builder
	metrics.WritePrometheus(&sb)
	out := sb.String()

	expectedMetrics := []string{
		"flowjs_auditlogger_events_received_total 20",
		"flowjs_auditlogger_events_persisted_total 18",
		"flowjs_auditlogger_batches_flushed_total 3",
		"flowjs_auditlogger_batch_flush_errors_total 1",
		"flowjs_auditlogger_http_requests_total 5",
		"flowjs_auditlogger_ready 1",
	}
	for _, m := range expectedMetrics {
		assert.Contains(t, out, m, "expected metric %q in output", m)
	}
}

func TestWritePrometheus_TypeAndHelpLines(t *testing.T) {
	var sb strings.Builder
	metrics.WritePrometheus(&sb)
	out := sb.String()

	assert.Contains(t, out, "# HELP flowjs_auditlogger_events_received_total")
	assert.Contains(t, out, "# TYPE flowjs_auditlogger_events_received_total counter")
	assert.Contains(t, out, "# HELP flowjs_auditlogger_ready")
	assert.Contains(t, out, "# TYPE flowjs_auditlogger_ready gauge")
}

func TestReadyFlag(t *testing.T) {
	metrics.Ready.Store(0)
	assert.Equal(t, int32(0), metrics.Ready.Load())
	metrics.Ready.Store(1)
	assert.Equal(t, int32(1), metrics.Ready.Load())
}
