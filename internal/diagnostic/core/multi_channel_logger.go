package core

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Global instance for use across all test files
var globalMultiChannelLogger *MultiChannelLogger
var globalLoggerMutex sync.RWMutex

// GetGlobalMultiChannelLogger returns the command singleton MultiChannelLogger instance
// This now uses the command-level singleton to prevent duplicate JSONL files
func GetGlobalMultiChannelLogger() *MultiChannelLogger {
	// PHASE 3: Use command logger singleton if available
	if commandLogger := GetCommandLogger(); commandLogger != nil {
		return commandLogger.GetMultiChannelLogger()
	}

	// Fallback to legacy behavior for backward compatibility (when not in command context)
	globalLoggerMutex.RLock()
	if globalMultiChannelLogger != nil {
		defer globalLoggerMutex.RUnlock()
		return globalMultiChannelLogger
	}
	globalLoggerMutex.RUnlock()

	// Need to upgrade to write lock to initialize
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()

	// Double-check after acquiring write lock
	if globalMultiChannelLogger != nil {
		return globalMultiChannelLogger
	}

	// Initialize with default values (fallback only)
	logger, err := NewMultiChannelLogger("k8s-diagnostic", false)
	if err != nil {
		// Fallback: create a minimal logger that won't crash
		// This should rarely happen, but ensures robustness
		fmt.Printf("Warning: Failed to create global MultiChannelLogger: %v\n", err)
		return nil
	}

	globalMultiChannelLogger = logger
	return globalMultiChannelLogger
}

// InitGlobalMultiChannelLogger allows explicit initialization of the global logger
// This is useful when you need to set specific parameters
func InitGlobalMultiChannelLogger(namespace string, verbose bool) error {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()

	// Close existing logger if it exists
	if globalMultiChannelLogger != nil {
		globalMultiChannelLogger.Close()
	}

	// Create new logger with specified parameters
	logger, err := NewMultiChannelLogger(namespace, verbose)
	if err != nil {
		return fmt.Errorf("failed to initialize global MultiChannelLogger: %v", err)
	}

	globalMultiChannelLogger = logger
	return nil
}

// CloseGlobalMultiChannelLogger closes the global logger instance
func CloseGlobalMultiChannelLogger() error {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()

	if globalMultiChannelLogger != nil {
		err := globalMultiChannelLogger.Close()
		globalMultiChannelLogger = nil
		return err
	}
	return nil
}

// ProgressTracker tracks test progress across all test groups
type ProgressTracker struct {
	mu                    sync.Mutex
	totalTests            int
	completedTests        int
	currentTest           string
	currentStep           string
	currentStepNumber     int
	totalSteps            int
	testNumber            int
	groupName             string
	subgroupName          string
	commandIDCounter      int
	globalStepCounter     int            // Global sequential step counter across all operations
	operationStepCounters map[string]int // Per-operation step counters for display
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		commandIDCounter:      0,
		globalStepCounter:     0,
		operationStepCounters: make(map[string]int),
	}
}

// SetTotalTests sets the total number of tests
func (p *ProgressTracker) SetTotalTests(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.totalTests = total
}

// SetCurrentTest sets the current test information
func (p *ProgressTracker) SetCurrentTest(testName string, testNumber int, groupName, subgroupName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentTest = testName
	p.testNumber = testNumber
	p.groupName = groupName
	p.subgroupName = subgroupName
}

// SetCurrentStep sets the current step information with global sequential numbering
func (p *ProgressTracker) SetCurrentStep(stepName string, stepNumber, totalSteps int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Increment global step counter for sequential numbering
	p.globalStepCounter++

	// Store the original step info but use global counter for display
	p.currentStep = stepName
	p.currentStepNumber = p.globalStepCounter
	p.totalSteps = totalSteps
}

// GetCurrentGlobalStepNumber returns the current global step number
func (p *ProgressTracker) GetCurrentGlobalStepNumber() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.globalStepCounter
}

