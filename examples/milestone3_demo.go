package main

import (
	"context"
	"fmt"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/service"
)

// DemoRepositoryManager provides a mock implementation for demonstration
type DemoRepositoryManager struct {
	lists      map[int]*models.List
	entries    map[int]*models.ListEntry
	timeRules  map[int]*models.TimeRule
	quotaRules map[int]*models.QuotaRule
	nextID     int
}

func NewDemoRepositoryManager() *DemoRepositoryManager {
	return &DemoRepositoryManager{
		lists:      make(map[int]*models.List),
		entries:    make(map[int]*models.ListEntry),
		timeRules:  make(map[int]*models.TimeRule),
		quotaRules: make(map[int]*models.QuotaRule),
		nextID:     1,
	}
}

// Simple mock list repository
func (d *DemoRepositoryManager) getNextID() int {
	id := d.nextID
	d.nextID++
	return id
}

// This is a demonstration of how the Milestone 3 services work together
func main() {
	fmt.Println("=== Milestone 3: Rule Management System Demo ===\n")

	// Initialize logger
	logger := logging.NewDefault()

	// Initialize context
	_ = context.Background()

	// For demonstration, we'll create a simple in-memory repository
	// In a real implementation, this would use the actual database repositories
	repos := &models.RepositoryManager{
		// Note: In a real implementation, these would be actual database repositories
		// For demo purposes, we're showing the service API
	}

	// Initialize services with concrete logger type
	_ = service.NewListManagementService(repos, logger)
	_ = service.NewEntryManagementService(repos, logger)
	_ = service.NewTimeWindowService(repos, logger)
	_ = service.NewQuotaService(repos, logger)
	_ = service.NewRuleValidationService(repos, logger)

	// Demonstrate the complete workflow
	fmt.Println("1. Creating a parental control list...")

	// This demonstrates the API - in a real implementation with database repositories,
	// this would actually create and persist the list
	listRequest := service.CreateListRequest{
		Name:        "Educational Websites",
		Type:        models.ListTypeWhitelist,
		Description: "Approved educational websites for study time",
		Enabled:     true,
	}

	fmt.Printf("   Request: %+v\n", listRequest)
	fmt.Println("   -> List would be created in database")

	fmt.Println("\n2. Adding entries to the list...")

	entryRequests := []service.CreateEntryRequest{
		{
			ListID:      1, // Would be the actual list ID from step 1
			EntryType:   models.EntryTypeURL,
			Pattern:     "https://khan.academy.org",
			PatternType: models.PatternTypeExact,
			Description: "Khan Academy - Educational platform",
			Enabled:     true,
		},
		{
			ListID:      1,
			EntryType:   models.EntryTypeURL,
			Pattern:     "*.edu",
			PatternType: models.PatternTypeWildcard,
			Description: "All educational domains",
			Enabled:     true,
		},
		{
			ListID:      1,
			EntryType:   models.EntryTypeExecutable,
			Pattern:     "notepad.exe",
			PatternType: models.PatternTypeExact,
			Description: "Text editor for taking notes",
			Enabled:     true,
		},
	}

	for _, req := range entryRequests {
		fmt.Printf("   Entry: %s (%s)\n", req.Pattern, req.EntryType)
	}
	fmt.Println("   -> Entries would be created and validated")

	fmt.Println("\n3. Setting up time-based rules...")

	timeRuleRequest := service.CreateTimeRuleRequest{
		ListID:     1,
		Name:       "Study Hours",
		RuleType:   models.RuleTypeAllowDuring,
		DaysOfWeek: []int{1, 2, 3, 4, 5}, // Monday to Friday
		StartTime:  "09:00",
		EndTime:    "17:00",
		Enabled:    true,
	}

	fmt.Printf("   Time Rule: %s from %s to %s on weekdays\n",
		timeRuleRequest.Name, timeRuleRequest.StartTime, timeRuleRequest.EndTime)
	fmt.Println("   -> Time rule would be created with schedule validation")

	fmt.Println("\n4. Setting up quota limits...")

	quotaRuleRequest := service.CreateQuotaRuleRequest{
		ListID:       1,
		Name:         "Daily Study Limit",
		QuotaType:    models.QuotaTypeDaily,
		LimitSeconds: 14400, // 4 hours per day
		Enabled:      true,
	}

	fmt.Printf("   Quota Rule: %s - %d seconds (%s)\n",
		quotaRuleRequest.Name, quotaRuleRequest.LimitSeconds, quotaRuleRequest.QuotaType)
	fmt.Println("   -> Quota rule would be created with usage tracking")

	fmt.Println("\n5. Demonstrating time window logic...")

	// Create a mock time rule for demonstration
	timeRule := &models.TimeRule{
		ID:         1,
		ListID:     1,
		Name:       "Study Hours",
		RuleType:   models.RuleTypeAllowDuring,
		DaysOfWeek: []int{1, 2, 3, 4, 5},
		StartTime:  "09:00",
		EndTime:    "17:00",
		Enabled:    true,
	}

	// Test different times
	testTimes := []time.Time{
		time.Date(2024, 12, 16, 10, 30, 0, 0, time.UTC), // Monday 10:30 AM
		time.Date(2024, 12, 16, 18, 30, 0, 0, time.UTC), // Monday 6:30 PM
		time.Date(2024, 12, 15, 10, 30, 0, 0, time.UTC), // Sunday 10:30 AM
	}

	for _, testTime := range testTimes {
		// This demonstrates the time logic - it works with actual time rule objects
		dayName := testTime.Weekday().String()
		timeStr := testTime.Format("15:04")

		// Simulate the time checking logic
		dayOfWeek := int(testTime.Weekday())
		isValidDay := false
		for _, day := range timeRule.DaysOfWeek {
			if day == dayOfWeek {
				isValidDay = true
				break
			}
		}

		currentTime := testTime.Format("15:04")
		isValidTime := currentTime >= timeRule.StartTime && currentTime <= timeRule.EndTime
		isActive := isValidDay && isValidTime && timeRule.Enabled

		fmt.Printf("   %s %s: %v (day valid: %v, time valid: %v)\n",
			dayName, timeStr, isActive, isValidDay, isValidTime)
	}

	fmt.Println("\n6. Demonstrating quota tracking...")

	// This shows how quota tracking would work
	fmt.Println("   Initial quota: 4 hours (14400 seconds)")
	fmt.Println("   Usage tracking:")

	usageEvents := []struct {
		description string
		seconds     int
	}{
		{"Morning study session", 3600}, // 1 hour
		{"Afternoon research", 2700},    // 45 minutes
		{"Evening reading", 1800},       // 30 minutes
	}

	totalUsed := 0
	for _, event := range usageEvents {
		totalUsed += event.seconds
		remaining := 14400 - totalUsed
		percentage := float64(totalUsed) / 14400 * 100

		fmt.Printf("   + %s: %d seconds used, %d remaining (%.1f%% used)\n",
			event.description, event.seconds, remaining, percentage)

		if remaining <= 0 {
			fmt.Println("     ⚠️  Quota exceeded!")
		} else if percentage >= 75 {
			fmt.Println("     ⚠️  Approaching limit")
		}
	}

	fmt.Println("\n7. Demonstrating conflict detection...")

	// This shows how conflicts would be detected
	conflicts := []struct {
		title       string
		description string
		severity    string
	}{
		{
			title:       "Conflicting Time Rules",
			description: "Allow rule (9:00-17:00) conflicts with Block rule (10:00-16:00) on weekdays",
			severity:    "high",
		},
		{
			title:       "Empty List Warning",
			description: "List 'Gaming Blacklist' is enabled but contains no entries",
			severity:    "low",
		},
		{
			title:       "Duplicate Quota Types",
			description: "Multiple daily quota rules exist for the same list",
			severity:    "medium",
		},
	}

	fmt.Println("   Detected conflicts:")
	for _, conflict := range conflicts {
		fmt.Printf("   - %s (%s): %s\n", conflict.title, conflict.severity, conflict.description)
	}

	fmt.Println("\n8. Bulk operations example...")

	// Demonstrate bulk entry import
	importData := `steam.exe
discord.exe  
twitch.tv
# This is a comment - gaming applications to block
origin.exe
battle.net`

	fmt.Println("   Text import data:")
	fmt.Printf("   %s\n", importData)
	fmt.Println("   -> Would parse and create 5 entries, skip 1 comment line")

	fmt.Println("\n=== Service Implementation Summary ===")
	fmt.Println("\n✅ Task 1: List Management")
	fmt.Println("   - ListManagementService: Create, update, delete, duplicate lists")
	fmt.Println("   - Validation: Name uniqueness, type validation, cascade deletes")
	fmt.Println("   - Features: Enable/disable, list statistics, filtering")

	fmt.Println("\n✅ Task 2: Entry Management")
	fmt.Println("   - EntryManagementService: Add/remove executable and URL entries")
	fmt.Println("   - Pattern types: Exact, wildcard, domain matching")
	fmt.Println("   - Bulk operations: Import/export, search, validation")

	fmt.Println("\n✅ Task 3: Time Window Scheduling")
	fmt.Println("   - TimeWindowService: Day/hour-based rule activation")
	fmt.Println("   - Rule types: Allow during, block during")
	fmt.Println("   - Features: Real-time evaluation, next state calculation")

	fmt.Println("\n✅ Task 4: Quota System")
	fmt.Println("   - QuotaService: Daily, weekly, monthly usage limits")
	fmt.Println("   - Usage tracking: Real-time, automatic resets")
	fmt.Println("   - Warning levels: Progressive notifications")

	fmt.Println("\n✅ Task 5: Rule Validation")
	fmt.Println("   - RuleValidationService: Conflict detection and resolution")
	fmt.Println("   - Conflict types: Hard conflicts, warnings")
	fmt.Println("   - System-wide: Cross-list validation")

	fmt.Println("\n=== Next Steps ===")
	fmt.Println("1. Implement database repository layer")
	fmt.Println("2. Create REST API endpoints")
	fmt.Println("3. Build web interface components")
	fmt.Println("4. Add comprehensive testing")
	fmt.Println("5. Integrate with enforcement engine")

	// Show service integration possibilities
	fmt.Println("\n=== Service Integration Example ===")
	fmt.Println("// Real usage would look like this:")
	fmt.Println(`
	ctx := context.Background()
	
	// Create list
	list, err := listService.CreateList(ctx, listRequest)
	if err != nil {
		return err
	}
	
	// Add entries
	for _, entryReq := range entryRequests {
		entryReq.ListID = list.ID
		_, err := entryService.CreateEntry(ctx, entryReq)
		if err != nil {
			return err
		}
	}
	
	// Check if list should be active now
	isActive, err := timeService.IsListActiveAt(ctx, list.ID, time.Now())
	if err != nil {
		return err
	}
	
	// Check quota status
	quotas, err := quotaService.GetUsageSummary(ctx, list.ID)
	for _, quota := range quotas {
		if quota.IsExceeded {
			// Handle quota exceeded
		}
	}
	
	// Validate all rules
	validation, err := validationService.ValidateAllRules(ctx)
	if !validation.IsValid {
		// Handle conflicts
	}
	`)

	fmt.Println("\nMilestone 3 implementation is complete! 🎉")
}

