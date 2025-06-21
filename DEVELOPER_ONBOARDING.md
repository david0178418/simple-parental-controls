# Developer Onboarding Guide
## Parental Control Service - LLM Quick Reference

**Last Updated:** 2025-06-21  
**Purpose:** Get an LLM quickly up to speed on application structure and functionality

---

## 🎯 **Application Overview**

### **What It Is**
A Go-based parental control service that provides:
- **DNS-based website blocking** using iptables redirection
- **Process monitoring** for application control
- **Web dashboard** for management (React frontend)
- **RESTful API** for configuration and monitoring
- **Cross-platform support** (Linux/Windows)

### **Core Architecture**
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Frontend  │    │   HTTP Server    │    │ Enforcement     │
│   (React)       │◄──►│   (Go stdlib)    │◄──►│ Engine          │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                       ┌──────────────────┐    ┌─────────────────┐
                       │   Database       │    │ DNS Blocker     │
                       │   (SQLite)       │    │ (iptables)      │
                       └──────────────────┘    └─────────────────┘
```

---

## 📁 **Project Structure**

### **Key Directories**
```
test/
├── cmd/parental-control/          # Main application entry point
├── internal/                      # Private application code
│   ├── app/                      # Application orchestration
│   ├── auth/                     # Authentication & security
│   ├── config/                   # Configuration management
│   ├── database/                 # SQLite database layer
│   ├── enforcement/              # Core blocking engine
│   ├── logging/                  # Structured logging
│   ├── models/                   # Data models & repositories
│   ├── server/                   # HTTP server & API endpoints
│   └── service/                  # Business logic services
├── web/                          # React frontend
├── data/                         # Runtime data (DB, PID files)
├── config/                       # Configuration files
└── docs/                         # Documentation
```

### **Critical Files**
- `cmd/parental-control/main.go` - Application entry point and initialization
- `internal/enforcement/engine.go` - Core enforcement orchestration
- `internal/enforcement/dns_blocker.go` - DNS-based website blocking
- `internal/server/server.go` - HTTP server implementation
- `internal/config/config.go` - Configuration management
- `config/example.yaml` - Configuration template

---

## 🔧 **Core Components**

### **1. Enforcement Engine** (`internal/enforcement/`)
**Purpose:** Orchestrates all blocking and monitoring functionality

**Key Files:**
- `engine.go` - Main enforcement coordinator
- `dns_blocker.go` - DNS redirection and blocking
- `dns_manager.go` - iptables DNS rule management
- `process_monitor_linux.go` - Process monitoring (Linux)
- `types.go` - Core data structures

**How It Works:**
1. **DNS Redirection**: Uses iptables to redirect DNS queries (port 53) to local DNS server
2. **Rule Matching**: Checks domains against blacklist/whitelist patterns
3. **Response**: Returns 0.0.0.0 for blocked domains, forwards others to upstream DNS
4. **Cleanup**: Removes iptables rules on shutdown (CRITICAL for internet restoration)

**Key Methods:**
```go
// Start enforcement with proper DNS setup
func (ee *EnforcementEngine) Start(ctx context.Context) error

// Stop with DNS cleanup (CRITICAL - must clean iptables)
func (ee *EnforcementEngine) Stop(ctx context.Context) error

