package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TestDataCollector centralizes test execution data collection and user-friendly message generation
type TestDataCollector struct {
	testName            string
	testType            string
	startTime           time.Time
	environmentSnapshot *EnvironmentSnapshot
	executionData       *TestExecutionData
	logger              *FrontendJSONLogger
	infrastructure      *ClusterInfrastructure

	// HTTP API configuration
	httpClient   *http.Client
	webServerURL string
	testId       string
}

// NewTestDataCollector creates a new test data collector with infrastructure context
func NewTestDataCollector(testName, testType string, logger *FrontendJSONLogger, infrastructure *ClusterInfrastructure) *TestDataCollector {
	return &TestDataCollector{
		testName:       testName,
		testType:       testType,
		startTime:      time.Now(),
		logger:         logger,
		infrastructure: infrastructure,
		executionData: &TestExecutionData{
			PodsCreated:       []PodCreationResult{},
			ServicesCreated:   []ServiceCreationResult{},
			PoliciesApplied:   []PolicyApplicationResult{},
			ConnectivityTests: []ConnectivityTestResult{},
			FailurePoints:     []FailurePoint{},
		},
	}
}

// NewTestDataCollectorWithHTTP creates a new test data collector with HTTP API support
func NewTestDataCollectorWithHTTP(testName, testType, testId string, infrastructure *ClusterInfrastructure, webServerURL string) *TestDataCollector {
	return &TestDataCollector{
		testName:       testName,
		testType:       testType,
		startTime:      time.Now(),
		infrastructure: infrastructure,
		testId:         testId,
		webServerURL:   webServerURL,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		executionData: &TestExecutionData{
			PodsCreated:       []PodCreationResult{},
			ServicesCreated:   []ServiceCreationResult{},
			PoliciesApplied:   []PolicyApplicationResult{},
			ConnectivityTests: []ConnectivityTestResult{},
			FailurePoints:     []FailurePoint{},
		},
	}
}

// RecordPodCreation tracks pod creation attempt with user-friendly messaging
func (c *TestDataCollector) RecordPodCreation(podName, requestedNode string) *PodCreationResult {
	result := &PodCreationResult{
		PodName:       podName,
		RequestedNode: requestedNode,
		CreationTime:  time.Now(),
		Status:        "creating",
	}
	c.executionData.PodsCreated = append(c.executionData.PodsCreated, *result)

	// Generate user-friendly message based on test type and context
	title := "Creating test pod"
	description := fmt.Sprintf("Deploying %s", podName)
	context := "Setting up test environment for connectivity validation"

	if requestedNode != "" {
		description = fmt.Sprintf("Deploying %s on node %s", podName, requestedNode)
		if c.testType == "networking" && strings.Contains(c.testName, "cross-node") {
			context = "Setting up cross-node connectivity test - pods will be placed on different worker nodes"
		}
	}

	c.LogUserStepHTTP("setup", "progress", title, description, context, []string{}, map[string]interface{}{
		"podName":       podName,
		"requestedNode": requestedNode,
		"creationTime":  result.CreationTime,
		"testType":      c.testType,
	})

	return result
}

// UpdatePodStatus updates pod deployment status with intelligent user feedback
func (c *TestDataCollector) UpdatePodStatus(podName, status, actualNode, podIP, errorMsg string) {
	for i, pod := range c.executionData.PodsCreated {
		if pod.PodName == podName {
			c.executionData.PodsCreated[i].Status = status
			c.executionData.PodsCreated[i].ActualNode = actualNode
			c.executionData.PodsCreated[i].PodIP = podIP
			c.executionData.PodsCreated[i].Error = errorMsg

			if status == "running" {
				now := time.Now()
				c.executionData.PodsCreated[i].ReadyTime = &now
			}

			// Generate context-aware user messages
			c.generatePodStatusMessage(podName, status, actualNode, podIP, errorMsg)
			break
		}
	}
}

