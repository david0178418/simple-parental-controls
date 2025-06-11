package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity of log messages
type LogLevel int

const (
	// DEBUG level for debugging information
	DEBUG LogLevel = iota
	// INFO level for informational messages
	INFO
	// WARN level for warning messages
	WARN
	// ERROR level for error messages
	ERROR
	// FATAL level for fatal errors that cause program termination
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger interface defines the contract for logging implementations
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	SetLevel(level LogLevel)
}

// ConcreteLogger provides structured logging functionality
type ConcreteLogger struct {
	level  LogLevel
	logger *log.Logger
}

// Config holds the logger configuration
type Config struct {
	Level  LogLevel
	Output io.Writer
}

// New creates a new logger with the given configuration
func New(config Config) *ConcreteLogger {
	if config.Output == nil {
		config.Output = os.Stdout
	}

	return &ConcreteLogger{
		level:  config.Level,
		logger: log.New(config.Output, "", 0), // No default flags, we'll format ourselves
	}
}

// NewDefault creates a logger with default configuration
func NewDefault() *ConcreteLogger {
	return New(Config{
		Level:  INFO,
		Output: os.Stdout,
	})
}

// SetLevel changes the minimum log level
func (l *ConcreteLogger) SetLevel(level LogLevel) {
	l.level = level
}

// Debug logs a debug message
func (l *ConcreteLogger) Debug(msg string, fields ...Field) {
	if l.level <= DEBUG {
		l.log(DEBUG, msg, fields...)
	}
}

// Info logs an info message
func (l *ConcreteLogger) Info(msg string, fields ...Field) {
	if l.level <= INFO {
		l.log(INFO, msg, fields...)
	}
}

// Warn logs a warning message
func (l *ConcreteLogger) Warn(msg string, fields ...Field) {
	if l.level <= WARN {
		l.log(WARN, msg, fields...)
	}
}

// Error logs an error message
func (l *ConcreteLogger) Error(msg string, fields ...Field) {
	if l.level <= ERROR {
		l.log(ERROR, msg, fields...)
	}
}

// Fatal logs a fatal message and exits the program
func (l *ConcreteLogger) Fatal(msg string, fields ...Field) {
	l.log(FATAL, msg, fields...)
	os.Exit(1)
}

// log formats and writes the log message
func (l *ConcreteLogger) log(level LogLevel, msg string, fields ...Field) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	logLine := timestamp + " [" + level.String() + "] " + msg

	// Append fields if any
	for _, field := range fields {
		logLine += " " + field.String()
	}

	l.logger.Println(logLine)
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// String returns the string representation of the field
func (f Field) String() string {
	return f.Key + "=" + formatValue(f.Value)
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// formatValue formats a field value for logging
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return "\"" + v + "\""
	case int, int8, int16, int32, int64:
		return fmt.Sprint(v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(v)
	case float32, float64:
		return fmt.Sprint(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

// Global logger instance
var globalLogger *ConcreteLogger = NewDefault()

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *ConcreteLogger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *ConcreteLogger {
	return globalLogger
}

// Global logging functions
func Debug(msg string, fields ...Field) {
	globalLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	globalLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	globalLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	globalLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	globalLogger.Fatal(msg, fields...)
}
