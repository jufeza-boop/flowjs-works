// Package controller manages the lifecycle of per-flow Kubernetes Deployments.
// When a flow is deployed via the engine, the orchestrator creates a dedicated
// Deployment (one engine replica) scoped to that single flow. When the flow is
// stopped, the Deployment is deleted.
//
// The controller communicates with the Kubernetes API server using the in-cluster
// service-account token mounted at /var/run/secrets/kubernetes.io/serviceaccount.
// Outside a cluster, set the KUBECONFIG environment variable to use a local
// kubeconfig file.
package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	// k8sAPIServer is the in-cluster Kubernetes API server base URL.
	k8sAPIServer = "https://kubernetes.default.svc"
	// k8sTokenFile is the service-account bearer token.
	k8sTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	// k8sCACertFile is the service-account CA bundle for TLS verification.
	k8sCACertFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Config holds the per-flow deployment parameters.
type Config struct {
	// Namespace is the Kubernetes namespace where flow deployments are created.
	Namespace string
	// EngineImage is the Docker image used for each per-flow engine pod.
	EngineImage string
	// NATSUrl is injected into each flow pod so it can publish audit events.
	NATSUrl string
	// DatabaseURL is injected into each flow pod for secrets/process store.
	DatabaseURL string
	// AESKey is the secrets encryption key injected into each flow pod.
	AESKey string
}

// FlowController creates and deletes per-flow Kubernetes Deployments.
type FlowController struct {
	cfg    Config
	client *http.Client
	token  string
	apiURL string
}

// New creates a FlowController. It reads the in-cluster service-account token
// automatically. When running outside a cluster provide KUBECONFIG_TOKEN and
// KUBECONFIG_API_SERVER environment variables.
func New(cfg Config) (*FlowController, error) {
	token, err := readToken()
	if err != nil {
		return nil, fmt.Errorf("orchestrator: read k8s token: %w", err)
	}
	apiURL := envOrDefault("KUBECONFIG_API_SERVER", k8sAPIServer)

	return &FlowController{
		cfg:    cfg,
		client: &http.Client{},
		token:  token,
		apiURL: apiURL,
	}, nil
}

// Deploy creates (or replaces) a Kubernetes Deployment for the given flow.
// The Deployment runs a single engine replica configured to serve only this flow.
func (c *FlowController) Deploy(ctx context.Context, flowID string) error {
	manifest := c.buildDeploymentManifest(flowID)
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("orchestrator: marshal deployment for %q: %w", flowID, err)
	}

	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s",
		c.apiURL, c.cfg.Namespace, deploymentName(flowID))

	// Try to update first (PUT); fall back to create (POST) on 404.
	if err := c.doRequest(ctx, http.MethodPut, url, body); err != nil {
		if !isNotFound(err) {
			return err
		}
		createURL := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments",
			c.apiURL, c.cfg.Namespace)
		if err := c.doRequest(ctx, http.MethodPost, createURL, body); err != nil {
			return fmt.Errorf("orchestrator: create deployment for %q: %w", flowID, err)
		}
	}
	log.Printf("orchestrator: deployed flow %q as Deployment %s", flowID, deploymentName(flowID))
	return nil
}

// Stop deletes the Kubernetes Deployment for the given flow.
func (c *FlowController) Stop(ctx context.Context, flowID string) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s",
		c.apiURL, c.cfg.Namespace, deploymentName(flowID))
	if err := c.doRequest(ctx, http.MethodDelete, url, nil); err != nil && !isNotFound(err) {
		return fmt.Errorf("orchestrator: delete deployment for %q: %w", flowID, err)
	}
	log.Printf("orchestrator: stopped flow %q (deleted Deployment %s)", flowID, deploymentName(flowID))
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// deploymentName returns a safe K8s resource name derived from a flow ID.
func deploymentName(flowID string) string {
	name := "flow-" + strings.ToLower(flowID)
	// K8s names must be <= 63 chars.
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// buildDeploymentManifest returns a minimal Kubernetes Deployment manifest as a
// map ready for JSON marshalling. It passes the flow-specific environment
// through a ConfigMap-less direct env block to keep each deployment self-contained.
func (c *FlowController) buildDeploymentManifest(flowID string) map[string]interface{} {
	name := deploymentName(flowID)
	one := int64(1)
	_ = one

	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": c.cfg.Namespace,
			"labels": map[string]string{
				"app":                          name,
				"flowjs.works/flow-id":         flowID,
				"app.kubernetes.io/managed-by": "flowjs-orchestrator",
			},
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{"app": name},
			},
			"strategy": map[string]interface{}{
				"type": "RollingUpdate",
				"rollingUpdate": map[string]interface{}{
					"maxSurge":       1,
					"maxUnavailable": 0,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"app": name},
				},
				"spec": map[string]interface{}{
					"terminationGracePeriodSeconds": 30,
					"containers": []map[string]interface{}{
						{
							"name":            "engine",
							"image":           c.cfg.EngineImage,
							"imagePullPolicy": "IfNotPresent",
							"ports": []map[string]interface{}{
								{"containerPort": 9090, "name": "http"},
							},
							"env": []map[string]interface{}{
								{"name": "APP_ENV", "value": "production"},
								{"name": "FLOW_ID", "value": flowID},
								{"name": "NATS_URL", "value": c.cfg.NATSUrl},
								{"name": "DATABASE_URL", "value": c.cfg.DatabaseURL},
								{"name": "SECRETS_AES_KEY", "value": c.cfg.AESKey},
								{"name": "HTTP_ADDR", "value": ":9090"},
							},
							"livenessProbe": map[string]interface{}{
								"httpGet":             map[string]interface{}{"path": "/health", "port": 9090},
								"initialDelaySeconds": 10,
								"periodSeconds":       30,
							},
							"readinessProbe": map[string]interface{}{
								"httpGet":             map[string]interface{}{"path": "/ready", "port": 9090},
								"initialDelaySeconds": 5,
								"periodSeconds":       10,
							},
							"resources": map[string]interface{}{
								"requests": map[string]string{"cpu": "100m", "memory": "128Mi"},
								"limits":   map[string]string{"cpu": "500m", "memory": "512Mi"},
							},
						},
					},
				},
			},
		},
	}
}

// doRequest executes an authenticated HTTP request against the K8s API server.
func (c *FlowController) doRequest(ctx context.Context, method, url string, body []byte) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &k8sError{code: http.StatusNotFound}
	}
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("k8s API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type k8sError struct{ code int }

func (e *k8sError) Error() string { return fmt.Sprintf("k8s error %d", e.code) }

func isNotFound(err error) bool {
	if e, ok := err.(*k8sError); ok {
		return e.code == http.StatusNotFound
	}
	return false
}

func readToken() (string, error) {
	// Allow override for local development / testing.
	if tok := os.Getenv("KUBECONFIG_TOKEN"); tok != "" {
		return tok, nil
	}
	data, err := os.ReadFile(k8sTokenFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
