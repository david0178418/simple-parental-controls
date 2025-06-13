package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestTLSManager_DefaultConfig(t *testing.T) {
	config := DefaultTLSConfig()

	if config.Enabled {
		t.Error("Expected TLS to be disabled by default")
	}
	if !config.AutoGenerate {
		t.Error("Expected auto-generation to be enabled by default")
	}
	if config.Hostname != "localhost" {
		t.Errorf("Expected hostname to be 'localhost', got %s", config.Hostname)
	}
	if config.ValidDuration != 365*24*time.Hour {
		t.Errorf("Expected valid duration to be 1 year, got %v", config.ValidDuration)
	}
}

func TestTLSManager_GenerateCertificates(t *testing.T) {
	// Create temporary directory for certificates
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       tempDir,
		Hostname:      "test.example.com",
		ValidDuration: 24 * time.Hour, // 1 day for testing
		MinTLSVersion: tls.VersionTLS12,
	}

	manager := NewTLSManager(config)

	// Generate certificates
	err = manager.generateCertificates()
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	// Check if certificate files exist
	certPath := manager.getCertPath()
	keyPath := manager.getKeyPath()

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Certificate file not created: %s", certPath)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Key file not created: %s", keyPath)
	}

	// Validate certificate content
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to read certificate: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Check certificate properties
	if cert.Subject.CommonName != config.Hostname {
		t.Errorf("Expected CN=%s, got %s", config.Hostname, cert.Subject.CommonName)
	}

	// Check if hostname is in DNS names
	found := false
	for _, name := range cert.DNSNames {
		if name == config.Hostname {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Hostname %s not found in DNS names: %v", config.Hostname, cert.DNSNames)
	}

	// Check validity period
	if time.Until(cert.NotAfter) > config.ValidDuration+time.Hour {
		t.Errorf("Certificate validity period too long")
	}
}

func TestTLSManager_ValidateCertificates(t *testing.T) {
	// Create temporary directory for certificates
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       tempDir,
		Hostname:      "localhost",
		ValidDuration: 24 * time.Hour,
		MinTLSVersion: tls.VersionTLS12,
	}

	manager := NewTLSManager(config)

	// Generate certificates first
	err = manager.generateCertificates()
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	// Validate certificates
	err = manager.validateCertificates()
	if err != nil {
		t.Errorf("Certificate validation failed: %v", err)
	}
}

func TestTLSManager_GetTLSConfig(t *testing.T) {
	// Create temporary directory for certificates
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       tempDir,
		Hostname:      "localhost",
		ValidDuration: 24 * time.Hour,
		MinTLSVersion: tls.VersionTLS12,
	}

	manager := NewTLSManager(config)

	// Generate certificates first
	err = manager.generateCertificates()
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	// Get TLS config
	tlsConfig, err := manager.GetTLSConfig()
	if err != nil {
		t.Fatalf("Failed to get TLS config: %v", err)
	}

	// Check TLS config properties
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion %x, got %x", tls.VersionTLS12, tlsConfig.MinVersion)
	}

	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}

	// Check if certificate is valid
	cert := tlsConfig.Certificates[0]
	if len(cert.Certificate) == 0 {
		t.Error("Certificate chain is empty")
	}
}

func TestTLSManager_GetCertificateInfo(t *testing.T) {
	// Create temporary directory for certificates
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       tempDir,
		Hostname:      "test.local",
		ValidDuration: 24 * time.Hour,
		MinTLSVersion: tls.VersionTLS12,
	}

	manager := NewTLSManager(config)

	// Test when no certificates exist
	info, err := manager.GetCertificateInfo()
	if err != nil {
		t.Fatalf("Failed to get certificate info: %v", err)
	}

	if info["enabled"].(bool) {
		t.Error("Expected certificate to be disabled when no certs exist")
	}

	// Generate certificates
	err = manager.generateCertificates()
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	// Test after certificates are generated
	info, err = manager.GetCertificateInfo()
	if err != nil {
		t.Fatalf("Failed to get certificate info: %v", err)
	}

	if !info["enabled"].(bool) {
		t.Error("Expected certificate to be enabled after generation")
	}

	if info["cert_file"].(string) != manager.getCertPath() {
		t.Errorf("Expected cert_file to be %s, got %s", manager.getCertPath(), info["cert_file"])
	}

	dnsNames, ok := info["dns_names"].([]string)
	if !ok {
		t.Error("Expected dns_names to be []string")
	} else {
		found := false
		for _, name := range dnsNames {
			if name == config.Hostname {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected hostname %s in DNS names %v", config.Hostname, dnsNames)
		}
	}
}

