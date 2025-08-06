package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// FrontendLogLevel represents log levels for frontend consumption
type FrontendLogLevel string

const (
	FrontendLevelDebug   FrontendLogLevel = "DEBUG"
	FrontendLevelInfo    FrontendLogLevel = "INFO"
	FrontendLevelSuccess FrontendLogLevel = "SUCCESS"
	FrontendLevelWarning FrontendLogLevel = "WARNING"
	FrontendLevelError   FrontendLogLevel = "ERROR"
)

// FrontendEventType represents the type of event for frontend processing
type FrontendEventType string

const (
	// Suite level events
	EventSuiteStart       FrontendEventType = "SUITE_START"
	EventSuiteComplete    FrontendEventType = "SUITE_COMPLETE"
	EventSuiteInterrupted FrontendEventType = "SUITE_INTERRUPTED"

	// Group level events
	EventGroupStart    FrontendEventType = "GROUP_START"
	EventGroupComplete FrontendEventType = "GROUP_COMPLETE"

	// Subgroup level events
	EventSubgroupStart    FrontendEventType = "SUBGROUP_START"
	EventSubgroupComplete FrontendEventType = "SUBGROUP_COMPLETE"

	// Test level events
	EventTestStart    FrontendEventType = "TEST_START"
	EventTestComplete FrontendEventType = "TEST_COMPLETE"
	EventTestError    FrontendEventType = "TEST_ERROR"

	// Operation level events
	EventStep          FrontendEventType = "STEP"
	EventStepComplete  FrontendEventType = "STEP_COMPLETE"
	EventCommand       FrontendEventType = "COMMAND"
	EventCommandResult FrontendEventType = "COMMAND_RESULT"
	EventCleanup       FrontendEventType = "CLEANUP"
)

// HierarchyContext tracks the current execution hierarchy
type HierarchyContext struct {
	GroupId    string `json:"groupId,omitempty"`
	SubgroupId string `json:"subgroupId,omitempty"`
	TestId     string `json:"testId,omitempty"`
	Phase      string `json:"phase,omitempty"`
}

// FrontendLogEntry represents a single log entry in JSON Lines format
type FrontendLogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     FrontendLogLevel       `json:"level"`
	Type      FrontendEventType      `json:"type"`
	Context   string                 `json:"context"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`

	// Hierarchy tracking fields
	GroupId    string `json:"groupId,omitempty"`
	SubgroupId string `json:"subgroupId,omitempty"`
	TestId     string `json:"testId,omitempty"`
	Phase      string `json:"phase,omitempty"`
}

// FrontendJSONLogger handles structured logging for frontend consumption with JSONL immediate writes
type FrontendJSONLogger struct {
	file    *os.File
	encoder *json.Encoder
	context string
	mu      sync.Mutex // Single mutex for thread safety
}

// NewFrontendJSONLogger creates a new frontend JSON logger
func NewFrontendJSONLogger(sharedTime *SharedTimestamp) (*FrontendJSONLogger, error) {
	// Create test_results/logs directory if it doesn't exist
	logsDir := "test_results/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create frontend log file with .frontend.jsonl extension
	frontendPath := sharedTime.GetFrontendLogFilePath()

	file, err := os.Create(frontendPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create frontend log file: %v", err)
	}

	logger := &FrontendJSONLogger{
		file:    file,
		encoder: json.NewEncoder(file),
		context: "",
	}

	return logger, nil
}

// SetContext sets the current context for logging
func (f *FrontendJSONLogger) SetContext(context string) {
	f.context = context
}

// GetContext returns the current context
func (f *FrontendJSONLogger) GetContext() string {
	return f.context
}

// writeDirectlyToDisk writes a single entry directly to disk with immediate sync
func (f *FrontendJSONLogger) writeDirectlyToDisk(entry FrontendLogEntry) error {
	// Validate entry
	if entry.Timestamp.IsZero() || entry.Message == "" || entry.Type == "" {
		return fmt.Errorf("invalid entry: missing required fields")
	}

	// Marshal entry
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal failed: %v", err)
	}

	// Thread-safe write with immediate sync
	f.mu.Lock()
	defer f.mu.Unlock()

	// Write JSON data directly to file
	if _, err := f.file.Write(jsonData); err != nil {
		return fmt.Errorf("write failed: %v", err)
	}

	// Add newline
	if _, err := f.file.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write newline failed: %v", err)
	}

	// IMMEDIATE sync to disk for real-time visibility
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync failed: %v", err)
	}

	return nil
}

