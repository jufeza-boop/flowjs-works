// Package subscriber handles NATS connectivity and routes incoming audit messages
// to the Batcher for accumulation before bulk database persistence.
package subscriber

import (
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	"flowjs-works/audit-logger/internal/batcher"
)

const auditSubject = "audit.logs"

// Subscriber wraps a NATS connection and forwards messages to a Batcher.
type Subscriber struct {
	conn    *nats.Conn
	batcher *batcher.Batcher
	sub     *nats.Subscription
}

// New connects to NATS with automatic reconnection enabled and returns a Subscriber.
// It retries the initial connection up to maxRetries times.
func New(natsURL string, b *batcher.Batcher) (*Subscriber, error) {
	const maxRetries = 10

	opts := []nats.Option{
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // unlimited reconnects
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("audit-logger: NATS disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("audit-logger: NATS reconnected to %s", nc.ConnectedUrl())
		}),
	}

	var (
		nc  *nats.Conn
		err error
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		nc, err = nats.Connect(natsURL, opts...)
		if err == nil {
			log.Printf("audit-logger: connected to NATS at %s (attempt %d)", natsURL, attempt)
			break
		}
		wait := time.Duration(attempt) * time.Second
		log.Printf("audit-logger: NATS not ready (attempt %d/%d): %v — retrying in %s",
			attempt, maxRetries, err, wait)
		time.Sleep(wait)
	}
	if err != nil {
		return nil, err
	}

	return &Subscriber{conn: nc, batcher: b}, nil
}

// Start registers the subscription on audit.logs and begins processing messages.
func (s *Subscriber) Start() error {
	sub, err := s.conn.Subscribe(auditSubject, s.handleMessage)
	if err != nil {
		return err
	}
	s.sub = sub
	log.Printf("audit-logger: subscribed to NATS subject %q", auditSubject)
	return nil
}

// Stop drains the subscription and closes the NATS connection.
func (s *Subscriber) Stop() {
	if s.sub != nil {
		_ = s.sub.Drain()
	}
	if s.conn != nil {
		s.conn.Close()
	}
}

// handleMessage parses an incoming NATS message and enqueues it in the batcher.
func (s *Subscriber) handleMessage(msg *nats.Msg) {
	var event batcher.AuditEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("audit-logger: failed to parse audit event: %v — payload: %s", err, string(msg.Data))
		return
	}
	s.batcher.Add(event)
}
