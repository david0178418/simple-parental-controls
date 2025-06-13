# Parental Control Application

A robust parental control desktop application for Windows 10+ and Linux that enforces configurable access rules on applications and websites. Features a local web management interface, password authentication, and comprehensive security features.

## Overview

This application provides parents and guardians with tools to manage and monitor their children's computer usage through:

- **Application Control**: Whitelist/blacklist executable programs
- **Website Filtering**: URL-based filtering with exact, wildcard, and domain matching
- **Time Management**: Configurable time windows and duration-based quotas
- **Audit Logging**: Comprehensive activity tracking with configurable retention
- **Web Management**: Local web interface accessible via QR code
- **Security**: Password-protected administration with bcrypt hashing

## Current Implementation Status

### ✅ Completed Features

#### Core Infrastructure
- **HTTP Server**: Robust web server with middleware chain
- **Configuration System**: File-based and environment variable configuration
- **Logging System**: Structured logging with multiple output formats
- **Database Layer**: SQLite-based data storage with migrations
- **Testing Framework**: Comprehensive test coverage with utilities

#### Authentication & Security (Milestone 6 - Complete)
- **Password System**: bcrypt hashing with strength validation and history tracking
- **Session Management**: Secure session tokens with activity tracking and concurrent limits  
- **Authentication Middleware**: Role-based access control with public/protected endpoints
- **HTTPS Support**: Self-signed certificate generation with TLS 1.2+ security

#### Web Interface Foundation
- **React Frontend**: TypeScript-based SPA with Material-UI components
- **API Integration**: RESTful API with authentication endpoints
- **Build System**: Bun-based development and production builds

### 🚧 In Development
- **Enforcement Engine**: Application and network filtering (planned)
- **Rule Management**: UI for creating and managing access rules (planned)
- **QR Code Generation**: Dynamic QR codes for easy access (planned)

## Technology Stack

- **Backend**: Go 1.24+ with SQLite database
- **Frontend**: React + TypeScript + Material-UI
- **Build Tools**: Make, Bun (frontend)
- **Security**: bcrypt, crypto/rand, TLS 1.2+
- **Testing**: Go testing framework with coverage reporting

## Quick Start

### Prerequisites

- Go 1.24 or higher
- Node.js 18+ (for web interface development)
- Bun (for frontend build tools)

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd parental-control
   ```

2. **Install dependencies**
   ```bash
   # Backend dependencies
   go mod download
   
   # Frontend dependencies (optional, for development)
   cd web && bun install && cd ..
   ```

3. **Build the application**
   ```bash
   # Build for current platform
   make build
   
   # Or build for production (optimized)
   make build-prod
   ```

4. **Run the application**
   ```bash
   # Run directly
   ./build/parental-control
   
   # Or use make
   make run
   ```

### Development Setup

```bash
# Install development dependencies
make deps

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linter (requires golangci-lint)
make lint

# Clean build artifacts
make clean
```

## Configuration
 current state of the app
The application supports both file-based and environment variable configuration.

### Configuration File

Create a YAML configuration file:

```yaml
web:
  port: 8080
  host: "localhost"
  enable_tls: false
  tls_port: 8443

security:
  password_min_length: 8
  password_require_uppercase: true
  password_require_lowercase: true  
  password_require_numbers: true
  password_require_special: true
  password_history_count: 5
  max_login_attempts: 5
  lockout_duration: "15m"
  session_timeout: "24h"
  session_cleanup_interval: "1h"
  max_concurrent_sessions: 3

database:
  path: "./data/parental-control.db"
  
logging:
  level: "info"
  format: "json"
```

### Environment Variables

```bash
# Web server configuration
WEB_PORT=8080
WEB_HOST=localhost
WEB_ENABLE_TLS=false

# Security settings
SECURITY_PASSWORD_MIN_LENGTH=8
SECURITY_MAX_LOGIN_ATTEMPTS=5
SECURITY_SESSION_TIMEOUT=24h

# Database configuration  
DATABASE_PATH=./data/parental-control.db

# Logging configuration
LOGGING_LEVEL=info
LOGGING_FORMAT=json
```

### Command Line Options

```bash
# Show version information
./parental-control -version

# Specify configuration file
./parental-control -config /path/to/config.yaml

