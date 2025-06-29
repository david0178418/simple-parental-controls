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
)

type linuxManager struct {
	config *Config
}

func newPlatformManager(config *Config) Manager {
	return &linuxManager{config: config}
}

func (m *linuxManager) IsElevated() bool {
	return os.Geteuid() == 0
}

func (m *linuxManager) CanElevate() bool {
	if m.IsElevated() {
		return true
	}
	
	methods := m.getAvailableMethods()
	return len(methods) > 0
}

func (m *linuxManager) getAvailableMethods() []string {
	var methods []string
	
	if _, err := exec.LookPath("pkexec"); err == nil {
		methods = append(methods, "pkexec")
	}
	
	if _, err := exec.LookPath("sudo"); err == nil {
		methods = append(methods, "sudo")
	}
	
	if _, err := exec.LookPath("gksudo"); err == nil {
		methods = append(methods, "gksudo")
	}
	
	if _, err := exec.LookPath("kdesudo"); err == nil {
		methods = append(methods, "kdesudo")
	}
	
	return methods
}

func (m *linuxManager) GetElevationMethod() ElevationMethod {
	switch m.config.Method {
	case ElevationMethodSudo:
		return ElevationMethodSudo
	case ElevationMethodPkexec:
		return ElevationMethodPkexec
	default:
		methods := m.getAvailableMethods()
		if len(methods) > 0 {
			switch methods[0] {
			case "pkexec":
				return ElevationMethodPkexec
			case "sudo":
				return ElevationMethodSudo
			}
		}
		return ElevationMethodSudo
	}
}

func (m *linuxManager) RequestElevation(ctx context.Context, reason string) error {
	if m.IsElevated() {
		return ErrAlreadyElevated
	}
	
	if !m.CanElevate() {
		return ErrNotSupported
	}
	
	return m.RestartElevated(ctx, os.Args)
}

func (m *linuxManager) RestartElevated(ctx context.Context, args []string) error {
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
	
	methods := m.getAvailableMethods()
	if len(methods) == 0 {
		return ErrNotSupported
	}
	
	method := m.selectElevationMethod(methods)
	
	timeout := time.Duration(m.config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	var cmd *exec.Cmd
	switch method {
	case "pkexec":
		allArgs := append([]string{resolvedExe}, args[1:]...)
		cmd = exec.CommandContext(ctx, "pkexec", allArgs...)
	case "sudo":
		if isDesktopEnvironment() {
			allArgs := append([]string{"-A", resolvedExe}, args[1:]...)
			cmd = exec.CommandContext(ctx, "sudo", allArgs...)
		} else {
			allArgs := append([]string{resolvedExe}, args[1:]...)
			cmd = exec.CommandContext(ctx, "sudo", allArgs...)
		}
	case "gksudo":
		allArgs := append([]string{resolvedExe}, args[1:]...)
		cmd = exec.CommandContext(ctx, "gksudo", allArgs...)
	case "kdesudo":
		allArgs := append([]string{resolvedExe}, args[1:]...)
		cmd = exec.CommandContext(ctx, "kdesudo", allArgs...)
	default:
		return ErrNotSupported
	}
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	if method == "sudo" && isDesktopEnvironment() {
		cmd.Env = append(os.Environ(), "SUDO_ASKPASS="+getSudoAskpassPath())
	}
	
	err = cmd.Start()
	if err != nil {
		if m.config.AllowFallback && len(methods) > 1 {
			return m.tryFallbackMethod(ctx, args, methods, method)
		}
		return fmt.Errorf("failed to start elevated process: %w", err)
	}
	
	// Wait for the command to complete or timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		return ErrElevationTimeout
	case err := <-done:
		if err != nil {
			if m.config.AllowFallback && len(methods) > 1 {
				return m.tryFallbackMethod(ctx, args, methods, method)
			}
			return fmt.Errorf("elevation process failed: %w", err)
		}
		// If we get here, the elevated process started successfully
		// Exit the current process since the elevated one is now running
		os.Exit(0)
		return nil
	}
}

