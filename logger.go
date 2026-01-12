package rivo

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LogLevel represents a log level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
}

// JobLogger provides logging capabilities within a job context.
type JobLogger struct {
	mu      sync.Mutex
	entries []LogEntry
}

// NewJobLogger creates a new JobLogger.
func NewJobLogger() *JobLogger {
	return &JobLogger{
		entries: make([]LogEntry, 0),
	}
}

// log adds a log entry with the given level and message.
func (l *JobLogger) log(level LogLevel, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	l.entries = append(l.entries, LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
}

// Debug logs a debug message.
func (l *JobLogger) Debug(format string, args ...any) {
	l.log(LogLevelDebug, format, args...)
}

// Info logs an info message.
func (l *JobLogger) Info(format string, args ...any) {
	l.log(LogLevelInfo, format, args...)
}

// Warn logs a warning message.
func (l *JobLogger) Warn(format string, args ...any) {
	l.log(LogLevelWarn, format, args...)
}

// Error logs an error message.
func (l *JobLogger) Error(format string, args ...any) {
	l.log(LogLevelError, format, args...)
}

// Print logs an info message (for compatibility with log.Logger).
func (l *JobLogger) Print(args ...any) {
	l.log(LogLevelInfo, "%s", fmt.Sprint(args...))
}

// Printf logs a formatted info message (for compatibility with log.Logger).
func (l *JobLogger) Printf(format string, args ...any) {
	l.log(LogLevelInfo, format, args...)
}

// Println logs an info message with newline (for compatibility with log.Logger).
func (l *JobLogger) Println(args ...any) {
	l.log(LogLevelInfo, "%s", fmt.Sprintln(args...))
}

// Entries returns all log entries.
func (l *JobLogger) Entries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

// JSON returns the log entries as JSON bytes.
func (l *JobLogger) JSON() json.RawMessage {
	l.mu.Lock()
	defer l.mu.Unlock()
	data, _ := json.Marshal(l.entries)
	return data
}

// Clear clears all log entries.
func (l *JobLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}
