//go:build windows

package enforcement

import (
	"context"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Windows API types and constants
type PROCESSENTRY32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriorityClass   int32
	Flags           uint32
	ExeFile         [260]uint16 // MAX_PATH
}

const (
	TH32CS_SNAPPROCESS         = 0x00000002
	INVALID_HANDLE_VALUE       = ^uintptr(0)
	PROCESS_TERMINATE          = 0x0001
	PROCESS_QUERY_INFORMATION  = 0x0400
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	STILL_ACTIVE               = 259
	WAIT_TIMEOUT               = 0x00000102
	INFINITE                   = 0xFFFFFFFF
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	createToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First            = kernel32.NewProc("Process32FirstW")
	process32Next             = kernel32.NewProc("Process32NextW")
	closeHandle               = kernel32.NewProc("CloseHandle")
	openProcess               = kernel32.NewProc("OpenProcess")
	queryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
	terminateProcess          = kernel32.NewProc("TerminateProcess")
	getExitCodeProcess        = kernel32.NewProc("GetExitCodeProcess")
	waitForSingleObject       = kernel32.NewProc("WaitForSingleObject")
)

// WindowsProcessMonitor implements process monitoring for Windows
type WindowsProcessMonitor struct {
	*BaseProcessMonitor
}

// NewWindowsProcessMonitor creates a new Windows process monitor
func NewWindowsProcessMonitor(pollInterval time.Duration) *WindowsProcessMonitor {
	return &WindowsProcessMonitor{
		BaseProcessMonitor: NewBaseProcessMonitor(pollInterval),
	}
}

// GetProcesses returns all running processes on Windows
func (wpm *WindowsProcessMonitor) GetProcesses(ctx context.Context) ([]*ProcessInfo, error) {
	snapshot, err := wpm.createProcessSnapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to create process snapshot: %w", err)
	}
	defer wpm.closeHandle(snapshot)

	var processes []*ProcessInfo

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))

	// Get first process
	ret, _, _ := process32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("failed to get first process")
	}

	for {
		process := &ProcessInfo{
			PID:  int(entry.ProcessID),
			PPID: int(entry.ParentProcessID),
			Name: syscall.UTF16ToString(entry.ExeFile[:]),
		}

		// Get full process path
		if path, err := wpm.getProcessPath(entry.ProcessID); err == nil {
			process.Path = path
		}

		// Get command line (would require additional Windows API calls)
		// For now, we'll use the executable name
		process.CommandLine = process.Name

		processes = append(processes, process)

		// Get next process
		ret, _, _ := process32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return processes, nil
}

// GetProcess returns information about a specific process on Windows
func (wpm *WindowsProcessMonitor) GetProcess(ctx context.Context, pid int) (*ProcessInfo, error) {
	processes, err := wpm.GetProcesses(ctx)
	if err != nil {
		return nil, err
	}

	for _, proc := range processes {
		if proc.PID == pid {
			return proc, nil
		}
	}

	return nil, fmt.Errorf("process %d not found", pid)
}

// Start begins monitoring processes on Windows
func (wpm *WindowsProcessMonitor) Start(ctx context.Context) error {
	if wpm.isRunning() {
		return fmt.Errorf("process monitor is already running")
	}

	wpm.setRunning(true)

	// Initial process snapshot
	initialProcesses, err := wpm.GetProcesses(ctx)
	if err != nil {
		wpm.setRunning(false)
		return fmt.Errorf("failed to get initial process list: %w", err)
	}

	wpm.lastMu.Lock()
	for _, proc := range initialProcesses {
		wpm.lastProcesses[proc.PID] = proc
	}
	wpm.lastMu.Unlock()

	// Start monitoring goroutine
	wpm.wg.Add(1)
	go wpm.monitorLoop(ctx)

	return nil
}

// monitorLoop runs the process monitoring loop for Windows
func (wpm *WindowsProcessMonitor) monitorLoop(ctx context.Context) {
	defer wpm.wg.Done()

	ticker := time.NewTicker(wpm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wpm.stopCh:
			return
		case <-ticker.C:
			if processes, err := wpm.GetProcesses(ctx); err == nil {
				wpm.detectChanges(processes)
			}
		}
	}
}