# Override port
./parental-control -port 9000
```

## API Endpoints

### Public Endpoints
- `GET /health` - Health check
- `GET /status` - Application status
- `GET /api/v1/ping` - API connectivity test
- `GET /api/v1/info` - Server information

### Authentication Endpoints
- `POST /api/v1/auth/setup` - Initial admin setup
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout
- `POST /api/v1/auth/password/strength` - Password validation

### Protected Endpoints (Require Authentication)
- `GET /api/v1/auth/me` - Current user information
- `POST /api/v1/auth/password/change` - Change password
- `GET /api/v1/auth/sessions` - List user sessions
- `DELETE /api/v1/auth/sessions` - Revoke user sessions
- `POST /api/v1/auth/sessions/refresh` - Extend session
- `POST /api/v1/auth/sessions/revoke` - Revoke specific session

### Admin Endpoints (Require Admin Role)
- `GET /api/v1/auth/users` - User management
- `GET /api/v1/auth/security/stats` - Security statistics
- `POST /api/v1/tls/generate` - Generate TLS certificates
- `GET /api/v1/tls/certificate` - Get current certificate info

## Security Features

### Password Security
- **bcrypt Hashing**: Industry-standard password hashing with configurable cost
- **Strength Validation**: Enforced complexity requirements  
- **History Tracking**: Prevents password reuse (configurable history count)
- **Account Lockout**: Temporary lockout after failed attempts

### Session Security
- **Secure Tokens**: 256-bit cryptographically secure session identifiers
- **Activity Tracking**: IP addresses, request counts, last access times
- **Concurrent Limits**: Configurable maximum concurrent sessions per user
- **Automatic Cleanup**: Background cleanup of expired sessions

### Transport Security
- **HTTPS Support**: Optional TLS with self-signed certificates
- **Modern TLS**: TLS 1.2+ with secure cipher suites
- **Security Headers**: HSTS, Content Security Policy, and other security headers
- **HTTP/2 Support**: Modern protocol support for better performance

## Development

### Project Structure

```
parental-control/
├── cmd/
│   └── parental-control/          # Application entry point
├── internal/
│   ├── app/                       # Application orchestration
│   ├── auth/                      # Authentication system
│   ├── config/                    # Configuration management
│   ├── database/                  # Database layer
│   ├── logging/                   # Logging framework
│   ├── server/                    # HTTP server and middleware
│   └── service/                   # Business logic services
├── web/
│   ├── src/                       # Frontend React application
│   ├── public/                    # Static assets
│   └── build/                     # Production build output
├── data/                          # Runtime data (databases, logs)
├── plans/                         # Development milestones and tasks
└── examples/                      # Example configurations
```

### Build Targets

```bash
# Development builds
make build              # Build for current platform
make build-linux        # Build for Linux
make build-windows      # Build for Windows  
make build-cross        # Build for all platforms

# Production builds
make build-prod         # Optimized build for current platform

# Development tasks
make run               # Build and run
make test              # Run tests
make test-coverage     # Tests with coverage report
make fmt               # Format code
make lint              # Run linter
make clean             # Clean build artifacts

# Installation (requires sudo on Unix)
make install           # Install to system
make uninstall         # Remove from system
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/auth/... -v

# Run benchmarks
go test -bench=. ./...
```

## Deployment

### Linux Installation

```bash
# Build production binary
make build-prod

# Install system-wide (requires sudo)
sudo cp build/parental-control /usr/local/bin/

# Create systemd service (optional)
sudo nano /etc/systemd/system/parental-control.service
```

Example systemd service:
```ini
[Unit]
Description=Parental Control Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/parental-control -config /etc/parental-control/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Windows Installation

```bash
# Build Windows binary
make build-windows

# Deploy binary to desired location
# Configure as Windows Service (using nssm or similar tools)
```

## Roadmap

### Upcoming Milestones

- **Milestone 7**: List Management System - Create/edit whitelist and blacklist rules
- **Milestone 8**: Scheduling System - Time windows and quota management  
- **Milestone 9**: Enforcement Engine - Application and network filtering
- **Milestone 10**: Management Interface - Complete web UI for rule management
- **Milestone 11**: QR Code & Discovery - Easy access via QR codes
- **Milestone 12**: Audit & Reporting - Comprehensive activity monitoring

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Maintain test coverage above 80%
- Use TypeScript strict mode for frontend code
- Follow security best practices
- Document public APIs and complex logic

## License

[License information to be added]

## Support

For issues, questions, or contributions, please use the GitHub issue tracker.

---

**Version**: Development  
**Build**: In Development  
**Last Updated**: 2024 