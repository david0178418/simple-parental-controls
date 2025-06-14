package enforcement

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProcessInfo represents information about a running process
type ProcessInfo struct {
	PID         int       `json:"pid"`
	PPID        int       `json:"ppid"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	CommandLine string    `json:"command_line"`
	StartTime   time.Time `json:"start_time"`
}

// ProcessMonitor interface defines the contract for process monitoring
type ProcessMonitor interface {
	// GetProcesses returns all currently running processes
	GetProcesses(ctx context.Context) ([]*ProcessInfo, error)

	// GetProcess returns information about a specific process by PID
	GetProcess(ctx context.Context, pid int) (*ProcessInfo, error)

	// Start begins monitoring processes for changes
	Start(ctx context.Context) error

	// Stop stops the process monitoring
	Stop() error

	// Subscribe to process events (start/stop)
	Subscribe() <-chan ProcessEvent
}

// ProcessEvent represents a process lifecycle event
type ProcessEvent struct {
	Type      ProcessEventType `json:"type"`
	Process   *ProcessInfo     `json:"process"`
	Timestamp time.Time        `json:"timestamp"`
}

// ProcessEventType defines the type of process event
type ProcessEventType string

const (
	ProcessStarted ProcessEventType = "started"
	ProcessStopped ProcessEventType = "stopped"
)

// ProcessIdentifier handles process identification and matching
type ProcessIdentifier struct {
	// Known processes for identification
	knownProcesses map[string]*ProcessSignature
	mu             sync.RWMutex
}

// ProcessSignature represents a process signature for identification
type ProcessSignature struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Hash         string            `json:"hash,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	MatchMethods []MatchMethod     `json:"match_methods"`
}

// MatchMethod defines how to match a process
type MatchMethod string

const (
	MatchByPath MatchMethod = "path"
	MatchByHash MatchMethod = "hash"
	MatchByName MatchMethod = "name"
)

// NewProcessIdentifier creates a new process identifier
func NewProcessIdentifier() *ProcessIdentifier {
	return &ProcessIdentifier{
		knownProcesses: make(map[string]*ProcessSignature),
	}
}

// AddSignature adds a process signature for identification
func (pi *ProcessIdentifier) AddSignature(signature *ProcessSignature) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	key := pi.generateKey(signature)
	pi.knownProcesses[key] = signature
}

// IdentifyProcess attempts to identify a process using registered signatures
func (pi *ProcessIdentifier) IdentifyProcess(process *ProcessInfo) (*ProcessSignature, bool) {
	pi.mu.RLock()
	defer pi.mu.RUnlock()

	// Try different matching methods
	for _, signature := range pi.knownProcesses {
		if pi.matchProcess(process, signature) {
			return signature, true
		}
	}

	return nil, false
}

// generateKey generates a unique key for a process signature
func (pi *ProcessIdentifier) generateKey(signature *ProcessSignature) string {
	return fmt.Sprintf("%s:%s", signature.Name, signature.Path)
}

// matchProcess checks if a process matches a signature
func (pi *ProcessIdentifier) matchProcess(process *ProcessInfo, signature *ProcessSignature) bool {
	for _, method := range signature.MatchMethods {
		switch method {
		case MatchByPath:
			if strings.EqualFold(process.Path, signature.Path) {
				return true
			}
		case MatchByName:
			if strings.EqualFold(process.Name, signature.Name) {
				return true
			}
		case MatchByHash:
			// Hash matching would require computing file hash
			// Implementation depends on requirements
			continue
		}
	}
	return false
}

