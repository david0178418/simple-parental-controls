package enforcement

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"parental-control/internal/logging"

	"github.com/miekg/dns"
)

// DNSBlocker intercepts DNS queries and blocks requests based on rules.
type DNSBlocker struct {
	config  *DNSBlockerConfig
	logger  logging.Logger
	manager *DNSManager
	rules   map[string]*FilterRule
	rulesMu sync.RWMutex

	server4   *dns.Server
	server6   *dns.Server
	running   bool
	runningMu sync.RWMutex

	stats   DNSBlockerStats
	statsMu sync.Mutex
}

// DNSBlockerConfig holds configuration for the DNSBlocker.
type DNSBlockerConfig struct {
	ListenAddr    string        `json:"listen_addr"`
	BlockIPv4     string        `json:"block_ipv4"`
	BlockIPv6     string        `json:"block_ipv6"`
	UpstreamDNS   []string      `json:"upstream_dns"`
	CacheTTL      time.Duration `json:"cache_ttl"`
	EnableLogging bool          `json:"enable_logging"`
}

// DNSBlockerStats holds statistics about DNS blocking activities.
type DNSBlockerStats struct {
	TotalQueries    int64 `json:"total_queries"`
	BlockedQueries  int64 `json:"blocked_queries"`
	AllowedQueries  int64 `json:"allowed_queries"`
	UpstreamLookups int64 `json:"upstream_lookups"`
	CacheHits       int64 `json:"cache_hits"`
	Errors          int64 `json:"errors"`
}

// NewDNSBlocker creates a new DNSBlocker.
func NewDNSBlocker(config *DNSBlockerConfig, logger logging.Logger) (*DNSBlocker, error) {
	if config.ListenAddr == "" {
		config.ListenAddr = ":53"
	}
	if config.BlockIPv4 == "" {
		config.BlockIPv4 = "0.0.0.0"
	}
	if config.BlockIPv6 == "" {
		config.BlockIPv6 = "::"
	}
	if len(config.UpstreamDNS) == 0 {
		config.UpstreamDNS = []string{"8.8.8.8:53", "1.1.1.1:53"}
	}

	return &DNSBlocker{
		config:  config,
		logger:  logger,
		manager: NewDNSManager(logger),
		rules:   make(map[string]*FilterRule),
	}, nil
}

// Start starts the DNS blocker server.
func (b *DNSBlocker) Start(ctx context.Context) error {
	b.runningMu.Lock()
	if b.running {
		b.runningMu.Unlock()
		return fmt.Errorf("DNS blocker is already running")
	}

	if err := b.manager.Setup(); err != nil {
		b.logger.Error("Failed to set up DNS manager, running without automatic DNS configuration.", logging.Err(err))
	}

	dns.HandleFunc(".", b.handleDNSRequest)

	b.server4 = &dns.Server{Addr: b.config.ListenAddr, Net: "udp4"}
	b.server6 = &dns.Server{Addr: b.config.ListenAddr, Net: "udp6"}

	b.running = true
	b.runningMu.Unlock()

	b.logger.Info("Starting DNS blocker", logging.String("address", b.config.ListenAddr))

	go func() {
		if err := b.server6.ListenAndServe(); err != nil {
			b.runningMu.RLock()
			if b.running {
				b.logger.Error("IPv6 DNS blocker failed", logging.Err(err))
			}
			b.runningMu.RUnlock()
		}
	}()

	go func() {
		if err := b.server4.ListenAndServe(); err != nil {
			b.runningMu.RLock()
			if b.running {
				b.logger.Error("IPv4 DNS blocker failed", logging.Err(err))
			}
			b.runningMu.RUnlock()
		}
	}()

	return nil
}

// Stop stops the DNS blocker server.
func (b *DNSBlocker) Stop(ctx context.Context) error {
	b.runningMu.Lock()
	defer b.runningMu.Unlock()

	if !b.running {
		return nil
	}

	if err := b.manager.Teardown(); err != nil {
		b.logger.Error("Failed to tear down DNS manager", logging.Err(err))
	}

	b.running = false
	if b.server4 != nil {
		if err := b.server4.Shutdown(); err != nil {
			b.logger.Error("Error stopping IPv4 DNS blocker", logging.Err(err))
		}
	}
	if b.server6 != nil {
		if err := b.server6.Shutdown(); err != nil {
			b.logger.Error("Error stopping IPv6 DNS blocker", logging.Err(err))
		}
	}

	b.logger.Info("DNS blocker stopped")
	return nil
}

