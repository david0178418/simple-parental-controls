# Task 2: Dashboard API Endpoints

**Status:** 🔴 Not Started  
**Dependencies:** Task 1.1, Milestone 6 Complete  

## Description
Implement the dashboard-specific API endpoints that the React application expects to consume for displaying system information and statistics.

---

## Subtasks

### 2.1 Dashboard Statistics API 🔴
- Implement `/api/v1/dashboard/stats` endpoint
- Return system overview statistics (lists, rules, activity)
- Integrate with authentication middleware
- Format responses consistently with existing API patterns

### 2.2 System Overview Endpoints 🔴
- Add endpoints for system status and health information
- Implement basic metrics collection for dashboard display
- Create response models for dashboard data
- Ensure proper error handling and status codes

### 2.3 API Integration Testing 🔴
- Test dashboard API endpoints with authentication
- Verify response formats match React app expectations
- Test error scenarios and edge cases
- Performance testing for dashboard API responses

---

## API Endpoints to Implement

### Dashboard Statistics
```http
GET /api/v1/dashboard/stats
Authorization: Bearer <session-token>

Response:
{
  "total_lists": 5,
  "total_entries": 42, 
  "active_rules": 12,
  "today_blocks": 15,
  "today_allows": 128,
  "quotas_near_limit": 2
}
```

### System Information
```http
GET /api/v1/dashboard/system
Authorization: Bearer <session-token>

Response:
{
  "uptime": "2h15m30s",
  "version": "1.0.0",
  "platform": "linux/amd64",
  "memory_usage": "45MB",
  "cpu_usage": "2.1%",
  "active_connections": 3
}
```

---

## Implementation Details

### File Structure
```
internal/server/
├── api_dashboard.go        # New file for dashboard endpoints
├── api_auth.go            # Existing auth endpoints  
├── api_simple.go          # Existing simple endpoints
└── server.go              # Server registration
```

### Code Structure
```go
// internal/server/api_dashboard.go
package server

type DashboardAPIServer struct {
    // Dependencies for data collection
}

func NewDashboardAPIServer() *DashboardAPIServer {
    return &DashboardAPIServer{}
}

func (api *DashboardAPIServer) RegisterRoutes(server *Server) {
    // Protected middleware chain
    protectedMiddleware := NewMiddlewareChain(
        RequestIDMiddleware(),
        LoggingMiddleware(),
        RecoveryMiddleware(),
        SecurityHeadersMiddleware(),
        JSONMiddleware(),
        AuthenticationMiddleware(), // From Milestone 6
    )

    server.AddHandler("/api/v1/dashboard/stats", 
        protectedMiddleware.ThenFunc(api.handleDashboardStats))
    server.AddHandler("/api/v1/dashboard/system", 
        protectedMiddleware.ThenFunc(api.handleSystemInfo))
}
```

### Data Models
```go
type DashboardStats struct {
    TotalLists       int `json:"total_lists"`
    TotalEntries     int `json:"total_entries"`
    ActiveRules      int `json:"active_rules"`
    TodayBlocks      int `json:"today_blocks"`
    TodayAllows      int `json:"today_allows"`
    QuotasNearLimit  int `json:"quotas_near_limit"`
}

type SystemInfo struct {
    Uptime            string `json:"uptime"`
    Version           string `json:"version"`
    Platform          string `json:"platform"`
    MemoryUsage       string `json:"memory_usage"`
    CPUUsage          string `json:"cpu_usage"`
    ActiveConnections int    `json:"active_connections"`
}
```

---

## Integration Points

### Authentication Integration
- Use existing authentication middleware from Milestone 6
- Ensure all dashboard endpoints require valid session
- Return appropriate 401/403 for unauthorized access

### Data Sources
- For now, return mock/placeholder data
- Design interfaces for future integration with:
  - Rule management system (future milestone)
  - Enforcement engine (future milestone) 
  - Audit logging (future milestone)

### Error Handling
- Consistent with existing API error response format
- Proper HTTP status codes
- Informative error messages for debugging

---

## Acceptance Criteria
- [ ] Dashboard statistics endpoint returns expected JSON structure
- [ ] System information endpoint provides useful system data
- [ ] All endpoints require authentication and handle unauthorized access
- [ ] Error responses follow existing API patterns
- [ ] Response times are under 200ms for dashboard endpoints
- [ ] Integration tests verify endpoint functionality
- [ ] React dashboard can successfully consume the API data

---

## Testing Strategy

### Unit Tests
```go
func TestDashboardStatsEndpoint(t *testing.T) {
    // Test authenticated access
    // Test response format
    // Test error conditions
}

func TestSystemInfoEndpoint(t *testing.T) {
    // Test system data collection
    // Test response format
    // Test performance requirements
}
```

### Integration Tests  
```bash
# Test with authentication
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/dashboard/stats

# Test without authentication (should fail)
curl http://localhost:8080/api/v1/dashboard/stats

# Performance testing
ab -n 100 -c 5 -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/dashboard/stats
```

### Frontend Integration
- Verify React app can fetch and display dashboard data
- Test loading states and error handling in UI
- Ensure data updates properly on dashboard refresh

---

## Future Considerations

### Data Sources Evolution
As other milestones are completed, these endpoints will need to:
- Connect to rule management system for real list/rule counts
- Integrate with enforcement engine for block/allow statistics  
- Pull from audit logging for activity metrics
- Connect to monitoring system for performance data

### Performance Optimization
- Consider caching for frequently requested data
- Implement efficient data aggregation
- Add pagination for large datasets in future

---

## Implementation Notes

### Decisions Made
_Document any architectural or implementation decisions here_

### Issues Encountered  
_Track any problems faced and their solutions_

### Resources Used
_Links to documentation, examples, or references consulted_

---

**Last Updated:** 2024  
**Completed By:** _[Name/Date when marked complete]_ 