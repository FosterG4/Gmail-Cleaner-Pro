package middleware

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"mailcleanerpro/pkg/logger"
)

// RecoveryWithLogger creates a recovery middleware with structured logging
func RecoveryWithLogger() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(gin.DefaultErrorWriter, recoveryHandler)
}

// AdvancedRecoveryWithLogger creates an advanced recovery middleware with detailed logging
func AdvancedRecoveryWithLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				handlePanic(c, err)
			}
		}()
		c.Next()
	}
}

// recoveryHandler handles panic recovery with basic logging
func recoveryHandler(c *gin.Context, recovered interface{}) {
	if err, ok := recovered.(string); ok {
		logger.L().Error("Panic recovered",
			zap.String("error", err),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("client_ip", c.ClientIP()),
		)
	}
	c.AbortWithStatus(http.StatusInternalServerError)
}

// handlePanic handles panic with comprehensive logging and error response
func handlePanic(c *gin.Context, recovered interface{}) {
	// Get request ID from context
	requestID := GetRequestID(c)

	// Create logger with request context
	reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

	// Check for a broken connection, as it is not really a
	// condition that warrants a panic stack trace.
	var brokenPipe bool
	if ne, ok := recovered.(*net.OpError); ok {
		if se, ok := ne.Err.(*os.SyscallError); ok {
			if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
				strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
				brokenPipe = true
			}
		}
	}

	// Capture request details for logging
	httpRequest, _ := httputil.DumpRequest(c.Request, false)
	headers := strings.Split(string(httpRequest), "\r\n")
	for idx, header := range headers {
		current := strings.Split(header, ":")
		if current[0] == "Authorization" {
			headers[idx] = current[0] + ": [MASKED]"
		}
	}
	headersToLog := strings.Join(headers, "\r\n")

	// Log the panic with full context
	if brokenPipe {
		reqLogger.Error("Broken pipe detected",
			zap.Any("error", recovered),
			zap.String("request", headersToLog),
		)
	} else {
		reqLogger.Error("Panic recovered",
			zap.Any("error", recovered),
			zap.String("request", headersToLog),
			zap.String("stack_trace", string(debug.Stack())),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Time("timestamp", time.Now()),
		)
	}

	// If the connection is dead, we can't write a status to it.
	if brokenPipe {
		c.Error(fmt.Errorf("%v", recovered))
		c.Abort()
		return
	}

	// Return appropriate error response
	errorResponse := gin.H{
		"error":      "Internal server error",
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	// In development mode, include more details
	if gin.Mode() == gin.DebugMode {
		errorResponse["details"] = fmt.Sprintf("%v", recovered)
		errorResponse["stack_trace"] = string(debug.Stack())
	}

	c.JSON(http.StatusInternalServerError, errorResponse)
	c.Abort()
}

// ErrorHandlingMiddleware creates middleware for handling and logging errors
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			handleErrors(c)
		}
	}
}

// handleErrors processes and logs errors from the context
func handleErrors(c *gin.Context) {
	requestID := GetRequestID(c)
	reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

	// Log all errors
	for _, ginErr := range c.Errors {
		switch ginErr.Type {
		case gin.ErrorTypeBind:
			reqLogger.Warn("Request binding error",
				zap.Error(ginErr.Err),
				zap.String("error_type", "binding"),
				zap.String("client_ip", c.ClientIP()),
			)
		case gin.ErrorTypePublic:
			reqLogger.Info("Public error",
				zap.Error(ginErr.Err),
				zap.String("error_type", "public"),
			)
		case gin.ErrorTypePrivate:
			reqLogger.Error("Private error",
				zap.Error(ginErr.Err),
				zap.String("error_type", "private"),
				zap.String("client_ip", c.ClientIP()),
			)
		default:
			reqLogger.Error("Unknown error",
				zap.Error(ginErr.Err),
				zap.String("error_type", "unknown"),
				zap.String("client_ip", c.ClientIP()),
			)
		}
	}

	// If response hasn't been written yet, send error response
	if !c.Writer.Written() {
		lastError := c.Errors.Last()
		if lastError != nil {
			statusCode := http.StatusInternalServerError
			errorMessage := "Internal server error"

			// Determine appropriate status code based on error type
			switch lastError.Type {
			case gin.ErrorTypeBind:
				statusCode = http.StatusBadRequest
				errorMessage = "Invalid request data"
			case gin.ErrorTypePublic:
				statusCode = http.StatusBadRequest
				errorMessage = lastError.Error()
			}

			errorResponse := gin.H{
				"error":      errorMessage,
				"request_id": requestID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			}

			// In development mode, include more details
			if gin.Mode() == gin.DebugMode {
				errorResponse["details"] = lastError.Error()
				errorResponse["all_errors"] = c.Errors.Errors()
			}

			c.JSON(statusCode, errorResponse)
		}
	}
}

