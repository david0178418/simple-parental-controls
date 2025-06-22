package service

import (
	"context"
	"testing"
	"time"

	"parental-control/internal/database"
	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/testutil"
)

func TestAuditService_LogEnforcementAction(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Test logging enforcement action
	err := auditService.LogEnforcementAction(
		ctx,
		models.ActionTypeBlock,
		models.TargetTypeURL,
		"malicious-site.com",
		"blacklist",
		intPtr(123),
		map[string]interface{}{
			"reason":    "blocked by security rule",
			"source_ip": "192.168.1.100",
		},
	)

	if err != nil {
		t.Errorf("LogEnforcementAction failed: %v", err)
	}

	// Verify the log was created
	logs, err := auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to retrieve audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 audit log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != "enforcement_action" {
		t.Errorf("Expected event_type 'enforcement_action', got '%s'", log.EventType)
	}

	if log.Action != models.ActionTypeBlock {
		t.Errorf("Expected action 'block', got '%s'", log.Action)
	}

	if log.TargetValue != "malicious-site.com" {
		t.Errorf("Expected target_value 'malicious-site.com', got '%s'", log.TargetValue)
	}

	// Test details parsing
	details, err := log.GetDetailsMap()
	if err != nil {
		t.Errorf("Failed to parse details: %v", err)
	}

	if details["reason"] != "blocked by security rule" {
		t.Errorf("Expected reason 'blocked by security rule', got '%v'", details["reason"])
	}
}

func TestAuditService_LogRuleChange(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Test logging rule change
	err := auditService.LogRuleChange(
		ctx,
		"time_rule",
		456,
		"update",
		map[string]interface{}{
			"old_value": "allow_during",
			"new_value": "block_during",
		},
	)

	if err != nil {
		t.Errorf("LogRuleChange failed: %v", err)
	}

	// Verify the log was created
	logs, err := auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to retrieve audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 audit log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != "rule_change" {
		t.Errorf("Expected event_type 'rule_change', got '%s'", log.EventType)
	}

	if log.RuleType != "time_rule" {
		t.Errorf("Expected rule_type 'time_rule', got '%s'", log.RuleType)
	}

	if log.RuleID == nil || *log.RuleID != 456 {
		t.Errorf("Expected rule_id 456, got %v", log.RuleID)
	}
}

func TestAuditService_LogUserAction(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Test logging user action
	err := auditService.LogUserAction(
		ctx,
		1,
		"login",
		map[string]interface{}{
			"ip_address": "192.168.1.200",
			"user_agent": "Mozilla/5.0",
		},
	)

	if err != nil {
		t.Errorf("LogUserAction failed: %v", err)
	}

	// Verify the log was created
	logs, err := auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to retrieve audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 audit log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != "user_action" {
		t.Errorf("Expected event_type 'user_action', got '%s'", log.EventType)
	}

	if log.TargetValue != "user:1" {
		t.Errorf("Expected target_value 'user:1', got '%s'", log.TargetValue)
	}
}

func TestAuditService_LogSystemEvent(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Test logging system event
	err := auditService.LogSystemEvent(
		ctx,
		"service_started",
		"info",
		map[string]interface{}{
			"version": "1.0.0",
			"pid":     12345,
		},
	)

	if err != nil {
		t.Errorf("LogSystemEvent failed: %v", err)
	}

	// Verify the log was created
	logs, err := auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to retrieve audit logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 audit log, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != "system_event" {
		t.Errorf("Expected event_type 'system_event', got '%s'", log.EventType)
	}

	if log.TargetValue != "service_started" {
		t.Errorf("Expected target_value 'service_started', got '%s'", log.TargetValue)
	}

	if log.Action != models.ActionTypeAllow {
		t.Errorf("Expected action 'allow' for info severity, got '%s'", log.Action)
	}
}

