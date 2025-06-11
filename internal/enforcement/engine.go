package enforcement

import (
	"context"
	"fmt"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// EnforcementEngine coordinates process monitoring and network filtering
type EnforcementEngine struct {
	// Core components
	processMonitor     ProcessMonitor
	networkFilter      NetworkFilter
	trafficInterceptor TrafficInterceptor
	identifier         *ProcessIdentifier

	// Configuration
	config *EnforcementConfig

	// State management
	running   bool
	runningMu sync.RWMutex

	// Event handling
	processEvents chan ProcessEvent
	stopCh        chan struct{}
	wg            sync.WaitGroup

	// Logging
	logger logging.Logger

	// Statistics and monitoring
	stats      *EnforcementStats
	statsMu    sync.RWMutex
	lastUpdate time.Time
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
func NewEnforcementEngine(config *EnforcementConfig, logger logging.Logger) *EnforcementEngine {
	if config == nil {
		config = &EnforcementConfig{
			ProcessPollInterval:    1 * time.Second,
			EnableNetworkFiltering: true,
			MaxConcurrentChecks:    10,
			CacheTimeout:           5 * time.Minute,
			BlockUnknownProcesses:  false,
			LogAllActivity:         true,
			EnableEmergencyMode:    false,
		}
	}

	engine := &EnforcementEngine{
		config:        config,
		logger:        logger,
		identifier:    NewProcessIdentifier(),
		processEvents: make(chan ProcessEvent, 100),
		stopCh:        make(chan struct{}),
		stats:         &EnforcementStats{},
		lastUpdate:    time.Now(),
	}

	// Initialize components
	engine.processMonitor = NewProcessMonitor(config.ProcessPollInterval)
	engine.trafficInterceptor = NewHTTPTrafficInterceptor(nil)

	if config.EnableNetworkFiltering {
		engine.networkFilter = NewPlatformNetworkFilter(engine.processMonitor)
	}

	return engine
}

// Start starts the enforcement engine
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

	// Start traffic interception
	if err := ee.trafficInterceptor.Start(ctx); err != nil {
		ee.processMonitor.Stop()
		return fmt.Errorf("failed to start traffic interceptor: %w", err)
	}

	// Start network filtering if enabled
	if ee.networkFilter != nil {
		if err := ee.networkFilter.Start(ctx); err != nil {
			ee.processMonitor.Stop()
			ee.trafficInterceptor.Stop()
			return fmt.Errorf("failed to start network filter: %w", err)
		}
	}

	ee.running = true

	// Start event processing goroutines
	ee.wg.Add(3)
	go ee.processEventHandler(ctx)
	go ee.networkTrafficHandler(ctx)
	go ee.statsUpdateLoop(ctx)

	ee.logger.Info("Enforcement engine started successfully")
	return nil
}

// Stop stops the enforcement engine
func (ee *EnforcementEngine) Stop() error {
	ee.runningMu.Lock()
	defer ee.runningMu.Unlock()

	if !ee.running {
		return nil
	}

	ee.logger.Info("Stopping enforcement engine")

	ee.running = false
	close(ee.stopCh)

	// Stop components
	if ee.networkFilter != nil {
		ee.networkFilter.Stop()
	}

	if ee.trafficInterceptor != nil {
		ee.trafficInterceptor.Stop()
	}

	if ee.processMonitor != nil {
		ee.processMonitor.Stop()
	}

	// Wait for goroutines to finish
	ee.wg.Wait()

	ee.logger.Info("Enforcement engine stopped")
	return nil
}

// IsRunning returns whether the enforcement engine is running
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
	if ee.networkFilter == nil {
		return fmt.Errorf("network filtering not enabled")
	}

	if err := ee.networkFilter.AddRule(rule); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to add network rule: %w", err))
		return err
	}

	ee.logger.Info("Added network rule", logging.String("name", rule.Name), logging.String("action", string(rule.Action)))
	return nil
}

// RemoveNetworkRule removes a network filtering rule
func (ee *EnforcementEngine) RemoveNetworkRule(ruleID string) error {
	if ee.networkFilter == nil {
		return fmt.Errorf("network filtering not enabled")
	}

	if err := ee.networkFilter.RemoveRule(ruleID); err != nil {
		ee.incrementErrorCount(fmt.Errorf("failed to remove network rule: %w", err))
		return err
	}

	ee.logger.Info("Removed network rule", logging.String("rule_id", ruleID))
	return nil
}

