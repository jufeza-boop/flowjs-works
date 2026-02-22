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
	return "http_request"
}

// Execute performs an HTTP request
func (a *HTTPActivity) Execute(input map[string]interface{}, config map[string]interface{}, ctx *models.ExecutionContext) (map[string]interface{}, error) {
	// Extract configuration
	url, ok := config["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required in config")
	}
	
	method := "GET"
	if methodVal, ok := config["method"].(string); ok {
		method = methodVal
	}
	
	timeout := 30 * time.Second
	if timeoutVal, ok := config["timeout"].(float64); ok {
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
	
	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Try to parse as JSON, fall back to string
	var responseData interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		responseData = string(respBody)
	}
	
	// Prepare output
	output := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        responseData,
	}
	
	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return output, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}
	
	return output, nil
}
