package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PolicyTestConfig defines configuration for a single policy test
type PolicyTestConfig struct {
	PolicyPath    string // Path to YAML file (policy names extracted dynamically)
	GroupId       string // "l3-policies", "l4-policies", "l7-policies", "networking"
	SubgroupId    string // "ip-cidr", "port", "http", "pod-connectivity", "services", "dns", etc.
	TestId        string // "cidr-ingress", "tcp-port-ingress", "pod-to-pod-same-node", etc.
	TestTitle     string // "CIDR Ingress Policy Test", "Pod-to-Pod Same-Node Connectivity Test"
	LogStepName   string // "Deploying CIDR ingress policy", "Testing pod connectivity"
	LogStepFile   string // "Policy file: cidr-ingress-policy.yaml", "Test: pod connectivity"
	ExpectSuccess bool   // true/false for policy expectation

	// Enhanced formatting fields for consistent success/failure output
	ExpectedBehavior   string `json:"expected_behavior,omitempty"`   // 📋 Expected: description
	ResultConfirmation string `json:"result_confirmation,omitempty"` // 🎯 Result: description

	// NEW: Networking-specific configuration
	NetworkingConfig *NetworkingTestConfig `json:"networking_config,omitempty"`
}

// NetworkingTestConfig defines networking-specific test configuration
type NetworkingTestConfig struct {
	TestType       string            `json:"test_type"`       // "connectivity", "service", "dns"
	ResourcePrefix string            `json:"resource_prefix"` // "netshoot-test" (generated dynamically)
	ServiceType    string            `json:"service_type"`    // "ClusterIP", "NodePort", "LoadBalancer"
	PlacementType  string            `json:"placement_type"`  // "same-node", "cross-node", "both"
	RequiredNodes  int               `json:"required_nodes"`  // 1, 2 (for validation)
	Timeout        time.Duration     `json:"timeout"`         // 120s default
	ResourceNames  map[string]string `json:"resource_names"`  // Generated at runtime
}

// PolicyTestGroup represents a group of policy tests
type PolicyTestGroup struct {
	Name        string
	GroupId     string // l3-policies, l4-policies, l7-policies
	TestConfigs []PolicyTestConfig
}

// ExecutePolicyTest handles the common boilerplate for all policy tests
func ExecutePolicyTest(
	config PolicyTestConfig,
	logger *MultiChannelLogger,
	t *Tester,
	ctx context.Context,
	reuseResources bool,
	verbose bool,
	testNumber int,
	totalTests int,
) TestResult {
	startTime := time.Now()
	var details []string

	// Extract real policy name from YAML metadata - MANDATORY (no fallback to static names)
	policyName, err := t.ExtractPolicyNameFromFile(config.PolicyPath)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract policy name from %s: %v", config.PolicyPath, err),
		}
	}

	// Create fresh context for reliable test execution
	freshCtx := context.Background()
	testCtx, cancel := context.WithTimeout(freshCtx, 3*time.Minute)
	defer cancel()

	// Ensure policy cleanup after test completion - use fresh context for cleanup too
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := t.CleanupNetworkPolicy(cleanupCtx, policyName, config.PolicyPath); err != nil {
			logger.LogError("Failed to cleanup policy %s: %v", policyName, err)
		}
	}()

	// Start test with structured logging (suppress verbose headers in hierarchical mode)
	if verbose {
		logger.LogTestStart(config.TestTitle, testNumber, totalTests, config.GroupId)
	}

	// Policy deployment - visible in both verbose and non-verbose modes (no step numbers)
	logger.LogStepName(config.LogStepName, config.LogStepFile)

	result := t.TestNetworkPolicy(
		testCtx,
		policyName,
		config.PolicyPath,
		config.ExpectSuccess,
		&details,
		verbose,
		reuseResources,
	)

	// Calculate test duration for verbose reporting
	duration := time.Since(startTime)

	if result.Success {
		successMsg := fmt.Sprintf("%s deployed and tested successfully", config.TestTitle)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, true, successMsg)
	} else {
		// Enhanced verbose mode: Show detailed failure information using ACTUAL command history
		if verbose {
			// Get the REAL command executor used during TestNetworkPolicy
			executor := t.GetLastExecutor()
			if executor != nil {
				failurePoint := determineFailurePoint(config.TestId)
				errorDetails := ExtractErrorDetailsFromCommand(executor.GetCommandHistory(), fmt.Sprintf("%s policy validation", strings.ToUpper(config.GroupId[:2])))

				enhancedResult := CreateEnhancedTestResult(result, executor, failurePoint, errorDetails)
				verboseOutput := FormatVerboseTestResultForHierarchy(config.TestId, enhancedResult, duration.Seconds(), verbose)

				logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, verboseOutput)
			} else {
				logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, result.Message)
			}
		} else {
			logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, result.Message)
		}
	}

	// Complete test with logging and forced flush
	hierarchy := &HierarchyContext{
		GroupId:    config.GroupId,
		SubgroupId: config.SubgroupId,
		TestId:     config.TestId,
		Phase:      "execution",
	}

	// Use UNIFIED logging method with complete TestResult data (preserves all rich error information)
	logger.GetFrontendLogger().LogTestComplete(
		config.TestId,
		config.GroupId,
		config.SubgroupId,
		testNumber,
		totalTests,
		result.Success,
		duration.Seconds(),
		hierarchy,
		&result, // Pass complete TestResult - CRITICAL for error details, diagnostics, and failure information
	)

	return result
}

