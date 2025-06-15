package service

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// PerformanceMonitor provides centralized performance monitoring and metrics collection
type PerformanceMonitor struct {
	logger logging.Logger
	config PerformanceConfig

	// Service dependencies
	auditService     *AuditService
	retentionService *RetentionService
	rotationService  *LogRotationService

	// Performance tracking
	metrics   *SystemMetrics
	metricsMu sync.RWMutex

	// Thresholds and alerting
	thresholds map[string]PerformanceThreshold
	alerts     []PerformanceAlert

	// Service state
	running   bool
	runningMu sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// Performance analysis
	trendData    []MetricSnapshot
	trendDataMu  sync.RWMutex
	maxTrendData int
}

// PerformanceConfig holds configuration for performance monitoring
type PerformanceConfig struct {
	// Collection settings
	CollectionInterval time.Duration `json:"collection_interval"`
	TrendDataRetention time.Duration `json:"trend_data_retention"`
	MaxTrendDataPoints int           `json:"max_trend_data_points"`

	// Alerting settings
	EnableAlerting      bool          `json:"enable_alerting"`
	AlertCheckInterval  time.Duration `json:"alert_check_interval"`
	AlertCooldownPeriod time.Duration `json:"alert_cooldown_period"`

	// Performance limits
	MaxMemoryUsageMB    int64   `json:"max_memory_usage_mb"`
	MaxCPUUsagePercent  float64 `json:"max_cpu_usage_percent"`
	MaxResponseTimeMs   int64   `json:"max_response_time_ms"`
	MaxDiskUsagePercent float64 `json:"max_disk_usage_percent"`

	// Analysis settings
	EnableTrendAnalysis bool    `json:"enable_trend_analysis"`
	TrendAnalysisWindow int     `json:"trend_analysis_window"`
	RegressionThreshold float64 `json:"regression_threshold"`
}

// DefaultPerformanceConfig returns performance monitoring configuration with sensible defaults
func DefaultPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		CollectionInterval:  30 * time.Second,
		TrendDataRetention:  24 * time.Hour,
		MaxTrendDataPoints:  2880, // 24 hours at 30-second intervals
		EnableAlerting:      true,
		AlertCheckInterval:  1 * time.Minute,
		AlertCooldownPeriod: 5 * time.Minute,
		MaxMemoryUsageMB:    512,
		MaxCPUUsagePercent:  80.0,
		MaxResponseTimeMs:   1000,
		MaxDiskUsagePercent: 85.0,
		EnableTrendAnalysis: true,
		TrendAnalysisWindow: 60,  // 30 minutes at 30-second intervals
		RegressionThreshold: 0.2, // 20% performance degradation threshold
	}
}

// SystemMetrics holds comprehensive system performance metrics
type SystemMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// System resource metrics
	CPUUsage    float64 `json:"cpu_usage_percent"`
	MemoryUsage int64   `json:"memory_usage_bytes"`
	DiskUsage   float64 `json:"disk_usage_percent"`
	DiskFree    int64   `json:"disk_free_bytes"`

	// Service-specific metrics
	AuditMetrics       *AuditPerformanceMetrics       `json:"audit_metrics"`
	EnforcementMetrics *EnforcementPerformanceMetrics `json:"enforcement_metrics"`
	RetentionMetrics   *RetentionPerformanceMetrics   `json:"retention_metrics"`
	RotationMetrics    *RotationPerformanceMetrics    `json:"rotation_metrics"`
	SessionMetrics     *SessionPerformanceMetrics     `json:"session_metrics"`

	// Performance indicators
	ResponseTimes       map[string]time.Duration `json:"response_times"`
	ThroughputRates     map[string]float64       `json:"throughput_rates"`
	ErrorRates          map[string]float64       `json:"error_rates"`
	ResourceUtilization map[string]float64       `json:"resource_utilization"`
}

// Service-specific performance metrics structures
type AuditPerformanceMetrics struct {
	TotalLogged      int64         `json:"total_logged"`
	BufferedCount    int64         `json:"buffered_count"`
	AverageLatency   time.Duration `json:"average_latency"`
	ThroughputPerSec float64       `json:"throughput_per_sec"`
	FailureRate      float64       `json:"failure_rate"`
}

