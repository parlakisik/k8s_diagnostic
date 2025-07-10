package diagnostic

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents different severity levels for logging
type LogLevel int

const (
	// Log levels from least to most severe
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger handles both console output and file logging
type Logger struct {
	logFile       *os.File
	logFilePath   string
	timestampFmt  string
	consoleOutput bool
	minLevel      LogLevel
	context       string // current context (e.g., test name, component)
}

// NewLogger creates a new logger instance that writes to both console and file
func NewLogger(consoleOutput bool) (*Logger, error) {
	return NewLoggerWithLevel(consoleOutput, INFO)
}

// NewLoggerWithLevel creates a logger with a specific minimum log level
func NewLoggerWithLevel(consoleOutput bool, level LogLevel) (*Logger, error) {
	// Create test_results/logs directory if it doesn't exist
	logsDir := "test_results/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create timestamp-based filename (same format as JSON report)
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("k8s-diagnostic-logs-%s.log", timestamp)
	fullPath := filepath.Join(logsDir, filename)

	// Open log file for writing
	logFile, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	logger := &Logger{
		logFile:       logFile,
		logFilePath:   fullPath,
		timestampFmt:  "2006-01-02 15:04:05",
		consoleOutput: consoleOutput,
		minLevel:      level,
	}

	// Log logger initialization
	logger.LogInfo("Logging system initialized. Log file: %s", filepath.Base(fullPath))

	return logger, nil
}

// GetLogFilePath returns the path to the log file
func (l *Logger) GetLogFilePath() string {
	return l.logFilePath
}

// GetLogFilename returns just the filename portion of the log file
func (l *Logger) GetLogFilename() string {
	return filepath.Base(l.logFilePath)
}

// SetContext sets the current context for logging
func (l *Logger) SetContext(context string) {
	l.context = context
}

// ClearContext clears the current context
func (l *Logger) ClearContext() {
	l.context = ""
}

// logWithLevel logs a message with the specified level
func (l *Logger) logWithLevel(level LogLevel, format string, args ...interface{}) {
	if level < l.minLevel {
		return
	}

	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format(l.timestampFmt)

	// Build log message with level and context
	var logParts []string
	logParts = append(logParts, timestamp)
	logParts = append(logParts, level.String())

	if l.context != "" {
		logParts = append(logParts, l.context)
	}

	// Get calling function info
	_, file, line, ok := runtime.Caller(2) // Skip 2 frames: logWithLevel and the specific log method
	if ok {
		// Use short file path - just filename
		fileName := filepath.Base(file)
		logParts = append(logParts, fmt.Sprintf("%s:%d", fileName, line))
	}

	logHeader := fmt.Sprintf("[%s]", strings.Join(logParts, "]["))
	logMessage := fmt.Sprintf("%s %s", logHeader, message)

	// Write to console if enabled
	if l.consoleOutput {
		// Add color to console output based on log level
		var colorCode string
		switch level {
		case DEBUG:
			colorCode = "\033[37m" // Light Gray
		case INFO:
			colorCode = "\033[0m" // Default color
		case WARNING:
			colorCode = "\033[33m" // Yellow
		case ERROR:
			colorCode = "\033[31m" // Red
		}
		resetCode := "\033[0m"

		// Only color the log level in the console
		consoleMessage := fmt.Sprintf("[%s][%s%s%s]", timestamp, colorCode, level.String(), resetCode)
		if l.context != "" {
			consoleMessage += fmt.Sprintf("[%s]", l.context)
		}
		if ok {
			fileName := filepath.Base(file)
			consoleMessage += fmt.Sprintf("[%s:%d]", fileName, line)
		}
		consoleMessage += fmt.Sprintf(" %s", message)

		fmt.Println(consoleMessage)
	}

	// Write to log file
	fmt.Fprintln(l.logFile, logMessage)
}

// LogDebug logs a debug message
func (l *Logger) LogDebug(format string, args ...interface{}) {
	l.logWithLevel(DEBUG, format, args...)
}

// LogInfo logs an informational message
func (l *Logger) LogInfo(format string, args ...interface{}) {
	l.logWithLevel(INFO, format, args...)
}

// LogWarning logs a warning message
func (l *Logger) LogWarning(format string, args ...interface{}) {
	l.logWithLevel(WARNING, format, args...)
}

// LogError logs an error message
func (l *Logger) LogError(format string, args ...interface{}) {
	l.logWithLevel(ERROR, format, args...)
}

// LogErrorWithStack logs an error message with stack trace
func (l *Logger) LogErrorWithStack(err error, format string, args ...interface{}) {
	if err == nil {
		l.LogError(format, args...)
		return
	}

	message := fmt.Sprintf(format, args...)
	fullMessage := fmt.Sprintf("%s: %v", message, err)
	l.logWithLevel(ERROR, fullMessage)

	// Capture stack trace if available
	type stackTracer interface {
		StackTrace() []byte
	}

	if st, ok := err.(stackTracer); ok {
		l.logWithLevel(ERROR, "Stack Trace:\n%s", string(st.StackTrace()))
	}
}

// Log writes a message to both console and log file (legacy method, maps to LogInfo)
func (l *Logger) Log(format string, args ...interface{}) {
	l.LogInfo(format, args...)
}

// LogNoTimestamp writes a message to both console and log file without a timestamp in the log file
func (l *Logger) LogNoTimestamp(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// Write to console if enabled
	if l.consoleOutput {
		fmt.Print(message)
	}

	// Write to log file without timestamp
	fmt.Fprint(l.logFile, message)
}

// LogCommandExecution logs command execution details
func (l *Logger) LogCommandExecution(command string, exitCode int, stdout string, stderr string, duration string) {
	l.LogInfo("Command executed: %s", command)
	l.LogInfo("Exit code: %d", exitCode)
	l.LogInfo("Duration: %s", duration)

	if stdout != "" {
		l.LogInfo("Command stdout:")
		l.LogNoTimestamp("%s\n", stdout)
	}

	if stderr != "" && exitCode != 0 {
		l.LogWarning("Command stderr:")
		l.LogNoTimestamp("%s\n", stderr)
	}
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.logFile != nil {
		l.LogInfo("Closing log file: %s", l.GetLogFilename())
		return l.logFile.Close()
	}
	return nil
}

// CaptureCommandOutput is a helper function to capture command execution details
func (l *Logger) CaptureCommandOutput(cmdOutput CommandOutput) {
	l.LogCommandExecution(
		cmdOutput.Command,
		cmdOutput.ExitCode,
		cmdOutput.Stdout,
		cmdOutput.Stderr,
		cmdOutput.Duration,
	)

	if cmdOutput.ExitCode != 0 {
		l.LogError("Command failed: %s (exit code %d)", cmdOutput.Command, cmdOutput.ExitCode)
	}
}
