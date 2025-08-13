package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPLogEvent represents a structured event sent to the API
type HTTPLogEvent struct {
	TestID    string                 `json:"testId"`
	Type      string                 `json:"type"`
	Phase     string                 `json:"phase,omitempty"`
	Container string                 `json:"container,omitempty"`
	Step      string                 `json:"step,omitempty"`
	TestName  string                 `json:"testName,omitempty"`
	Line      string                 `json:"line,omitempty"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// HTTPLogger replaces FrontendJSONLogger with direct HTTP API calls
type HTTPLogger struct {
	apiURL       string
	client       *http.Client
	testID       string
	currentPhase string
	currentTest  string
	currentStep  string
	mu           sync.RWMutex
	enabled      bool // Can be disabled if API is unavailable
	eventQueue   chan HTTPLogEvent
	stopQueue    chan bool
	wg           sync.WaitGroup
}

// NewHTTPLogger creates a new HTTP logger that posts to the API
func NewHTTPLogger(testID string) (*HTTPLogger, error) {
	logger := &HTTPLogger{
		apiURL:     "http://localhost:3000/api/log-events",
		client:     &http.Client{Timeout: 5 * time.Second},
		testID:     testID,
		enabled:    true,
		eventQueue: make(chan HTTPLogEvent, 1000), // Buffer up to 1000 events
		stopQueue:  make(chan bool, 1),
	}

	// Start background worker to process events
	logger.wg.Add(1)
	go logger.eventWorker()

	return logger, nil
}

// eventWorker processes events asynchronously to avoid blocking the main thread
func (h *HTTPLogger) eventWorker() {
	defer h.wg.Done()

	for {
		select {
		case event := <-h.eventQueue:
			h.sendEventSync(event)
		case <-h.stopQueue:
			// Process remaining events before stopping
			for len(h.eventQueue) > 0 {
				event := <-h.eventQueue
				h.sendEventSync(event)
			}
			return
		case <-time.After(30 * time.Second):
			// Periodic health check - send a heartbeat if no events for 30 seconds
			if h.enabled {
				heartbeat := HTTPLogEvent{
					TestID:    h.testID,
					Type:      "heartbeat",
					Timestamp: time.Now().Format(time.RFC3339),
				}
				h.sendEventSync(heartbeat)
			}
		}
	}
}

// sendEvent queues an event for async processing
func (h *HTTPLogger) sendEvent(event HTTPLogEvent) {
	h.mu.RLock()
	enabled := h.enabled
	h.mu.RUnlock()

	if !enabled {
		return // Skip if disabled
	}

	// Set testID and timestamp if not provided
	if event.TestID == "" {
		event.TestID = h.testID
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Try to queue the event, drop if queue is full
	select {
	case h.eventQueue <- event:
		// Event queued successfully
	default:
		// Queue is full, drop the event to prevent blocking
		fmt.Printf("Warning: HTTP logger queue full, dropping event: %s\n", event.Type)
	}
}

// sendEventSync sends an event synchronously
func (h *HTTPLogger) sendEventSync(event HTTPLogEvent) {
	h.mu.RLock()
	enabled := h.enabled
	h.mu.RUnlock()

	if !enabled {
		return
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Error marshaling event: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", h.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		h.disableOnError()
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		// Network error - disable temporarily
		h.disableOnError()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read error response for debugging
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("HTTP Logger error %d: %s\n", resp.StatusCode, string(body))

		if resp.StatusCode >= 500 {
			h.disableOnError()
		}
	}
}

// disableOnError temporarily disables the logger on errors
func (h *HTTPLogger) disableOnError() {
	h.mu.Lock()
	h.enabled = false
	h.mu.Unlock()

	// Re-enable after 30 seconds
	time.AfterFunc(30*time.Second, func() {
		h.mu.Lock()
		h.enabled = true
		h.mu.Unlock()
	})
}

// Context management methods
func (h *HTTPLogger) SetContext(context string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Parse context to determine phase/container/step
	if context != "" {
		h.currentStep = context
	}
}

func (h *HTTPLogger) updateContext(phase, container, step string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if phase != "" {
		h.currentPhase = phase
	}
	if container != "" {
		h.currentTest = container
	}
	if step != "" {
		h.currentStep = step
	}
}

// Suite level methods
func (h *HTTPLogger) LogSuiteStart(totalTests int, groups []string) error {
	event := HTTPLogEvent{
		Type:      "suite_start",
		Phase:     "setup",
		Container: "suite",
		Line:      fmt.Sprintf("Starting test suite with %d tests", totalTests),
		Data: map[string]interface{}{
			"totalTests": totalTests,
			"groups":     groups,
		},
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogSuiteStartWithInfrastructure(totalTests int, groups []string, infrastructure *ClusterInfrastructure) error {
	data := map[string]interface{}{
		"totalTests": totalTests,
		"groups":     groups,
	}

	if infrastructure != nil {
		data["infrastructure"] = infrastructure.GetInfrastructureSummary()
	}

	event := HTTPLogEvent{
		Type:      "suite_start",
		Phase:     "setup",
		Container: "suite",
		Line:      fmt.Sprintf("Starting test suite with %d tests", totalTests),
		Data:      data,
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogSuiteComplete(totalTests, passed, failed int, duration float64) error {
	event := HTTPLogEvent{
		Type:      "suite_complete",
		Phase:     "complete",
		Container: "suite",
		Line:      fmt.Sprintf("Test suite completed: %d passed, %d failed", passed, failed),
		Data: map[string]interface{}{
			"totalTests": totalTests,
			"passed":     passed,
			"failed":     failed,
			"duration":   duration,
		},
	}
	h.sendEvent(event)
	return nil
}

// Cleanup methods
func (h *HTTPLogger) LogCleanup(operation string, resources []string) error {
	h.updateContext("cleanup", "universal_cleanup", operation)

	event := HTTPLogEvent{
		Type:      "cleanup_start",
		Phase:     "cleanup",
		Container: "universal_cleanup",
		Step:      operation,
		Line:      fmt.Sprintf("├── %s: %s", operation, "Starting cleanup..."),
		Data: map[string]interface{}{
			"operation": operation,
			"resources": resources,
		},
	}
	h.sendEvent(event)
	return nil
}

// Step methods
func (h *HTTPLogger) LogStep(stepName, message string, step, totalSteps int, status string) error {
	h.mu.RLock()
	currentPhase := h.currentPhase
	currentContainer := h.currentTest
	h.mu.RUnlock()

	if currentContainer == "" {
		currentContainer = "universal_cleanup"
	}

	event := HTTPLogEvent{
		Type:      "step_progress",
		Phase:     currentPhase,
		Container: currentContainer,
		Step:      stepName,
		Line:      message,
		Data: map[string]interface{}{
			"stepName":   stepName,
			"step":       step,
			"totalSteps": totalSteps,
			"status":     status,
		},
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogStepComplete(stepName string, success bool, message string) error {
	h.mu.RLock()
	currentPhase := h.currentPhase
	currentContainer := h.currentTest
	h.mu.RUnlock()

	if currentContainer == "" {
		currentContainer = "universal_cleanup"
	}

	status := "✅ Done"
	if !success {
		status = "❌ Failed"
	}

	event := HTTPLogEvent{
		Type:      "step_complete",
		Phase:     currentPhase,
		Container: currentContainer,
		Step:      stepName,
		Line:      fmt.Sprintf("%s %s", status, message),
		Data: map[string]interface{}{
			"stepName": stepName,
			"success":  success,
			"message":  message,
		},
	}
	h.sendEvent(event)
	return nil
}

// Test methods
func (h *HTTPLogger) LogTestStart(testName string, testNumber, totalTests int, subgroup string) error {
	h.updateContext("testing", testName, "")

	event := HTTPLogEvent{
		Type:      "test_start",
		Phase:     "testing",
		Container: testName,
		TestName:  testName,
		Line:      fmt.Sprintf("└── (%d/%d) %s: Testing policies...", testNumber, totalTests, testName),
		Data: map[string]interface{}{
			"testName":   testName,
			"testNumber": testNumber,
			"totalTests": totalTests,
			"subgroup":   subgroup,
		},
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogTestStartWithHierarchy(testName string, testNumber, totalTests int, subgroup string, hierarchy *HierarchyContext) error {
	return h.LogTestStart(testName, testNumber, totalTests, subgroup)
}

func (h *HTTPLogger) LogTestComplete(testName, group, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext, testResult *TestResult) error {
	status := "✅ PASS"
	if !success {
		status = "❌ FAIL"
	}

	line := fmt.Sprintf("%s (%ss)", status, fmt.Sprintf("%.1f", duration))

	data := map[string]interface{}{
		"testName":   testName,
		"testNumber": testNumber,
		"totalTests": totalTests,
		"success":    success,
		"duration":   duration,
		"group":      group,
		"subgroup":   subgroupName,
	}

	// Add test result details
	if testResult != nil {
		if testResult.Message != "" {
			data["result"] = testResult.Message
			line += fmt.Sprintf("\n   🎯 Result: %s", testResult.Message)
		}
		if len(testResult.Details) > 0 {
			data["details"] = testResult.Details
		}
	}

	event := HTTPLogEvent{
		Type:      "test_complete",
		Phase:     "testing",
		Container: testName,
		TestName:  testName,
		Line:      line,
		Data:      data,
	}
	h.sendEvent(event)
	return nil
}

// Command methods
func (h *HTTPLogger) LogCommand(command, cmdID, workingDir string) error {
	h.mu.RLock()
	currentContainer := h.currentTest
	h.mu.RUnlock()

	if currentContainer == "" {
		currentContainer = "universal_cleanup"
	}

	event := HTTPLogEvent{
		Type:      "command_start",
		Phase:     "testing",
		Container: currentContainer,
		Line:      fmt.Sprintf("Executing: %s", command),
		Data: map[string]interface{}{
			"command":    command,
			"cmdId":      cmdID,
			"workingDir": workingDir,
		},
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogCommandResult(cmdID string, exitCode int, duration float64, stdout, stderr string, success bool) error {
	h.mu.RLock()
	currentContainer := h.currentTest
	h.mu.RUnlock()

	if currentContainer == "" {
		currentContainer = "universal_cleanup"
	}

	// Include stdout if not too large
	line := ""
	if len(stdout) > 0 && len(stdout) < 500 {
		line = fmt.Sprintf("Command stdout (%d bytes):\n%s", len(stdout), stdout)
	} else if len(stdout) > 0 {
		line = fmt.Sprintf("Command stdout (%d bytes): [output truncated]", len(stdout))
	}

	if len(stderr) > 0 {
		line += fmt.Sprintf("\nCommand stderr: %s", stderr)
	}

	event := HTTPLogEvent{
		Type:      "command_result",
		Phase:     "testing",
		Container: currentContainer,
		Line:      line,
		Data: map[string]interface{}{
			"cmdId":    cmdID,
			"exitCode": exitCode,
			"duration": duration,
			"stdout":   stdout,
			"stderr":   stderr,
			"success":  success,
		},
	}
	h.sendEvent(event)
	return nil
}

// Compatibility methods for all the legacy frontend logger methods
func (h *HTTPLogger) LogTestCompleteWithHierarchy(testName string, testNumber, totalTests int, success bool, duration float64, message string, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success, Message: message}
	return h.LogTestComplete(testName, "unknown", "unknown", testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (h *HTTPLogger) LogSuiteInterrupted(reason string) error {
	event := HTTPLogEvent{
		Type:      "suite_interrupted",
		Phase:     "interrupted",
		Container: "suite",
		Line:      fmt.Sprintf("Test suite interrupted: %s", reason),
		Data:      map[string]interface{}{"reason": reason},
	}
	h.sendEvent(event)
	return nil
}

// Add all other required methods as no-ops or simple implementations
func (h *HTTPLogger) LogTestError(testName string, testNumber int, errorMsg, stage string, retryable bool) error {
	event := HTTPLogEvent{
		Type:      "test_error",
		Container: testName,
		Line:      fmt.Sprintf("Error: %s", errorMsg),
		Data: map[string]interface{}{
			"testName":   testName,
			"testNumber": testNumber,
			"error":      errorMsg,
			"stage":      stage,
			"retryable":  retryable,
		},
	}
	h.sendEvent(event)
	return nil
}

func (h *HTTPLogger) LogTestRetry(testName string, attempt, maxAttempts int, reason string) error {
	return nil
}
func (h *HTTPLogger) LogAPICall(method, endpoint string, duration float64, statusCode int) error {
	return nil
}
func (h *HTTPLogger) FlushOnTestComplete() error { return nil }
func (h *HTTPLogger) LogStepCompleteWithForcedFlush(stepName string, success bool, message string) error {
	return h.LogStepComplete(stepName, success, message)
}

// Legacy test complete methods - delegate to unified method
func (h *HTTPLogger) LogL3TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success}
	return h.LogTestComplete(testName, "l3-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (h *HTTPLogger) LogL4TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success}
	return h.LogTestComplete(testName, "l4-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (h *HTTPLogger) LogL7TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success}
	return h.LogTestComplete(testName, "l7-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (h *HTTPLogger) LogNetworkingTestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success}
	return h.LogTestComplete(testName, "networking", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

// Legacy cleanup methods
func (h *HTTPLogger) LogL3PreCleanup(message string) error {
	return h.LogCleanup("L3 Pre-cleanup", []string{})
}
func (h *HTTPLogger) LogL4PreCleanup(message string) error {
	return h.LogCleanup("L4 Pre-cleanup", []string{})
}
func (h *HTTPLogger) LogL7PreCleanup(message string) error {
	return h.LogCleanup("L7 Pre-cleanup", []string{})
}
func (h *HTTPLogger) LogNetworkingPreCleanup(message string) error {
	return h.LogCleanup("Networking Pre-cleanup", []string{})
}

// Legacy subgroup methods
func (h *HTTPLogger) LogL3SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return nil
}
func (h *HTTPLogger) LogL4SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return nil
}
func (h *HTTPLogger) LogL7SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return nil
}
func (h *HTTPLogger) LogNetworkingSubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return nil
}

// Group and subgroup methods
func (h *HTTPLogger) LogGroupStart(groupName string, groupNumber, totalGroups int, subgroups []string, testsInGroup int) error {
	return nil
}
func (h *HTTPLogger) LogGroupComplete(groupName string, passed, failed int, duration float64) error {
	return nil
}
func (h *HTTPLogger) LogSubgroupStart(subgroupName string, testsInSubgroup, subgroupNumber, totalSubgroups int) error {
	return nil
}
func (h *HTTPLogger) LogSubgroupStartWithHierarchy(subgroupName string, testsInSubgroup, subgroupNumber, totalSubgroups int, hierarchy *HierarchyContext) error {
	return nil
}
func (h *HTTPLogger) LogSubgroupComplete(subgroupName string, passed, failed int, duration float64) error {
	return nil
}
func (h *HTTPLogger) LogSubgroupCompleteWithHierarchy(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return nil
}

// Additional required methods
func (h *HTTPLogger) LogTemplateVariableDiscovery(templateVars *TemplateVariables, testName string, hierarchy *HierarchyContext) error {
	return nil
}

// GetLogFilePath returns empty string since we don't write to files
func (h *HTTPLogger) GetLogFilePath() string { return "" }

// Close stops the event worker and cleans up
func (h *HTTPLogger) Close() error {
	// Signal worker to stop
	h.stopQueue <- true

	// Wait for worker to finish
	h.wg.Wait()

	// Close channels
	close(h.eventQueue)
	close(h.stopQueue)

	return nil
}
