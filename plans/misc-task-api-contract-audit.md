# Miscellaneous Task: API Contract Audit & Integration Testing

**Priority:** High  
**Type:** Quality Assurance / Technical Debt  
**Estimated Effort:** 6-8 hours  

## Overview

The discovery of missing API endpoints in post-Milestone 5 testing reveals a gap in API contract validation between frontend and backend. This task addresses systematic auditing and prevention of similar issues.

## Problems Identified

### 1. No API Contract Validation
- **Issue**: Frontend and backend developed with different API assumptions
- **Impact**: Runtime 404 errors, broken features
- **Root Cause**: Lack of shared API specification

### 2. Missing Integration Testing
- **Issue**: No end-to-end tests covering API interactions
- **Impact**: Issues only discovered during manual testing
- **Gaps**: Authentication flows, CRUD operations, error scenarios

### 3. Incomplete API Documentation
- **Issue**: No single source of truth for API contracts
- **Impact**: Development misalignment, harder maintenance

## Required Actions

### Phase 1: Comprehensive API Audit
1. **Inventory Frontend API Calls**
   - Scan all files in `web/src/services/` and `web/src/contexts/`
   - Document every API endpoint expected by frontend
   - Note HTTP methods, request/response formats

2. **Inventory Backend API Endpoints**
   - Review all handlers in `internal/*/handlers.go`
   - Document all registered routes and their specifications
   - Note authentication requirements

3. **Gap Analysis**
   - Compare frontend expectations vs backend reality
   - Identify missing endpoints
   - Identify mismatched endpoints (method, path, format)
   - Identify unused backend endpoints

### Phase 2: API Contract Specification
1. **Create OpenAPI Specification**
   - Document all API endpoints in OpenAPI 3.0 format
   - Include request/response schemas
   - Document authentication requirements
   - Add error response specifications

2. **Generate API Documentation**
   - Use OpenAPI spec to generate human-readable docs
   - Include code examples for common operations
   - Document authentication flows

### Phase 3: Integration Testing Framework
1. **Backend API Tests**
   - Unit tests for each handler
   - Integration tests for auth flows
   - Error scenario testing

2. **Frontend API Tests**
   - Mock API responses based on contracts
   - Test error handling
   - Test authentication state management

3. **Contract Testing**
   - Implement consumer-driven contract testing
   - Validate frontend expectations against backend reality
   - Automated validation in CI/CD pipeline

## Discovered Endpoint Mismatches

Based on initial analysis:

### Missing Backend Endpoints
- `GET /api/v1/auth/check` (called by frontend)
- `POST /api/v1/auth/change-password` (called by frontend)

### Endpoint Mismatches
- Frontend: `POST /api/v1/auth/change-password`
- Backend: `POST /api/v1/auth/password/change`

### Potentially Missing Endpoints
Need to audit these areas:
- Dashboard stats endpoint
- All list management endpoints  
- Time rules endpoints
- Quota rules endpoints
- Audit log endpoints
- Configuration endpoints

## Implementation Plan

### Step 1: Quick Audit (2 hours)
```bash
# Frontend API calls audit
grep -r "request(" web/src/ | grep -E "'/api/" > frontend-api-calls.txt

# Backend endpoints audit  
grep -r "AddHandler" internal/ | grep -E "'/api/" > backend-endpoints.txt

# Compare and identify gaps
```

### Step 2: OpenAPI Specification (3 hours)
Create `docs/api-spec.yaml` with comprehensive API documentation

### Step 3: Integration Tests (3 hours)
Create test suites:
- `tests/integration/auth_test.go`
- `tests/integration/api_contract_test.go`
- `web/src/tests/api.test.ts`

## Success Criteria

- [ ] Complete inventory of all API endpoints (frontend + backend)
- [ ] OpenAPI specification covering all endpoints
- [ ] Zero endpoint mismatches between frontend and backend
- [ ] Integration test suite with >80% coverage of API interactions
- [ ] Automated contract validation in CI/CD
- [ ] API documentation published and accessible

## Deliverables

1. **API Audit Report**
   - `docs/api-audit-report.md`
   - Gap analysis with prioritized fixes

2. **OpenAPI Specification**
   - `docs/api-spec.yaml`
   - Generated documentation in `docs/api/`

3. **Test Suites**
   - Backend integration tests
   - Frontend API contract tests
   - CI/CD integration

4. **Documentation**
   - API usage guide for developers
   - Authentication flow documentation
   - Error handling guide

## Dependencies

- Completion of `misc-task-api-integration-fixes.md` (critical issues)
- Access to both frontend and backend codebases
- CI/CD pipeline for automated testing

## Long-term Benefits

- **Prevent API Contract Drift**: Automated validation catches mismatches early
- **Faster Development**: Clear API contracts reduce development confusion
- **Better Quality**: Comprehensive testing improves reliability
- **Easier Maintenance**: Documentation makes changes safer
- **Team Alignment**: Shared understanding of API contracts

## Notes

This task directly addresses the root cause of the authentication 404 error and similar integration issues. It's an investment in development workflow quality that will pay dividends in future development cycles. 