// CompleteTest increments completed tests counter
func (p *ProgressTracker) CompleteTest() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completedTests++
}

// GetNextCommandID returns a unique command ID
func (p *ProgressTracker) GetNextCommandID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commandIDCounter++
	return fmt.Sprintf("CMD-%03d", p.commandIDCounter)
}

// GetProgress returns current progress information
func (p *ProgressTracker) GetProgress() (int, int, string, string, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.completedTests, p.totalTests, p.currentTest, p.currentStep, p.currentStepNumber, p.totalSteps
}

// MultiChannelLogger provides unified logging across all output channels
type MultiChannelLogger struct {
	verboseLogger    *Logger           // Existing human-readable logs
	httpLogger       *HTTPLogger       // HTTP-based structured logging
	progress         *ProgressTracker  // Progress tracking
	sharedTime       *SharedTimestamp  // Consistent naming
	verbose          bool              // Verbose mode flag
	namespace        string            // Test namespace
	currentHierarchy *HierarchyContext // Current execution hierarchy
	mu               sync.RWMutex      // Protect concurrent access
}

// NewMultiChannelLogger creates a new multi-channel logger system
func NewMultiChannelLogger(namespace string, verbose bool) (*MultiChannelLogger, error) {
	// DEBUG: Add console debugging to track execution flow
	fmt.Printf("DEBUG: NewMultiChannelLogger starting (namespace=%s, verbose=%t)\n", namespace, verbose)

	// Create shared timestamp for consistent file naming
	fmt.Printf("DEBUG: Creating shared timestamp...\n")
	sharedTime := NewSharedTimestamp()
	fmt.Printf("DEBUG: Shared timestamp created\n")

	// Create verbose logger (existing system) - disable console output, let multi-channel logger control it
	fmt.Printf("DEBUG: Creating verbose logger...\n")
	verboseLogger, err := NewLoggerWithSharedTimestamp(sharedTime, false, DEBUG)
	if err != nil {
		fmt.Printf("DEBUG: Failed to create verbose logger: %v\n", err)
		return nil, fmt.Errorf("failed to create verbose logger: %v", err)
	}
	fmt.Printf("DEBUG: Verbose logger created successfully\n")

	// CRITICAL FIX: Conditional HTTP logger creation based on environment
	var httpLogger *HTTPLogger

	// Check if HTTP_LOG_URL is configured (UI integration mode)
	fmt.Printf("DEBUG: Checking UI integration mode...\n")
	if isUIIntegrationMode() {
		fmt.Printf("DEBUG: UI integration mode detected - creating full HTTP logger\n")
		// Generate a test ID for HTTP logging (using timestamp for uniqueness)
		testID := fmt.Sprintf("%d", time.Now().UnixNano()/1000000) // Millisecond timestamp

		// Create HTTP logger (new system) - replaces FrontendJSONLogger
		httpLogger, err = NewHTTPLogger(testID)
		if err != nil {
			verboseLogger.Close()
			return nil, fmt.Errorf("failed to create HTTP logger: %v", err)
		}

		// Log HTTP logger initialization
		verboseLogger.LogInfo("HTTP logger initialized with testID: %s", testID)
	} else {
		fmt.Printf("DEBUG: Standalone Docker mode detected - creating minimal no-op logger\n")
		// Standalone Docker mode - create minimal no-op HTTP logger
		httpLogger = NewNoOpHTTPLogger()
		verboseLogger.LogInfo("Standalone mode detected - using minimal logging")
	}
	fmt.Printf("DEBUG: HTTP logger created successfully\n")

	// Create progress tracker
	fmt.Printf("DEBUG: Creating progress tracker...\n")
	progress := NewProgressTracker()
	fmt.Printf("DEBUG: Progress tracker created\n")

	logger := &MultiChannelLogger{
		verboseLogger: verboseLogger,
		httpLogger:    httpLogger,
		progress:      progress,
		sharedTime:    sharedTime,
		verbose:       verbose,
		namespace:     namespace,
	}

	// Log initialization
	logger.verboseLogger.LogInfo("Multi-channel logging system initialized")
	logger.verboseLogger.LogInfo("Verbose log: %s", sharedTime.GetLogFilePath())

	fmt.Printf("DEBUG: NewMultiChannelLogger completed successfully\n")
	return logger, nil
}

