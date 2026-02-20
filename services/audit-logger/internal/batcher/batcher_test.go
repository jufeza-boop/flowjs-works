package batcher

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeEvent is a helper that creates a minimal AuditEvent for testing.
func makeEvent(nodeID string) AuditEvent {
	return AuditEvent{
		ExecutionID: "exec-1",
		NodeID:      nodeID,
		Status:      "SUCCESS",
	}
}

// TestBatcher_FlushOnMaxBatchSize verifies that the buffer is flushed immediately
// when maxBatchSize events have been added, without waiting for the ticker.
func TestBatcher_FlushOnMaxBatchSize(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed [][]AuditEvent
	)

	flushFn := func(events []AuditEvent) error {
		mu.Lock()
		flushed = append(flushed, events)
		mu.Unlock()
		return nil
	}

	b := New(3, 10*time.Second, flushFn)
	defer b.Stop()

	b.Add(makeEvent("n1"))
	b.Add(makeEvent("n2"))
	b.Add(makeEvent("n3")) // should trigger flush

	// Small sleep to let goroutine scheduling settle (the flush is synchronous
	// in Add, so it should already be done).
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, flushed, 1, "expected exactly one flush call")
	assert.Len(t, flushed[0], 3, "expected 3 events in the batch")
}

// TestBatcher_FlushOnInterval verifies that events are flushed periodically
// even when maxBatchSize is not yet reached.
func TestBatcher_FlushOnInterval(t *testing.T) {
	var (
		mu    sync.Mutex
		count int
	)

	flushFn := func(events []AuditEvent) error {
		mu.Lock()
		count += len(events)
		mu.Unlock()
		return nil
	}

	b := New(100, 50*time.Millisecond, flushFn)
	defer b.Stop()

	b.Add(makeEvent("a"))
	b.Add(makeEvent("b"))

	// Wait for at least one tick
	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, count, "both events should have been flushed by the ticker")
}

// TestBatcher_StopFlushesPendingEvents verifies that Stop() flushes the remaining
// buffered events even if neither maxBatchSize nor interval was reached.
func TestBatcher_StopFlushesPendingEvents(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed []AuditEvent
	)

	flushFn := func(events []AuditEvent) error {
		mu.Lock()
		flushed = append(flushed, events...)
		mu.Unlock()
		return nil
	}

	b := New(100, 10*time.Second, flushFn)
	b.Add(makeEvent("x"))
	b.Stop() // must flush before returning

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 1)
	assert.Equal(t, "x", flushed[0].NodeID)
}

// TestBatcher_EmptyBufferNoFlushCall verifies that flushFn is NOT invoked when
// the buffer is empty (avoids unnecessary DB round-trips).
func TestBatcher_EmptyBufferNoFlushCall(t *testing.T) {
	var called atomic.Int32

	flushFn := func(events []AuditEvent) error {
		called.Add(1)
		return nil
	}

	b := New(100, 50*time.Millisecond, flushFn)
	// No events added — wait for multiple ticks.
	time.Sleep(150 * time.Millisecond)
	b.Stop()

	assert.Equal(t, int32(0), called.Load(), "flushFn should not be called on empty buffer")
}

// TestBatcher_MultipleFlushesAccumulation verifies that events added across
// multiple batches are all eventually delivered exactly once.
func TestBatcher_MultipleFlushesAccumulation(t *testing.T) {
	var (
		mu    sync.Mutex
		total int
	)

	flushFn := func(events []AuditEvent) error {
		mu.Lock()
		total += len(events)
		mu.Unlock()
		return nil
	}

	b := New(2, 10*time.Second, flushFn) // flush every 2 events
	defer b.Stop()

	for i := 0; i < 10; i++ {
		b.Add(makeEvent("n"))
	}

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 10, total, "all 10 events must be flushed across 5 batches")
}

// TestBatcher_ConcurrentAdds verifies that the batcher is safe for concurrent use.
func TestBatcher_ConcurrentAdds(t *testing.T) {
	var total atomic.Int32

	flushFn := func(events []AuditEvent) error {
		total.Add(int32(len(events)))
		return nil
	}

	b := New(10, 10*time.Second, flushFn)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Add(makeEvent("concurrent"))
		}()
	}
	wg.Wait()
	b.Stop() // flushes remainder

	assert.Equal(t, int32(50), total.Load(), "all concurrent events must be persisted")
}
