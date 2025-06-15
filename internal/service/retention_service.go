package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// RetentionService manages retention policy execution and enforcement
type RetentionService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
	config RetentionConfig

	// Execution management
	running   bool
	runningMu sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// Scheduler
	scheduler *RetentionScheduler

	// Statistics
	stats   *RetentionServiceStats
	statsMu sync.RWMutex
}

// RetentionConfig holds configuration for the retention service
type RetentionConfig struct {
	// Execution settings
	CheckInterval     time.Duration `json:"check_interval"`      // How often to check for policies to execute
	MaxConcurrentJobs int           `json:"max_concurrent_jobs"` // Maximum concurrent retention jobs
	JobTimeout        time.Duration `json:"job_timeout"`         // Timeout for individual retention jobs

	// Safety settings
	MaxDeleteBatchSize int     `json:"max_delete_batch_size"` // Maximum entries to delete in one batch
	SafetyThreshold    float64 `json:"safety_threshold"`      // Percentage threshold for safety checks (0.0-1.0)
	DryRunMode         bool    `json:"dry_run_mode"`          // If true, don't actually delete anything

	// Performance settings
	DeleteBatchSize  int           `json:"delete_batch_size"`  // Batch size for deletions
	DeleteBatchDelay time.Duration `json:"delete_batch_delay"` // Delay between delete batches

	// Monitoring
	EnableDetailedStats bool `json:"enable_detailed_stats"` // Enable detailed statistics collection
}

// DefaultRetentionConfig returns retention service configuration with sensible defaults
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		CheckInterval:       5 * time.Minute,
		MaxConcurrentJobs:   3,
		JobTimeout:          30 * time.Minute,
		MaxDeleteBatchSize:  10000,
		SafetyThreshold:     0.8, // Don't delete more than 80% of logs in one run
		DryRunMode:          false,
		DeleteBatchSize:     1000,
		DeleteBatchDelay:    100 * time.Millisecond,
		EnableDetailedStats: true,
	}
}

// RetentionServiceStats holds statistics about retention service operations
type RetentionServiceStats struct {
	TotalExecutions      int64                `json:"total_executions"`
	SuccessfulExecutions int64                `json:"successful_executions"`
	FailedExecutions     int64                `json:"failed_executions"`
	TotalEntriesDeleted  int64                `json:"total_entries_deleted"`
	TotalBytesFreed      int64                `json:"total_bytes_freed"`
	LastExecutionTime    time.Time            `json:"last_execution_time"`
	AverageExecutionTime time.Duration        `json:"average_execution_time"`
	ActiveJobs           int                  `json:"active_jobs"`
	PolicyStats          map[int]*PolicyStats `json:"policy_stats"`
}

