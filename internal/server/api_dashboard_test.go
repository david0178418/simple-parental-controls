package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewDashboardAPIServer(t *testing.T) {
	api := NewDashboardAPIServer()
	if api == nil {
		t.Fatal("NewDashboardAPIServer returned nil")
	}

	if api.startTime.IsZero() {
		t.Error("Start time was not set")
	}

	// Verify start time is recent (within 1 second)
	if time.Since(api.startTime) > time.Second {
		t.Error("Start time is not recent")
	}
}

func TestHandleDashboardStats(t *testing.T) {
	api := NewDashboardAPIServer()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		checkBody      bool
	}{
		{
			name:           "valid GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
		{
			name:           "invalid POST request",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			checkBody:      false,
		},
		{
			name:           "invalid PUT request",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
			checkBody:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/dashboard/stats", nil)
			w := httptest.NewRecorder()

			api.handleDashboardStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkBody {
				var stats DashboardStats
				err := json.Unmarshal(w.Body.Bytes(), &stats)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Verify expected mock data
				expectedStats := DashboardStats{
					TotalLists:      5,
					TotalEntries:    42,
					ActiveRules:     12,
					TodayBlocks:     15,
					TodayAllows:     128,
					QuotasNearLimit: 2,
				}

				if stats != expectedStats {
					t.Errorf("Expected stats %+v, got %+v", expectedStats, stats)
				}

				// Verify Content-Type header
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
				}
			}
		})
	}
}

func TestHandleSystemInfo(t *testing.T) {
	api := NewDashboardAPIServer()

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		checkBody      bool
	}{
		{
			name:           "valid GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
		{
			name:           "invalid POST request",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			checkBody:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/dashboard/system", nil)
			w := httptest.NewRecorder()

			api.handleSystemInfo(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkBody {
				var systemInfo SystemInfo
				err := json.Unmarshal(w.Body.Bytes(), &systemInfo)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Verify basic fields are present
				if systemInfo.Version == "" {
					t.Error("Version should not be empty")
				}
				if systemInfo.Platform == "" {
					t.Error("Platform should not be empty")
				}
				if systemInfo.GoVersion == "" {
					t.Error("GoVersion should not be empty")
				}
				if systemInfo.StartTime == "" {
					t.Error("StartTime should not be empty")
				}
				if systemInfo.Uptime == "" {
					t.Error("Uptime should not be empty")
				}
				if systemInfo.MemoryUsage == "" {
					t.Error("MemoryUsage should not be empty")
				}

				// Verify Content-Type header
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
				}

				// Verify start time is parseable
				if _, err := time.Parse(time.RFC3339, systemInfo.StartTime); err != nil {
					t.Errorf("StartTime is not valid RFC3339 format: %v", err)
				}
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds only",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "minutes only",
			duration: 5 * time.Minute,
			expected: "5m0s",
		},
		{
			name:     "hours only",
			duration: 2 * time.Hour,
			expected: "2h0m",
		},
		{
			name:     "complex duration",
			duration: 25*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "1d1h30m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    500,
			expected: "500 B",
		},
		{
			name:     "kilobytes",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    2621440, // 2.5 MB
			expected: "2.5 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1288490189, // ~1.2 GB
			expected: "1.2 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDashboardStatsJSONSerialization(t *testing.T) {
	stats := DashboardStats{
		TotalLists:      5,
		TotalEntries:    42,
		ActiveRules:     12,
		TodayBlocks:     15,
		TodayAllows:     128,
		QuotasNearLimit: 2,
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal DashboardStats: %v", err)
	}

	var unmarshaled DashboardStats
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal DashboardStats: %v", err)
	}

	if stats != unmarshaled {
		t.Errorf("Expected %+v, got %+v", stats, unmarshaled)
	}

	// Verify JSON field names
	expectedFields := []string{
		"total_lists", "total_entries", "active_rules",
		"today_blocks", "today_allows", "quotas_near_limit",
	}

	jsonStr := string(data)
	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON should contain field %s", field)
		}
	}
}

func TestSystemInfoJSONSerialization(t *testing.T) {
	systemInfo := SystemInfo{
		Uptime:            "2h30m",
		Version:           "1.0.0",
		Platform:          "linux/amd64",
		MemoryUsage:       "45.2 MB",
		CPUUsage:          "2.1%",
		ActiveConnections: 3,
		GoVersion:         "go1.21.0",
		StartTime:         time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(systemInfo)
	if err != nil {
		t.Fatalf("Failed to marshal SystemInfo: %v", err)
	}

	var unmarshaled SystemInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SystemInfo: %v", err)
	}

	if systemInfo != unmarshaled {
		t.Errorf("Expected %+v, got %+v", systemInfo, unmarshaled)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsAt(s, substr, 1))))
}

func containsAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	if start+len(substr) > len(s) {
		return containsAt(s, substr, start+1)
	}
	if s[start:start+len(substr)] == substr {
		return true
	}
	return containsAt(s, substr, start+1)
}
