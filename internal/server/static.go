package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// StaticFileServer handles serving static files for the web UI
type StaticFileServer struct {
	config     Config
	fileSystem fs.FS
	cache      *fileCache
}

// fileCache provides basic file caching for static assets
type fileCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
}

type cacheEntry struct {
	content     []byte
	contentType string
	modTime     time.Time
	compressed  []byte
}

// NewStaticFileServer creates a new static file server
func NewStaticFileServer(config Config, embeddedFS fs.FS) *StaticFileServer {
	cache := &fileCache{
		entries: make(map[string]*cacheEntry),
		maxSize: 100, // Cache up to 100 files
	}

	return &StaticFileServer{
		config:     config,
		fileSystem: embeddedFS,
		cache:      cache,
	}
}

// ServeHTTP implements the http.Handler interface
func (sfs *StaticFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clean the path and remove leading slash
	filePath := path.Clean(r.URL.Path)
	if filePath == "/" {
		filePath = "/index.html"
	}
	filePath = strings.TrimPrefix(filePath, "/")

	// Try to serve from cache first
	if entry := sfs.getFromCache(filePath); entry != nil {
		sfs.serveFromCache(w, r, entry)
		return
	}

	// Try to serve from filesystem
	if err := sfs.serveFromFileSystem(w, r, filePath); err != nil {
		// If file not found and it's a client-side route, serve index.html
		if os.IsNotExist(err) && sfs.isClientRoute(filePath) {
			if err := sfs.serveFromFileSystem(w, r, "index.html"); err != nil {
				http.NotFound(w, r)
				return
			}
			return
		}

		logging.Debug("Static file not found",
			logging.String("path", filePath),
			logging.Err(err))
		http.NotFound(w, r)
		return
	}
}

// serveFromFileSystem serves a file from the configured filesystem
func (sfs *StaticFileServer) serveFromFileSystem(w http.ResponseWriter, r *http.Request, filePath string) error {
	file, err := sfs.fileSystem.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	if stat.IsDir() {
		// Try to serve index.html from directory
		indexPath := path.Join(filePath, "index.html")
		return sfs.serveFromFileSystem(w, r, indexPath)
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Determine content type
	contentType := sfs.getContentType(filePath)

	// Create cache entry
	entry := &cacheEntry{
		content:     content,
		contentType: contentType,
		modTime:     stat.ModTime(),
	}

	// Compress if enabled and appropriate
	if sfs.config.EnableCompression && sfs.shouldCompress(contentType) {
		entry.compressed = sfs.compressContent(content)
	}

	// Cache the entry if there's room
	sfs.addToCache(filePath, entry)

	// Serve the content
	sfs.serveFromCache(w, r, entry)
	return nil
}

// serveFromCache serves content from a cache entry
func (sfs *StaticFileServer) serveFromCache(w http.ResponseWriter, r *http.Request, entry *cacheEntry) {
	// Set headers
	w.Header().Set("Content-Type", entry.contentType)
	w.Header().Set("Last-Modified", entry.modTime.UTC().Format(http.TimeFormat))

	// Check if client has cached version
	if ifModSince := r.Header.Get("If-Modified-Since"); ifModSince != "" {
		if t, err := time.Parse(http.TimeFormat, ifModSince); err == nil {
			if entry.modTime.Before(t.Add(time.Second)) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Set cache headers for static assets
	if sfs.isStaticAsset(r.URL.Path) {
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
	}

	// Check if client accepts compression
	acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	// Serve compressed content if available and accepted
	if acceptsGzip && len(entry.compressed) > 0 {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(entry.compressed)))

		if r.Method != http.MethodHead {
			w.Write(entry.compressed)
		}
		return
	}

	// Serve uncompressed content
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(entry.content)))

	if r.Method != http.MethodHead {
		w.Write(entry.content)
	}
}

// getFromCache retrieves a file from cache
func (sfs *StaticFileServer) getFromCache(filePath string) *cacheEntry {
	sfs.cache.mu.RLock()
	defer sfs.cache.mu.RUnlock()

	return sfs.cache.entries[filePath]
}

