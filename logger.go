package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger provides thread-safe logging to file
type Logger struct {
	file  *os.File
	mutex sync.Mutex
}

var globalLogger *Logger

// InitLogger creates a new logger that writes to a file
func InitLogger(filepath string) error {
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	globalLogger = &Logger{
		file: f,
	}

	globalLogger.Info("=== Music Visualizer Started ===")
	return nil
}

// Close closes the log file
func CloseLogger() {
	if globalLogger != nil {
		globalLogger.Info("=== Music Visualizer Stopped ===")
		globalLogger.file.Close()
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.write("INFO", format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.write("ERROR", format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.write("DEBUG", format, args...)
}

// Panic logs a panic with full context
func (l *Logger) Panic(panicValue interface{}, context string) {
	l.write("PANIC", "%s: %v", context, panicValue)
}

func (l *Logger) write(level, format string, args ...interface{}) {
	if l == nil {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

	io.WriteString(l.file, logLine)
	l.file.Sync() // Ensure written immediately
}

// Convenience functions for global logger
func LogInfo(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

func LogError(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

func LogDebug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, args...)
	}
}

func LogPanic(panicValue interface{}, context string) {
	if globalLogger != nil {
		globalLogger.Panic(panicValue, context)
	}
}