// LogEntry logs a structured entry with immediate write
func (f *FrontendJSONLogger) LogEntry(level FrontendLogLevel, eventType FrontendEventType, message string, data map[string]interface{}) error {
	entry := FrontendLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Type:      eventType,
		Context:   f.context,
		Message:   message,
		Data:      data,
	}

	return f.writeDirectlyToDisk(entry)
}

// LogEntryWithHierarchy logs a structured entry with hierarchy context
func (f *FrontendJSONLogger) LogEntryWithHierarchy(level FrontendLogLevel, eventType FrontendEventType, message string, data map[string]interface{}, hierarchy *HierarchyContext) error {
	entry := FrontendLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Type:      eventType,
		Context:   f.context,
		Message:   message,
		Data:      data,
	}

	// Add hierarchy information if provided
	if hierarchy != nil {
		entry.GroupId = hierarchy.GroupId
		entry.SubgroupId = hierarchy.SubgroupId
		entry.TestId = hierarchy.TestId
		entry.Phase = hierarchy.Phase
	}

	return f.writeDirectlyToDisk(entry)
}

// UNIFIED TEST COMPLETE METHOD - eliminates L3/L4/L7/Networking redundancy
func (f *FrontendJSONLogger) LogTestComplete(testName, group, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext, testResult *TestResult) error {
	level := FrontendLevelSuccess
	if !success {
		level = FrontendLevelError
	}

	result := "PASS"
	if !success {
		result = "FAIL"
	}

	message := fmt.Sprintf("Group %s Subgroup %s Test %s DONE", group, subgroupName, testName)

	// Create base data map with REAL data from test results
	data := map[string]interface{}{
		"testName":   testName,
		"group":      group,
		"subgroup":   subgroupName,
		"testNumber": testNumber,
		"totalTests": totalTests,
		"result":     result,
		"duration":   duration,
		"success":    success,
	}

	// Extract REAL data from test results instead of hardcoded values
	if testResult != nil {
		// Add real test details from actual execution
		if len(testResult.Details) > 0 {
			data["details"] = testResult.Details
		}

		// Add real diagnostic information if available
		if testResult.DetailedDiagnostics != nil {
			diagnostics := map[string]interface{}{}

			if testResult.DetailedDiagnostics.FailureStage != "" {
				diagnostics["failureStage"] = testResult.DetailedDiagnostics.FailureStage
			}

			if testResult.DetailedDiagnostics.TechnicalError != "" {
				diagnostics["technicalError"] = testResult.DetailedDiagnostics.TechnicalError
			}

			if len(testResult.DetailedDiagnostics.CommandOutputs) > 0 {
				diagnostics["commandOutputs"] = testResult.DetailedDiagnostics.CommandOutputs
			}

			if len(diagnostics) > 0 {
				data["diagnostics"] = diagnostics
			}
		}

		// Extract real connectivity data instead of hardcoded values
		if success && testResult.Message != "" {
			data["realResult"] = testResult.Message
		}
	}

	// Create entry without hardcoded expectations
	entry := FrontendLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Type:      EventTestComplete,
		Context:   f.context,
		Message:   message,
		Data:      data,
	}

	// Add hierarchy information if provided
	if hierarchy != nil {
		entry.GroupId = hierarchy.GroupId
		entry.SubgroupId = hierarchy.SubgroupId
		entry.TestId = hierarchy.TestId
		entry.Phase = hierarchy.Phase
	}

	return f.writeDirectlyToDisk(entry)
}

// UNIFIED CLEANUP METHOD - eliminates L3/L4/L7/Networking redundancy
func (f *FrontendJSONLogger) LogCleanupEvent(phase, group, message string) error {
	entry := FrontendLogEntry{
		Timestamp: time.Now(),
		Level:     FrontendLevelInfo,
		Type:      EventCleanup,
		Context:   f.context,
		Message:   message,
		Data: map[string]interface{}{
			"phase": phase,
			"group": group,
		},
	}

	return f.writeDirectlyToDisk(entry)
}

