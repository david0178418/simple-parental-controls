package service

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// LogRotationService manages log file rotation, compression, and archival
type LogRotationService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
	config LogRotationConfig

	// Service state
	running   bool
	runningMu sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// Disk space monitoring
	diskMonitor *DiskSpaceMonitor

	// Statistics
	stats   *models.RotationStats
	statsMu sync.RWMutex

	// File operation safety
	operationMu sync.Mutex
}

// LogRotationConfig holds configuration for the log rotation service
type LogRotationConfig struct {
	// Monitoring settings
	CheckInterval          time.Duration `json:"check_interval"`           // How often to check for rotation triggers
	DiskCheckInterval      time.Duration `json:"disk_check_interval"`      // How often to check disk space
	EmergencyCheckInterval time.Duration `json:"emergency_check_interval"` // How often to check during emergency

	// Safety settings
	MaxConcurrentRotations int     `json:"max_concurrent_rotations"` // Maximum concurrent rotation operations
	SafetyThreshold        float64 `json:"safety_threshold"`         // Safety threshold for bulk operations
	DryRunMode             bool    `json:"dry_run_mode"`             // If true, don't actually perform operations

	// Performance settings
	CompressionBuffer int   `json:"compression_buffer"` // Buffer size for compression operations
	IOBufferSize      int   `json:"io_buffer_size"`     // Buffer size for file I/O
	MaxArchiveSize    int64 `json:"max_archive_size"`   // Maximum size for individual archive files

	// File system settings
	TempDirectory    string `json:"temp_directory"`    // Directory for temporary files during operations
	ArchiveDirectory string `json:"archive_directory"` // Default archive directory
	BackupOriginals  bool   `json:"backup_originals"`  // Whether to backup original files before rotation

	// Monitoring and alerting
	EnableDiskMonitoring bool `json:"enable_disk_monitoring"` // Enable disk space monitoring
	EnableAlerting       bool `json:"enable_alerting"`        // Enable alerting for issues
}

// DefaultLogRotationConfig returns log rotation service configuration with sensible defaults
func DefaultLogRotationConfig() LogRotationConfig {
	return LogRotationConfig{
		CheckInterval:          5 * time.Minute,
		DiskCheckInterval:      1 * time.Minute,
		EmergencyCheckInterval: 30 * time.Second,
		MaxConcurrentRotations: 3,
		SafetyThreshold:        0.8,
		DryRunMode:             false,
		CompressionBuffer:      64 * 1024,         // 64KB
		IOBufferSize:           32 * 1024,         // 32KB
		MaxArchiveSize:         100 * 1024 * 1024, // 100MB
		TempDirectory:          "data/temp",
		ArchiveDirectory:       "data/archives",
		BackupOriginals:        true,
		EnableDiskMonitoring:   true,
		EnableAlerting:         true,
	}
}

// DiskSpaceMonitor monitors disk space and triggers emergency cleanup
type DiskSpaceMonitor struct {
	config          LogRotationConfig
	logger          logging.Logger
	lastCheck       time.Time
	emergencyActive bool
	emergencyMu     sync.RWMutex
}

// FileRotationResult represents the result of a file rotation operation
type FileRotationResult struct {
	Files            []models.FileRotationInfo `json:"files"`
	TotalFiles       int                       `json:"total_files"`
	TotalBytesFreed  int64                     `json:"total_bytes_freed"`
	TotalCompressed  int64                     `json:"total_compressed"`
	CompressionRatio float64                   `json:"compression_ratio"`
	Duration         time.Duration             `json:"duration"`
	Errors           []string                  `json:"errors"`
}

// NewLogRotationService creates a new log rotation service
func NewLogRotationService(repos *models.RepositoryManager, logger logging.Logger, config LogRotationConfig) *LogRotationService {
	service := &LogRotationService{
		repos:  repos,
		logger: logger,
		config: config,
		stopCh: make(chan struct{}),
		stats: &models.RotationStats{
			PolicyStats: make(map[int]*models.PolicyRotationStats),
		},
	}

	// Initialize disk monitor
	if config.EnableDiskMonitoring {
		service.diskMonitor = &DiskSpaceMonitor{
			config: config,
			logger: logger,
		}
	}

	// Ensure required directories exist
	service.ensureDirectories()

	return service
}

