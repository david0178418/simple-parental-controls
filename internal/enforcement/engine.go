package enforcement

import (
	"context"
	"fmt"
	"sync"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// EnforcementEngine coordinates process monitoring and network filtering
type EnforcementEngine struct {
	// Core components
	processMonitor ProcessMonitor
	dnsBlocker     *DNSBlocker
	identifier     *ProcessIdentifier

	// Audit logging
	auditService AuditLogger

	// Configuration
	config *EnforcementConfig

	// State management
	running   bool
	runningMu sync.RWMutex

	// Event handling
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Logging
	logger logging.Logger

	// Statistics and monitoring
	stats   *EnforcementStats
	statsMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	rules map[string]*FilterRule
}

// EnforcementConfig holds configuration for the enforcement engine
type EnforcementConfig struct {
	// Process monitoring interval
	ProcessPollInterval time.Duration `json:"process_poll_interval"`

	// Network filtering settings
	EnableNetworkFiltering bool `json:"enable_network_filtering"`

	// Performance settings
	MaxConcurrentChecks int           `json:"max_concurrent_checks"`
	CacheTimeout        time.Duration `json:"cache_timeout"`

	// Enforcement actions
	BlockUnknownProcesses bool `json:"block_unknown_processes"`
	LogAllActivity        bool `json:"log_all_activity"`

	// Emergency settings
	EnableEmergencyMode bool     `json:"enable_emergency_mode"`
	EmergencyWhitelist  []string `json:"emergency_whitelist"`
}

// EnforcementStats holds statistics about enforcement activities
type EnforcementStats struct {
	// Process monitoring stats
	ProcessesMonitored int64 `json:"processes_monitored"`
	ProcessStartEvents int64 `json:"process_start_events"`
	ProcessStopEvents  int64 `json:"process_stop_events"`

	// Network filtering stats
	NetworkRequestsTotal   int64 `json:"network_requests_total"`
	NetworkRequestsBlocked int64 `json:"network_requests_blocked"`
	NetworkRequestsAllowed int64 `json:"network_requests_allowed"`

	// Enforcement actions
	EnforcementActions int64 `json:"enforcement_actions"`
	RuleViolations     int64 `json:"rule_violations"`

	// Performance metrics
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastEnforcementTime time.Time     `json:"last_enforcement_time"`

	// Error tracking
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`
}

// NewEnforcementEngine creates a new enforcement engine
func NewEnforcementEngine(config *EnforcementConfig, logger logging.Logger, auditService AuditLogger) *EnforcementEngine {
	ctx, cancel := context.WithCancel(context.Background())

	if config.ProcessPollInterval == 0 {
		config.ProcessPollInterval = 5 * time.Second
	}

	dnsBlockerConfig := &DNSBlockerConfig{
		ListenAddr:    ":53",
		BlockIPv4:     "0.0.0.0", 
		BlockIPv6:     "::",
		UpstreamDNS:   []string{"8.8.8.8:53", "1.1.1.1:53"},
		CacheTTL:      300 * time.Second,
		EnableLogging: config.LogAllActivity,
	}
	dnsBlocker, err := NewDNSBlocker(dnsBlockerConfig, logger)
	if err != nil {
		// In a real application, we might handle this more gracefully
		panic(fmt.Sprintf("failed to create dns blocker: %v", err))
	}

	return &EnforcementEngine{
		config:         config,
		logger:         logger,
		auditService:   auditService,
		processMonitor: NewLinuxProcessMonitor(config.ProcessPollInterval),
		dnsBlocker:     dnsBlocker,
		identifier:     NewProcessIdentifier(),
		rules:          make(map[string]*FilterRule),
		stats:          &EnforcementStats{},
		ctx:            ctx,
		cancel:         cancel,
		stopCh:         make(chan struct{}),
	}
}

// Start starts the enforcement engine and its components
func (ee *EnforcementEngine) Start(ctx context.Context) error {
	ee.runningMu.Lock()
	defer ee.runningMu.Unlock()

	if ee.running {
		return fmt.Errorf("enforcement engine is already running")
	}

	ee.logger.Info("Starting enforcement engine")

	// Start process monitoring
	if err := ee.processMonitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start process monitor: %w", err)
	}

	// Start dns blocker
	if err := ee.dnsBlocker.Start(ctx); err != nil {
		ee.processMonitor.Stop()
		return fmt.Errorf("failed to start dns blocker: %w", err)
	}

	ee.running = true

	// Start event processing goroutines
	ee.wg.Add(2)
	go ee.processEventHandler(ctx)
	go ee.statsUpdateLoop(ctx)

	ee.logger.Info("Enforcement engine started successfully")
	return nil
}

// Stop stops the enforcement engine gracefully
func (ee *EnforcementEngine) Stop(ctx context.Context) error {
	ee.runningMu.Lock()
	defer ee.runningMu.Unlock()

	if !ee.running {
		return nil
	}

	ee.logger.Info("Stopping enforcement engine")

	var shutdownErrors []error

	// Signal all goroutines to stop
	if ee.stopCh != nil {
		close(ee.stopCh)
	}
	if ee.cancel != nil {
		ee.cancel()
	}

	// Stop DNS blocker first to clean up network rules
	if ee.dnsBlocker != nil {
		if err := ee.dnsBlocker.Stop(ctx); err != nil {
			ee.logger.Error("Error stopping DNS blocker", logging.Err(err))
			shutdownErrors = append(shutdownErrors, fmt.Errorf("DNS blocker shutdown failed: %w", err))
		}
	}

	// Stop process monitor
	if ee.processMonitor != nil {
		if err := ee.processMonitor.Stop(); err != nil {
			ee.logger.Error("Error stopping process monitor", logging.Err(err))
			shutdownErrors = append(shutdownErrors, fmt.Errorf("process monitor shutdown failed: %w", err))
		}
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		ee.wg.Wait()
	}()

	select {
	case <-done:
		ee.logger.Info("All enforcement engine components stopped")
	case <-ctx.Done():
		err := fmt.Errorf("enforcement engine shutdown timed out")
		ee.logger.Error("Shutdown timeout", logging.Err(err))
		shutdownErrors = append(shutdownErrors, err)
	}

	ee.running = false

	// Return combined error if any occurred
	if len(shutdownErrors) > 0 {
		return fmt.Errorf("enforcement engine shutdown completed with errors: %v", shutdownErrors)
	}

	return nil
}

// IsRunning returns true if the engine is running
func (ee *EnforcementEngine) IsRunning() bool {
	ee.runningMu.RLock()
	defer ee.runningMu.RUnlock()
	return ee.running
}

// AddProcessSignature adds a process signature for identification
func (ee *EnforcementEngine) AddProcessSignature(signature *ProcessSignature) {
	ee.identifier.AddSignature(signature)
	ee.logger.Debug("Added process signature", logging.String("name", signature.Name))
}

// AddNetworkRule adds a network filtering rule
func (ee *EnforcementEngine) AddNetworkRule(rule *FilterRule) error {
	if ee.dnsBlocker == nil {
		return fmt.Errorf("dns blocker not enabled")
	}

	if err := ee.dnsBlocker.AddRule(rule); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to add network rule: %w", err))
		return err
	}

	ee.logger.Info("Added network rule", logging.String("name", rule.Name), logging.String("action", string(rule.Action)))
	return nil
}

// RemoveNetworkRule removes a network filtering rule
func (ee *EnforcementEngine) RemoveNetworkRule(ruleID string) error {
	if ee.dnsBlocker == nil {
		return fmt.Errorf("dns blocker not enabled")
	}

	// Note: DNSBlocker uses pattern, but engine uses ID. This is a simplification.
	if err := ee.dnsBlocker.RemoveRule(ruleID); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to remove network rule: %w", err))
		return err
	}

	ee.logger.Info("Removed network rule", logging.String("rule_id", ruleID))
	return nil
}

// GetCurrentRules returns all currently active rules from the DNS blocker
func (ee *EnforcementEngine) GetCurrentRules() map[string]*FilterRule {
	if ee.dnsBlocker == nil {
		return make(map[string]*FilterRule)
	}
	return ee.dnsBlocker.GetAllRules()
}

// ClearAllRules removes all rules from the enforcement engine
func (ee *EnforcementEngine) ClearAllRules() error {
	if ee.dnsBlocker == nil {
		return fmt.Errorf("dns blocker not enabled")
	}
	
	ee.dnsBlocker.ClearAllRules()
	ee.logger.Info("Cleared all enforcement rules")
	return nil
}

// GetProcesses returns all currently running processes
func (ee *EnforcementEngine) GetProcesses(ctx context.Context) ([]*ProcessInfo, error) {
	if ee.processMonitor == nil {
		return nil, fmt.Errorf("process monitor not available")
	}
	return ee.processMonitor.GetProcesses(ctx)
}

// GetProcess returns information about a specific process
func (ee *EnforcementEngine) GetProcess(ctx context.Context, pid int) (*ProcessInfo, error) {
	if ee.processMonitor == nil {
		return nil, fmt.Errorf("process monitor not available")
	}
	return ee.processMonitor.GetProcess(ctx, pid)
}

// KillProcess terminates a process by PID
func (ee *EnforcementEngine) KillProcess(ctx context.Context, pid int, graceful bool) error {
	if ee.processMonitor == nil {
		return fmt.Errorf("process monitor not available")
	}
	
	ee.logger.Info("Terminating process", 
		logging.Int("pid", pid), 
		logging.Bool("graceful", graceful))
	
	if err := ee.processMonitor.KillProcess(ctx, pid, graceful); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to kill process: %w", err))
		return err
	}
	
	ee.logger.Info("Process terminated successfully", logging.Int("pid", pid))
	return nil
}

// KillProcessByName terminates all processes matching a name pattern
func (ee *EnforcementEngine) KillProcessByName(ctx context.Context, namePattern string, graceful bool) error {
	if ee.processMonitor == nil {
		return fmt.Errorf("process monitor not available")
	}
	
	ee.logger.Info("Terminating processes by name", 
		logging.String("pattern", namePattern), 
		logging.Bool("graceful", graceful))
	
	if err := ee.processMonitor.KillProcessByName(ctx, namePattern, graceful); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to kill processes by name: %w", err))
		return err
	}
	
	ee.logger.Info("Processes terminated successfully", logging.String("pattern", namePattern))
	return nil
}

// IsProcessRunning checks if a process is currently running
func (ee *EnforcementEngine) IsProcessRunning(ctx context.Context, pid int) bool {
	if ee.processMonitor == nil {
		return false
	}
	return ee.processMonitor.IsProcessRunning(ctx, pid)
}

// EvaluateNetworkRequest evaluates a network request for enforcement
func (ee *EnforcementEngine) EvaluateNetworkRequest(ctx context.Context, url string, processInfo *ProcessInfo) (*FilterDecision, error) {
	// This function is now a stub, as DNS blocking handles this implicitly.
	// We can keep it for future use or for non-DNS based filtering.
	return &FilterDecision{
		Action:    ActionAllow,
		Reason:    "DNS blocking is the primary method",
		Timestamp: time.Now(),
		URL:       url,
	}, nil
}

// GetStats returns current enforcement statistics
func (ee *EnforcementEngine) GetStats() *EnforcementStats {
	ee.statsMu.RLock()
	defer ee.statsMu.RUnlock()

	// Create a copy to prevent race conditions
	stats := *ee.stats
	return &stats
}

// GetSystemInfo returns system information about enforcement components
func (ee *EnforcementEngine) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})
	info["running"] = ee.IsRunning()
	info["process_monitoring_enabled"] = ee.processMonitor != nil
	info["network_filtering_enabled"] = ee.dnsBlocker != nil
	info["config"] = ee.config

	return info
}

// processEventHandler handles process start/stop events
func (ee *EnforcementEngine) processEventHandler(ctx context.Context) {
	defer ee.wg.Done()

	// Subscribe to process events
	eventCh := ee.processMonitor.Subscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ee.stopCh:
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}

			ee.handleProcessEvent(ctx, event)
		}
	}
}

// handleProcessEvent processes a single process event
func (ee *EnforcementEngine) handleProcessEvent(ctx context.Context, event ProcessEvent) {
	ee.statsMu.Lock()
	switch event.Type {
	case ProcessStarted:
		ee.stats.ProcessStartEvents++
		ee.stats.ProcessesMonitored++
	case ProcessStopped:
		ee.stats.ProcessStopEvents++
		if ee.stats.ProcessesMonitored > 0 {
			ee.stats.ProcessesMonitored--
		}
	}
	ee.statsMu.Unlock()

	// Try to identify the process
	if signature, identified := ee.identifier.IdentifyProcess(event.Process); identified {
		ee.logger.Debug("Identified process", logging.String("process", event.Process.Name), logging.String("signature", signature.Name))

		// Apply any process-specific enforcement logic here
		ee.applyProcessEnforcement(ctx, event.Process, signature)
	} else if ee.config.LogAllActivity {
		ee.logger.Debug("Unknown process",
			logging.String("event_type", string(event.Type)),
			logging.String("process", event.Process.Name),
			logging.Int("pid", event.Process.PID))
	}
}

// applyProcessEnforcement applies enforcement rules specific to a process
func (ee *EnforcementEngine) applyProcessEnforcement(ctx context.Context, process *ProcessInfo, signature *ProcessSignature) {
	// This is where we would implement process-specific enforcement logic
	// For example, blocking certain processes, limiting their network access, etc.

	if ee.config.BlockUnknownProcesses {
		// Log process blocking action
		if ee.auditService != nil {
			details := map[string]interface{}{
				"process_name":      process.Name,
				"process_pid":       process.PID,
				"process_path":      process.Path,
				"signature_matched": signature.Name,
				"reason":            "blocked unknown process",
			}

			// Log asynchronously
			go func() {
				if err := ee.auditService.LogEnforcementAction(
					context.Background(),
					models.ActionTypeBlock,
					models.TargetTypeExecutable,
					process.Name,
					"process_control",
					nil, // No specific rule ID for process blocking
					details,
				); err != nil {
					ee.logger.Error("Failed to log process enforcement action", logging.Err(err))
				}
			}()
		}

		ee.logger.Warn("Would block unknown process", logging.String("process", process.Name))
	}

	ee.statsMu.Lock()
	ee.stats.EnforcementActions++
	ee.statsMu.Unlock()
}

// statsUpdateLoop periodically updates internal statistics
func (ee *EnforcementEngine) statsUpdateLoop(ctx context.Context) {
	defer ee.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ee.stopCh:
			return
		case <-ticker.C:
			ee.updateInternalStats()
		}
	}
}

// updateInternalStats updates internal statistics from components
func (ee *EnforcementEngine) updateInternalStats() {
	ee.statsMu.Lock()
	defer ee.statsMu.Unlock()

	// Update network filter stats if available
	if ee.dnsBlocker != nil {
		dnsStats := ee.dnsBlocker.GetStats()
		ee.stats.NetworkRequestsTotal = dnsStats.TotalQueries
		ee.stats.NetworkRequestsBlocked = dnsStats.BlockedQueries
		ee.stats.NetworkRequestsAllowed = dnsStats.AllowedQueries
	}

	// Process monitor stats are updated in real-time in handleProcessEvent
	// No additional process monitor statistics to update here

	ee.stats.LastEnforcementTime = time.Now()
}

// incrementErrorCount increments the error count and logs the error
func (ee *EnforcementEngine) incrementErrorCount(err error) {
	ee.statsMu.Lock()
	ee.stats.ErrorCount++
	ee.stats.LastError = err.Error()
	ee.stats.LastErrorTime = time.Now()
	ee.statsMu.Unlock()

	ee.logger.Error("Enforcement error", logging.Err(err))
}