// isUIIntegrationMode detects if we're running in UI integration mode
func isUIIntegrationMode() bool {
	// Check for explicit HTTP_LOG_URL configuration
	if httpURL := os.Getenv("HTTP_LOG_URL"); httpURL != "" {
		return true
	}

	// Check for Docker container environment without UI integration
	if isDockerContainer() && !hasUIIntegration() {
		return false
	}

	// Default to UI integration mode for backward compatibility
	return true
}

// isDockerContainer detects if we're running inside a Docker container
func isDockerContainer() bool {
	// Check for Docker environment indicators
	if os.Getenv("DOCKER_CONTAINER") == "true" {
		return true
	}

	// Check for container runtime indicators
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	return false
}

// hasUIIntegration checks if UI integration is explicitly configured
func hasUIIntegration() bool {
	// Check for UI-specific environment variables
	return os.Getenv("HTTP_LOG_URL") != "" ||
		os.Getenv("BATCH_TEST_ID") != "" ||
		os.Getenv("ENABLE_UI_INTEGRATION") == "true"
}

// Suite level logging methods

func (m *MultiChannelLogger) LogSuiteStart(totalTests int, groups []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update progress tracker
	m.progress.SetTotalTests(totalTests)

	// Log to verbose system
	m.verboseLogger.SetContext("SUITE")
	m.verboseLogger.LogInfo("Starting Kubernetes diagnostic tests suite")
	m.verboseLogger.LogInfo("Total tests: %d, Groups: %v", totalTests, groups)

	// Log to HTTP logger system
	return m.httpLogger.LogSuiteStart(totalTests, groups)
}

// LogSuiteStartWithInfrastructure logs suite start with comprehensive cluster infrastructure context
func (m *MultiChannelLogger) LogSuiteStartWithInfrastructure(totalTests int, groups []string, infrastructure *ClusterInfrastructure) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update progress tracker
	m.progress.SetTotalTests(totalTests)

	// Log to verbose system with infrastructure context
	m.verboseLogger.SetContext("SUITE")
	m.verboseLogger.LogInfo("Starting Kubernetes diagnostic tests suite")
	m.verboseLogger.LogInfo("Total tests: %d, Groups: %v", totalTests, groups)

	if infrastructure != nil {
		if infrastructure.HasCriticalErrors() {
			m.verboseLogger.LogWarning("Infrastructure collection had issues: couldn't verify infrastructure settings")
		} else {
			m.verboseLogger.LogInfo("Cluster infrastructure: %s", infrastructure.GetInfrastructureSummary())
		}
	}

	// Log to HTTP logger system with infrastructure context
	return m.httpLogger.LogSuiteStartWithInfrastructure(totalTests, groups, infrastructure)
}

func (m *MultiChannelLogger) LogSuiteComplete(passed, failed int, duration float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalTests := passed + failed

	// Log to verbose system
	m.verboseLogger.LogInfo("Completed test suite: %d passed, %d failed (%.1fs)", passed, failed, duration)

	// Log to HTTP logger system
	return m.httpLogger.LogSuiteComplete(totalTests, passed, failed, duration)
}

func (m *MultiChannelLogger) LogSuiteInterrupted(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogWarning("Test suite interrupted: %s", reason)

	// Log to HTTP logger system
	return m.httpLogger.LogSuiteInterrupted(reason)
}

// Group level logging methods

func (m *MultiChannelLogger) LogGroupStart(groupName string, groupNumber, totalGroups int, subgroups []string, testsInGroup int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.SetContext(groupName)
	m.verboseLogger.LogInfo("Starting %s test group (%d/%d)", groupName, groupNumber, totalGroups)
	m.verboseLogger.LogInfo("Subgroups: %v, Tests in group: %d", subgroups, testsInGroup)

	// Log to HTTP logger system
	return m.httpLogger.LogGroupStart(groupName, groupNumber, totalGroups, subgroups, testsInGroup)
}

