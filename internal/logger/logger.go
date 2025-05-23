// Package logger provides a simple logging interface using zerolog
package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Level represents the logging level
type Level string

// Available log levels
const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelFatal Level = "fatal"
)

// Format represents the logging format
type Format string

// Available log formats
const (
	FormatJSON   Format = "json"
	FormatPretty Format = "pretty"
)

// Config holds logger configuration
type Config struct {
	// Level sets the minimum log level to output
	Level Level `yaml:"level"`
	// Format defines the output format (json, pretty)
	Format Format `yaml:"format"`
	// Output defines where logs are written (stdout, stderr, file)
	Output string `yaml:"output"`
	// File is the file path when Output is set to "file"
	File string `yaml:"file,omitempty"`
	// IncludeCaller adds caller information to log entries
	IncludeCaller bool `yaml:"includeCaller"`
	// TimeFormat specifies the time format for logs
	TimeFormat string `yaml:"timeFormat,omitempty"`
	// DisableTimestamp disables adding timestamp to logs
	DisableTimestamp bool `yaml:"disableTimestamp,omitempty"`
}

var (
	// Default configuration
	defaultConfig = Config{
		Level:         LevelInfo,
		Format:        FormatJSON,
		Output:        "stdout",
		IncludeCaller: false,
		TimeFormat:    time.RFC3339,
	}

	// Instance of the zerolog logger
	instance zerolog.Logger

	// Flag to track initialization
	initialized bool
)

// Initialize sets up the logger with the provided configuration
func Initialize(cfg Config) {
	// Apply defaults for empty values
	if cfg.Level == "" {
		cfg.Level = defaultConfig.Level
	}
	if cfg.Format == "" {
		cfg.Format = defaultConfig.Format
	}
	if cfg.Output == "" {
		cfg.Output = defaultConfig.Output
	}
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = defaultConfig.TimeFormat
	}

	// Set up the zerolog level
	var level zerolog.Level
	switch strings.ToLower(string(cfg.Level)) {
	case string(LevelDebug):
		level = zerolog.DebugLevel
	case string(LevelInfo):
		level = zerolog.InfoLevel
	case string(LevelWarn):
		level = zerolog.WarnLevel
	case string(LevelError):
		level = zerolog.ErrorLevel
	case string(LevelFatal):
		level = zerolog.FatalLevel
	default:
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output writer
	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.File != "" {
			file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				// If we can't open the file, fall back to stdout
				output = os.Stdout
				// Use a temporary logger to report the error
				tmpLogger := zerolog.New(output)
				tmpLogger.Error().Err(err).Msg("Failed to open log file, using stdout")
			} else {
				output = file
			}
		} else {
			output = os.Stdout
		}
	default:
		output = os.Stdout
	}

	// Configure time format
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// Configure logger format
	if strings.EqualFold(string(cfg.Format), string(FormatPretty)) && (cfg.Output == "stdout" || cfg.Output == "stderr") {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: cfg.TimeFormat,
			NoColor:    false,
		}
	}

	// Create the logger
	contextLogger := zerolog.New(output)

	// Add timestamp if not disabled
	if !cfg.DisableTimestamp {
		contextLogger = contextLogger.With().Timestamp().Logger()
	}

	// Add caller information if enabled
	if cfg.IncludeCaller {
		contextLogger = contextLogger.With().Caller().Logger()
	}

	instance = contextLogger
	initialized = true

	// Log the initialization at debug level
	instance.Debug().Str("level", string(cfg.Level)).Str("format", string(cfg.Format)).Msg("Logger initialized")
}

// ensureInitialized makes sure the logger is initialized
func ensureInitialized() {
	if !initialized {
		Initialize(defaultConfig)
	}
}

// GetLogger returns the configured zerolog logger
func GetLogger() *zerolog.Logger {
	ensureInitialized()
	return &instance
}

// Debug logs a message at debug level
func Debug(msg string) {
	ensureInitialized()
	instance.Debug().Msg(msg)
}

// DebugWithFields logs a message at debug level with additional fields
func DebugWithFields(msg string, fields map[string]interface{}) {
	ensureInitialized()
	event := instance.Debug()
	for k, v := range fields {
		event.Interface(k, v)
	}
	event.Msg(msg)
}

// Info logs a message at info level
func Info(msg string) {
	ensureInitialized()
	instance.Info().Msg(msg)
}

// InfoWithFields logs a message at info level with additional fields
func InfoWithFields(msg string, fields map[string]interface{}) {
	ensureInitialized()
	event := instance.Info()
	for k, v := range fields {
		event.Interface(k, v)
	}
	event.Msg(msg)
}

// Warn logs a message at warn level
func Warn(msg string) {
	ensureInitialized()
	instance.Warn().Msg(msg)
}

// WarnWithFields logs a message at warn level with additional fields
func WarnWithFields(msg string, fields map[string]interface{}) {
	ensureInitialized()
	event := instance.Warn()
	for k, v := range fields {
		event.Interface(k, v)
	}
	event.Msg(msg)
}

// Error logs a message at error level
func Error(msg string, err error) {
	ensureInitialized()
	event := instance.Error()
	if err != nil {
		event.Err(err)
	}
	event.Msg(msg)
}

// ErrorWithFields logs a message at error level with additional fields
func ErrorWithFields(msg string, err error, fields map[string]interface{}) {
	ensureInitialized()
	event := instance.Error()
	if err != nil {
		event.Err(err)
	}
	for k, v := range fields {
		event.Interface(k, v)
	}
	event.Msg(msg)
}

// Fatal logs a message at fatal level and then exits
func Fatal(msg string, err error) {
	ensureInitialized()
	event := instance.Fatal()
	if err != nil {
		event.Err(err)
	}
	event.Msg(msg)
}

// FatalWithFields logs a message at fatal level with additional fields and then exits
func FatalWithFields(msg string, err error, fields map[string]interface{}) {
	ensureInitialized()
	event := instance.Fatal()
	if err != nil {
		event.Err(err)
	}
	for k, v := range fields {
		event.Interface(k, v)
	}
	event.Msg(msg)
}
