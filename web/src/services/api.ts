import {
  ApiResponse,
  HealthStatus,
  List,
  ListEntry,
  TimeRule,
  QuotaRule,
  QuotaUsage,
  AuditLog,
  DashboardStats,
  CreateListRequest,
  UpdateListRequest,
  CreateListEntryRequest,
  UpdateListEntryRequest,
  CreateTimeRuleRequest,
  UpdateTimeRuleRequest,
  CreateQuotaRuleRequest,
  UpdateQuotaRuleRequest,
  LoginRequest,
  LoginResponse,
  SearchFilters,
  AuditLogFilters,
  Config,
  ApplicationInfo,
  ApplicationDiscoveryResponse
} from '../types/api';

class ApiError extends Error {
  public status: number;
  public response?: Response | undefined;

  constructor(
    status: number,
    message: string,
    response?: Response | undefined
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.response = response;
  }
}

class ApiClient {
  private baseUrl: string;
  private authToken: string | null = null;

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl;
    this.authToken = localStorage.getItem('auth_token');
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    
    const defaultHeaders: HeadersInit = {
      'Content-Type': 'application/json',
    };

    if (this.authToken) {
      defaultHeaders['Authorization'] = `Bearer ${this.authToken}`;
    }

    const config: RequestInit = {
      ...options,
      headers: {
        ...defaultHeaders,
        ...options.headers,
      },
    };

    try {
      const response = await fetch(url, config);

      if (!response.ok) {
        const errorText = await response.text();
        let errorMessage = `HTTP ${response.status}`;
        
        try {
          const errorJson = JSON.parse(errorText);
          errorMessage = errorJson.message || errorJson.error || errorMessage;
        } catch {
          errorMessage = errorText || errorMessage;
        }

        throw new ApiError(response.status, errorMessage, response);
      }

      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return await response.json();
      }