// createProcessSnapshot creates a snapshot of running processes
func (wpm *WindowsProcessMonitor) createProcessSnapshot() (uintptr, error) {
	ret, _, err := createToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if ret == INVALID_HANDLE_VALUE {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	return ret, nil
}

// closeHandle closes a Windows handle
func (wpm *WindowsProcessMonitor) closeHandle(handle uintptr) {
	closeHandle.Call(handle)
}

// getProcessPath gets the full path of a process
func (wpm *WindowsProcessMonitor) getProcessPath(pid uint32) (string, error) {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	// Open process handle
	handle, _, err := openProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION,
		0, // bInheritHandle
		uintptr(pid),
	)
	if handle == 0 {
		return "", fmt.Errorf("OpenProcess failed: %v", err)
	}
	defer wpm.closeHandle(handle)

	// Query full process image name
	var pathBuffer [1024]uint16
	pathSize := uint32(len(pathBuffer))

	ret, _, err := queryFullProcessImageName.Call(
		handle,
		0, // dwFlags (0 for full path)
		uintptr(unsafe.Pointer(&pathBuffer[0])),
		uintptr(unsafe.Pointer(&pathSize)),
	)

	if ret == 0 {
		return "", fmt.Errorf("QueryFullProcessImageName failed: %v", err)
	}

	return syscall.UTF16ToString(pathBuffer[:pathSize]), nil
}

// IsProcessRunning checks if a process with the given PID is running on Windows
func (wpm *WindowsProcessMonitor) IsProcessRunning(ctx context.Context, pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, _, _ := openProcess.Call(
		PROCESS_QUERY_INFORMATION,
		0, // bInheritHandle
		uintptr(pid),
	)
	if handle == 0 {
		return false
	}
	defer wpm.closeHandle(handle)

	var exitCode uint32
	ret, _, _ := getExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false
	}

	return exitCode == STILL_ACTIVE
}

// KillProcess terminates a process by PID on Windows
func (wpm *WindowsProcessMonitor) KillProcess(ctx context.Context, pid int, graceful bool) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	// Get process info for safety checks
	process, err := wpm.GetProcess(ctx, pid)
	if err != nil {
		return fmt.Errorf("failed to get process info: %w", err)
	}

	// Safety checks
	if IsSystemProcess(pid) {
		return fmt.Errorf("refusing to kill system process with PID %d", pid)
	}

	if IsCriticalProcess(process.Name) {
		return fmt.Errorf("refusing to kill critical process: %s", process.Name)
	}

	// Open process handle with terminate permission
	handle, _, err := openProcess.Call(
		PROCESS_TERMINATE|PROCESS_QUERY_INFORMATION,
		0, // bInheritHandle
		uintptr(pid),
	)
	if handle == 0 {
		return fmt.Errorf("failed to open process %d: %v", pid, err)
	}
	defer wpm.closeHandle(handle)

	if graceful {
		// On Windows, we don't have direct SIGTERM equivalent for arbitrary processes
		// We'll use a shorter timeout and then force terminate
		// First, check if the process is still active
		if !wpm.IsProcessRunning(ctx, pid) {
			return nil
		}

		// Wait a bit for potential graceful shutdown (some apps handle WM_CLOSE, etc.)
		time.Sleep(2 * time.Second)

		// Check again if process terminated gracefully
		if !wpm.IsProcessRunning(ctx, pid) {
			return nil
		}
	}

	// Terminate the process
	ret, _, err := terminateProcess.Call(handle, 1) // Exit code 1
	if ret == 0 {
		return fmt.Errorf("failed to terminate process %d: %v", pid, err)
	}

	return nil
}

// KillProcessByName terminates all processes matching a name pattern on Windows
func (wpm *WindowsProcessMonitor) KillProcessByName(ctx context.Context, namePattern string, graceful bool) error {
	processes, err := wpm.GetProcesses(ctx)
	if err != nil {
		return fmt.Errorf("failed to get process list: %w", err)
	}

	var killedCount int
	var errors []error

	for _, process := range processes {
		// Check if process name matches pattern
		matched, err := filepath.Match(namePattern, process.Name)
		if err != nil {
			errors = append(errors, fmt.Errorf("invalid pattern %s: %w", namePattern, err))
			continue
		}

		if matched {
			if err := wpm.KillProcess(ctx, process.PID, graceful); err != nil {
				errors = append(errors, fmt.Errorf("failed to kill process %s (PID %d): %w", process.Name, process.PID, err))
			} else {
				killedCount++
			}
		}
	}

	if killedCount == 0 && len(errors) == 0 {
		return fmt.Errorf("no processes found matching pattern: %s", namePattern)
	}

	if len(errors) > 0 {
		return fmt.Errorf("killed %d processes, but encountered %d errors: %v", killedCount, len(errors), errors)
	}

	return nil
}

// Platform-specific factory function for Windows
func newPlatformProcessMonitor(pollInterval time.Duration) ProcessMonitor {
	return NewWindowsProcessMonitor(pollInterval)
}