// generatePodStatusMessage creates intelligent user feedback based on pod status
func (c *TestDataCollector) generatePodStatusMessage(podName, status, actualNode, podIP, errorMsg string) {
	switch status {
	case "running":
		title := "Test pod ready"
		description := fmt.Sprintf("Pod %s is running", podName)
		context := "Test infrastructure is ready for connectivity testing"

		if actualNode != "" && podIP != "" {
			description = fmt.Sprintf("Pod %s is running on %s with IP %s", podName, actualNode, podIP)
			if c.testType == "networking" && strings.Contains(c.testName, "cross-node") {
				context = "Cross-node test environment ready - pods are distributed across worker nodes"
			}
		}

		c.LogUserStepHTTP("setup", "success", title, description, context, []string{}, map[string]interface{}{
			"podName":    podName,
			"actualNode": actualNode,
			"podIP":      podIP,
			"readyTime":  time.Now(),
		})

	case "failed", "timeout":
		title := "Pod deployment failed"
		description := fmt.Sprintf("Could not deploy %s", podName)
		context := "Cannot proceed with connectivity testing"
		hints := c.generatePodFailureHints(errorMsg)

		if errorMsg != "" {
			description = fmt.Sprintf("Could not deploy %s: %s", podName, errorMsg)
		}

		c.LogUserStepHTTP("setup", "failure", title, description, context, hints, map[string]interface{}{
			"podName":     podName,
			"error":       errorMsg,
			"failureTime": time.Now(),
			"actualNode":  actualNode,
		})

		// Record failure point for detailed diagnostics
		c.executionData.FailurePoints = append(c.executionData.FailurePoints, FailurePoint{
			Phase:     "setup",
			Component: "pod",
			Error:     errorMsg,
			Timestamp: time.Now(),
			Context: map[string]interface{}{
				"podName":       podName,
				"requestedNode": actualNode,
			},
			Remediation: hints,
		})
	}
}

// generatePodFailureHints provides intelligent remediation suggestions based on error patterns
func (c *TestDataCollector) generatePodFailureHints(errorMsg string) []string {
	hints := []string{}
	errorLower := strings.ToLower(errorMsg)

	switch {
	case strings.Contains(errorLower, "insufficient") || strings.Contains(errorLower, "resource"):
		hints = append(hints, "Check node resources and capacity")
		hints = append(hints, "Verify no resource quotas are blocking pod creation")
		if c.infrastructure != nil && c.infrastructure.NodeCount > 1 {
			hints = append(hints, "Consider distributing workload across multiple nodes")
		}

	case strings.Contains(errorLower, "image") || strings.Contains(errorLower, "pull"):
		hints = append(hints, "Verify container image is available and accessible")
		hints = append(hints, "Check image pull policies and secrets")

	case strings.Contains(errorLower, "node") || strings.Contains(errorLower, "schedule"):
		hints = append(hints, "Check node scheduling constraints and taints")
		hints = append(hints, "Verify nodes have sufficient resources")
		if c.infrastructure != nil {
			hints = append(hints, fmt.Sprintf("Cluster has %d nodes available", c.infrastructure.NodeCount))
		}

	case strings.Contains(errorLower, "security") || strings.Contains(errorLower, "policy"):
		hints = append(hints, "Check pod security policies and contexts")
		hints = append(hints, "Verify RBAC permissions for test namespace")

	default:
		hints = append(hints, "Check cluster events for detailed error information")
		hints = append(hints, "Verify cluster has sufficient resources")
		hints = append(hints, "Review pod specifications and constraints")
	}

	return hints
}

// RecordConnectivityTest tracks connectivity test initiation
func (c *TestDataCollector) RecordConnectivityTest(sourcePod, targetPod, testType string) *ConnectivityTestResult {
	result := &ConnectivityTestResult{
		SourcePod: sourcePod,
		TargetPod: targetPod,
		TestType:  testType,
		StartTime: time.Now(),
	}

	title := "Testing connectivity"
	description := fmt.Sprintf("Checking %s connectivity from %s to %s", testType, sourcePod, targetPod)
	context := "Validating network communication between test pods"

	if strings.Contains(c.testName, "cross-node") {
		context = "Testing cross-node network connectivity - validating CNI and cluster networking"
	}

	c.LogUserStepHTTP("execution", "progress", title, description, context, []string{}, map[string]interface{}{
		"sourcePod": sourcePod,
		"targetPod": targetPod,
		"testType":  testType,
		"startTime": result.StartTime,
	})

	return result
}

