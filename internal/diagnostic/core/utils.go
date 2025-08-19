package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// RetryConfig defines configuration parameters for the retry mechanism
type RetryConfig struct {
	MaxAttempts   int           // Maximum number of attempts
	InitialDelay  time.Duration // Initial delay before first retry
	MaxDelay      time.Duration // Maximum delay between retries
	Multiplier    float64       // Multiplier for exponential backoff
	JitterFactor  float64       // Factor for random jitter (0-1)
	RetryableErrs []string      // Substrings of error messages that should trigger retries
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  20,                // Significantly increased attempts
		InitialDelay: 5 * time.Second,   // Longer initial delay
		MaxDelay:     180 * time.Second, // Much longer max delay
		Multiplier:   3.0,               // Even faster exponential backoff
		JitterFactor: 0.5,               // Higher jitter to reduce synchronization
		RetryableErrs: []string{
			"client rate limiter Wait returned an error",
			"context deadline exceeded",
			"dial tcp: i/o timeout",
			"too many requests",
			"timed out waiting",
			"connection refused",
			"object is being deleted",
			"etcdserver: request timed out",
			"net/http: request canceled",
			"error contacting apiserver",
			"failed to create namespace", // Add more specific namespace errors
			"already exists",
		},
	}
}

// NamespaceRetryConfig returns a specialized retry configuration for namespace operations
// These operations are more prone to rate limiting during concurrent tests
func NamespaceRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  15,                // More attempts for critical namespace operations
		InitialDelay: 3 * time.Second,   // Longer initial delay
		MaxDelay:     120 * time.Second, // Much longer max delay
		Multiplier:   3.0,               // Faster exponential backoff
		JitterFactor: 0.5,               // Higher jitter to reduce synchronization
		RetryableErrs: []string{
			"client rate limiter Wait returned an error",
			"context deadline exceeded",
			"dial tcp: i/o timeout",
			"too many requests",
			"timed out waiting",
			"connection refused",
			"object is being deleted",
			"etcdserver: request timed out",
			"net/http: request canceled",
			"error contacting apiserver",
			"namespace creation", // Additional namespace-specific errors
			"already exists",
		},
	}
}

// WithRetry executes the provided function with exponential backoff retry logic
func WithRetry(ctx context.Context, operation string, fn func() error, config RetryConfig) error {
	var err error
	delay := config.InitialDelay

	// Execute the operation with retries
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err = fn()

		// If successful or context canceled, return immediately
		if err == nil || ctx.Err() != nil {
			return err
		}

		// Check if the error is retryable
		isRetryable := false
		errStr := err.Error()
		for _, retryErrSubstr := range config.RetryableErrs {
			if strings.Contains(errStr, retryErrSubstr) {
				isRetryable = true
				break
			}
		}

		// If not retryable or last attempt, return the error
		if !isRetryable || attempt == config.MaxAttempts {
			return err
		}

		// Calculate jitter for the delay to prevent synchronization problems
		jitter := time.Duration(rand.Float64() * config.JitterFactor * float64(delay))
		actualDelay := delay + jitter

		// Wait for the calculated delay with rate limit retry

		// Wait for the calculated delay
		select {
		case <-time.After(actualDelay):
			// Continue to next attempt
		case <-ctx.Done():
			// Context canceled during delay
			return ctx.Err()
		}

		// Exponential backoff for next attempt's delay
		delay = time.Duration(math.Min(
			float64(config.MaxDelay),
			float64(delay)*config.Multiplier,
		))
	}

	return err
}

// GetNodeInfo retrieves basic node information
func GetNodeInfo(ctx context.Context, verbose bool) (map[string]string, error) {
	nodeInfo := make(map[string]string)

	// This is a placeholder implementation
	// In a real scenario, this would use the Kubernetes client to get node info
	nodeInfo["node1"] = "worker-node-1"
	nodeInfo["node2"] = "worker-node-2"

	return nodeInfo, nil
}

