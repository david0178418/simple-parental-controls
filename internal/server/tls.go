package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"parental-control/internal/logging"
)

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	// Enabled indicates if TLS is enabled
	Enabled bool
	// CertFile path to certificate file
	CertFile string
	// KeyFile path to private key file
	KeyFile string
	// AutoGenerate automatically generates self-signed certificates
	AutoGenerate bool
	// CertDir directory to store generated certificates
	CertDir string
	// Hostname for certificate generation
	Hostname string
	// IPAddresses for certificate generation
	IPAddresses []net.IP
	// ValidDuration for generated certificates
	ValidDuration time.Duration
	// MinTLSVersion minimum TLS version
	MinTLSVersion uint16
	// RedirectHTTP automatically redirect HTTP to HTTPS
	RedirectHTTP bool
	// HTTPPort port for HTTP server (for redirects)
	HTTPPort int
}

// DefaultTLSConfig returns TLS configuration with sensible defaults
func DefaultTLSConfig() TLSConfig {
	return TLSConfig{
		Enabled:       false,
		CertFile:      "",
		KeyFile:       "",
		AutoGenerate:  true,
		CertDir:       "./certs",
		Hostname:      "localhost",
		IPAddresses:   []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		ValidDuration: 365 * 24 * time.Hour, // 1 year
		MinTLSVersion: tls.VersionTLS12,
		RedirectHTTP:  false,
		HTTPPort:      8080,
	}
}

// TLSManager handles TLS certificate management and server configuration
type TLSManager struct {
	config TLSConfig
}

// NewTLSManager creates a new TLS manager
func NewTLSManager(config TLSConfig) *TLSManager {
	return &TLSManager{
		config: config,
	}
}

// EnsureCertificates ensures TLS certificates exist, generating them if necessary
func (tm *TLSManager) EnsureCertificates() error {
	if !tm.config.Enabled {
		return nil
	}

	// Use provided certificate files if specified
	if tm.config.CertFile != "" && tm.config.KeyFile != "" {
		if tm.certificatesExist() {
			if err := tm.validateCertificates(); err != nil {
				logging.Warn("Existing certificates are invalid, regenerating", logging.Err(err))
				return tm.generateCertificates()
			}
			logging.Info("Using existing TLS certificates",
				logging.String("cert_file", tm.config.CertFile),
				logging.String("key_file", tm.config.KeyFile))
			return nil
		}
	}

	// Auto-generate certificates if enabled
	if tm.config.AutoGenerate {
		return tm.generateCertificates()
	}

	return fmt.Errorf("TLS enabled but no certificates available and auto-generation disabled")
}

// GetTLSConfig returns a configured tls.Config for the server
func (tm *TLSManager) GetTLSConfig() (*tls.Config, error) {
	if !tm.config.Enabled {
		return nil, fmt.Errorf("TLS not enabled")
	}

	cert, err := tls.LoadX509KeyPair(tm.getCertPath(), tm.getKeyPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tm.config.MinTLSVersion,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2", "http/1.1"},
	}, nil
}

// certificatesExist checks if certificate files exist
func (tm *TLSManager) certificatesExist() bool {
	certPath := tm.getCertPath()
	keyPath := tm.getKeyPath()

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil
}

// validateCertificates validates existing certificates
func (tm *TLSManager) validateCertificates() error {
	certPath := tm.getCertPath()
	keyPath := tm.getKeyPath()

	// Try to load the certificate pair
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load certificate pair: %w", err)
	}

	// Parse the certificate to check validity
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if certificate is expired or will expire soon
	now := time.Now()
	if now.After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate expired on %v", x509Cert.NotAfter)
	}

	if now.Add(30 * 24 * time.Hour).After(x509Cert.NotAfter) {
		logging.Warn("Certificate will expire soon",
			logging.String("expires_at", x509Cert.NotAfter.Format(time.RFC3339)))
	}

	return nil
}

