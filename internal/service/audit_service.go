package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"parental-control/internal/database"
	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// AuditService provides comprehensive audit logging functionality
type AuditService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
	config AuditConfig

	// Asynchronous logging
	logBuffer chan *models.AuditLog
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	runningMu sync.RWMutex

	// Performance metrics
	stats   *AuditStats
	statsMu sync.RWMutex

	// Batch processing
	batchMu   sync.Mutex
	batch     []*models.AuditLog
	lastFlush time.Time
}

// AuditConfig holds configuration for the audit service
type AuditConfig struct {
	// Buffer size for asynchronous logging
	BufferSize int `json:"buffer_size"`

	// Batch processing settings
	BatchSize     int           `json:"batch_size"`
	BatchTimeout  time.Duration `json:"batch_timeout"`
	FlushInterval time.Duration `json:"flush_interval"`

	// Performance settings
	EnableBuffering bool `json:"enable_buffering"`
	EnableBatching  bool `json:"enable_batching"`

	// Retention settings
	RetentionDays   int           `json:"retention_days"`
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// Event filtering
	LogLevels         []string `json:"log_levels"`
	EnabledEventTypes []string `json:"enabled_event_types"`
}

// DefaultAuditConfig returns audit service configuration with sensible defaults
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		BufferSize:        1000,
		BatchSize:         50,
		BatchTimeout:      5 * time.Second,
		FlushInterval:     10 * time.Second,
		EnableBuffering:   true,
		EnableBatching:    true,
		RetentionDays:     30,
		CleanupInterval:   24 * time.Hour,
		LogLevels:         []string{"info", "warn", "error", "critical"},
		EnabledEventTypes: []string{"enforcement_action", "rule_change", "user_action", "system_event"},
	}
}

// AuditStats holds performance and usage statistics
type AuditStats struct {
	TotalLogged    int64         `json:"total_logged"`
	BufferedCount  int64         `json:"buffered_count"`
	BatchCount     int64         `json:"batch_count"`
	FailedCount    int64         `json:"failed_count"`
	AverageLatency time.Duration `json:"average_latency"`
	LastCleanup    time.Time     `json:"last_cleanup"`
	CleanedCount   int64         `json:"cleaned_count"`

	// Event type statistics
	EventTypeStats  map[string]int64 `json:"event_type_stats"`
	ActionTypeStats map[string]int64 `json:"action_type_stats"`
	TargetTypeStats map[string]int64 `json:"target_type_stats"`
}

// NewAuditService creates a new audit service
func NewAuditService(repos *models.RepositoryManager, logger logging.Logger, config AuditConfig) *AuditService {
	return &AuditService{
		repos:     repos,
		logger:    logger,
		config:    config,
		logBuffer: make(chan *models.AuditLog, config.BufferSize),
		stopCh:    make(chan struct{}),
		batch:     make([]*models.AuditLog, 0, config.BatchSize),
		lastFlush: time.Now(),
		stats: &AuditStats{
			EventTypeStats:  make(map[string]int64),
			ActionTypeStats: make(map[string]int64),
			TargetTypeStats: make(map[string]int64),
		},
	}
}

// Start starts the audit service background processing
func (s *AuditService) Start(ctx context.Context) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return fmt.Errorf("audit service is already running")
	}

	s.logger.Info("Starting audit service")

	// Start background workers
	if s.config.EnableBuffering {
		s.wg.Add(1)
		go s.bufferProcessor(ctx)
	}

	if s.config.EnableBatching {
		s.wg.Add(1)
		go s.batchProcessor(ctx)
	}

	// Start cleanup routine
	s.wg.Add(1)
	go s.cleanupRoutine(ctx)

	s.running = true
	s.logger.Info("Audit service started successfully")
	return nil
}