// ExecuteNetworkingTest handles networking tests using common framework with robust emergency cleanup
func ExecuteNetworkingTest(
	config PolicyTestConfig,
	logger *MultiChannelLogger,
	t *Tester,
	ctx context.Context,
	verbose bool,
	testNumber int,
	totalTests int,
) TestResult {
	startTime := time.Now()

	// Generate dynamic resource names
	if config.NetworkingConfig != nil {
		config.NetworkingConfig.ResourceNames = generateResourceNames(config.TestId)
	}

	// Create fresh context for reliable test execution
	freshCtx := context.Background()
	testCtx, cancel := context.WithTimeout(freshCtx, 5*time.Minute) // Longer timeout for networking tests
	defer cancel()

	// CRITICAL FIX: Add defer-based emergency cleanup like L3/L4/L7 tests
	defer func() {
		// Use fresh context for cleanup to ensure it completes even if test context is cancelled
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()

		if config.NetworkingConfig != nil && config.NetworkingConfig.ResourceNames != nil {
			logger.LogInfo("Emergency cleanup: Starting networking resource cleanup for test %s", config.TestId)
			performNetworkingEmergencyCleanup(cleanupCtx, config, logger, t)
		}
	}()

	// Use the tester's networking execution capability to run live tests
	var result TestResult
	if config.NetworkingConfig != nil {
		// Call the tester's method to execute networking tests with live K8s resources
		result = t.ExecuteNetworkingTestLive(testCtx, config, logger, verbose)
	} else {
		result = TestResult{
			Success: false,
			Message: "Invalid networking test configuration",
		}
	}

	// Calculate test duration
	duration := time.Since(startTime)

	// Use networking-specific frontend logger for consistent output
	subgroupName := config.SubgroupId
	if subgroupName == "" {
		// Determine subgroup based on test type
		switch config.NetworkingConfig.TestType {
		case "connectivity":
			subgroupName = "pod-connectivity"
		case "service":
			subgroupName = "services"
		case "dns":
			subgroupName = "dns"
		default:
			subgroupName = "networking"
		}
	}

	// Create hierarchy context for networking tests
	hierarchy := &HierarchyContext{
		GroupId:    "networking",
		SubgroupId: subgroupName,
		TestId:     config.TestId,
		Phase:      "execution",
	}

	// Use UNIFIED logging method with complete TestResult data (preserves all rich error information)
	logger.GetFrontendLogger().LogTestComplete(
		config.TestId,
		"networking",
		subgroupName,
		testNumber,
		totalTests,
		result.Success,
		duration.Seconds(),
		hierarchy,
		&result, // Pass complete TestResult - CRITICAL for networking error details, diagnostics, and failure information
	)

	// Return the basic result for backward compatibility
	// The enhanced result will be used by FormatEnhancedTestSummary
	return result
}

