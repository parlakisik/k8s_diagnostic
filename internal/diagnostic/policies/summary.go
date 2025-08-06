package diagnostic

import (
	"fmt"
	"strings"

	"k8s-diagnostic/internal/diagnostic/core"
)

// extractSuccessMessage extracts a clean success message from test result
func extractSuccessMessage(message string) string {
	// Remove any prefixes like "[SUBGROUP] testname: "
	if idx := strings.LastIndex(message, ": "); idx >= 0 {
		return message[idx+2:]
	}
	return message
}

// extractErrorMessage extracts a clean error message from test result
func extractErrorMessage(message string) string {
	// Remove any prefixes like "[SUBGROUP] testname: "
	if idx := strings.LastIndex(message, ": "); idx >= 0 {
		return message[idx+2:]
	}
	return message
}

// FormatDetailedTestSummary formats and displays a detailed test summary with enhanced format
func FormatDetailedTestSummary(timedResults []core.TimedTestResult, testNames []string, displayNames map[string]string) {
	fmt.Println("\n📊 Detailed Test Summary:")
	fmt.Println("================================================================================")

	for i, result := range timedResults {
		testKey := testNames[i]
		displayName := displayNames[testKey]
		if displayName == "" {
			displayName = testKey // fallback to test key if no display name
		}

		duration := result.EndTime.Sub(result.StartTime).Seconds()

		if result.TestResult.Success {
			fmt.Printf("✅ %s: PASSED (%.1fs)\n", displayName, duration)

			// Enhanced format with Expected/Received/Result
			expectedBehavior := getExpectedBehaviorForTest(testKey)
			actualResult := extractActualResultFromMessage(result.TestResult.Message)
			resultInterpretation := generateResultInterpretation(testKey, true, result.TestResult.Message)

			fmt.Printf("   📋 Expected: %s\n", expectedBehavior)
			fmt.Printf("   📥 Received: %s\n", actualResult)
			fmt.Printf("   🎯 Result: %s\n", resultInterpretation)
		} else {
			fmt.Printf("❌ %s: FAILED (%.1fs)\n", displayName, duration)

			// Enhanced format for failures
			expectedBehavior := getExpectedBehaviorForTest(testKey)
			errorDetails := extractErrorMessage(result.TestResult.Message)
			resultInterpretation := generateResultInterpretation(testKey, false, result.TestResult.Message)

			fmt.Printf("   📋 Expected: %s\n", expectedBehavior)
			fmt.Printf("   💥 Error: %s\n", errorDetails)
			fmt.Printf("   🎯 Result: %s\n", resultInterpretation)
		}

		if i < len(timedResults)-1 {
			fmt.Println()
		}
	}

	fmt.Println("================================================================================")
}

// getExpectedBehaviorForTest returns expected behavior description for different test types
func getExpectedBehaviorForTest(testKey string) string {
	switch {
	// Networking tests
	case testKey == "dns-resolution":
		return "DNS resolution for Kubernetes services (FQDN resolution working)"
	case testKey == "service-nodeport":
		return "NodePort service connectivity (Network infrastructure working correctly)"
	case testKey == "service-clusterip":
		return "ClusterIP service connectivity (Internal cluster networking working)"
	case testKey == "pod-to-pod-same-node":
		return "Pod-to-pod connectivity on same node (CNI networking validation)"
	case testKey == "pod-to-pod-cross-node":
		return "Pod-to-pod connectivity across nodes (CNI cross-node networking)"

	// L4 policy tests
	case testKey == "tcp-port-ingress":
		return "HTTP 200 responses (policy allows TCP ingress on ports 80,443)"
	case testKey == "basic-sni":
		return "TLS SNI-based policy enforcement (SNI header inspection working)"
	case testKey == "tcp-port-egress":
		return "HTTP 200 responses (policy allows TCP egress on specified ports)"
	case strings.Contains(testKey, "port"):
		return "Port-based policy enforcement (L4 policy working correctly)"
	case strings.Contains(testKey, "icmp"):
		return "ICMP policy enforcement (ICMP traffic control working)"

	// L3 policy tests
	case strings.Contains(testKey, "cidr"):
		return "CIDR-based policy enforcement (IP address range filtering working)"
	case strings.Contains(testKey, "dns-based"):
		return "DNS-based policy enforcement (FQDN filtering working)"
	case strings.Contains(testKey, "node"):
		return "Node-based policy enforcement (Node selector policies working)"

	// L7 policy tests
	case strings.Contains(testKey, "http"):
		return "HTTP-level policy enforcement (L7 application filtering working)"
	case strings.Contains(testKey, "dns-match"):
		return "DNS pattern matching policy enforcement (DNS query filtering working)"

	default:
		return "Network policy test completion (Policy deployment and validation working)"
	}
}