// Stop stops the audit service
func (s *AuditService) Stop() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping audit service")

	// Stop processing
	close(s.stopCh)

	// Flush any remaining logs
	if err := s.flushBatch(context.Background()); err != nil {
		s.logger.Error("Error flushing final batch", logging.Err(err))
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.running = false
	s.logger.Info("Audit service stopped")
	return nil
}

// LogEnforcementAction logs an enforcement action (allow/block)
func (s *AuditService) LogEnforcementAction(ctx context.Context, action models.ActionType, targetType models.TargetType, targetValue string, ruleType string, ruleID *int, details map[string]interface{}) error {
	return s.LogEvent(ctx, AuditEventRequest{
		EventType:   "enforcement_action",
		TargetType:  targetType,
		TargetValue: targetValue,
		Action:      action,
		RuleType:    ruleType,
		RuleID:      ruleID,
		Details:     details,
	})
}

// LogRuleChange logs a rule configuration change
func (s *AuditService) LogRuleChange(ctx context.Context, ruleType string, ruleID int, operation string, details map[string]interface{}) error {
	return s.LogEvent(ctx, AuditEventRequest{
		EventType:   "rule_change",
		TargetType:  models.TargetTypeURL, // Default for config changes
		TargetValue: fmt.Sprintf("%s:%d", ruleType, ruleID),
		Action:      models.ActionTypeAllow, // Config changes are neutral
		RuleType:    ruleType,
		RuleID:      &ruleID,
		Details: map[string]interface{}{
			"operation": operation,
			"details":   details,
		},
	})
}

// LogUserAction logs a user action (login, logout, config change)
func (s *AuditService) LogUserAction(ctx context.Context, userID int, action string, details map[string]interface{}) error {
	return s.LogEvent(ctx, AuditEventRequest{
		EventType:   "user_action",
		TargetType:  models.TargetTypeURL, // User actions are system-level
		TargetValue: fmt.Sprintf("user:%d", userID),
		Action:      models.ActionTypeAllow,
		Details: map[string]interface{}{
			"user_id": userID,
			"action":  action,
			"details": details,
		},
	})
}

// LogSystemEvent logs a system event (startup, shutdown, error)
func (s *AuditService) LogSystemEvent(ctx context.Context, eventType string, severity string, details map[string]interface{}) error {
	action := models.ActionTypeAllow
	if severity == "error" || severity == "critical" {
		action = models.ActionTypeBlock // Use block to indicate errors
	}

	return s.LogEvent(ctx, AuditEventRequest{
		EventType:   "system_event",
		TargetType:  models.TargetTypeURL,
		TargetValue: eventType,
		Action:      action,
		Details: map[string]interface{}{
			"severity": severity,
			"details":  details,
		},
	})
}

