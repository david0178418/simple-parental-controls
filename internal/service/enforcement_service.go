package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"parental-control/internal/enforcement"
	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// EnforcementService manages the enforcement engine and rule synchronization
type EnforcementService struct {
	engine   *enforcement.EnforcementEngine
	repos    *models.RepositoryManager
	logger   logging.Logger
	config   enforcement.EnforcementConfig
	
	// State management
	running   bool
	runningMu sync.RWMutex
	
	// Rule synchronization
	syncInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewEnforcementService creates a new enforcement service
func NewEnforcementService(
	repos *models.RepositoryManager,
	logger logging.Logger,
	config enforcement.EnforcementConfig,
) *EnforcementService {
	auditConfig := AuditConfig{
		BufferSize:      1000,
		BatchSize:       10,
		BatchTimeout:    5 * time.Second,
		FlushInterval:   30 * time.Second,
		EnableBuffering: true,
	}
	auditService := NewAuditService(repos, logger, auditConfig)
	engine := enforcement.NewEnforcementEngine(&config, logger, auditService)
	
	return &EnforcementService{
		engine:       engine,
		repos:        repos,
		logger:       logger,
		config:       config,
		syncInterval: 30 * time.Second, // Sync rules every 30 seconds
		stopCh:       make(chan struct{}),
	}
}

// Start starts the enforcement service and begins rule synchronization
func (es *EnforcementService) Start(ctx context.Context) error {
	es.runningMu.Lock()
	defer es.runningMu.Unlock()
	
	if es.running {
		return fmt.Errorf("enforcement service is already running")
	}
	
	es.logger.Info("Starting enforcement service")
	
	// Start the enforcement engine
	if err := es.engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start enforcement engine: %w", err)
	}
	
	// Perform initial rule synchronization
	if err := es.SyncRules(ctx); err != nil {
		es.logger.Error("Initial rule synchronization failed", logging.Err(err))
		// Don't fail startup - continue with periodic sync
	}
	
	es.running = true
	
	// Start periodic rule synchronization
	es.wg.Add(1)
	go es.ruleSyncLoop(ctx)
	
	es.logger.Info("Enforcement service started successfully")
	return nil
}

// Stop stops the enforcement service gracefully
func (es *EnforcementService) Stop(ctx context.Context) error {
	es.runningMu.Lock()
	defer es.runningMu.Unlock()
	
	if !es.running {
		return nil
	}
	
	es.logger.Info("Stopping enforcement service")
	
	// Signal sync loop to stop
	close(es.stopCh)
	
	// Wait for sync loop to finish
	es.wg.Wait()
	
	// Stop the enforcement engine
	if err := es.engine.Stop(ctx); err != nil {
		es.logger.Error("Error stopping enforcement engine", logging.Err(err))
		return err
	}
	
	es.running = false
	es.logger.Info("Enforcement service stopped successfully")
	return nil
}

// IsRunning returns true if the enforcement service is running
func (es *EnforcementService) IsRunning() bool {
	es.runningMu.RLock()
	defer es.runningMu.RUnlock()
	return es.running
}

// SyncRules synchronizes rules from the database to the enforcement engine
func (es *EnforcementService) SyncRules(ctx context.Context) error {
	es.logger.Debug("Starting rule synchronization")
	
	// Get current rules from enforcement engine
	currentRules := es.engine.GetCurrentRules()
	
	// Get desired rules from database
	desiredRules, err := es.getDesiredRulesFromDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to get desired rules: %w", err)
	}
	
	es.logger.Debug("Rule sync status",
		logging.Int("current_rules_count", len(currentRules)),
		logging.Int("desired_rules_count", len(desiredRules)))
	
	var rulesAdded, rulesRemoved, rulesSkipped int
	
	// Add new rules that don't exist
	for pattern, rule := range desiredRules {
		if _, exists := currentRules[pattern]; !exists {
			if err := es.engine.AddNetworkRule(rule); err != nil {
				es.logger.Error("Failed to add network rule",
					logging.Err(err),
					logging.String("pattern", pattern))
				rulesSkipped++
				continue
			}
			rulesAdded++
		}
	}
	
	// Remove rules that no longer exist in database
	for pattern, rule := range currentRules {
		if _, exists := desiredRules[pattern]; !exists {
			if err := es.engine.RemoveNetworkRule(pattern); err != nil {
				es.logger.Error("Failed to remove network rule",
					logging.Err(err),
					logging.String("pattern", pattern))
				rulesSkipped++
				continue
			}
			rulesRemoved++
			es.logger.Info("Removed network rule", 
				logging.String("pattern", pattern),
				logging.String("rule_name", rule.Name))
		}
	}
	
	es.logger.Info("Rule synchronization completed",
		logging.Int("rules_added", rulesAdded),
		logging.Int("rules_removed", rulesRemoved),
		logging.Int("rules_skipped", rulesSkipped),
		logging.Int("total_current", len(currentRules)),
		logging.Int("total_desired", len(desiredRules)))
	
	return nil
}