func TestTLSAPIServer_HandleTLSInfo(t *testing.T) {
	// Create a test server with TLS disabled
	config := DefaultTLSConfig()
	config.Enabled = false

	serverConfig := Config{
		Port:              8080,
		BindToLAN:         false,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		StaticFileRoot:    "./web/build",
		EnableCompression: true,
		TLS:               config,
	}

	server := New(serverConfig)
	tlsAPI := NewTLSAPIServer(server)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tls/info", nil)
	w := httptest.NewRecorder()

	// Handle request
	tlsAPI.handleTLSInfo(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["tls_enabled"].(bool) {
		t.Error("Expected TLS to be disabled")
	}
}

func TestTLSAPIServer_HandleCertificateDownload(t *testing.T) {
	// Create temporary directory for certificates
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test server with TLS enabled
	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       tempDir,
		Hostname:      "localhost",
		ValidDuration: 24 * time.Hour,
		MinTLSVersion: tls.VersionTLS12,
	}

	serverConfig := Config{
		Port:              8080,
		BindToLAN:         false,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		StaticFileRoot:    "./web/build",
		EnableCompression: true,
		TLS:               config,
	}

	server := New(serverConfig)
	tlsAPI := NewTLSAPIServer(server)

	// Generate certificates
	err = server.tlsManager.generateCertificates()
	if err != nil {
		t.Fatalf("Failed to generate certificates: %v", err)
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tls/certificate", nil)
	w := httptest.NewRecorder()

	// Handle request
	tlsAPI.handleCertificateDownload(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check content type
	expectedContentType := "application/x-pem-file"
	if w.Header().Get("Content-Type") != expectedContentType {
		t.Errorf("Expected Content-Type %s, got %s", expectedContentType, w.Header().Get("Content-Type"))
	}

	// Check content disposition
	if w.Header().Get("Content-Disposition") != "attachment; filename=\"server.crt\"" {
		t.Errorf("Unexpected Content-Disposition header: %s", w.Header().Get("Content-Disposition"))
	}

	// Check if response contains valid PEM certificate
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	block, _ := pem.Decode(body)
	if block == nil {
		t.Error("Response does not contain valid PEM certificate")
	}
}

func TestTLSAPIServer_HandleTrustInstructions(t *testing.T) {
	// Create a test server with TLS enabled
	config := TLSConfig{
		Enabled:       true,
		AutoGenerate:  true,
		CertDir:       "/tmp/test-certs",
		Hostname:      "localhost",
		ValidDuration: 24 * time.Hour,
		MinTLSVersion: tls.VersionTLS12,
	}

	serverConfig := Config{
		Port:              8080,
		BindToLAN:         false,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		StaticFileRoot:    "./web/build",
		EnableCompression: true,
		TLS:               config,
	}

	server := New(serverConfig)
	tlsAPI := NewTLSAPIServer(server)

	// Test JSON response
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tls/trust-instructions?format=json", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	tlsAPI.handleTrustInstructions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if _, ok := response["instructions"]; !ok {
		t.Error("Expected 'instructions' field in JSON response")
	}

	// Test plain text response
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tls/trust-instructions", nil)
	w = httptest.NewRecorder()

	tlsAPI.handleTrustInstructions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Expected text/plain content type, got %s", w.Header().Get("Content-Type"))
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(body) == 0 {
		t.Error("Expected non-empty trust instructions")
	}
}

func TestHTTPRedirectHandler(t *testing.T) {
	config := TLSConfig{
		Enabled:      true,
		RedirectHTTP: true,
		Hostname:     "localhost",
		HTTPPort:     8080,
	}

	manager := NewTLSManager(config)
	handler := manager.HTTPRedirectHandler(8443)

	// Test GET request redirect
	req := httptest.NewRequest(http.MethodGet, "/test/path?param=value", nil)
	req.Host = "localhost:8080"
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status 301, got %d", w.Code)
	}

	expectedLocation := "https://localhost:8443/test/path?param=value"
	if w.Header().Get("Location") != expectedLocation {
		t.Errorf("Expected Location %s, got %s", expectedLocation, w.Header().Get("Location"))
	}

	// Check HSTS header
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("Expected Strict-Transport-Security header")
	}

	// Test POST request redirect (should use 308)
	req = httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.Host = "localhost:8080"
	w = httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusPermanentRedirect {
		t.Errorf("Expected status 308, got %d", w.Code)
	}
}