type EnforcementPerformanceMetrics struct {
	NetworkRequestsPerSec float64       `json:"network_requests_per_sec"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	BlockRate             float64       `json:"block_rate"`
	ProcessesMonitored    int64         `json:"processes_monitored"`
	ErrorRate             float64       `json:"error_rate"`
}

type RetentionPerformanceMetrics struct {
	ExecutionsPerHour    float64       `json:"executions_per_hour"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
	EntriesDeletedPerSec float64       `json:"entries_deleted_per_sec"`
	SuccessRate          float64       `json:"success_rate"`
}

type RotationPerformanceMetrics struct {
	RotationsPerHour       float64       `json:"rotations_per_hour"`
	AverageCompressionTime time.Duration `json:"average_compression_time"`
	CompressionRatio       float64       `json:"compression_ratio"`
	DiskSpaceSavedMB       float64       `json:"disk_space_saved_mb"`
}

type SessionPerformanceMetrics struct {
	ActiveSessions     int           `json:"active_sessions"`
	AverageSessionTime time.Duration `json:"average_session_time"`
	SessionsPerHour    float64       `json:"sessions_per_hour"`
	SessionFailureRate float64       `json:"session_failure_rate"`
}

// Performance threshold and alerting types
type PerformanceThreshold struct {
	Name        string  `json:"name"`
	MetricPath  string  `json:"metric_path"`
	Threshold   float64 `json:"threshold"`
	Operator    string  `json:"operator"` // "gt", "lt", "eq"
	Severity    string  `json:"severity"` // "info", "warning", "critical"
	Description string  `json:"description"`
}

type PerformanceAlert struct {
	ID           string               `json:"id"`
	Timestamp    time.Time            `json:"timestamp"`
	Threshold    PerformanceThreshold `json:"threshold"`
	CurrentValue float64              `json:"current_value"`
	Severity     string               `json:"severity"`
	Message      string               `json:"message"`
	Resolved     bool                 `json:"resolved"`
	ResolvedAt   *time.Time           `json:"resolved_at,omitempty"`
}

// MetricSnapshot represents a point-in-time metric collection
type MetricSnapshot struct {
	Timestamp time.Time     `json:"timestamp"`
	Metrics   SystemMetrics `json:"metrics"`
}

// TrendAnalysis represents performance trend analysis results
type TrendAnalysis struct {
	MetricName     string    `json:"metric_name"`
	CurrentValue   float64   `json:"current_value"`
	TrendDirection string    `json:"trend_direction"` // "improving", "degrading", "stable"
	ChangePercent  float64   `json:"change_percent"`
	Significance   string    `json:"significance"` // "low", "medium", "high"
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	Recommendation string    `json:"recommendation"`
}

// PerformanceReport represents a comprehensive performance analysis report
type PerformanceReport struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	CurrentMetrics  SystemMetrics      `json:"current_metrics"`
	TrendAnalysis   []TrendAnalysis    `json:"trend_analysis"`
	ActiveAlerts    []PerformanceAlert `json:"active_alerts"`
	Recommendations []string           `json:"recommendations"`
	HealthScore     float64            `json:"health_score"`
}

// NewPerformanceMonitor creates a new performance monitoring service
func NewPerformanceMonitor(
	config PerformanceConfig,
	logger logging.Logger,
	auditService *AuditService,
	retentionService *RetentionService,
	rotationService *LogRotationService,
) *PerformanceMonitor {
	return &PerformanceMonitor{
		logger:           logger,
		config:           config,
		auditService:     auditService,
		retentionService: retentionService,
		rotationService:  rotationService,
		metrics:          &SystemMetrics{},
		thresholds:       make(map[string]PerformanceThreshold),
		alerts:           make([]PerformanceAlert, 0),
		stopCh:           make(chan struct{}),
		maxTrendData:     config.MaxTrendDataPoints,
		trendData:        make([]MetricSnapshot, 0, config.MaxTrendDataPoints),
	}
}