// UpdateConnectivityResult updates connectivity test results with intelligent analysis
func (c *TestDataCollector) UpdateConnectivityResult(test *ConnectivityTestResult, success bool, statusCode, responseBody, errorMsg string) {
	test.Success = success
	test.Duration = time.Since(test.StartTime).Seconds()
	test.HTTPStatusCode = statusCode
	test.ResponseBody = responseBody
	test.Error = errorMsg

	c.executionData.ConnectivityTests = append(c.executionData.ConnectivityTests, *test)

	// Generate intelligent user feedback based on result
	if success {
		c.generateConnectivitySuccessMessage(test, statusCode)
	} else {
		c.generateConnectivityFailureMessage(test, errorMsg)
	}
}

// generateConnectivitySuccessMessage creates user-friendly success feedback
func (c *TestDataCollector) generateConnectivitySuccessMessage(test *ConnectivityTestResult, statusCode string) {
	title := "Connectivity test passed"
	description := fmt.Sprintf("%s communication working", strings.ToUpper(test.TestType))
	context := "Network connectivity is functioning correctly"

	// Enhance message based on test type and infrastructure
	switch test.TestType {
	case "http":
		if statusCode != "" {
			description = fmt.Sprintf("HTTP communication working (Status: %s)", statusCode)
		}
		context = "Web traffic can flow properly between pods"

	case "dns":
		description = "DNS resolution working correctly"
		context = "Cluster DNS and service discovery are functioning"

	case "ping":
		description = "ICMP connectivity working"
		context = "Basic network connectivity is healthy"
	}

	// Add cross-node context if relevant
	if strings.Contains(c.testName, "cross-node") {
		context = "Cross-node network connectivity is healthy - your CNI configuration is working correctly"
	}

	hints := []string{}
	if c.testType == "networking" {
		hints = append(hints, "Your cluster networking is healthy")
		if c.infrastructure != nil && c.infrastructure.CNIProvider != "" {
			hints = append(hints, fmt.Sprintf("%s CNI is functioning correctly", c.infrastructure.CNIProvider))
		}
	}

	c.LogUserStepHTTP("execution", "success", title, description, context, hints, map[string]interface{}{
		"testType":   test.TestType,
		"statusCode": statusCode,
		"duration":   test.Duration,
		"sourcePod":  test.SourcePod,
		"targetPod":  test.TargetPod,
	})
}

// generateConnectivityFailureMessage creates intelligent failure analysis and remediation
func (c *TestDataCollector) generateConnectivityFailureMessage(test *ConnectivityTestResult, errorMsg string) {
	title := "Connectivity test failed"
	description := fmt.Sprintf("%s communication blocked", strings.ToUpper(test.TestType))
	context := "Network connectivity issue detected"
	hints := c.generateConnectivityFailureHints(errorMsg, test.TestType)

	// Enhance message based on error patterns
	if errorMsg != "" {
		description = fmt.Sprintf("%s communication blocked: %s", strings.ToUpper(test.TestType), errorMsg)
	}

	// Add infrastructure-specific context
	if c.infrastructure != nil {
		switch c.infrastructure.CNIProvider {
		case "cilium":
			context = "Network connectivity issue - check Cilium configuration and policies"
		case "calico":
			context = "Network connectivity issue - check Calico configuration and policies"
		case "flannel":
			context = "Network connectivity issue - check Flannel configuration"
		default:
			context = fmt.Sprintf("Network connectivity issue - check %s CNI configuration", c.infrastructure.CNIProvider)
		}
	}

	c.LogUserStepHTTP("execution", "failure", title, description, context, hints, map[string]interface{}{
		"testType": test.TestType,
		"error":    errorMsg,
		"duration": test.Duration,
	})

	// Record failure point for detailed diagnostics
	c.executionData.FailurePoints = append(c.executionData.FailurePoints, FailurePoint{
		Phase:     "execution",
		Component: "network",
		Error:     errorMsg,
		Timestamp: time.Now(),
		Context: map[string]interface{}{
			"testType":  test.TestType,
			"sourcePod": test.SourcePod,
			"targetPod": test.TargetPod,
		},
		Remediation: hints,
	})
}

