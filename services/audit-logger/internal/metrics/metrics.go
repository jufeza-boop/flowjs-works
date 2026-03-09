// Package metrics provides a lightweight Prometheus text-format exposition layer
// for the audit-logger service. It exposes atomic counters and gauges scraped
// by a Prometheus server via GET /metrics.
package metrics

import (
	"fmt"
	"io"
	"sync/atomic"
)

// Counters are global singletons incremented throughout the audit-logger lifecycle.
var (
	EventsReceived   atomic.Int64 // total audit events received from NATS
	EventsPersisted  atomic.Int64 // total audit events persisted to PostgreSQL
	BatchesFlushed   atomic.Int64 // total batch flush operations executed
	BatchFlushErrors atomic.Int64 // batch flush failures
	HTTPRequestsTotal atomic.Int64 // total inbound HTTP requests handled

	// Ready signals readiness: 0 = not ready, 1 = ready.
	Ready atomic.Int32
)

// WritePrometheus writes all metrics in the Prometheus text exposition format to w.
func WritePrometheus(w io.Writer) {
	writeCounter(w, "flowjs_auditlogger_events_received_total",
		"Total number of audit events received from NATS.", EventsReceived.Load())
	writeCounter(w, "flowjs_auditlogger_events_persisted_total",
		"Total number of audit events persisted to PostgreSQL.", EventsPersisted.Load())
	writeCounter(w, "flowjs_auditlogger_batches_flushed_total",
		"Total number of batch flush operations executed.", BatchesFlushed.Load())
	writeCounter(w, "flowjs_auditlogger_batch_flush_errors_total",
		"Total number of batch flush failures.", BatchFlushErrors.Load())
	writeCounter(w, "flowjs_auditlogger_http_requests_total",
		"Total number of inbound HTTP requests handled.", HTTPRequestsTotal.Load())
	writeGauge(w, "flowjs_auditlogger_ready",
		"1 when the audit-logger is initialised and ready, 0 otherwise.",
		int64(Ready.Load()))
}

func writeCounter(w io.Writer, name, help string, value int64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s counter\n", name)
	fmt.Fprintf(w, "%s %d\n", name, value)
}

func writeGauge(w io.Writer, name, help string, value int64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", name)
	fmt.Fprintf(w, "%s %d\n", name, value)
}
