package service

import (
	"context"
	"testing"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/testutil"
)

// TestMilestone3Integration demonstrates the complete rule management system
func TestMilestone3Integration(t *testing.T) {
	// Setup test environment
	ctx := context.Background()
	logger := logging.NewDefault()

	// Create mock repositories (in a real test, you'd use actual database)
	repos := &models.RepositoryManager{
		List:       testutil.NewMockListRepository(),
		ListEntry:  testutil.NewMockListEntryRepository(),
		TimeRule:   testutil.NewMockTimeRuleRepository(),
		QuotaRule:  testutil.NewMockQuotaRuleRepository(),
		QuotaUsage: testutil.NewMockQuotaUsageRepository(),
		AuditLog:   testutil.NewMockAuditLogRepository(),
	}

	// Initialize services
	listService := NewListManagementService(repos, logger)
	entryService := NewEntryManagementService(repos, logger)
	timeService := NewTimeWindowService(repos, logger)
	quotaService := NewQuotaService(repos, logger)
	validationService := NewRuleValidationService(repos, logger)

	t.Run("Complete Rule Management Workflow", func(t *testing.T) {
		// Step 1: Create a whitelist for social media
		listReq := CreateListRequest{
			Name:        "Social Media Whitelist",
			Type:        models.ListTypeWhitelist,
			Description: "Allowed social media platforms during study hours",
			Enabled:     true,
		}

		list, err := listService.CreateList(ctx, listReq)
		if err != nil {
			t.Fatalf("Failed to create list: %v", err)
		}

		t.Logf("Created list: %s (ID: %d)", list.Name, list.ID)

		// Step 2: Add entries to the list
		entries := []CreateEntryRequest{
			{
				ListID:      list.ID,
				EntryType:   models.EntryTypeURL,
				Pattern:     "https://education.example.com",
				PatternType: models.PatternTypeExact,
				Description: "Educational platform",
				Enabled:     true,
			},
			{
				ListID:      list.ID,
				EntryType:   models.EntryTypeURL,
				Pattern:     "*.edu",
				PatternType: models.PatternTypeWildcard,
				Description: "Educational domains",
				Enabled:     true,
			},
			{
				ListID:      list.ID,
				EntryType:   models.EntryTypeExecutable,
				Pattern:     "notepad.exe",
				PatternType: models.PatternTypeExact,
				Description: "Text editor for notes",
				Enabled:     true,
			},
		}

		for _, entryReq := range entries {
			entry, err := entryService.CreateEntry(ctx, entryReq)
			if err != nil {
				t.Fatalf("Failed to create entry: %v", err)
			}
			t.Logf("Created entry: %s", entry.Pattern)
		}

		// Step 3: Create time-based rules
		timeRuleReq := CreateTimeRuleRequest{
			ListID:     list.ID,
			Name:       "Study Hours Allow",
			RuleType:   models.RuleTypeAllowDuring,
			DaysOfWeek: []int{1, 2, 3, 4, 5}, // Monday to Friday
			StartTime:  "09:00",
			EndTime:    "17:00",
			Enabled:    true,
		}

		timeRule, err := timeService.CreateTimeRule(ctx, timeRuleReq)
		if err != nil {
			t.Fatalf("Failed to create time rule: %v", err)
		}
		t.Logf("Created time rule: %s", timeRule.Name)

		// Step 4: Create quota rules
		quotaRuleReq := CreateQuotaRuleRequest{
			ListID:       list.ID,
			Name:         "Daily Social Media Limit",
			QuotaType:    models.QuotaTypeDaily,
			LimitSeconds: 3600, // 1 hour per day
			Enabled:      true,
		}

		quotaRule, err := quotaService.CreateQuotaRule(ctx, quotaRuleReq)
		if err != nil {
			t.Fatalf("Failed to create quota rule: %v", err)
		}
		t.Logf("Created quota rule: %s", quotaRule.Name)

		// Step 5: Test time window logic
		now := time.Now()
		testTime := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, now.Location()) // 2:30 PM on a weekday

		isActive := timeService.IsRuleActiveAt(timeRule, testTime)
		if !isActive {
			t.Errorf("Expected time rule to be active at %v", testTime)
		}
		t.Logf("Time rule is active at %v: %v", testTime, isActive)

		// Step 6: Test quota tracking
		err = quotaService.TrackUsage(ctx, quotaRule.ID, 1800) // 30 minutes used
		if err != nil {
			t.Fatalf("Failed to track usage: %v", err)
		}

		quotaStatus, err := quotaService.GetQuotaRuleStatus(ctx, quotaRule.ID)
		if err != nil {
			t.Fatalf("Failed to get quota status: %v", err)
		}
		t.Logf("Quota status: %v remaining, warning level: %v",
			quotaStatus.RemainingTime, quotaStatus.WarningLevel)

		// Step 7: Validate all rules
		validation, err := validationService.ValidateListRules(ctx, list.ID)
		if err != nil {
			t.Fatalf("Failed to validate rules: %v", err)
		}

		if !validation.IsValid {
			t.Logf("Found %d conflicts:", len(validation.Conflicts))
			for _, conflict := range validation.Conflicts {
				t.Logf("  - %s: %s", conflict.Title, conflict.Description)
			}
		} else {
			t.Logf("All rules are valid!")
		}

		// Step 8: Test list management operations
		// Toggle list state
		toggledList, err := listService.ToggleListEnabled(ctx, list.ID)
		if err != nil {
			t.Fatalf("Failed to toggle list: %v", err)
		}
		t.Logf("Toggled list enabled state to: %v", toggledList.Enabled)

		// Get usage summary
		summary, err := quotaService.GetUsageSummary(ctx, list.ID)
		if err != nil {
			t.Fatalf("Failed to get usage summary: %v", err)
		}

		for _, usage := range summary {
			t.Logf("Usage summary for %s: %.1f%% used", usage.RuleName, usage.UsagePercent)
		}

		// Step 9: Test entry search
		searchResults, err := entryService.SearchEntries(ctx, list.ID, "edu", nil)
		if err != nil {
			t.Fatalf("Failed to search entries: %v", err)
		}
		t.Logf("Found %d entries matching 'edu'", len(searchResults))

		// Step 10: Test conflict detection with a conflicting rule
		conflictingTimeRule := CreateTimeRuleRequest{
			ListID:     list.ID,
			Name:       "Study Hours Block",
			RuleType:   models.RuleTypeBlockDuring,
			DaysOfWeek: []int{1, 2, 3}, // Monday to Wednesday (overlaps with allow rule)
			StartTime:  "10:00",
			EndTime:    "16:00",
			Enabled:    true,
		}

		_, err = timeService.CreateTimeRule(ctx, conflictingTimeRule)
		if err != nil {
			t.Fatalf("Failed to create conflicting time rule: %v", err)
		}

		// Validate again - should now detect conflicts
		validation, err = validationService.ValidateListRules(ctx, list.ID)
		if err != nil {
			t.Fatalf("Failed to validate rules: %v", err)
		}

		if len(validation.Conflicts) > 0 {
			t.Logf("Successfully detected %d conflicts after adding conflicting rule", len(validation.Conflicts))
			for _, conflict := range validation.Conflicts {
				t.Logf("  - Conflict: %s (%s)", conflict.Title, conflict.Severity)
			}
		}
	})

	t.Run("Bulk Operations", func(t *testing.T) {
		// Create a blacklist for testing bulk operations
		blacklistReq := CreateListRequest{
			Name:        "Gaming Blacklist",
			Type:        models.ListTypeBlacklist,
			Description: "Blocked gaming applications",
			Enabled:     true,
		}

		blacklist, err := listService.CreateList(ctx, blacklistReq)
		if err != nil {
			t.Fatalf("Failed to create blacklist: %v", err)
		}

		// Test bulk entry creation
		bulkEntries := []CreateEntryRequest{
			{
				ListID:      blacklist.ID,
				EntryType:   models.EntryTypeExecutable,
				Pattern:     "steam.exe",
				PatternType: models.PatternTypeExact,
				Enabled:     true,
			},
			{
				ListID:      blacklist.ID,
				EntryType:   models.EntryTypeExecutable,
				Pattern:     "discord.exe",
				PatternType: models.PatternTypeExact,
				Enabled:     true,
			},
			{
				ListID:      blacklist.ID,
				EntryType:   models.EntryTypeURL,
				Pattern:     "twitch.tv",
				PatternType: models.PatternTypeDomain,
				Enabled:     true,
			},
		}

		bulkResult, err := entryService.BulkCreateEntries(ctx, BulkCreateEntriesRequest{
			ListID:  blacklist.ID,
			Entries: bulkEntries,
		})
		if err != nil {
			t.Fatalf("Failed to bulk create entries: %v", err)
		}

		t.Logf("Bulk creation result: %d successful, %d failed",
			bulkResult.SuccessCount, bulkResult.FailureCount)

		// Test text import
		textData := []byte(`notepad++.exe
firefox.exe
# This is a comment
chrome.exe`)

		importResult, err := entryService.ImportEntries(ctx, blacklist.ID, textData, ExportFormatTXT)
		if err != nil {
			t.Fatalf("Failed to import entries: %v", err)
		}

		t.Logf("Import result: %d successful, %d failed",
			importResult.SuccessCount, importResult.FailureCount)

		// Test export
		exportData, err := entryService.ExportEntries(ctx, blacklist.ID, ExportFormatTXT)
		if err != nil {
			t.Fatalf("Failed to export entries: %v", err)
		}

		t.Logf("Exported data (%d bytes):\n%s", len(exportData), string(exportData))
	})

	t.Run("System-wide Validation", func(t *testing.T) {
		// Test system-wide validation
		systemValidation, err := validationService.ValidateAllRules(ctx)
		if err != nil {
			t.Fatalf("Failed to validate all rules: %v", err)
		}

		t.Logf("System-wide validation result: %v", systemValidation.IsValid)
		t.Logf("Found %d conflicts, %d warnings",
			len(systemValidation.Conflicts), len(systemValidation.Warnings))

		for _, suggestion := range systemValidation.Suggestions {
			t.Logf("Suggestion: %s", suggestion)
		}
	})
}

