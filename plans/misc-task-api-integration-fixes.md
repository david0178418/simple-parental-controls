# Miscellaneous Task: Frontend-Backend API Integration Fixes

**Priority:** Critical  
**Type:** Bug Fix / Integration  
**Estimated Effort:** 4-6 hours  

## Overview

During post-Milestone 5 testing, several critical API integration issues were discovered where the frontend expects endpoints that don't exist in the backend, causing 404 errors and broken functionality.

## Issues Identified

### 1. Missing Auth Check Endpoint (**Critical**)
- **Problem**: Frontend calls `/api/v1/auth/check` but backend doesn't implement this endpoint
- **Impact**: Authentication status checking fails on page load
- **Location**: `web/src/services/api.ts:138` → `internal/auth/handlers.go`

### 2. Password Change Endpoint Mismatch (**Medium**)
- **Problem**: Frontend calls `/api/v1/auth/change-password` but backend implements `/api/v1/auth/password/change`
- **Impact**: Password change functionality broken
- **Location**: `web/src/services/api.ts:318` → `internal/auth/handlers.go:180`

## Required Actions

### Option A: Add Missing Backend Endpoints (Recommended)
1. **Add `/api/v1/auth/check` endpoint**
   - Location: `internal/auth/handlers.go`
   - Should validate session and return basic auth status
   - Should be lightweight (no user data)
   
2. **Add `/api/v1/auth/change-password` endpoint alias**
   - Can redirect to existing `/api/v1/auth/password/change`
   - Or update frontend to use correct endpoint

### Option B: Update Frontend to Use Existing Endpoints
1. **Update `checkAuth()` method**
   - Change to use `/api/v1/auth/me` endpoint instead
   - Handle the different response format
   
2. **Fix password change endpoint**
   - Update to use `/api/v1/auth/password/change`

## Implementation Details

### Backend Changes (Option A)
```go
// Add to RegisterRoutes in internal/auth/handlers.go
srv.AddHandler("/api/v1/auth/check", authMiddleware.ThenFunc(ah.handleAuthCheck))

// New handler method
func (ah *AuthHandlers) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
        return
    }
    
    // Simple auth status check
    server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
        "authenticated": true,
        "timestamp": time.Now().UTC(),
    })
}
```

### Frontend Changes (Option B)
```typescript
public async checkAuth(): Promise<boolean> {
    try {
        await this.request('/api/v1/auth/me');
        return true;
    } catch {
        this.setAuthToken(null);
        return false;
    }
}
```

## Testing Requirements

1. **Integration Testing**
   - Test auth check on page load
   - Test password change functionality
   - Verify error handling for all scenarios

2. **Manual Testing**
   - Fresh page load authentication
   - Login/logout flows
   - Password change with various scenarios

## Dependencies

- None (standalone fixes)

## Success Criteria

- [ ] Frontend auth check works without 404 errors
- [ ] Password change functionality works end-to-end
- [ ] No console errors related to missing API endpoints
- [ ] All authentication flows work as expected

## Notes

- This represents a gap in the integration testing that should be addressed
- Consider adding API contract testing to prevent future mismatches
- May need to audit other endpoints for similar issues

**Recommendation**: Implement Option A (backend fixes) as it requires fewer changes and maintains the intended API design from the frontend perspective. 