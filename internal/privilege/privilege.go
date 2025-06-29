package privilege

import (
	"context"
	"errors"
	"os"
)

var (
	ErrElevationDenied    = errors.New("privilege elevation was denied by user")
	ErrElevationFailed    = errors.New("privilege elevation failed")
	ErrNotSupported       = errors.New("privilege elevation not supported on this platform")
	ErrAlreadyElevated    = errors.New("process is already running with elevated privileges")
	ErrElevationTimeout   = errors.New("privilege elevation request timed out")
)

type ElevationMethod int

const (
	ElevationMethodAuto ElevationMethod = iota
	ElevationMethodUAC
	ElevationMethodSudo
	ElevationMethodPkexec
)

type Manager interface {
	IsElevated() bool
	CanElevate() bool
	RequestElevation(ctx context.Context, reason string) error
	RestartElevated(ctx context.Context, args []string) error
	GetElevationMethod() ElevationMethod
}

type Config struct {
	Method              ElevationMethod
	TimeoutSeconds      int
	AllowFallback       bool
	PreferredElevator   string
	RestartOnElevation  bool
}

func DefaultConfig() *Config {
	return &Config{
		Method:              ElevationMethodAuto,
		TimeoutSeconds:      30,
		AllowFallback:       true,
		RestartOnElevation:  true,
	}
}

func NewManager(config *Config) Manager {
	if config == nil {
		config = DefaultConfig()
	}
	
	return newPlatformManager(config)
}

func IsElevated() bool {
	manager := NewManager(nil)
	return manager.IsElevated()
}

func RequestElevation(ctx context.Context, reason string) error {
	manager := NewManager(nil)
	return manager.RequestElevation(ctx, reason)
}

func RestartElevated(ctx context.Context) error {
	manager := NewManager(nil)
	return manager.RestartElevated(ctx, os.Args)
}