// Start starts the log rotation service
func (s *LogRotationService) Start(ctx context.Context) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return fmt.Errorf("log rotation service is already running")
	}

	s.logger.Info("Starting log rotation service")

	// Start main rotation loop
	s.wg.Add(1)
	go s.rotationLoop(ctx)

	// Start disk monitoring if enabled
	if s.config.EnableDiskMonitoring {
		s.wg.Add(1)
		go s.diskMonitorLoop(ctx)
	}

	s.running = true
	s.logger.Info("Log rotation service started successfully")
	return nil
}

// Stop stops the log rotation service
func (s *LogRotationService) Stop() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping log rotation service")

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	s.logger.Info("Log rotation service stopped")
	return nil
}

// ExecutePolicy manually executes a specific rotation policy
func (s *LogRotationService) ExecutePolicy(ctx context.Context, policyID int) (*models.LogRotationExecution, error) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()

	policy, err := s.repos.LogRotationPolicy.GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rotation policy: %w", err)
	}

	if !policy.Enabled {
		return nil, fmt.Errorf("rotation policy %d is disabled", policyID)
	}

	return s.executePolicy(ctx, policy, models.TriggerManual)
}

// ExecuteAllPolicies manually executes all enabled rotation policies
func (s *LogRotationService) ExecuteAllPolicies(ctx context.Context) ([]*models.LogRotationExecution, error) {
	policies, err := s.repos.LogRotationPolicy.GetByPriority(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled policies: %w", err)
	}

	var executions []*models.LogRotationExecution
	for _, policy := range policies {
		execution, err := s.executePolicy(ctx, &policy, models.TriggerManual)
		if err != nil {
			s.logger.Error("Failed to execute rotation policy",
				logging.Int("policy_id", policy.ID),
				logging.String("policy_name", policy.Name),
				logging.Err(err))
			continue
		}
		executions = append(executions, execution)
	}

	return executions, nil
}

// GetStats returns rotation service statistics
func (s *LogRotationService) GetStats() *models.RotationStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// Get fresh stats from database
	dbStats, err := s.repos.LogRotationExecution.GetStats(context.Background())
	if err != nil {
		// Return cached stats if database query fails
		return s.stats
	}

	// Add disk space information
	if s.config.EnableDiskMonitoring {
		dbStats.DiskSpaceInfo = s.getCurrentDiskSpace()
	}

	return dbStats
}

// GetDiskSpaceInfo returns current disk space information
func (s *LogRotationService) GetDiskSpaceInfo() *models.DiskSpaceInfo {
	return s.getCurrentDiskSpace()
}

// TriggerEmergencyCleanup manually triggers emergency cleanup
func (s *LogRotationService) TriggerEmergencyCleanup(ctx context.Context) error {
	s.logger.Warn("Emergency cleanup triggered manually")
	return s.performEmergencyCleanup(ctx)
}

// Private methods

func (s *LogRotationService) rotationLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndExecutePolicies(ctx)
		}
	}
}

func (s *LogRotationService) diskMonitorLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.DiskCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkDiskSpace(ctx)
		}
	}
}

func (s *LogRotationService) checkAndExecutePolicies(ctx context.Context) {
	policies, err := s.repos.LogRotationPolicy.GetByPriority(ctx)
	if err != nil {
		s.logger.Error("Failed to get rotation policies", logging.Err(err))
		return
	}

	for _, policy := range policies {
		if s.shouldExecutePolicy(&policy) {
			trigger := s.determineTriggerReason(&policy)

			// Execute policy in a separate goroutine to avoid blocking
			go func(p models.LogRotationPolicy, t models.RotationTrigger) {
				if _, err := s.executePolicy(ctx, &p, t); err != nil {
					s.logger.Error("Failed to execute scheduled rotation policy",
						logging.Int("policy_id", p.ID),
						logging.String("policy_name", p.Name),
						logging.String("trigger", string(t)),
						logging.Err(err))
				}
			}(policy, trigger)
		}
	}
}

