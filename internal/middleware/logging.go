package middleware

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"mailcleanerpro/pkg/logger"
)

// RequestResponseLogger creates a middleware for logging HTTP requests and responses
func RequestResponseLogger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: customLogFormatter,
		Output:    gin.DefaultWriter,
		SkipPaths: []string{"/health", "/metrics"},
	})
}

// AdvancedRequestResponseLogger creates an advanced middleware for detailed request/response logging
func AdvancedRequestResponseLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID
		requestID := generateRequestID()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Start time
		startTime := time.Now()

		// Create request logger
		reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

		// Log request details
		logRequest(reqLogger, c)

		// Capture response
		responseWriter := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Log response details
		logResponse(reqLogger, c, responseWriter, duration)
	}
}

// DetailedRequestResponseLogger creates the most comprehensive logging middleware
func DetailedRequestResponseLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate request ID
		requestID := generateRequestID()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Start time
		startTime := time.Now()

		// Create request logger
		reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

		// Capture request body if needed
		var requestBody []byte
		if shouldLogRequestBody(c) {
			requestBody = captureRequestBody(c)
		}

		// Log detailed request
		logDetailedRequest(reqLogger, c, requestBody)

		// Capture response
		responseWriter := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Log detailed response
		logDetailedResponse(reqLogger, c, responseWriter, duration)
	}
}

// responseWriter wraps gin.ResponseWriter to capture response body
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// RequestIDMiddleware generates a unique request ID for each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID already exists in header
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate new request ID
			requestID = generateRequestID()
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// customLogFormatter provides custom log formatting for basic logging
func customLogFormatter(param gin.LogFormatterParams) string {
	return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
		param.ClientIP,
		param.TimeStamp.Format(time.RFC3339),
		param.Method,
		param.Path,
		param.Request.Proto,
		param.StatusCode,
		param.Latency,
		param.Request.UserAgent(),
		param.ErrorMessage,
	)
}

// logRequest logs basic request information
func logRequest(logger *zap.Logger, c *gin.Context) {
	logger.Info("HTTP Request Started",
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
		zap.String("referer", c.Request.Referer()),
		zap.String("proto", c.Request.Proto),
		zap.String("host", c.Request.Host),
		zap.String("remote_addr", c.Request.RemoteAddr),
		zap.Int64("content_length", c.Request.ContentLength),
		zap.String("content_type", c.Request.Header.Get("Content-Type")),
		zap.Any("query_params", c.Request.URL.Query()),
	)
}

// logResponse logs basic response information
func logResponse(logger *zap.Logger, c *gin.Context, rw *responseWriter, duration time.Duration) {
	statusCode := c.Writer.Status()
	responseSize := c.Writer.Size()

	// Determine log level based on status code
	logLevel := getLogLevelForStatus(statusCode)

	fields := []zap.Field{
		zap.Int("status_code", statusCode),
		zap.Int("response_size", responseSize),
		zap.Duration("duration", duration),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	}

	// Add error information if present
	if len(c.Errors) > 0 {
		fields = append(fields, zap.Any("errors", c.Errors.Errors()))
	}

	switch logLevel {
	case "error":
		logger.Error("HTTP Request Completed", fields...)
	case "warn":
		logger.Warn("HTTP Request Completed", fields...)
	default:
		logger.Info("HTTP Request Completed", fields...)
	}
}

// logDetailedRequest logs comprehensive request information
func logDetailedRequest(logger *zap.Logger, c *gin.Context, requestBody []byte) {
	headers := make(map[string]string)
	// Only log safe headers; mask all others
	safeHeaders := map[string]bool{
		"User-Agent":  true,
		"Referer":     true,
		"Accept":      true,
		"Accept-Language": true,
		"Accept-Encoding": true,
		"Content-Type": true,
	}
	for name, values := range c.Request.Header {
		if safeHeaders[name] {
			headers[name] = strings.Join(values, ", ")
		} else {
			headers[name] = "[MASKED]"
		}
	}

	fields := []zap.Field{
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
		zap.String("referer", c.Request.Referer()),
		zap.String("proto", c.Request.Proto),
		zap.String("host", c.Request.Host),
		zap.String("remote_addr", c.Request.RemoteAddr),
		zap.Int64("content_length", c.Request.ContentLength),
		zap.String("content_type", c.Request.Header.Get("Content-Type")),
		zap.Any("query_params", c.Request.URL.Query()),
		zap.Any("headers", headers),
	}

	// Add request body if captured and not too large
	if len(requestBody) > 0 && len(requestBody) < 10240 { // 10KB limit
		if isJSONContent(c.Request.Header.Get("Content-Type")) {
			fields = append(fields, zap.String("request_body", string(requestBody)))
		} else {
			fields = append(fields, zap.String("request_body_size", strconv.Itoa(len(requestBody))))
		}
	}

	logger.Info("HTTP Request Started (Detailed)", fields...)
}

