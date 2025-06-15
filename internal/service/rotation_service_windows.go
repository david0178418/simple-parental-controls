//go:build windows

package service

import (
	"syscall"
	"time"
	"unsafe"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getCurrentDiskSpace returns disk space information on Windows
func (s *LogRotationService) getCurrentDiskSpace() *models.DiskSpaceInfo {
	// Get current working directory to check disk space
	pwd, err := syscall.Getwd()
	if err != nil {
		s.logger.Error("Failed to get current directory", logging.Err(err))
		return nil
	}

	// Convert to UTF16 for Windows API
	pwdUTF16, err := syscall.UTF16PtrFromString(pwd)
	if err != nil {
		s.logger.Error("Failed to convert path to UTF16", logging.Err(err))
		return nil
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	// Call GetDiskFreeSpaceEx
	ret, _, err := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pwdUTF16)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		s.logger.Error("Failed to get disk space info", logging.Err(err))
		return nil
	}

	totalSpace := int64(totalBytes)
	freeSpace := int64(freeBytesAvailable)
	usedSpace := totalSpace - freeSpace
	usagePercent := float64(usedSpace) / float64(totalSpace)

	return &models.DiskSpaceInfo{
		TotalSpace:   totalSpace,
		UsedSpace:    usedSpace,
		FreeSpace:    freeSpace,
		UsagePercent: usagePercent,
		LastUpdated:  time.Now(),
	}
}