func (s *LogRotationService) checkDiskSpace(ctx context.Context) {
	if s.diskMonitor == nil {
		return
	}

	diskInfo := s.getCurrentDiskSpace()
	if diskInfo == nil {
		return
	}

	s.diskMonitor.emergencyMu.RLock()
	emergencyActive := s.diskMonitor.emergencyActive
	s.diskMonitor.emergencyMu.RUnlock()

	// Check if we need to trigger emergency cleanup
	if diskInfo.UsagePercent >= 0.85 { // 85% threshold for emergency
		if !emergencyActive {
			s.logger.Warn("High disk usage detected, triggering emergency cleanup",
				logging.Field{Key: "usage_percent", Value: diskInfo.UsagePercent})

			s.diskMonitor.emergencyMu.Lock()
			s.diskMonitor.emergencyActive = true
			s.diskMonitor.emergencyMu.Unlock()

			go func() {
				if err := s.performEmergencyCleanup(ctx); err != nil {
					s.logger.Error("Emergency cleanup failed", logging.Err(err))
				}

				s.diskMonitor.emergencyMu.Lock()
				s.diskMonitor.emergencyActive = false
				s.diskMonitor.emergencyMu.Unlock()
			}()
		}
	}

	s.diskMonitor.lastCheck = time.Now()
}

func (s *LogRotationService) shouldExecutePolicy(policy *models.LogRotationPolicy) bool {
	// Check if it's time for scheduled execution
	if !policy.NextExecution.IsZero() && time.Now().After(policy.NextExecution) {
		return true
	}

	// Check size-based triggers
	if policy.SizeBasedRotation != nil {
		if s.checkSizeBasedTrigger(policy) {
			return true
		}
	}

	// Check time-based triggers
	if policy.TimeBasedRotation != nil {
		if s.checkTimeBasedTrigger(policy) {
			return true
		}
	}

	return false
}

func (s *LogRotationService) determineTriggerReason(policy *models.LogRotationPolicy) models.RotationTrigger {
	// Check emergency conditions first
	diskInfo := s.getCurrentDiskSpace()
	if diskInfo != nil && diskInfo.UsagePercent >= 0.9 {
		return models.TriggerEmergency
	}

	// Check size-based triggers
	if policy.SizeBasedRotation != nil && s.checkSizeBasedTrigger(policy) {
		return models.TriggerSize
	}

	// Check time-based triggers
	if policy.TimeBasedRotation != nil && s.checkTimeBasedTrigger(policy) {
		return models.TriggerTime
	}

	// Default to scheduled
	return models.TriggerScheduled
}

func (s *LogRotationService) checkSizeBasedTrigger(policy *models.LogRotationPolicy) bool {
	if policy.SizeBasedRotation == nil {
		return false
	}

	// Get total size of target files
	totalSize, err := s.calculateTargetFilesSize(policy.TargetLogFiles)
	if err != nil {
		s.logger.Error("Failed to calculate target files size", logging.Err(err))
		return false
	}

	return totalSize >= policy.SizeBasedRotation.MaxTotalSize
}

func (s *LogRotationService) checkTimeBasedTrigger(policy *models.LogRotationPolicy) bool {
	if policy.TimeBasedRotation == nil {
		return false
	}

	// Check if enough time has passed since last execution
	if policy.LastExecuted.IsZero() {
		return true // Never executed before
	}

	timeSinceLastExecution := time.Since(policy.LastExecuted)
	return timeSinceLastExecution >= policy.TimeBasedRotation.RotationInterval
}

