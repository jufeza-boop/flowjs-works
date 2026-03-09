package metrics_test

import (
	"strings"
	"testing"

	"flowjs-works/engine/internal/metrics"

	"github.com/stretchr/testify/assert"
)

func TestWritePrometheus_ContainsAllMetrics(t *testing.T) {
	// Reset all counters before the test.
	metrics.ExecutionsTotal.Store(5)
	metrics.ExecutionsSuccess.Store(3)
	metrics.ExecutionsError.Store(2)
	metrics.HTTPRequestsTotal.Store(10)
	metrics.NATSPublishTotal.Store(7)
	metrics.NATSPublishErrors.Store(1)
	metrics.Ready.Store(1)

	var sb strings.Builder
	metrics.WritePrometheus(&sb)
	out := sb.String()

	expectedMetrics := []string{
		"flowjs_engine_executions_total 5",
		"flowjs_engine_executions_success_total 3",
		"flowjs_engine_executions_error_total 2",
		"flowjs_engine_http_requests_total 10",
		"flowjs_engine_nats_publish_total 7",
		"flowjs_engine_nats_publish_errors_total 1",
		"flowjs_engine_ready 1",
	}
	for _, m := range expectedMetrics {
		assert.Contains(t, out, m, "expected metric %q in output", m)
	}
}

func TestWritePrometheus_TypeAndHelpLines(t *testing.T) {
	var sb strings.Builder
	metrics.WritePrometheus(&sb)
	out := sb.String()

	// Every metric must have a HELP and TYPE header.
	assert.Contains(t, out, "# HELP flowjs_engine_executions_total")
	assert.Contains(t, out, "# TYPE flowjs_engine_executions_total counter")
	assert.Contains(t, out, "# HELP flowjs_engine_ready")
	assert.Contains(t, out, "# TYPE flowjs_engine_ready gauge")
}

func TestReadyFlag(t *testing.T) {
	metrics.Ready.Store(0)
	assert.Equal(t, int32(0), metrics.Ready.Load())
	metrics.Ready.Store(1)
	assert.Equal(t, int32(1), metrics.Ready.Load())
}
