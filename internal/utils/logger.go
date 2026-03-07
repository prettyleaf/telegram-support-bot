package utils

import (
	"os"
	"strings"
	"unicode/utf8"

	"remnafamily/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

// InitLogger initializes the global logger
func InitLogger() error {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.Encoding = "console"
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	// Default to info level
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	return nil
}

// UpdateLogLevel updates the logger level from YAML config
func UpdateLogLevel() error {
	if config.YAMLCfg == nil {
		return nil
	}

	level := strings.ToLower(config.YAMLCfg.Bot.LogLevel)
	var zapLevel zapcore.Level
	
	switch level {
	case "debug":
		zapLevel = zap.DebugLevel
	case "info":
		zapLevel = zap.InfoLevel
	case "warn", "warning":
		zapLevel = zap.WarnLevel
	case "error":
		zapLevel = zap.ErrorLevel
	default:
		zapLevel = zap.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.Encoding = "console"
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	return nil
}

// Sync flushes the logger
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// LogSystem logs a system-level message
func LogSystem(format string, args ...interface{}) {
	Logger.Infof("🔧 "+format, args...)
}

// LogTicket logs a ticket-related message
func LogTicket(format string, args ...interface{}) {
	Logger.Infof("🎫 "+format, args...)
}

// LogAdmin logs an admin action
func LogAdmin(format string, args ...interface{}) {
	Logger.Infof("👤 "+format, args...)
}

// LogDebug logs a debug message
func LogDebug(format string, args ...interface{}) {
	Logger.Debugf("🔍 "+format, args...)
}

// LogError logs an error with context
func LogError(context string, err error) {
	Logger.Errorf("❌ %s: %v", context, err)
}

// GetEnv gets an environment variable with a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SanitizeUTF8 removes invalid UTF-8 byte sequences from a string
func SanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Build a valid UTF-8 string by replacing invalid sequences
	var result strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			// Invalid byte, skip it
			s = s[1:]
			continue
		}
		result.WriteRune(r)
		s = s[size:]
	}
	return result.String()
}

// SanitizeUTF8Ptr sanitizes a string pointer, returning nil if input is nil
func SanitizeUTF8Ptr(s *string) *string {
	if s == nil {
		return nil
	}
	sanitized := SanitizeUTF8(*s)
	return &sanitized
}