// extractActualResultFromMessage extracts real values from test result messages
func extractActualResultFromMessage(message string) string {
	// Extract real data patterns from messages
	switch {
	case strings.Contains(message, "Success (200)"):
		return "HTTP 200 response received"
	case strings.Contains(message, "Success (") && strings.Contains(message, ")"):
		// Extract actual status code
		start := strings.Index(message, "Success (") + 9
		end := strings.Index(message[start:], ")")
		if end > 0 {
			statusCode := message[start : start+end]
			return fmt.Sprintf("HTTP %s response received", statusCode)
		}
		return "HTTP response received"
	case strings.Contains(message, "DNS resolution test passed"):
		return "DNS resolution successful"
	case strings.Contains(message, "completed successfully"):
		return "Test execution completed successfully"
	case strings.Contains(message, "FAILED") || strings.Contains(message, "failed"):
		return "Test execution failed"
	case strings.Contains(message, "timeout"):
		return "Connection timeout occurred"
	case strings.Contains(message, "connection refused"):
		return "Connection refused"
	default:
		// Clean up the message by removing test prefixes and return actual content
		cleanMessage := message
		if idx := strings.LastIndex(message, ": "); idx >= 0 {
			cleanMessage = message[idx+2:]
		}
		// If it's still a generic message, try to extract more specific info
		if len(cleanMessage) > 100 {
			cleanMessage = cleanMessage[:97] + "..."
		}
		return cleanMessage
	}
}

// generateResultInterpretation creates result interpretation based on actual test data
func generateResultInterpretation(testKey string, success bool, message string) string {
	if success {
		// Extract real timing and status information
		switch {
		case strings.Contains(message, "Success (200)"):
			return "Policy allows traffic - HTTP 200 confirmed"
		case strings.Contains(message, "Success ("):
			// Extract actual status code for interpretation
			start := strings.Index(message, "Success (") + 9
			end := strings.Index(message[start:], ")")
			if end > 0 {
				statusCode := message[start : start+end]
				return fmt.Sprintf("Policy allows traffic - HTTP %s confirmed", statusCode)
			}
			return "Policy allows traffic - connection successful"
		case strings.Contains(message, "DNS resolution test passed"):
			return "DNS policy allows resolution - service discovery working"
		case strings.Contains(message, "completed successfully") && strings.Contains(testKey, "policy"):
			return "Network policy deployed and validated successfully"
		case strings.Contains(message, "connectivity"):
			return "Network connectivity confirmed - infrastructure working"
		default:
			return "Test passed - expected behavior confirmed"
		}
	} else {
		// Extract real failure information
		switch {
		case strings.Contains(message, "timeout"):
			return "Connection blocked - policy or network issue causing timeout"
		case strings.Contains(message, "connection refused"):
			return "Connection refused - service not accessible or policy blocking"
		case strings.Contains(message, "DNS"):
			return "DNS resolution failed - DNS policy or infrastructure issue"
		case strings.Contains(message, "policy") && strings.Contains(message, "failed"):
			return "Policy deployment or validation failed"
		case strings.Contains(message, "not found"):
			return "Resource not found - deployment or configuration issue"
		default:
			// Try to extract actual error details
			errorPart := message
			if idx := strings.Index(message, "error:"); idx >= 0 {
				errorPart = message[idx+6:]
			} else if idx := strings.Index(message, "Error:"); idx >= 0 {
				errorPart = message[idx+6:]
			}
			if len(errorPart) > 80 {
				errorPart = errorPart[:77] + "..."
			}
			return fmt.Sprintf("Test failed - %s", strings.TrimSpace(errorPart))
		}
	}
}

// FormatBasicTestSummary formats the basic summary line
func FormatBasicTestSummary(totalTests, passedTests, failedTests int) {
	fmt.Printf("\n📊 Test Summary:\n")
	fmt.Printf("  Total Tests: %d, Passed: %d, Failed: %d\n", totalTests, passedTests, failedTests)
}