// UNIFIED SUBGROUP SUMMARY - eliminates L3/L4/L7/Networking redundancy
func (f *FrontendJSONLogger) LogSubgroupSummary(group, subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	message := fmt.Sprintf("Subgroup %s DONE", subgroupName)
	if subgroupName == "SUMMARY" {
		message = "SUMMARY DONE"
	}

	entry := FrontendLogEntry{
		Timestamp: time.Now(),
		Level:     FrontendLevelInfo,
		Type:      EventSubgroupComplete,
		Context:   f.context,
		Message:   message,
		Data: map[string]interface{}{
			"group":    group,
			"subgroup": subgroupName,
			"passed":   passed,
			"failed":   failed,
			"duration": duration,
			"success":  failed == 0,
		},
	}

	// Add hierarchy information if provided
	if hierarchy != nil {
		entry.GroupId = hierarchy.GroupId
		entry.SubgroupId = hierarchy.SubgroupId
		entry.TestId = hierarchy.TestId
		entry.Phase = hierarchy.Phase
	}

	return f.writeDirectlyToDisk(entry)
}

// Suite level logging methods
func (f *FrontendJSONLogger) LogSuiteStart(totalTests int, groups []string) error {
	return f.LogEntry(FrontendLevelInfo, EventSuiteStart, "Starting Kubernetes diagnostic tests", map[string]interface{}{
		"totalTests": totalTests,
		"groups":     groups,
	})
}

// LogSuiteStartWithInfrastructure logs suite start with comprehensive cluster infrastructure context
func (f *FrontendJSONLogger) LogSuiteStartWithInfrastructure(totalTests int, groups []string, infrastructure *ClusterInfrastructure) error {
	data := map[string]interface{}{
		"totalTests": totalTests,
		"groups":     groups,
	}

	// Add infrastructure information if available
	if infrastructure != nil {
		// Add cluster information that affects test interpretation
		clusterInfo := map[string]interface{}{
			"kubernetesVersion": infrastructure.KubernetesVersion,
			"nodeCount":         infrastructure.NodeCount,
			"cniProvider":       infrastructure.CNIProvider,
			"platform":          infrastructure.Platform,
		}

		// Add CNI version if available
		if infrastructure.CNIVersion != "" && infrastructure.CNIVersion != "detected" {
			clusterInfo["cniVersion"] = infrastructure.CNIVersion
		}

		// Add node information for detailed context
		if len(infrastructure.Nodes) > 0 {
			nodeInfo := make([]map[string]interface{}, len(infrastructure.Nodes))
			for i, node := range infrastructure.Nodes {
				nodeInfo[i] = map[string]interface{}{
					"name": node.Name,
					"role": node.Role,
				}
				// Add additional details if available
				if node.KernelVersion != "" {
					nodeInfo[i]["kernelVersion"] = node.KernelVersion
				}
				if node.PodCIDR != "" {
					nodeInfo[i]["podCIDR"] = node.PodCIDR
				}
			}
			clusterInfo["nodes"] = nodeInfo
		}

		// Add resource information if available
		if infrastructure.ClusterResources != nil && infrastructure.ClusterResources.PodCapacity > 0 {
			clusterInfo["podCapacity"] = infrastructure.ClusterResources.PodCapacity
		}

		// Add collection status
		clusterInfo["collectionErrors"] = len(infrastructure.CollectionErrors)
		clusterInfo["collectedAt"] = infrastructure.CollectedAt.Format("2006-01-02T15:04:05Z07:00")

		data["clusterInfo"] = clusterInfo

		// Add human-readable infrastructure summary
		if infrastructure.HasCriticalErrors() {
			data["infrastructureSummary"] = "couldn't verify infrastructure settings"
		} else {
			data["infrastructureSummary"] = infrastructure.GetInfrastructureSummary()
		}

		// Log collection warnings if any (non-critical)
		if len(infrastructure.CollectionErrors) > 0 && len(infrastructure.CollectionErrors) <= 3 {
			data["infrastructureWarnings"] = infrastructure.CollectionErrors
		}
	} else {
		// Fallback when infrastructure collection fails
		data["infrastructureSummary"] = "couldn't verify infrastructure settings"
		data["clusterInfo"] = map[string]interface{}{
			"kubernetesVersion": "unknown",
			"nodeCount":         0,
			"cniProvider":       "unknown",
			"platform":          "unknown",
			"collectionErrors":  1,
		}
	}

	return f.LogEntry(FrontendLevelInfo, EventSuiteStart, "Starting Kubernetes diagnostic tests", data)
}

