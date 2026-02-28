// Package batcher groups incoming audit events and flushes them in batches
// to reduce database pressure. A batch is flushed either when it reaches
// MaxBatchSize events or when the FlushInterval elapses — whichever comes first.
package batcher

import (
	"log"
	"sync"
	"time"
)

const (
	// DefaultMaxBatchSize is the maximum number of events accumulated before a forced flush.
	DefaultMaxBatchSize = 100
	// DefaultFlushInterval is the maximum time between flushes.
	DefaultFlushInterval = 5 * time.Second
)

// AuditEvent represents a single audit log entry received from NATS.
type AuditEvent struct {
	ExecutionID string                 `json:"execution_id"`
	FlowID      string                 `json:"flow_id"`
	NodeID      string                 `json:"node_id"`
	NodeType    string                 `json:"node_type"`
	Status      string                 `json:"status"`
	InputData   map[string]interface{} `json:"input"`
	OutputData  map[string]interface{} `json:"output"`
	ErrorMsg    string                 `json:"error"`
	DurationMs  int                    `json:"duration_ms"`
	Timestamp   string                 `json:"timestamp"`
}

// FlushFunc is called with a batch of events to be persisted.
type FlushFunc func(events []AuditEvent) error

// Batcher accumulates events and calls FlushFunc when a threshold is reached.
type Batcher struct {
	mu            sync.Mutex
	buf           []AuditEvent
	maxBatchSize  int
	flushInterval time.Duration
	flushFn       FlushFunc
	ticker        *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// New creates and starts a new Batcher.
// maxBatchSize and flushInterval control when the buffer is flushed.
// If 0 is passed for either, the defaults are used.
func New(maxBatchSize int, flushInterval time.Duration, flushFn FlushFunc) *Batcher {
	if maxBatchSize <= 0 {
		maxBatchSize = DefaultMaxBatchSize
	}
	if flushInterval <= 0 {
		flushInterval = DefaultFlushInterval
	}
	b := &Batcher{
		buf:           make([]AuditEvent, 0, maxBatchSize),
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		flushFn:       flushFn,
		ticker:        time.NewTicker(flushInterval),
		stopCh:        make(chan struct{}),
	}
	b.wg.Add(1)
	go b.run()
	return b
}

// Add enqueues an event. If the buffer reaches maxBatchSize the batch is
// flushed immediately (synchronously under the lock so callers do not block).
func (b *Batcher) Add(event AuditEvent) {
	b.mu.Lock()
	b.buf = append(b.buf, event)
	shouldFlush := len(b.buf) >= b.maxBatchSize
	b.mu.Unlock()

	if shouldFlush {
		b.flush()
	}
}

// Stop flushes any remaining events and shuts down the background ticker.
func (b *Batcher) Stop() {
	close(b.stopCh)
	b.wg.Wait()
	b.flush()
}

// run is the background goroutine that triggers periodic flushes.
func (b *Batcher) run() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ticker.C:
			b.flush()
		case <-b.stopCh:
			b.ticker.Stop()
			return
		}
	}
}

// flush drains the internal buffer and calls flushFn with the collected events.
func (b *Batcher) flush() {
	b.mu.Lock()
	if len(b.buf) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.buf
	b.buf = make([]AuditEvent, 0, b.maxBatchSize)
	b.mu.Unlock()

	if err := b.flushFn(batch); err != nil {
		log.Printf("batcher: flush failed for %d events: %v", len(batch), err)
	}
}
