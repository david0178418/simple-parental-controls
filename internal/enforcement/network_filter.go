package enforcement

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// NetworkFilter interface defines the contract for network filtering
type NetworkFilter interface {
	// AddRule adds a filtering rule
	AddRule(rule *FilterRule) error

	// RemoveRule removes a filtering rule by ID
	RemoveRule(ruleID string) error

	// EvaluateURL evaluates a URL against all rules and returns the decision
	EvaluateURL(ctx context.Context, url string, processInfo *ProcessInfo) (*FilterDecision, error)

	// Start starts the network filter
	Start(ctx context.Context) error

	// Stop stops the network filter
	Stop() error

	// GetStats returns filtering statistics
	GetStats() *FilterStats
}

// FilterRule represents a network filtering rule
type FilterRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Action      FilterAction `json:"action"`
	Pattern     string       `json:"pattern"`
	MatchType   MatchType    `json:"match_type"`
	ProcessID   int          `json:"process_id,omitempty"`
	ProcessName string       `json:"process_name,omitempty"`
	Categories  []string     `json:"categories,omitempty"`
	Priority    int          `json:"priority"`
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// FilterAction defines what action to take when a rule matches
type FilterAction string

const (
	ActionAllow FilterAction = "allow"
	ActionBlock FilterAction = "block"
	ActionLog   FilterAction = "log"
)

// MatchType defines how to match the pattern
type MatchType string

const (
	MatchExact    MatchType = "exact"
	MatchWildcard MatchType = "wildcard"
	MatchRegex    MatchType = "regex"
	MatchDomain   MatchType = "domain"
)