// generateConnectivityFailureHints provides intelligent remediation based on failure patterns
func (c *TestDataCollector) generateConnectivityFailureHints(errorMsg, testType string) []string {
	hints := []string{}
	errorLower := strings.ToLower(errorMsg)

	// Generic network connectivity hints
	switch {
	case strings.Contains(errorLower, "connection refused"):
		hints = append(hints, "Check firewall rules between worker nodes")
		hints = append(hints, "Verify target service is running and accessible")
		if c.infrastructure != nil && c.infrastructure.CNIProvider != "" {
			hints = append(hints, fmt.Sprintf("Review %s CNI configuration", c.infrastructure.CNIProvider))
		}

	case strings.Contains(errorLower, "timeout"):
		hints = append(hints, "Check network latency between nodes")
		hints = append(hints, "Verify no network policies are blocking traffic")
		hints = append(hints, "Review cluster resource constraints")

	case strings.Contains(errorLower, "dns") || strings.Contains(errorLower, "resolve"):
		hints = append(hints, "Check CoreDNS pod status and configuration")
		hints = append(hints, "Verify cluster DNS settings")
		hints = append(hints, "Review DNS policy configurations")

	case strings.Contains(errorLower, "unreachable") || strings.Contains(errorLower, "no route"):
		hints = append(hints, "Check pod network routing configuration")
		hints = append(hints, "Verify CNI bridge and routing setup")
		if c.infrastructure != nil && c.infrastructure.NodeCount > 1 {
			hints = append(hints, "Verify cross-node routing is properly configured")
		}

	default:
		hints = append(hints, "Check cluster network policies")
		hints = append(hints, "Verify CNI configuration and health")
		hints = append(hints, "Review firewall rules between nodes")
	}

	// Add CNI-specific hints
	if c.infrastructure != nil {
		switch c.infrastructure.CNIProvider {
		case "cilium":
			hints = append(hints, "Check Cilium agent status: kubectl logs -n kube-system -l k8s-app=cilium")
			hints = append(hints, "Verify Cilium network policies: kubectl get ciliumnetworkpolicies --all-namespaces")

		case "calico":
			hints = append(hints, "Check Calico node status: kubectl get nodes -o wide")
			hints = append(hints, "Verify Calico network policies: kubectl get networkpolicies --all-namespaces")

		case "flannel":
			hints = append(hints, "Check Flannel daemon set: kubectl get daemonset -n kube-system")
			hints = append(hints, "Verify pod subnet configuration")
		}
	}

	return hints
}

// GetDetailedTestResult creates a comprehensive test result with all collected data
func (c *TestDataCollector) GetDetailedTestResult(finalResult TestResult) DetailedTestResult {
	// Create environment snapshot
	environmentSnapshot := &EnvironmentSnapshot{
		Infrastructure: c.infrastructure,
		TestNamespace:  "", // Will be set by caller
		StartTime:      c.startTime,
		NodeStates:     []NodeState{},       // Could be populated if needed
		CNIHealth:      CNIHealthSnapshot{}, // Could be populated if needed
	}

	// Create user context based on final results
	userContext := c.generateUserContext(finalResult)

	return DetailedTestResult{
		TestResult:          finalResult,
		EnvironmentSnapshot: environmentSnapshot,
		ExecutionData:       c.executionData,
		UserContext:         userContext,
	}
}

// generateUserContext creates user-friendly summary based on test execution
func (c *TestDataCollector) generateUserContext(result TestResult) *UserTestContext {
	if result.Success {
		return c.generateSuccessContext()
	}
	return c.generateFailureContext()
}