// BaseProcessMonitor provides common functionality for process monitoring
type BaseProcessMonitor struct {
	subscribers   []chan ProcessEvent
	subscribersMu sync.RWMutex

	lastProcesses map[int]*ProcessInfo
	lastMu        sync.RWMutex

	pollInterval time.Duration
	running      bool
	runningMu    sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewBaseProcessMonitor creates a new base process monitor
func NewBaseProcessMonitor(pollInterval time.Duration) *BaseProcessMonitor {
	return &BaseProcessMonitor{
		subscribers:   make([]chan ProcessEvent, 0),
		lastProcesses: make(map[int]*ProcessInfo),
		pollInterval:  pollInterval,
		stopCh:        make(chan struct{}),
	}
}

// Subscribe returns a channel for process events
func (bpm *BaseProcessMonitor) Subscribe() <-chan ProcessEvent {
	bpm.subscribersMu.Lock()
	defer bpm.subscribersMu.Unlock()

	ch := make(chan ProcessEvent, 100) // Buffered channel
	bpm.subscribers = append(bpm.subscribers, ch)
	return ch
}

// publishEvent sends an event to all subscribers
func (bpm *BaseProcessMonitor) publishEvent(event ProcessEvent) {
	bpm.subscribersMu.RLock()
	defer bpm.subscribersMu.RUnlock()

	for _, ch := range bpm.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip this subscriber
		}
	}
}

// detectChanges compares current processes with last known state
func (bpm *BaseProcessMonitor) detectChanges(currentProcesses []*ProcessInfo) {
	bpm.lastMu.Lock()
	defer bpm.lastMu.Unlock()

	currentMap := make(map[int]*ProcessInfo)
	for _, proc := range currentProcesses {
		currentMap[proc.PID] = proc
	}

	// Detect new processes (started)
	for pid, proc := range currentMap {
		if _, exists := bpm.lastProcesses[pid]; !exists {
			bpm.publishEvent(ProcessEvent{
				Type:      ProcessStarted,
				Process:   proc,
				Timestamp: time.Now(),
			})
		}
	}

	// Detect stopped processes
	for pid, proc := range bpm.lastProcesses {
		if _, exists := currentMap[pid]; !exists {
			bpm.publishEvent(ProcessEvent{
				Type:      ProcessStopped,
				Process:   proc,
				Timestamp: time.Now(),
			})
		}
	}

	// Update last known state
	bpm.lastProcesses = currentMap
}

// isRunning returns the current running state
func (bpm *BaseProcessMonitor) isRunning() bool {
	bpm.runningMu.RLock()
	defer bpm.runningMu.RUnlock()
	return bpm.running
}

// setRunning sets the running state
func (bpm *BaseProcessMonitor) setRunning(running bool) {
	bpm.runningMu.Lock()
	defer bpm.runningMu.Unlock()
	bpm.running = running
}

// Stop stops the process monitoring
func (bpm *BaseProcessMonitor) Stop() error {
	if !bpm.isRunning() {
		return nil
	}

	bpm.setRunning(false)
	close(bpm.stopCh)
	bpm.wg.Wait()

	// Close all subscriber channels
	bpm.subscribersMu.Lock()
	defer bpm.subscribersMu.Unlock()

	for _, ch := range bpm.subscribers {
		close(ch)
	}
	bpm.subscribers = bpm.subscribers[:0]

	return nil
}

// Linux-specific implementation
type LinuxProcessMonitor struct {
	*BaseProcessMonitor
}

// NewLinuxProcessMonitor creates a new Linux process monitor
func NewLinuxProcessMonitor(pollInterval time.Duration) *LinuxProcessMonitor {
	return &LinuxProcessMonitor{
		BaseProcessMonitor: NewBaseProcessMonitor(pollInterval),
	}
}

// GetProcesses returns all running processes on Linux
func (lpm *LinuxProcessMonitor) GetProcesses(ctx context.Context) ([]*ProcessInfo, error) {
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %w", err)
	}

	var processes []*ProcessInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		process, err := lpm.GetProcess(ctx, pid)
		if err != nil {
			// Skip processes we can't read (permission issues, etc.)
			continue
		}

		processes = append(processes, process)
	}

	return processes, nil
}

