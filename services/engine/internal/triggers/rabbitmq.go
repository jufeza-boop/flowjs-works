package triggers

import (
	"context"
	"fmt"
	"log"
	"time"

	"flowjs-works/engine/internal/models"

	amqp "github.com/rabbitmq/amqp091-go"
)

// consumerDrainTimeout is the time Stop() waits for in-flight deliveries to
// complete before closing the AMQP connection.
const consumerDrainTimeout = 100 * time.Millisecond
// message received. Each delivery is ACKed on successful execution.
type rabbitMQTrigger struct {
	executor  Executor
	conn      *amqp.Connection
	channel   *amqp.Channel
	done      chan struct{}
	processID string
}

func newRabbitMQTrigger(executor Executor) *rabbitMQTrigger {
	return &rabbitMQTrigger{executor: executor}
}

// Start connects to the AMQP broker, sets up the consumer, and begins consuming
// in a background goroutine.
func (t *rabbitMQTrigger) Start(ctx context.Context, proc *models.Process) error {
	urlAMQP, queue, vhost, err := rabbitmqTriggerConfig(proc.Trigger.Config)
	if err != nil {
		return fmt.Errorf("rabbitmq_trigger: %w", err)
	}

	conn, err := amqp.Dial(urlAMQP)
	if err != nil {
		return fmt.Errorf("rabbitmq_trigger: dial %q: %w", urlAMQP, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq_trigger: open channel: %w", err)
	}

	_ = vhost // vhost is embedded in the AMQP URL by convention; kept for DSL completeness

	deliveries, err := ch.Consume(
		queue,           // queue name
		"flowjs-runner", // consumer tag
		false,           // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("rabbitmq_trigger: consume %q: %w", queue, err)
	}

	t.conn = conn
	t.channel = ch
	t.done = make(chan struct{})
	t.processID = proc.Definition.ID

	procCopy := *proc
	go t.consume(deliveries, &procCopy)

	log.Printf("rabbitmq_trigger: listening on queue %q for process %q", queue, proc.Definition.ID)
	return nil
}

func (t *rabbitMQTrigger) consume(deliveries <-chan amqp.Delivery, proc *models.Process) {
	for {
		select {
		case <-t.done:
			return
		case d, ok := <-deliveries:
			if !ok {
				log.Printf("rabbitmq_trigger: delivery channel closed for %q", t.processID)
				return
			}
			t.handleDelivery(d, proc)
		}
	}
}

func (t *rabbitMQTrigger) handleDelivery(d amqp.Delivery, proc *models.Process) {
	triggerData := map[string]interface{}{
		"payload": string(d.Body),
		"properties": map[string]interface{}{
			"delivery_mode": int(d.DeliveryMode),
			"headers":       amqpHeadersToMap(d.Headers),
		},
	}

	if _, err := t.executor.Execute(proc, triggerData); err != nil {
		log.Printf("rabbitmq_trigger: execution error for %q: %v — NAcking message", proc.Definition.ID, err)
		_ = d.Nack(false, true) // requeue on failure
		return
	}

	_ = d.Ack(false)
}

// Stop closes the channel, connection, and signals the consumer goroutine.
func (t *rabbitMQTrigger) Stop() error {
	if t.done != nil {
		close(t.done)
		t.done = nil
	}
	if t.channel != nil {
		if err := t.channel.Cancel("flowjs-runner", false); err != nil {
			log.Printf("rabbitmq_trigger: cancel consumer: %v", err)
		}
		t.channel.Close()
		t.channel = nil
	}
	if t.conn != nil {
		// Give the consumer goroutine a short window to drain.
		time.Sleep(consumerDrainTimeout)
		t.conn.Close()
		t.conn = nil
	}
	return nil
}

func (t *rabbitMQTrigger) Type() string { return "rabbitmq" }

// rabbitmqTriggerConfig extracts AMQP connection parameters from trigger config.
func rabbitmqTriggerConfig(config map[string]interface{}) (urlAMQP, queue, vhost string, err error) {
	if config == nil {
		return "", "", "", fmt.Errorf("trigger config is nil; expected {\"url_amqp\":\"...\",\"queue\":\"...\"}")
	}
	urlAMQP, _ = config["url_amqp"].(string)
	if urlAMQP == "" {
		return "", "", "", fmt.Errorf("trigger config missing required field \"url_amqp\"")
	}
	queue, _ = config["queue"].(string)
	if queue == "" {
		return "", "", "", fmt.Errorf("trigger config missing required field \"queue\"")
	}
	vhost, _ = config["vhost"].(string)
	return urlAMQP, queue, vhost, nil
}

// amqpHeadersToMap converts amqp.Table into a plain map for JSON serialization.
func amqpHeadersToMap(headers amqp.Table) map[string]interface{} {
	out := make(map[string]interface{}, len(headers))
	for k, v := range headers {
		out[k] = v
	}
	return out
}