// Add blocking rules
func (ee *EnforcementEngine) AddNetworkRule(rule *FilterRule) error
```

### **2. Database Layer** (`internal/database/`)
**Purpose:** SQLite-based persistence for lists, entries, and audit logs

**Key Files:**
- `db.go` - Database connection and lifecycle
- `list_repository.go` - List CRUD operations
- `list_entry_repository.go` - Entry CRUD operations
- `migrations/` - SQL schema migrations

**Schema Overview:**
```sql
-- Core tables
lists                 -- Blacklist/whitelist definitions
list_entries         -- Individual patterns (domains, executables)
audit_logs          -- Enforcement actions and changes
retention_policies  -- Log cleanup rules
```

### **3. HTTP Server** (`internal/server/`)
**Purpose:** RESTful API and web interface serving

**Key Files:**
- `server.go` - HTTP server lifecycle
- `api_auth.go` - Authentication endpoints
- `api_dashboard.go` - Dashboard data endpoints
- `api_simple.go` - Basic system endpoints
- `static.go` - Frontend asset serving

**Current API Status:**
- ✅ **Authentication**: `/api/v1/auth/*` (login, logout, check, password change)
- ✅ **Dashboard**: `/api/v1/dashboard/stats` (system statistics)
- ✅ **System**: `/api/v1/ping`, `/api/v1/info` (health checks)
- ❌ **CRUD APIs**: Missing 15+ endpoints for lists, entries, rules (see API audit)

### **4. Configuration** (`internal/config/`)
**Purpose:** YAML + environment variable configuration management

**Key Features:**
- **Hierarchical config**: Service, Database, Web, Security, Enforcement
- **Environment overrides**: `PC_*` environment variables
- **Validation**: Comprehensive config validation with detailed errors
- **Defaults**: Sensible defaults for all settings

**Example Usage:**
```go
// Load configuration
config, err := config.LoadFromFile("config.yaml")

// Access enforcement settings
if config.Enforcement.Enabled {
    // Start enforcement engine
}
```

---

## 🔄 **Application Lifecycle**

### **Startup Sequence**
1. **Root Check**: Verify running as root (required for iptables)
2. **Config Loading**: Load YAML config + environment overrides
3. **Service Init**: Database, repositories, audit service
4. **HTTP Server**: Start web server and API endpoints
5. **Enforcement Engine**: Start DNS blocking and process monitoring
6. **Rule Loading**: Load blocking rules from database into engine

### **Shutdown Sequence** (CRITICAL)
1. **Signal Handling**: Catch SIGINT/SIGTERM
2. **Enforcement Stop**: Stop DNS blocker (cleans iptables rules)
3. **HTTP Server**: Graceful server shutdown
4. **Database**: Close connections
5. **Cleanup**: Remove PID files, temp resources

**⚠️ CRITICAL**: The enforcement engine MUST be stopped properly or DNS iptables rules will persist, breaking internet connectivity.

---

## 🏗️ **Data Models**

### **Core Entities** (`internal/models/`)
```go
// List represents a blacklist or whitelist
type List struct {
    ID          int       `json:"id"`
    Name        string    `json:"name"`
    Type        string    `json:"type"`        // "blacklist" or "whitelist"
    Description string    `json:"description"`
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// ListEntry represents a pattern within a list
type ListEntry struct {
    ID          int       `json:"id"`
    ListID      int       `json:"list_id"`
    Pattern     string    `json:"pattern"`     // Domain or executable pattern
    PatternType string    `json:"pattern_type"` // "domain", "executable"
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"created_at"`
}

// FilterRule represents an enforcement rule
type FilterRule struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    Pattern   string     `json:"pattern"`
    Action    ActionType `json:"action"`      // "block" or "allow"
    MatchType MatchType  `json:"match_type"` // "domain", "wildcard", "regex"
    Priority  int        `json:"priority"`
    Enabled   bool       `json:"enabled"`
}
```

### **Repository Pattern**
```go
// All data access goes through repository interfaces
type RepositoryManager struct {
    List      ListRepository
    ListEntry ListEntryRepository
    AuditLog  AuditLogRepository
}

// Example repository usage
lists, err := repos.List.GetAll(ctx)
entries, err := repos.ListEntry.GetByListID(ctx, listID)
```

---

## 🔐 **Security & Authentication**

### **Authentication Modes**
- **Disabled** (`auth_enabled: false`): All endpoints accessible, mock auth responses
- **Enabled** (`auth_enabled: true`): Session-based authentication required

### **Key Security Features**
- **Password hashing**: bcrypt with configurable cost
- **Session management**: Secure session tokens with expiration
- **Rate limiting**: Login attempt limiting with lockout
- **TLS support**: Optional HTTPS with auto-generated certificates
- **LAN-only binding**: Server only accessible from local network

### **Auth Flow**
```go
// Login
POST /api/v1/auth/login
{"username": "admin", "password": "password"}

// Check authentication status
GET /api/v1/auth/check
// Returns: {"authenticated": true, "timestamp": "..."}

// Logout
POST /api/v1/auth/logout
```

---

## 🌐 **Frontend Integration**

### **Web Dashboard** (`web/`)
- **Technology**: React + TypeScript + Bun
- **Build Output**: `web/build/` (served by Go server)
- **API Integration**: Calls backend REST endpoints

### **Key Frontend Files**
- `src/services/api.ts` - API client with all endpoint calls
- `src/pages/` - Main application pages
- `src/components/` - Reusable UI components
- `src/contexts/AuthContext.tsx` - Authentication state management

### **API Contract Issues**
- **Current Status**: Frontend expects 25+ endpoints, backend provides ~7
- **Impact**: Most UI functionality is non-functional (demo mode)
- **Priority**: Implement missing CRUD APIs for lists, entries, rules

---

## 🐛 **Common Issues & Solutions**

### **1. DNS Cleanup Problems**
**Symptom**: No internet after app shutdown
**Cause**: iptables DNS rules not cleaned up
**Solution**: Ensure `EnforcementEngine.Stop()` is called properly
**Emergency Fix**: `sudo iptables -t nat -F OUTPUT`

### **2. Permission Issues**
**Symptom**: "Permission denied" for iptables
**Cause**: Not running as root
**Solution**: Run with `sudo ./parental-control`

### **3. Database Locked**
**Symptom**: "Database is locked" errors
**Cause**: Multiple instances or improper shutdown
**Solution**: Check for existing PID file, kill old processes

### **4. Frontend 404 Errors**
**Symptom**: API calls returning 404
**Cause**: Missing backend endpoints
**Solution**: Implement missing CRUD APIs (see API audit report)

---

## 🔍 **Debugging & Monitoring**

### **Logging**
```go
// Structured logging throughout application
logging.Info("Starting enforcement engine")
logging.Error("DNS blocker failed", logging.Err(err))
logging.Debug("Process identified", logging.String("name", processName))
```

### **Key Log Messages**
- `"Starting enforcement engine"` - Enforcement initialization
- `"Setting up DNS redirection"` - iptables rules being applied
- `"Successfully restored original DNS settings"` - DNS cleanup success
- `"Blocked DNS query"` - Domain blocking in action

### **Health Checks**
```bash
# Application health
curl http://localhost:8080/api/v1/ping

# System information
curl http://localhost:8080/api/v1/info

# Dashboard statistics
curl http://localhost:8080/api/v1/dashboard/stats
```

### **Database Inspection**
```bash
# Direct database access
sqlite3 ./data/parental-control.db
.tables
SELECT * FROM lists;
SELECT * FROM list_entries;
```

---

## 🚀 **Development Workflow**

### **Building & Running**
```bash
# Build application
make build

# Run with default config
sudo ./build/parental-control

# Run with custom config
sudo ./build/parental-control --config config/production.yaml

# Build frontend
cd web && bun install && bun run build
```

### **Testing**
```bash
# Unit tests
go test ./internal/...

# Integration tests
go test ./tests/...

# API contract testing
./test-api-contract.sh
```

### **Configuration**
```bash
# Copy example config
cp config/example.yaml config/local.yaml

# Edit configuration
vim config/local.yaml

# Environment overrides
export PC_ENFORCEMENT_ENABLED=true
export PC_WEB_PORT=8080
```

---

## 📊 **Current Status & Priorities**

### **✅ Working Components**
- ✅ **Core Engine**: DNS blocking and process monitoring
- ✅ **Database**: SQLite with migrations and repositories  
- ✅ **Authentication**: Login/logout/session management
- ✅ **Configuration**: Comprehensive YAML + env var system
- ✅ **Shutdown**: Proper DNS cleanup (recently fixed)

### **❌ Missing Components (High Priority)**
- ❌ **CRUD APIs**: Lists, entries, rules management endpoints
- ❌ **Frontend Integration**: 15+ missing API endpoints
- ❌ **Time Rules**: Time-based blocking functionality
- ❌ **Quota Rules**: Usage-based blocking functionality
- ❌ **Audit UI**: Audit log viewing interface

### **🔄 In Progress**
- 🔄 **API Implementation**: Working on missing CRUD endpoints
- 🔄 **Testing**: Expanding test coverage
- 🔄 **Documentation**: API specification and user guides

---

## 💡 **Key Insights for LLM Operation**

### **1. DNS Blocking is Critical**
- The entire blocking mechanism relies on DNS redirection
- iptables rules MUST be cleaned up on shutdown
- Any DNS-related changes require root privileges

### **2. Repository Pattern**
- All database access goes through repository interfaces
- Service layer contains business logic
- Models define data structures and validation

### **3. Configuration-Driven**
- Most behavior is configurable via YAML + environment variables
- Always check config before hardcoding values
- Use `config.Default()` for sensible defaults

### **4. Frontend-Backend Gap**
- Frontend is fully implemented but backend APIs are missing
- Focus on implementing missing CRUD endpoints for immediate impact
- See `docs/api-audit-report.md` for detailed gap analysis

### **5. Error Handling**
- Use structured logging with context
- Collect and return multiple errors during shutdown
- Validate inputs at service layer boundaries

---

**🎯 Quick Start Checklist for New LLM:**
1. ✅ Understand DNS blocking mechanism (iptables + local DNS server)
2. ✅ Know the repository pattern for data access
3. ✅ Recognize frontend-backend API gap (15+ missing endpoints)
4. ✅ Always ensure proper shutdown cleanup (DNS rules)
5. ✅ Use configuration system instead of hardcoded values
6. ✅ Follow structured logging patterns
7. ✅ Check `docs/` directory for additional context

**Priority Focus Areas:**
1. **Implement missing CRUD APIs** (immediate frontend functionality)
2. **Enhance error handling** (production stability)
3. **Add comprehensive testing** (reliability)
4. **Improve documentation** (maintainability) 