// PolicyStats holds statistics for a specific retention policy
type PolicyStats struct {
	PolicyID             int           `json:"policy_id"`
	ExecutionCount       int64         `json:"execution_count"`
	LastExecutionTime    time.Time     `json:"last_execution_time"`
	TotalEntriesDeleted  int64         `json:"total_entries_deleted"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
	SuccessRate          float64       `json:"success_rate"`
}

// NewRetentionService creates a new retention service
func NewRetentionService(repos *models.RepositoryManager, logger logging.Logger, config RetentionConfig) *RetentionService {
	return &RetentionService{
		repos:     repos,
		logger:    logger,
		config:    config,
		stopCh:    make(chan struct{}),
		scheduler: NewRetentionScheduler(),
		stats: &RetentionServiceStats{
			PolicyStats: make(map[int]*PolicyStats),
		},
	}
}

// Start starts the retention service
func (rs *RetentionService) Start(ctx context.Context) error {
	rs.runningMu.Lock()
	defer rs.runningMu.Unlock()

	if rs.running {
		return fmt.Errorf("retention service is already running")
	}

	rs.logger.Info("Starting retention service")

	// Start the scheduler
	rs.wg.Add(1)
	go rs.schedulerLoop(ctx)

	rs.running = true
	rs.logger.Info("Retention service started successfully")
	return nil
}

// Stop stops the retention service
func (rs *RetentionService) Stop() error {
	rs.runningMu.Lock()
	defer rs.runningMu.Unlock()

	if !rs.running {
		return nil
	}

	rs.logger.Info("Stopping retention service")

	// Stop the scheduler
	close(rs.stopCh)
	rs.wg.Wait()

	rs.running = false
	rs.logger.Info("Retention service stopped")
	return nil
}

// ExecutePolicy manually executes a specific retention policy
func (rs *RetentionService) ExecutePolicy(ctx context.Context, policyID int) (*models.RetentionPolicyExecution, error) {
	// Get the policy
	policy, err := rs.repos.RetentionPolicy.GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	if !policy.Enabled {
		return nil, fmt.Errorf("retention policy %d is disabled", policyID)
	}

	return rs.executePolicy(ctx, policy)
}

// ExecuteAllPolicies manually executes all enabled retention policies
func (rs *RetentionService) ExecuteAllPolicies(ctx context.Context) ([]*models.RetentionPolicyExecution, error) {
	policies, err := rs.repos.RetentionPolicy.GetEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled policies: %w", err)
	}

	var executions []*models.RetentionPolicyExecution
	for _, policy := range policies {
		execution, err := rs.executePolicy(ctx, &policy)
		if err != nil {
			rs.logger.Error("Failed to execute retention policy",
				logging.Int("policy_id", policy.ID),
				logging.String("policy_name", policy.Name),
				logging.Err(err))
			continue
		}
		executions = append(executions, execution)
	}

	return executions, nil
}

// GetStats returns retention service statistics
func (rs *RetentionService) GetStats() *RetentionServiceStats {
	rs.statsMu.RLock()
	defer rs.statsMu.RUnlock()

	// Create a copy to prevent race conditions
	stats := &RetentionServiceStats{
		TotalExecutions:      rs.stats.TotalExecutions,
		SuccessfulExecutions: rs.stats.SuccessfulExecutions,
		FailedExecutions:     rs.stats.FailedExecutions,
		TotalEntriesDeleted:  rs.stats.TotalEntriesDeleted,
		TotalBytesFreed:      rs.stats.TotalBytesFreed,
		LastExecutionTime:    rs.stats.LastExecutionTime,
		AverageExecutionTime: rs.stats.AverageExecutionTime,
		ActiveJobs:           rs.stats.ActiveJobs,
		PolicyStats:          make(map[int]*PolicyStats),
	}

	// Copy policy stats
	for k, v := range rs.stats.PolicyStats {
		stats.PolicyStats[k] = &PolicyStats{
			PolicyID:             v.PolicyID,
			ExecutionCount:       v.ExecutionCount,
			LastExecutionTime:    v.LastExecutionTime,
			TotalEntriesDeleted:  v.TotalEntriesDeleted,
			AverageExecutionTime: v.AverageExecutionTime,
			SuccessRate:          v.SuccessRate,
		}
	}

	return stats
}

// PreviewPolicyExecution previews what a policy execution would do without actually executing it
func (rs *RetentionService) PreviewPolicyExecution(ctx context.Context, policyID int) (*RetentionPreview, error) {
	policy, err := rs.repos.RetentionPolicy.GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	return rs.previewPolicyExecution(ctx, policy)
}

// RetentionPreview represents a preview of what a retention policy execution would do
type RetentionPreview struct {
	PolicyID            int           `json:"policy_id"`
	PolicyName          string        `json:"policy_name"`
	EstimatedDeletions  int64         `json:"estimated_deletions"`
	EstimatedBytesFreed int64         `json:"estimated_bytes_freed"`
	AffectedTimeRange   TimeRange     `json:"affected_time_range"`
	RuleBreakdown       []RulePreview `json:"rule_breakdown"`
	SafetyWarnings      []string      `json:"safety_warnings"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RulePreview represents a preview of what a specific rule would do
type RulePreview struct {
	RuleType           string    `json:"rule_type"`
	EstimatedDeletions int64     `json:"estimated_deletions"`
	CutoffTime         time.Time `json:"cutoff_time,omitempty"`
	Description        string    `json:"description"`
}

// Private methods

func (rs *RetentionService) schedulerLoop(ctx context.Context) {
	defer rs.wg.Done()

	ticker := time.NewTicker(rs.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rs.stopCh:
			return
		case <-ticker.C:
			rs.checkAndExecutePolicies(ctx)
		}
	}
}