func (m *MultiChannelLogger) LogGroupComplete(groupName string, passed, failed int, duration float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogInfo("Completed %s group: %d passed, %d failed (%.1fs)", groupName, passed, failed, duration)

	// Log to HTTP logger system
	return m.httpLogger.LogGroupComplete(groupName, passed, failed, duration)
}

// Subgroup level logging methods

func (m *MultiChannelLogger) LogSubgroupStart(subgroupName string, testsInSubgroup, subgroupNumber, totalSubgroups int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.SetContext(subgroupName)
	m.verboseLogger.LogInfo("Starting %s subgroup (%d/%d)", subgroupName, subgroupNumber, totalSubgroups)
	m.verboseLogger.LogInfo("Tests in subgroup: %d", testsInSubgroup)

	// Log to HTTP logger system
	return m.httpLogger.LogSubgroupStart(subgroupName, testsInSubgroup, subgroupNumber, totalSubgroups)
}

func (m *MultiChannelLogger) LogSubgroupComplete(subgroupName string, passed, failed int, duration float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogInfo("Completed %s subgroup: %d passed, %d failed (%.1fs)", subgroupName, passed, failed, duration)

	// Log to HTTP logger system
	return m.httpLogger.LogSubgroupComplete(subgroupName, passed, failed, duration)
}

// Test level logging methods

func (m *MultiChannelLogger) LogTestStart(testName string, testNumber, totalTests int, subgroup string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update progress tracker
	m.progress.SetCurrentTest(testName, testNumber, "", subgroup)

	// Log to console (terminal output) - only in verbose mode
	if m.verbose {
		fmt.Println("================================================================================")
		fmt.Printf("TEST: %s [STARTED]\n", testName)
		fmt.Printf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Println("================================================================================")
		fmt.Println()
	}

	// Log to verbose system
	m.verboseLogger.SetContext(testName)
	m.verboseLogger.LogInfo("Starting test %d/%d: %s (subgroup: %s)", testNumber, totalTests, testName, subgroup)

	// Log to HTTP logger system
	return m.httpLogger.LogTestStart(testName, testNumber, totalTests, subgroup)
}

func (m *MultiChannelLogger) LogTestComplete(testName string, testNumber, totalTests int, success bool, duration float64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update progress tracker
	m.progress.CompleteTest()

	// Terminal output is now handled by frontend logger methods (LogL3TestComplete, LogNetworkingTestComplete, etc.)
	// This prevents double output and allows enhanced Expected/Result format

	// Log to verbose system
	status := "PASSED"
	if !success {
		status = "FAILED"
	}
	m.verboseLogger.LogInfo("Test %d/%d %s: %s (%.1fs) - %s", testNumber, totalTests, status, testName, duration, message)

	// Log to HTTP logger system - use legacy method for compatibility
	return m.httpLogger.LogTestCompleteWithHierarchy(testName, testNumber, totalTests, success, duration, message, m.currentHierarchy)
}

func (m *MultiChannelLogger) LogTestError(testName string, testNumber int, errorMsg, stage string, retryable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogError("Test %d ERROR (%s): %s - %s (retryable: %t)", testNumber, stage, testName, errorMsg, retryable)

	// Log to HTTP logger system
	return m.httpLogger.LogTestError(testName, testNumber, errorMsg, stage, retryable)
}

func (m *MultiChannelLogger) LogTestRetry(testName string, attempt, maxAttempts int, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogWarning("Retrying %s (attempt %d/%d): %s", testName, attempt, maxAttempts, reason)

	// Log to HTTP logger system
	return m.httpLogger.LogTestRetry(testName, attempt, maxAttempts, reason)
}

// Operation level logging methods