// TestMilestone3ServiceIntegration tests the integration between services
func TestMilestone3ServiceIntegration(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewDefault()

	// Mock repositories would be implemented in testutil package
	repos := &models.RepositoryManager{
		List:      testutil.NewMockListRepository(),
		ListEntry: testutil.NewMockListEntryRepository(),
		TimeRule:  testutil.NewMockTimeRuleRepository(),
	}

	listService := NewListManagementService(repos, logger)
	entryService := NewEntryManagementService(repos, logger)
	timeService := NewTimeWindowService(repos, logger)

	t.Run("Cross-Service Validation", func(t *testing.T) {
		// Create a list
		list, err := listService.CreateList(ctx, CreateListRequest{
			Name: "Test List",
			Type: models.ListTypeWhitelist,
		})
		if err != nil {
			t.Fatalf("Failed to create list: %v", err)
		}

		// Add entry to the list
		_, err = entryService.CreateEntry(ctx, CreateEntryRequest{
			ListID:      list.ID,
			EntryType:   models.EntryTypeURL,
			Pattern:     "example.com",
			PatternType: models.PatternTypeExact,
			Enabled:     true,
		})
		if err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}

		// Add time rule to the list
		_, err = timeService.CreateTimeRule(ctx, CreateTimeRuleRequest{
			ListID:     list.ID,
			Name:       "Test Rule",
			RuleType:   models.RuleTypeAllowDuring,
			DaysOfWeek: []int{1, 2, 3, 4, 5},
			StartTime:  "09:00",
			EndTime:    "17:00",
			Enabled:    true,
		})
		if err != nil {
			t.Fatalf("Failed to create time rule: %v", err)
		}

		// Delete the list should cascade to entries and time rules
		err = listService.DeleteList(ctx, list.ID)
		if err != nil {
			t.Fatalf("Failed to delete list: %v", err)
		}

		t.Log("Successfully tested cross-service integration")
	})
}

