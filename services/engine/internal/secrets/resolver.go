// Package secrets provides the SecretResolver interface and built-in implementations
// used to inject credentials into node configs at execution time.
package secrets

import "context"

// SecretResolver resolves a named secret reference to a map of key/value pairs
// that are merged into the node config before execution.
type SecretResolver interface {
	Resolve(ctx context.Context, ref string) (map[string]interface{}, error)
}

// NoopResolver always returns an empty map (secrets disabled / testing).
type NoopResolver struct{}

func (r *NoopResolver) Resolve(_ context.Context, _ string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