// logDetailedResponse logs comprehensive response information
func logDetailedResponse(logger *zap.Logger, c *gin.Context, rw *responseWriter, duration time.Duration) {
	statusCode := c.Writer.Status()
	responseSize := c.Writer.Size()
	responseBody := rw.body.String()

	// Determine log level based on status code
	logLevel := getLogLevelForStatus(statusCode)

	// Capture response headers
	responseHeaders := make(map[string]string)
	for name, values := range c.Writer.Header() {
		responseHeaders[name] = strings.Join(values, ", ")
	}

	fields := []zap.Field{
		zap.Int("status_code", statusCode),
		zap.Int("response_size", responseSize),
		zap.Duration("duration", duration),
		zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		zap.Any("response_headers", responseHeaders),
	}

	// Add response body if not too large and is JSON
	if len(responseBody) > 0 && len(responseBody) < 10240 { // 10KB limit
		contentType := c.Writer.Header().Get("Content-Type")
		if isJSONContent(contentType) {
			fields = append(fields, zap.String("response_body", responseBody))
		} else {
			fields = append(fields, zap.String("response_body_size", strconv.Itoa(len(responseBody))))
		}
	}

	// Add error information if present
	if len(c.Errors) > 0 {
		fields = append(fields, zap.Any("errors", c.Errors.Errors()))
	}

	// Add performance metrics
	fields = append(fields,
		zap.String("performance_category", getPerformanceCategory(duration)),
		zap.Bool("slow_request", duration > 5*time.Second),
	)

	switch logLevel {
	case "error":
		logger.Error("HTTP Request Completed (Detailed)", fields...)
	case "warn":
		logger.Warn("HTTP Request Completed (Detailed)", fields...)
	default:
		logger.Info("HTTP Request Completed (Detailed)", fields...)
	}
}

// shouldLogRequestBody determines if request body should be captured
func shouldLogRequestBody(c *gin.Context) bool {
	// Only log request body for specific content types and reasonable sizes
	contentType := c.Request.Header.Get("Content-Type")
	contentLength := c.Request.ContentLength

	// Skip if too large (> 1MB)
	if contentLength > 1024*1024 {
		return false
	}

	// Only log for JSON, XML, and form data
	return isJSONContent(contentType) ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/x-www-form-urlencoded")
}

// captureRequestBody captures and restores request body
func captureRequestBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}

	// Restore the body for further processing
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return body
}

// isSensitiveHeader checks if a header contains sensitive information
func isSensitiveHeader(headerName string) bool {
	sensitiveHeaders := []string{
		"authorization",
		"cookie",
		"x-api-key",
		"x-auth-token",
		"x-access-token",
		"x-csrf-token",
	}

	headerLower := strings.ToLower(headerName)
	for _, sensitive := range sensitiveHeaders {
		if headerLower == sensitive {
			return true
		}
	}
	return false
}

// isJSONContent checks if content type is JSON
func isJSONContent(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}

// getLogLevelForStatus determines appropriate log level based on HTTP status code
func getLogLevelForStatus(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "error"
	case statusCode >= 400:
		return "warn"
	default:
		return "info"
	}
}

// getPerformanceCategory categorizes request performance
func getPerformanceCategory(duration time.Duration) string {
	switch {
	case duration < 100*time.Millisecond:
		return "fast"
	case duration < 500*time.Millisecond:
		return "normal"
	case duration < 2*time.Second:
		return "slow"
	default:
		return "very_slow"
	}
}

// GetRequestID retrieves request ID from context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return generateRequestID()
}
