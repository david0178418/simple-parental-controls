package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// APITestSuite represents the integration test suite for API contracts
type APITestSuite struct {
	BaseURL    string
	HTTPClient *http.Client
	SessionID  string
}

// NewAPITestSuite creates a new API test suite
func NewAPITestSuite(baseURL string) *APITestSuite {
	return &APITestSuite{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestAPIContract runs comprehensive API contract tests
func TestAPIContract(t *testing.T) {
	suite := NewAPITestSuite("http://192.168.1.24:8080")

	t.Run("Authentication Endpoints", func(t *testing.T) {
		suite.TestAuthenticationEndpoints(t)
	})

	t.Run("Dashboard Endpoints", func(t *testing.T) {
		suite.TestDashboardEndpoints(t)
	})

	t.Run("Missing Endpoints", func(t *testing.T) {
		suite.TestMissingEndpoints(t)
	})

	t.Run("System Endpoints", func(t *testing.T) {
		suite.TestSystemEndpoints(t)
	})
}

// TestAuthenticationEndpoints tests all authentication-related endpoints
func (suite *APITestSuite) TestAuthenticationEndpoints(t *testing.T) {
	// Test auth check endpoint (should always be available)
	t.Run("GET /api/v1/auth/check", func(t *testing.T) {
		resp, err := suite.GET("/api/v1/auth/check")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response structure
		if _, ok := result["authenticated"]; !ok {
			t.Error("Response missing 'authenticated' field")
		}

		if _, ok := result["timestamp"]; !ok {
			t.Error("Response missing 'timestamp' field")
		}

		t.Logf("✅ Auth check endpoint working correctly")
	})

	// Test login endpoint (may not work if auth disabled)
	t.Run("POST /api/v1/auth/login", func(t *testing.T) {
		loginData := map[string]interface{}{
			"username": "admin",
			"password": "password123",
		}

		resp, err := suite.POST("/api/v1/auth/login", loginData)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// When auth is disabled, we expect 405 or 404
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
			t.Logf("⚠️ Login endpoint not available (auth disabled)")
			return
		}

		// If auth is enabled, we expect proper response
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Unexpected status code: %d", resp.StatusCode)
		}

		t.Logf("✅ Login endpoint responding correctly")
	})

	// Test logout endpoint
	t.Run("POST /api/v1/auth/logout", func(t *testing.T) {
		resp, err := suite.POST("/api/v1/auth/logout", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Similar to login - may not be available when auth disabled
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
			t.Logf("⚠️ Logout endpoint not available (auth disabled)")
			return
		}

		t.Logf("✅ Logout endpoint responding correctly")
	})

	// Test password change endpoint
	t.Run("POST /api/v1/auth/change-password", func(t *testing.T) {
		passwordData := map[string]interface{}{
			"old_password": "oldpass123",
			"new_password": "newpass123",
		}

		resp, err := suite.POST("/api/v1/auth/change-password", passwordData)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// When auth is disabled, we expect 405
		if resp.StatusCode == http.StatusMethodNotAllowed {
			t.Logf("⚠️ Password change endpoint not available (auth disabled)")
			return
		}

		// If auth is enabled, we expect authentication required
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 (unauthorized), got %d", resp.StatusCode)
		}

		t.Logf("✅ Password change endpoint responding correctly")
	})
}

// TestDashboardEndpoints tests dashboard-related endpoints
func (suite *APITestSuite) TestDashboardEndpoints(t *testing.T) {
	t.Run("GET /api/v1/dashboard/stats", func(t *testing.T) {
		resp, err := suite.GET("/api/v1/dashboard/stats")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify expected dashboard stats fields
		expectedFields := []string{
			"total_lists", "total_entries", "active_rules",
			"today_blocks", "today_allows", "quotas_near_limit",
		}

		for _, field := range expectedFields {
			if _, ok := result[field]; !ok {
				t.Errorf("Dashboard stats missing field: %s", field)
			}
		}

		t.Logf("✅ Dashboard stats endpoint working correctly")
	})
}

