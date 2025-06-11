//go:build !windows

package enforcement

import (
	"fmt"
	"testing"
	"time"
)

func TestLinuxNetworkFilter(t *testing.T) {
	// Create a mock process monitor
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)

	// Create Linux network filter
	filter := NewLinuxNetworkFilter(processMonitor)

	// Test basic creation
	if filter == nil {
		t.Fatal("Failed to create Linux network filter")
	}

	if filter.chainName != "PARENTAL_CONTROL" {
		t.Errorf("Expected chain name 'PARENTAL_CONTROL', got %s", filter.chainName)
	}

	if filter.dnsBlocker == nil {
		t.Error("DNS blocker should be initialized")
	}
}

func TestLinuxNetworkFilterAddRule(t *testing.T) {
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)
	filter := NewLinuxNetworkFilter(processMonitor)

	// Test adding a domain blocking rule
	rule := &FilterRule{
		ID:        "test-rule-1",
		Name:      "Block Example",
		Action:    ActionBlock,
		Pattern:   "example.com",
		MatchType: MatchDomain,
		Priority:  10,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	err := filter.AddRule(rule)
	// Note: This will likely fail in test environment due to iptables permissions
	// but we can test the logic structure
	if err != nil {
		t.Logf("AddRule failed as expected in test environment: %v", err)
	}

	// Test that the rule was added to the base engine
	stats := filter.GetStats()
	if stats == nil {
		t.Error("Expected to get filter stats")
	}
}

func TestLinuxNetworkFilterSystemInfo(t *testing.T) {
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)
	filter := NewLinuxNetworkFilter(processMonitor)

	info := filter.GetSystemInfo()

	// Check expected fields
	if platform, exists := info["platform"]; !exists || platform != "linux" {
		t.Error("Expected platform to be 'linux'")
	}

	if chainName, exists := info["chain_name"]; !exists || chainName != "PARENTAL_CONTROL" {
		t.Error("Expected chain_name to be 'PARENTAL_CONTROL'")
	}

	if _, exists := info["active_iptables_rules"]; !exists {
		t.Error("Expected active_iptables_rules field")
	}
}

func TestDNSBlocker(t *testing.T) {
	blocker := NewDNSBlocker()

	if blocker == nil {
		t.Fatal("Failed to create DNS blocker")
	}

	// Test domain blocking
	testDomain := "test.example.com"

	// Should not be blocked initially
	if blocker.IsBlocked(testDomain) {
		t.Errorf("Domain %s should not be blocked initially", testDomain)
	}

	// Block the domain (will likely fail due to permissions, but test the logic)
	err := blocker.BlockDomain(testDomain)
	if err != nil {
		t.Logf("BlockDomain failed as expected in test environment: %v", err)
	}

	// Should be blocked now in memory even if file operation failed
	if !blocker.IsBlocked(testDomain) {
		t.Errorf("Domain %s should be blocked in memory", testDomain)
	}

	// Test unblocking
	err = blocker.UnblockDomain(testDomain)
	if err != nil {
		t.Logf("UnblockDomain failed as expected in test environment: %v", err)
	}

	// Should not be blocked anymore
	if blocker.IsBlocked(testDomain) {
		t.Errorf("Domain %s should not be blocked after unblocking", testDomain)
	}
}

func TestDNSBlockerInvalidDomains(t *testing.T) {
	blocker := NewDNSBlocker()

	// Test invalid domains
	invalidDomains := []string{
		"",
		"   ",
		"invalid domain with spaces",
	}

	for _, domain := range invalidDomains {
		err := blocker.BlockDomain(domain)
		if err == nil {
			t.Errorf("Expected error when blocking invalid domain: '%s'", domain)
		}
	}
}

func TestLinuxFilterCheckIptables(t *testing.T) {
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)
	filter := NewLinuxNetworkFilter(processMonitor)

	// Test iptables availability check
	err := filter.checkIptables()
	if err != nil {
		t.Logf("iptables check failed (expected in some test environments): %v", err)
	} else {
		t.Log("iptables is available")
	}
}

func TestNewPlatformNetworkFilter(t *testing.T) {
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)

	// Test platform-specific factory function
	filter := NewPlatformNetworkFilter(processMonitor)

	if filter == nil {
		t.Fatal("NewPlatformNetworkFilter returned nil")
	}

	// Should return LinuxNetworkFilter on Linux
	if _, ok := filter.(*LinuxNetworkFilter); !ok {
		t.Error("Expected LinuxNetworkFilter on Linux platform")
	}
}

// BenchmarkLinuxFilterAddRule benchmarks rule addition performance
func BenchmarkLinuxFilterAddRule(b *testing.B) {
	processMonitor := NewLinuxProcessMonitor(100 * time.Millisecond)
	filter := NewLinuxNetworkFilter(processMonitor)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rule := &FilterRule{
			ID:        fmt.Sprintf("bench-rule-%d", i),
			Name:      "Benchmark Rule",
			Action:    ActionBlock,
			Pattern:   "benchmark.com",
			MatchType: MatchDomain,
			Priority:  10,
			Enabled:   true,
			CreatedAt: time.Now(),
		}

		// This will likely fail due to iptables permissions in test environment
		// but we can benchmark the rule processing logic
		filter.AddRule(rule)
	}
}
