// Package metrics provides a lightweight Prometheus text-format exposition layer
// for the engine service. It exposes atomic counters and gauges that can be
// scraped by a Prometheus server via the GET /metrics HTTP endpoint.
package metrics

import (
	"fmt"
	"io"
	"sync/atomic"
)

// Counters are global singletons incremented throughout the engine lifecycle.
var (
	ExecutionsTotal   atomic.Int64 // total flow executions started
	ExecutionsSuccess atomic.Int64 // executions that completed without error
	ExecutionsError   atomic.Int64 // executions that ended with an error
	HTTPRequestsTotal atomic.Int64 // total inbound HTTP requests handled
	NATSPublishTotal  atomic.Int64 // total NATS audit messages published
	NATSPublishErrors atomic.Int64 // NATS publish failures

	// Ready signals that the service has finished initialising and is ready to
	// serve traffic. 0 = not ready, 1 = ready.
	Ready atomic.Int32
)

// WritePrometheus writes all metrics in the Prometheus text exposition format
// to w. The format is compatible with Prometheus scrape endpoints.
func WritePrometheus(w io.Writer) {
	writeCounter(w, "flowjs_engine_executions_total",
		"Total number of flow executions started.", ExecutionsTotal.Load())
	writeCounter(w, "flowjs_engine_executions_success_total",
		"Total number of flow executions that completed without error.", ExecutionsSuccess.Load())
	writeCounter(w, "flowjs_engine_executions_error_total",
		"Total number of flow executions that ended with an error.", ExecutionsError.Load())
	writeCounter(w, "flowjs_engine_http_requests_total",
		"Total number of inbound HTTP requests handled by the engine.", HTTPRequestsTotal.Load())
	writeCounter(w, "flowjs_engine_nats_publish_total",
		"Total number of NATS audit messages published.", NATSPublishTotal.Load())
	writeCounter(w, "flowjs_engine_nats_publish_errors_total",
		"Total number of NATS publish failures.", NATSPublishErrors.Load())
	writeGauge(w, "flowjs_engine_ready",
		"1 when the engine is initialised and ready to serve traffic, 0 otherwise.",
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
