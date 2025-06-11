package enforcement

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DNSBlocker handles DNS-level domain blocking
type DNSBlocker struct {
	// Configuration
	hostsFile     string
	dnsmasqConfig string
	enabled       bool

	// Blocked domains
	blockedDomains map[string]bool
	domainsMu      sync.RWMutex

	// Statistics
	stats   *DNSBlockerStats
	statsMu sync.RWMutex

	// State management
	running   bool
	runningMu sync.RWMutex
}

// DNSBlockerStats holds DNS blocking statistics
type DNSBlockerStats struct {
	TotalQueries   int64     `json:"total_queries"`
	BlockedQueries int64     `json:"blocked_queries"`
	AllowedQueries int64     `json:"allowed_queries"`
	BlockedDomains int64     `json:"blocked_domains"`
	LastBlockTime  time.Time `json:"last_block_time"`
	LastQueryTime  time.Time `json:"last_query_time"`
}

// NewDNSBlocker creates a new DNS blocker
func NewDNSBlocker() *DNSBlocker {
	return &DNSBlocker{
		hostsFile:      "/etc/hosts",
		dnsmasqConfig:  "/etc/dnsmasq.d/parental-control.conf",
		enabled:        true,
		blockedDomains: make(map[string]bool),
		stats:          &DNSBlockerStats{},
	}
}

// Start starts the DNS blocker
func (db *DNSBlocker) Start(ctx context.Context) error {
	db.runningMu.Lock()
	defer db.runningMu.Unlock()

	if db.running {
		return fmt.Errorf("DNS blocker already running")
	}

	if !db.enabled {
		return fmt.Errorf("DNS blocker is disabled")
	}

	// Check if we can write to hosts file or dnsmasq config
	if err := db.checkPermissions(); err != nil {
		return fmt.Errorf("insufficient permissions for DNS blocking: %w", err)
	}

	db.running = true
	return nil
}

// Stop stops the DNS blocker
func (db *DNSBlocker) Stop() error {
	db.runningMu.Lock()
	defer db.runningMu.Unlock()

	if !db.running {
		return nil
	}

	// Clean up any DNS blocking rules
	db.cleanupDNSBlocking()

	db.running = false
	return nil
}

// BlockDomain adds a domain to the DNS block list
func (db *DNSBlocker) BlockDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("invalid domain")
	}

	db.domainsMu.Lock()
	db.blockedDomains[domain] = true
	db.domainsMu.Unlock()

	return db.applyDNSBlock(domain)
}

// UnblockDomain removes a domain from the DNS block list
func (db *DNSBlocker) UnblockDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	db.domainsMu.Lock()
	delete(db.blockedDomains, domain)
	db.domainsMu.Unlock()

	return db.removeDNSBlock(domain)
}

// IsBlocked checks if a domain is blocked
func (db *DNSBlocker) IsBlocked(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))

	db.domainsMu.RLock()
	defer db.domainsMu.RUnlock()

	return db.blockedDomains[domain]
}

// GetBlockedDomains returns list of blocked domains
func (db *DNSBlocker) GetBlockedDomains() []string {
	db.domainsMu.RLock()
	defer db.domainsMu.RUnlock()

	domains := make([]string, 0, len(db.blockedDomains))
	for domain := range db.blockedDomains {
		domains = append(domains, domain)
	}

	return domains
}

// GetStats returns DNS blocking statistics
func (db *DNSBlocker) GetStats() *DNSBlockerStats {
	db.statsMu.RLock()
	defer db.statsMu.RUnlock()

	stats := *db.stats
	return &stats
}

// ProcessDNSQuery processes a DNS query and updates statistics
func (db *DNSBlocker) ProcessDNSQuery(domain string) bool {
	db.statsMu.Lock()
	db.stats.TotalQueries++
	db.stats.LastQueryTime = time.Now()
	db.statsMu.Unlock()

	if db.IsBlocked(domain) {
		db.statsMu.Lock()
		db.stats.BlockedQueries++
		db.stats.LastBlockTime = time.Now()
		db.statsMu.Unlock()
		return true // Blocked
	}

	db.statsMu.Lock()
	db.stats.AllowedQueries++
	db.statsMu.Unlock()
	return false // Allowed
}

// isRunning checks if DNS blocker is running
func (db *DNSBlocker) isRunning() bool {
	db.runningMu.RLock()
	defer db.runningMu.RUnlock()
	return db.running
}

// checkPermissions verifies DNS blocking permissions
func (db *DNSBlocker) checkPermissions() error {
	// Try to check if we can write to hosts file
	cmd := exec.Command("test", "-w", db.hostsFile)
	if err := cmd.Run(); err != nil {
		// Try alternative approach with dnsmasq
		return db.checkDnsmasqPermissions()
	}

	return nil
}

// checkDnsmasqPermissions checks if we can use dnsmasq for DNS blocking
func (db *DNSBlocker) checkDnsmasqPermissions() error {
	// Check if dnsmasq is available
	cmd := exec.Command("which", "dnsmasq")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dnsmasq not available and cannot write to hosts file")
	}

	// Check if we can write to dnsmasq config directory
	cmd = exec.Command("test", "-w", "/etc/dnsmasq.d/")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot write to dnsmasq config directory")
	}

	return nil
}

// applyDNSBlock applies DNS blocking for a domain using hosts file
func (db *DNSBlocker) applyDNSBlock(domain string) error {
	entry := fmt.Sprintf("127.0.0.1 %s", domain)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' >> %s", entry, db.hostsFile))
	return cmd.Run()
}

// removeDNSBlock removes DNS blocking for a domain
func (db *DNSBlocker) removeDNSBlock(domain string) error {
	cmd := exec.Command("sed", "-i", fmt.Sprintf("/\\b%s\\b/d", domain), db.hostsFile)
	return cmd.Run()
}

// cleanupDNSBlocking removes all DNS blocking rules
func (db *DNSBlocker) cleanupDNSBlocking() {
	db.domainsMu.RLock()
	domains := make([]string, 0, len(db.blockedDomains))
	for domain := range db.blockedDomains {
		domains = append(domains, domain)
	}
	db.domainsMu.RUnlock()

	// Remove all blocked domains
	for _, domain := range domains {
		db.removeDNSBlock(domain)
	}

	// Clear memory
	db.domainsMu.Lock()
	db.blockedDomains = make(map[string]bool)
	db.domainsMu.Unlock()
}

// SetEnabled enables or disables DNS blocking
func (db *DNSBlocker) SetEnabled(enabled bool) {
	db.enabled = enabled
}

// IsEnabled returns whether DNS blocking is enabled
func (db *DNSBlocker) IsEnabled() bool {
	return db.enabled
}

// FlushDNSCache flushes the system DNS cache
func (db *DNSBlocker) FlushDNSCache() error {
	// Try different methods to flush DNS cache on Linux
	commands := [][]string{
		{"systemctl", "flush-dns"},
		{"systemd-resolve", "--flush-caches"},
		{"service", "systemd-resolved", "restart"},
		{"service", "dnsmasq", "restart"},
		{"nscd", "-i", "hosts"},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to flush DNS cache")
}

// ValidateDomain validates a domain name
func (db *DNSBlocker) ValidateDomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Basic domain validation
	if strings.Contains(domain, " ") {
		return false
	}

	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}

	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
	}

	return true
}
