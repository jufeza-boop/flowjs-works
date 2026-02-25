package activities

import (
"os"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestRabbitMQActivity_MissingURLAMQP(t *testing.T) {
a := &RabbitMQActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"routing_key": "test.queue",
"payload":     map[string]interface{}{"msg": "hello"},
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "url_amqp")
}

func TestRabbitMQActivity_PublishIntegration(t *testing.T) {
if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
}
a := &RabbitMQActivity{}
out, err := a.Execute(nil, map[string]interface{}{
"url_amqp":    "amqp://guest:guest@localhost:5672/",
"routing_key": "test.queue",
"payload":     map[string]interface{}{"event": "test"},
}, nil)
require.NoError(t, err)
assert.Equal(t, true, out["published"])
}