// addToCache adds a file to cache
func (sfs *StaticFileServer) addToCache(filePath string, entry *cacheEntry) {
	sfs.cache.mu.Lock()
	defer sfs.cache.mu.Unlock()

	// If cache is full, remove oldest entry
	if len(sfs.cache.entries) >= sfs.cache.maxSize {
		var oldestPath string
		var oldestTime time.Time

		for path, entry := range sfs.cache.entries {
			if oldestPath == "" || entry.modTime.Before(oldestTime) {
				oldestPath = path
				oldestTime = entry.modTime
			}
		}

		if oldestPath != "" {
			delete(sfs.cache.entries, oldestPath)
		}
	}

	sfs.cache.entries[filePath] = entry
}

// getContentType determines the MIME type for a file
func (sfs *StaticFileServer) getContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	contentType := mime.TypeByExtension(ext)

	if contentType == "" {
		// Default content types for common web assets
		switch ext {
		case ".js":
			contentType = "application/javascript"
		case ".css":
			contentType = "text/css"
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".json":
			contentType = "application/json"
		case ".svg":
			contentType = "image/svg+xml"
		case ".woff", ".woff2":
			contentType = "font/woff"
		case ".ttf":
			contentType = "font/ttf"
		case ".eot":
			contentType = "application/vnd.ms-fontobject"
		default:
			contentType = "application/octet-stream"
		}
	}

	return contentType
}

// shouldCompress determines if content should be compressed
func (sfs *StaticFileServer) shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"text/",
		"application/javascript",
		"application/json",
		"application/xml",
		"image/svg+xml",
	}

	for _, t := range compressibleTypes {
		if strings.HasPrefix(contentType, t) {
			return true
		}
	}

	return false
}

// compressContent compresses content using gzip
func (sfs *StaticFileServer) compressContent(content []byte) []byte {
	var buf strings.Builder
	gzipWriter := gzip.NewWriter(&buf)

	gzipWriter.Write(content)
	gzipWriter.Close()

	return []byte(buf.String())
}

// isStaticAsset determines if a path is a static asset that can be cached long-term
func (sfs *StaticFileServer) isStaticAsset(path string) bool {
	staticExtensions := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".woff", ".woff2", ".ttf", ".eot", ".ico",
	}

	ext := filepath.Ext(path)
	for _, staticExt := range staticExtensions {
		if ext == staticExt {
			return true
		}
	}

	// Check for hashed filenames (e.g., main.abc123.js)
	name := filepath.Base(path)
	parts := strings.Split(name, ".")
	if len(parts) >= 3 {
		// Look for hash-like patterns
		for i := 1; i < len(parts)-1; i++ {
			if len(parts[i]) >= 8 && isAlphaNumeric(parts[i]) {
				return true
			}
		}
	}

	return false
}

// isClientRoute determines if a path should be handled by client-side routing
func (sfs *StaticFileServer) isClientRoute(path string) bool {
	// Don't treat paths with extensions as client routes
	if filepath.Ext(path) != "" {
		return false
	}

	// Don't treat API paths as client routes
	if strings.HasPrefix(path, "api/") {
		return false
	}

	return true
}

// isAlphaNumeric checks if a string contains only alphanumeric characters
func isAlphaNumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// ClearCache clears the file cache
func (sfs *StaticFileServer) ClearCache() {
	sfs.cache.mu.Lock()
	defer sfs.cache.mu.Unlock()

	sfs.cache.entries = make(map[string]*cacheEntry)
}

// GetCacheStats returns cache statistics
func (sfs *StaticFileServer) GetCacheStats() map[string]interface{} {
	sfs.cache.mu.RLock()
	defer sfs.cache.mu.RUnlock()

	return map[string]interface{}{
		"entries":  len(sfs.cache.entries),
		"max_size": sfs.cache.maxSize,
	}
}