// Start starts the performance monitoring service
func (pm *PerformanceMonitor) Start(ctx context.Context) error {
	pm.runningMu.Lock()
	defer pm.runningMu.Unlock()

	if pm.running {
		return fmt.Errorf("performance monitor is already running")
	}

	pm.logger.Info("Starting performance monitor")

	// Initialize default thresholds
	pm.initializeDefaultThresholds()

	// Start metric collection
	pm.wg.Add(1)
	go pm.metricsCollectionLoop(ctx)

	// Start alerting if enabled
	if pm.config.EnableAlerting {
		pm.wg.Add(1)
		go pm.alertingLoop(ctx)
	}

	// Start trend analysis if enabled
	if pm.config.EnableTrendAnalysis {
		pm.wg.Add(1)
		go pm.trendAnalysisLoop(ctx)
	}

	pm.running = true
	pm.logger.Info("Performance monitor started successfully")
	return nil
}

// Stop stops the performance monitoring service
func (pm *PerformanceMonitor) Stop() error {
	pm.runningMu.Lock()
	defer pm.runningMu.Unlock()

	if !pm.running {
		return nil
	}

	pm.logger.Info("Stopping performance monitor")

	close(pm.stopCh)
	pm.wg.Wait()

	pm.running = false
	pm.logger.Info("Performance monitor stopped")
	return nil
}

// GetCurrentMetrics returns the current system performance metrics
func (pm *PerformanceMonitor) GetCurrentMetrics() *SystemMetrics {
	pm.metricsMu.RLock()
	defer pm.metricsMu.RUnlock()

	// Create a deep copy to prevent race conditions
	metrics := *pm.metrics
	return &metrics
}

// GetPerformanceReport generates a comprehensive performance report
func (pm *PerformanceMonitor) GetPerformanceReport() *PerformanceReport {
	currentMetrics := pm.GetCurrentMetrics()
	trendAnalysis := pm.analyzeTrends()
	activeAlerts := pm.getActiveAlerts()
	recommendations := pm.generateRecommendations(currentMetrics, trendAnalysis, activeAlerts)
	healthScore := pm.calculateHealthScore(currentMetrics, activeAlerts)

	return &PerformanceReport{
		GeneratedAt:     time.Now(),
		CurrentMetrics:  *currentMetrics,
		TrendAnalysis:   trendAnalysis,
		ActiveAlerts:    activeAlerts,
		Recommendations: recommendations,
		HealthScore:     healthScore,
	}
}

// GetActiveAlerts returns currently active performance alerts
func (pm *PerformanceMonitor) GetActiveAlerts() []PerformanceAlert {
	return pm.getActiveAlerts()
}

// AddThreshold adds a custom performance threshold
func (pm *PerformanceMonitor) AddThreshold(threshold PerformanceThreshold) {
	pm.thresholds[threshold.Name] = threshold
	pm.logger.Info("Added performance threshold",
		logging.String("name", threshold.Name),
		logging.String("metric", threshold.MetricPath),
		logging.Field{Key: "threshold", Value: threshold.Threshold})
}

// RemoveThreshold removes a performance threshold
func (pm *PerformanceMonitor) RemoveThreshold(name string) {
	delete(pm.thresholds, name)
	pm.logger.Info("Removed performance threshold",
		logging.String("name", name))
}

// Private methods

func (pm *PerformanceMonitor) metricsCollectionLoop(ctx context.Context) {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.collectMetrics()
		}
	}
}

func (pm *PerformanceMonitor) collectMetrics() {
	metrics := &SystemMetrics{
		Timestamp:           time.Now(),
		ResponseTimes:       make(map[string]time.Duration),
		ThroughputRates:     make(map[string]float64),
		ErrorRates:          make(map[string]float64),
		ResourceUtilization: make(map[string]float64),
	}

	// Collect system resource metrics
	pm.collectSystemMetrics(metrics)

	// Collect service-specific metrics
	pm.collectServiceMetrics(metrics)

	// Update current metrics
	pm.metricsMu.Lock()
	pm.metrics = metrics
	pm.metricsMu.Unlock()

	// Add to trend data
	pm.addToTrendData(*metrics)
}

func (pm *PerformanceMonitor) collectSystemMetrics(metrics *SystemMetrics) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics.MemoryUsage = int64(m.Alloc)
	metrics.CPUUsage = pm.getCPUUsage()

	// Get disk usage (this would be platform-specific in a real implementation)
	metrics.DiskUsage = 0.5               // Placeholder
	metrics.DiskFree = 1024 * 1024 * 1024 // Placeholder: 1GB
}

