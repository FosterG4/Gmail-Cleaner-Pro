package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"mailcleanerpro/pkg/logger"
)

// MetricsCollector holds application metrics
type MetricsCollector struct {
	mu                  sync.RWMutex
	RequestCount        map[string]int64            `json:"request_count"`
	RequestDuration     map[string][]float64        `json:"request_duration"`
	StatusCodeCount     map[int]int64               `json:"status_code_count"`
	EndpointMetrics     map[string]*EndpointMetrics `json:"endpoint_metrics"`
	ActiveConnections   int64                       `json:"active_connections"`
	TotalRequests       int64                       `json:"total_requests"`
	TotalErrors         int64                       `json:"total_errors"`
	AverageResponseTime float64                     `json:"average_response_time"`
	StartTime           time.Time                   `json:"start_time"`
}

// EndpointMetrics holds metrics for specific endpoints
type EndpointMetrics struct {
	Count           int64     `json:"count"`
	TotalDuration   float64   `json:"total_duration"`
	AverageDuration float64   `json:"average_duration"`
	MinDuration     float64   `json:"min_duration"`
	MaxDuration     float64   `json:"max_duration"`
	ErrorCount      int64     `json:"error_count"`
	LastAccessed    time.Time `json:"last_accessed"`
}

// Global metrics collector instance
var globalMetrics = &MetricsCollector{
	RequestCount:    make(map[string]int64),
	RequestDuration: make(map[string][]float64),
	StatusCodeCount: make(map[int]int64),
	EndpointMetrics: make(map[string]*EndpointMetrics),
	StartTime:       time.Now(),
}

// MetricsMiddleware creates middleware for collecting and logging metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics collection for health check endpoints
		if isHealthCheckPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		startTime := time.Now()
		requestID := GetRequestID(c)

		// Increment active connections
		globalMetrics.incrementActiveConnections()
		defer globalMetrics.decrementActiveConnections()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)
		durationMs := float64(duration.Nanoseconds()) / 1e6

		// Collect metrics
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		statusCode := c.Writer.Status()
		endpoint := method + " " + path

		// Update metrics
		globalMetrics.recordRequest(endpoint, durationMs, statusCode)

		// Log metrics
		logMetrics(requestID, method, path, statusCode, durationMs)

		// Log slow requests
		if duration > 5*time.Second {
			logSlowRequest(requestID, method, path, durationMs)
		}
	}
}

// PerformanceMetricsMiddleware creates middleware for detailed performance logging
func PerformanceMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health check endpoints
		if isHealthCheckPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		startTime := time.Now()
		requestID := GetRequestID(c)
		reqLogger := logger.RequestLogger(requestID, c.Request.Method, c.Request.URL.Path)

		// Log request start with performance context
		reqLogger.Debug("Performance tracking started",
			zap.Time("start_time", startTime),
			zap.String("endpoint", c.Request.Method+" "+c.Request.URL.Path),
		)

		// Process request
		c.Next()

		// Calculate metrics
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		durationMs := float64(duration.Nanoseconds()) / 1e6
		statusCode := c.Writer.Status()
		responseSize := c.Writer.Size()

		// Performance analysis
		performanceData := analyzePerformance(duration, statusCode, responseSize)

		// Log detailed performance metrics
		reqLogger.Info("Performance metrics",
			zap.Float64("duration_ms", durationMs),
			zap.Duration("duration", duration),
			zap.Int("status_code", statusCode),
			zap.Int("response_size_bytes", responseSize),
			zap.String("performance_category", performanceData.Category),
			zap.Bool("is_slow", performanceData.IsSlow),
			zap.Bool("is_error", performanceData.IsError),
			zap.Float64("requests_per_second", performanceData.RequestsPerSecond),
			zap.Float64("throughput_mb_per_sec", performanceData.ThroughputMBPerSec),
			zap.Time("end_time", endTime),
		)

		// Log warnings for performance issues
		if performanceData.IsSlow {
			reqLogger.Warn("Slow request detected",
				zap.Float64("duration_ms", durationMs),
				zap.String("threshold_exceeded", "5000ms"),
				zap.String("optimization_needed", "true"),
			)
		}

		if responseSize > 10*1024*1024 { // 10MB
			reqLogger.Warn("Large response detected",
				zap.Int("response_size_bytes", responseSize),
				zap.Float64("response_size_mb", float64(responseSize)/(1024*1024)),
				zap.String("consider_pagination", "true"),
			)
		}
	}
}