func (rs *RetentionService) checkAndExecutePolicies(ctx context.Context) {
	policies, err := rs.repos.RetentionPolicy.GetEnabled(ctx)
	if err != nil {
		rs.logger.Error("Failed to get enabled retention policies", logging.Err(err))
		return
	}

	for _, policy := range policies {
		if rs.shouldExecutePolicy(&policy) {
			// Execute policy in a separate goroutine to avoid blocking
			go func(p models.RetentionPolicy) {
				if _, err := rs.executePolicy(ctx, &p); err != nil {
					rs.logger.Error("Failed to execute scheduled retention policy",
						logging.Int("policy_id", p.ID),
						logging.String("policy_name", p.Name),
						logging.Err(err))
				}
			}(policy)
		}
	}
}

func (rs *RetentionService) shouldExecutePolicy(policy *models.RetentionPolicy) bool {
	// Simple time-based check - in a real implementation, you'd use a proper cron parser
	if policy.NextExecution.IsZero() {
		return true // Never executed before
	}

	return time.Now().After(policy.NextExecution)
}

func (rs *RetentionService) executePolicy(ctx context.Context, policy *models.RetentionPolicy) (*models.RetentionPolicyExecution, error) {
	startTime := time.Now()

	// Create execution record
	execution := &models.RetentionPolicyExecution{
		PolicyID:      policy.ID,
		ExecutionTime: startTime,
		Status:        models.ExecutionStatusRunning,
	}

	if err := rs.repos.RetentionExecution.Create(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// Update active jobs count
	rs.statsMu.Lock()
	rs.stats.ActiveJobs++
	rs.statsMu.Unlock()

	defer func() {
		rs.statsMu.Lock()
		rs.stats.ActiveJobs--
		rs.statsMu.Unlock()
	}()

	rs.logger.Info("Starting retention policy execution",
		logging.Int("policy_id", policy.ID),
		logging.String("policy_name", policy.Name))

	var totalDeleted int64
	var totalBytesFreed int64
	var executionError error

	// Execute each rule type
	if policy.TimeBasedRule != nil {
		deleted, bytesFreed, err := rs.executeTimeBasedRule(ctx, policy, policy.TimeBasedRule)
		if err != nil {
			executionError = fmt.Errorf("time-based rule failed: %w", err)
		} else {
			totalDeleted += deleted
			totalBytesFreed += bytesFreed
		}
	}

	if policy.SizeBasedRule != nil && executionError == nil {
		deleted, bytesFreed, err := rs.executeSizeBasedRule(ctx, policy, policy.SizeBasedRule)
		if err != nil {
			executionError = fmt.Errorf("size-based rule failed: %w", err)
		} else {
			totalDeleted += deleted
			totalBytesFreed += bytesFreed
		}
	}

	if policy.CountBasedRule != nil && executionError == nil {
		deleted, bytesFreed, err := rs.executeCountBasedRule(ctx, policy, policy.CountBasedRule)
		if err != nil {
			executionError = fmt.Errorf("count-based rule failed: %w", err)
		} else {
			totalDeleted += deleted
			totalBytesFreed += bytesFreed
		}
	}

	// Update execution record
	execution.Duration = time.Since(startTime)
	execution.EntriesDeleted = totalDeleted
	execution.BytesFreed = totalBytesFreed

	if executionError != nil {
		execution.Status = models.ExecutionStatusFailed
		execution.ErrorMessage = executionError.Error()
	} else {
		execution.Status = models.ExecutionStatusCompleted
	}

	// Set execution details
	details := map[string]interface{}{
		"policy_name":        policy.Name,
		"execution_duration": execution.Duration.String(),
		"dry_run_mode":       rs.config.DryRunMode,
	}
	if err := execution.SetDetailsMap(details); err != nil {
		rs.logger.Error("Failed to set execution details", logging.Err(err))
	}

	if err := rs.repos.RetentionExecution.Update(ctx, execution); err != nil {
		rs.logger.Error("Failed to update execution record", logging.Err(err))
	}

	// Update policy's next execution time
	rs.updatePolicyNextExecution(ctx, policy)

	// Update statistics
	rs.updateStats(policy.ID, execution, executionError == nil)

	if executionError != nil {
		return execution, executionError
	}

	rs.logger.Info("Completed retention policy execution",
		logging.Int("policy_id", policy.ID),
		logging.String("policy_name", policy.Name),
		logging.Int("entries_deleted", int(totalDeleted)),
		logging.Int("bytes_freed", int(totalBytesFreed)),
		logging.String("duration", execution.Duration.String()))

	return execution, nil
}

func (rs *RetentionService) executeTimeBasedRule(ctx context.Context, policy *models.RetentionPolicy, rule *models.TimeBasedRetention) (int64, int64, error) {
	cutoffTime := time.Now().Add(-rule.MaxAge)

	if rs.config.DryRunMode {
		// In dry run mode, just count what would be deleted
		count, err := rs.repos.AuditLog.CountByTimeRange(ctx, time.Time{}, cutoffTime)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to count logs for time-based rule: %w", err)
		}

		rs.logger.Info("Time-based rule dry run",
			logging.Int("policy_id", policy.ID),
			logging.String("cutoff_time", cutoffTime.Format(time.RFC3339)),
			logging.Int("would_delete", count))

		return int64(count), 0, nil // Assume 0 bytes freed in dry run
	}

	// Apply safety threshold
	totalCount, err := rs.repos.AuditLog.Count(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total log count: %w", err)
	}

	deleteCount, err := rs.repos.AuditLog.CountByTimeRange(ctx, time.Time{}, cutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count logs for deletion: %w", err)
	}

	if float64(deleteCount)/float64(totalCount) > rs.config.SafetyThreshold {
		return 0, 0, fmt.Errorf("safety threshold exceeded: would delete %d/%d logs (%.2f%%)",
			deleteCount, totalCount, float64(deleteCount)/float64(totalCount)*100)
	}

	// Perform the deletion
	err = rs.repos.AuditLog.CleanupOldLogs(ctx, cutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	return int64(deleteCount), 0, nil // TODO: Calculate actual bytes freed
}

func (rs *RetentionService) executeSizeBasedRule(ctx context.Context, policy *models.RetentionPolicy, rule *models.SizeBasedRetention) (int64, int64, error) {
	// This is a simplified implementation - in practice, you'd need to calculate actual sizes
	// For now, we'll use a heuristic based on entry count

	totalCount, err := rs.repos.AuditLog.Count(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total log count: %w", err)
	}

	// Estimate size (rough heuristic: 500 bytes per log entry)
	estimatedSize := int64(totalCount) * 500

	if estimatedSize <= rule.MaxTotalSize {
		return 0, 0, nil // No cleanup needed
	}

	// Calculate how many entries to delete
	excessSize := estimatedSize - rule.MaxTotalSize
	entriesToDelete := excessSize / 500

	if rs.config.DryRunMode {
		rs.logger.Info("Size-based rule dry run",
			logging.Int("policy_id", policy.ID),
			logging.Int("estimated_size", int(estimatedSize)),
			logging.Int("max_size", int(rule.MaxTotalSize)),
			logging.Int("would_delete", int(entriesToDelete)))

		return entriesToDelete, excessSize, nil
	}

	// For simplicity, delete oldest entries
	// In a real implementation, you'd implement the specific cleanup strategy
	cutoffTime := time.Now().AddDate(0, 0, -7) // Delete entries older than 7 days as a fallback
	err = rs.repos.AuditLog.CleanupOldLogs(ctx, cutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup logs for size rule: %w", err)
	}

	return entriesToDelete, excessSize, nil
}

func (rs *RetentionService) executeCountBasedRule(ctx context.Context, policy *models.RetentionPolicy, rule *models.CountBasedRetention) (int64, int64, error) {
	totalCount, err := rs.repos.AuditLog.Count(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get total log count: %w", err)
	}

	if int64(totalCount) <= rule.MaxCount {
		return 0, 0, nil // No cleanup needed
	}

	entriesToDelete := int64(totalCount) - rule.MaxCount

	if rs.config.DryRunMode {
		rs.logger.Info("Count-based rule dry run",
			logging.Int("policy_id", policy.ID),
			logging.Int("total_count", totalCount),
			logging.Int("max_count", int(rule.MaxCount)),
			logging.Int("would_delete", int(entriesToDelete)))

		return entriesToDelete, 0, nil
	}

	// For simplicity, delete oldest entries
	// In a real implementation, you'd implement the specific cleanup strategy
	cutoffTime := time.Now().AddDate(0, 0, -30) // Delete entries older than 30 days as a fallback
	err = rs.repos.AuditLog.CleanupOldLogs(ctx, cutoffTime)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup logs for count rule: %w", err)
	}

	return entriesToDelete, 0, nil
}