// LogTemplateVariableDiscovery logs template variable discovery status to show discovered vs fallback values
func (f *FrontendJSONLogger) LogTemplateVariableDiscovery(templateVars *TemplateVariables, testName string, hierarchy *HierarchyContext) error {
	if templateVars == nil || templateVars.DiscoveryStatus == nil {
		return nil // No discovery data to log
	}

	// Categorize variables by discovery status
	discovered := make(map[string]string)
	fallbacks := make(map[string]string)
	derived := make(map[string]string)

	// Count statistics
	discoveredCount := 0
	fallbackCount := 0
	derivedCount := 0

	// Process all template variables and their discovery status
	variableValues := map[string]string{
		"POD_CIDR":                   templateVars.PodCIDR,
		"NODE_CIDR":                  templateVars.NodeCIDR,
		"NODE1_CIDR":                 templateVars.Node1CIDR,
		"EXCEPT_CIDR":                templateVars.ExceptCIDR,
		"CLUSTER_DOMAIN":             templateVars.ClusterDomain,
		"DNS_SERVER1":                templateVars.DNSServer1,
		"DNS_SERVER2":                templateVars.DNSServer2,
		"API_DOMAIN":                 templateVars.APIDomain,
		"CILIUM_DOMAIN_WILDCARD":     templateVars.CiliumDomainWildcard,
		"GITHUB_DOMAIN_WILDCARD":     templateVars.GithubDomainWildcard,
		"DOCKER_DOMAIN_WILDCARD":     templateVars.DockerDomainWildcard,
		"GOOGLEAPIS_DOMAIN_WILDCARD": templateVars.GoogleapisDomainWildcard,
		"AWS_DOMAIN_WILDCARD":        templateVars.AWSDomainWildcard,
		"CILIUM_BASE_DOMAIN":         templateVars.CiliumBaseDomain,
		"CILIUM_API_DOMAIN":          templateVars.CiliumAPIDomain,
		"CILIUM_DOCS_DOMAIN":         templateVars.CiliumDocsDomain,
		"GITHUB_BASE_DOMAIN":         templateVars.GithubBaseDomain,
		"DOCKER_REGISTRY_DOMAIN":     templateVars.DockerRegistryDomain,
		"TEST_DOMAIN_PATTERN":        templateVars.TestDomainPattern,
	}

	// Categorize each variable based on its discovery status
	for varName, value := range variableValues {
		if status, exists := templateVars.DiscoveryStatus[varName]; exists {
			statusLower := strings.ToLower(status)
			switch {
			case strings.Contains(statusLower, "fallback"):
				fallbacks[varName] = value
				fallbackCount++
			case strings.Contains(statusLower, "derived") || strings.Contains(statusLower, "calculated") || strings.Contains(statusLower, "extracted"):
				derived[varName] = value
				derivedCount++
			default:
				discovered[varName] = value
				discoveredCount++
			}
		}
	}

	// Calculate reliability score
	totalVars := discoveredCount + fallbackCount + derivedCount
	reliabilityScore := 0.0
	if totalVars > 0 {
		// Discovered = 1.0, Derived = 0.8, Fallback = 0.3
		reliabilityScore = (float64(discoveredCount)*1.0 + float64(derivedCount)*0.8 + float64(fallbackCount)*0.3) / float64(totalVars)
	}

	// Determine overall discovery quality
	var discoveryQuality string
	var qualityLevel FrontendLogLevel
	if reliabilityScore >= 0.8 {
		discoveryQuality = "HIGH"
		qualityLevel = FrontendLevelSuccess
	} else if reliabilityScore >= 0.6 {
		discoveryQuality = "MEDIUM"
		qualityLevel = FrontendLevelWarning
	} else {
		discoveryQuality = "LOW"
		qualityLevel = FrontendLevelWarning
	}

	// Build log entry data
	data := map[string]interface{}{
		"testName":         testName,
		"discoveryQuality": discoveryQuality,
		"reliabilityScore": fmt.Sprintf("%.1f", reliabilityScore*100),
		"totalVariables":   totalVars,
		"discoveredCount":  discoveredCount,
		"derivedCount":     derivedCount,
		"fallbackCount":    fallbackCount,
		"discoveredValues": discovered,
		"derivedValues":    derived,
		"fallbackValues":   fallbacks,
	}

	// Add detailed discovery status for debugging
	if len(templateVars.DiscoveryStatus) > 0 {
		data["discoveryStatus"] = templateVars.DiscoveryStatus
	}

	message := fmt.Sprintf("Template variable discovery completed for %s - Quality: %s (%d discovered, %d derived, %d fallback)",
		testName, discoveryQuality, discoveredCount, derivedCount, fallbackCount)

	return f.LogEntryWithHierarchy(qualityLevel, EventStep, message, data, hierarchy)
}