// TestMilestone3CompletionCriteria tests the milestone completion criteria
func TestMilestone3CompletionCriteria(t *testing.T) {
	// This test would verify all the acceptance criteria from the milestone
	ctx := context.Background()
	logger := logging.NewDefault()

	repos := &models.RepositoryManager{
		List:       testutil.NewMockListRepository(),
		ListEntry:  testutil.NewMockListEntryRepository(),
		TimeRule:   testutil.NewMockTimeRuleRepository(),
		QuotaRule:  testutil.NewMockQuotaRuleRepository(),
		QuotaUsage: testutil.NewMockQuotaUsageRepository(),
	}

	listService := NewListManagementService(repos, logger)
	entryService := NewEntryManagementService(repos, logger)
	timeService := NewTimeWindowService(repos, logger)
	quotaService := NewQuotaService(repos, logger)
	validationService := NewRuleValidationService(repos, logger)

	// Test all acceptance criteria
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "Whitelist and blacklist creation works",
			testFunc: func(t *testing.T) {
				// Test whitelist
				whitelist, err := listService.CreateList(ctx, CreateListRequest{
					Name: "Test Whitelist",
					Type: models.ListTypeWhitelist,
				})
				if err != nil || whitelist.Type != models.ListTypeWhitelist {
					t.Errorf("Failed to create whitelist")
				}

				// Test blacklist
				blacklist, err := listService.CreateList(ctx, CreateListRequest{
					Name: "Test Blacklist",
					Type: models.ListTypeBlacklist,
				})
				if err != nil || blacklist.Type != models.ListTypeBlacklist {
					t.Errorf("Failed to create blacklist")
				}
			},
		},
		{
			name: "Application and URL entries can be added/removed",
			testFunc: func(t *testing.T) {
				list, _ := listService.CreateList(ctx, CreateListRequest{
					Name: "Entry Test List",
					Type: models.ListTypeWhitelist,
				})

				// Test URL entry
				urlEntry, err := entryService.CreateEntry(ctx, CreateEntryRequest{
					ListID:      list.ID,
					EntryType:   models.EntryTypeURL,
					Pattern:     "https://example.com",
					PatternType: models.PatternTypeExact,
					Enabled:     true,
				})
				if err != nil {
					t.Errorf("Failed to create URL entry: %v", err)
				}

				// Test executable entry
				exeEntry, err := entryService.CreateEntry(ctx, CreateEntryRequest{
					ListID:      list.ID,
					EntryType:   models.EntryTypeExecutable,
					Pattern:     "notepad.exe",
					PatternType: models.PatternTypeExact,
					Enabled:     true,
				})
				if err != nil {
					t.Errorf("Failed to create executable entry: %v", err)
				}

				// Test removal
				err = entryService.DeleteEntry(ctx, urlEntry.ID)
				if err != nil {
					t.Errorf("Failed to delete URL entry: %v", err)
				}

				err = entryService.DeleteEntry(ctx, exeEntry.ID)
				if err != nil {
					t.Errorf("Failed to delete executable entry: %v", err)
				}
			},
		},
		{
			name: "Time windows support days of week and hours",
			testFunc: func(t *testing.T) {
				list, _ := listService.CreateList(ctx, CreateListRequest{
					Name: "Time Test List",
					Type: models.ListTypeWhitelist,
				})

				timeRule, err := timeService.CreateTimeRule(ctx, CreateTimeRuleRequest{
					ListID:     list.ID,
					Name:       "Weekday Morning",
					RuleType:   models.RuleTypeAllowDuring,
					DaysOfWeek: []int{1, 2, 3, 4, 5}, // Monday to Friday
					StartTime:  "08:00",
					EndTime:    "12:00",
					Enabled:    true,
				})
				if err != nil {
					t.Errorf("Failed to create time rule: %v", err)
				}

				// Test if rule is active during specified time
				testTime := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC) // Monday 10:00 AM
				isActive := timeService.IsRuleActiveAt(timeRule, testTime)
				if !isActive {
					t.Errorf("Expected rule to be active on Monday at 10:00 AM")
				}

				// Test if rule is inactive outside specified time
				testTime = time.Date(2024, 1, 8, 14, 0, 0, 0, time.UTC) // Monday 2:00 PM
				isActive = timeService.IsRuleActiveAt(timeRule, testTime)
				if isActive {
					t.Errorf("Expected rule to be inactive on Monday at 2:00 PM")
				}
			},
		},
		{
			name: "Daily, weekly, and monthly quotas function",
			testFunc: func(t *testing.T) {
				list, _ := listService.CreateList(ctx, CreateListRequest{
					Name: "Quota Test List",
					Type: models.ListTypeWhitelist,
				})

				// Test daily quota
				dailyQuota, err := quotaService.CreateQuotaRule(ctx, CreateQuotaRuleRequest{
					ListID:       list.ID,
					Name:         "Daily Limit",
					QuotaType:    models.QuotaTypeDaily,
					LimitSeconds: 3600, // 1 hour
					Enabled:      true,
				})
				if err != nil || dailyQuota.QuotaType != models.QuotaTypeDaily {
					t.Errorf("Failed to create daily quota")
				}

				// Test weekly quota
				weeklyQuota, err := quotaService.CreateQuotaRule(ctx, CreateQuotaRuleRequest{
					ListID:       list.ID,
					Name:         "Weekly Limit",
					QuotaType:    models.QuotaTypeWeekly,
					LimitSeconds: 25200, // 7 hours
					Enabled:      true,
				})
				if err != nil || weeklyQuota.QuotaType != models.QuotaTypeWeekly {
					t.Errorf("Failed to create weekly quota")
				}

				// Test monthly quota
				monthlyQuota, err := quotaService.CreateQuotaRule(ctx, CreateQuotaRuleRequest{
					ListID:       list.ID,
					Name:         "Monthly Limit",
					QuotaType:    models.QuotaTypeMonthly,
					LimitSeconds: 108000, // 30 hours
					Enabled:      true,
				})
				if err != nil || monthlyQuota.QuotaType != models.QuotaTypeMonthly {
					t.Errorf("Failed to create monthly quota")
				}
			},
		},
		{
			name: "Rule conflicts are detected and handled appropriately",
			testFunc: func(t *testing.T) {
				list, _ := listService.CreateList(ctx, CreateListRequest{
					Name: "Conflict Test List",
					Type: models.ListTypeWhitelist,
				})

				// Create conflicting time rules
				_, err := timeService.CreateTimeRule(ctx, CreateTimeRuleRequest{
					ListID:     list.ID,
					Name:       "Allow Rule",
					RuleType:   models.RuleTypeAllowDuring,
					DaysOfWeek: []int{1, 2, 3, 4, 5},
					StartTime:  "09:00",
					EndTime:    "17:00",
					Enabled:    true,
				})
				if err != nil {
					t.Errorf("Failed to create allow rule: %v", err)
				}

				_, err = timeService.CreateTimeRule(ctx, CreateTimeRuleRequest{
					ListID:     list.ID,
					Name:       "Block Rule",
					RuleType:   models.RuleTypeBlockDuring,
					DaysOfWeek: []int{1, 2, 3}, // Overlaps with allow rule
					StartTime:  "10:00",
					EndTime:    "16:00",
					Enabled:    true,
				})
				if err != nil {
					t.Errorf("Failed to create block rule: %v", err)
				}

				// Check for conflicts
				validation, err := validationService.ValidateListRules(ctx, list.ID)
				if err != nil {
					t.Errorf("Failed to validate rules: %v", err)
				}

				if len(validation.Conflicts) == 0 {
					t.Errorf("Expected to detect conflicts between opposing time rules")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.testFunc)
	}
}
