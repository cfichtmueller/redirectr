package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/domain/statistics"
)

// RedirectType represents the type of redirect to perform
type RedirectType int

const (
	RedirectTypePermanent     RedirectType = iota // 301 Moved Permanently
	RedirectTypeTemporary                         // 302 Found
	RedirectTypeTemporaryPost                     // 307 Temporary Redirect
	RedirectTypePermanentPost                     // 308 Permanent Redirect
)

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	Enabled           bool
	RequestsPerMinute int
	BurstSize         int
	CleanupInterval   time.Duration
}

// RateLimiterEntry represents a rate limiter entry for an IP
type RateLimiterEntry struct {
	Requests    int
	LastSeen    time.Time
	WindowStart time.Time
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	config   RateLimiterConfig
	entries  map[string]*RateLimiterEntry
	mutex    sync.RWMutex
	stopChan chan struct{}
}

// Config holds configuration for the redirect handler
type Config struct {
	DefaultRedirectType RedirectType
	MaxURLLength        int
	AllowedMethods      []string
	RateLimitEnabled    bool
	SecurityHeaders     bool
	RateLimiter         RateLimiterConfig
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultRedirectType: RedirectTypePermanent,
		MaxURLLength:        2048, // RFC 7230 recommendation
		AllowedMethods:      []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		RateLimitEnabled:    true,
		SecurityHeaders:     true,
		RateLimiter: RateLimiterConfig{
			Enabled:           true,
			RequestsPerMinute: 1000, // 1000 requests per minute per IP
			BurstSize:         100,  // Allow bursts up to 100 requests
			CleanupInterval:   5 * time.Minute,
		},
	}
}

var (
	// Domain validation regex - allows internationalized domain names
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

	// Path validation regex - allows most valid URL paths
	pathRegex = regexp.MustCompile(`^[a-zA-Z0-9\-._~:/?#[\]@!$&'()*+,;=%]*$`)
)

func Configure() http.Handler {
	return ConfigureWithConfig(DefaultConfig())
}

func ConfigureWithConfig(config *Config) http.Handler {
	// Create rate limiter if enabled
	var rateLimiter *RateLimiter
	if config.RateLimitEnabled && config.RateLimiter.Enabled {
		rateLimiter = NewRateLimiter(config.RateLimiter)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRedirectWithConfig(w, r, config, rateLimiter)
	})
}

