// API Response Types
export interface ApiResponse<T = unknown> {
  data?: T;
  error?: string;
  message?: string;
  code?: number;
}

export interface HealthStatus {
  status: string;
  timestamp: string;
  uptime?: string;
  version: string;
  endpoints: Record<string, string>;
}

// Core Entity Types (matching Go models)
export interface Config {
  id: number;
  key: string;
  value: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export type ListType = 'whitelist' | 'blacklist';
export type EntryType = 'executable' | 'url';
export type PatternType = 'exact' | 'wildcard' | 'domain';
export type RuleType = 'allow_during' | 'block_during';
export type QuotaType = 'daily' | 'weekly' | 'monthly';
export type ActionType = 'allow' | 'block';
export type TargetType = 'executable' | 'url';

export interface List {
  id: number;
  name: string;
  type: ListType;
  description: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  entries?: ListEntry[];
}

export interface ListEntry {
  id: number;
  list_id: number;
  entry_type: EntryType;
  pattern: string;
  pattern_type: PatternType;
  description: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface TimeRule {
  id: number;
  list_id: number;
  name: string;
  rule_type: RuleType;
  days_of_week: number[];
  start_time: string;
  end_time: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface QuotaRule {
  id: number;
  list_id: number;
  name: string;
  quota_type: QuotaType;
  limit_seconds: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface QuotaUsage {
  id: number;
  quota_rule_id: number;
  period_start: string;
  period_end: string;
  used_seconds: number;
  created_at: string;
  updated_at: string;
}

export interface AuditLog {
  id: number;
  timestamp: string;
  event_type: string;
  target_type: TargetType;
  target_value: string;
  action: ActionType;
  rule_type?: string;
  rule_id?: number | null;
  details: string;
  created_at: string;
}

export interface DashboardStats {
  total_lists: number;
  total_entries: number;
  active_rules: number;
  today_blocks: number;
  today_allows: number;
  quotas_near_limit: number;
}

// Request/Response Types
export interface CreateListRequest {
  name: string;
  type: ListType;
  description: string;
  enabled: boolean;
}

export interface UpdateListRequest extends CreateListRequest {
  id: number;
}

export interface CreateListEntryRequest {
  list_id: number;
  entry_type: EntryType;
  pattern: string;
  pattern_type: PatternType;
  description: string;
  enabled: boolean;
}

export interface UpdateListEntryRequest extends CreateListEntryRequest {
  id: number;
}

export interface CreateTimeRuleRequest {
  list_id: number;
  name: string;
  rule_type: RuleType;
  days_of_week: number[];
  start_time: string;
  end_time: string;
  enabled: boolean;
}

export interface UpdateTimeRuleRequest extends CreateTimeRuleRequest {
  id: number;
}

export interface CreateQuotaRuleRequest {
  list_id: number;
  name: string;
  quota_type: QuotaType;
  limit_seconds: number;
  enabled: boolean;
}

export interface UpdateQuotaRuleRequest extends CreateQuotaRuleRequest {
  id: number;
}

// Authentication Types
export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  success: boolean;
  token?: string;
  message?: string;
}

export interface AuthUser {
  authenticated: boolean;
  timestamp: string;
}

// Filter and Query Types
export interface PaginationParams {
  limit?: number;
  offset?: number;
}

export interface SearchFilters extends PaginationParams {
  enabled?: boolean;
  list_type?: ListType;
  entry_type?: EntryType;
  start_date?: string;
  end_date?: string;
  search?: string;
}

export interface AuditLogFilters extends PaginationParams {
  action?: ActionType;
  target_type?: TargetType;
  start_time?: string;
  end_time?: string;
  search?: string;
} 