# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a parental control application written in Go with a React TypeScript frontend. It provides parents and guardians with tools to manage and monitor their children's computer usage through application control, website filtering, time management, audit logging, and a password-protected web interface.

## Development Commands

### Backend (Go)

```bash
# Build application for current platform
make build

# Build optimized production binary
make build-prod

# Cross-platform builds
make build-linux        # Linux
make build-windows      # Windows  
make build-cross        # All platforms

# Development workflow
make run                # Build and run
make test               # Run all tests
make test-coverage      # Tests with HTML coverage report
make fmt                # Format all Go code
make lint               # Run golangci-lint (requires installation)
make clean              # Clean build artifacts

# Dependency management
make deps               # Download dependencies
make tidy               # Clean up go.mod

# System installation (requires sudo on Unix)
make install            # Install to /usr/local/bin
make uninstall          # Remove from system
```

### Frontend (React + TypeScript)

```bash
# Navigate to web directory first
cd web

# Development server
bun dev

# Production build
bun build

# Type checking
bun type-check
```

### Testing

```bash
# Run specific test packages
go test ./internal/auth/... -v
go test ./internal/database/... -v
go test ./internal/enforcement/... -v

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...
```

## Architecture Overview

### Core Components

- **Application Orchestration** (`internal/app/`): Main application lifecycle management, coordinating all services and components
- **HTTP Server** (`internal/server/`): Embedded web server with middleware chain, API endpoints, static file serving, and TLS support  
- **Authentication System** (`internal/auth/`): Password management with bcrypt, session handling, role-based access control
- **Configuration Management** (`internal/config/`): YAML/environment variable configuration with validation
- **Database Layer** (`internal/database/`): SQLite with migrations, repositories for different entity types
- **Business Logic** (`internal/service/`): Core parental control services including list management, time windows, quotas
- **Enforcement Engine** (`internal/enforcement/`): Process monitoring and network filtering (Windows/Linux compatible)
- **Logging Framework** (`internal/logging/`): Structured logging with configurable outputs

### Frontend Architecture

- **React SPA** (`web/src/`): TypeScript-based single page application
- **Material-UI Components**: Modern component library for consistent UI
- **API Integration** (`web/src/services/api.ts`): RESTful API client with authentication
- **Build System**: Bun-based development and production builds

### Key Design Patterns

- **Repository Pattern**: Database access abstraction in `internal/database/`
- **Service Layer**: Business logic separation in `internal/service/`
- **Middleware Chain**: HTTP request processing in `internal/server/`
- **Configuration Injection**: Environment and file-based config throughout
- **Cross-Platform Compatibility**: Platform-specific implementations in `internal/enforcement/`

## Configuration

### Environment Variables

All configuration can be overridden with environment variables prefixed with `PC_`:

```bash
# Core settings
PC_WEB_PORT=8080
PC_WEB_HOST=localhost
PC_DATABASE_PATH=./data/parental-control.db
PC_LOGGING_LEVEL=info

# Security settings  
PC_SECURITY_ENABLE_AUTH=true
PC_SECURITY_ADMIN_PASSWORD=your_password
PC_SECURITY_SESSION_TIMEOUT=24h
PC_SECURITY_BCRYPT_COST=12

# TLS settings
PC_WEB_TLS_ENABLED=true
PC_WEB_TLS_AUTO_GENERATE=true
PC_WEB_TLS_CERT_DIR=./certs
```

### Configuration File

Create `config/example.yaml` or specify with `-config` flag:

```yaml
web:
  port: 8080
  host: "localhost"
  tls_enabled: false

security:
  enable_auth: false
  min_password_length: 8
  session_timeout: "24h"

database:
  path: "./data/parental-control.db"
  
logging:
  level: "info"
  format: "json"
```

## Database

- **Engine**: SQLite with WAL mode
- **Migrations**: Located in `internal/database/migrations/`
- **Schema**: Entities defined in `internal/models/entities.go`
- **Repositories**: Type-specific data access in `internal/database/`

### Running Migrations

Migrations run automatically on application startup. Manual application:

```bash
# Apply retention policies
sqlite3 data/parental-control.db < scripts/apply_retention_migration.sql
```

## API Endpoints

### Public Endpoints
- `GET /health` - Health check
- `GET /status` - Application status  
- `GET /api/v1/ping` - API connectivity
- `GET /api/v1/info` - Server information

### Authentication Endpoints
- `POST /api/v1/auth/setup` - Initial admin setup
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout

### Protected Endpoints (Require Authentication)
- `GET /api/v1/auth/me` - Current user info
- `POST /api/v1/auth/password/change` - Change password
- `GET /api/v1/auth/sessions` - List sessions
- `POST /api/v1/tls/generate` - Generate TLS certificates

## Security Features

- **Password Security**: bcrypt hashing, strength validation, history tracking
- **Session Management**: Secure tokens, activity tracking, concurrent limits
- **Transport Security**: TLS 1.2+, self-signed certificate generation
- **Access Control**: Role-based authentication middleware

## Cross-Platform Notes

- **Windows Compatibility**: Platform-specific process monitoring in `internal/enforcement/process_monitor_windows.go`
- **Linux Support**: Native process monitoring in `internal/enforcement/process_monitor_linux.go`
- **Build Targets**: Use `make build-windows` or `make build-linux` for specific platforms

## Development Guidelines

- **Testing**: Maintain test coverage above 80%. Use `make test-coverage` to generate reports
- **Code Style**: Run `make fmt` before committing. Use `make lint` if golangci-lint is available
- **Dependencies**: Use `make tidy` to clean up go.mod after adding/removing dependencies
- **Configuration**: Always validate new config options in `internal/config/config.go`
- **Database Changes**: Create new migration files in `internal/database/migrations/`
- **API Changes**: Update both server handlers and frontend API client
- **Security**: Follow secure coding practices, never log sensitive data

## Common Development Workflow

1. **Start Development**: `make run` (builds and starts server)
2. **Frontend Development**: `cd web && bun dev` (separate terminal)
3. **Make Changes**: Edit Go/TypeScript files
4. **Test Changes**: `make test` for backend, frontend rebuilds automatically
5. **Format Code**: `make fmt` before committing
6. **Build for Production**: `make build-prod`

## Troubleshooting

- **Database Issues**: Check file permissions on `./data/` directory
- **TLS Certificate Issues**: Delete `./certs/` directory to regenerate
- **Port Conflicts**: Change `PC_WEB_PORT` environment variable
- **Build Failures**: Run `make clean && make deps` to reset build state
- **Frontend Issues**: Run `cd web && bun install` to refresh dependencies

## Project Specific Notes

- This app requires sudo. Ask me to run the application when you need to test its operation.