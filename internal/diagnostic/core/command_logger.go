package core

import (
	"fmt"
	"sync"
)

// CommandLogger manages the single logger instance per command execution
// This ensures all events (cleanup, setup, tests, results) go to the same JSONL file
type CommandLogger struct {
	multiChannelLogger *MultiChannelLogger
	sharedTime         *SharedTimestamp
	initialized        bool
	namespace          string
	verbose            bool
	mu                 sync.Mutex
}

// Global command logger singleton
var commandLoggerInstance *CommandLogger
var commandLoggerMu sync.Mutex

// InitializeCommandLogger initializes the single command-level logger
// This should be called once at the very start of each command execution
func InitializeCommandLogger(namespace string, verbose bool) error {
	commandLoggerMu.Lock()
	defer commandLoggerMu.Unlock()

	// Close existing logger if it exists
	if commandLoggerInstance != nil {
		if err := commandLoggerInstance.close(); err != nil {
			fmt.Printf("Warning: Error closing existing command logger: %v\n", err)
		}
	}

	// Create shared timestamp for consistent file naming
	sharedTime := NewSharedTimestamp()

	// Create multi-channel logger
	multiChannelLogger, err := NewMultiChannelLogger(namespace, verbose)
	if err != nil {
		return fmt.Errorf("failed to create multi-channel logger: %v", err)
	}

	// Create command logger instance
	commandLoggerInstance = &CommandLogger{
		multiChannelLogger: multiChannelLogger,
		sharedTime:         sharedTime,
		initialized:        true,
		namespace:          namespace,
		verbose:            verbose,
	}

	return nil
}

// GetCommandLogger returns the singleton command logger instance
// Returns nil if not initialized - this is thread-safe
func GetCommandLogger() *CommandLogger {
	commandLoggerMu.Lock()
	defer commandLoggerMu.Unlock()

	if commandLoggerInstance != nil && commandLoggerInstance.initialized {
		return commandLoggerInstance
	}
	return nil
}

// GetMultiChannelLogger returns the underlying MultiChannelLogger
// This is used by the global access function
func (c *CommandLogger) GetMultiChannelLogger() *MultiChannelLogger {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.multiChannelLogger
}

// GetSharedTimestamp returns the shared timestamp instance
func (c *CommandLogger) GetSharedTimestamp() *SharedTimestamp {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sharedTime
}

// IsInitialized checks if the command logger is properly initialized
func (c *CommandLogger) IsInitialized() bool {
	if c == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.initialized
}

// GetNamespace returns the namespace for this command execution
func (c *CommandLogger) GetNamespace() string {
	if c == nil {
		return ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.namespace
}

// IsVerbose returns whether verbose mode is enabled
func (c *CommandLogger) IsVerbose() bool {
	if c == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.verbose
}

// close closes the command logger (internal method)
func (c *CommandLogger) close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	if c.multiChannelLogger != nil {
		err = c.multiChannelLogger.Close()
		c.multiChannelLogger = nil
	}

	c.initialized = false
	return err
}

// CloseCommandLogger closes the global command logger instance
// This should be called once at the very end of each command execution
func CloseCommandLogger() error {
	commandLoggerMu.Lock()
	defer commandLoggerMu.Unlock()

	if commandLoggerInstance == nil {
		return nil
	}

	// HTTPLogger processes events asynchronously and will flush on close
	// No manual flush needed as the Close() method handles this

	err := commandLoggerInstance.close()
	commandLoggerInstance = nil
	return err
}

// LogCommandStart logs the start of a command execution
func (c *CommandLogger) LogCommandStart(commandName string, args []string) error {
	if c == nil || c.multiChannelLogger == nil {
		return fmt.Errorf("command logger not initialized")
	}

	return c.multiChannelLogger.LogInfo("Command started: %s %v", commandName, args)
}

// LogCommandComplete logs the completion of a command execution
func (c *CommandLogger) LogCommandComplete(commandName string, success bool, duration float64) error {
	if c == nil || c.multiChannelLogger == nil {
		return fmt.Errorf("command logger not initialized")
	}

	if success {
		return c.multiChannelLogger.LogSuccess("Command completed successfully: %s (%.1fs)", commandName, duration)
	} else {
		return c.multiChannelLogger.LogError("Command failed: %s (%.1fs)", commandName, duration)
	}
}
