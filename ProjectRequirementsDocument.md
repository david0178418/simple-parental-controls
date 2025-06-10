### Project Requirements Document

#### 1. Overview

Parental control desktop application for Windows 10+ and Linux that enforces configurable access rules on applications and websites. Management via a local web app, discoverable by QR code, protected by password.

#### 2. Stakeholders

* **Parent/Guardian** (configures rules)
* **Child** (subject to restrictions)
* **Administrator** (any LAN user with password)
* **Developer/Ops** (maintains the app)

#### 3. Scope

**In-scope**

* Native desktop service on Windows 10+ and Linux (major distros)
* Whitelist/blacklist of executables (paths/identifiers) and URLs (exact, wildcard, domain)
* Time-window rules (“allow only” or “block only”) per list
* Duration-based limits (daily, weekly, monthly quotas)
* Configurable audit-log retention period
* Embedded HTTP server for management UI on LAN
* QR code for UI discovery
* Password authentication

**Out-of-scope**

* WAN/Internet management
* Deep content inspection or keyword scanning
* High-security threat models

#### 4. Functional Requirements

1. **List Management**

   * Create/edit/delete named lists (Whitelist/Blacklist)
   * Add/remove entries: executables by path/ID; URLs (exact, wildcard, domain)

2. **Scheduling & Quotas**

   * Time-window rules: select days of week; set start/end times (24-hour); “allow during” vs “block during”
   * Duration-based quotas per list:

     * Daily limit (e.g. 2 hours of YouTube per day)
     * Weekly limit (e.g. 5 hours of gaming per week)
     * Monthly limit
   * Automatic reset at period boundaries

3. **Enforcement Engine**

   * Go-based service/daemon
   * Process-monitoring for apps; pluggable network-filter module (iptables/proxy on Linux; native API on Windows)
   * Real-time rule enforcement; enforce quotas and time windows

4. **Management Interface**

   * Embedded HTTP server bound to localhost & LAN IP
   * React SPA (TypeScript) with MUI default theme; all TS strict options enabled
   * Login page (password)
   * Views to:

     * View/edit lists, schedules, quotas
     * Configure password and log-retention period
     * Audit-log viewer (timestamp, entry, action)

5. **QR Code Generation**

   * Dynamically rendered in UI and CLI output

6. **Authentication & Security**

   * Single admin password, hashed with bcrypt
   * All management endpoints require authentication
   * Server listens only on LAN interfaces

7. **Logging & Audit**

   * Record block/allow events with timestamp, type, entry
   * Configurable retention period (days) via UI
   * Automatic rotation based on retention setting

8. **CLI/Installer**

   * Linux: shell-script installer
   * Windows: MSI or NSIS installer

#### 5. Non-Functional Requirements

* **Platforms:** Windows 10+; Linux (Debian, Ubuntu, Fedora)
* **Performance:** Idle ≤ 5 % CPU; ≤ 50 MB RAM
* **Reliability:** Auto-restart on crash; persistent rules across reboots
* **Usability:** Minimal dependencies; straightforward installers
* **Maintainability:** Modular Go code; clear separation of enforcement, storage, API
* **Localization:** English only (proof of concept)
* **Accessibility:** Basic keyboard navigation for web UI

#### 6. Security & Privacy

* Password hashed with bcrypt
* Web server restricted to LAN interfaces
* No external telemetry or data collection
* Optional HTTPS with self-signed cert
* Default “fail-safe” mode: on service failure, enforce strictest blacklist policy

#### 7. System Architecture

```
[Go Service/Daemon]
     ├─ Enforcement Engine
     │    ├─ Process Monitor
     │    └─ Network Filter (iptables/API/proxy)
     ├─ Rule Store (SQLite)
     └─ Web API
           └─ Web UI (React + TypeScript + MUI)
```

#### 8. Technology Stack

* **Language:** Go
* **Rule Store:** SQLite
* **Web UI:** React + TypeScript (all strict type-checking enabled) + MUI (default theme)
* **QR Generation:** Go library or JS module
* **Network Enforcement:** Pluggable module (iptables/proxy on Linux; native API on Windows)
* **Installer:** Shell script (Linux), MSI/NSIS (Windows)

#### 9. Constraints & Assumptions

* Service runs with elevated privileges
* LAN allows HTTP access without captive portals
* Children use non-admin accounts or cannot disable the service

#### 10. Acceptance Criteria

* Rules (time windows and quotas) enforce correctly in test scenarios
* Web UI accessible via QR code and password from another LAN device
* Audit logs respect the configured retention period
* Service persists rules across reboots and stays within performance budget