func (rs *RetentionService) previewPolicyExecution(ctx context.Context, policy *models.RetentionPolicy) (*RetentionPreview, error) {
	preview := &RetentionPreview{
		PolicyID:   policy.ID,
		PolicyName: policy.Name,
	}

	var totalEstimatedDeletions int64

	// Preview time-based rule
	if policy.TimeBasedRule != nil {
		cutoffTime := time.Now().Add(-policy.TimeBasedRule.MaxAge)
		count, err := rs.repos.AuditLog.CountByTimeRange(ctx, time.Time{}, cutoffTime)
		if err != nil {
			return nil, fmt.Errorf("failed to preview time-based rule: %w", err)
		}

		preview.RuleBreakdown = append(preview.RuleBreakdown, RulePreview{
			RuleType:           "time_based",
			EstimatedDeletions: int64(count),
			CutoffTime:         cutoffTime,
			Description:        fmt.Sprintf("Delete logs older than %s", policy.TimeBasedRule.MaxAge),
		})

		totalEstimatedDeletions += int64(count)
		preview.AffectedTimeRange.End = cutoffTime
	}

	// Preview size-based rule
	if policy.SizeBasedRule != nil {
		totalCount, err := rs.repos.AuditLog.Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get total count for size preview: %w", err)
		}

		estimatedSize := int64(totalCount) * 500 // Rough estimate
		if estimatedSize > policy.SizeBasedRule.MaxTotalSize {
			excessSize := estimatedSize - policy.SizeBasedRule.MaxTotalSize
			entriesToDelete := excessSize / 500

			preview.RuleBreakdown = append(preview.RuleBreakdown, RulePreview{
				RuleType:           "size_based",
				EstimatedDeletions: entriesToDelete,
				Description:        fmt.Sprintf("Delete %d entries to stay under %d bytes", entriesToDelete, policy.SizeBasedRule.MaxTotalSize),
			})

			totalEstimatedDeletions += entriesToDelete
			preview.EstimatedBytesFreed += excessSize
		}
	}

	// Preview count-based rule
	if policy.CountBasedRule != nil {
		totalCount, err := rs.repos.AuditLog.Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get total count for count preview: %w", err)
		}

		if int64(totalCount) > policy.CountBasedRule.MaxCount {
			entriesToDelete := int64(totalCount) - policy.CountBasedRule.MaxCount

			preview.RuleBreakdown = append(preview.RuleBreakdown, RulePreview{
				RuleType:           "count_based",
				EstimatedDeletions: entriesToDelete,
				Description:        fmt.Sprintf("Delete %d entries to stay under %d total", entriesToDelete, policy.CountBasedRule.MaxCount),
			})

			totalEstimatedDeletions += entriesToDelete
		}
	}

	preview.EstimatedDeletions = totalEstimatedDeletions

	// Add safety warnings
	if totalEstimatedDeletions > 0 {
		totalCount, _ := rs.repos.AuditLog.Count(ctx)
		deletionRatio := float64(totalEstimatedDeletions) / float64(totalCount)

		if deletionRatio > rs.config.SafetyThreshold {
			preview.SafetyWarnings = append(preview.SafetyWarnings,
				fmt.Sprintf("Warning: This policy would delete %.1f%% of all logs, exceeding the safety threshold of %.1f%%",
					deletionRatio*100, rs.config.SafetyThreshold*100))
		}

		if totalEstimatedDeletions > int64(rs.config.MaxDeleteBatchSize) {
			preview.SafetyWarnings = append(preview.SafetyWarnings,
				fmt.Sprintf("Warning: This policy would delete %d entries, exceeding the maximum batch size of %d",
					totalEstimatedDeletions, rs.config.MaxDeleteBatchSize))
		}
	}

	return preview, nil
}