// PerformanceData holds performance analysis results
type PerformanceData struct {
	Category           string
	IsSlow             bool
	IsError            bool
	RequestsPerSecond  float64
	ThroughputMBPerSec float64
}

// analyzePerformance analyzes request performance
func analyzePerformance(duration time.Duration, statusCode, responseSize int) PerformanceData {
	durationSeconds := duration.Seconds()
	requestsPerSecond := 1.0 / durationSeconds
	throughputMBPerSec := (float64(responseSize) / (1024 * 1024)) / durationSeconds

	return PerformanceData{
		Category:           getPerformanceCategory(duration),
		IsSlow:             duration > 5*time.Second,
		IsError:            statusCode >= 400,
		RequestsPerSecond:  requestsPerSecond,
		ThroughputMBPerSec: throughputMBPerSec,
	}
}

// recordRequest records metrics for a request
func (m *MetricsCollector) recordRequest(endpoint string, durationMs float64, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update total requests
	m.TotalRequests++

	// Update error count
	if statusCode >= 400 {
		m.TotalErrors++
	}

	// Update request count by endpoint
	m.RequestCount[endpoint]++

	// Update request duration
	if m.RequestDuration[endpoint] == nil {
		m.RequestDuration[endpoint] = make([]float64, 0)
	}
	m.RequestDuration[endpoint] = append(m.RequestDuration[endpoint], durationMs)

	// Keep only last 1000 durations to prevent memory leak
	if len(m.RequestDuration[endpoint]) > 1000 {
		m.RequestDuration[endpoint] = m.RequestDuration[endpoint][len(m.RequestDuration[endpoint])-1000:]
	}

	// Update status code count
	m.StatusCodeCount[statusCode]++

	// Update endpoint metrics
	if m.EndpointMetrics[endpoint] == nil {
		m.EndpointMetrics[endpoint] = &EndpointMetrics{
			MinDuration: durationMs,
			MaxDuration: durationMs,
		}
	}

	metric := m.EndpointMetrics[endpoint]
	metric.Count++
	metric.TotalDuration += durationMs
	metric.AverageDuration = metric.TotalDuration / float64(metric.Count)
	metric.LastAccessed = time.Now()

	if durationMs < metric.MinDuration {
		metric.MinDuration = durationMs
	}
	if durationMs > metric.MaxDuration {
		metric.MaxDuration = durationMs
	}
	if statusCode >= 400 {
		metric.ErrorCount++
	}

	// Update average response time
	totalDuration := 0.0
	for _, durations := range m.RequestDuration {
		for _, duration := range durations {
			totalDuration += duration
		}
	}
	m.AverageResponseTime = totalDuration / float64(m.TotalRequests)
}

// incrementActiveConnections increments active connection count
func (m *MetricsCollector) incrementActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections++
}

// decrementActiveConnections decrements active connection count
func (m *MetricsCollector) decrementActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections--
}

// GetMetrics returns current metrics
func GetMetrics() *MetricsCollector {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metricsCopy := &MetricsCollector{
		RequestCount:        make(map[string]int64),
		RequestDuration:     make(map[string][]float64),
		StatusCodeCount:     make(map[int]int64),
		EndpointMetrics:     make(map[string]*EndpointMetrics),
		ActiveConnections:   globalMetrics.ActiveConnections,
		TotalRequests:       globalMetrics.TotalRequests,
		TotalErrors:         globalMetrics.TotalErrors,
		AverageResponseTime: globalMetrics.AverageResponseTime,
		StartTime:           globalMetrics.StartTime,
	}

	// Copy maps
	for k, v := range globalMetrics.RequestCount {
		metricsCopy.RequestCount[k] = v
	}
	for k, v := range globalMetrics.StatusCodeCount {
		metricsCopy.StatusCodeCount[k] = v
	}
	for k, v := range globalMetrics.EndpointMetrics {
		metricsCopy.EndpointMetrics[k] = &EndpointMetrics{
			Count:           v.Count,
			TotalDuration:   v.TotalDuration,
			AverageDuration: v.AverageDuration,
			MinDuration:     v.MinDuration,
			MaxDuration:     v.MaxDuration,
			ErrorCount:      v.ErrorCount,
			LastAccessed:    v.LastAccessed,
		}
	}

	return metricsCopy
}

// logMetrics logs basic request metrics
func logMetrics(requestID, method, path string, statusCode int, durationMs float64) {
	reqLogger := logger.RequestLogger(requestID, method, path)

	reqLogger.Info("Request metrics",
		zap.Int("status_code", statusCode),
		zap.Float64("duration_ms", durationMs),
		zap.String("endpoint", method+" "+path),
		zap.String("status_category", getStatusCategory(statusCode)),
		zap.String("performance_tier", getPerformanceTier(durationMs)),
	)
}