// GetPodFailureReason extracts failure information from a pod
func GetPodFailureReason(pod *corev1.Pod) string {
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}

	if pod.Status.Message != "" {
		return pod.Status.Message
	}

	// Check container statuses
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Message != "" {
			return strings.TrimSpace(containerStatus.State.Waiting.Reason + ": " +
				containerStatus.State.Waiting.Message)
		}

		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Message != "" {
			return "Container terminated: " + strings.TrimSpace(containerStatus.State.Terminated.Message)
		}
	}

	return "Unknown failure"
}

// IsPodStuckDueToNetworking checks if a pod appears to be stuck due to networking issues
func IsPodStuckDueToNetworking(pod *corev1.Pod) bool {
	// Only consider pods that have been around for at least 60 seconds
	// to avoid false positives during normal pod startup
	if !pod.CreationTimestamp.Time.Before(time.Now().Add(-60 * time.Second)) {
		return false
	}

	// Check for serious networking issues
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil {
			reason := containerStatus.State.Waiting.Reason
			message := containerStatus.State.Waiting.Message

			// Only consider specific network-related issues
			if reason == "NetworkNotReady" || reason == "NetworkPluginNotReady" {
				return true
			}

			// Check for CNI-related error messages
			if message != "" && (strings.Contains(strings.ToLower(message), "cni") ||
				strings.Contains(strings.ToLower(message), "network") ||
				strings.Contains(strings.ToLower(message), "cilium")) {
				return true
			}
		}
	}

	return false
}

// ElapsedSeconds calculates the elapsed time in seconds from a start time
func ElapsedSeconds(startTime time.Time) float64 {
	return time.Since(startTime).Seconds()
}

// Enhanced verbose formatting functions for detailed test output

// FormatVerboseTestFailure formats a test failure with detailed command trace and error information
func FormatVerboseTestFailure(testName string, duration float64, commandHistory []VerboseCommandExecution, failurePoint string, errorDetails *VerboseErrorDetails) string {
	var output strings.Builder

	// Test failure header
	output.WriteString(fmt.Sprintf("❌ %s FAILED (%.1fs)\n", testName, duration))

	// Commands executed section
	if len(commandHistory) > 0 {
		output.WriteString("   🔧 Commands executed:\n")
		for _, cmd := range commandHistory {
			status := "✅"
			if !cmd.Success {
				status = fmt.Sprintf("❌ Exit %d", cmd.ExitCode)
			}
			// Clean up command for display (remove namespace clutter)
			cleanCommand := cleanCommandForDisplay(cmd.Command)
			output.WriteString(fmt.Sprintf("      %s → %s (%.1fs)\n", cleanCommand, status, cmd.Duration))
		}
	}

	// Failure point
	if failurePoint != "" {
		output.WriteString(fmt.Sprintf("   📍 Failure point: %s\n", failurePoint))
	}

	// Error details
	if errorDetails != nil {
		output.WriteString("   🔍 Error details:\n")
		if errorDetails.Command != "" {
			output.WriteString(fmt.Sprintf("      Command: %s\n", errorDetails.Command))
		}
		if errorDetails.Output != "" {
			// Clean up output (remove excessive whitespace, limit length)
			cleanOutput := cleanOutputForDisplay(errorDetails.Output)
			output.WriteString(fmt.Sprintf("      Output: %s\n", cleanOutput))
		}
	}

	return output.String()
}

// FormatVerboseTestSuccess formats a successful test result
func FormatVerboseTestSuccess(testName string, duration float64) string {
	return fmt.Sprintf("✅ %s PASS (%.1fs)", testName, duration)
}

// cleanCommandForDisplay cleans up a command string for better display
func cleanCommandForDisplay(command string) string {
	// Remove redundant namespace flags that clutter output
	cleaned := command

	// Replace long kubectl commands with shorter versions
	if strings.Contains(cleaned, "kubectl") {
		// Simplify kubectl exec commands
		if strings.Contains(cleaned, "kubectl exec") {
			parts := strings.Fields(cleaned)
			simplified := []string{}
			skip := 0

			for i, part := range parts {
				if skip > 0 {
					skip--
					continue
				}

				if part == "-n" && i+1 < len(parts) {
					// Skip namespace flag and value for cleaner display
					skip = 1
					continue
				}

				simplified = append(simplified, part)
			}
			cleaned = strings.Join(simplified, " ")
		}

		// Shorten file paths for apply commands
		if strings.Contains(cleaned, "kubectl apply -f /tmp/") {
			cleaned = strings.ReplaceAll(cleaned, "/tmp/policy-", "policy-")
		}
	}

	return cleaned
}