// generateCertificates generates new self-signed certificates
func (tm *TLSManager) generateCertificates() error {
	logging.Info("Generating self-signed TLS certificates")

	// Ensure certificate directory exists
	if err := os.MkdirAll(tm.config.CertDir, 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Parental Control"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    tm.config.Hostname,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(tm.config.ValidDuration),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add hostname to certificate
	template.DNSNames = []string{tm.config.Hostname}

	// Add additional hostnames
	if tm.config.Hostname != "localhost" {
		template.DNSNames = append(template.DNSNames, "localhost")
	}

	// Add IP addresses
	template.IPAddresses = tm.config.IPAddresses

	// Auto-detect local IP addresses
	if localIPs, err := tm.getLocalIPAddresses(); err == nil {
		for _, ip := range localIPs {
			// Avoid duplicates
			found := false
			for _, existing := range template.IPAddresses {
				if ip.Equal(existing) {
					found = true
					break
				}
			}
			if !found {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
		}
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	certPath := tm.getCertPath()
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Save private key
	keyPath := tm.getKeyPath()
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	logging.Info("TLS certificates generated successfully",
		logging.String("cert_file", certPath),
		logging.String("key_file", keyPath),
		logging.String("expires_at", template.NotAfter.Format(time.RFC3339)),
		logging.String("dns_names", fmt.Sprintf("%v", template.DNSNames)),
		logging.Int("ip_addresses", len(template.IPAddresses)))

	return nil
}

// getCertPath returns the path to the certificate file
func (tm *TLSManager) getCertPath() string {
	if tm.config.CertFile != "" {
		return tm.config.CertFile
	}
	return filepath.Join(tm.config.CertDir, "server.crt")
}

// getKeyPath returns the path to the private key file
func (tm *TLSManager) getKeyPath() string {
	if tm.config.KeyFile != "" {
		return tm.config.KeyFile
	}
	return filepath.Join(tm.config.CertDir, "server.key")
}

// getLocalIPAddresses returns local IP addresses for certificate generation
func (tm *TLSManager) getLocalIPAddresses() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && !ip.IsLoopback() {
				if ip.To4() != nil || ip.To16() != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	return ips, nil
}

// GetCertificateInfo returns information about the current certificate
func (tm *TLSManager) GetCertificateInfo() (map[string]interface{}, error) {
	if !tm.config.Enabled || !tm.certificatesExist() {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	certPath := tm.getCertPath()
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Calculate fingerprint
	fingerprint := fmt.Sprintf("%x", cert.Signature)
	if len(fingerprint) > 20 {
		fingerprint = fingerprint[:20] + "..."
	}

	return map[string]interface{}{
		"enabled":      true,
		"subject":      cert.Subject.String(),
		"issuer":       cert.Issuer.String(),
		"not_before":   cert.NotBefore,
		"not_after":    cert.NotAfter,
		"dns_names":    cert.DNSNames,
		"ip_addresses": cert.IPAddresses,
		"fingerprint":  fingerprint,
		"cert_file":    certPath,
		"key_file":     tm.getKeyPath(),
	}, nil
}

// ExportCertificate exports the certificate in PEM format for manual trust
func (tm *TLSManager) ExportCertificate() ([]byte, error) {
	if !tm.config.Enabled || !tm.certificatesExist() {
		return nil, fmt.Errorf("no certificate available")
	}

	certPath := tm.getCertPath()
	return os.ReadFile(certPath)
}

// HTTPRedirectHandler returns a handler that redirects HTTP to HTTPS
func (tm *TLSManager) HTTPRedirectHandler(httpsPort int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Construct HTTPS URL
		host := r.Host
		if host == "" {
			host = tm.config.Hostname
		}

		// Remove port from host if present
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		// Add HTTPS port if not standard
		if httpsPort != 443 {
			host = fmt.Sprintf("%s:%d", host, httpsPort)
		}

		httpsURL := fmt.Sprintf("https://%s%s", host, r.RequestURI)

		// Set security headers
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Location", httpsURL)

		// Use 301 for GET requests, 308 for others to preserve method
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMovedPermanently)
		} else {
			w.WriteHeader(http.StatusPermanentRedirect)
		}

		// Write a simple HTML response for browsers
		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Redirecting to HTTPS</title>
    <meta http-equiv="refresh" content="0; url=%s">
</head>
<body>
    <h1>Redirecting to HTTPS</h1>
    <p>Please follow <a href="%s">this link</a> to the secure version.</p>
</body>
</html>`, httpsURL, httpsURL)))
	}
}

// GetTrustInstructions returns instructions for trusting the self-signed certificate
func (tm *TLSManager) GetTrustInstructions() string {
	var buf bytes.Buffer

	buf.WriteString("To trust the self-signed certificate:\n\n")
	buf.WriteString("1. Download the certificate:\n")
	buf.WriteString("   curl -k https://localhost:8443/api/v1/tls/certificate > server.crt\n\n")
	buf.WriteString("2. Add to system trust store:\n\n")
	buf.WriteString("   Linux (Ubuntu/Debian):\n")
	buf.WriteString("   sudo cp server.crt /usr/local/share/ca-certificates/\n")
	buf.WriteString("   sudo update-ca-certificates\n\n")
	buf.WriteString("   macOS:\n")
	buf.WriteString("   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain server.crt\n\n")
	buf.WriteString("   Windows:\n")
	buf.WriteString("   certlm.msc -> Trusted Root Certification Authorities -> Import\n\n")
	buf.WriteString("3. Restart your browser to pick up the new certificate\n")

	return buf.String()
}
