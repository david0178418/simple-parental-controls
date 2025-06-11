//go:build !windows

package enforcement

import (
	"time"
)

// Platform-specific factory function for Linux/Unix
func newPlatformProcessMonitor(pollInterval time.Duration) ProcessMonitor {
	return NewLinuxProcessMonitor(pollInterval)
}
