package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"parental-control/internal/database"
	"parental-control/internal/enforcement"
	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// ServiceState represents the current state of the service
type ServiceState int

const (
	// StateStopped indicates the service is not running
	StateStopped ServiceState = iota
	// StateStarting indicates the service is in the process of starting
	StateStarting
	// StateRunning indicates the service is running normally
	StateRunning
	// StateStopping indicates the service is in the process of stopping
	StateStopping
	// StateError indicates the service is in an error state
	StateError
)

// String returns the string representation of the service state
func (s ServiceState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Config holds the service configuration
type Config struct {
	// PIDFile path for storing process ID
	PIDFile string
	// ShutdownTimeout for graceful shutdown
	ShutdownTimeout time.Duration
	// DatabaseConfig for database connection
	DatabaseConfig database.Config
	// HealthCheckInterval for periodic health checks
	HealthCheckInterval time.Duration
	// EnforcementConfig for enforcement engine
	EnforcementConfig enforcement.EnforcementConfig
	// EnforcementEnabled indicates if enforcement should be started
	EnforcementEnabled bool
}

// DefaultConfig returns a service configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		PIDFile:             "./data/parental-control.pid",
		ShutdownTimeout:     30 * time.Second,
		DatabaseConfig:      database.DefaultConfig(),
		HealthCheckInterval: 30 * time.Second,
		EnforcementConfig: enforcement.EnforcementConfig{
			ProcessPollInterval:    10 * time.Second,
			EnableNetworkFiltering: true,
			MaxConcurrentChecks:    5,
			CacheTimeout:           30 * time.Second,
			BlockUnknownProcesses:  false, // Start with safer defaults
			LogAllActivity:         true,
			EnableEmergencyMode:    false,
			EmergencyWhitelist:     []string{"192.168.1.1"},
		},
		EnforcementEnabled: true,
	}
}

// Service manages the application lifecycle
type Service struct {
	config            Config
	state             ServiceState
	stateMu           sync.RWMutex
	db                *database.DB
	repos             *models.RepositoryManager
	enforcementService *EnforcementService
	ctx               context.Context
	cancel            context.CancelFunc
	startTime         time.Time
	errors            []error
	errorsMu          sync.RWMutex
}

// New creates a new service instance with the given configuration
func New(config Config) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		config: config,
		state:  StateStopped,
		ctx:    ctx,
		cancel: cancel,
		errors: make([]error, 0),
	}
}

// Start initializes and starts the service
func (s *Service) Start() error {
	s.setState(StateStarting)
	s.startTime = time.Now()

	logging.Info("Starting Parental Control Service")

	// Initialize components in order
	if err := s.initializeDatabase(); err != nil {
		s.addError(fmt.Errorf("database initialization failed: %w", err))
		s.setState(StateError)
		return err
	}

	if err := s.initializeRepositories(); err != nil {
		s.addError(fmt.Errorf("repository initialization failed: %w", err))
		s.setState(StateError)
		return err
	}

	if err := s.initializeEnforcementService(); err != nil {
		s.addError(fmt.Errorf("enforcement service initialization failed: %w", err))
		s.setState(StateError)
		return err
	}

	if err := s.writePIDFile(); err != nil {
		s.addError(fmt.Errorf("PID file creation failed: %w", err))
		s.setState(StateError)
		return err
	}

	// Set up signal handling
	s.setupSignalHandling()

	// Start health check routine
	go s.healthCheckRoutine()

	s.setState(StateRunning)
	logging.Info("Service started successfully",
		logging.String("pid_file", s.config.PIDFile),
		logging.String("startup_time", time.Since(s.startTime).String()))

	return nil
}

// Stop gracefully shuts down the service
func (s *Service) Stop(ctx context.Context) error {
	s.setState(StateStopping)
	logging.Info("Stopping Parental Control Service")

	// Cancel context to signal all goroutines to stop
	s.cancel()

	// Create a timeout context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer shutdownCancel()

	// Cleanup in reverse order of initialization
	s.cleanup(shutdownCtx)

	s.setState(StateStopped)
	logging.Info("Service stopped successfully")

	return nil
}

// Restart stops and then starts the service
func (s *Service) Restart() error {
	logging.Info("Restarting service")

	if err := s.Stop(context.Background()); err != nil {
		return fmt.Errorf("failed to stop service during restart: %w", err)
	}

	// Brief pause before restart
	time.Sleep(1 * time.Second)

	return s.Start()
}

// GetState returns the current service state
func (s *Service) GetState() ServiceState {
	return s.getState()
}

// GetStatus returns detailed status information
func (s *Service) GetStatus() map[string]interface{} {
	s.stateMu.RLock()
	s.errorsMu.RLock()
	defer s.stateMu.RUnlock()
	defer s.errorsMu.RUnlock()

	status := map[string]interface{}{
		"state":      s.state.String(),
		"start_time": s.startTime,
		"uptime":     time.Since(s.startTime).String(),
		"pid":        os.Getpid(),
		"errors":     len(s.errors),
	}

	if len(s.errors) > 0 {
		errorStrings := make([]string, len(s.errors))
		for i, err := range s.errors {
			errorStrings[i] = err.Error()
		}
		status["error_details"] = errorStrings
	}

	if s.db != nil {
		dbStats, err := s.db.GetStats()
		if err == nil {
			status["database"] = dbStats
		}
	}

	if s.enforcementService != nil {
		status["enforcement"] = map[string]interface{}{
			"running": s.enforcementService.IsRunning(),
			"stats":   s.enforcementService.GetStats(),
			"system":  s.enforcementService.GetSystemInfo(),
		}
	}

	return status
}