func (f *FrontendJSONLogger) LogSuiteComplete(totalTests, passed, failed int, duration float64) error {
	return f.LogEntry(FrontendLevelInfo, EventSuiteComplete, "Completed Kubernetes diagnostic tests", map[string]interface{}{
		"totalTests": totalTests,
		"passed":     passed,
		"failed":     failed,
		"duration":   duration,
		"success":    failed == 0,
	})
}

// Group level logging methods
func (f *FrontendJSONLogger) LogGroupStart(groupName string, groupNumber, totalGroups int, subgroups []string, testsInGroup int) error {
	f.SetContext(groupName)
	hierarchy := &HierarchyContext{
		GroupId: groupName,
		Phase:   "setup",
	}
	return f.LogEntryWithHierarchy(FrontendLevelInfo, EventGroupStart, fmt.Sprintf("Starting %s tests", groupName), map[string]interface{}{
		"groupNumber":  groupNumber,
		"totalGroups":  totalGroups,
		"subgroups":    subgroups,
		"testsInGroup": testsInGroup,
		"groupName":    groupName,
	}, hierarchy)
}

func (f *FrontendJSONLogger) LogGroupComplete(groupName string, passed, failed int, duration float64) error {
	hierarchy := &HierarchyContext{
		GroupId: groupName,
		Phase:   "teardown",
	}
	return f.LogEntryWithHierarchy(FrontendLevelInfo, EventGroupComplete, fmt.Sprintf("Completed %s tests", groupName), map[string]interface{}{
		"groupName": groupName,
		"passed":    passed,
		"failed":    failed,
		"duration":  duration,
		"success":   failed == 0,
	}, hierarchy)
}

// Subgroup level logging methods
func (f *FrontendJSONLogger) LogSubgroupStart(subgroupName string, testsInSubgroup, subgroupNumber, totalSubgroups int) error {
	return f.LogSubgroupStartWithHierarchy(subgroupName, testsInSubgroup, subgroupNumber, totalSubgroups, nil)
}

func (f *FrontendJSONLogger) LogSubgroupStartWithHierarchy(subgroupName string, testsInSubgroup, subgroupNumber, totalSubgroups int, hierarchy *HierarchyContext) error {
	f.SetContext(subgroupName)
	if hierarchy == nil {
		hierarchy = &HierarchyContext{
			SubgroupId: subgroupName,
			Phase:      "setup",
		}
	}
	return f.LogEntryWithHierarchy(FrontendLevelInfo, EventSubgroupStart, fmt.Sprintf("Starting %s subgroup", subgroupName), map[string]interface{}{
		"subgroup":        subgroupName,
		"testsInSubgroup": testsInSubgroup,
		"subgroupNumber":  subgroupNumber,
		"totalSubgroups":  totalSubgroups,
	}, hierarchy)
}

func (f *FrontendJSONLogger) LogSubgroupComplete(subgroupName string, passed, failed int, duration float64) error {
	return f.LogSubgroupCompleteWithHierarchy(subgroupName, passed, failed, duration, nil)
}

func (f *FrontendJSONLogger) LogSubgroupCompleteWithHierarchy(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	if hierarchy == nil {
		hierarchy = &HierarchyContext{
			SubgroupId: subgroupName,
			Phase:      "teardown",
		}
	}
	return f.LogEntryWithHierarchy(FrontendLevelInfo, EventSubgroupComplete, fmt.Sprintf("Completed %s subgroup", subgroupName), map[string]interface{}{
		"subgroup": subgroupName,
		"passed":   passed,
		"failed":   failed,
		"duration": duration,
		"success":  failed == 0,
	}, hierarchy)
}

