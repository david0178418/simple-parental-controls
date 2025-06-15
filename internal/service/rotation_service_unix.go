//go:build !windows

package service

import (
	"syscall"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// getCurrentDiskSpace returns disk space information on Unix-like systems
func (s *LogRotationService) getCurrentDiskSpace() *models.DiskSpaceInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		s.logger.Error("Failed to get disk space info", logging.Err(err))
		return nil
	}

	totalSpace := int64(stat.Blocks) * int64(stat.Bsize)
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
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