func (s *LogRotationService) executePolicy(ctx context.Context, policy *models.LogRotationPolicy, trigger models.RotationTrigger) (*models.LogRotationExecution, error) {
	startTime := time.Now()

	// Create execution record
	execution := &models.LogRotationExecution{
		PolicyID:      policy.ID,
		ExecutionTime: startTime,
		Status:        models.ExecutionStatusRunning,
		TriggerReason: trigger,
	}

	if err := s.repos.LogRotationExecution.Create(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	s.logger.Info("Starting log rotation policy execution",
		logging.Int("policy_id", policy.ID),
		logging.String("policy_name", policy.Name),
		logging.String("trigger", string(trigger)))

	// Get target files
	targetFiles, err := s.findTargetFiles(policy.TargetLogFiles)
	if err != nil {
		s.updateExecutionError(ctx, execution, fmt.Errorf("failed to find target files: %w", err))
		return execution, err
	}

	if len(targetFiles) == 0 {
		s.logger.Info("No target files found for rotation",
			logging.Int("policy_id", policy.ID))
		execution.Status = models.ExecutionStatusCompleted
		execution.Duration = time.Since(startTime)
		s.repos.LogRotationExecution.Update(ctx, execution)
		return execution, nil
	}

	// Perform rotation
	result, err := s.rotateFiles(ctx, policy, targetFiles)
	if err != nil {
		s.updateExecutionError(ctx, execution, fmt.Errorf("rotation failed: %w", err))
		return execution, err
	}

	// Update execution record with results
	execution.Status = models.ExecutionStatusCompleted
	execution.Duration = time.Since(startTime)
	execution.FilesRotated = result.TotalFiles
	execution.BytesFreed = result.TotalBytesFreed
	execution.BytesCompressed = result.TotalCompressed
	execution.CompressionRatio = result.CompressionRatio

	// Set execution details
	details := map[string]interface{}{
		"policy_name":     policy.Name,
		"trigger_reason":  string(trigger),
		"target_patterns": policy.TargetLogFiles,
		"files_processed": result.Files,
		"dry_run_mode":    s.config.DryRunMode,
	}
	if err := execution.SetDetailsMap(details); err != nil {
		s.logger.Error("Failed to set execution details", logging.Err(err))
	}

	if err := s.repos.LogRotationExecution.Update(ctx, execution); err != nil {
		s.logger.Error("Failed to update execution record", logging.Err(err))
	}

	// Update policy's execution time
	s.updatePolicyNextExecution(ctx, policy)

	// Update statistics
	s.updateStats(policy.ID, execution, true)

	s.logger.Info("Completed log rotation policy execution",
		logging.Int("policy_id", policy.ID),
		logging.String("policy_name", policy.Name),
		logging.Int("files_rotated", execution.FilesRotated),
		logging.Int("bytes_freed", int(execution.BytesFreed)),
		logging.String("duration", execution.Duration.String()))

	return execution, nil
}

func (s *LogRotationService) rotateFiles(ctx context.Context, policy *models.LogRotationPolicy, files []string) (*FileRotationResult, error) {
	result := &FileRotationResult{
		Files: make([]models.FileRotationInfo, 0, len(files)),
	}
	startTime := time.Now()

	for _, file := range files {
		fileInfo, err := s.rotateFile(ctx, policy, file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file, err))
			s.logger.Error("Failed to rotate file",
				logging.String("file", file),
				logging.Err(err))
			continue
		}

		if fileInfo != nil {
			result.Files = append(result.Files, *fileInfo)
			result.TotalFiles++
			result.TotalBytesFreed += fileInfo.OriginalSize
			if fileInfo.CompressedSize > 0 {
				result.TotalCompressed += fileInfo.CompressedSize
			}
		}
	}

	result.Duration = time.Since(startTime)

	// Calculate compression ratio
	if result.TotalCompressed > 0 && result.TotalBytesFreed > 0 {
		result.CompressionRatio = float64(result.TotalCompressed) / float64(result.TotalBytesFreed)
	}

	return result, nil
}

func (s *LogRotationService) rotateFile(ctx context.Context, policy *models.LogRotationPolicy, filePath string) (*models.FileRotationInfo, error) {
	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, skip
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check size-based rotation criteria
	if policy.SizeBasedRotation != nil {
		if fileInfo.Size() < policy.SizeBasedRotation.MaxFileSize {
			return nil, nil // File not large enough to rotate
		}
	}

	// Check time-based rotation criteria
	if policy.TimeBasedRotation != nil {
		if time.Since(fileInfo.ModTime()) < policy.TimeBasedRotation.RotationInterval {
			return nil, nil // File not old enough to rotate
		}
	}

	rotationInfo := &models.FileRotationInfo{
		OriginalPath: filePath,
		OriginalSize: fileInfo.Size(),
		RotatedAt:    time.Now(),
	}

	// Generate rotated file name
	rotatedPath := s.generateRotatedFileName(filePath)
	rotationInfo.RotatedPath = rotatedPath

	// Calculate checksum before rotation
	if checksum, err := s.calculateFileChecksum(filePath); err == nil {
		rotationInfo.Checksum = checksum
	}

	if s.config.DryRunMode {
		s.logger.Info("Dry run: would rotate file",
			logging.String("original", filePath),
			logging.String("rotated", rotatedPath))
		return rotationInfo, nil
	}

	// Backup original if configured
	if s.config.BackupOriginals {
		backupPath := filePath + ".backup"
		if err := s.copyFile(filePath, backupPath); err != nil {
			s.logger.Warn("Failed to create backup", logging.Err(err))
		}
	}

	// Move the file to rotated name
	if err := os.Rename(filePath, rotatedPath); err != nil {
		return nil, fmt.Errorf("failed to rotate file: %w", err)
	}

	// Create new empty file if needed
	if _, err := os.Create(filePath); err != nil {
		s.logger.Warn("Failed to create new log file",
			logging.String("file", filePath),
			logging.Err(err))
	}

	// Handle archival and compression
	if policy.ArchivalPolicy != nil {
		if err := s.archiveFile(ctx, policy.ArchivalPolicy, rotationInfo); err != nil {
			s.logger.Error("Failed to archive file", logging.Err(err))
			// Don't fail the entire operation if archival fails
		}
	}

	return rotationInfo, nil
}

