package enforcement

import (
	"context"
	"time"

	"parental-control/internal/models"
)

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

// AuditLogger interface for audit logging functionality
type AuditLogger interface {
	LogEnforcementAction(
		ctx context.Context,
		action models.ActionType,
		targetType models.TargetType,
		targetValue string,
		ruleType string,
		ruleID *int,
		details map[string]interface{},
	) error
}
