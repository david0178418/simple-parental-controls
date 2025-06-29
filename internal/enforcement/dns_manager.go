package enforcement

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"parental-control/internal/logging"
	"parental-control/internal/privilege"
)

// DNSManager handles system-level DNS configuration changes via iptables.
type DNSManager struct {
	logger logging.Logger
}

// NewDNSManager creates a new DNSManager.
func NewDNSManager(logger logging.Logger) *DNSManager {
	return &DNSManager{
		logger: logger,
	}
}

// Setup redirects system DNS traffic to the local listener.
func (m *DNSManager) Setup() error {
	if !privilege.IsElevated() {
		return fmt.Errorf("DNS redirection requires elevated privileges")
	}

	m.logger.Info("Setting up DNS redirection using iptables...")

	rules := [][]string{
		// Redirect outbound DNS UDP traffic to localhost, excluding traffic from the root user
		{"-t", "nat", "-A", "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "!", "--uid-owner", "0", "-j", "REDIRECT", "--to-ports", "53"},
		// Redirect outbound DNS TCP traffic to localhost, excluding traffic from the root user
		{"-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "!", "--uid-owner", "0", "-j", "REDIRECT", "--to-ports", "53"},
	}

	for _, rule := range rules {
		if err := m.runIptables(rule...); err != nil {
			// Try to clean up if one of the rules fails
			m.Teardown()
			return fmt.Errorf("failed to add iptables rule (%s): %w", strings.Join(rule, " "), err)
		}
	}

	m.logger.Info("Successfully set up DNS redirection.")
	return nil
}

// Teardown restores the original DNS settings by removing the iptables rules.
func (m *DNSManager) Teardown() error {
	if !privilege.IsElevated() {
		m.logger.Warn("Attempting to teardown DNS redirection without elevated privileges")
		return nil
	}

	m.logger.Info("Restoring original DNS settings by removing iptables rules...")

	rules := [][]string{
		// Remove the UDP redirection rule
		{"-t", "nat", "-D", "OUTPUT", "-p", "udp", "--dport", "53", "-m", "owner", "!", "--uid-owner", "0", "-j", "REDIRECT", "--to-ports", "53"},
		// Remove the TCP redirection rule
		{"-t", "nat", "-D", "OUTPUT", "-p", "tcp", "--dport", "53", "-m", "owner", "!", "--uid-owner", "0", "-j", "REDIRECT", "--to-ports", "53"},
	}

	var firstErr error
	for _, rule := range rules {
		if err := m.runIptables(rule...); err != nil {
			m.logger.Error("Failed to remove iptables rule", logging.Err(err), logging.String("rule", strings.Join(rule, " ")))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if firstErr != nil {
		m.logger.Error("Failed to restore original DNS settings completely.", logging.Err(firstErr))
		return firstErr
	}

	m.logger.Info("Successfully restored original DNS settings.")
	return nil
}

func (m *DNSManager) runIptables(args ...string) error {
	cmd := exec.Command("iptables", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables command failed: %s - %w", stderr.String(), err)
	}

	return nil
}
