package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowjs-works/orchestrator/internal/controller"
)

// setupFakeK8s creates a fake Kubernetes API server that records requests
// and returns 200 OK for creates and 404 for the initial PUT (to exercise
// the PUT→POST fallback path in Deploy).
func setupFakeK8s(t *testing.T) (string, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPut:
			// Simulate resource not found so controller falls back to POST.
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "created"})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &calls
}

func TestFlowController_Deploy(t *testing.T) {
	apiURL, calls := setupFakeK8s(t)

	// Provide a fake token so the controller doesn't try to read from disk.
	t.Setenv("KUBECONFIG_TOKEN", "fake-token")
	t.Setenv("KUBECONFIG_API_SERVER", apiURL)

	cfg := controller.Config{
		Namespace:   "flowjs",
		EngineImage: "flowjs-engine:test",
		NATSUrl:     "nats://localhost:4222",
	}

	ctrl, err := controller.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := ctrl.Deploy(t.Context(), "my-flow"); err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	// Expect a PUT (→ 404) and then a POST.
	if len(*calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d: %v", len(*calls), *calls)
	}
	if !strings.HasPrefix((*calls)[0], "PUT") {
		t.Errorf("first call should be PUT, got %q", (*calls)[0])
	}
	if !strings.HasPrefix((*calls)[1], "POST") {
		t.Errorf("second call should be POST, got %q", (*calls)[1])
	}
}

func TestFlowController_Stop(t *testing.T) {
	apiURL, calls := setupFakeK8s(t)
	t.Setenv("KUBECONFIG_TOKEN", "fake-token")
	t.Setenv("KUBECONFIG_API_SERVER", apiURL)

	cfg := controller.Config{Namespace: "flowjs"}
	ctrl, err := controller.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := ctrl.Stop(t.Context(), "my-flow"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if len(*calls) < 1 || !strings.HasPrefix((*calls)[0], "DELETE") {
		t.Errorf("expected DELETE call, got %v", *calls)
	}
}

func TestDeploymentName_Truncation(t *testing.T) {
	// We can test the side-effect: Deploy sends a request with a path that
	// contains the deployment name. Names must be at most 63 chars.
	apiURL, calls := setupFakeK8s(t)
	t.Setenv("KUBECONFIG_TOKEN", "fake-token")
	t.Setenv("KUBECONFIG_API_SERVER", apiURL)

	cfg := controller.Config{Namespace: "flowjs"}
	ctrl, _ := controller.New(cfg)
	longID := strings.Repeat("x", 100)
	_ = ctrl.Deploy(t.Context(), longID)

	for _, c := range *calls {
		// Extract path component after last "/"
		parts := strings.Split(c, "/")
		name := parts[len(parts)-1]
		if len(name) > 63 {
			t.Errorf("deployment name %q exceeds 63 chars", name)
		}
	}
}