// GetProcess returns information about a specific process on Linux
func (lpm *LinuxProcessMonitor) GetProcess(ctx context.Context, pid int) (*ProcessInfo, error) {
	procPath := filepath.Join("/proc", strconv.Itoa(pid))

	// Check if process directory exists
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("process %d not found", pid)
	}

	process := &ProcessInfo{
		PID: pid,
	}

	// Read process name and command line
	if err := lpm.readProcessInfo(procPath, process); err != nil {
		return nil, fmt.Errorf("failed to read process info for PID %d: %w", pid, err)
	}

	return process, nil
}

// readProcessInfo reads process information from /proc filesystem
func (lpm *LinuxProcessMonitor) readProcessInfo(procPath string, process *ProcessInfo) error {
	// Read command line
	cmdlineFile := filepath.Join(procPath, "cmdline")
	if cmdlineData, err := os.ReadFile(cmdlineFile); err == nil {
		// Command line arguments are null-separated
		cmdline := strings.ReplaceAll(string(cmdlineData), "\x00", " ")
		process.CommandLine = strings.TrimSpace(cmdline)
	}

	// Read executable path
	exeLink := filepath.Join(procPath, "exe")
	if exePath, err := os.Readlink(exeLink); err == nil {
		process.Path = exePath
		process.Name = filepath.Base(exePath)
	}

	// Read stat file for PPID and other info
	statFile := filepath.Join(procPath, "stat")
	if statData, err := os.ReadFile(statFile); err == nil {
		if err := lpm.parseStatFile(string(statData), process); err == nil {
			// Successfully parsed stat file
		}
	}

	// If we couldn't get name from exe, try comm file
	if process.Name == "" {
		commFile := filepath.Join(procPath, "comm")
		if commData, err := os.ReadFile(commFile); err == nil {
			process.Name = strings.TrimSpace(string(commData))
		}
	}

	return nil
}

// parseStatFile parses the /proc/[pid]/stat file
func (lpm *LinuxProcessMonitor) parseStatFile(statData string, process *ProcessInfo) error {
	// The stat file format: pid comm state ppid ...
	// comm can contain spaces and parentheses, so we need to parse carefully
	fields := strings.Fields(statData)
	if len(fields) < 4 {
		return fmt.Errorf("invalid stat file format")
	}

	// Find the end of comm field (last closing parenthesis)
	commEnd := strings.LastIndex(statData, ")")
	if commEnd == -1 {
		return fmt.Errorf("could not find end of comm field")
	}

	// Split the rest of the line after comm
	remaining := strings.TrimSpace(statData[commEnd+1:])
	restFields := strings.Fields(remaining)

	if len(restFields) >= 2 {
		if ppid, err := strconv.Atoi(restFields[1]); err == nil {
			process.PPID = ppid
		}
	}

	return nil
}

// Start begins monitoring processes on Linux
func (lpm *LinuxProcessMonitor) Start(ctx context.Context) error {
	if lpm.isRunning() {
		return fmt.Errorf("process monitor is already running")
	}

	lpm.setRunning(true)

	// Initial process snapshot
	initialProcesses, err := lpm.GetProcesses(ctx)
	if err != nil {
		lpm.setRunning(false)
		return fmt.Errorf("failed to get initial process list: %w", err)
	}

	lpm.lastMu.Lock()
	for _, proc := range initialProcesses {
		lpm.lastProcesses[proc.PID] = proc
	}
	lpm.lastMu.Unlock()

	// Start monitoring goroutine
	lpm.wg.Add(1)
	go lpm.monitorLoop(ctx)

	return nil
}

// monitorLoop runs the process monitoring loop
func (lpm *LinuxProcessMonitor) monitorLoop(ctx context.Context) {
	defer lpm.wg.Done()

	ticker := time.NewTicker(lpm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lpm.stopCh:
			return
		case <-ticker.C:
			if processes, err := lpm.GetProcesses(ctx); err == nil {
				lpm.detectChanges(processes)
			}
		}
	}
}

// NewProcessMonitor creates a platform-specific process monitor
func NewProcessMonitor(pollInterval time.Duration) ProcessMonitor {
	return newPlatformProcessMonitor(pollInterval)
}
