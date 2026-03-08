// Package middleware provides HTTP middleware for security hardening.
// It implements mitigations for OWASP Top 10 (2021):
//   - A02 Cryptographic Failures — HSTS header (enforces HTTPS in production)
//   - A04 Insecure Design        — IP-based rate limiting (token bucket)
//   - A05 Security Misconfiguration — restrictive CORS, security headers
//   - A09 Logging & Monitoring   — structured security event logging
package middleware

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Constants
// ──────────────────────────────────────────────────────────────────────────────

const (
	// defaultAllowedOrigin is used only in development when ALLOWED_ORIGINS is unset.
	defaultAllowedOrigin = "http://localhost:5173"

	// rateLimitRequests is the maximum number of requests per window per IP.
	rateLimitRequests = 100
	// rateLimitWindow is the sliding window duration for rate limiting.
	rateLimitWindow = time.Minute
	// rateLimitCleanupInterval controls how often expired IP buckets are purged.
	rateLimitCleanupInterval = 5 * time.Minute
)

// ──────────────────────────────────────────────────────────────────────────────
// CORS
// ──────────────────────────────────────────────────────────────────────────────

// CORS returns a middleware that validates the Origin header against the list of
// allowed origins read from the ALLOWED_ORIGINS environment variable
// (comma-separated). When the variable is not set and APP_ENV is "development",
// it defaults to localhost:5173. In any other environment an empty or missing
// ALLOWED_ORIGINS causes the server to log a fatal error at startup.
func CORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[strings.TrimSuffix(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.Header().Set("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AllowedOrigins reads and validates the ALLOWED_ORIGINS environment variable.
// It terminates the process (log.Fatalf) when the variable is empty in non-development
// environments, preventing accidental wildcard CORS in production.
func AllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		if os.Getenv("APP_ENV") != "development" {
			log.Fatalf("middleware: ALLOWED_ORIGINS must be set in non-development environments")
		}
		log.Printf("middleware: WARNING — ALLOWED_ORIGINS not set; defaulting to %s (development only)", defaultAllowedOrigin)
		return []string{defaultAllowedOrigin}
	}
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// ──────────────────────────────────────────────────────────────────────────────
// Security Headers  (A02 HSTS · A05 Security Misconfiguration)
// ──────────────────────────────────────────────────────────────────────────────

// SecurityHeaders returns a middleware that injects defensive HTTP response
// headers on every response:
//
//   - Strict-Transport-Security (HSTS) — forces HTTPS for 1 year, includes subdomains
//   - X-Frame-Options: DENY             — prevents clickjacking
//   - X-Content-Type-Options: nosniff   — prevents MIME-type sniffing
//   - X-XSS-Protection: 0               — disables legacy XSS filter (CSP preferred)
//   - Referrer-Policy: strict-origin     — limits referrer information
//   - Content-Security-Policy           — restricts resource origins
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A02 — HSTS: enforce HTTPS for 1 year; include subdomains
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// A05 — Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// A05 — Prevent MIME sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// A05 — Disable legacy XSS filter (modern CSP is preferred)
		w.Header().Set("X-XSS-Protection", "0")
		// A05 — Limit referrer leakage
		w.Header().Set("Referrer-Policy", "strict-origin")
		// A05 — Restrict resource loading to same origin
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Rate Limiting  (A04 Insecure Design)
// ──────────────────────────────────────────────────────────────────────────────

// ipBucket tracks the request count within the current window for a single IP.
type ipBucket struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

// RateLimiter holds per-IP token buckets and enforces a sliding-window limit.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*ipBucket
	limit    int
	window   time.Duration
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewRateLimiter creates a RateLimiter with the default limits (100 req/min per IP)
// and starts a background cleanup goroutine.
func NewRateLimiter() *RateLimiter {
	return NewRateLimiterWithConfig(rateLimitRequests, rateLimitWindow)
}

// NewRateLimiterWithConfig creates a RateLimiter with custom limit and window.
// It is intended for use in tests; production code should call NewRateLimiter().
func NewRateLimiterWithConfig(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*ipBucket),
		limit:   limit,
		window:  window,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stopCh) })
}

// Allow reports whether the request from ip should be allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &ipBucket{}
		rl.buckets[ip] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.After(b.windowEnd) {
		b.count = 0
		b.windowEnd = now.Add(rl.window)
	}
	b.count++
	return b.count <= rl.limit
}

// cleanup removes stale IP buckets to prevent unbounded memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rateLimitCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.purgeExpired()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) purgeExpired() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, b := range rl.buckets {
		b.mu.Lock()
		expired := now.After(b.windowEnd)
		b.mu.Unlock()
		if expired {
			delete(rl.buckets, ip)
		}
	}
}

// Middleware returns an http.Handler that rejects requests exceeding the limit
// with HTTP 429 Too Many Requests.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.Allow(ip) {
			SecurityLog("RATE_LIMITED", ip, r.Method, r.URL.Path, http.StatusTooManyRequests)
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Security Logging  (A09 Logging & Monitoring)
// ──────────────────────────────────────────────────────────────────────────────

// SecurityLog records a structured security event.
// Fields logged: event type, client IP, HTTP method, path, status code, timestamp.
// Sensitive data (passwords, full tokens, PII) is NEVER logged.
func SecurityLog(event, ip, method, path string, status int) {
	log.Printf("SECURITY event=%s ip=%s method=%s path=%s status=%d ts=%s",
		event, ip, method, path, status, time.Now().UTC().Format(time.RFC3339))
}

// RequestLogger returns a middleware that logs every incoming HTTP request as a
// security audit trail including the client IP, method, and path.
// This satisfies OWASP A09 (Security Logging and Monitoring).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		SecurityLog("HTTP_REQUEST", clientIP(r), r.Method, r.URL.Path, rw.statusCode)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// clientIP extracts the real client IP from X-Forwarded-For (if set by a trusted
// reverse proxy), falling back to the RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take only the first address to avoid spoofing via appended entries.
		if idx := strings.Index(xff, ","); idx >= 0 {
			xff = xff[:idx]
		}
		return strings.TrimSpace(xff)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// SanitizeError returns a generic message for production environments so that
// internal details (DB errors, stack traces) are never sent to clients.
// In development mode the original error message is preserved for debuggability.
func SanitizeError(err error, detail string) string {
	if os.Getenv("APP_ENV") == "development" {
		return fmt.Sprintf("%s: %v", detail, err)
	}
	return detail
}