func (s *LogRotationService) archiveFile(ctx context.Context, archivalPolicy *models.ArchivalPolicy, rotationInfo *models.FileRotationInfo) error {
	if !archivalPolicy.EnableCompression {
		return nil // No compression requested
	}

	// Ensure archive directory exists
	if err := os.MkdirAll(archivalPolicy.ArchiveLocation, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Generate archive file name
	archiveName := filepath.Base(rotationInfo.RotatedPath) + ".gz"
	archivePath := filepath.Join(archivalPolicy.ArchiveLocation, archiveName)
	rotationInfo.ArchivePath = archivePath

	if s.config.DryRunMode {
		s.logger.Info("Dry run: would compress file",
			logging.String("source", rotationInfo.RotatedPath),
			logging.String("archive", archivePath))
		return nil
	}

	// Compress the file
	compressedSize, err := s.compressFile(rotationInfo.RotatedPath, archivePath, archivalPolicy.CompressionLevel)
	if err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}

	rotationInfo.CompressedSize = compressedSize
	if rotationInfo.OriginalSize > 0 {
		rotationInfo.CompressionRatio = float64(compressedSize) / float64(rotationInfo.OriginalSize)
	}

	// Remove the rotated file after successful compression
	if err := os.Remove(rotationInfo.RotatedPath); err != nil {
		s.logger.Warn("Failed to remove rotated file after compression",
			logging.String("file", rotationInfo.RotatedPath),
			logging.Err(err))
	}

	return nil
}

func (s *LogRotationService) compressFile(srcPath, dstPath string, compressionLevel int) (int64, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Create gzip writer with specified compression level
	gzipWriter, err := gzip.NewWriterLevel(dstFile, compressionLevel)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	// Copy data with compression
	buffer := make([]byte, s.config.IOBufferSize)
	if _, err := io.CopyBuffer(gzipWriter, srcFile, buffer); err != nil {
		return 0, fmt.Errorf("failed to compress file: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return 0, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Get compressed file size
	fileInfo, err := dstFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get compressed file info: %w", err)
	}

	return fileInfo.Size(), nil
}

func (s *LogRotationService) performEmergencyCleanup(ctx context.Context) error {
	s.logger.Warn("Performing emergency cleanup")

	// Get emergency policies
	policies, err := s.repos.LogRotationPolicy.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to get emergency policies: %w", err)
	}

	var emergencyPolicies []models.LogRotationPolicy
	for _, policy := range policies {
		if policy.EmergencyConfig != nil {
			emergencyPolicies = append(emergencyPolicies, policy)
		}
	}

	if len(emergencyPolicies) == 0 {
		s.logger.Warn("No emergency policies configured")
		return nil
	}

	// Sort by priority (highest first)
	sort.Slice(emergencyPolicies, func(i, j int) bool {
		return emergencyPolicies[i].Priority > emergencyPolicies[j].Priority
	})

	// Execute emergency policies
	for _, policy := range emergencyPolicies {
		if _, err := s.executePolicy(ctx, &policy, models.TriggerEmergency); err != nil {
			s.logger.Error("Emergency policy execution failed",
				logging.Int("policy_id", policy.ID),
				logging.Err(err))
			continue
		}

		// Check if disk space is now under control
		diskInfo := s.getCurrentDiskSpace()
		if diskInfo != nil && diskInfo.UsagePercent < 0.8 {
			s.logger.Info("Emergency cleanup successful - disk usage under control",
				logging.Field{Key: "usage_percent", Value: diskInfo.UsagePercent})
			break
		}
	}

	return nil
}

// Utility methods

func (s *LogRotationService) findTargetFiles(patterns []string) ([]string, error) {
	var files []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to match pattern %s: %w", pattern, err)
		}

		for _, match := range matches {
			// Check if it's a regular file
			if fileInfo, err := os.Stat(match); err == nil && fileInfo.Mode().IsRegular() {
				files = append(files, match)
			}
		}
	}

	return files, nil
}