func (m *linuxManager) selectElevationMethod(methods []string) string {
	if m.config.PreferredElevator != "" {
		for _, method := range methods {
			if method == m.config.PreferredElevator {
				return method
			}
		}
	}
	
	switch m.config.Method {
	case ElevationMethodPkexec:
		for _, method := range methods {
			if method == "pkexec" {
				return method
			}
		}
	case ElevationMethodSudo:
		for _, method := range methods {
			if method == "sudo" {
				return method
			}
		}
	}
	
	if isDesktopEnvironment() {
		for _, method := range []string{"pkexec", "gksudo", "kdesudo", "sudo"} {
			for _, available := range methods {
				if method == available {
					return method
				}
			}
		}
	}
	
	return methods[0]
}

func (m *linuxManager) tryFallbackMethod(ctx context.Context, args []string, methods []string, failedMethod string) error {
	for _, method := range methods {
		if method == failedMethod {
			continue
		}
		
		originalMethod := m.config.Method
		if method == "pkexec" {
			m.config.Method = ElevationMethodPkexec
		} else if method == "sudo" {
			m.config.Method = ElevationMethodSudo
		}
		
		err := m.RestartElevated(ctx, args)
		m.config.Method = originalMethod
		
		if err == nil {
			return nil
		}
	}
	
	return ErrElevationFailed
}

func isDesktopEnvironment() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func getSudoAskpassPath() string {
	askpassPaths := []string{
		"/usr/bin/ssh-askpass",
		"/usr/lib/ssh/ssh-askpass",
		"/usr/libexec/openssh/ssh-askpass",
		"/usr/bin/ksshaskpass",
		"/usr/bin/x11-ssh-askpass",
	}
	
	for _, path := range askpassPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	return "/usr/bin/ssh-askpass"
}

func createPolkitPolicy() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	
	policyDir := "/usr/share/polkit-1/actions"
	if _, err := os.Stat(policyDir); os.IsNotExist(err) {
		return nil
	}
	
	policyContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE policyconfig PUBLIC "-//freedesktop//DTD PolicyKit Policy Configuration 1.0//EN"
        "http://www.freedesktop.org/standards/PolicyKit/1/policyconfig.dtd">
<policyconfig>
    <action id="com.parental-control.run">
        <description>Run Parental Control Application</description>
        <message>Authentication is required to run the parental control application with administrator privileges</message>
        <defaults>
            <allow_any>auth_admin</allow_any>
            <allow_inactive>auth_admin</allow_inactive>
            <allow_active>auth_admin</allow_active>
        </defaults>
        <annotate key="org.freedesktop.policykit.exec.path">%s</annotate>
        <annotate key="org.freedesktop.policykit.exec.allow_gui">true</annotate>
    </action>
</policyconfig>`, executable)
	
	policyPath := filepath.Join(policyDir, "com.parental-control.policy")
	
	return os.WriteFile(policyPath, []byte(policyContent), 0644)
}

func checkSudoVulnerabilities() error {
	out, err := exec.Command("sudo", "--version").Output()
	if err != nil {
		return nil
	}
	
	version := string(out)
	if strings.Contains(version, "1.8.") || strings.Contains(version, "1.9.") {
		return fmt.Errorf("detected potentially vulnerable sudo version - ensure patches for CVE-2021-3156 are applied")
	}
	
	return nil
}

func checkPkexecVulnerabilities() error {
	if _, err := exec.LookPath("pkexec"); err != nil {
		return nil
	}
	
	stat, err := os.Stat("/usr/bin/pkexec")
	if err != nil {
		return nil
	}
	
	if stat.Mode()&os.ModeSetuid == 0 {
		return nil
	}
	
	if stat.Sys().(*syscall.Stat_t).Uid != 0 {
		return fmt.Errorf("pkexec has incorrect ownership - potential security risk")
	}
	
	return nil
}