// AuditEventRequest represents a request to log an audit event
type AuditEventRequest struct {
	EventType   string                 `json:"event_type"`
	TargetType  models.TargetType      `json:"target_type"`
	TargetValue string                 `json:"target_value"`
	Action      models.ActionType      `json:"action"`
	RuleType    string                 `json:"rule_type,omitempty"`
	RuleID      *int                   `json:"rule_id,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// LogEvent logs a generic audit event
func (s *AuditService) LogEvent(ctx context.Context, req AuditEventRequest) error {
	startTime := time.Now()

	// Check if event type is enabled
	if !s.isEventTypeEnabled(req.EventType) {
		return nil // Silently skip disabled event types
	}

	// Create audit log entry
	auditLog := &models.AuditLog{
		Timestamp:   time.Now(),
		EventType:   req.EventType,
		TargetType:  req.TargetType,
		TargetValue: req.TargetValue,
		Action:      req.Action,
		RuleType:    req.RuleType,
		RuleID:      req.RuleID,
		CreatedAt:   time.Now(),
	}

	// Set details if provided
	if req.Details != nil {
		if err := auditLog.SetDetailsMap(req.Details); err != nil {
			s.logger.Error("Failed to set audit log details", logging.Err(err))
			return fmt.Errorf("failed to set audit log details: %w", err)
		}
	}

	// Update statistics
	s.updateStats(auditLog, time.Since(startTime))

	// Handle logging based on configuration
	if s.config.EnableBuffering && s.running {
		return s.bufferLog(ctx, auditLog)
	}

	// Direct database write
	return s.writeLog(ctx, auditLog)
}

// GetAuditLogs retrieves audit logs with filtering
func (s *AuditService) GetAuditLogs(ctx context.Context, filters AuditLogFilters) ([]models.AuditLog, int, error) {
	// Convert to repository filters
	repoFilters := database.AuditLogFilters{
		Action:     filters.Action,
		TargetType: filters.TargetType,
		EventType:  filters.EventType,
		StartTime:  filters.StartTime,
		EndTime:    filters.EndTime,
		Search:     filters.Search,
		Limit:      filters.Limit,
		Offset:     filters.Offset,
	}

	// Get logs
	logs, err := s.repos.AuditLog.(*database.AuditLogRepository).GetByFilters(ctx, repoFilters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get audit logs: %w", err)
	}

	// Get total count for pagination
	totalCount, err := s.getFilteredCount(ctx, filters)
	if err != nil {
		s.logger.Error("Failed to get audit log count", logging.Err(err))
		totalCount = len(logs) // Fallback to current page count
	}

	return logs, totalCount, nil
}

// GetStats returns audit service statistics
func (s *AuditService) GetStats() *AuditStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// Create a copy to prevent race conditions
	stats := &AuditStats{
		TotalLogged:     s.stats.TotalLogged,
		BufferedCount:   s.stats.BufferedCount,
		BatchCount:      s.stats.BatchCount,
		FailedCount:     s.stats.FailedCount,
		AverageLatency:  s.stats.AverageLatency,
		LastCleanup:     s.stats.LastCleanup,
		CleanedCount:    s.stats.CleanedCount,
		EventTypeStats:  make(map[string]int64),
		ActionTypeStats: make(map[string]int64),
		TargetTypeStats: make(map[string]int64),
	}

	// Copy maps
	for k, v := range s.stats.EventTypeStats {
		stats.EventTypeStats[k] = v
	}
	for k, v := range s.stats.ActionTypeStats {
		stats.ActionTypeStats[k] = v
	}
	for k, v := range s.stats.TargetTypeStats {
		stats.TargetTypeStats[k] = v
	}

	return stats
}

// CleanupOldLogs manually triggers cleanup of old logs
func (s *AuditService) CleanupOldLogs(ctx context.Context) (int64, error) {
	if s.config.RetentionDays <= 0 {
		return 0, nil // No cleanup configured
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.config.RetentionDays)

	// Count logs before cleanup for reporting
	count, err := s.repos.AuditLog.CountByTimeRange(ctx, time.Time{}, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to count logs for cleanup: %w", err)
	}

	if count == 0 {
		return 0, nil // Nothing to cleanup
	}

	// Perform cleanup
	err = s.repos.AuditLog.CleanupOldLogs(ctx, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	// Update statistics
	s.statsMu.Lock()
	s.stats.LastCleanup = time.Now()
	s.stats.CleanedCount += int64(count)
	s.statsMu.Unlock()

	s.logger.Info("Audit log cleanup completed",
		logging.Int("cleaned_count", count),
		logging.String("cutoff_time", cutoffTime.Format(time.RFC3339)))

	return int64(count), nil
}

// Private methods

func (s *AuditService) bufferLog(ctx context.Context, log *models.AuditLog) error {
	select {
	case s.logBuffer <- log:
		s.statsMu.Lock()
		s.stats.BufferedCount++
		s.statsMu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Buffer is full, fall back to direct write
		s.logger.Warn("Audit log buffer full, falling back to direct write")
		return s.writeLog(ctx, log)
	}
}

func (s *AuditService) writeLog(ctx context.Context, log *models.AuditLog) error {
	err := s.repos.AuditLog.Create(ctx, log)
	if err != nil {
		s.statsMu.Lock()
		s.stats.FailedCount++
		s.statsMu.Unlock()
		return err
	}

	return nil
}

func (s *AuditService) bufferProcessor(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case log := <-s.logBuffer:
			if s.config.EnableBatching {
				s.addToBatch(log)
			} else {
				if err := s.writeLog(ctx, log); err != nil {
					s.logger.Error("Failed to write audit log", logging.Err(err))
				}
			}
		}
	}
}

func (s *AuditService) batchProcessor(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.flushBatch(ctx); err != nil {
				s.logger.Error("Failed to flush audit log batch", logging.Err(err))
			}
		}
	}
}

func (s *AuditService) addToBatch(log *models.AuditLog) {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	s.batch = append(s.batch, log)

	// Check if batch is full or timeout reached
	if len(s.batch) >= s.config.BatchSize || time.Since(s.lastFlush) >= s.config.BatchTimeout {
		// Schedule immediate flush (in a separate goroutine to avoid blocking)
		go func() {
			if err := s.flushBatch(context.Background()); err != nil {
				s.logger.Error("Failed to flush batch on size/timeout", logging.Err(err))
			}
		}()
	}
}

func (s *AuditService) flushBatch(ctx context.Context) error {
	s.batchMu.Lock()
	defer s.batchMu.Unlock()

	if len(s.batch) == 0 {
		return nil
	}

	// Copy batch and reset
	batch := make([]*models.AuditLog, len(s.batch))
	copy(batch, s.batch)
	s.batch = s.batch[:0] // Reset slice but keep capacity
	s.lastFlush = time.Now()

	// Write batch to database
	for _, log := range batch {
		if err := s.writeLog(ctx, log); err != nil {
			s.logger.Error("Failed to write batched audit log", logging.Err(err))
			// Continue with other logs in batch
		}
	}

	s.statsMu.Lock()
	s.stats.BatchCount++
	s.statsMu.Unlock()

	return nil
}

func (s *AuditService) cleanupRoutine(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if _, err := s.CleanupOldLogs(ctx); err != nil {
				s.logger.Error("Automatic cleanup failed", logging.Err(err))
			}
		}
	}
}

func (s *AuditService) updateStats(log *models.AuditLog, latency time.Duration) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	s.stats.TotalLogged++
	s.stats.EventTypeStats[log.EventType]++
	s.stats.ActionTypeStats[string(log.Action)]++
	s.stats.TargetTypeStats[string(log.TargetType)]++

	// Update average latency
	if s.stats.AverageLatency == 0 {
		s.stats.AverageLatency = latency
	} else {
		s.stats.AverageLatency = (s.stats.AverageLatency + latency) / 2
	}
}

func (s *AuditService) isEventTypeEnabled(eventType string) bool {
	for _, enabled := range s.config.EnabledEventTypes {
		if enabled == eventType {
			return true
		}
	}
	return false
}

func (s *AuditService) convertFilters(filters AuditLogFilters) AuditLogFilters {
	// This allows for any filter transformations if needed
	return filters
}

func (s *AuditService) getFilteredCount(ctx context.Context, filters AuditLogFilters) (int, error) {
	if filters.StartTime != nil && filters.EndTime != nil {
		return s.repos.AuditLog.CountByTimeRange(ctx, *filters.StartTime, *filters.EndTime)
	}
	return s.repos.AuditLog.Count(ctx)
}

// AuditLogFilters represents filtering options for audit log queries
type AuditLogFilters struct {
	Action     *models.ActionType `json:"action,omitempty"`
	TargetType *models.TargetType `json:"target_type,omitempty"`
	EventType  string             `json:"event_type,omitempty"`
	StartTime  *time.Time         `json:"start_time,omitempty"`
	EndTime    *time.Time         `json:"end_time,omitempty"`
	Search     string             `json:"search,omitempty"`
	Limit      int                `json:"limit,omitempty"`
	Offset     int                `json:"offset,omitempty"`
}
