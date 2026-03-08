package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"flowjs-works/engine/internal/middleware"
)

// ──────────────────────────────────────────────────────────────────────────────
// SecurityHeaders tests (A02 / A05)
// ──────────────────────────────────────────────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	headers := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"X-XSS-Protection":          "0",
		"Referrer-Policy":           "strict-origin",
	}
	for name, want := range headers {
		assert.Equal(t, want, rec.Header().Get(name), "header %s mismatch", name)
	}
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

// ──────────────────────────────────────────────────────────────────────────────
// CORS tests (A05)
// ──────────────────────────────────────────────────────────────────────────────

func TestCORSAllowsPermittedOrigin(t *testing.T) {
	origins := []string{"http://localhost:5173", "https://app.example.com"}
	handler := middleware.CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORSBlocksUnknownOrigin(t *testing.T) {
	origins := []string{"https://app.example.com"}
	handler := middleware.CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://evil.attacker.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Response should NOT contain the CORS header for unknown origins.
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	// But the request itself is still processed (origin blocking is via header, not rejection)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORSPreflightReturns204(t *testing.T) {
	origins := []string{"https://app.example.com"}
	handler := middleware.CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/flow", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCORSWildcardNotAllowed(t *testing.T) {
	// Validate that we never emit Access-Control-Allow-Origin: * for unknown origins.
	origins := []string{"https://trusted.example.com"}
	handler := middleware.CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://untrusted.net")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEqual(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

// ──────────────────────────────────────────────────────────────────────────────
// AllowedOrigins tests
// ──────────────────────────────────────────────────────────────────────────────

func TestAllowedOriginsFromEnv(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")
	origins := middleware.AllowedOrigins()
	require.Len(t, origins, 2)
	assert.Equal(t, "https://app.example.com", origins[0])
	assert.Equal(t, "https://admin.example.com", origins[1])
}

func TestAllowedOriginsDevFallback(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("APP_ENV", "development")
	origins := middleware.AllowedOrigins()
	require.Len(t, origins, 1)
	assert.Equal(t, "http://localhost:5173", origins[0])
}

// ──────────────────────────────────────────────────────────────────────────────
// Rate Limiter tests (A04)
// ──────────────────────────────────────────────────────────────────────────────

func TestRateLimiterAllowsNormalTraffic(t *testing.T) {
	rl := middleware.NewRateLimiter()
	defer rl.Stop()

	// Under the limit every request should be allowed.
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow("192.168.1.1"), "request %d should be allowed", i+1)
	}
}

func TestRateLimiterBlocksExcessTraffic(t *testing.T) {
	// Use a custom limiter with a very small limit for testing.
	rl := middleware.NewRateLimiterWithConfig(3, time.Minute)
	defer rl.Stop()

	ip := "10.0.0.1"
	for i := 0; i < 3; i++ {
		assert.True(t, rl.Allow(ip), "request %d should be allowed", i+1)
	}
	// 4th request must be denied.
	assert.False(t, rl.Allow(ip), "4th request should be rate limited")
}

func TestRateLimiterWindowReset(t *testing.T) {
	// Short window to verify reset behaviour.
	rl := middleware.NewRateLimiterWithConfig(2, 100*time.Millisecond)
	defer rl.Stop()

	ip := "10.0.0.2"
	assert.True(t, rl.Allow(ip))
	assert.True(t, rl.Allow(ip))
	assert.False(t, rl.Allow(ip)) // over limit

	// After window expires the counter resets.
	time.Sleep(150 * time.Millisecond)
	assert.True(t, rl.Allow(ip), "should be allowed after window reset")
}

func TestRateLimiterMiddlewareReturns429(t *testing.T) {
	rl := middleware.NewRateLimiterWithConfig(1, time.Minute)
	defer rl.Stop()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Middleware(inner)

	makeReq := func() int {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "10.0.0.3:9999"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, makeReq())
	assert.Equal(t, http.StatusTooManyRequests, makeReq())
}

// ──────────────────────────────────────────────────────────────────────────────
// SanitizeError tests (A05)
// ──────────────────────────────────────────────────────────────────────────────

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestSanitizeErrorProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	err := &testError{msg: "pq: password authentication failed for user admin"}
	result := middleware.SanitizeError(err, "internal server error")
	// Must NOT expose DB details in production.
	assert.Equal(t, "internal server error", result)
	assert.NotContains(t, result, "pq:")
}

func TestSanitizeErrorDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	err := &testError{msg: "pq: password authentication failed for user admin"}
	result := middleware.SanitizeError(err, "internal server error")
	// In dev, full error is exposed for debuggability.
	assert.Contains(t, result, "pq:")
}

// ──────────────────────────────────────────────────────────────────────────────
// RequestLogger tests (A09)
// ──────────────────────────────────────────────────────────────────────────────

func TestRequestLoggerPassesThrough(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})
	handler := middleware.RequestLogger(inner)

	req := httptest.NewRequest(http.MethodPost, "/v1/flow", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

// ──────────────────────────────────────────────────────────────────────────────
// clientIP extraction (indirect, via ResponseWriter capture)
// ──────────────────────────────────────────────────────────────────────────────

func TestRequestLoggerCapturesForwardedFor(t *testing.T) {
	// Verify that the middleware processes without panic when X-Forwarded-For is set.
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequestLogger(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:4444"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ──────────────────────────────────────────────────────────────────────────────
// Ensure APP_ENV cleanup doesn't affect other tests
// ──────────────────────────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	// Save and restore APP_ENV to avoid cross-test pollution.
	orig := os.Getenv("APP_ENV")
	code := m.Run()
	_ = os.Setenv("APP_ENV", orig)
	os.Exit(code)
}