func (m *MultiChannelLogger) LogStep(stepName, message string, step, totalSteps int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update progress tracker
	m.progress.SetCurrentStep(stepName, step, totalSteps)

	// Console output only in verbose mode
	if m.verbose {
		fmt.Printf("Step %d: %s\n", step, stepName)
	}

	// Log to verbose system (file only)
	m.verboseLogger.LogInfo("Step %d/%d (%s): %s", step, totalSteps, stepName, message)

	// Log to HTTP logger system
	return m.httpLogger.LogStep(stepName, message, step, totalSteps, "in_progress")
}

func (m *MultiChannelLogger) LogStepName(stepName, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Console output in BOTH verbose and non-verbose modes (no step numbers)
	fmt.Printf("%s\n", stepName)

	// Log to verbose system (file only)
	m.verboseLogger.LogInfo("%s: %s", stepName, message)

	// Log to HTTP logger system
	return m.httpLogger.LogStep(stepName, message, 0, 0, "in_progress")
}

func (m *MultiChannelLogger) LogStepComplete(stepName string, success bool, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Console output only in verbose mode
	if m.verbose {
		if success {
			fmt.Printf("✅ %s completed\n", stepName)
		} else {
			fmt.Printf("❌ %s failed\n", stepName)
		}
	}

	// Log to verbose system (file only)
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	m.verboseLogger.LogInfo("Step %s: %s - %s", status, stepName, message)

	// Log to HTTP logger system using new LogStepComplete method (triggers immediate flush)
	return m.httpLogger.LogStepComplete(stepName, success, message)
}

func (m *MultiChannelLogger) LogCommand(command, workingDir string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate unique command ID
	cmdID := m.progress.GetNextCommandID()

	// Log to verbose system (only when verbose mode is enabled)
	if m.verbose {
		m.verboseLogger.LogInfo("Executing command (%s): %s", cmdID, command)
		if workingDir != "" {
			m.verboseLogger.LogInfo("Working directory: %s", workingDir)
		}
	}

	// Log to HTTP logger system
	err := m.httpLogger.LogCommand(command, cmdID, workingDir)

	return cmdID, err
}

func (m *MultiChannelLogger) LogCommandResult(cmdID string, exitCode int, duration float64, stdout, stderr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	success := exitCode == 0

	// Log to verbose system with existing method (only when verbose mode is enabled)
	if m.verbose {
		m.verboseLogger.LogCommandExecution(
			fmt.Sprintf("Command %s", cmdID),
			exitCode,
			stdout,
			stderr,
			fmt.Sprintf("%.3fs", duration),
		)
	}

	// Log to HTTP logger system
	return m.httpLogger.LogCommandResult(cmdID, exitCode, duration, stdout, stderr, success)
}

func (m *MultiChannelLogger) LogCleanup(operation string, resources []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogInfo("Cleanup operation: %s (resources: %v)", operation, resources)

	// Log to HTTP logger system
	return m.httpLogger.LogCleanup(operation, resources)
}

func (m *MultiChannelLogger) LogAPICall(method, endpoint string, duration float64, statusCode int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Log to verbose system
	m.verboseLogger.LogDebug("API call: %s %s -> %d (%.3fs)", method, endpoint, statusCode, duration)

	// Log to HTTP logger system
	return m.httpLogger.LogAPICall(method, endpoint, duration, statusCode)
}

// Additional logging methods for tests

func (m *MultiChannelLogger) LogInfo(message string, args ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Format message if args provided
	formattedMessage := message
	if len(args) > 0 {
		formattedMessage = fmt.Sprintf(message, args...)
	}

	// Log to console (terminal output) - only in verbose mode
	if m.verbose {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("%s INFO: %s\n", timestamp, formattedMessage)
	}

	// Log to verbose system
	m.verboseLogger.LogInfo(formattedMessage)

	return nil
}