func TestAuditService_GetAuditLogs(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Create some test logs
	testLogs := []struct {
		action     models.ActionType
		targetType models.TargetType
		eventType  string
	}{
		{models.ActionTypeAllow, models.TargetTypeURL, "enforcement_action"},
		{models.ActionTypeBlock, models.TargetTypeURL, "enforcement_action"},
		{models.ActionTypeAllow, models.TargetTypeExecutable, "enforcement_action"},
		{models.ActionTypeAllow, models.TargetTypeURL, "user_action"},
	}

	for i, testLog := range testLogs {
		err := auditService.LogEvent(ctx, AuditEventRequest{
			EventType:   testLog.eventType,
			TargetType:  testLog.targetType,
			TargetValue: "test-value",
			Action:      testLog.action,
			Details:     map[string]interface{}{"test": i},
		})
		if err != nil {
			t.Errorf("Failed to create test log %d: %v", i, err)
		}
	}

	// Test GetAuditLogs with no filters
	logs, totalCount, err := auditService.GetAuditLogs(ctx, AuditLogFilters{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Errorf("GetAuditLogs failed: %v", err)
	}

	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	if totalCount != 4 {
		t.Errorf("Expected total count 4, got %d", totalCount)
	}

	// Test GetAuditLogs with action filter
	blockAction := models.ActionTypeBlock
	logs, _, err = auditService.GetAuditLogs(ctx, AuditLogFilters{
		Action: &blockAction,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Errorf("GetAuditLogs with action filter failed: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log with block action, got %d", len(logs))
	}

	// Test GetAuditLogs with target type filter
	executableType := models.TargetTypeExecutable
	logs, _, err = auditService.GetAuditLogs(ctx, AuditLogFilters{
		TargetType: &executableType,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Errorf("GetAuditLogs with target type filter failed: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("Expected 1 log with executable target type, got %d", len(logs))
	}
}

func TestAuditService_GetStats(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Initial stats should be empty
	stats := auditService.GetStats()
	if stats.TotalLogged != 0 {
		t.Errorf("Expected TotalLogged 0, got %d", stats.TotalLogged)
	}

	// Log some events
	err := auditService.LogEnforcementAction(ctx, models.ActionTypeBlock, models.TargetTypeURL, "test.com", "test", nil, nil)
	if err != nil {
		t.Errorf("Failed to log enforcement action: %v", err)
	}

	err = auditService.LogUserAction(ctx, 1, "login", nil)
	if err != nil {
		t.Errorf("Failed to log user action: %v", err)
	}

	// Check updated stats
	stats = auditService.GetStats()
	if stats.TotalLogged != 2 {
		t.Errorf("Expected TotalLogged 2, got %d", stats.TotalLogged)
	}

	if stats.EventTypeStats["enforcement_action"] != 1 {
		t.Errorf("Expected enforcement_action count 1, got %d", stats.EventTypeStats["enforcement_action"])
	}

	if stats.EventTypeStats["user_action"] != 1 {
		t.Errorf("Expected user_action count 1, got %d", stats.EventTypeStats["user_action"])
	}

	if stats.ActionTypeStats["block"] != 1 {
		t.Errorf("Expected block action count 1, got %d", stats.ActionTypeStats["block"])
	}

	if stats.ActionTypeStats["allow"] != 1 {
		t.Errorf("Expected allow action count 1, got %d", stats.ActionTypeStats["allow"])
	}
}

func TestAuditService_CleanupOldLogs(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDatabase(t)
	defer testDB.Cleanup()

	// Create audit repository
	auditRepo := database.NewAuditLogRepository(testDB.DB.Connection())

	// Create repository manager with audit repository
	repos := &models.RepositoryManager{
		AuditLog: auditRepo,
	}

	// Create audit service with 1-day retention
	logger := logging.NewDefault()
	config := DefaultAuditConfig()
	config.RetentionDays = 1
	config.EnableBuffering = false // Disable for testing
	config.EnableBatching = false

	auditService := NewAuditService(repos, logger, config)

	ctx := context.Background()

	// Create an old log entry (manually insert to control timestamp)
	oldLog := &models.AuditLog{
		Timestamp:   time.Now().AddDate(0, 0, -2), // 2 days ago
		EventType:   "test_event",
		TargetType:  models.TargetTypeURL,
		TargetValue: "old-log.com",
		Action:      models.ActionTypeAllow,
		CreatedAt:   time.Now().AddDate(0, 0, -2),
	}
	err := auditRepo.Create(ctx, oldLog)
	if err != nil {
		t.Errorf("Failed to create old log: %v", err)
	}

	// Create a recent log
	err = auditService.LogEnforcementAction(ctx, models.ActionTypeAllow, models.TargetTypeURL, "new-log.com", "test", nil, nil)
	if err != nil {
		t.Errorf("Failed to create recent log: %v", err)
	}

	// Verify we have 2 logs
	logs, err := auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to get all logs: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs before cleanup, got %d", len(logs))
	}

	// Run cleanup
	cleanedCount, err := auditService.CleanupOldLogs(ctx)
	if err != nil {
		t.Errorf("CleanupOldLogs failed: %v", err)
	}

	if cleanedCount != 1 {
		t.Errorf("Expected to clean 1 log, cleaned %d", cleanedCount)
	}

	// Verify only 1 log remains
	logs, err = auditRepo.GetAll(ctx, 10, 0)
	if err != nil {
		t.Errorf("Failed to get all logs after cleanup: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("Expected 1 log after cleanup, got %d", len(logs))
	}

	// Verify the remaining log is the recent one
	if logs[0].TargetValue != "new-log.com" {
		t.Errorf("Expected remaining log to be 'new-log.com', got '%s'", logs[0].TargetValue)
	}
}

// Helper function to create an int pointer
func intPtr(i int) *int {
	return &i
}
