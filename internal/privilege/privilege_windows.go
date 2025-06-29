package privilege

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type windowsManager struct {
	config *Config
}

func newPlatformManager(config *Config) Manager {
	return &windowsManager{config: config}
}

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	procGetCurrentProcess     = kernel32.NewProc("GetCurrentProcess")
	procOpenProcessToken      = advapi32.NewProc("OpenProcessToken")
	procGetTokenInformation   = advapi32.NewProc("GetTokenInformation")
	procCloseHandle           = kernel32.NewProc("CloseHandle")
	procShellExecuteW         = shell32.NewProc("ShellExecuteW")
	procCreateFileW           = kernel32.NewProc("CreateFileW")
)

const (
	TOKEN_QUERY         = 0x0008
	TokenElevationType  = 18
	TokenElevated       = 1
	TokenElevationTypeFull = 2
	GENERIC_READ        = 0x80000000
	OPEN_EXISTING       = 3
	FILE_ATTRIBUTE_NORMAL = 0x80
)

type TOKEN_ELEVATION_TYPE int32

const (
	TokenElevationTypeDefault TOKEN_ELEVATION_TYPE = 1
	TokenElevationTypeFull    TOKEN_ELEVATION_TYPE = 2
	TokenElevationTypeLimited TOKEN_ELEVATION_TYPE = 3
)

func (m *windowsManager) IsElevated() bool {
	return m.isUserAdmin() || m.isTokenElevated()
}

func (m *windowsManager) isUserAdmin() bool {
	handle, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	
	var elevation uint32
	var returnedLen uint32
	
	ret, _, _ := procGetTokenInformation.Call(
		uintptr(handle),
		uintptr(TokenElevated),
		uintptr(unsafe.Pointer(&elevation)),
		uintptr(unsafe.Sizeof(elevation)),
		uintptr(unsafe.Pointer(&returnedLen)),
	)
	
	return ret != 0 && elevation != 0
}

func (m *windowsManager) isTokenElevated() bool {
	handle, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	
	var elevationType TOKEN_ELEVATION_TYPE
	var returnedLen uint32
	
	ret, _, _ := procGetTokenInformation.Call(
		uintptr(handle),
		uintptr(TokenElevationType),
		uintptr(unsafe.Pointer(&elevationType)),
		uintptr(unsafe.Sizeof(elevationType)),
		uintptr(unsafe.Pointer(&returnedLen)),
	)
	
	return ret != 0 && elevationType == TokenElevationTypeFull
}

func (m *windowsManager) canAccessPhysicalDrive() bool {
	drive := `\\.\PHYSICALDRIVE0`
	drivePtr, _ := syscall.UTF16PtrFromString(drive)
	
	handle, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(drivePtr)),
		uintptr(GENERIC_READ),
		0,
		0,
		uintptr(OPEN_EXISTING),
		uintptr(FILE_ATTRIBUTE_NORMAL),
		0,
	)
	
	if handle != 0 && handle != ^uintptr(0) {
		procCloseHandle.Call(handle)
		return true
	}
	
	return false
}

func (m *windowsManager) CanElevate() bool {
	return true
}

func (m *windowsManager) GetElevationMethod() ElevationMethod {
	return ElevationMethodUAC
}

func (m *windowsManager) RequestElevation(ctx context.Context, reason string) error {
	if m.IsElevated() {
		return ErrAlreadyElevated
	}
	
	return m.RestartElevated(ctx, os.Args)
}