// cleanOutputForDisplay cleans up command output for better display
func cleanOutputForDisplay(output string) string {
	// Remove excessive whitespace
	lines := strings.Split(output, "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	result := strings.Join(cleanLines, " ")

	// Limit output length to prevent overwhelming display
	if len(result) > 200 {
		result = result[:197] + "..."
	}

	return result
}

// CreateEnhancedTestResult creates an enhanced test result from a basic test result and command executor
func CreateEnhancedTestResult(basicResult TestResult, executor *CommandExecutor, failurePoint string, errorDetails *VerboseErrorDetails) EnhancedTestResult {
	return EnhancedTestResult{
		TestResult:       basicResult,
		ExecutedCommands: executor.GetCommandHistory(),
		FailurePoint:     failurePoint,
		ErrorDetails:     errorDetails,
	}
}

// ExtractErrorDetailsFromCommand extracts error details from a failed command execution
func ExtractErrorDetailsFromCommand(commandHistory []VerboseCommandExecution, failureContext string) *VerboseErrorDetails {
	// Find the first failed command
	for _, cmd := range commandHistory {
		if !cmd.Success {
			return &VerboseErrorDetails{
				Command: cmd.Command,
				Output:  getFailureOutput(cmd),
				Stage:   failureContext,
			}
		}
	}

	return nil
}

// getFailureOutput extracts the most relevant error information from command output
func getFailureOutput(cmd VerboseCommandExecution) string {
	if cmd.Stderr != "" {
		return cmd.Stderr
	}
	if cmd.Stdout != "" {
		return cmd.Stdout
	}
	return fmt.Sprintf("Command failed with exit code %d", cmd.ExitCode)
}

// FormatVerboseTestResultForHierarchy formats test results for hierarchical display
func FormatVerboseTestResultForHierarchy(testName string, result EnhancedTestResult, duration float64, verbose bool) string {
	if result.Success {
		return FormatVerboseTestSuccess(testName, duration)
	}

	// Only show detailed failure information in verbose mode
	if verbose {
		return FormatVerboseTestFailure(testName, duration, result.ExecutedCommands, result.FailurePoint, result.ErrorDetails)
	}

	// Non-verbose mode: simple failure message
	return fmt.Sprintf("❌ %s FAILED (%.1fs)", testName, duration)
}

// PolicyExpectations maps policy names to their expected behaviors
var PolicyExpectations = map[string]PolicyExpectation{
	// L3 Policy expectations
	"cidr-ingress-policy": {
		Expected:    "ICMP ping success",
		Explanation: "policy allows traffic from client pod CIDR",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"cidr-egress-policy": {
		Expected:    "ICMP ping success to external CIDR",
		Explanation: "policy allows egress to specific CIDR ranges",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"cidr-with-except-policy": {
		Expected:    "ICMP ping success with exceptions",
		Explanation: "policy allows CIDR access except blocked ranges",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"dns-based-policy": {
		Expected:    "DNS resolution success",
		Explanation: "policy allows egress to specific DNS names",
		Protocol:    "DNS",
		ShouldAllow: true,
	},
	"entities-based-policy": {
		Expected:    "Connectivity to world entities",
		Explanation: "policy allows egress to world/cluster entities",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"endpoints-label-selector": {
		Expected:    "Connection to labeled endpoints",
		Explanation: "policy allows traffic to endpoints with specific labels",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"kubernetes-service-policy": {
		Expected:    "Service connectivity success",
		Explanation: "policy allows access to Kubernetes services",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"node-based-policy-clusterwide": {
		Expected:    "Node-to-node communication",
		Explanation: "policy allows traffic based on node selectors",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"node-cidr-policy": {
		Expected:    "Node CIDR connectivity",
		Explanation: "policy allows traffic to node CIDR ranges",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"traditional-node-selector": {
		Expected:    "Traditional node selection working",
		Explanation: "policy uses traditional node selector mechanisms",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"pod-node-name-policy": {
		Expected:    "Pod node name policy working",
		Explanation: "policy allows traffic based on pod node name",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	// Additional L3 test key mappings (test key + "-policy")
	"cidr-except-policy": {
		Expected:    "ICMP ping success with exceptions",
		Explanation: "policy allows CIDR access except blocked ranges",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"endpoints-label-policy": {
		Expected:    "Connection to labeled endpoints",
		Explanation: "policy allows traffic to endpoints with specific labels",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"node-selector-policy": {
		Expected:    "Traditional node selection working",
		Explanation: "policy uses traditional node selector mechanisms",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"node-based-policy": {
		Expected:    "Node-to-node communication",
		Explanation: "policy allows traffic based on node selectors",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},

	// L4 Policy expectations
	"tcp-port-ingress-policy": {
		Expected:    "HTTP 200 responses",
		Explanation: "policy allows TCP ingress on ports 80,443",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"tcp-port-egress-policy": {
		Expected:    "HTTP 200 responses",
		Explanation: "policy allows TCP egress on ports 80,443",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"multiple-port-policy": {
		Expected:    "Multi-port connectivity",
		Explanation: "policy allows TCP on multiple specified ports",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"port-range-policy": {
		Expected:    "Port range connectivity",
		Explanation: "policy allows TCP on port ranges 8000-9000",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"icmp-type-policy": {
		Expected:    "ICMP ping success",
		Explanation: "policy allows specific ICMP types (ping)",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"icmpv6-type-policy": {
		Expected:    "ICMPv6 ping success",
		Explanation: "policy allows specific ICMPv6 types",
		Protocol:    "ICMPv6",
		ShouldAllow: true,
	},
	"mixed-icmp-policy": {
		Expected:    "Mixed ICMP connectivity",
		Explanation: "policy allows both IPv4 and IPv6 ICMP",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"basic-sni-policy": {
		Expected:    "TLS SNI connection success",
		Explanation: "policy allows TLS connections with specific SNI",
		Protocol:    "HTTPS",
		ShouldAllow: true,
	},
	"multi-domain-sni-policy": {
		Expected:    "Multi-domain TLS success",
		Explanation: "policy allows TLS to multiple SNI domains",
		Protocol:    "HTTPS",
		ShouldAllow: true,
	},
	"combined-l4-sni-policy": {
		Expected:    "Combined L4+SNI connectivity",
		Explanation: "policy combines L4 ports with SNI filtering",
		Protocol:    "HTTPS",
		ShouldAllow: true,
	},

	// L7 Policy expectations
	"basic-http-get-policy": {
		Expected:    "HTTP 200 for GET requests",
		Explanation: "L7 policy allows HTTP GET method only",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"path-method-policy": {
		Expected:    "HTTP access to specific paths",
		Explanation: "L7 policy allows specific HTTP paths and methods",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"http-with-headers-policy": {
		Expected:    "HTTP with required headers",
		Explanation: "L7 policy requires specific HTTP headers",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"dns-matchname-policy": {
		Expected:    "DNS resolution to specific names",
		Explanation: "L7 DNS policy allows specific domain names",
		Protocol:    "DNS",
		ShouldAllow: true,
	},
	"dns-matchpattern-policy": {
		Expected:    "DNS pattern matching success",
		Explanation: "L7 DNS policy allows domains matching patterns",
		Protocol:    "DNS",
		ShouldAllow: true,
	},
	"deny-ingress-policy": {
		Expected:    "Ingress connections blocked",
		Explanation: "policy explicitly denies ingress traffic",
		Protocol:    "HTTP",
		ShouldAllow: false,
	},
	"deny-with-allow-policy": {
		Expected:    "Selective access (deny with exceptions)",
		Explanation: "policy denies traffic but allows specific exceptions",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},

	// Networking Test expectations
	"pod-to-pod-same-node": {
		Expected:    "Pod-to-pod connectivity success",
		Explanation: "Network infrastructure working correctly - pod-to-pod connectivity success",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"pod-to-pod-cross-node": {
		Expected:    "Pod-to-pod connectivity across nodes",
		Explanation: "Network infrastructure working correctly - pod-to-pod connectivity across nodes",
		Protocol:    "ICMP",
		ShouldAllow: true,
	},
	"service-clusterip": {
		Expected:    "ClusterIP service connectivity",
		Explanation: "Network infrastructure working correctly - clusterip service connectivity",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"service-nodeport": {
		Expected:    "NodePort service connectivity",
		Explanation: "Network infrastructure working correctly - nodeport service connectivity",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"service-loadbalancer": {
		Expected:    "LoadBalancer service connectivity",
		Explanation: "Network infrastructure working correctly - loadbalancer service connectivity",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"service-cross-node": {
		Expected:    "Cross-node service connectivity",
		Explanation: "Network infrastructure working correctly - cross-node service connectivity",
		Protocol:    "HTTP",
		ShouldAllow: true,
	},
	"dns-resolution": {
		Expected:    "Service FQDN DNS resolution",
		Explanation: "Network behavior confirmed - service fqdn dns resolution",
		Protocol:    "DNS",
		ShouldAllow: true,
	},
}

// FormatEnhancedSuccessMessage formats a comprehensive success message showing expected vs received results
func FormatEnhancedSuccessMessage(result EnhancedTestResult, testName string, duration float64) string {
	var output strings.Builder

	// Main success line
	output.WriteString(fmt.Sprintf("✅ PASS (%.1fs)\n", duration))

	// Expected outcome
	if result.ExpectedOutcome != "" && result.PolicyBehavior != "" {
		output.WriteString(fmt.Sprintf("   📋 Expected: %s (%s)\n", result.ExpectedOutcome, result.PolicyBehavior))
	}

	// Received results with individual connection details
	if len(result.TestDetails) > 0 {
		receivedParts := []string{}
		for _, detail := range result.TestDetails {
			receivedParts = append(receivedParts,
				fmt.Sprintf("%s %s from %s → %s (%.3fs)",
					detail.Protocol, detail.StatusCode, detail.Source, detail.Target, detail.Duration))
		}
		output.WriteString(fmt.Sprintf("   📥 Received: %s\n", strings.Join(receivedParts, ", ")))
	} else if result.ReceivedOutcome != "" {
		output.WriteString(fmt.Sprintf("   📥 Received: %s\n", result.ReceivedOutcome))
	}

	// Result interpretation
	if result.ExpectedOutcome != "" {
		interpretationText := strings.ToLower(result.ExpectedOutcome)
		if strings.Contains(interpretationText, "success") || strings.Contains(interpretationText, "200") {
			output.WriteString(fmt.Sprintf("   🎯 Result: Policy working correctly - %s\n", interpretationText))
		} else {
			output.WriteString(fmt.Sprintf("   🎯 Result: Policy behavior confirmed - %s\n", interpretationText))
		}
	} else {
		output.WriteString("   🎯 Result: Policy working correctly - connectivity successful\n")
	}

	return output.String()
}

// GetPolicyExpectation retrieves the expected behavior for a given policy name
func GetPolicyExpectation(policyName string) (PolicyExpectation, bool) {
	expectation, exists := PolicyExpectations[policyName]
	return expectation, exists
}

// CreateEnhancedTestResultWithExpectation creates an enhanced test result with policy expectations
func CreateEnhancedTestResultWithExpectation(basicResult TestResult, policyName string, testDetails []ConnectivityResult, executor *CommandExecutor) EnhancedTestResult {
	enhanced := EnhancedTestResult{
		TestResult:  basicResult,
		TestDetails: testDetails,
	}

	// Add command history if executor is provided
	if executor != nil {
		enhanced.ExecutedCommands = executor.GetCommandHistory()
	}

	// Add policy expectations if available
	if expectation, exists := PolicyExpectations[policyName]; exists {
		enhanced.ExpectedOutcome = expectation.Expected
		enhanced.PolicyBehavior = expectation.Explanation

		// Generate received outcome from test details
		if len(testDetails) > 0 {
			var outcomes []string
			for _, detail := range testDetails {
				if detail.Success {
					outcomes = append(outcomes, fmt.Sprintf("%s %s from %s",
						detail.Protocol, detail.StatusCode, detail.Source))
				} else {
					outcomes = append(outcomes, fmt.Sprintf("%s failed from %s",
						detail.Protocol, detail.Source))
				}
			}
			enhanced.ReceivedOutcome = strings.Join(outcomes, ", ")
		}
	}

	return enhanced
}

// FormatEnhancedTestResultForHierarchy formats enhanced test results for hierarchical display
func FormatEnhancedTestResultForHierarchy(testName string, result EnhancedTestResult, duration float64, verbose bool) string {
	if result.Success {
		// For networking tests (which don't have policy suffix), always use enhanced formatting
		if isNetworkingTest(testName) {
			return FormatNetworkingSuccessMessage(result, testName, duration)
		}

		// Use enhanced success formatting if we have detailed results
		if verbose && (result.ExpectedOutcome != "" || len(result.TestDetails) > 0) {
			return FormatEnhancedSuccessMessage(result, testName, duration)
		}
		// Fall back to simple success format
		return FormatVerboseTestSuccess(testName, duration)
	}

	// Handle failure cases (existing verbose failure logic)
	if verbose {
		return FormatVerboseTestFailure(testName, duration, result.ExecutedCommands, result.FailurePoint, result.ErrorDetails)
	}

	// Non-verbose mode: simple failure message
	return fmt.Sprintf("❌ %s FAILED (%.1fs)", testName, duration)
}

// FormatEnhancedTestSummary formats comprehensive test summaries with Expected vs Received details
func FormatEnhancedTestSummary(timedResults []TimedTestResult, testNames []string, displayNames map[string]string, verbose bool) {
	// Use the global logger instead of direct console output
	logger := GetGlobalMultiChannelLogger()
	if logger != nil {
		// Use structured logging for better output control
		for i, result := range timedResults {
			testKey := testNames[i]
			displayName := displayNames[testKey]
			if displayName == "" {
				displayName = testKey
			}

			duration := result.EndTime.Sub(result.StartTime).Seconds()

			if result.TestResult.Success {
				logger.LogInfo("✅ %s: PASSED (%.1fs)", displayName, duration)
			} else {
				logger.LogError("❌ %s: FAILED (%.1fs)", displayName, duration)
			}
		}
		return
	}

	// Fallback to simplified console output if no logger available
	for i, result := range timedResults {
		testKey := testNames[i]
		displayName := displayNames[testKey]
		if displayName == "" {
			displayName = testKey
		}

		duration := result.EndTime.Sub(result.StartTime).Seconds()

		if result.TestResult.Success {
			fmt.Printf("✅ %s: PASSED (%.1fs)\n", displayName, duration)
		} else {
			fmt.Printf("❌ %s: FAILED (%.1fs)\n", displayName, duration)
		}
	}
}

// extractRealConnectivityData extracts real connectivity data from test result details
func extractRealConnectivityData(result TestResult) []string {
	var receivedParts []string

	// Extract real connectivity data from result details
	for _, detail := range result.Details {
		if strings.HasPrefix(detail, "CONNECTIVITY_DATA:") {
			// Extract the connectivity data (remove the prefix)
			connectivityData := strings.TrimPrefix(detail, "CONNECTIVITY_DATA:")
			receivedParts = append(receivedParts, connectivityData)
		} else if strings.HasPrefix(detail, "ENHANCED_DATA:") {
			// Parse enhanced data format: "ENHANCED_DATA:expected|received"
			enhancedInfo := strings.TrimPrefix(detail, "ENHANCED_DATA:")
			parts := strings.Split(enhancedInfo, "|")
			if len(parts) >= 2 {
				receivedParts = append(receivedParts, parts[1])
			}
		}
	}

	return receivedParts
}

// extractRealPolicyData extracts real connectivity data from policy test results
func extractRealPolicyData(result TestResult, testKey string) []string {
	var receivedParts []string

	// First try to extract real connectivity data
	realData := extractRealConnectivityData(result)
	if len(realData) > 0 {
		return realData
	}

	// If no real connectivity data, extract from test result message
	if result.Success && result.Message != "" {
		// For successful policy tests, try to extract timing information
		if strings.Contains(result.Message, "passed") || strings.Contains(result.Message, "successful") {
			receivedParts = append(receivedParts, fmt.Sprintf("Policy test completed successfully"))
		}
	}

	return receivedParts
}

// extractLatencyFromMessage extracts latency information from test result messages
func extractLatencyFromMessage(message string) string {
	// Handle messages like "0.09ms" or "Both same-node and cross-node connectivity tests passed"
	if strings.Contains(message, "ms") && len(message) < 20 {
		// Simple latency message like "0.09ms"
		latency := strings.TrimSuffix(message, "ms")
		if _, err := fmt.Sscanf(latency, "%f", new(float64)); err == nil {
			// Convert ms to seconds for consistency
			if val, err := fmt.Sscanf(latency, "%f", new(float64)); err == nil && val == 1 {
				var f float64
				fmt.Sscanf(latency, "%f", &f)
				return fmt.Sprintf("%.3f", f/1000.0) // Convert ms to seconds
			}
		}
	}
	return ""
}

// isNetworkingTest checks if a test is a networking test (as opposed to a policy test)
func isNetworkingTest(testName string) bool {
	networkingTests := []string{
		"pod-to-pod-same-node",
		"pod-to-pod-cross-node",
		"service-clusterip",
		"service-nodeport",
		"service-loadbalancer",
		"service-cross-node",
		"dns-resolution",
	}

	for _, netTest := range networkingTests {
		if testName == netTest {
			return true
		}
	}
	return false
}

// FormatNetworkingSuccessMessage formats networking test success messages with L3/L4/L7 style
func FormatNetworkingSuccessMessage(result EnhancedTestResult, testName string, duration float64) string {
	var output strings.Builder

	// Main success line
	output.WriteString(fmt.Sprintf("✅ PASS (%.1fs)\n", duration))

	// Get expectation from PolicyExpectations map
	if expectation, exists := PolicyExpectations[testName]; exists {
		// Expected outcome
		output.WriteString(fmt.Sprintf("   📋 Expected: %s (%s)\n", expectation.Expected, expectation.Explanation))

		// Received data - only use real data from test results
		var receivedData string
		if result.ReceivedOutcome != "" {
			receivedData = result.ReceivedOutcome
		} else {
			// Extract only real connectivity data from test results
			receivedParts := extractRealConnectivityData(result.TestResult)
			receivedData = strings.Join(receivedParts, ", ")
		}

		if receivedData != "" {
			output.WriteString(fmt.Sprintf("   📥 Received: %s\n", receivedData))
		}

		// Result interpretation
		interpretationText := strings.ToLower(expectation.Expected)
		if strings.Contains(interpretationText, "success") || strings.Contains(interpretationText, "200") {
			output.WriteString(fmt.Sprintf("   🎯 Result: Network infrastructure working correctly - %s\n", interpretationText))
		} else {
			output.WriteString(fmt.Sprintf("   🎯 Result: Network behavior confirmed - %s\n", interpretationText))
		}
	} else {
		// No expectations defined - show only real data if available
		realData := extractRealConnectivityData(result.TestResult)
		if len(realData) > 0 {
			output.WriteString(fmt.Sprintf("   📥 Received: %s\n", strings.Join(realData, ", ")))
		}
		// Only show result if we have real data
		if len(realData) > 0 {
			output.WriteString(fmt.Sprintf("   🎯 Result: Test completed with real connectivity data\n"))
		}
	}

	return output.String()
}