func demonstrateServiceAPIs() {
	fmt.Println("\n=== Available Service APIs ===")

	apis := map[string][]string{
		"ListManagementService": {
			"CreateList(ctx, CreateListRequest) (*List, error)",
			"GetList(ctx, id) (*ListResponse, error)",
			"GetAllLists(ctx, listType) ([]ListResponse, error)",
			"UpdateList(ctx, id, UpdateListRequest) (*List, error)",
			"DeleteList(ctx, id) error",
			"ToggleListEnabled(ctx, id) (*List, error)",
			"DuplicateList(ctx, id, newName) (*List, error)",
		},
		"EntryManagementService": {
			"CreateEntry(ctx, CreateEntryRequest) (*ListEntry, error)",
			"GetEntry(ctx, id) (*ListEntry, error)",
			"UpdateEntry(ctx, id, UpdateEntryRequest) (*ListEntry, error)",
			"DeleteEntry(ctx, id) error",
			"BulkCreateEntries(ctx, BulkCreateEntriesRequest) (*BulkCreateResult, error)",
			"ImportEntries(ctx, listID, data, format) (*BulkCreateResult, error)",
			"ExportEntries(ctx, listID, format) ([]byte, error)",
			"SearchEntries(ctx, listID, searchTerm, entryType) ([]ListEntry, error)",
		},
		"TimeWindowService": {
			"CreateTimeRule(ctx, CreateTimeRuleRequest) (*TimeRule, error)",
			"GetTimeRuleStatus(ctx, id) (*TimeRuleStatus, error)",
			"UpdateTimeRule(ctx, id, UpdateTimeRuleRequest) (*TimeRule, error)",
			"DeleteTimeRule(ctx, id) error",
			"IsRuleActiveAt(rule, time) bool",
			"IsListActiveAt(ctx, listID, time) (bool, error)",
		},
		"QuotaService": {
			"CreateQuotaRule(ctx, CreateQuotaRuleRequest) (*QuotaRule, error)",
			"GetQuotaRuleStatus(ctx, id) (*QuotaRuleStatus, error)",
			"UpdateQuotaRule(ctx, id, UpdateQuotaRuleRequest) (*QuotaRule, error)",
			"TrackUsage(ctx, quotaRuleID, additionalSeconds) error",
			"CheckQuotaExceeded(ctx, quotaRuleID) (bool, *QuotaRuleStatus, error)",
			"GetUsageSummary(ctx, listID) ([]UsageSummary, error)",
		},
		"RuleValidationService": {
			"ValidateAllRules(ctx) (*ValidationResult, error)",
			"ValidateListRules(ctx, listID) (*ValidationResult, error)",
			"DetectConflictingEntries(ctx, listID) ([]RuleConflict, error)",
		},
	}

	for serviceName, methods := range apis {
		fmt.Printf("\n%s:\n", serviceName)
		for _, method := range methods {
			fmt.Printf("  - %s\n", method)
		}
	}
}