func (pm *PerformanceMonitor) collectServiceMetrics(metrics *SystemMetrics) {
	// Collect audit service metrics
	if pm.auditService != nil {
		auditStats := pm.auditService.GetStats()
		metrics.AuditMetrics = &AuditPerformanceMetrics{
			TotalLogged:      auditStats.TotalLogged,
			BufferedCount:    auditStats.BufferedCount,
			AverageLatency:   auditStats.AverageLatency,
			ThroughputPerSec: pm.calculateThroughput(auditStats.TotalLogged),
			FailureRate:      pm.calculateFailureRate(auditStats.FailedCount, auditStats.TotalLogged),
		}
	}

	// Collect retention service metrics
	if pm.retentionService != nil {
		retentionStats := pm.retentionService.GetStats()
		metrics.RetentionMetrics = &RetentionPerformanceMetrics{
			ExecutionsPerHour:    pm.calculateExecutionsPerHour(retentionStats.TotalExecutions),
			AverageExecutionTime: retentionStats.AverageExecutionTime,
			EntriesDeletedPerSec: pm.calculateDeletionRate(retentionStats.TotalEntriesDeleted),
			SuccessRate:          pm.calculateSuccessRate(retentionStats.SuccessfulExecutions, retentionStats.TotalExecutions),
		}
	}

	// Collect rotation service metrics
	if pm.rotationService != nil {
		rotationStats := pm.rotationService.GetStats()
		if rotationStats != nil {
			metrics.RotationMetrics = &RotationPerformanceMetrics{
				RotationsPerHour:       pm.calculateRotationsPerHour(rotationStats.TotalFilesRotated),
				AverageCompressionTime: 0, // Note: AverageCompressionTime not available in current RotationStats
				CompressionRatio:       rotationStats.AverageCompressionRatio,
				DiskSpaceSavedMB:       float64(rotationStats.TotalBytesFreed) / (1024 * 1024),
			}
		}
	}
}

func (pm *PerformanceMonitor) getCPUUsage() float64 {
	// This is a simplified CPU usage calculation
	// In a real implementation, this would use platform-specific system calls
	return 5.0 // Placeholder: 5% CPU usage
}

func (pm *PerformanceMonitor) calculateThroughput(total int64) float64 {
	// Calculate throughput based on recent activity
	// This is a simplified calculation
	return float64(total) / 3600.0 // Per hour
}

func (pm *PerformanceMonitor) calculateFailureRate(failed, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(failed) / float64(total) * 100.0
}

func (pm *PerformanceMonitor) calculateExecutionsPerHour(executions int64) float64 {
	return float64(executions) / 24.0 // Assuming 24-hour period
}

func (pm *PerformanceMonitor) calculateDeletionRate(deleted int64) float64 {
	return float64(deleted) / 3600.0 // Per second over an hour
}

func (pm *PerformanceMonitor) calculateSuccessRate(successful, total int64) float64 {
	if total == 0 {
		return 100.0
	}
	return float64(successful) / float64(total) * 100.0
}

func (pm *PerformanceMonitor) calculateRotationsPerHour(rotations int64) float64 {
	return float64(rotations) / 24.0 // Assuming 24-hour period
}

func (pm *PerformanceMonitor) addToTrendData(metrics SystemMetrics) {
	pm.trendDataMu.Lock()
	defer pm.trendDataMu.Unlock()

	snapshot := MetricSnapshot{
		Timestamp: metrics.Timestamp,
		Metrics:   metrics,
	}

	pm.trendData = append(pm.trendData, snapshot)

	// Remove old data if we exceed the maximum
	if len(pm.trendData) > pm.maxTrendData {
		pm.trendData = pm.trendData[1:]
	}
}

func (pm *PerformanceMonitor) initializeDefaultThresholds() {
	thresholds := []PerformanceThreshold{
		{
			Name:        "high_memory_usage",
			MetricPath:  "memory_usage_bytes",
			Threshold:   float64(pm.config.MaxMemoryUsageMB * 1024 * 1024),
			Operator:    "gt",
			Severity:    "warning",
			Description: "Memory usage exceeds configured limit",
		},
		{
			Name:        "high_cpu_usage",
			MetricPath:  "cpu_usage_percent",
			Threshold:   pm.config.MaxCPUUsagePercent,
			Operator:    "gt",
			Severity:    "warning",
			Description: "CPU usage exceeds configured limit",
		},
		{
			Name:        "high_disk_usage",
			MetricPath:  "disk_usage_percent",
			Threshold:   pm.config.MaxDiskUsagePercent,
			Operator:    "gt",
			Severity:    "critical",
			Description: "Disk usage exceeds configured limit",
		},
		{
			Name:        "high_audit_failure_rate",
			MetricPath:  "audit_metrics.failure_rate",
			Threshold:   5.0, // 5% failure rate
			Operator:    "gt",
			Severity:    "warning",
			Description: "Audit logging failure rate is too high",
		},
	}

	for _, threshold := range thresholds {
		pm.thresholds[threshold.Name] = threshold
	}
}

