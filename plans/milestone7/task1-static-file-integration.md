# Task 1: Static File Server Integration

**Status:** 🔴 Not Started  
**Dependencies:** Milestone 6 Complete  

## Description
Wire up the existing `StaticFileServer` implementation to serve the React dashboard instead of the current placeholder HTML.

---

## Subtasks

### 1.1 Replace Placeholder Root Handler 🔴
- Remove the placeholder HTML in `handleRoot` function
- Create and configure `StaticFileServer` instance
- Wire up the static file server to handle root requests
- Ensure proper error handling for missing files

### 1.2 Configure File System Access 🔴
- Set up file system access to `web/build/` directory
- Choose between embedded or file-based serving
- Configure proper path resolution and security
- Test access to all asset types (JS, CSS, HTML, fonts, images)

### 1.3 Integrate with Server Initialization 🔴
- Modify server startup to initialize static file server
- Ensure static file server works with existing middleware
- Configure caching and compression settings
- Test SPA routing (serve index.html for client-side routes)

---

## Implementation Details

### Current State
```go
// internal/server/server.go - handleRoot function
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
    // For now, return a simple response
    // Later this will serve the React app
    if r.URL.Path == "/" {
        w.Header().Set("Content-Type", "text/html")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`<!DOCTYPE html>...`))
        return
    }
    http.NotFound(w, r)
}
```

### Target Implementation
```go
// New approach: Use StaticFileServer
type Server struct {
    // ... existing fields ...
    staticServer *StaticFileServer
}

func (s *Server) initializeStaticServer() {
    // Configure filesystem access
    fileSystem := os.DirFS(s.config.StaticFileRoot)
    s.staticServer = NewStaticFileServer(s.config, fileSystem)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
    // Delegate to static file server
    s.staticServer.ServeHTTP(w, r)
}
```

### Files to Modify
- `internal/server/server.go` - Replace handleRoot, add static server initialization
- `internal/app/app.go` - Ensure StaticFileRoot configuration is passed correctly
- Add integration tests for static file serving

---

## Acceptance Criteria
- [ ] `http://localhost:8080/` serves the React dashboard instead of placeholder
- [ ] All static assets (JS, CSS, images, fonts) load correctly
- [ ] SPA client-side routing works (any path serves index.html)
- [ ] Proper MIME types set for all asset types
- [ ] Gzip compression enabled for compressible content
- [ ] File caching works correctly
- [ ] Error handling for missing files returns appropriate responses

---

## Testing Plan

### Manual Testing
```bash
# Test dashboard loading
curl -v http://localhost:8080/
curl -v http://localhost:8080/dashboard
curl -v http://localhost:8080/static/js/main.*.js

# Test compression
curl -H "Accept-Encoding: gzip" -v http://localhost:8080/static/js/main.*.js

# Test SPA routing
curl -v http://localhost:8080/some/client/route
```

### Automated Testing
- Unit tests for static file server integration
- Integration tests for asset serving
- Performance tests for file serving speed

---

## Implementation Notes

### Decisions Made
_Document any architectural or implementation decisions here_

### Issues Encountered  
_Track any problems faced and their solutions_

### Resources Used
_Links to documentation, examples, or references consulted_

---

**Last Updated:** 2024  
**Completed By:** _[Name/Date when marked complete]_ 