// handleRedirectWithConfig processes incoming redirect requests with comprehensive edge case handling
func handleRedirectWithConfig(w http.ResponseWriter, r *http.Request, config *Config, rateLimiter *RateLimiter) {
	startTime := time.Now()
	clientIP := getClientIP(r)
	var statusCode int

	// Set security headers first
	if config.SecurityHeaders {
		setSecurityHeaders(w)
	}

	// Apply rate limiting if enabled
	if rateLimiter != nil {
		if !rateLimiter.Allow(clientIP) {
			statusCode = http.StatusTooManyRequests
			logErrorEvent("rate_limit_exceeded", statusCode,
				fmt.Errorf("rate limit exceeded for IP %s", clientIP), r,
				map[string]any{"ip": clientIP})
			w.Header().Set("Retry-After", "60") // Retry after 60 seconds
			writeErrorResponse(w, statusCode, "RATE_LIMIT_EXCEEDED",
				"Too Many Requests", "Rate limit exceeded. Please try again later.")
			return
		}
	}

	// Handle OPTIONS requests for CORS
	if r.Method == "OPTIONS" {
		handleOptionsRequest(w)
		return
	}

	// Validate HTTP method
	if !isAllowedMethod(r.Method, config.AllowedMethods) {
		statusCode = http.StatusMethodNotAllowed
		logErrorEvent("disallowed_http_method", statusCode,
			fmt.Errorf("disallowed HTTP method: %s", r.Method), r,
			map[string]interface{}{"method": r.Method})
		writeErrorResponse(w, statusCode, "METHOD_NOT_ALLOWED",
			"Method Not Allowed", fmt.Sprintf("HTTP method %s is not allowed", r.Method))
		return
	}

	// Extract and validate the requested domain from the Host header
	host, err := extractAndValidateHost(r)
	if err != nil {
		statusCode = http.StatusBadRequest
		logErrorEvent("invalid_host_header", statusCode, err, r,
			map[string]any{"host": r.Host})
		writeErrorResponse(w, statusCode, "INVALID_HOST",
			"Invalid Host header", err.Error())
		return
	}

	// Validate request URL length
	if len(r.URL.String()) > config.MaxURLLength {
		statusCode = http.StatusRequestURITooLong
		logErrorEvent("request_uri_too_long", statusCode,
			fmt.Errorf("request URI too long: %d characters", len(r.URL.String())), r,
			map[string]any{"length": len(r.URL.String()), "max": config.MaxURLLength})
		writeErrorResponse(
			w,
			statusCode,
			"REQUEST_URI_TOO_LONG",
			"Request URI Too Long",
			fmt.Sprintf("Request URI exceeds maximum length of %d characters", config.MaxURLLength),
		)
		return
	}

	// Validate path if present
	if r.URL.Path != "" && !pathRegex.MatchString(r.URL.Path) {
		statusCode = http.StatusBadRequest
		logErrorEvent("invalid_path_characters", statusCode,
			fmt.Errorf("invalid path characters: %s", r.URL.Path), r,
			map[string]any{"path": r.URL.Path})
		writeErrorResponse(
			w,
			statusCode,
			"INVALID_PATH",
			"Invalid path",
			"Path contains invalid characters",
		)
		return
	}

	// Create context with timeout for database operations
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Look up the redirect
	result, err := redirect.Lookup(ctx, host)
	if err != nil {
		statusCode = http.StatusInternalServerError
		logErrorEvent("redirect_lookup_failed", statusCode, err, r,
			map[string]any{"host": host})
		writeErrorResponse(w, statusCode, "INTERNAL_ERROR",
			"Internal Server Error", "Failed to lookup redirect")
		return
	}

	// If no redirect found, return 404
	if !result.Found {
		statusCode = http.StatusNotFound
		logErrorEvent("redirect_not_found", statusCode,
			fmt.Errorf("no redirect found for host: %s", host), r,
			map[string]any{"host": host})
		writeErrorResponse(
			w,
			statusCode,
			"NOT_FOUND",
			"Not Found",
			"No redirect configured for this domain",
		)
		return
	}

	// Build and validate the redirect URL
	redirectURL, err := buildAndValidateRedirectURL(r, result.TargetDomain, config.MaxURLLength, result.UTMTags)
	if err != nil {
		statusCode = http.StatusBadRequest
		logErrorEvent("invalid_redirect_url", statusCode, err, r,
			map[string]any{"host": host, "target": result.TargetDomain})
		writeErrorResponse(
			w,
			statusCode,
			"CLIENT_ERROR",
			"Invalid Request",
			"",
		)
		return
	}

	// Determine redirect type based on the redirect configuration
	redirectType := determineRedirectTypeFromString(result.RedirectType, r.Method, config.DefaultRedirectType)

	// Log the successful redirect
	duration := time.Since(startTime)
	logRedirectEvent(host, result.TargetDomain, redirectURL, r.Method, redirectType, result.FromCache, duration, r)

	// Record statistics asynchronously (non-blocking)
	if result.Found && result.RedirectID != "" && result.UserID != "" {
		// Store only the path and query parameters (not the full URL)
		// The host information is already available through the redirect configuration
		requestedPath := r.URL.Path
		if r.URL.RawQuery != "" {
			requestedPath += "?" + r.URL.RawQuery
		}

		cmd := statistics.RecordHitCommand{
			RedirectID:   result.RedirectID,
			UserID:       result.UserID,
			IP:           clientIP,
			UserAgent:    r.Header.Get("User-Agent"),
			Referer:      r.Header.Get("Referer"),
			RequestedURL: requestedPath,
			Timestamp:    time.Now(),
		}

		// Use background context for statistics recording
		statsCtx := context.Background()
		if err := statistics.QueueRedirectHitAsync(statsCtx, cmd); err != nil {
			slog.Error("failed to queue redirect hit for statistics",
				"error", err,
				"redirectId", result.RedirectID,
				"userId", result.UserID)
		}
	}

	// Perform the redirect
	performRedirect(w, redirectURL, redirectType)
}

// Helper functions for edge case handling

// setSecurityHeaders sets security-related HTTP headers
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
}

// handleOptionsRequest handles CORS preflight requests
func handleOptionsRequest(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
	w.WriteHeader(http.StatusOK)
}

// isAllowedMethod checks if the HTTP method is allowed
func isAllowedMethod(method string, allowedMethods []string) bool {
	for _, allowed := range allowedMethods {
		if method == allowed {
			return true
		}
	}
	return false
}

