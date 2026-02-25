package activities

import (
"os"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestMailActivity_ReceiveStub(t *testing.T) {
a := &MailActivity{}
out, err := a.Execute(nil, map[string]interface{}{
"action": "receive",
}, nil)
require.NoError(t, err)
assert.Equal(t, "imap receive not yet implemented", out["note"])
}

func TestMailActivity_UnknownAction(t *testing.T) {
a := &MailActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"action": "fax",
}, nil)
assert.Error(t, err)
}

func TestMailActivity_SendMissingHost(t *testing.T) {
a := &MailActivity{}
_, err := a.Execute(nil, map[string]interface{}{
"action": "send",
}, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "host")
}

func TestMailActivity_SendIntegration(t *testing.T) {
if os.Getenv("FLOWJS_RUN_EXTERNAL_TESTS") != "1" {
t.Skip("skipping external test; set FLOWJS_RUN_EXTERNAL_TESTS=1 to enable")
}
a := &MailActivity{}
out, err := a.Execute(nil, map[string]interface{}{
"action":   "send",
"host":     "smtp.example.com",
"port":     587,
"security": "STARTTLS",
"auth":     map[string]interface{}{"user": "test@example.com", "password": "secret"},
"to":       []interface{}{"dest@example.com"},
"subject":  "Test",
"body":     "Hello from flowjs-works",
}, nil)
require.NoError(t, err)
assert.Equal(t, true, out["sent"])
}
