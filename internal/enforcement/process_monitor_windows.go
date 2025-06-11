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
	TH32CS_SNAPPROCESS   = 0x00000002
	INVALID_HANDLE_VALUE = ^uintptr(0)
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	createToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First            = kernel32.NewProc("Process32FirstW")
	process32Next             = kernel32.NewProc("Process32NextW")
	closeHandle               = kernel32.NewProc("CloseHandle")
	openProcess               = kernel32.NewProc("OpenProcess")
	queryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
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

// Platform-specific factory function for Windows
func newPlatformProcessMonitor(pollInterval time.Duration) ProcessMonitor {
	return NewWindowsProcessMonitor(pollInterval)
}