// extractAndValidateHost extracts and validates the host from the request
func extractAndValidateHost(r *http.Request) (string, error) {
	host := r.Host
	if host == "" {
		return "", fmt.Errorf("missing Host header")
	}

	// Remove port if present (e.g., "example.com:8081" -> "example.com")
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Validate domain format
	if !domainRegex.MatchString(host) {
		return "", fmt.Errorf("invalid domain format: %s", host)
	}

	// Check for suspicious patterns
	if strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return "", fmt.Errorf("suspicious domain pattern: %s", host)
	}

	return strings.ToLower(host), nil
}

// buildAndValidateRedirectURL constructs and validates the full redirect URL
func buildAndValidateRedirectURL(req *http.Request, targetDomain string, maxURLLength int, utmTags *redirect.UTMTags) (string, error) {
	// Validate target domain
	if !domainRegex.MatchString(targetDomain) {
		return "", fmt.Errorf("invalid target domain format: %s", targetDomain)
	}

	// Start with the target domain using HTTPS
	redirectURL := "https://" + strings.ToLower(targetDomain)

	// Enhanced path handling with edge cases
	path := req.URL.Path
	if path != "" && path != "/" {
		// Normalize path: remove double slashes, handle trailing slashes
		path = normalizePath(path)

		// Validate path doesn't contain dangerous patterns
		if err := validatePath(path); err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		redirectURL += path
	}

	// Enhanced query parameter handling with UTM tag support
	queryParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid query parameters: %w", err)
	}

	// Apply configured UTM tags (they override any existing UTM tags in the original URL)
	if utmTags != nil && utmTags.HasUTMTags() {
		if utmTags.Source != "" {
			queryParams.Set("utm_source", utmTags.Source)
		}
		if utmTags.Medium != "" {
			queryParams.Set("utm_medium", utmTags.Medium)
		}
		if utmTags.Campaign != "" {
			queryParams.Set("utm_campaign", utmTags.Campaign)
		}
		if utmTags.Term != "" {
			queryParams.Set("utm_term", utmTags.Term)
		}
		if utmTags.Content != "" {
			queryParams.Set("utm_content", utmTags.Content)
		}
	}

	// Validate and sanitize query parameters
	if len(queryParams) > 0 {
		sanitizedQuery, err := sanitizeQueryParameters(queryParams.Encode())
		if err != nil {
			return "", fmt.Errorf("invalid query parameters: %w", err)
		}

		if sanitizedQuery != "" {
			redirectURL += "?" + sanitizedQuery
		}
	}

	// Validate final URL length
	if len(redirectURL) > maxURLLength {
		return "", fmt.Errorf("redirect URL too long: %d characters", len(redirectURL))
	}

	// Parse and validate the URL
	if _, err := url.Parse(redirectURL); err != nil {
		return "", fmt.Errorf("invalid redirect URL: %w", err)
	}

	return redirectURL, nil
}

// normalizePath normalizes a URL path by removing double slashes and handling edge cases
func normalizePath(path string) string {
	// Remove double slashes (but preserve the leading slash)
	normalized := strings.ReplaceAll(path, "//", "/")

	// Handle edge case where normalization might have removed the leading slash
	if !strings.HasPrefix(normalized, "/") && normalized != "" {
		normalized = "/" + normalized
	}

	// Remove trailing slash unless it's the root path
	if normalized != "/" && strings.HasSuffix(normalized, "/") {
		normalized = strings.TrimSuffix(normalized, "/")
	}

	return normalized
}

// validatePath validates a URL path for security and correctness
func validatePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null byte detected in path: %s", path)
	}

	// Check for control characters
	for _, char := range path {
		if char < 32 || char == 127 {
			return fmt.Errorf("control character detected in path: %s", path)
		}
	}

	// Check for excessive path length (prevent DoS)
	if len(path) > 2048 {
		return fmt.Errorf("path too long: %d characters", len(path))
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"/.htaccess",
		"/.env",
		"/wp-admin",
		"/admin",
		"/phpmyadmin",
		"/.git",
		"/.svn",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return fmt.Errorf("suspicious path pattern detected: %s", path)
		}
	}

	return nil
}

