//go:build !windows

package enforcement

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// LinuxNetworkFilter implements network filtering for Linux using iptables
type LinuxNetworkFilter struct {
	*NetworkFilterEngine

	// iptables chain name for our rules
	chainName string

	// Active iptables rules
	activeRules   map[string]string
	activeRulesMu sync.RWMutex

	// Process monitor for application-specific filtering
	processMonitor ProcessMonitor

	// DNS blocker for enhanced domain filtering
	dnsBlocker *DNSBlocker
}

// NewLinuxNetworkFilter creates a new Linux network filter
func NewLinuxNetworkFilter(processMonitor ProcessMonitor) *LinuxNetworkFilter {
	return &LinuxNetworkFilter{
		NetworkFilterEngine: NewNetworkFilterEngine(),
		chainName:           "PARENTAL_CONTROL",
		activeRules:         make(map[string]string),
		processMonitor:      processMonitor,
		dnsBlocker:          NewDNSBlocker(),
	}
}

// Start starts the Linux network filter with iptables setup
func (lnf *LinuxNetworkFilter) Start(ctx context.Context) error {
	// Start the base engine
	if err := lnf.NetworkFilterEngine.Start(ctx); err != nil {
		return err
	}

	// Check if iptables is available
	if err := lnf.checkIptables(); err != nil {
		return fmt.Errorf("iptables not available: %w", err)
	}

	// Create our custom chain
	if err := lnf.createChain(); err != nil {
		return fmt.Errorf("failed to create iptables chain: %w", err)
	}

	// Insert our chain into the OUTPUT chain
	if err := lnf.insertChainRule(); err != nil {
		return fmt.Errorf("failed to insert chain rule: %w", err)
	}

	return nil
}

// Stop stops the Linux network filter and cleans up iptables rules
func (lnf *LinuxNetworkFilter) Stop() error {
	// Clean up iptables rules
	lnf.cleanupIptables()

	// Stop the base engine
	return lnf.NetworkFilterEngine.Stop()
}

// AddRule adds a filtering rule and creates corresponding iptables rules
func (lnf *LinuxNetworkFilter) AddRule(rule *FilterRule) error {
	// Add rule to the base engine
	if err := lnf.NetworkFilterEngine.AddRule(rule); err != nil {
		return err
	}

	// Create iptables rule if it's a blocking rule
	if rule.Action == ActionBlock && rule.Enabled {
		// For domain rules, also add DNS blocking
		if rule.MatchType == MatchDomain {
			if err := lnf.dnsBlocker.BlockDomain(rule.Pattern); err != nil {
				// DNS blocking failed, but continue with iptables
				// This provides fallback if DNS blocking isn't available
			}
		}

		if err := lnf.addIptablesRule(rule); err != nil {
			// Remove from base engine if iptables rule failed
			lnf.NetworkFilterEngine.RemoveRule(rule.ID)
			return fmt.Errorf("failed to add iptables rule: %w", err)
		}
	}

	return nil
}

// RemoveRule removes a filtering rule and its corresponding iptables rule
func (lnf *LinuxNetworkFilter) RemoveRule(ruleID string) error {
	// Remove iptables rule first
	lnf.removeIptablesRule(ruleID)

	// Remove from base engine
	return lnf.NetworkFilterEngine.RemoveRule(ruleID)
}

// checkIptables verifies that iptables is available and we have permissions
func (lnf *LinuxNetworkFilter) checkIptables() error {
	cmd := exec.Command("iptables", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables command not found or not accessible: %w", err)
	}

	// Test if we can list rules (check permissions)
	cmd = exec.Command("iptables", "-L", "-n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("insufficient permissions to access iptables: %w", err)
	}

	return nil
}

// createChain creates our custom iptables chain
func (lnf *LinuxNetworkFilter) createChain() error {
	// Create new chain
	cmd := exec.Command("iptables", "-t", "filter", "-N", lnf.chainName)
	if err := cmd.Run(); err != nil {
		// Chain might already exist, check if it does
		if !strings.Contains(err.Error(), "Chain already exists") {
			return fmt.Errorf("failed to create chain: %w", err)
		}
	}

	// Flush any existing rules in our chain
	cmd = exec.Command("iptables", "-t", "filter", "-F", lnf.chainName)
	return cmd.Run()
}

// insertChainRule inserts our chain into the OUTPUT chain
func (lnf *LinuxNetworkFilter) insertChainRule() error {
	// Check if rule already exists
	checkCmd := exec.Command("iptables", "-t", "filter", "-C", "OUTPUT", "-j", lnf.chainName)
	if checkCmd.Run() == nil {
		// Rule already exists
		return nil
	}

	// Insert our chain at the beginning of OUTPUT chain
	cmd := exec.Command("iptables", "-t", "filter", "-I", "OUTPUT", "1", "-j", lnf.chainName)
	return cmd.Run()
}