// HealthCheckMiddleware creates middleware for health check logging
func HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip detailed logging for health check endpoints
		if isHealthCheckPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		c.Next()
	}
}

// isHealthCheckPath checks if the path is a health check endpoint
func isHealthCheckPath(path string) bool {
	healthPaths := []string{
		"/health",
		"/healthz",
		"/ping",
		"/status",
		"/metrics",
		"/ready",
		"/live",
	}

	for _, healthPath := range healthPaths {
		if path == healthPath {
			return true
		}
	}
	return false
}

// SecurityHeadersMiddleware adds security headers and logs security events
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Updated CSP to allow CSS and necessary resources
		csp := "default-src 'self'; " +
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; " +
			"font-src 'self' https://fonts.gstatic.com https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; " +
			"script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com https://cdn.jsdelivr.net; " +
			"img-src 'self' data: https:; " +
			"connect-src 'self' https://accounts.google.com https://oauth2.googleapis.com; " +
			"frame-src 'none'; " +
			"object-src 'none'; " +
			"base-uri 'self'"
		c.Header("Content-Security-Policy", csp)

		// Log potential security issues
		logSecurityEvents(c)

		c.Next()
	}
}

// logSecurityEvents logs potential security-related events
func logSecurityEvents(c *gin.Context) {
	requestID := GetRequestID(c)
	reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

	// Check for suspicious patterns
	userAgent := c.Request.UserAgent()
	if userAgent == "" {
		reqLogger.Warn("Request with empty User-Agent",
			zap.String("client_ip", c.ClientIP()),
			zap.String("security_event", "empty_user_agent"),
		)
	}

	// Check for potential SQL injection patterns in query parameters
	for key, values := range c.Request.URL.Query() {
		for _, value := range values {
			if containsSQLInjectionPattern(value) {
				reqLogger.Warn("Potential SQL injection attempt detected",
					zap.String("client_ip", c.ClientIP()),
					zap.String("parameter", key),
					zap.String("value", value),
					zap.String("security_event", "sql_injection_attempt"),
				)
			}
		}
	}

	// Check for potential XSS patterns
	for key, values := range c.Request.URL.Query() {
		for _, value := range values {
			if containsXSSPattern(value) {
				reqLogger.Warn("Potential XSS attempt detected",
					zap.String("client_ip", c.ClientIP()),
					zap.String("parameter", key),
					zap.String("value", value),
					zap.String("security_event", "xss_attempt"),
				)
			}
		}
	}
}

// containsSQLInjectionPattern checks for common SQL injection patterns
func containsSQLInjectionPattern(value string) bool {
	patterns := []string{
		"' OR '1'='1",
		"' OR 1=1",
		"'; DROP TABLE",
		"'; DELETE FROM",
		"UNION SELECT",
		"<script",
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.Contains(valueLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// containsXSSPattern checks for common XSS patterns
func containsXSSPattern(value string) bool {
	patterns := []string{
		"<script",
		"javascript:",
		"onload=",
		"onerror=",
		"onclick=",
		"<iframe",
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.Contains(valueLower, pattern) {
			return true
		}
	}
	return false
}