// sanitizeQueryParameters sanitizes and validates query parameters
func sanitizeQueryParameters(rawQuery string) (string, error) {
	// Check query string length
	if len(rawQuery) > 1000 {
		return "", fmt.Errorf("query string too long: %d characters", len(rawQuery))
	}

	// Parse query parameters
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid query format: %w", err)
	}

	// Validate each parameter
	for key, valueSlice := range values {
		// Check parameter name
		if err := validateQueryParameterName(key); err != nil {
			return "", fmt.Errorf("invalid parameter name '%s': %w", key, err)
		}

		// Check parameter values
		for _, value := range valueSlice {
			if err := validateQueryParameterValue(value); err != nil {
				return "", fmt.Errorf("invalid parameter value for '%s': %w", key, err)
			}
		}
	}

	// Rebuild query string
	return values.Encode(), nil
}

// validateQueryParameterName validates a query parameter name
func validateQueryParameterName(name string) error {
	// Check for empty name
	if name == "" {
		return fmt.Errorf("empty parameter name")
	}

	// Check for null bytes
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("null byte in parameter name")
	}

	// Check for control characters
	for _, char := range name {
		if char < 32 || char == 127 {
			return fmt.Errorf("control character in parameter name")
		}
	}

	// Check for suspicious patterns
	suspiciousNames := []string{
		"password",
		"passwd",
		"pwd",
		"token",
		"key",
		"secret",
		"auth",
		"session",
	}

	lowerName := strings.ToLower(name)
	for _, suspicious := range suspiciousNames {
		if strings.Contains(lowerName, suspicious) {
			return fmt.Errorf("suspicious parameter name: %s", name)
		}
	}

	return nil
}

// validateQueryParameterValue validates a query parameter value
func validateQueryParameterValue(value string) error {
	// Check for null bytes
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("null byte in parameter value")
	}

	// Check for excessive length
	if len(value) > 500 {
		return fmt.Errorf("parameter value too long: %d characters", len(value))
	}

	// Check for suspicious patterns (SQL injection attempts)
	suspiciousPatterns := []string{
		"' OR '1'='1",
		"'; DROP TABLE",
		"UNION SELECT",
		"<script",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerValue, pattern) {
			return fmt.Errorf("suspicious parameter value pattern: %s", value)
		}
	}

	return nil
}

// determineRedirectTypeFromString determines the appropriate redirect type based on the configured redirect type and HTTP method
func determineRedirectTypeFromString(redirectTypeStr string, method string, defaultType RedirectType) RedirectType {
	// Parse the redirect type string
	switch redirectTypeStr {
	case "301":
		// For 301 redirects, use permanent redirects
		if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
			return RedirectTypePermanentPost // 308 Permanent Redirect
		}
		return RedirectTypePermanent // 301 Moved Permanently
	case "302":
		// For 302 redirects, use temporary redirects
		if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
			return RedirectTypeTemporaryPost // 307 Temporary Redirect
		}
		return RedirectTypeTemporary // 302 Found
	default:
		// Fallback to the original logic if redirect type is not recognized
		return determineRedirectType(method, defaultType)
	}
}

// determineRedirectType determines the appropriate redirect type based on HTTP method
func determineRedirectType(method string, defaultType RedirectType) RedirectType {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		// For methods that can have request bodies, use temporary redirects
		// to preserve the method and body
		return RedirectTypeTemporaryPost
	case "GET", "HEAD":
		// For safe methods, use the default type (usually permanent)
		return defaultType
	default:
		return RedirectTypeTemporary
	}
}

// performRedirect performs the actual HTTP redirect
func performRedirect(w http.ResponseWriter, redirectURL string, redirectType RedirectType) {
	w.Header().Set("Location", redirectURL)

	switch redirectType {
	case RedirectTypePermanent:
		w.WriteHeader(http.StatusMovedPermanently)
	case RedirectTypeTemporary:
		w.WriteHeader(http.StatusFound)
	case RedirectTypeTemporaryPost:
		w.WriteHeader(http.StatusTemporaryRedirect)
	case RedirectTypePermanentPost:
		w.WriteHeader(http.StatusPermanentRedirect)
	default:
		w.WriteHeader(http.StatusMovedPermanently)
	}

	// Add cache control headers based on redirect type
	if redirectType == RedirectTypePermanent || redirectType == RedirectTypePermanentPost {
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	} else {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}

	w.Write([]byte(""))
}