// generateResourceNames creates dynamic resource names for networking tests
func generateResourceNames(testId string) map[string]string {
	timestamp := time.Now().Unix()
	return map[string]string{
		"pod1":       fmt.Sprintf("%s-pod1-%d", testId, timestamp),
		"pod2":       fmt.Sprintf("%s-pod2-%d", testId, timestamp),
		"deployment": fmt.Sprintf("%s-deployment", testId),
		"service":    fmt.Sprintf("%s-service", testId),
		"testpod":    fmt.Sprintf("%s-testpod-%d", testId, timestamp),
	}
}

// generateNetworkingDisplayName creates proper display names for networking tests
func generateNetworkingDisplayName(testKey string) string {
	switch testKey {
	case "pod-to-pod-same-node":
		return "Pod-to-Pod Same-Node Connectivity"
	case "pod-to-pod-cross-node":
		return "Pod-to-Pod Cross-Node Connectivity"
	case "service-clusterip":
		return "Service ClusterIP Connectivity"
	case "service-nodeport":
		return "Service NodePort Connectivity"
	case "service-loadbalancer":
		return "Service LoadBalancer Connectivity"
	case "service-cross-node":
		return "Cross-Node Service Connectivity"
	case "dns-resolution":
		return "DNS Resolution"
	default:
		// Convert kebab-case to Title Case as fallback
		parts := strings.Split(testKey, "-")
		for i, part := range parts {
			parts[i] = strings.Title(part)
		}
		return strings.Join(parts, " ")
	}
}

// determineFailurePoint returns appropriate failure point based on test type
func determineFailurePoint(testId string) string {
	switch {
	case strings.Contains(testId, "http"):
		return "HTTP connectivity test"
	case strings.Contains(testId, "icmp"):
		return "ICMP connectivity test"
	case strings.Contains(testId, "sni") || strings.Contains(testId, "tls"):
		return "TLS/SNI connectivity test"
	case strings.Contains(testId, "dns"):
		return "DNS connectivity test"
	default:
		return "HTTP connectivity test"
	}
}

// determineNetworkingFailurePoint returns appropriate failure point based on networking test type
func determineNetworkingFailurePoint(testId string, errorMessage string) string {
	switch {
	case strings.Contains(errorMessage, "timeout"):
		if strings.Contains(testId, "pod-to-pod") {
			return "Pod connectivity test timeout"
		}
		return "Networking timeout"
	case strings.Contains(errorMessage, "failed to create"):
		return "Resource creation failure"
	case strings.Contains(errorMessage, "Cilium"):
		return "CNI health check failure"
	case strings.Contains(errorMessage, "worker nodes"):
		return "Node availability check"
	case strings.Contains(testId, "service") && strings.Contains(errorMessage, "deployment"):
		return "Service deployment failure"
	case strings.Contains(testId, "dns"):
		return "DNS resolution failure"
	default:
		return "Network connectivity test failure"
	}
}

// validateL4DataCapture validates L4-specific connectivity data to ensure real data capture is working
func validateL4DataCapture(testId string, results []ConnectivityResult) error {
	if len(results) == 0 {
		return fmt.Errorf("No connectivity data captured for L4 test %s", testId)
	}

	// L4-specific validation based on test type
	switch {
	case strings.Contains(testId, "port"):
		return validatePortConnectivity(testId, results)
	case strings.Contains(testId, "icmp"):
		return validateICMPConnectivity(testId, results)
	case strings.Contains(testId, "sni") || strings.Contains(testId, "tls"):
		return validateTLSConnectivity(testId, results)
	default:
		// Generic validation for other L4 tests
		return validateGenericL4Connectivity(testId, results)
	}
}