func (rs *RetentionService) updatePolicyNextExecution(ctx context.Context, policy *models.RetentionPolicy) {
	// Simple implementation - add 24 hours to next execution
	// In a real implementation, you'd parse the cron expression
	policy.LastExecuted = time.Now()
	policy.NextExecution = time.Now().Add(24 * time.Hour)

	if err := rs.repos.RetentionPolicy.Update(ctx, policy); err != nil {
		rs.logger.Error("Failed to update policy execution times",
			logging.Int("policy_id", policy.ID),
			logging.Err(err))
	}
}

func (rs *RetentionService) updateStats(policyID int, execution *models.RetentionPolicyExecution, success bool) {
	rs.statsMu.Lock()
	defer rs.statsMu.Unlock()

	rs.stats.TotalExecutions++
	if success {
		rs.stats.SuccessfulExecutions++
	} else {
		rs.stats.FailedExecutions++
	}

	rs.stats.TotalEntriesDeleted += execution.EntriesDeleted
	rs.stats.TotalBytesFreed += execution.BytesFreed
	rs.stats.LastExecutionTime = execution.ExecutionTime

	// Update average execution time
	if rs.stats.AverageExecutionTime == 0 {
		rs.stats.AverageExecutionTime = execution.Duration
	} else {
		rs.stats.AverageExecutionTime = (rs.stats.AverageExecutionTime + execution.Duration) / 2
	}

	// Update policy-specific stats
	if rs.stats.PolicyStats[policyID] == nil {
		rs.stats.PolicyStats[policyID] = &PolicyStats{
			PolicyID: policyID,
		}
	}

	policyStats := rs.stats.PolicyStats[policyID]
	policyStats.ExecutionCount++
	policyStats.LastExecutionTime = execution.ExecutionTime
	policyStats.TotalEntriesDeleted += execution.EntriesDeleted

	// Update average execution time for policy
	if policyStats.AverageExecutionTime == 0 {
		policyStats.AverageExecutionTime = execution.Duration
	} else {
		policyStats.AverageExecutionTime = (policyStats.AverageExecutionTime + execution.Duration) / 2
	}

	// Update success rate for policy
	if success {
		policyStats.SuccessRate = (policyStats.SuccessRate*float64(policyStats.ExecutionCount-1) + 1.0) / float64(policyStats.ExecutionCount)
	} else {
		policyStats.SuccessRate = (policyStats.SuccessRate * float64(policyStats.ExecutionCount-1)) / float64(policyStats.ExecutionCount)
	}
}

// RetentionScheduler manages scheduling of retention policy executions
type RetentionScheduler struct {
	// This is a simplified scheduler - in practice, you'd use a proper cron library
}

// NewRetentionScheduler creates a new retention scheduler
func NewRetentionScheduler() *RetentionScheduler {
	return &RetentionScheduler{}
}
