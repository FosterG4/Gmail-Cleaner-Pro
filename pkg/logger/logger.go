package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log  *zap.Logger
	once sync.Once
)

// LogLevel represents the logging level
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)

// Config holds logger configuration
type Config struct {
	Level      LogLevel `env:"LOG_LEVEL" envDefault:"info"`
	Format     string   `env:"LOG_FORMAT" envDefault:"json"` // json or console
	OutputPath string   `env:"LOG_OUTPUT_PATH" envDefault:"stdout"`
	ErrorPath  string   `env:"LOG_ERROR_PATH" envDefault:"stderr"`
	MaxSize    int      `env:"LOG_MAX_SIZE" envDefault:"100"` // MB
	MaxBackups int      `env:"LOG_MAX_BACKUPS" envDefault:"3"`
	MaxAge     int      `env:"LOG_MAX_AGE" envDefault:"28"` // days
	Compress   bool     `env:"LOG_COMPRESS" envDefault:"true"`
}

// InitLogger initializes the global logger with configuration
func InitLogger(config *Config) error {
	var err error
	once.Do(func() {
		log, err = NewLogger(config)
	})
	return err
}

// NewLogger creates a new logger instance with the given configuration
func NewLogger(config *Config) (*zap.Logger, error) {
	if config == nil {
		config = &Config{
			Level:      InfoLevel,
			Format:     "json",
			OutputPath: "stdout",
			ErrorPath:  "stderr",
		}
	}

	// Configure log level
	level := zapcore.InfoLevel
	switch config.Level {
	case DebugLevel:
		level = zapcore.DebugLevel
	case InfoLevel:
		level = zapcore.InfoLevel
	case WarnLevel:
		level = zapcore.WarnLevel
	case ErrorLevel:
		level = zapcore.ErrorLevel
	case FatalLevel:
		level = zapcore.FatalLevel
	}

	// Configure output paths
	outputPaths := []string{config.OutputPath}
	errorPaths := []string{config.ErrorPath}

	// Configure encoder config based on format
	var encoderConfig zapcore.EncoderConfig
	if config.Format == "console" {
		encoderConfig = getConsoleEncoderConfig()
	} else {
		encoderConfig = getJSONEncoderConfig()
	}

	// Create logger configuration
	loggerConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         config.Format,
		EncoderConfig:    encoderConfig,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorPaths,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
	}

	// Build logger
	logger, err := loggerConfig.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// getJSONEncoderConfig returns the JSON encoder configuration
func getJSONEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// getConsoleEncoderConfig returns the console encoder configuration
func getConsoleEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// L returns the global logger instance
func L() *zap.Logger {
	once.Do(func() {
		// Initialize with default config if not already initialized
		config := &Config{
			Level:      LogLevel(getEnvOrDefault("LOG_LEVEL", "info")),
			Format:     getEnvOrDefault("LOG_FORMAT", "json"),
			OutputPath: getEnvOrDefault("LOG_OUTPUT_PATH", "stdout"),
			ErrorPath:  getEnvOrDefault("LOG_ERROR_PATH", "stderr"),
		}
		var err error
		log, err = NewLogger(config)
		if err != nil {
			// Fallback to basic production logger
			log, _ = zap.NewProduction()
		}
	})
	return log
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// RequestLogger creates a logger with request context
func RequestLogger(requestID, method, path string) *zap.Logger {
	return L().With(
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("component", "http"),
	)
}

// ServiceLogger creates a logger for service layer
func ServiceLogger(service string) *zap.Logger {
	return L().With(
		zap.String("component", "service"),
		zap.String("service", service),
	)
}

// RepositoryLogger creates a logger for repository layer
func RepositoryLogger(repository string) *zap.Logger {
	return L().With(
		zap.String("component", "repository"),
		zap.String("repository", repository),
	)
}

// HandlerLogger creates a logger for handler layer
func HandlerLogger(handler string) *zap.Logger {
	return L().With(
		zap.String("component", "handler"),
		zap.String("handler", handler),
	)
}

// Sync flushes any buffered log entries
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}