func (pm *PerformanceMonitor) alertingLoop(ctx context.Context) {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.AlertCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.checkThresholds()
		}
	}
}

func (pm *PerformanceMonitor) checkThresholds() {
	currentMetrics := pm.GetCurrentMetrics()

	for _, threshold := range pm.thresholds {
		value := pm.extractMetricValue(currentMetrics, threshold.MetricPath)
		if pm.evaluateThreshold(value, threshold) {
			pm.triggerAlert(threshold, value)
		}
	}
}

func (pm *PerformanceMonitor) extractMetricValue(metrics *SystemMetrics, path string) float64 {
	// Simple metric extraction based on path
	// In a real implementation, this would use reflection or a more sophisticated approach
	switch path {
	case "memory_usage_bytes":
		return float64(metrics.MemoryUsage)
	case "cpu_usage_percent":
		return metrics.CPUUsage
	case "disk_usage_percent":
		return metrics.DiskUsage
	case "audit_metrics.failure_rate":
		if metrics.AuditMetrics != nil {
			return metrics.AuditMetrics.FailureRate
		}
	}
	return 0.0
}

func (pm *PerformanceMonitor) evaluateThreshold(value float64, threshold PerformanceThreshold) bool {
	switch threshold.Operator {
	case "gt":
		return value > threshold.Threshold
	case "lt":
		return value < threshold.Threshold
	case "eq":
		return value == threshold.Threshold
	}
	return false
}

func (pm *PerformanceMonitor) triggerAlert(threshold PerformanceThreshold, currentValue float64) {
	alert := PerformanceAlert{
		ID:           pm.generateAlertID(threshold),
		Timestamp:    time.Now(),
		Threshold:    threshold,
		CurrentValue: currentValue,
		Severity:     threshold.Severity,
		Message:      fmt.Sprintf("%s: current value %.2f exceeds threshold %.2f", threshold.Description, currentValue, threshold.Threshold),
		Resolved:     false,
	}

	pm.alerts = append(pm.alerts, alert)

	pm.logger.Warn("Performance alert triggered",
		logging.String("alert_id", alert.ID),
		logging.String("threshold", threshold.Name),
		logging.Field{Key: "current_value", Value: currentValue},
		logging.Field{Key: "threshold_value", Value: threshold.Threshold})
}

func (pm *PerformanceMonitor) generateAlertID(threshold PerformanceThreshold) string {
	return fmt.Sprintf("%s_%d", threshold.Name, time.Now().Unix())
}

func (pm *PerformanceMonitor) getActiveAlerts() []PerformanceAlert {
	var activeAlerts []PerformanceAlert
	for _, alert := range pm.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts
}

func (pm *PerformanceMonitor) trendAnalysisLoop(ctx context.Context) {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.CollectionInterval * 10) // Analyze trends every 10 collection intervals
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.analyzeTrends()
		}
	}
}

func (pm *PerformanceMonitor) analyzeTrends() []TrendAnalysis {
	pm.trendDataMu.RLock()
	defer pm.trendDataMu.RUnlock()

	if len(pm.trendData) < pm.config.TrendAnalysisWindow {
		return nil // Not enough data for analysis
	}

	var analyses []TrendAnalysis

	// Analyze memory usage trend
	memoryTrend := pm.analyzeTrendForMetric("memory_usage", func(snapshot MetricSnapshot) float64 {
		return float64(snapshot.Metrics.MemoryUsage)
	})
	if memoryTrend != nil {
		analyses = append(analyses, *memoryTrend)
	}

	// Analyze CPU usage trend
	cpuTrend := pm.analyzeTrendForMetric("cpu_usage", func(snapshot MetricSnapshot) float64 {
		return snapshot.Metrics.CPUUsage
	})
	if cpuTrend != nil {
		analyses = append(analyses, *cpuTrend)
	}

	return analyses
}

