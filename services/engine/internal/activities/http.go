package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"flowjs-works/engine/internal/models"
)

// HTTPActivity makes HTTP requests
type HTTPActivity struct{}

// Name returns the activity type name
func (a *HTTPActivity) Name() string {
	return "http"
}

// Execute performs an HTTP request.
// Network and transport errors are captured in the output under the "error" key rather
// than propagated as fatal Go errors, so the flow can continue and the caller can inspect
// the result via transitions/conditions.  HTTP 4xx/5xx responses are also returned as
// data (not errors) — only the status_code distinguishes success from failure.
func (a *HTTPActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
	// Extract configuration
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required in config")
	}

	method := "GET"
	if methodVal, ok := config["method"].(string); ok && methodVal != "" {
		method = methodVal
	}

	timeout := 30 * time.Second
	if timeoutVal, ok := config["timeout"].(float64); ok && timeoutVal > 0 {
		timeout = time.Duration(timeoutVal) * time.Second
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Prepare request body
	var bodyReader io.Reader
	if body, ok := input["body"]; ok && body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create request
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Auth injection from secrets: token → Bearer header, user+password → Basic auth.
	// Headers set via input["headers"] or config["headers"] below take priority and can
	// override this injected Authorization header.
	if token, ok := config["token"].(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if user, ok := config["user"].(string); ok && user != "" {
		if pass, _ := config["password"].(string); pass != "" {
			req.SetBasicAuth(user, pass)
		}
	}

	if headers, ok := input["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, strVal)
			}
		}
	}

	// Override headers from config
	if headers, ok := config["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, strVal)
			}
		}
	}

	// Execute request — transport errors are captured as output, not fatal errors.
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"status_code": 0,
			"body":        nil,
			"headers":     map[string]interface{}{},
			"error":       err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        nil,
			"headers":     map[string]interface{}{},
			"error":       fmt.Sprintf("failed to read response body: %v", err),
		}, nil
	}

	// Try to parse as JSON, fall back to string
	var responseData interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		responseData = string(respBody)
	}

	// Return full response as output — HTTP 4xx/5xx are data, not fatal errors.
	// The caller can inspect status_code via transitions/conditions.
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        responseData,
	}, nil
}