// validatePortConnectivity validates port-specific connectivity data
func validatePortConnectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Check for real port connectivity
		if result.Protocol != "HTTP" && result.Protocol != "TCP" {
			return fmt.Errorf("Port test %s should have HTTP/TCP protocol, got: %s", testId, result.Protocol)
		}

		// Check for real status codes (not fake ones)
		if result.StatusCode != "" && result.StatusCode != "200" && result.StatusCode != "403" && result.StatusCode != "timeout" {
			// Valid status codes include 200 (success), 403 (blocked), or timeout
			return fmt.Errorf("Port test %s has unexpected status code: %s", testId, result.StatusCode)
		}

		// Check for reasonable timing (real tests should have some duration)
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("Port test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateICMPConnectivity validates ICMP-specific connectivity data
func validateICMPConnectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Check for ICMP protocol
		if result.Protocol != "ICMP" && result.Protocol != "ICMPv6" {
			return fmt.Errorf("ICMP test %s should have ICMP/ICMPv6 protocol, got: %s", testId, result.Protocol)
		}

		// ICMP tests should have different status patterns
		if result.StatusCode != "" && result.StatusCode != "ping_success" && result.StatusCode != "ping_blocked" && result.StatusCode != "timeout" {
			return fmt.Errorf("ICMP test %s has unexpected status code: %s", testId, result.StatusCode)
		}

		// Check for reasonable ping timing
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("ICMP test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateTLSConnectivity validates TLS/SNI-specific connectivity data
func validateTLSConnectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Check for HTTPS/TLS protocol
		if result.Protocol != "HTTPS" && result.Protocol != "TLS" {
			return fmt.Errorf("TLS test %s should have HTTPS/TLS protocol, got: %s", testId, result.Protocol)
		}

		// TLS tests should have certificate-related status codes
		if result.StatusCode != "" && result.StatusCode != "200" && result.StatusCode != "cert_blocked" && result.StatusCode != "sni_blocked" && result.StatusCode != "timeout" {
			return fmt.Errorf("TLS test %s has unexpected status code: %s", testId, result.StatusCode)
		}

		// Check for reasonable TLS handshake timing
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("TLS test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateGenericL4Connectivity provides generic L4 validation for other test types
func validateGenericL4Connectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Generic check for reasonable connectivity data
		if result.Source == "" || result.Target == "" {
			return fmt.Errorf("L4 test %s should have non-empty source and target", testId)
		}

		// Check for real timing on successful tests
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("L4 test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateL7DataCapture validates L7-specific connectivity data to ensure real data capture is working
func validateL7DataCapture(testId string, results []ConnectivityResult) error {
	if len(results) == 0 {
		return fmt.Errorf("No connectivity data captured for L7 test %s", testId)
	}

	// L7-specific validation based on test type
	switch {
	case strings.Contains(testId, "http"):
		return validateHTTPConnectivity(testId, results)
	case strings.Contains(testId, "dns"):
		return validateDNSConnectivity(testId, results)
	default:
		// Generic validation for other L7 tests
		return validateGenericL7Connectivity(testId, results)
	}
}

// validateHTTPConnectivity validates HTTP-specific connectivity data
func validateHTTPConnectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Check for HTTP protocol
		if result.Protocol != "HTTP" && result.Protocol != "HTTPS" {
			return fmt.Errorf("HTTP test %s should have HTTP/HTTPS protocol, got: %s", testId, result.Protocol)
		}

		// Check for real HTTP status codes
		if result.StatusCode != "" && result.StatusCode != "200" && result.StatusCode != "404" && result.StatusCode != "403" && result.StatusCode != "blocked" && result.StatusCode != "timeout" {
			return fmt.Errorf("HTTP test %s has unexpected status code: %s", testId, result.StatusCode)
		}

		// Check for reasonable timing
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("HTTP test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateDNSConnectivity validates DNS-specific connectivity data
func validateDNSConnectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Check for DNS protocol
		if result.Protocol != "DNS" && result.Protocol != "UDP" {
			return fmt.Errorf("DNS test %s should have DNS/UDP protocol, got: %s", testId, result.Protocol)
		}

		// Check for reasonable timing
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("DNS test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// validateGenericL7Connectivity provides generic L7 validation for other test types
func validateGenericL7Connectivity(testId string, results []ConnectivityResult) error {
	for _, result := range results {
		// Generic check for reasonable connectivity data
		if result.Source == "" || result.Target == "" {
			return fmt.Errorf("L7 test %s should have non-empty source and target", testId)
		}

		// Check for real timing on successful tests
		if result.Success && result.Duration == 0 {
			return fmt.Errorf("L7 test %s successful result should have non-zero duration", testId)
		}
	}
	return nil
}

// ExecutePolicyTestGroups runs policy test groups with common timing and cleanup logic
func ExecutePolicyTestGroups(
	groups []PolicyTestGroup,
	t *Tester,
	ctx context.Context,
	verbose bool,
	connectivityDataCapture func(context.Context, string) []ConnectivityResult,
) ([]TimedTestResult, []string) {
	var timedResults []TimedTestResult
	var testNames []string

	// Use command logger singleton for JSONL logging
	logger := GetGlobalMultiChannelLogger()
	if logger == nil {
		fmt.Printf("ERROR: Failed to get global multi-channel logger\n")
		return timedResults, testNames
	}

	// Calculate total number of tests
	totalTests := 0
	for _, group := range groups {
		totalTests += len(group.TestConfigs)
	}

	currentTestNumber := 1

	// Execute subgroups with clean tree structure
	for i, group := range groups {
		// Print testing phase header before each subgroup
		fmt.Println("\n🧪 TESTING PHASE")

		// Determine tree connector
		groupConnector := "├──"
		if i == len(groups)-1 {
			groupConnector = "└──"
		}

		// Print group header with special handling for networking tests
		if group.GroupId == "networking" {
			fmt.Printf("%s Group: %s (%d tests)\n", groupConnector, strings.ToUpper(group.Name), len(group.TestConfigs))
		} else {
			fmt.Printf("%s %s Subgroup: %s (%d tests)\n", groupConnector, strings.ToUpper(group.GroupId[:2]), strings.ToUpper(group.Name), len(group.TestConfigs))
		}

		// Create group-specific context with timeout
		groupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		// Run tests in this group
		for j, config := range group.TestConfigs {
			// Determine test tree connector
			testConnector := "│   ├──"
			if i == len(groups)-1 {
				testConnector = "    ├──"
			}
			if j == len(group.TestConfigs)-1 {
				if i == len(groups)-1 {
					testConnector = "    └──"
				} else {
					testConnector = "│   └──"
				}
			}

			// Track test execution time
			testStart := time.Now()

			// Generate display name for networking tests vs policy tests
			displayName := config.TestId
			if config.GroupId == "networking" {
				displayName = generateNetworkingDisplayName(config.TestId)
			}

			// Print test start
			fmt.Printf("%s (%d/%d) %s: Testing policies... ", testConnector, currentTestNumber, totalTests, displayName)

			// Execute the test using appropriate framework based on group type
			var result TestResult
			if config.GroupId == "networking" {
				// Use networking test framework for networking tests
				result = ExecuteNetworkingTest(config, logger, t, groupCtx, verbose, currentTestNumber, totalTests)
			} else {
				// Use policy test framework for policy tests
				result = ExecutePolicyTest(config, logger, t, groupCtx, false, verbose, currentTestNumber, totalTests)
			}
			testEnd := time.Now()
			testElapsed := ElapsedSeconds(testStart)
			currentTestNumber++

			if result.Success {
				// Capture REAL connectivity data during policy tests
				connectivityResults := connectivityDataCapture(groupCtx, config.TestId)

				// If no real results captured, show empty results rather than fake data
				if len(connectivityResults) == 0 {
					connectivityResults = []ConnectivityResult{} // Empty - no fake data
				}

				// For L4 tests, validate the connectivity data to ensure real data capture is working
				if config.GroupId == "l4-policies" && len(connectivityResults) > 0 {
					if validationErr := validateL4DataCapture(config.TestId, connectivityResults); validationErr != nil {
						// Log validation warning but don't fail the test (connectivity data quality check)
						if verbose {
							fmt.Printf("\n⚠️  L4 Data Validation Warning for %s: %v\n", config.TestId, validationErr)
						}
					}
				}

				// For L7 tests, validate the connectivity data to ensure real data capture is working
				if config.GroupId == "l7-policies" && len(connectivityResults) > 0 {
					if validationErr := validateL7DataCapture(config.TestId, connectivityResults); validationErr != nil {
						// Log validation warning but don't fail the test (connectivity data quality check)
						if verbose {
							fmt.Printf("\n⚠️  L7 Data Validation Warning for %s: %v\n", config.TestId, validationErr)
						}
					}
				}

				// Create enhanced test result with real connectivity data (or empty if none captured)
				enhancedResult := CreateEnhancedTestResultWithExpectation(result, config.TestId+"-policy", connectivityResults, nil)
				formattedResult := FormatEnhancedTestResultForHierarchy(config.TestId, enhancedResult, testElapsed, verbose)
				fmt.Print(formattedResult)
			} else {
				// For failed tests, print newline first to break from tree structure
				fmt.Print("\n")

				// For failed tests in verbose mode, get real command history and create proper enhanced result
				if verbose {
					executor := t.GetLastExecutor()
					if executor != nil {
						failurePoint := determineFailurePoint(config.TestId)
						errorDetails := ExtractErrorDetailsFromCommand(executor.GetCommandHistory(), fmt.Sprintf("%s policy validation", strings.ToUpper(config.GroupId[:2])))
						enhancedResult := CreateEnhancedTestResult(result, executor, failurePoint, errorDetails)

						// Use the verbose failure formatting that shows command history
						formattedResult := FormatVerboseTestFailure(config.TestId, testElapsed, enhancedResult.ExecutedCommands, enhancedResult.FailurePoint, enhancedResult.ErrorDetails)
						fmt.Print(formattedResult)
					} else {
						// Fallback if no executor available
						fmt.Printf("❌ %s FAILED (%.1fs)", config.TestId, testElapsed)
					}
				} else {
					// Non-verbose mode: simple failure message
					fmt.Printf("❌ %s FAILED (%.1fs)", config.TestId, testElapsed)
				}
			}

			// Add visual separator line after every test completion
			fmt.Println("\n────────────────────────────────────────────────────────")

			// Create TimedTestResult
			timedResult := TimedTestResult{
				TestResult: result,
				StartTime:  testStart,
				EndTime:    testEnd,
			}

			// Store results
			timedResults = append(timedResults, timedResult)
			testNames = append(testNames, config.TestId)

			// Brief cooling period between tests
			if j < len(group.TestConfigs)-1 {
				time.Sleep(2 * time.Second)
			}
		}

		// Cooling period between groups
		if i < len(groups)-1 {
			time.Sleep(3 * time.Second)
		}
	}

	return timedResults, testNames
}

// Dynamic test context mapping for consistent expected behavior descriptions
var testContextMap = map[string]string{
	"service-clusterip":     "Service connectivity via ClusterIP (internal cluster access)",
	"service-nodeport":      "Service connectivity via NodePort (external node access)",
	"service-loadbalancer":  "Service connectivity via LoadBalancer (external load balancer)",
	"dns-resolution":        "DNS resolution for Kubernetes services",
	"pod-to-pod-same-node":  "Pod-to-pod connectivity on same node (CNI networking validation)",
	"pod-to-pod-cross-node": "Pod-to-pod connectivity across nodes (CNI cross-node networking)",
	"service-cross-node":    "Cross-node service connectivity validation",
	"cilium-connectivity":   "Cilium CNI connectivity validation",
	"network-policy-l3":     "L3 network policy enforcement validation",
	"network-policy-l4":     "L4 network policy enforcement validation",
	"network-policy-l7":     "L7 network policy enforcement validation",
	"cidr-ingress":          "CIDR-based ingress policy enforcement",
	"cidr-egress":           "CIDR-based egress policy enforcement",
	"port-ingress":          "Port-based ingress policy enforcement",
	"port-egress":           "Port-based egress policy enforcement",
	"http-policy":           "HTTP-level policy enforcement",
	"dns-policy":            "DNS-level policy enforcement",
	"tls-sni-policy":        "TLS SNI-based policy enforcement",
	"icmp-policy":           "ICMP-based policy enforcement",
}

// getExpectedBehaviorForTest returns the expected behavior description for a test using dynamic mapping
func getExpectedBehaviorForTest(testId string, networkingConfig *NetworkingTestConfig) string {
	// First check the explicit test context map
	if expectedBehavior, exists := testContextMap[testId]; exists {
		return expectedBehavior
	}

	// Check for partial matches with test ID patterns
	for pattern, behavior := range testContextMap {
		if strings.Contains(testId, pattern) {
			return behavior
		}
	}

	// Dynamic behavior based on networking configuration
	if networkingConfig != nil {
		switch networkingConfig.TestType {
		case "service":
			serviceType := networkingConfig.ServiceType
			if serviceType == "" {
				serviceType = "ClusterIP"
			}
			return fmt.Sprintf("Service connectivity via %s (%s access)",
				serviceType, getServiceAccessType(serviceType))
		case "dns":
			return "DNS resolution for Kubernetes services"
		case "connectivity":
			placementType := networkingConfig.PlacementType
			if placementType == "" {
				placementType = "both"
			}
			return fmt.Sprintf("Pod-to-pod connectivity validation (%s placement)", placementType)
		}
	}

	// Check for policy-related patterns
	if strings.Contains(testId, "policy") {
		if strings.Contains(testId, "l3") || strings.Contains(testId, "cidr") || strings.Contains(testId, "ip") {
			return "L3 network policy enforcement validation"
		} else if strings.Contains(testId, "l4") || strings.Contains(testId, "port") || strings.Contains(testId, "tcp") || strings.Contains(testId, "udp") {
			return "L4 network policy enforcement validation"
		} else if strings.Contains(testId, "l7") || strings.Contains(testId, "http") || strings.Contains(testId, "dns") {
			return "L7 network policy enforcement validation"
		}
		return "Network policy enforcement validation"
	}

	// Fallback for unknown test types
	return "Network connectivity test"
}

// getServiceAccessType returns the access type description for a service type
func getServiceAccessType(serviceType string) string {
	switch strings.ToLower(serviceType) {
	case "clusterip":
		return "internal cluster"
	case "nodeport":
		return "external node"
	case "loadbalancer":
		return "external load balancer"
	default:
		return "cluster"
	}
}

// formatEnhancedFailure formats the enhanced failure output with diagnostic information
func formatEnhancedFailure(expectedBehavior, failureReason, failedCommand, commandOutput string) string {
	return fmt.Sprintf("❌ FAIL\n   📋 Expected: %s\n   💥 Failure: %s\n   🔍 Command: %s\n   📟 Output: %s",
		expectedBehavior, failureReason, failedCommand, commandOutput)
}

// evaluateHTTPStatusCode evaluates HTTP status codes and returns success status with message
func evaluateHTTPStatusCode(statusCode string) (bool, string) {
	// Handle empty status code
	if statusCode == "" {
		return false, "Empty status code"
	}

	// Try to parse as integer
	if code, err := strconv.Atoi(statusCode); err == nil {
		if code >= 200 && code < 300 {
			return true, fmt.Sprintf("Success (%d)", code)
		}
		return false, fmt.Sprintf("HTTP error (%d)", code)
	}

	// Handle non-numeric status codes (like "timeout", "connection_refused", etc.)
	switch strings.ToLower(statusCode) {
	case "200", "ok", "success":
		return true, "Success (200)"
	case "timeout", "connection_timeout":
		return false, "Connection timeout"
	case "connection_refused", "refused":
		return false, "Connection refused"
	case "dns_error", "name_resolution_failed":
		return false, "DNS resolution failed"
	default:
		return false, fmt.Sprintf("Invalid or unknown status: %s", statusCode)
	}
}

// performNetworkingEmergencyCleanup performs comprehensive cleanup of networking resources
func performNetworkingEmergencyCleanup(ctx context.Context, config PolicyTestConfig, logger *MultiChannelLogger, t *Tester) {
	logger.LogInfo("Emergency cleanup: Starting networking resource cleanup for test %s", config.TestId)

	if config.NetworkingConfig == nil || config.NetworkingConfig.ResourceNames == nil {
		logger.LogInfo("Emergency cleanup: No resource names to cleanup")
		return
	}

	resourceNames := config.NetworkingConfig.ResourceNames
	cleanupErrors := []string{}

	// 1. Pod cleanup - most critical
	if pod1Name, exists := resourceNames["pod1"]; exists && pod1Name != "" {
		t.CleanupPod(ctx, pod1Name)
		logger.LogInfo("Emergency cleanup: Cleaned up pod %s", pod1Name)
	}

	if pod2Name, exists := resourceNames["pod2"]; exists && pod2Name != "" {
		t.CleanupPod(ctx, pod2Name)
		logger.LogInfo("Emergency cleanup: Cleaned up pod %s", pod2Name)
	}

	if testPodName, exists := resourceNames["testpod"]; exists && testPodName != "" {
		t.CleanupPod(ctx, testPodName)
		logger.LogInfo("Emergency cleanup: Cleaned up testpod %s", testPodName)
	}

	// 2. Service cleanup
	if serviceName, exists := resourceNames["service"]; exists && serviceName != "" {
		executor := NewCommandExecutor(logger, "diagnostic-test", false)
		_, err := executor.ExecuteKubectlCommand(ctx, "delete", "service", serviceName, "--ignore-not-found=true")
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("service %s: %v", serviceName, err))
		} else {
			logger.LogInfo("Emergency cleanup: Successfully cleaned up service %s", serviceName)
		}
	}

	// 3. Deployment cleanup
	if deploymentName, exists := resourceNames["deployment"]; exists && deploymentName != "" {
		executor := NewCommandExecutor(logger, "diagnostic-test", false)
		_, err := executor.ExecuteKubectlCommand(ctx, "delete", "deployment", deploymentName, "--ignore-not-found=true")
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("deployment %s: %v", deploymentName, err))
		} else {
			logger.LogInfo("Emergency cleanup: Successfully cleaned up deployment %s", deploymentName)
		}
	}

	// 4. Clean up any DNS test pods (dynamic names)
	if config.NetworkingConfig.TestType == "dns" {
		executor := NewCommandExecutor(logger, "diagnostic-test", false)
		_, err := executor.ExecuteKubectlCommand(ctx, "delete", "pod", "--ignore-not-found=true", "-l", "app=dns-test")
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("DNS test pods: %v", err))
		} else {
			logger.LogInfo("Emergency cleanup: Successfully cleaned up DNS test pods")
		}
	}

	// 5. Final summary
	if len(cleanupErrors) == 0 {
		logger.LogInfo("Emergency cleanup: All networking resources cleaned up successfully for test %s", config.TestId)
	} else {
		logger.LogError("Emergency cleanup: Some resources failed to cleanup for test %s: %s", config.TestId, strings.Join(cleanupErrors, ", "))
	}
}

// Networking test execution will be handled by the networking package implementations
// These are removed from core since they should be in networking/tests.go
