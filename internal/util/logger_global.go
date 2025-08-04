package util

import (
	"sync"
)

var (
	globalLogger LoggerInterface
	loggerOnce   sync.Once
)

// InitLogger initializes the global logger instance with debug mode support
func InitLogger(logLevel, logFile string, debugToConsole bool) {
	loggerOnce.Do(func() {
		globalLogger = NewLogger(logLevel, logFile, debugToConsole)
	})
}

// LogInfo convenience functions for logging
func LogInfo(msg string) {
	if globalLogger != nil {
		globalLogger.Info(msg)
	}
}

func LogInfof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

func LogDebug(msg string) {
	if globalLogger != nil {
		globalLogger.Debug(msg)
	}
}

func LogDebugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

func LogWarn(msg string) {
	if globalLogger != nil {
		globalLogger.Warn(msg)
	}
}

func LogWarnf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

func LogError(msg string) {
	if globalLogger != nil {
		globalLogger.Error(msg)
	}
}

func LogErrorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}