func (pm *PerformanceMonitor) analyzeTrendForMetric(metricName string, extractor func(MetricSnapshot) float64) *TrendAnalysis {
	windowSize := pm.config.TrendAnalysisWindow
	if len(pm.trendData) < windowSize {
		return nil
	}

	recentData := pm.trendData[len(pm.trendData)-windowSize:]

	// Calculate trend direction and significance
	firstValue := extractor(recentData[0])
	lastValue := extractor(recentData[len(recentData)-1])

	changePercent := ((lastValue - firstValue) / firstValue) * 100

	var direction string
	var significance string

	if abs(changePercent) < 5 {
		direction = "stable"
		significance = "low"
	} else if changePercent > 0 {
		direction = "degrading"
		significance = pm.getSignificance(changePercent)
	} else {
		direction = "improving"
		significance = pm.getSignificance(abs(changePercent))
	}

	recommendation := pm.generateTrendRecommendation(metricName, direction, changePercent)

	return &TrendAnalysis{
		MetricName:     metricName,
		CurrentValue:   lastValue,
		TrendDirection: direction,
		ChangePercent:  changePercent,
		Significance:   significance,
		WindowStart:    recentData[0].Timestamp,
		WindowEnd:      recentData[len(recentData)-1].Timestamp,
		Recommendation: recommendation,
	}
}

func (pm *PerformanceMonitor) getSignificance(changePercent float64) string {
	if changePercent > 20 {
		return "high"
	} else if changePercent > 10 {
		return "medium"
	}
	return "low"
}

func (pm *PerformanceMonitor) generateTrendRecommendation(metricName, direction string, changePercent float64) string {
	if direction == "stable" {
		return "Performance is stable. Continue monitoring."
	}

	switch metricName {
	case "memory_usage":
		if direction == "degrading" {
			return "Memory usage is increasing. Consider optimizing memory usage or increasing available memory."
		}
		return "Memory usage is improving. Good optimization work."
	case "cpu_usage":
		if direction == "degrading" {
			return "CPU usage is increasing. Consider optimizing algorithms or scaling resources."
		}
		return "CPU usage is improving. Good performance optimization."
	}

	return "Monitor this trend and consider optimization if degradation continues."
}

func (pm *PerformanceMonitor) generateRecommendations(metrics *SystemMetrics, trends []TrendAnalysis, alerts []PerformanceAlert) []string {
	var recommendations []string

	// Generate recommendations based on current metrics
	if metrics.MemoryUsage > int64(pm.config.MaxMemoryUsageMB*1024*1024)*8/10 { // 80% of limit
		recommendations = append(recommendations, "Memory usage is approaching the configured limit. Consider memory optimization.")
	}

	if metrics.CPUUsage > pm.config.MaxCPUUsagePercent*0.8 { // 80% of limit
		recommendations = append(recommendations, "CPU usage is high. Consider performance optimization or resource scaling.")
	}

	// Generate recommendations based on trends
	for _, trend := range trends {
		if trend.TrendDirection == "degrading" && trend.Significance == "high" {
			recommendations = append(recommendations, fmt.Sprintf("Significant performance degradation detected in %s. Immediate attention recommended.", trend.MetricName))
		}
	}

	// Generate recommendations based on alerts
	if len(alerts) > 0 {
		recommendations = append(recommendations, "Active performance alerts require attention. Review and resolve threshold violations.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "System performance is within acceptable parameters. Continue regular monitoring.")
	}

	return recommendations
}

func (pm *PerformanceMonitor) calculateHealthScore(metrics *SystemMetrics, alerts []PerformanceAlert) float64 {
	score := 100.0

	// Deduct points for resource usage
	memoryUsagePercent := float64(metrics.MemoryUsage) / float64(pm.config.MaxMemoryUsageMB*1024*1024) * 100
	if memoryUsagePercent > 80 {
		score -= (memoryUsagePercent - 80) * 0.5
	}

	if metrics.CPUUsage > 80 {
		score -= (metrics.CPUUsage - 80) * 0.5
	}

	// Deduct points for active alerts
	for _, alert := range alerts {
		switch alert.Severity {
		case "critical":
			score -= 20
		case "warning":
			score -= 10
		case "info":
			score -= 2
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