// Test level logging methods
func (f *FrontendJSONLogger) LogTestStart(testName string, testNumber, totalTests int, subgroup string) error {
	return f.LogTestStartWithHierarchy(testName, testNumber, totalTests, subgroup, nil)
}

func (f *FrontendJSONLogger) LogTestStartWithHierarchy(testName string, testNumber, totalTests int, subgroup string, hierarchy *HierarchyContext) error {
	f.SetContext(testName)
	progress := float64(testNumber-1) / float64(totalTests) * 100

	if hierarchy == nil {
		hierarchy = &HierarchyContext{
			TestId: testName,
			Phase:  "execution",
		}
	}

	return f.LogEntryWithHierarchy(FrontendLevelInfo, EventTestStart, fmt.Sprintf("Starting %s test", testName), map[string]interface{}{
		"testNumber": testNumber,
		"totalTests": totalTests,
		"testName":   testName,
		"subgroup":   subgroup,
		"progress":   progress,
	}, hierarchy)
}

// Legacy method compatibility - delegates to unified method
func (f *FrontendJSONLogger) LogTestCompleteWithHierarchy(testName string, testNumber, totalTests int, success bool, duration float64, message string, hierarchy *HierarchyContext) error {
	// Extract group from hierarchy or context
	group := "unknown"
	subgroup := "unknown"

	if hierarchy != nil {
		if hierarchy.GroupId != "" {
			group = hierarchy.GroupId
		}
		if hierarchy.SubgroupId != "" {
			subgroup = hierarchy.SubgroupId
		}
	}

	// Create a basic test result from the message
	testResult := &TestResult{
		Success: success,
		Message: message,
		Details: []string{},
	}

	return f.LogTestComplete(testName, group, subgroup, testNumber, totalTests, success, duration, hierarchy, testResult)
}

// Operation level logging methods
func (f *FrontendJSONLogger) LogStep(stepName, message string, step, totalSteps int, status string) error {
	return f.LogEntry(FrontendLevelInfo, EventStep, message, map[string]interface{}{
		"step":       step,
		"totalSteps": totalSteps,
		"stepName":   stepName,
		"status":     status,
	})
}

func (f *FrontendJSONLogger) LogStepComplete(stepName string, success bool, message string) error {
	level := FrontendLevelSuccess
	if !success {
		level = FrontendLevelError
	}

	result := "PASS"
	if !success {
		result = "FAIL"
	}

	return f.LogEntry(level, EventStepComplete, message, map[string]interface{}{
		"stepName": stepName,
		"result":   result,
		"success":  success,
	})
}

func (f *FrontendJSONLogger) LogCommand(command, cmdID, workingDir string) error {
	return f.LogEntry(FrontendLevelDebug, EventCommand, "Executing command", map[string]interface{}{
		"command":    command,
		"cmdId":      cmdID,
		"workingDir": workingDir,
	})
}

func (f *FrontendJSONLogger) LogCommandResult(cmdID string, exitCode int, duration float64, stdout, stderr string, success bool) error {
	level := FrontendLevelDebug
	if !success {
		level = FrontendLevelError
	}

	message := "Command completed successfully"
	if !success {
		message = "Command failed"
	}

	return f.LogEntry(level, EventCommandResult, message, map[string]interface{}{
		"cmdId":    cmdID,
		"exitCode": exitCode,
		"duration": duration,
		"stdout":   stdout,
		"stderr":   stderr,
		"success":  success,
	})
}

func (f *FrontendJSONLogger) LogCleanup(operation string, resources []string) error {
	return f.LogEntry(FrontendLevelDebug, EventCleanup, fmt.Sprintf("%s starting", operation), map[string]interface{}{
		"operation": operation,
		"resources": resources,
	})
}

// Essential compatibility methods only
func (f *FrontendJSONLogger) LogSuiteInterrupted(reason string) error {
	return f.LogEntry(FrontendLevelWarning, EventSuiteInterrupted, "Test suite interrupted", map[string]interface{}{
		"reason": reason,
	})
}

