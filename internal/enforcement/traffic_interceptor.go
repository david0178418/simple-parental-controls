package enforcement

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TrafficInterceptor defines the interface for network traffic interception
type TrafficInterceptor interface {
	// Start begins traffic interception
	Start(ctx context.Context) error

	// Stop stops traffic interception
	Stop() error

	// Subscribe to intercepted network requests
	Subscribe() <-chan NetworkRequest

	// GetStats returns interception statistics
	GetStats() *InterceptionStats
}

// NetworkRequest represents an intercepted network request
type NetworkRequest struct {
	URL          string            `json:"url"`
	Domain       string            `json:"domain"`
	Protocol     string            `json:"protocol"`
	Port         int               `json:"port"`
	ProcessInfo  *ProcessInfo      `json:"process_info,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
	ConnectionID string            `json:"connection_id"`
}

// InterceptionStats holds statistics about traffic interception
type InterceptionStats struct {
	TotalRequests    int64            `json:"total_requests"`
	HTTPRequests     int64            `json:"http_requests"`
	HTTPSRequests    int64            `json:"https_requests"`
	DNSQueries       int64            `json:"dns_queries"`
	ProcessBreakdown map[string]int64 `json:"process_breakdown"`
	DomainBreakdown  map[string]int64 `json:"domain_breakdown"`
	LastRequestTime  time.Time        `json:"last_request_time"`
}

// HTTPTrafficInterceptor implements traffic interception for HTTP/HTTPS
type HTTPTrafficInterceptor struct {
	// Configuration
	listenPorts []int
	enableHTTPS bool
	enableDNS   bool

	// State management
	running   bool
	runningMu sync.RWMutex

	// Event handling
	subscribers   []chan NetworkRequest
	subscribersMu sync.RWMutex

	// Statistics
	stats   *InterceptionStats
	statsMu sync.RWMutex

	// Internal components
	httpProxy      *HTTPProxy
	dnsInterceptor *DNSInterceptor
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// HTTPProxy represents an HTTP proxy for traffic interception
type HTTPProxy struct {
	listener net.Listener
	port     int
}

// DNSInterceptor represents a DNS query interceptor
type DNSInterceptor struct {
	listener net.PacketConn
	port     int
}

// NewHTTPTrafficInterceptor creates a new HTTP traffic interceptor
func NewHTTPTrafficInterceptor(config *InterceptorConfig) *HTTPTrafficInterceptor {
	if config == nil {
		config = &InterceptorConfig{
			ListenPorts: []int{8080, 8443},
			EnableHTTPS: true,
			EnableDNS:   true,
		}
	}

	return &HTTPTrafficInterceptor{
		listenPorts: config.ListenPorts,
		enableHTTPS: config.EnableHTTPS,
		enableDNS:   config.EnableDNS,
		subscribers: make([]chan NetworkRequest, 0),
		stopCh:      make(chan struct{}),
		stats: &InterceptionStats{
			ProcessBreakdown: make(map[string]int64),
			DomainBreakdown:  make(map[string]int64),
		},
	}
}

// InterceptorConfig holds configuration for traffic interceptor
type InterceptorConfig struct {
	ListenPorts []int `json:"listen_ports"`
	EnableHTTPS bool  `json:"enable_https"`
	EnableDNS   bool  `json:"enable_dns"`
}

// Start begins traffic interception
func (hti *HTTPTrafficInterceptor) Start(ctx context.Context) error {
	hti.runningMu.Lock()
	defer hti.runningMu.Unlock()

	if hti.running {
		return fmt.Errorf("traffic interceptor already running")
	}

	// Start HTTP proxy
	if err := hti.startHTTPProxy(); err != nil {
		return fmt.Errorf("failed to start HTTP proxy: %w", err)
	}

	// Start DNS interceptor if enabled
	if hti.enableDNS {
		if err := hti.startDNSInterceptor(); err != nil {
			hti.stopHTTPProxy()
			return fmt.Errorf("failed to start DNS interceptor: %w", err)
		}
	}

	hti.running = true

	// Start processing goroutines
	hti.wg.Add(1)
	go hti.processingLoop(ctx)

	return nil
}

// Stop stops traffic interception
func (hti *HTTPTrafficInterceptor) Stop() error {
	hti.runningMu.Lock()
	defer hti.runningMu.Unlock()

	if !hti.running {
		return nil
	}

	hti.running = false
	close(hti.stopCh)

	// Stop components
	hti.stopHTTPProxy()
	hti.stopDNSInterceptor()

	// Wait for goroutines
	hti.wg.Wait()

	// Close subscriber channels
	hti.subscribersMu.Lock()
	for _, ch := range hti.subscribers {
		close(ch)
	}
	hti.subscribers = hti.subscribers[:0]
	hti.subscribersMu.Unlock()

	return nil
}

// Subscribe returns a channel for intercepted network requests
func (hti *HTTPTrafficInterceptor) Subscribe() <-chan NetworkRequest {
	hti.subscribersMu.Lock()
	defer hti.subscribersMu.Unlock()

	ch := make(chan NetworkRequest, 100)
	hti.subscribers = append(hti.subscribers, ch)
	return ch
}

// GetStats returns interception statistics
func (hti *HTTPTrafficInterceptor) GetStats() *InterceptionStats {
	hti.statsMu.RLock()
	defer hti.statsMu.RUnlock()

	// Create a copy
	stats := &InterceptionStats{
		TotalRequests:    hti.stats.TotalRequests,
		HTTPRequests:     hti.stats.HTTPRequests,
		HTTPSRequests:    hti.stats.HTTPSRequests,
		DNSQueries:       hti.stats.DNSQueries,
		ProcessBreakdown: make(map[string]int64),
		DomainBreakdown:  make(map[string]int64),
		LastRequestTime:  hti.stats.LastRequestTime,
	}

	for k, v := range hti.stats.ProcessBreakdown {
		stats.ProcessBreakdown[k] = v
	}
	for k, v := range hti.stats.DomainBreakdown {
		stats.DomainBreakdown[k] = v
	}

	return stats
}

// startHTTPProxy starts the HTTP proxy for intercepting requests
func (hti *HTTPTrafficInterceptor) startHTTPProxy() error {
	// For now, we'll implement a simple HTTP request parser
	// In a full implementation, this would be a proper HTTP proxy

	if len(hti.listenPorts) == 0 {
		return fmt.Errorf("no listen ports specified")
	}

	// Use the first port for HTTP proxy
	port := hti.listenPorts[0]

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	hti.httpProxy = &HTTPProxy{
		listener: listener,
		port:     port,
	}

	// Start accepting connections
	hti.wg.Add(1)
	go hti.handleHTTPConnections()

	return nil
}

// stopHTTPProxy stops the HTTP proxy
func (hti *HTTPTrafficInterceptor) stopHTTPProxy() {
	if hti.httpProxy != nil && hti.httpProxy.listener != nil {
		hti.httpProxy.listener.Close()
	}
}

// startDNSInterceptor starts DNS query interception
func (hti *HTTPTrafficInterceptor) startDNSInterceptor() error {
	// Listen on DNS port (would require root privileges in real implementation)
	// For testing, we'll use a different port
	conn, err := net.ListenPacket("udp", ":15353") // Non-standard DNS port for testing
	if err != nil {
		return fmt.Errorf("failed to listen for DNS on port 15353: %w", err)
	}

	hti.dnsInterceptor = &DNSInterceptor{
		listener: conn,
		port:     15353,
	}

	// Start handling DNS queries
	hti.wg.Add(1)
	go hti.handleDNSQueries()

	return nil
}

// stopDNSInterceptor stops DNS query interception
func (hti *HTTPTrafficInterceptor) stopDNSInterceptor() {
	if hti.dnsInterceptor != nil && hti.dnsInterceptor.listener != nil {
		hti.dnsInterceptor.listener.Close()
	}
}

// handleHTTPConnections handles incoming HTTP connections
func (hti *HTTPTrafficInterceptor) handleHTTPConnections() {
	defer hti.wg.Done()

	for {
		conn, err := hti.httpProxy.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-hti.stopCh:
				return
			default:
				continue
			}
		}

		// Handle connection in goroutine
		go hti.handleHTTPConnection(conn)
	}
}

// handleHTTPConnection handles a single HTTP connection
func (hti *HTTPTrafficInterceptor) handleHTTPConnection(conn net.Conn) {
	defer conn.Close()

	// Read HTTP request
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return
	}

	requestData := string(buffer[:n])

	// Parse HTTP request
	if request := hti.parseHTTPRequest(requestData); request != nil {
		hti.publishRequest(*request)
	}
}

// handleDNSQueries handles DNS query interception
func (hti *HTTPTrafficInterceptor) handleDNSQueries() {
	defer hti.wg.Done()

	buffer := make([]byte, 512)

	for {
		select {
		case <-hti.stopCh:
			return
		default:
			n, addr, err := hti.dnsInterceptor.listener.ReadFrom(buffer)
			if err != nil {
				continue
			}

			// Parse DNS query
			if request := hti.parseDNSQuery(buffer[:n], addr); request != nil {
				hti.publishRequest(*request)
			}
		}
	}
}

// parseHTTPRequest parses an HTTP request and extracts URL information
func (hti *HTTPTrafficInterceptor) parseHTTPRequest(requestData string) *NetworkRequest {
	lines := strings.Split(requestData, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Parse request line (GET /path HTTP/1.1)
	requestLine := strings.TrimSpace(lines[0])
	parts := strings.Fields(requestLine)
	if len(parts) < 2 {
		return nil
	}

	method := parts[0]
	path := parts[1]

	// Find Host header
	var host string
	headers := make(map[string]string)

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			continue
		}

		headerName := strings.TrimSpace(line[:colonIndex])
		headerValue := strings.TrimSpace(line[colonIndex+1:])
		headers[headerName] = headerValue

		if strings.EqualFold(headerName, "Host") {
			host = headerValue
		}
	}

	if host == "" {
		return nil
	}

	// Construct full URL
	protocol := "http"
	if hti.enableHTTPS {
		protocol = "https" // Simplified - would need actual TLS detection
	}

	fullURL := fmt.Sprintf("%s://%s%s", protocol, host, path)

	return &NetworkRequest{
		URL:          fullURL,
		Domain:       hti.extractDomain(host),
		Protocol:     strings.ToUpper(method),
		Port:         hti.httpProxy.port,
		Headers:      headers,
		Timestamp:    time.Now(),
		ConnectionID: fmt.Sprintf("http-%d", time.Now().UnixNano()),
	}
}

// parseDNSQuery parses a DNS query (simplified implementation)
func (hti *HTTPTrafficInterceptor) parseDNSQuery(data []byte, addr net.Addr) *NetworkRequest {
	// This is a very simplified DNS parser
	// In a real implementation, you'd parse the DNS protocol properly

	if len(data) < 12 {
		return nil
	}

	// Skip DNS header (12 bytes) and try to extract domain name
	domain := hti.extractDomainFromDNS(data[12:])
	if domain == "" {
		return nil
	}

	return &NetworkRequest{
		URL:          fmt.Sprintf("dns://%s", domain),
		Domain:       domain,
		Protocol:     "DNS",
		Port:         53,
		Timestamp:    time.Now(),
		ConnectionID: fmt.Sprintf("dns-%d", time.Now().UnixNano()),
	}
}

// extractDomain extracts domain from host header
func (hti *HTTPTrafficInterceptor) extractDomain(host string) string {
	// Remove port if present
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	return strings.ToLower(host)
}

// extractDomainFromDNS extracts domain from DNS query data (simplified)
func (hti *HTTPTrafficInterceptor) extractDomainFromDNS(data []byte) string {
	// Very simplified DNS domain extraction
	// In reality, DNS names are encoded with length prefixes

	domain := ""
	i := 0

	for i < len(data) {
		length := int(data[i])
		if length == 0 {
			break
		}

		if i+1+length >= len(data) {
			break
		}

		if domain != "" {
			domain += "."
		}

		domain += string(data[i+1 : i+1+length])
		i += 1 + length
	}

	return domain
}

// publishRequest publishes a network request to all subscribers
func (hti *HTTPTrafficInterceptor) publishRequest(request NetworkRequest) {
	// Update statistics
	hti.updateStats(&request)

	// Send to subscribers
	hti.subscribersMu.RLock()
	defer hti.subscribersMu.RUnlock()

	for _, ch := range hti.subscribers {
		select {
		case ch <- request:
		default:
			// Channel full, skip
		}
	}
}

// updateStats updates interception statistics
func (hti *HTTPTrafficInterceptor) updateStats(request *NetworkRequest) {
	hti.statsMu.Lock()
	defer hti.statsMu.Unlock()

	hti.stats.TotalRequests++
	hti.stats.LastRequestTime = request.Timestamp

	switch request.Protocol {
	case "HTTP", "GET", "POST", "PUT", "DELETE":
		hti.stats.HTTPRequests++
	case "HTTPS":
		hti.stats.HTTPSRequests++
	case "DNS":
		hti.stats.DNSQueries++
	}

	// Update domain breakdown
	if request.Domain != "" {
		hti.stats.DomainBreakdown[request.Domain]++
	}

	// Update process breakdown if available
	if request.ProcessInfo != nil {
		hti.stats.ProcessBreakdown[request.ProcessInfo.Name]++
	}
}

// processingLoop handles background processing tasks
func (hti *HTTPTrafficInterceptor) processingLoop(ctx context.Context) {
	defer hti.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hti.stopCh:
			return
		case <-ticker.C:
			// Periodic cleanup or maintenance tasks
			hti.performMaintenance()
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (hti *HTTPTrafficInterceptor) performMaintenance() {
	// Could implement cache cleanup, log rotation, etc.
}

// URLExtractor provides utility functions for URL analysis
type URLExtractor struct{}

// NewURLExtractor creates a new URL extractor
func NewURLExtractor() *URLExtractor {
	return &URLExtractor{}
}

// ExtractDomain extracts domain from a URL
func (ue *URLExtractor) ExtractDomain(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	return strings.ToLower(parsedURL.Host), nil
}

// ExtractPath extracts path from a URL
func (ue *URLExtractor) ExtractPath(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	return parsedURL.Path, nil
}

// IsHTTPS determines if a URL uses HTTPS
func (ue *URLExtractor) IsHTTPS(urlStr string) bool {
	return strings.HasPrefix(strings.ToLower(urlStr), "https://")
}

// MatchesPattern checks if a URL matches a pattern (supports wildcards)
func (ue *URLExtractor) MatchesPattern(urlStr, pattern string) bool {
	// Convert wildcard pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "?", ".")
	regexPattern = "^" + regexPattern + "$"

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return regex.MatchString(urlStr)
}