func (m *windowsManager) RestartElevated(ctx context.Context, args []string) error {
	if m.IsElevated() {
		return ErrAlreadyElevated
	}
	
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	resolvedExe, err := filepath.EvalSymlinks(executable)
	if err != nil {
		resolvedExe = executable
	}
	
	timeout := time.Duration(m.config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	parameters := ""
	if len(args) > 1 {
		parameters = strings.Join(args[1:], " ")
	}
	
	err = m.shellExecuteElevated(resolvedExe, parameters)
	if err != nil {
		return fmt.Errorf("UAC elevation failed: %w", err)
	}
	
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
	
	select {
	case <-ctx.Done():
		return ErrElevationTimeout
	case <-time.After(1 * time.Second):
		return nil
	}
}

func (m *windowsManager) shellExecuteElevated(executable, parameters string) error {
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(executable)
	params, _ := syscall.UTF16PtrFromString(parameters)
	
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		0,
		1, // SW_SHOWNORMAL
	)
	
	if ret <= 32 {
		switch ret {
		case 2:
			return fmt.Errorf("file not found")
		case 3:
			return fmt.Errorf("path not found")
		case 5:
			return ErrElevationDenied
		case 8:
			return fmt.Errorf("insufficient memory")
		case 26:
			return fmt.Errorf("sharing violation")
		case 27:
			return fmt.Errorf("filename association incomplete")
		case 28:
			return ErrElevationTimeout
		case 29:
			return fmt.Errorf("DDE transaction failed")
		case 30:
			return fmt.Errorf("DDE transaction busy")
		case 31:
			return fmt.Errorf("no association for file type")
		case 32:
			return fmt.Errorf("DLL not found")
		default:
			return fmt.Errorf("elevation failed with code %d", ret)
		}
	}
	
	return nil
}

func (m *windowsManager) isRunningAsService() bool {
	return os.Getenv("USERNAME") == "SYSTEM" || 
		   os.Getenv("SESSIONNAME") == "Services" ||
		   os.Getenv("USERDNSDOMAIN") != ""
}

func (m *windowsManager) checkUACSettings() (bool, error) {
	cmd := exec.Command("reg", "query", 
		"HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System",
		"/v", "EnableLUA")
	
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check UAC settings: %w", err)
	}
	
	return strings.Contains(string(output), "0x1"), nil
}

func (m *windowsManager) getElevationTypeDescription() string {
	handle, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "unknown"
	}
	defer syscall.CloseHandle(handle)
	
	var elevationType TOKEN_ELEVATION_TYPE
	var returnedLen uint32
	
	ret, _, _ := procGetTokenInformation.Call(
		uintptr(handle),
		uintptr(TokenElevationType),
		uintptr(unsafe.Pointer(&elevationType)),
		uintptr(unsafe.Sizeof(elevationType)),
		uintptr(unsafe.Pointer(&returnedLen)),
	)
	
	if ret == 0 {
		return "query_failed"
	}
	
	switch elevationType {
	case TokenElevationTypeDefault:
		return "default"
	case TokenElevationTypeFull:
		return "elevated"
	case TokenElevationTypeLimited:
		return "limited"
	default:
		return "unknown"
	}
}

func (m *windowsManager) checkWindowsVersion() (string, error) {
	cmd := exec.Command("ver")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

func (m *windowsManager) isUACEnabled() bool {
	enabled, err := m.checkUACSettings()
	if err != nil {
		return true
	}
	return enabled
}

func (m *windowsManager) createCompatibilityManifest() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	
	manifestPath := executable + ".manifest"
	manifestContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity
    version="1.0.0.0"
    processorArchitecture="*"
    name="ParentalControl"
    type="win32"
  />
  <description>Parental Control Application</description>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v2">
    <security>
      <requestedPrivileges xmlns="urn:schemas-microsoft-com:asm.v3">
        <requestedExecutionLevel level="requireAdministrator" uiAccess="false" />
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{e2011457-1546-43c5-a5fe-008deee3d3f0}"/>
      <supportedOS Id="{35138b9a-5d96-4fbd-8e2d-a2440225f93a}"/>
      <supportedOS Id="{4a2f28e3-53b9-4441-ba9c-d69d4a4a6e38}"/>
      <supportedOS Id="{1f676c76-80e1-4239-95bb-83d0f6d0da78}"/>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
</assembly>`
	
	return os.WriteFile(manifestPath, []byte(manifestContent), 0644)
}