// FilterDecision represents the result of evaluating a URL
type FilterDecision struct {
	Action      FilterAction `json:"action"`
	Rule        *FilterRule  `json:"rule,omitempty"`
	Reason      string       `json:"reason"`
	ProcessInfo *ProcessInfo `json:"process_info,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
	URL         string       `json:"url"`
}

// FilterStats represents filtering statistics
type FilterStats struct {
	TotalRequests   int64            `json:"total_requests"`
	BlockedRequests int64            `json:"blocked_requests"`
	AllowedRequests int64            `json:"allowed_requests"`
	RuleHits        map[string]int64 `json:"rule_hits"`
	ProcessStats    map[string]int64 `json:"process_stats"`
	CategoryStats   map[string]int64 `json:"category_stats"`
	LastReset       time.Time        `json:"last_reset"`
}

// NetworkFilterEngine implements the core network filtering logic
type NetworkFilterEngine struct {
	rules   map[string]*FilterRule
	rulesMu sync.RWMutex

	stats   *FilterStats
	statsMu sync.RWMutex

	cache     map[string]*FilterDecision
	cacheMu   sync.RWMutex
	cacheSize int
	cacheTTL  time.Duration

	running   bool
	runningMu sync.RWMutex
}

// NewNetworkFilterEngine creates a new network filter engine
func NewNetworkFilterEngine() *NetworkFilterEngine {
	return &NetworkFilterEngine{
		rules:     make(map[string]*FilterRule),
		cache:     make(map[string]*FilterDecision),
		cacheSize: 1000,
		cacheTTL:  5 * time.Minute,
		stats: &FilterStats{
			RuleHits:      make(map[string]int64),
			ProcessStats:  make(map[string]int64),
			CategoryStats: make(map[string]int64),
			LastReset:     time.Now(),
		},
	}
}

// AddRule adds a filtering rule
func (nfe *NetworkFilterEngine) AddRule(rule *FilterRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	// Validate rule pattern based on match type
	if err := nfe.validateRule(rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	nfe.rulesMu.Lock()
	nfe.rules[rule.ID] = rule
	nfe.rulesMu.Unlock()

	// Clear cache since rules have changed
	nfe.clearCache()

	return nil
}

// RemoveRule removes a filtering rule by ID
func (nfe *NetworkFilterEngine) RemoveRule(ruleID string) error {
	nfe.rulesMu.Lock()
	defer nfe.rulesMu.Unlock()

	if _, exists := nfe.rules[ruleID]; !exists {
		return fmt.Errorf("rule %s not found", ruleID)
	}

	delete(nfe.rules, ruleID)

	// Clear cache since rules have changed
	nfe.clearCache()

	return nil
}

// EvaluateURL evaluates a URL against all rules
func (nfe *NetworkFilterEngine) EvaluateURL(ctx context.Context, urlStr string, processInfo *ProcessInfo) (*FilterDecision, error) {
	// Check cache first
	cacheKey := nfe.generateCacheKey(urlStr, processInfo)
	if decision := nfe.getCachedDecision(cacheKey); decision != nil {
		nfe.updateStats(decision)
		return decision, nil
	}

	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Get applicable rules sorted by priority
	rules := nfe.getApplicableRules(processInfo)

	// Evaluate rules in priority order
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if nfe.matchesRule(parsedURL, rule) {
			decision := &FilterDecision{
				Action:      rule.Action,
				Rule:        rule,
				Reason:      fmt.Sprintf("Matched rule: %s", rule.Name),
				ProcessInfo: processInfo,
				Timestamp:   time.Now(),
				URL:         urlStr,
			}

			// Cache the decision
			nfe.cacheDecision(cacheKey, decision)

			// Update statistics
			nfe.updateStats(decision)

			return decision, nil
		}
	}

	// No rules matched, default to allow
	decision := &FilterDecision{
		Action:      ActionAllow,
		Reason:      "No matching rules",
		ProcessInfo: processInfo,
		Timestamp:   time.Now(),
		URL:         urlStr,
	}

	nfe.cacheDecision(cacheKey, decision)
	nfe.updateStats(decision)

	return decision, nil
}

// Start starts the network filter engine
func (nfe *NetworkFilterEngine) Start(ctx context.Context) error {
	nfe.runningMu.Lock()
	defer nfe.runningMu.Unlock()

	if nfe.running {
		return fmt.Errorf("network filter engine is already running")
	}

	nfe.running = true

	// Start cache cleanup goroutine
	go nfe.cacheCleanupLoop(ctx)

	return nil
}

// Stop stops the network filter engine
func (nfe *NetworkFilterEngine) Stop() error {
	nfe.runningMu.Lock()
	defer nfe.runningMu.Unlock()

	nfe.running = false
	return nil
}

// GetStats returns filtering statistics
func (nfe *NetworkFilterEngine) GetStats() *FilterStats {
	nfe.statsMu.RLock()
	defer nfe.statsMu.RUnlock()

	// Create a copy of stats to prevent race conditions
	stats := &FilterStats{
		TotalRequests:   nfe.stats.TotalRequests,
		BlockedRequests: nfe.stats.BlockedRequests,
		AllowedRequests: nfe.stats.AllowedRequests,
		RuleHits:        make(map[string]int64),
		ProcessStats:    make(map[string]int64),
		CategoryStats:   make(map[string]int64),
		LastReset:       nfe.stats.LastReset,
	}

	for k, v := range nfe.stats.RuleHits {
		stats.RuleHits[k] = v
	}
	for k, v := range nfe.stats.ProcessStats {
		stats.ProcessStats[k] = v
	}
	for k, v := range nfe.stats.CategoryStats {
		stats.CategoryStats[k] = v
	}

	return stats
}

// validateRule validates a rule's pattern and configuration
func (nfe *NetworkFilterEngine) validateRule(rule *FilterRule) error {
	switch rule.MatchType {
	case MatchRegex:
		_, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	case MatchExact, MatchWildcard, MatchDomain:
		if rule.Pattern == "" {
			return fmt.Errorf("pattern cannot be empty")
		}
	default:
		return fmt.Errorf("unsupported match type: %s", rule.MatchType)
	}

	return nil
}

// getApplicableRules returns rules that could apply to the given process
func (nfe *NetworkFilterEngine) getApplicableRules(processInfo *ProcessInfo) []*FilterRule {
	nfe.rulesMu.RLock()
	defer nfe.rulesMu.RUnlock()

	var applicableRules []*FilterRule

	for _, rule := range nfe.rules {
		// Check if rule applies to this process
		if rule.ProcessID != 0 && processInfo != nil && rule.ProcessID != processInfo.PID {
			continue
		}

		if rule.ProcessName != "" && processInfo != nil &&
			!strings.EqualFold(rule.ProcessName, processInfo.Name) {
			continue
		}

		applicableRules = append(applicableRules, rule)
	}

	// Sort by priority (higher priority first)
	for i := 0; i < len(applicableRules)-1; i++ {
		for j := i + 1; j < len(applicableRules); j++ {
			if applicableRules[i].Priority < applicableRules[j].Priority {
				applicableRules[i], applicableRules[j] = applicableRules[j], applicableRules[i]
			}
		}
	}

	return applicableRules
}

// matchesRule checks if a URL matches a specific rule
func (nfe *NetworkFilterEngine) matchesRule(parsedURL *url.URL, rule *FilterRule) bool {
	switch rule.MatchType {
	case MatchExact:
		return strings.EqualFold(parsedURL.String(), rule.Pattern)

	case MatchWildcard:
		return nfe.matchWildcard(parsedURL.Host, rule.Pattern) ||
			nfe.matchWildcard(parsedURL.String(), rule.Pattern)

	case MatchRegex:
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false
		}
		return regex.MatchString(parsedURL.String())

	case MatchDomain:
		return nfe.matchDomain(parsedURL.Host, rule.Pattern)

	default:
		return false
	}
}

// matchWildcard performs wildcard matching
func (nfe *NetworkFilterEngine) matchWildcard(text, pattern string) bool {
	// Convert wildcard pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "?", ".")
	regex, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		return false
	}
	return regex.MatchString(text)
}

// matchDomain checks if a hostname matches a domain pattern
func (nfe *NetworkFilterEngine) matchDomain(hostname, pattern string) bool {
	// Remove leading dot from pattern if present
	pattern = strings.TrimPrefix(pattern, ".")

	// Exact match
	if strings.EqualFold(hostname, pattern) {
		return true
	}

	// Subdomain match
	if strings.HasSuffix(hostname, "."+pattern) {
		return true
	}

	return false
}

// generateCacheKey generates a cache key for a URL and process
func (nfe *NetworkFilterEngine) generateCacheKey(url string, processInfo *ProcessInfo) string {
	if processInfo != nil {
		return fmt.Sprintf("%s|%d|%s", url, processInfo.PID, processInfo.Name)
	}
	return url
}

// getCachedDecision retrieves a cached decision
func (nfe *NetworkFilterEngine) getCachedDecision(key string) *FilterDecision {
	nfe.cacheMu.RLock()
	defer nfe.cacheMu.RUnlock()

	if decision, exists := nfe.cache[key]; exists {
		// Check if cache entry is still valid
		if time.Since(decision.Timestamp) < nfe.cacheTTL {
			return decision
		}
		// Remove expired entry
		delete(nfe.cache, key)
	}

	return nil
}

// cacheDecision caches a filtering decision
func (nfe *NetworkFilterEngine) cacheDecision(key string, decision *FilterDecision) {
	nfe.cacheMu.Lock()
	defer nfe.cacheMu.Unlock()

	// Implement simple LRU by removing oldest entries if cache is full
	if len(nfe.cache) >= nfe.cacheSize {
		// Remove one entry (simple implementation)
		for k := range nfe.cache {
			delete(nfe.cache, k)
			break
		}
	}

	nfe.cache[key] = decision
}

// clearCache clears the decision cache
func (nfe *NetworkFilterEngine) clearCache() {
	nfe.cacheMu.Lock()
	defer nfe.cacheMu.Unlock()

	nfe.cache = make(map[string]*FilterDecision)
}

// updateStats updates filtering statistics
func (nfe *NetworkFilterEngine) updateStats(decision *FilterDecision) {
	nfe.statsMu.Lock()
	defer nfe.statsMu.Unlock()

	nfe.stats.TotalRequests++

	switch decision.Action {
	case ActionBlock:
		nfe.stats.BlockedRequests++
	case ActionAllow:
		nfe.stats.AllowedRequests++
	}

	if decision.Rule != nil {
		nfe.stats.RuleHits[decision.Rule.ID]++

		for _, category := range decision.Rule.Categories {
			nfe.stats.CategoryStats[category]++
		}
	}

	if decision.ProcessInfo != nil {
		nfe.stats.ProcessStats[decision.ProcessInfo.Name]++
	}
}

// cacheCleanupLoop periodically cleans up expired cache entries
func (nfe *NetworkFilterEngine) cacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nfe.cleanupExpiredCache()
		}
	}
}

// cleanupExpiredCache removes expired entries from the cache
func (nfe *NetworkFilterEngine) cleanupExpiredCache() {
	nfe.cacheMu.Lock()
	defer nfe.cacheMu.Unlock()

	now := time.Now()
	for key, decision := range nfe.cache {
		if now.Sub(decision.Timestamp) > nfe.cacheTTL {
			delete(nfe.cache, key)
		}
	}
}
