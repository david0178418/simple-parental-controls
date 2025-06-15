//go:build windows

package enforcement

import (
	"context"
	"testing"
	"time"
)

func TestWindowsProcessMonitor(t *testing.T) {
	monitor := NewWindowsProcessMonitor(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test process enumeration
	processes, err := monitor.GetProcesses(ctx)
	if err != nil {
		t.Fatalf("GetProcesses failed: %v", err)
	}

	if len(processes) == 0 {
		t.Error("Expected at least one process, got none")
	}

	// Validate process information
	for _, proc := range processes[:min(5, len(processes))] { // Check first 5 processes
		if proc.PID <= 0 {
			t.Errorf("Invalid PID for process: %d", proc.PID)
		}
		if proc.Name == "" {
			t.Errorf("Empty name for process PID %d", proc.PID)
		}
		t.Logf("Process: PID=%d, Name=%s, Path=%s", proc.PID, proc.Name, proc.Path)
	}

	// Test specific process lookup
	if len(processes) > 0 {
		firstProc := processes[0]
		foundProc, err := monitor.GetProcess(ctx, firstProc.PID)
		if err != nil {
			t.Errorf("GetProcess failed for PID %d: %v", firstProc.PID, err)
		} else if foundProc.PID != firstProc.PID {
			t.Errorf("GetProcess returned wrong process: expected %d, got %d", firstProc.PID, foundProc.PID)
		}
	}
}

func TestWindowsNetworkFilter(t *testing.T) {
	// Create a dummy process monitor for testing
	processMonitor := NewWindowsProcessMonitor(time.Second)

	filter := NewWindowsNetworkFilter(processMonitor)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test filter initialization
	if err := filter.Start(ctx); err != nil {
		t.Fatalf("Failed to start Windows network filter: %v", err)
	}
	defer filter.Stop()

	// Test rule addition
	rule := &FilterRule{
		ID:        "test-rule-1",
		Name:      "Test Block Rule",
		Action:    ActionBlock,
		Pattern:   "example.com",
		MatchType: MatchDomain,
		Priority:  100,
		Enabled:   true,
	}

	if err := filter.AddRule(rule); err != nil {
		t.Errorf("Failed to add rule: %v", err)
	}

	// Test rule removal
	if err := filter.RemoveRule(rule.ID); err != nil {
		t.Errorf("Failed to remove rule: %v", err)
	}

	// Test stats
	stats := filter.GetStats()
	if stats == nil {
		t.Error("GetStats returned nil")
	}

	// Test system info
	info := filter.GetSystemInfo()
	if info == nil {
		t.Error("GetSystemInfo returned nil")
	}

	platform, exists := info["platform"]
	if !exists || platform != "windows" {
		t.Errorf("Expected platform 'windows', got %v", platform)
	}

	t.Logf("Windows Network Filter Info: %+v", info)
}

func TestWindowsPlatformNetworkFilter(t *testing.T) {
	processMonitor := NewWindowsProcessMonitor(time.Second)
	filter := NewPlatformNetworkFilter(processMonitor)

	if filter == nil {
		t.Fatal("NewPlatformNetworkFilter returned nil")
	}

	// Verify it's actually a Windows filter
	if windowsFilter, ok := filter.(*WindowsNetworkFilter); !ok {
		t.Errorf("Expected WindowsNetworkFilter, got %T", filter)
	} else {
		info := windowsFilter.GetSystemInfo()
		if info["platform"] != "windows" {
			t.Errorf("Expected Windows platform, got %v", info["platform"])
		}
	}
}

func TestWindowsWFPEngineHandling(t *testing.T) {
	processMonitor := NewWindowsProcessMonitor(time.Second)
	filter := NewWindowsNetworkFilter(processMonitor)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test WFP engine opening/closing
	if err := filter.Start(ctx); err != nil {
		// WFP might fail in test environment without proper privileges
		t.Logf("WFP engine start failed (expected in test environment): %v", err)
		return
	}
	defer filter.Stop()

	// Check engine handle
	info := filter.GetSystemInfo()
	if engineHandle, exists := info["wfp_engine_handle"]; exists {
		t.Logf("WFP Engine Handle Active: %v", engineHandle)
	}

	// Test filter operations
	rule := &FilterRule{
		ID:        "wfp-test-rule",
		Name:      "WFP Test Rule",
		Action:    ActionBlock,
		Pattern:   "test.example.com",
		MatchType: MatchDomain,
		Priority:  100,
		Enabled:   true,
	}

	if err := filter.AddRule(rule); err != nil {
		t.Errorf("Failed to add WFP rule: %v", err)
	}

	// Verify filter was added
	info = filter.GetSystemInfo()
	if filterCount, exists := info["active_wfp_filters"]; exists {
		t.Logf("Active WFP Filters: %v", filterCount)
	}

	if err := filter.RemoveRule(rule.ID); err != nil {
		t.Errorf("Failed to remove WFP rule: %v", err)
	}
}

func TestWindowsCompatibilityFeatures(t *testing.T) {
	t.Run("ProcessMonitorCompatibility", func(t *testing.T) {
		// Test that the process monitor works with Windows APIs
		monitor := newPlatformProcessMonitor(time.Second)
		if monitor == nil {
			t.Fatal("newPlatformProcessMonitor returned nil")
		}

		// Verify it's a Windows implementation
		if _, ok := monitor.(*WindowsProcessMonitor); !ok {
			t.Errorf("Expected WindowsProcessMonitor, got %T", monitor)
		}
	})

	t.Run("NetworkFilterCompatibility", func(t *testing.T) {
		// Test that the network filter works with Windows APIs
		monitor := NewWindowsProcessMonitor(time.Second)
		filter := NewPlatformNetworkFilter(monitor)
		if filter == nil {
			t.Fatal("NewPlatformNetworkFilter returned nil")
		}

		// Verify it's a Windows implementation
		if _, ok := filter.(*WindowsNetworkFilter); !ok {
			t.Errorf("Expected WindowsNetworkFilter, got %T", filter)
		}
	})

	t.Run("BuildCompatibility", func(t *testing.T) {
		// Test that Windows-specific types compile correctly
		monitor := NewWindowsProcessMonitor(time.Second)
		filter := NewWindowsNetworkFilter(monitor)

		if monitor == nil {
			t.Error("Windows process monitor creation failed")
		}
		if filter == nil {
			t.Error("Windows network filter creation failed")
		}

		t.Logf("Windows compatibility test passed - all types created successfully")
	})
}

// Helper function for older Go versions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