// AddRule adds a filtering rule.
func (b *DNSBlocker) AddRule(rule *FilterRule) error {
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	b.rules[rule.Pattern] = rule
	if b.config.EnableLogging {
		b.logger.Debug("Added DNS rule", logging.String("pattern", rule.Pattern))
	}
	return nil
}

// RemoveRule removes a filtering rule.
func (b *DNSBlocker) RemoveRule(pattern string) error {
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()

	if _, exists := b.rules[pattern]; !exists {
		return fmt.Errorf("rule for pattern %s not found", pattern)
	}
	delete(b.rules, pattern)
	return nil
}

// GetAllRules returns a copy of all current rules
func (b *DNSBlocker) GetAllRules() map[string]*FilterRule {
	b.rulesMu.RLock()
	defer b.rulesMu.RUnlock()

	// Create a copy to avoid race conditions
	rules := make(map[string]*FilterRule, len(b.rules))
	for pattern, rule := range b.rules {
		rules[pattern] = rule
	}
	return rules
}

// ClearAllRules removes all rules
func (b *DNSBlocker) ClearAllRules() {
	b.rulesMu.Lock()
	defer b.rulesMu.Unlock()

	b.rules = make(map[string]*FilterRule)
	if b.config.EnableLogging {
		b.logger.Debug("Cleared all DNS rules")
	}
}

func (b *DNSBlocker) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	b.statsMu.Lock()
	b.stats.TotalQueries++
	b.statsMu.Unlock()

	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")

	if b.shouldBlock(domain) {
		b.statsMu.Lock()
		b.stats.BlockedQueries++
		b.statsMu.Unlock()

		if b.config.EnableLogging {
			b.logger.Info("Blocked DNS query", logging.String("domain", domain))
		}

		msg := new(dns.Msg)
		msg.SetReply(r)
		msg.Authoritative = true

		blockIPv4 := net.ParseIP(b.config.BlockIPv4)
		blockIPv6 := net.ParseIP(b.config.BlockIPv6)

		if q.Qtype == dns.TypeA && blockIPv4 != nil {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   blockIPv4,
			})
		} else if q.Qtype == dns.TypeAAAA && blockIPv6 != nil {
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
				AAAA: blockIPv6,
			})
		}
		w.WriteMsg(msg)
		return
	}

	// Forward to upstream DNS
	b.statsMu.Lock()
	b.stats.AllowedQueries++
	b.stats.UpstreamLookups++
	b.statsMu.Unlock()

	if b.config.EnableLogging {
		b.logger.Debug("Forwarding DNS query", logging.String("domain", domain))
	}

	client := new(dns.Client)
	var resp *dns.Msg
	var err error

	for _, upstream := range b.config.UpstreamDNS {
		resp, _, err = client.Exchange(r, upstream)
		if err == nil {
			w.WriteMsg(resp)
			return
		}
	}

	b.statsMu.Lock()
	b.stats.Errors++
	b.statsMu.Unlock()
	b.logger.Error("Failed to forward DNS query to any upstream", logging.Err(err))
	dns.HandleFailed(w, r)
}

func (b *DNSBlocker) shouldBlock(domain string) bool {
	b.rulesMu.RLock()
	defer b.rulesMu.RUnlock()

	for pattern, rule := range b.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Action != ActionBlock {
			continue
		}

		// Simple domain matching for now
		if strings.HasSuffix(domain, pattern) {
			return true
		}
	}
	return false
}

// GetStats returns current DNS blocker statistics
func (b *DNSBlocker) GetStats() DNSBlockerStats {
	b.statsMu.Lock()
	defer b.statsMu.Unlock()

	// Return a copy to prevent race conditions
	return b.stats
}

// GetRuleCount returns the number of active rules
func (b *DNSBlocker) GetRuleCount() int {
	b.rulesMu.RLock()
	defer b.rulesMu.RUnlock()
	return len(b.rules)
}