func (f *FrontendJSONLogger) FlushOnTestComplete() error {
	return nil
}

func (f *FrontendJSONLogger) LogTestError(testName string, testNumber int, errorMsg, stage string, retryable bool) error {
	return f.LogEntry(FrontendLevelError, EventTestError, errorMsg, map[string]interface{}{
		"testNumber": testNumber,
		"testName":   testName,
		"error":      errorMsg,
		"stage":      stage,
		"retryable":  retryable,
	})
}

func (f *FrontendJSONLogger) LogTestRetry(testName string, attempt, maxAttempts int, reason string) error {
	return f.LogEntry(FrontendLevelWarning, EventTestError, fmt.Sprintf("Retrying %s (attempt %d/%d)", testName, attempt, maxAttempts), map[string]interface{}{
		"testName":    testName,
		"attempt":     attempt,
		"maxAttempts": maxAttempts,
		"reason":      reason,
	})
}

func (f *FrontendJSONLogger) LogAPICall(method, endpoint string, duration float64, statusCode int) error {
	return f.LogEntry(FrontendLevelDebug, EventCommand, fmt.Sprintf("Kubernetes API call: %s %s", method, endpoint), map[string]interface{}{
		"method":     method,
		"endpoint":   endpoint,
		"duration":   duration,
		"statusCode": statusCode,
	})
}

func (f *FrontendJSONLogger) LogStepCompleteWithForcedFlush(stepName string, success bool, message string) error {
	return f.LogStepComplete(stepName, success, message)
}

func (f *FrontendJSONLogger) flushBuffer() error {
	return nil
}

// LEGACY METHODS - Simple delegation to unified methods for compatibility
func (f *FrontendJSONLogger) LogL3TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success, Message: "", Details: []string{}}
	return f.LogTestComplete(testName, "l3-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (f *FrontendJSONLogger) LogL4TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success, Message: "", Details: []string{}}
	return f.LogTestComplete(testName, "l4-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (f *FrontendJSONLogger) LogL7TestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success, Message: "", Details: []string{}}
	return f.LogTestComplete(testName, "l7-policies", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (f *FrontendJSONLogger) LogNetworkingTestComplete(testName, subgroupName string, testNumber, totalTests int, success bool, duration float64, hierarchy *HierarchyContext) error {
	testResult := &TestResult{Success: success, Message: "", Details: []string{}}
	return f.LogTestComplete(testName, "networking", subgroupName, testNumber, totalTests, success, duration, hierarchy, testResult)
}

func (f *FrontendJSONLogger) LogL3PreCleanup(message string) error {
	return f.LogCleanupEvent("pre_cleanup", "l3-policies", message)
}

func (f *FrontendJSONLogger) LogL4PreCleanup(message string) error {
	return f.LogCleanupEvent("pre_cleanup", "l4-policies", message)
}

func (f *FrontendJSONLogger) LogL7PreCleanup(message string) error {
	return f.LogCleanupEvent("pre_cleanup", "l7-policies", message)
}

func (f *FrontendJSONLogger) LogNetworkingPreCleanup(message string) error {
	return f.LogCleanupEvent("pre_cleanup", "networking", message)
}

func (f *FrontendJSONLogger) LogL3SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return f.LogSubgroupSummary("l3-policies", subgroupName, passed, failed, duration, hierarchy)
}

func (f *FrontendJSONLogger) LogL4SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return f.LogSubgroupSummary("l4-policies", subgroupName, passed, failed, duration, hierarchy)
}

func (f *FrontendJSONLogger) LogL7SubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return f.LogSubgroupSummary("l7-policies", subgroupName, passed, failed, duration, hierarchy)
}

func (f *FrontendJSONLogger) LogNetworkingSubgroupSummary(subgroupName string, passed, failed int, duration float64, hierarchy *HierarchyContext) error {
	return f.LogSubgroupSummary("networking", subgroupName, passed, failed, duration, hierarchy)
}

// Close closes the frontend log file
func (f *FrontendJSONLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// GetLogFilePath returns the path to the frontend log file
func (f *FrontendJSONLogger) GetLogFilePath() string {
	if f.file != nil {
		return f.file.Name()
	}
	return ""
}