// TestMissingEndpoints tests for endpoints that should exist but don't
func (suite *APITestSuite) TestMissingEndpoints(t *testing.T) {
	// Critical missing endpoints that frontend expects
	missingEndpoints := []struct {
		method      string
		path        string
		description string
		priority    string
	}{
		{"GET", "/api/v1/lists", "List all parental control lists", "CRITICAL"},
		{"POST", "/api/v1/lists", "Create new list", "CRITICAL"},
		{"GET", "/api/v1/lists/1", "Get specific list", "CRITICAL"},
		{"PUT", "/api/v1/lists/1", "Update list", "CRITICAL"},
		{"DELETE", "/api/v1/lists/1", "Delete list", "CRITICAL"},
		{"GET", "/api/v1/lists/1/entries", "Get list entries", "CRITICAL"},
		{"POST", "/api/v1/lists/1/entries", "Create list entry", "CRITICAL"},
		{"PUT", "/api/v1/entries/1", "Update entry", "CRITICAL"},
		{"DELETE", "/api/v1/entries/1", "Delete entry", "CRITICAL"},
		{"GET", "/api/v1/time-rules", "Get time rules", "HIGH"},
		{"GET", "/api/v1/lists/1/time-rules", "Get list time rules", "HIGH"},
		{"POST", "/api/v1/lists/1/time-rules", "Create time rule", "HIGH"},
		{"PUT", "/api/v1/time-rules/1", "Update time rule", "HIGH"},
		{"DELETE", "/api/v1/time-rules/1", "Delete time rule", "HIGH"},
		{"GET", "/api/v1/quota-rules", "Get quota rules", "HIGH"},
		{"GET", "/api/v1/lists/1/quota-rules", "Get list quota rules", "HIGH"},
		{"POST", "/api/v1/lists/1/quota-rules", "Create quota rule", "HIGH"},
		{"PUT", "/api/v1/quota-rules/1", "Update quota rule", "HIGH"},
		{"DELETE", "/api/v1/quota-rules/1", "Delete quota rule", "HIGH"},
		{"GET", "/api/v1/quota-rules/1/usage", "Get quota usage", "MEDIUM"},
		{"POST", "/api/v1/quota-rules/1/reset", "Reset quota usage", "MEDIUM"},
		{"GET", "/api/v1/audit", "Get audit logs", "MEDIUM"},
		{"GET", "/api/v1/config", "Get configuration", "MEDIUM"},
		{"PUT", "/api/v1/config/key", "Update configuration", "MEDIUM"},
	}

	t.Logf("Testing %d missing endpoints that frontend expects...", len(missingEndpoints))

	criticalCount := 0
	highCount := 0
	mediumCount := 0

	for _, endpoint := range missingEndpoints {
		t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			var resp *http.Response
			var err error

			switch endpoint.method {
			case "GET":
				resp, err = suite.GET(endpoint.path)
			case "POST":
				resp, err = suite.POST(endpoint.path, map[string]interface{}{})
			case "PUT":
				resp, err = suite.PUT(endpoint.path, map[string]interface{}{})
			case "DELETE":
				resp, err = suite.DELETE(endpoint.path)
			}

			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// We expect 404 for missing endpoints
			if resp.StatusCode == http.StatusNotFound {
				switch endpoint.priority {
				case "CRITICAL":
					criticalCount++
					t.Errorf("🚨 CRITICAL endpoint missing: %s %s - %s",
						endpoint.method, endpoint.path, endpoint.description)
				case "HIGH":
					highCount++
					t.Errorf("⚠️ HIGH priority endpoint missing: %s %s - %s",
						endpoint.method, endpoint.path, endpoint.description)
				case "MEDIUM":
					mediumCount++
					t.Logf("ℹ️ MEDIUM priority endpoint missing: %s %s - %s",
						endpoint.method, endpoint.path, endpoint.description)
				}
			} else {
				t.Logf("✅ Endpoint exists: %s %s (status: %d)",
					endpoint.method, endpoint.path, resp.StatusCode)
			}
		})
	}

	// Summary report
	t.Logf("\n=== MISSING ENDPOINTS SUMMARY ===")
	t.Logf("🚨 CRITICAL missing: %d endpoints", criticalCount)
	t.Logf("⚠️ HIGH missing: %d endpoints", highCount)
	t.Logf("ℹ️ MEDIUM missing: %d endpoints", mediumCount)
	t.Logf("📊 Total missing: %d endpoints", criticalCount+highCount+mediumCount)

	if criticalCount > 0 {
		t.Errorf("CRITICAL: %d essential endpoints are missing - frontend functionality will be broken", criticalCount)
	}
}

// TestSystemEndpoints tests basic system endpoints
func (suite *APITestSuite) TestSystemEndpoints(t *testing.T) {
	t.Run("GET /api/v1/ping", func(t *testing.T) {
		resp, err := suite.GET("/api/v1/ping")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("✅ Ping endpoint working correctly")
	})

	t.Run("GET /api/v1/info", func(t *testing.T) {
		resp, err := suite.GET("/api/v1/info")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("✅ Info endpoint working correctly")
	})
}

// HTTP helper methods
func (suite *APITestSuite) GET(path string) (*http.Response, error) {
	return suite.HTTPClient.Get(suite.BaseURL + path)
}

func (suite *APITestSuite) POST(path string, data interface{}) (*http.Response, error) {
	jsonData, _ := json.Marshal(data)
	return suite.HTTPClient.Post(
		suite.BaseURL+path,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
}

func (suite *APITestSuite) PUT(path string, data interface{}) (*http.Response, error) {
	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("PUT", suite.BaseURL+path, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return suite.HTTPClient.Do(req)
}

func (suite *APITestSuite) DELETE(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", suite.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return suite.HTTPClient.Do(req)
}
