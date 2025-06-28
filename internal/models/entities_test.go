package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConfigModel(t *testing.T) {
	config := Config{
		ID:          1,
		Key:         "test_key",
		Value:       "test_value",
		Description: "Test configuration",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Errorf("Failed to marshal config to JSON: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Config
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal config from JSON: %v", err)
	}

	if unmarshaled.Key != config.Key {
		t.Errorf("Expected key %s, got %s", config.Key, unmarshaled.Key)
	}
}

func TestListTypes(t *testing.T) {
	tests := []struct {
		listType ListType
		valid    bool
	}{
		{ListTypeWhitelist, true},
		{ListTypeBlacklist, true},
		{ListType("invalid"), false},
	}

	for _, tt := range tests {
		switch tt.listType {
		case ListTypeWhitelist, ListTypeBlacklist:
			if !tt.valid {
				t.Errorf("Expected %s to be valid", tt.listType)
			}
		default:
			if tt.valid {
				t.Errorf("Expected %s to be invalid", tt.listType)
			}
		}
	}
}

func TestListModel(t *testing.T) {
	list := List{
		ID:          1,
		Name:        "Test List",
		Type:        ListTypeWhitelist,
		Description: "Test whitelist",
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Entries: []ListEntry{
			{
				ID:          1,
				ListID:      1,
				EntryType:   EntryTypeURL,
				Pattern:     "example.com",
				PatternType: PatternTypeDomain,
				Enabled:     true,
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(list)
	if err != nil {
		t.Errorf("Failed to marshal list to JSON: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled List
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal list from JSON: %v", err)
	}

	if unmarshaled.Name != list.Name {
		t.Errorf("Expected name %s, got %s", list.Name, unmarshaled.Name)
	}

	if len(unmarshaled.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(unmarshaled.Entries))
	}
}

func TestEntryTypes(t *testing.T) {
	tests := []struct {
		entryType EntryType
		valid     bool
	}{
		{EntryTypeExecutable, true},
		{EntryTypeURL, true},
		{EntryType("invalid"), false},
	}

	for _, tt := range tests {
		switch tt.entryType {
		case EntryTypeExecutable, EntryTypeURL:
			if !tt.valid {
				t.Errorf("Expected %s to be valid", tt.entryType)
			}
		default:
			if tt.valid {
				t.Errorf("Expected %s to be invalid", tt.entryType)
			}
		}
	}
}

func TestPatternTypes(t *testing.T) {
	tests := []struct {
		patternType PatternType
		valid       bool
	}{
		{PatternTypeExact, true},
		{PatternTypeWildcard, true},
		{PatternTypeDomain, true},
		{PatternType("invalid"), false},
	}

	for _, tt := range tests {
		switch tt.patternType {
		case PatternTypeExact, PatternTypeWildcard, PatternTypeDomain:
			if !tt.valid {
				t.Errorf("Expected %s to be valid", tt.patternType)
			}
		default:
			if tt.valid {
				t.Errorf("Expected %s to be invalid", tt.patternType)
			}
		}
	}
}

func TestTimeRuleDaysOfWeek(t *testing.T) {
	rule := TimeRule{
		ID:         1,
		ListID:     1,
		Name:       "Test Rule",
		RuleType:   RuleTypeAllowDuring,
		DaysOfWeek: []int{1, 2, 3, 4, 5}, // Monday to Friday
		StartTime:  "09:00",
		EndTime:    "17:00",
		Enabled:    true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Test marshal days of week
	daysJSON, err := rule.MarshalDaysOfWeek()
	if err != nil {
		t.Errorf("Failed to marshal days of week: %v", err)
	}

	// Test unmarshal days of week
	var newRule TimeRule
	if err := newRule.UnmarshalDaysOfWeek(daysJSON); err != nil {
		t.Errorf("Failed to unmarshal days of week: %v", err)
	}

	if len(newRule.DaysOfWeek) != 5 {
		t.Errorf("Expected 5 days, got %d", len(newRule.DaysOfWeek))
	}

	for i, day := range newRule.DaysOfWeek {
		if day != rule.DaysOfWeek[i] {
			t.Errorf("Expected day %d, got %d at index %d", rule.DaysOfWeek[i], day, i)
		}
	}
}

func TestValidateTimeFormat(t *testing.T) {
	tests := []struct {
		timeStr string
		valid   bool
	}{
		{"09:00", true},
		{"23:59", true},
		{"00:00", true},
		{"12:30", true},
		{"24:00", false},
		{"9:0", false},   // Invalid format
		{"09:60", false}, // Invalid minutes
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		err := ValidateTimeFormat(tt.timeStr)
		if tt.valid && err != nil {
			t.Errorf("Expected %s to be valid, got error: %v", tt.timeStr, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("Expected %s to be invalid, got no error", tt.timeStr)
		}
	}
}

func TestQuotaRuleDuration(t *testing.T) {
	rule := QuotaRule{
		ID:           1,
		ListID:       1,
		Name:         "Daily Limit",
		QuotaType:    QuotaTypeDaily,
		LimitSeconds: 3600, // 1 hour
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	duration := rule.GetLimitDuration()
	expected := time.Hour

	if duration != expected {
		t.Errorf("Expected duration %v, got %v", expected, duration)
	}
}

func TestQuotaUsageCalculations(t *testing.T) {
	usage := QuotaUsage{
		ID:          1,
		QuotaRuleID: 1,
		PeriodStart: time.Now().Add(-24 * time.Hour),
		PeriodEnd:   time.Now(),
		UsedSeconds: 1800, // 30 minutes
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test used duration
	usedDuration := usage.GetUsedDuration()
	expected := 30 * time.Minute

	if usedDuration != expected {
		t.Errorf("Expected used duration %v, got %v", expected, usedDuration)
	}

	// Test remaining seconds
	limitSeconds := 3600 // 1 hour
	remaining := usage.RemainingSeconds(limitSeconds)
	expectedRemaining := 1800 // 30 minutes

	if remaining != expectedRemaining {
		t.Errorf("Expected remaining %d seconds, got %d", expectedRemaining, remaining)
	}

	// Test exceeded quota
	usage.UsedSeconds = 3700 // More than limit
	remaining = usage.RemainingSeconds(limitSeconds)
	if remaining != 0 {
		t.Errorf("Expected 0 remaining when exceeded, got %d", remaining)
	}
}

func TestAuditLogDetails(t *testing.T) {
	log := AuditLog{
		ID:          1,
		Timestamp:   time.Now(),
		EventType:   "access_attempt",
		TargetType:  TargetTypeURL,
		TargetValue: "example.com",
		Action:      ActionTypeBlock,
		CreatedAt:   time.Now(),
	}

	// Test setting details
	details := map[string]interface{}{
		"reason":  "blocked by rule",
		"rule_id": 123,
	}

	if err := log.SetDetailsMap(details); err != nil {
		t.Errorf("Failed to set details: %v", err)
	}

	// Test getting details
	retrieved, err := log.GetDetailsMap()
	if err != nil {
		t.Errorf("Failed to get details: %v", err)
	}

	if retrieved["reason"] != "blocked by rule" {
		t.Errorf("Expected reason 'blocked by rule', got %v", retrieved["reason"])
	}

	// Test with nil details
	if err := log.SetDetailsMap(nil); err != nil {
		t.Errorf("Failed to set nil details: %v", err)
	}

	if log.Details != "" {
		t.Errorf("Expected empty details, got %s", log.Details)
	}

	// Test empty details
	log.Details = ""
	retrieved, err = log.GetDetailsMap()
	if err != nil {
		t.Errorf("Failed to get empty details: %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected empty map, got %v", retrieved)
	}
}

func TestValidationErrors(t *testing.T) {
	var errors ValidationErrors

	// Test empty errors
	if errors.HasErrors() {
		t.Error("Expected no errors initially")
	}

	if errors.Error() != "" {
		t.Errorf("Expected empty error string, got %s", errors.Error())
	}

	// Test adding errors
	errors.Add("field1", "error1")
	errors.Add("field2", "error2")

	if !errors.HasErrors() {
		t.Error("Expected to have errors")
	}

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}

	// Test single error
	singleError := ValidationErrors{
		{Field: "test", Message: "test error"},
	}

	errorStr := singleError.Error()
	expected := "validation error on field 'test': test error"
	if errorStr != expected {
		t.Errorf("Expected error string %s, got %s", expected, errorStr)
	}

	// Test multiple errors
	multiError := ValidationErrors{
		{Field: "field1", Message: "error1"},
		{Field: "field2", Message: "error2"},
	}

	errorStr = multiError.Error()
	if errorStr != "multiple validation errors: 2 errors" {
		t.Errorf("Unexpected multiple error string: %s", errorStr)
	}
}

func TestDefaultSearchFilters(t *testing.T) {
	filters := DefaultSearchFilters()

	if filters.Limit != 50 {
		t.Errorf("Expected default limit 50, got %d", filters.Limit)
	}

	if filters.Offset != 0 {
		t.Errorf("Expected default offset 0, got %d", filters.Offset)
	}

	if filters.Enabled != nil {
		t.Error("Expected enabled filter to be nil by default")
	}
}

func TestJSONMarshaling(t *testing.T) {
	// Test all major entity types can be marshaled/unmarshaled
	entities := []interface{}{
		&Config{ID: 1, Key: "test", Value: "value"},
		&List{ID: 1, Name: "test", Type: ListTypeWhitelist, Enabled: true},
		&ListEntry{ID: 1, ListID: 1, EntryType: EntryTypeURL, Pattern: "test.com", PatternType: PatternTypeDomain, Enabled: true},
		&TimeRule{ID: 1, ListID: 1, Name: "test", RuleType: RuleTypeAllowDuring, DaysOfWeek: []int{1, 2}, StartTime: "09:00", EndTime: "17:00", Enabled: true},
		&QuotaRule{ID: 1, ListID: 1, Name: "test", QuotaType: QuotaTypeDaily, LimitSeconds: 3600, Enabled: true},
		&QuotaUsage{ID: 1, QuotaRuleID: 1, PeriodStart: time.Now(), PeriodEnd: time.Now(), UsedSeconds: 1800},
		&AuditLog{ID: 1, EventType: "test", TargetType: TargetTypeURL, TargetValue: "test.com", Action: ActionTypeAllow},
		&DashboardStats{TotalLists: 5, TotalEntries: 10, ActiveRules: 3},
	}

	for i, entity := range entities {
		data, err := json.Marshal(entity)
		if err != nil {
			t.Errorf("Failed to marshal entity %d: %v", i, err)
			continue
		}

		// Try to unmarshal back (into interface{} to test general structure)
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("Failed to unmarshal entity %d: %v", i, err)
		}
	}
}