// GetRepositoryManager returns the repository manager for use by API servers
func (s *Service) GetRepositoryManager() *models.RepositoryManager {
	return s.repos
}

// GetEnforcementService returns the enforcement service for use by API servers
func (s *Service) GetEnforcementService() *EnforcementService {
	return s.enforcementService
}

// IsHealthy performs a health check and returns the result
func (s *Service) IsHealthy() error {
	if s.getState() != StateRunning {
		return fmt.Errorf("service is not running (state: %s)", s.getState())
	}

	// Check database health
	if s.db != nil {
		if err := s.db.HealthCheck(); err != nil {
			return fmt.Errorf("database health check failed: %w", err)
		}
	}

	return nil
}

// Wait blocks until the service stops
func (s *Service) Wait() {
	<-s.ctx.Done()
}

// initializeDatabase creates the database connection and runs migrations
func (s *Service) initializeDatabase() error {
	logging.Info("Initializing database connection")

	db, err := database.New(s.config.DatabaseConfig)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}

	logging.Info("Initializing database schema")
	if err := db.InitializeSchema(); err != nil {
		db.Close()
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	s.db = db
	logging.Info("Database initialized successfully")

	return nil
}

// initializeRepositories creates the repository manager
func (s *Service) initializeRepositories() error {
	logging.Info("Initializing repositories")

	// Get database connection
	dbConn := s.db.Connection()

	// Initialize actual repository implementations
	s.repos = &models.RepositoryManager{
		List:      database.NewListRepository(dbConn),
		ListEntry: database.NewListEntryRepository(dbConn),
		AuditLog:  database.NewAuditLogRepository(dbConn),
		// Other repositories will be added as needed
	}

	logging.Info("Repositories initialized successfully")
	return nil
}

// initializeEnforcementService creates and starts the enforcement service
func (s *Service) initializeEnforcementService() error {
	if !s.config.EnforcementEnabled {
		logging.Info("Enforcement service disabled in configuration")
		return nil
	}

	logging.Info("Initializing enforcement service")

	s.enforcementService = NewEnforcementService(
		s.repos,
		logging.NewDefault(),
		s.config.EnforcementConfig,
	)

	if err := s.enforcementService.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start enforcement service: %w", err)
	}

	logging.Info("Enforcement service initialized successfully")
	return nil
}

// writePIDFile creates a PID file containing the current process ID
func (s *Service) writePIDFile() error {
	pidDir := filepath.Dir(s.config.PIDFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	pid := os.Getpid()
	pidStr := strconv.Itoa(pid)

	if err := os.WriteFile(s.config.PIDFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	logging.Info("PID file created", logging.String("path", s.config.PIDFile), logging.Int("pid", pid))
	return nil
}

// removePIDFile removes the PID file
func (s *Service) removePIDFile() {
	if err := os.Remove(s.config.PIDFile); err != nil && !os.IsNotExist(err) {
		logging.Error("Failed to remove PID file", logging.Err(err))
	} else {
		logging.Info("PID file removed", logging.String("path", s.config.PIDFile))
	}
}

// setupSignalHandling configures signal handlers for graceful shutdown
func (s *Service) setupSignalHandling() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signals
		logging.Info("Received shutdown signal", logging.String("signal", sig.String()))

		if err := s.Stop(context.Background()); err != nil {
			logging.Error("Error during shutdown", logging.Err(err))
			os.Exit(1)
		}
	}()
}

// healthCheckRoutine runs periodic health checks
func (s *Service) healthCheckRoutine() {
	ticker := time.NewTicker(s.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.IsHealthy(); err != nil {
				logging.Error("Health check failed", logging.Err(err))
				s.addError(err)
			}
		}
	}
}

// cleanup performs cleanup tasks during shutdown
func (s *Service) cleanup(ctx context.Context) {
	logging.Info("Performing cleanup tasks")

	// Stop enforcement service first
	if s.enforcementService != nil {
		if err := s.enforcementService.Stop(ctx); err != nil {
			logging.Error("Error stopping enforcement service", logging.Err(err))
		}
	}

	// Close database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			logging.Error("Error closing database", logging.Err(err))
		}
	}

	// Remove PID file
	s.removePIDFile()

	logging.Info("Cleanup completed")
}

// setState safely updates the service state
func (s *Service) setState(state ServiceState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	oldState := s.state
	s.state = state

	if oldState != state {
		logging.Info("Service state changed",
			logging.String("from", oldState.String()),
			logging.String("to", state.String()))
	}
}

// getState safely retrieves the current service state
func (s *Service) getState() ServiceState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

// addError safely adds an error to the error list
func (s *Service) addError(err error) {
	s.errorsMu.Lock()
	defer s.errorsMu.Unlock()

	s.errors = append(s.errors, err)

	// Keep only the last 10 errors to prevent memory growth
	if len(s.errors) > 10 {
		s.errors = s.errors[len(s.errors)-10:]
	}
}