// logSlowRequest logs slow request warnings
func logSlowRequest(requestID, method, path string, durationMs float64) {
	reqLogger := logger.RequestLogger(requestID, method, path)

	reqLogger.Warn("Slow request detected",
		zap.Float64("duration_ms", durationMs),
		zap.String("endpoint", method+" "+path),
		zap.String("performance_impact", "high"),
		zap.String("action_required", "optimization_needed"),
		zap.Float64("threshold_ms", 5000),
	)
}

// getStatusCategory categorizes HTTP status codes
func getStatusCategory(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "success"
	case statusCode >= 300 && statusCode < 400:
		return "redirect"
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// getPerformanceTier categorizes request performance
func getPerformanceTier(durationMs float64) string {
	switch {
	case durationMs < 50:
		return "excellent"
	case durationMs < 200:
		return "good"
	case durationMs < 1000:
		return "acceptable"
	case durationMs < 5000:
		return "slow"
	default:
		return "very_slow"
	}
}

// MetricsReportingMiddleware periodically logs system metrics
func MetricsReportingMiddleware(interval time.Duration) gin.HandlerFunc {
	// Start background metrics reporting
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			logSystemMetrics()
		}
	}()

	return func(c *gin.Context) {
		c.Next()
	}
}

// logSystemMetrics logs periodic system metrics
func logSystemMetrics() {
	metrics := GetMetrics()
	uptime := time.Since(metrics.StartTime)

	// Calculate error rate
	errorRate := 0.0
	if metrics.TotalRequests > 0 {
		errorRate = (float64(metrics.TotalErrors) / float64(metrics.TotalRequests)) * 100
	}

	// Calculate requests per minute
	requestsPerMinute := 0.0
	if uptime.Minutes() > 0 {
		requestsPerMinute = float64(metrics.TotalRequests) / uptime.Minutes()
	}

	logger.L().Info("System metrics report",
		zap.Int64("total_requests", metrics.TotalRequests),
		zap.Int64("total_errors", metrics.TotalErrors),
		zap.Float64("error_rate_percent", errorRate),
		zap.Int64("active_connections", metrics.ActiveConnections),
		zap.Float64("average_response_time_ms", metrics.AverageResponseTime),
		zap.Float64("requests_per_minute", requestsPerMinute),
		zap.Duration("uptime", uptime),
		zap.Time("report_time", time.Now()),
		zap.String("component", "metrics_reporter"),
	)

	// Log top endpoints by request count
	logTopEndpoints(metrics)
}

// logTopEndpoints logs the most frequently accessed endpoints
func logTopEndpoints(metrics *MetricsCollector) {
	type endpointCount struct {
		Endpoint    string
		Count       int64
		AvgDuration float64
		ErrorCount  int64
	}

	var endpoints []endpointCount
	for endpoint, count := range metrics.RequestCount {
		avgDuration := 0.0
		errorCount := int64(0)
		if endpointMetric, exists := metrics.EndpointMetrics[endpoint]; exists {
			avgDuration = endpointMetric.AverageDuration
			errorCount = endpointMetric.ErrorCount
		}
		endpoints = append(endpoints, endpointCount{
			Endpoint:    endpoint,
			Count:       count,
			AvgDuration: avgDuration,
			ErrorCount:  errorCount,
		})
	}

	// Sort by request count (simple bubble sort for small datasets)
	for i := 0; i < len(endpoints)-1; i++ {
		for j := 0; j < len(endpoints)-i-1; j++ {
			if endpoints[j].Count < endpoints[j+1].Count {
				endpoints[j], endpoints[j+1] = endpoints[j+1], endpoints[j]
			}
		}
	}

	// Log top 5 endpoints
	maxEndpoints := 5
	if len(endpoints) < maxEndpoints {
		maxEndpoints = len(endpoints)
	}

	for i := 0; i < maxEndpoints; i++ {
		ep := endpoints[i]
		logger.L().Info("Top endpoint metrics",
			zap.String("endpoint", ep.Endpoint),
			zap.Int64("request_count", ep.Count),
			zap.Float64("avg_duration_ms", ep.AvgDuration),
			zap.Int64("error_count", ep.ErrorCount),
			zap.Int("rank", i+1),
			zap.String("component", "top_endpoints"),
		)
	}
}