// EvaluateNetworkRequest evaluates a network request for enforcement
func (ee *EnforcementEngine) EvaluateNetworkRequest(ctx context.Context, url string, processInfo *ProcessInfo) (*FilterDecision, error) {
	if ee.networkFilter == nil {
		return &FilterDecision{
			Action:    ActionAllow,
			Reason:    "Network filtering disabled",
			Timestamp: time.Now(),
			URL:       url,
		}, nil
	}

	startTime := time.Now()

	decision, err := ee.networkFilter.EvaluateURL(ctx, url, processInfo)
	if err != nil {
		ee.incrementErrorCount(fmt.Errorf("network evaluation error: %w", err))
		return nil, err
	}

	// Update statistics
	ee.statsMu.Lock()
	ee.stats.NetworkRequestsTotal++
	if decision.Action == ActionBlock {
		ee.stats.NetworkRequestsBlocked++
		ee.stats.RuleViolations++
	} else {
		ee.stats.NetworkRequestsAllowed++
	}

	// Update response time statistics
	responseTime := time.Since(startTime)
	if ee.stats.AverageResponseTime == 0 {
		ee.stats.AverageResponseTime = responseTime
	} else {
		ee.stats.AverageResponseTime = (ee.stats.AverageResponseTime + responseTime) / 2
	}

	ee.stats.LastEnforcementTime = time.Now()
	ee.statsMu.Unlock()

	// Log enforcement action if needed
	if ee.config.LogAllActivity || decision.Action == ActionBlock {
		ee.logger.Info("Network enforcement",
			logging.String("url", url),
			logging.String("action", string(decision.Action)),
			logging.String("rule", ee.getRuleName(decision.Rule)),
			logging.String("process", ee.getProcessName(processInfo)))
	}

	return decision, nil
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
	info["network_filtering_enabled"] = ee.networkFilter != nil
	info["config"] = ee.config

	// Add platform-specific info if available
	if lnf, ok := ee.networkFilter.(*LinuxNetworkFilter); ok {
		info["linux_filter_info"] = lnf.GetSystemInfo()
	}

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
		// Implementation would go here to block unknown processes
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
	// Update network filter stats if available
	if ee.networkFilter != nil {
		if nfe, ok := ee.networkFilter.(*NetworkFilterEngine); ok {
			netStats := nfe.GetStats()

			ee.statsMu.Lock()
			ee.stats.NetworkRequestsTotal = netStats.TotalRequests
			ee.stats.NetworkRequestsBlocked = netStats.BlockedRequests
			ee.stats.NetworkRequestsAllowed = netStats.AllowedRequests
			ee.statsMu.Unlock()
		}
	}

	ee.lastUpdate = time.Now()
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

// getRuleName safely gets rule name for logging
func (ee *EnforcementEngine) getRuleName(rule *FilterRule) string {
	if rule != nil {
		return rule.Name
	}
	return "default"
}

// getProcessName safely gets process name for logging
func (ee *EnforcementEngine) getProcessName(process *ProcessInfo) string {
	if process != nil {
		return process.Name
	}
	return "unknown"
}

// networkTrafficHandler handles network traffic events from the interceptor
func (ee *EnforcementEngine) networkTrafficHandler(ctx context.Context) {
	defer ee.wg.Done()

	// Subscribe to network traffic events
	trafficCh := ee.trafficInterceptor.Subscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ee.stopCh:
			return
		case request, ok := <-trafficCh:
			if !ok {
				return
			}
			ee.handleNetworkRequest(ctx, request)
		}
	}
}

// handleNetworkRequest processes a network request and applies filtering
func (ee *EnforcementEngine) handleNetworkRequest(ctx context.Context, request NetworkRequest) {
	// Update stats
	ee.statsMu.Lock()
	ee.stats.NetworkRequestsTotal++
	ee.statsMu.Unlock()

	// Evaluate against network filter
	if ee.networkFilter != nil {
		decision, err := ee.EvaluateNetworkRequest(ctx, request.URL, request.ProcessInfo)
		if err != nil {
			ee.incrementErrorCount(err)
			ee.logger.Error("Failed to evaluate network request",
				logging.Err(err),
				logging.String("url", request.URL))
			return
		}

		// Update stats based on decision
		ee.statsMu.Lock()
		if decision.Action == ActionBlock {
			ee.stats.NetworkRequestsBlocked++
			ee.stats.RuleViolations++
		} else {
			ee.stats.NetworkRequestsAllowed++
		}
		ee.stats.LastEnforcementTime = time.Now()
		ee.statsMu.Unlock()

		// Log the decision
		ee.logger.Info("Network request evaluated",
			logging.String("url", request.URL),
			logging.String("domain", request.Domain),
			logging.String("action", string(decision.Action)),
			logging.String("reason", decision.Reason))

		// If blocked, the network filter should handle the actual blocking
		if decision.Action == ActionBlock {
			ee.statsMu.Lock()
			ee.stats.EnforcementActions++
			ee.statsMu.Unlock()
		}
	}
}
