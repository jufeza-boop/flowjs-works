package activities

import (
"encoding/json"
"fmt"

amqp "github.com/rabbitmq/amqp091-go"

fmodels "flowjs-works/engine/internal/models"
)

// RabbitMQActivity implements the `rabbitmq` producer node type.
// config fields:
//   url_amqp:    AMQP URL (required)
//   vhost:       virtual host (default "/")
//   exchange:    exchange name (default "")
//   routing_key: routing key (required)
//   payload:     message body (any — serialised to JSON)
//   properties:  map with optional delivery_mode(int), content_type(string)
type RabbitMQActivity struct{}

func (a *RabbitMQActivity) Name() string { return "rabbitmq" }

func (a *RabbitMQActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *fmodels.ExecutionContext) (map[string]interface{}, error) {
urlAMQP, ok := config["url_amqp"].(string)
if !ok || urlAMQP == "" {
return nil, fmt.Errorf("rabbitmq activity: missing required config field 'url_amqp'")
}
routingKey, _ := config["routing_key"].(string)
exchange, _ := config["exchange"].(string)

payload := config["payload"]
payloadBytes, err := json.Marshal(payload)
if err != nil {
return nil, fmt.Errorf("rabbitmq activity: failed to marshal payload: %w", err)
}

contentType := "application/json"
var deliveryMode uint8 = 1
if props, ok := config["properties"].(map[string]interface{}); ok {
if ct, ok := props["content_type"].(string); ok {
contentType = ct
}
switch v := props["delivery_mode"].(type) {
case int:
deliveryMode = uint8(v)
case float64:
deliveryMode = uint8(v)
}
}

conn, err := amqp.Dial(urlAMQP)
if err != nil {
return nil, fmt.Errorf("rabbitmq activity: failed to connect: %w", err)
}
defer conn.Close()

ch, err := conn.Channel()
if err != nil {
return nil, fmt.Errorf("rabbitmq activity: failed to open channel: %w", err)
}
defer ch.Close()

err = ch.Publish(
exchange,
routingKey,
false,
false,
amqp.Publishing{
ContentType:  contentType,
DeliveryMode: deliveryMode,
Body:         payloadBytes,
},
)
if err != nil {
return nil, fmt.Errorf("rabbitmq activity: failed to publish: %w", err)
}

return map[string]interface{}{
"published":   true,
"routing_key": routingKey,
}, nil
}