func (s *LogRotationService) calculateTargetFilesSize(patterns []string) (int64, error) {
	files, err := s.findTargetFiles(patterns)
	if err != nil {
		return 0, err
	}

	var totalSize int64
	for _, file := range files {
		if fileInfo, err := os.Stat(file); err == nil {
			totalSize += fileInfo.Size()
		}
	}

	return totalSize, nil
}

func (s *LogRotationService) generateRotatedFileName(originalPath string) string {
	dir := filepath.Dir(originalPath)
	name := filepath.Base(originalPath)
	timestamp := time.Now().Format("20060102-150405")

	return filepath.Join(dir, fmt.Sprintf("%s.%s", name, timestamp))
}

func (s *LogRotationService) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (s *LogRotationService) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	buffer := make([]byte, s.config.IOBufferSize)
	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	return err
}

func (s *LogRotationService) getCurrentDiskSpace() *models.DiskSpaceInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		s.logger.Error("Failed to get disk space info", logging.Err(err))
		return nil
	}

	totalSpace := int64(stat.Blocks) * int64(stat.Bsize)
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
	usedSpace := totalSpace - freeSpace
	usagePercent := float64(usedSpace) / float64(totalSpace)

	return &models.DiskSpaceInfo{
		TotalSpace:   totalSpace,
		UsedSpace:    usedSpace,
		FreeSpace:    freeSpace,
		UsagePercent: usagePercent,
		LastUpdated:  time.Now(),
	}
}

func (s *LogRotationService) ensureDirectories() {
	directories := []string{
		s.config.TempDirectory,
		s.config.ArchiveDirectory,
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			s.logger.Error("Failed to create directory",
				logging.String("directory", dir),
				logging.Err(err))
		}
	}
}

func (s *LogRotationService) updateExecutionError(ctx context.Context, execution *models.LogRotationExecution, err error) {
	execution.Status = models.ExecutionStatusFailed
	execution.ErrorMessage = err.Error()
	execution.Duration = time.Since(execution.ExecutionTime)

	if updateErr := s.repos.LogRotationExecution.Update(ctx, execution); updateErr != nil {
		s.logger.Error("Failed to update execution error", logging.Err(updateErr))
	}

	s.updateStats(execution.PolicyID, execution, false)
}

func (s *LogRotationService) updatePolicyNextExecution(ctx context.Context, policy *models.LogRotationPolicy) {
	policy.LastExecuted = time.Now()

	// Simple implementation - add 24 hours for daily rotation
	// In a real implementation, you'd parse the cron expression
	policy.NextExecution = time.Now().Add(24 * time.Hour)

	if err := s.repos.LogRotationPolicy.Update(ctx, policy); err != nil {
		s.logger.Error("Failed to update policy execution times",
			logging.Int("policy_id", policy.ID),
			logging.Err(err))
	}
}

func (s *LogRotationService) updateStats(policyID int, execution *models.LogRotationExecution, success bool) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	// Update global stats
	if success {
		s.stats.TotalFilesRotated += int64(execution.FilesRotated)
		s.stats.TotalBytesFreed += execution.BytesFreed
		s.stats.TotalBytesCompressed += execution.BytesCompressed
	}

	s.stats.LastRotationTime = execution.ExecutionTime

	// Update policy-specific stats
	if s.stats.PolicyStats[policyID] == nil {
		s.stats.PolicyStats[policyID] = &models.PolicyRotationStats{
			PolicyID: policyID,
		}
	}

	policyStats := s.stats.PolicyStats[policyID]
	policyStats.ExecutionCount++
	policyStats.LastExecutionTime = execution.ExecutionTime
	if success {
		policyStats.TotalFilesRotated += int64(execution.FilesRotated)
		policyStats.TotalBytesFreed += execution.BytesFreed
	}

	// Update average execution time
	if policyStats.AverageExecutionTime == 0 {
		policyStats.AverageExecutionTime = execution.Duration
	} else {
		policyStats.AverageExecutionTime = (policyStats.AverageExecutionTime + execution.Duration) / 2
	}

	// Update success rate
	if success {
		successCount := float64(policyStats.ExecutionCount) * policyStats.SuccessRate
		successCount++
		policyStats.SuccessRate = successCount / float64(policyStats.ExecutionCount)
	} else {
		successCount := float64(policyStats.ExecutionCount) * policyStats.SuccessRate
		policyStats.SuccessRate = successCount / float64(policyStats.ExecutionCount)
	}
}