// generateSuccessContext creates user-friendly success summary
func (c *TestDataCollector) generateSuccessContext() *UserTestContext {
	summary := "Test completed successfully"
	details := "All connectivity checks passed"
	implications := "Your cluster networking is functioning correctly"
	hints := []string{}

	// Customize based on test type
	switch c.testType {
	case "networking":
		if strings.Contains(c.testName, "cross-node") {
			summary = "Cross-node networking working perfectly"
			details = "Pods can communicate seamlessly across your worker nodes"
			implications = "Your cluster can handle distributed workloads"
			hints = append(hints, "Your cluster is ready for distributed applications")
		} else {
			summary = "Network connectivity test passed"
			details = "Pod-to-pod communication is working correctly"
			implications = "Your cluster networking configuration is healthy"
		}
	}

	// Add infrastructure-specific insights
	if c.infrastructure != nil && c.infrastructure.CNIProvider != "" {
		implications += fmt.Sprintf(" - %s CNI is properly configured", c.infrastructure.CNIProvider)
	}

	return &UserTestContext{
		Summary:      summary,
		Details:      details,
		Implications: implications,
		Hints:        hints,
	}
}

// HTTP API Methods for real-time user messaging

// LogUserStepHTTP sends user-friendly messages directly to HTTP API for real-time frontend updates
func (c *TestDataCollector) LogUserStepHTTP(phase, status, title, description, context string, hints []string, technicalData map[string]interface{}) error {
	if c.httpClient == nil || c.webServerURL == "" || c.testId == "" {
		return nil
	}

	userMsg := UserMessage{
		Phase:       phase,
		Status:      status,
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}

	payload := map[string]interface{}{
		"testId":           c.testId,
		"testName":         c.testId, // Add testName field for frontend matching
		"type":             "user_step",
		"timestamp":        time.Now().Format(time.RFC3339),
		"userMessage":      userMsg,
		"technicalDetails": technicalData,
	}

	// Send HTTP POST to log-events API asynchronously to avoid blocking test execution
	go c.sendToHTTPAPI(payload)

	// Don't use fallback logger since it can be nil - HTTP API is primary now
	return nil
}

// sendToHTTPAPI sends payload to the web server's log-events endpoint
func (c *TestDataCollector) sendToHTTPAPI(payload map[string]interface{}) {
	if c.httpClient == nil || c.webServerURL == "" {
		return
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		// Silent failure - don't block test execution
		return
	}

	apiURL := c.webServerURL + "/api/log-events"
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// Non-blocking HTTP call with timeout
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Silent failure - test execution continues
		return
	}
	defer resp.Body.Close()
}

// generateFailureContext creates user-friendly failure summary with actionable insights
func (c *TestDataCollector) generateFailureContext() *UserTestContext {
	summary := "Test failed"
	details := "Connectivity issues detected"
	implications := "Your cluster networking needs attention"
	hints := []string{}

	// Analyze failure points for more specific guidance
	if len(c.executionData.FailurePoints) > 0 {
		lastFailure := c.executionData.FailurePoints[len(c.executionData.FailurePoints)-1]

		switch lastFailure.Phase {
		case "setup":
			summary = "Test environment setup failed"
			details = "Could not create required test resources"
			implications = "Cluster resource or configuration issue"

		case "execution":
			summary = "Network connectivity blocked"
			details = "Pods cannot communicate properly"
			implications = "CNI or network policy configuration issue"
		}

		// Use remediation hints from failure points
		if len(lastFailure.Remediation) > 0 {
			hints = lastFailure.Remediation
		}
	}

	// Add generic hints if none were generated
	if len(hints) == 0 {
		hints = append(hints, "Check cluster logs for detailed error information")
		hints = append(hints, "Verify cluster has sufficient resources")
		if c.infrastructure != nil && c.infrastructure.CNIProvider != "" {
			hints = append(hints, fmt.Sprintf("Review %s CNI configuration", c.infrastructure.CNIProvider))
		}
	}

	return &UserTestContext{
		Summary:      summary,
		Details:      details,
		Implications: implications,
		Hints:        hints,
	}
}