      return {} as T;
    } catch (error) {
      if (error instanceof ApiError) {
        throw error;
      }
      throw new ApiError(0, error instanceof Error ? error.message : 'Network error');
    }
  }

  public setAuthToken(token: string | null): void {
    this.authToken = token;
    if (token) {
      localStorage.setItem('auth_token', token);
    } else {
      localStorage.removeItem('auth_token');
    }
  }

  // Authentication API
  public async login(credentials: LoginRequest): Promise<LoginResponse> {
    const response = await this.request<LoginResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(credentials),
    });
    
    if (response.success && response.token) {
      this.setAuthToken(response.token);
    }
    
    return response;
  }

  public async logout(): Promise<void> {
    try {
      await this.request('/api/v1/auth/logout', {
        method: 'POST',
      });
    } finally {
      this.setAuthToken(null);
    }
  }

  public async checkAuth(): Promise<boolean> {
    try {
      const response = await this.request<{authenticated: boolean, auth_enabled: boolean}>('/api/v1/auth/check');
      
      // If auth is disabled, always consider user authenticated if they have a token
      if (!response.auth_enabled) {
        return response.authenticated;
      }
      
      // If auth is enabled, check the authentication status
      return response.authenticated;
    } catch {
      this.setAuthToken(null);
      return false;
    }
  }

  // Health and Status API
  public async getHealth(): Promise<HealthStatus> {
    return this.request<HealthStatus>('/health');
  }

  public async getStatus(): Promise<ApiResponse> {
    return this.request<ApiResponse>('/status');
  }

  public async getDashboardStats(): Promise<DashboardStats> {
    return this.request<DashboardStats>('/api/v1/dashboard/stats');
  }

  // Lists API
  public async getLists(filters?: SearchFilters): Promise<List[]> {
    const params = new URLSearchParams();
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          params.append(key, String(value));
        }
      });
    }
    
    const query = params.toString();
    const endpoint = query ? `/api/v1/lists?${query}` : '/api/v1/lists';
    
    const response = await this.request<{ lists: List[] }>(endpoint);
    return response.lists ?? [];
  }

  public async getList(id: number): Promise<List> {
    return this.request<List>(`/api/v1/lists/${id}`);
  }

  public async createList(list: CreateListRequest): Promise<List> {
    return this.request<List>('/api/v1/lists', {
      method: 'POST',
      body: JSON.stringify(list),
    });
  }

  public async updateList(list: UpdateListRequest): Promise<List> {
    return this.request<List>(`/api/v1/lists/${list.id}`, {
      method: 'PUT',
      body: JSON.stringify(list),
    });
  }

  public async deleteList(id: number): Promise<void> {
    await this.request(`/api/v1/lists/${id}`, {
      method: 'DELETE',
    });
  }

  // List Entries API
  public async getListEntries(listId: number): Promise<ListEntry[]> {
    const response = await this.request<ListEntry[]>(`/api/v1/lists/${listId}/entries`);
    return response ?? [];
  }

  public async createListEntry(entry: CreateListEntryRequest): Promise<ListEntry> {
    return this.request<ListEntry>(`/api/v1/lists/${entry.list_id}/entries`, {
      method: 'POST',
      body: JSON.stringify(entry),
    });
  }

  public async updateListEntry(entry: UpdateListEntryRequest): Promise<ListEntry> {
    return this.request<ListEntry>(`/api/v1/entries/${entry.id}`, {
      method: 'PUT',
      body: JSON.stringify(entry),
    });
  }

  public async deleteListEntry(id: number): Promise<void> {
    await this.request(`/api/v1/entries/${id}`, {
      method: 'DELETE',
    });
  }

  // Time Rules API
  public async getTimeRules(listId?: number): Promise<TimeRule[]> {
    const endpoint = listId ? `/api/v1/lists/${listId}/time-rules` : '/api/v1/time-rules';
    return this.request<TimeRule[]>(endpoint);
  }

  public async createTimeRule(rule: CreateTimeRuleRequest): Promise<TimeRule> {
    return this.request<TimeRule>(`/api/v1/lists/${rule.list_id}/time-rules`, {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  public async updateTimeRule(rule: UpdateTimeRuleRequest): Promise<TimeRule> {
    return this.request<TimeRule>(`/api/v1/time-rules/${rule.id}`, {
      method: 'PUT',
      body: JSON.stringify(rule),
    });
  }

  public async deleteTimeRule(id: number): Promise<void> {
    await this.request(`/api/v1/time-rules/${id}`, {
      method: 'DELETE',
    });
  }

  // Quota Rules API
  public async getQuotaRules(listId?: number): Promise<QuotaRule[]> {
    const endpoint = listId ? `/api/v1/lists/${listId}/quota-rules` : '/api/v1/quota-rules';
    return this.request<QuotaRule[]>(endpoint);
  }

  public async createQuotaRule(rule: CreateQuotaRuleRequest): Promise<QuotaRule> {
    return this.request<QuotaRule>(`/api/v1/lists/${rule.list_id}/quota-rules`, {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  public async updateQuotaRule(rule: UpdateQuotaRuleRequest): Promise<QuotaRule> {
    return this.request<QuotaRule>(`/api/v1/quota-rules/${rule.id}`, {
      method: 'PUT',
      body: JSON.stringify(rule),
    });
  }

  public async deleteQuotaRule(id: number): Promise<void> {
    await this.request(`/api/v1/quota-rules/${id}`, {
      method: 'DELETE',
    });
  }

  // Quota Usage API
  public async getQuotaUsage(quotaRuleId: number): Promise<QuotaUsage[]> {
    return this.request<QuotaUsage[]>(`/api/v1/quota-rules/${quotaRuleId}/usage`);
  }

  public async resetQuotaUsage(quotaRuleId: number): Promise<void> {
    await this.request(`/api/v1/quota-rules/${quotaRuleId}/reset`, {
      method: 'POST',
    });
  }

  // Audit Logs API
  public async getAuditLogs(filters?: AuditLogFilters): Promise<AuditLog[]> {
    const params = new URLSearchParams();
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          params.append(key, String(value));
        }
      });
    }
    
    const query = params.toString();
    const endpoint = query ? `/api/v1/audit?${query}` : '/api/v1/audit';
    
    return this.request<AuditLog[]>(endpoint);
  }

  // Configuration API
  public async getConfigs(): Promise<Config[]> {
    return this.request<Config[]>('/api/v1/config');
  }

  public async updateConfig(key: string, value: string): Promise<Config> {
    return this.request<Config>(`/api/v1/config/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    });
  }

  public async changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await this.request('/api/v1/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    });
  }

  // Applications API
  public async discoverApplications(): Promise<ApplicationInfo[]> {
    const response = await this.request<ApplicationDiscoveryResponse>('/api/v1/applications/discover');
    return response.applications ?? [];
  }

  public async getRunningApplications(): Promise<ApplicationInfo[]> {
    const response = await this.request<ApplicationDiscoveryResponse>('/api/v1/applications/running');
    return response.applications ?? [];
  }
}

// Export a singleton instance
export const apiClient = new ApiClient();
export { ApiError };
export default apiClient; 