// getClientIP extracts the client IP address from the request
func getClientIP(req *http.Request) string {
	// Check X-Forwarded-For header first (for load balancers/proxies)
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if commaIndex := strings.Index(xff, ","); commaIndex != -1 {
			return strings.TrimSpace(xff[:commaIndex])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if req.RemoteAddr != "" {
		// Remove port if present
		if colonIndex := strings.LastIndex(req.RemoteAddr, ":"); colonIndex != -1 {
			return req.RemoteAddr[:colonIndex]
		}
		return req.RemoteAddr
	}

	return "unknown"
}

// Rate Limiter Implementation

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:   config,
		entries:  make(map[string]*RateLimiterEntry),
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	if !rl.config.Enabled {
		return true
	}

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]

	if !exists {
		// First request from this IP
		rl.entries[ip] = &RateLimiterEntry{
			Requests:    1,
			LastSeen:    now,
			WindowStart: now,
		}
		return true
	}

	// Check if we're in a new time window
	if now.Sub(entry.WindowStart) >= time.Minute {
		// Reset the window
		entry.Requests = 1
		entry.WindowStart = now
		entry.LastSeen = now
		return true
	}

	// Check if we've exceeded the rate limit
	if entry.Requests >= rl.config.RequestsPerMinute {
		return false
	}

	// Allow the request
	entry.Requests++
	entry.LastSeen = now
	return true
}

// cleanup removes old entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mutex.Lock()
			now := time.Now()
			for ip, entry := range rl.entries {
				// Remove entries that haven't been seen for more than 2 cleanup intervals
				if now.Sub(entry.LastSeen) > 2*rl.config.CleanupInterval {
					delete(rl.entries, ip)
				}
			}
			rl.mutex.Unlock()

		case <-rl.stopChan:
			return
		}
	}
}

// Error Response and Logging Enhancement

// LogLevel represents different log levels for different types of events
type LogLevel int

const (
	LogLevelInfo LogLevel = iota
	LogLevelWarn
	LogLevelError
)

// LogEvent represents a structured log event
type LogEvent struct {
	Level      LogLevel
	Message    string
	Fields     map[string]interface{}
	StatusCode int
}

// Helper functions for error handling and logging

// logEvent logs a structured event
func logEvent(event LogEvent) {
	fields := make([]interface{}, 0, len(event.Fields)*2+2)
	fields = append(fields, "message", event.Message)
	fields = append(fields, "status_code", event.StatusCode)

	for k, v := range event.Fields {
		fields = append(fields, k, v)
	}

	switch event.Level {
	case LogLevelInfo:
		slog.Info(event.Message, fields...)
	case LogLevelWarn:
		slog.Warn(event.Message, fields...)
	case LogLevelError:
		slog.Error(event.Message, fields...)
	}
}

// logRedirectEvent logs a redirect event with comprehensive details
func logRedirectEvent(source, target, redirectURL, method string, redirectType RedirectType, fromCache bool, duration time.Duration, req *http.Request) {
	fields := map[string]interface{}{
		"source":        source,
		"target":        target,
		"redirect_url":  redirectURL,
		"method":        method,
		"redirect_type": redirectType,
		"from_cache":    fromCache,
		"duration_ms":   duration.Milliseconds(),
		"user_agent":    req.Header.Get("User-Agent"),
		"referer":       req.Header.Get("Referer"),
		"ip":            getClientIP(req),
		"status_code":   200, // Successful redirect
	}

	logEvent(LogEvent{
		Level:      LogLevelInfo,
		Message:    "redirect_performed",
		Fields:     fields,
		StatusCode: 200,
	})
}

// logErrorEvent logs an error event with comprehensive details
func logErrorEvent(message string, statusCode int, err error, req *http.Request, additionalFields map[string]interface{}) {
	fields := map[string]interface{}{
		"error":       err.Error(),
		"user_agent":  req.Header.Get("User-Agent"),
		"referer":     req.Header.Get("Referer"),
		"ip":          getClientIP(req),
		"status_code": statusCode,
	}

	// Add additional fields if provided
	for k, v := range additionalFields {
		fields[k] = v
	}

	var level LogLevel
	switch {
	case statusCode >= 500:
		level = LogLevelError
	case statusCode >= 400:
		level = LogLevelWarn
	default:
		level = LogLevelInfo
	}

	logEvent(LogEvent{
		Level:      level,
		Message:    message,
		Fields:     fields,
		StatusCode: statusCode,
	})
}