func (m *MultiChannelLogger) LogVerbose(message string, args ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Format message if args provided
	formattedMessage := message
	if len(args) > 0 {
		formattedMessage = fmt.Sprintf(message, args...)
	}

	// Log to console (terminal output) - only if verbose mode
	if m.verbose {
		fmt.Printf("    %s\n", formattedMessage)
	}

	// Log to verbose system
	m.verboseLogger.LogDebug(formattedMessage)

	return nil
}

func (m *MultiChannelLogger) LogSuccess(message string, args ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Format message if args provided
	formattedMessage := message
	if len(args) > 0 {
		formattedMessage = fmt.Sprintf(message, args...)
	}

	// Log to console (terminal output) - Direct SUCCESS output with CURRENT timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("%s SUCCESS: %s\n", timestamp, formattedMessage)

	// Log to verbose system
	m.verboseLogger.LogInfo("SUCCESS: %s", formattedMessage)

	return nil
}

func (m *MultiChannelLogger) LogError(message string, args ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Format message if args provided
	formattedMessage := message
	if len(args) > 0 {
		formattedMessage = fmt.Sprintf(message, args...)
	}

	// Log to console (terminal output) - Direct ERROR output with CURRENT timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("%s ERROR: %s\n", timestamp, formattedMessage)

	// Log to verbose system
	m.verboseLogger.LogError(formattedMessage)

	return nil
}

// LogSimpleStatus prints clean terminal messages without timestamps or formatting
func (m *MultiChannelLogger) LogSimpleStatus(message string) {
	// Always output to console directly, regardless of verboseLogger's console setting
	fmt.Println(message)

	// Also log to verbose logger for file logging (if enabled)
	m.verboseLogger.LogInfo("SIMPLE_STATUS: %s", message)
}

// Hierarchy management methods

func (m *MultiChannelLogger) SetHierarchyContext(hierarchy *HierarchyContext) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentHierarchy = hierarchy
}

func (m *MultiChannelLogger) GetHierarchyContext() *HierarchyContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentHierarchy == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	return &HierarchyContext{
		GroupId:    m.currentHierarchy.GroupId,
		SubgroupId: m.currentHierarchy.SubgroupId,
		TestId:     m.currentHierarchy.TestId,
		Phase:      m.currentHierarchy.Phase,
	}
}

func (m *MultiChannelLogger) UpdateHierarchy(groupId, subgroupId, testId, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentHierarchy == nil {
		m.currentHierarchy = &HierarchyContext{}
	}

	if groupId != "" {
		m.currentHierarchy.GroupId = groupId
	}
	if subgroupId != "" {
		m.currentHierarchy.SubgroupId = subgroupId
	}
	if testId != "" {
		m.currentHierarchy.TestId = testId
	}
	if phase != "" {
		m.currentHierarchy.Phase = phase
	}
}

// Utility methods

func (m *MultiChannelLogger) IsVerbose() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verbose
}

func (m *MultiChannelLogger) GetNamespace() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namespace
}

func (m *MultiChannelLogger) GetProgress() (int, int, string, string) {
	completed, total, currentTest, currentStep, _, _ := m.progress.GetProgress()
	return completed, total, currentTest, currentStep
}

func (m *MultiChannelLogger) GetSharedTimestamp() *SharedTimestamp {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sharedTime
}

func (m *MultiChannelLogger) GetVerboseLogger() *Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verboseLogger
}

func (m *MultiChannelLogger) GetHTTPLogger() *HTTPLogger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.httpLogger
}

// GetFrontendLogger provides backward compatibility - redirects to HTTPLogger
// This method should be deprecated and replaced with GetHTTPLogger()
func (m *MultiChannelLogger) GetFrontendLogger() *HTTPLogger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.httpLogger
}

// Close closes all logging channels
func (m *MultiChannelLogger) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	// Close verbose logger
	if err := m.verboseLogger.Close(); err != nil {
		errs = append(errs, fmt.Errorf("verbose logger close error: %v", err))
	}

	// Close HTTP logger
	if err := m.httpLogger.Close(); err != nil {
		errs = append(errs, fmt.Errorf("HTTP logger close error: %v", err))
	}

	// Return first error if any occurred
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}
