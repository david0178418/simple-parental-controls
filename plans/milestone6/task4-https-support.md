# Task 4: Optional HTTPS with Self-signed Certificates

**Status:** 🟢 Complete  
**Dependencies:** Task 3.2  

## Description
Implement optional HTTPS support with automatic self-signed certificate generation and secure communication.

---

## Subtasks

### 4.1 Certificate Generation and Management 🟢
- ✅ Implement self-signed certificate generation
- ✅ Create certificate storage and retrieval system
- ✅ Add certificate renewal and rotation
- ✅ Implement certificate validation and trust handling

### 4.2 HTTPS Server Configuration 🟢
- ✅ Add HTTPS server support alongside HTTP
- ✅ Implement TLS configuration and security settings
- ✅ Create HTTP to HTTPS redirection options
- ✅ Add certificate-based security headers

### 4.3 Client Integration and Security 🟢
- ✅ Handle certificate trust issues in web UI
- ✅ Implement certificate fingerprint verification
- ✅ Add security warnings and user guidance
- ✅ Create certificate export functionality for manual trust

---

## Acceptance Criteria
- [x] Self-signed certificate generation works
- [x] HTTPS mode is configurable and optional
- [x] Certificate validation is handled properly
- [x] HTTP to HTTPS redirection functions correctly
- [x] Security warnings are appropriate and helpful

---

## Implementation Notes

### Decisions Made
- **Certificate Generation**: Implemented RSA 2048-bit key generation with SHA-256 signatures
- **TLS Configuration**: Used TLS 1.2 minimum with secure cipher suites
- **Dual Server Mode**: HTTP and HTTPS servers can run simultaneously or HTTP can redirect to HTTPS
- **Certificate Management**: Auto-detection of local IP addresses for multi-interface certificates
- **API Endpoints**: Created RESTful endpoints for certificate download and trust instructions

### Architecture
The HTTPS implementation consists of several key components:

1. **TLS Manager** (`internal/server/tls.go`)
   - Certificate generation using crypto/x509 and crypto/rsa
   - Certificate validation and expiration checking
   - TLS configuration with secure defaults

2. **Server Integration** (`internal/server/server.go`)
   - Dual HTTP/HTTPS server support
   - Automatic certificate provisioning
   - HTTP to HTTPS redirection capability

3. **TLS API Server** (`internal/server/api_tls.go`)
   - Certificate download endpoint
   - TLS information and status API
   - Trust instruction generation

4. **Configuration Integration** (`internal/config/config.go`, `internal/app/app.go`)
   - Environment variable support for all TLS settings
   - Seamless integration with existing configuration system

### Security Features
- **Cipher Suites**: Limited to ECDHE-RSA with AES-GCM and ChaCha20-Poly1305
- **TLS Version**: Minimum TLS 1.2 (0x0303)
- **HTTP/2 Support**: Enabled via NextProtos configuration
- **HSTS Headers**: Automatic Strict-Transport-Security headers for HTTPS
- **Certificate Validation**: Real-time certificate expiration monitoring

### Configuration Options
The following configuration options are available:

**Web Configuration:**
- `tls_enabled`: Enable/disable HTTPS support
- `tls_cert_file`: Path to existing certificate file
- `tls_key_file`: Path to existing private key file
- `tls_auto_generate`: Auto-generate self-signed certificates
- `tls_cert_dir`: Directory for generated certificates
- `tls_hostname`: Hostname for certificate generation
- `tls_redirect_http`: Redirect HTTP to HTTPS
- `https_port`: Port for HTTPS server

**Environment Variables:**
- `PC_WEB_TLS_ENABLED`: Enable HTTPS
- `PC_WEB_TLS_CERT_FILE`: Certificate file path
- `PC_WEB_TLS_KEY_FILE`: Private key file path
- `PC_WEB_TLS_AUTO_GENERATE`: Auto-generate certificates
- `PC_WEB_TLS_CERT_DIR`: Certificate directory
- `PC_WEB_TLS_HOSTNAME`: Certificate hostname
- `PC_WEB_TLS_REDIRECT_HTTP`: Enable HTTP redirects
- `PC_WEB_HTTPS_PORT`: HTTPS port

### API Endpoints
- `GET /api/v1/tls/info` - TLS configuration and certificate information
- `GET /api/v1/tls/certificate` - Download server certificate
- `GET /api/v1/tls/trust-instructions` - Get trust instructions (text/JSON)

### Issues Encountered  
1. **Logging Function Compatibility**: Initial implementation used non-existent logging functions (`logging.Time`, `logging.Strings`). Fixed by converting to string format and using `fmt.Sprintf`.

2. **Configuration Conversion**: Required creating adapter functions to convert between config formats.

3. **Port Management**: Implemented logic to handle different ports for HTTP and HTTPS when redirection is enabled.

### Resources Used
- Go crypto/x509 documentation for certificate generation
- Go crypto/tls documentation for TLS configuration  
- RFC 5280 for X.509 certificate standards
- Mozilla SSL Configuration Generator for cipher suite selection

---

**Completed:** December 12, 2025  
**Completed By:** AI Assistant 