// getDesiredRulesFromDatabase gets all rules that should be active based on database state
func (es *EnforcementService) getDesiredRulesFromDatabase(ctx context.Context) (map[string]*enforcement.FilterRule, error) {
	desiredRules := make(map[string]*enforcement.FilterRule)
	
	// Get all enabled lists
	lists, err := es.repos.List.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}
	
	for _, list := range lists {
		if !list.Enabled {
			continue // Skip disabled lists
		}
		
		// Get entries for this list
		entries, err := es.repos.ListEntry.GetByListID(ctx, list.ID)
		if err != nil {
			es.logger.Error("Failed to get entries for list", 
				logging.Err(err), 
				logging.Int("list_id", list.ID))
			continue
		}
		
		// Convert entries to enforcement rules
		for _, entry := range entries {
			if !entry.Enabled {
				continue // Skip disabled entries
			}
			
			rule := es.convertEntryToRule(&list, &entry)
			if rule == nil {
				continue
			}
			
			// Use pattern as key to avoid duplicates
			desiredRules[rule.Pattern] = rule
		}
	}
	
	return desiredRules, nil
}

// RefreshRules forces an immediate rule refresh
func (es *EnforcementService) RefreshRules(ctx context.Context) error {
	es.logger.Debug("Forcing immediate rule refresh")
	return es.SyncRules(ctx)
}

// GetStats returns enforcement statistics
func (es *EnforcementService) GetStats() *enforcement.EnforcementStats {
	if es.engine == nil {
		return &enforcement.EnforcementStats{}
	}
	return es.engine.GetStats()
}

// GetSystemInfo returns system information about enforcement
func (es *EnforcementService) GetSystemInfo() map[string]interface{} {
	info := map[string]interface{}{
		"service_running": es.IsRunning(),
		"sync_interval":   es.syncInterval.String(),
	}
	
	if es.engine != nil {
		engineInfo := es.engine.GetSystemInfo()
		info["engine"] = engineInfo
	}
	
	return info
}

// convertEntryToRule converts a database entry to an enforcement rule
func (es *EnforcementService) convertEntryToRule(list *models.List, entry *models.ListEntry) *enforcement.FilterRule {
	// Determine action based on list type
	var action enforcement.FilterAction
	switch list.Type {
	case models.ListTypeWhitelist:
		action = enforcement.ActionAllow
	case models.ListTypeBlacklist:
		action = enforcement.ActionBlock
	default:
		es.logger.Warn("Unknown list type", logging.String("type", string(list.Type)))
		return nil
	}
	
	// Determine match type based on pattern type
	var matchType enforcement.MatchType
	switch entry.PatternType {
	case models.PatternTypeExact:
		matchType = enforcement.MatchExact
	case models.PatternTypeWildcard:
		matchType = enforcement.MatchWildcard
	case models.PatternTypeDomain:
		matchType = enforcement.MatchDomain
	default:
		matchType = enforcement.MatchExact // Default fallback
	}
	
	// Generate unique rule ID and name
	ruleID := fmt.Sprintf("rule_%d_%d", list.ID, entry.ID)
	ruleName := fmt.Sprintf("%s_%s_%d", list.Name, entry.EntryType, entry.ID)
	
	return &enforcement.FilterRule{
		ID:        ruleID,
		Name:      ruleName,
		Pattern:   entry.Pattern,
		Action:    action,
		MatchType: matchType,
		Priority:  1, // Default priority
		Enabled:   entry.Enabled,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	}
}

// ruleSyncLoop runs periodic rule synchronization
func (es *EnforcementService) ruleSyncLoop(ctx context.Context) {
	defer es.wg.Done()
	
	ticker := time.NewTicker(es.syncInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-es.stopCh:
			return
		case <-ticker.C:
			if err := es.SyncRules(ctx); err != nil {
				es.logger.Error("Periodic rule synchronization failed", 
					logging.Err(err),
					logging.String("sync_type", "periodic"))
			}
		}
	}
}