// addIptablesRule creates an iptables rule for blocking
func (lnf *LinuxNetworkFilter) addIptablesRule(rule *FilterRule) error {
	var args []string

	switch rule.MatchType {
	case MatchDomain:
		// For domain blocking, we'll use string matching on HTTP requests
		// This is a simplified approach - full implementation would need DNS interception
		args = []string{
			"-t", "filter",
			"-A", lnf.chainName,
			"-p", "tcp",
			"--dport", "80",
			"-m", "string",
			"--string", rule.Pattern,
			"--algo", "bm",
			"-j", "REJECT",
			"--reject-with", "tcp-reset",
		}

		// Add HTTPS blocking too
		httpsArgs := []string{
			"-t", "filter",
			"-A", lnf.chainName,
			"-p", "tcp",
			"--dport", "443",
			"-m", "string",
			"--string", rule.Pattern,
			"--algo", "bm",
			"-j", "REJECT",
			"--reject-with", "tcp-reset",
		}

		// Execute HTTP rule
		cmd := exec.Command("iptables", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add HTTP iptables rule: %w", err)
		}

		// Execute HTTPS rule
		cmd = exec.Command("iptables", httpsArgs...)
		if err := cmd.Run(); err != nil {
			// Clean up HTTP rule if HTTPS fails
			lnf.removeSpecificIptablesRule(args)
			return fmt.Errorf("failed to add HTTPS iptables rule: %w", err)
		}

		// Store the rule for cleanup
		lnf.activeRulesMu.Lock()
		lnf.activeRules[rule.ID] = strings.Join(args, " ")
		lnf.activeRulesMu.Unlock()

	case MatchExact:
		// For exact URL blocking, we could use more specific string matching
		// This is simplified - full implementation would need better URL parsing
		args = []string{
			"-t", "filter",
			"-A", lnf.chainName,
			"-m", "string",
			"--string", rule.Pattern,
			"--algo", "bm",
			"-j", "REJECT",
		}

		cmd := exec.Command("iptables", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add iptables rule: %w", err)
		}

		lnf.activeRulesMu.Lock()
		lnf.activeRules[rule.ID] = strings.Join(args, " ")
		lnf.activeRulesMu.Unlock()

	default:
		return fmt.Errorf("unsupported match type for iptables: %s", rule.MatchType)
	}

	return nil
}

// removeIptablesRule removes an iptables rule
func (lnf *LinuxNetworkFilter) removeIptablesRule(ruleID string) {
	lnf.activeRulesMu.Lock()
	defer lnf.activeRulesMu.Unlock()

	if ruleArgs, exists := lnf.activeRules[ruleID]; exists {
		// Convert -A to -D to delete the rule
		deleteArgs := strings.Replace(ruleArgs, "-A "+lnf.chainName, "-D "+lnf.chainName, 1)
		args := strings.Fields(deleteArgs)

		cmd := exec.Command("iptables", args...)
		cmd.Run() // Ignore errors during cleanup

		delete(lnf.activeRules, ruleID)
	}
}

// removeSpecificIptablesRule removes a specific iptables rule by converting add to delete
func (lnf *LinuxNetworkFilter) removeSpecificIptablesRule(addArgs []string) {
	deleteArgs := make([]string, len(addArgs))
	copy(deleteArgs, addArgs)

	// Replace -A with -D
	for i, arg := range deleteArgs {
		if arg == "-A" && i+1 < len(deleteArgs) {
			deleteArgs[i] = "-D"
			break
		}
	}

	cmd := exec.Command("iptables", deleteArgs...)
	cmd.Run() // Ignore errors during cleanup
}

// cleanupIptables removes all our iptables rules and chain
func (lnf *LinuxNetworkFilter) cleanupIptables() {
	// Remove all rules from our chain
	lnf.activeRulesMu.Lock()
	for ruleID := range lnf.activeRules {
		lnf.removeIptablesRule(ruleID)
	}
	lnf.activeRulesMu.Unlock()

	// Remove our chain from OUTPUT
	cmd := exec.Command("iptables", "-t", "filter", "-D", "OUTPUT", "-j", lnf.chainName)
	cmd.Run() // Ignore errors

	// Flush our chain
	cmd = exec.Command("iptables", "-t", "filter", "-F", lnf.chainName)
	cmd.Run() // Ignore errors

	// Delete our chain
	cmd = exec.Command("iptables", "-t", "filter", "-X", lnf.chainName)
	cmd.Run() // Ignore errors
}

// GetSystemInfo returns information about the Linux filtering setup
func (lnf *LinuxNetworkFilter) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Check iptables version
	cmd := exec.Command("iptables", "--version")
	if output, err := cmd.Output(); err == nil {
		info["iptables_version"] = strings.TrimSpace(string(output))
	}

	// Count active rules
	lnf.activeRulesMu.RLock()
	info["active_iptables_rules"] = len(lnf.activeRules)
	lnf.activeRulesMu.RUnlock()

	info["chain_name"] = lnf.chainName
	info["platform"] = "linux"

	return info
}

// NewPlatformNetworkFilter creates a platform-specific network filter
func NewPlatformNetworkFilter(processMonitor ProcessMonitor) NetworkFilter {
	return NewLinuxNetworkFilter(processMonitor)
}
