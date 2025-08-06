package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	reuseResources bool,
	verbose bool,
	testNumber int,
	totalTests int,
) core.TestResult {
	startTime := time.Now()
	var details []string

	// Extract real policy name from YAML metadata - MANDATORY (no fallback to static names)
	policyName, err := t.ExtractPolicyNameFromFile(config.PolicyPath)
	if err != nil {
		return core.TestResult{
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
				errorDetails := core.ExtractErrorDetailsFromCommand(executor.GetCommandHistory(), fmt.Sprintf("%s policy validation", strings.ToUpper(config.GroupId[:2])))

				enhancedResult := core.CreateEnhancedTestResult(result, executor, failurePoint, errorDetails)
				verboseOutput := core.FormatVerboseTestResultForHierarchy(config.TestId, enhancedResult, duration.Seconds(), verbose)

				logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, verboseOutput)
			} else {
				logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, result.Message)
			}
		} else {
			logger.GetFrontendLogger().LogStepCompleteWithForcedFlush(config.LogStepName, false, result.Message)
		}
	}

	// Complete test with logging and forced flush
	hierarchy := &core.HierarchyContext{
		GroupId:    config.GroupId,
		SubgroupId: config.SubgroupId,
		TestId:     config.TestId,
		Phase:      "execution",
	}

	// Use appropriate logging method based on group
	switch config.GroupId {
	case "l3-policies":
		logger.GetFrontendLogger().LogL3TestComplete(config.TestId, config.SubgroupId, testNumber, totalTests, result.Success, duration.Seconds(), hierarchy)
	case "l4-policies":
		logger.GetFrontendLogger().LogL4TestComplete(config.TestId, config.SubgroupId, testNumber, totalTests, result.Success, duration.Seconds(), hierarchy)
	case "l7-policies":
		logger.GetFrontendLogger().LogL7TestComplete(config.TestId, config.SubgroupId, testNumber, totalTests, result.Success, duration.Seconds(), hierarchy)
	}

	return result
}

// ExecuteNetworkingTest handles networking tests using common framework with robust emergency cleanup
func ExecuteNetworkingTest(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	testNumber int,
	totalTests int,
) core.TestResult {
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

	// Route to appropriate networking test handler with fresh context
	var result core.TestResult
	if config.NetworkingConfig != nil {
		switch config.NetworkingConfig.TestType {
		case "connectivity":
			result = executeConnectivityTest(config, logger, t, testCtx, verbose)
		case "service":
			result = executeServiceTest(config, logger, t, testCtx, verbose)
		case "dns":
			result = executeDNSTest(config, logger, t, testCtx, verbose)
		default:
			result = core.TestResult{
				Success: false,
				Message: "Invalid networking test configuration",
			}
		}
	} else {
		result = core.TestResult{
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
	hierarchy := &core.HierarchyContext{
		GroupId:    "networking",
		SubgroupId: subgroupName,
		TestId:     config.TestId,
		Phase:      "execution",
	}

	// Use direct frontend logging instead of fmt.Printf for consistent output
	logger.GetFrontendLogger().LogNetworkingTestComplete(
		config.TestId,
		subgroupName,
		testNumber,
		totalTests,
		result.Success,
		duration.Seconds(),
		hierarchy,
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

// executeConnectivityTest handles pod-to-pod connectivity tests and returns enhanced results
func executeConnectivityTest(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string
	var connectivityResults []core.ConnectivityResult

	// Start test with structured logging
	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Check Cilium CNI status
	logger.LogStepName("Checking Cilium CNI status", "Querying Cilium pods in kube-system namespace...")

	ciliumStatus, ciliumIssue := checkCiliumStatus(logger, t, ctx)
	if !ciliumStatus {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking Cilium CNI status", false, ciliumIssue)
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Pod-to-pod connectivity test failed - Cilium CNI issues detected")
		return core.TestResult{
			Success: false,
			Message: "Pod-to-pod connectivity test failed - Cilium CNI issues detected",
			Details: append(details, fmt.Sprintf("✗ Cilium CNI health check failed: %s", ciliumIssue)),
		}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking Cilium CNI status", true, "All Cilium pods running and ready")

	// Execute connectivity tests based on placement strategy
	var result core.TestResult
	placement := config.NetworkingConfig.PlacementType
	if placement == "" {
		placement = "both"
	}

	switch placement {
	case "same-node":
		result, connectivityResults = executeSameNodeConnectivityTestEnhanced(t, ctx, logger, config)
	case "cross-node":
		result, connectivityResults = executeCrossNodeConnectivityTestEnhanced(t, ctx, logger, config)
	case "both":
		result, connectivityResults = executeBothPlacementConnectivityTestsEnhanced(t, ctx, logger, config)
	default:
		result, connectivityResults = executeBothPlacementConnectivityTestsEnhanced(t, ctx, logger, config)
	}

	// Create enhanced result with connectivity data
	enhancedResult := core.CreateEnhancedTestResultWithExpectation(
		result,
		config.TestId, // Use testId directly since networking tests don't need "-policy" suffix
		connectivityResults,
		t.GetLastExecutor(),
	)

	// Store enhanced result data in the basic result for FormatEnhancedTestSummary to use
	// This is a workaround to pass enhanced data through the existing TestResult structure
	result.Details = append(result.Details, fmt.Sprintf("ENHANCED_DATA:%s|%s", enhancedResult.ExpectedOutcome, enhancedResult.ReceivedOutcome))

	// Complete test with final result
	duration := time.Since(startTime)
	logger.LogTestComplete(config.TestTitle, 1, 1, result.Success, duration.Seconds(), result.Message)
	return result
}

// executeServiceTest handles service connectivity tests by routing to specific implementations
func executeServiceTest(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	// Route to appropriate service test implementation based on service type and placement
	serviceType := config.NetworkingConfig.ServiceType
	placementType := config.NetworkingConfig.PlacementType

	// Use the new service test router approach
	return executeServiceTestWithRouter(config, logger, t, ctx, verbose, serviceType, placementType)
}

// executeServiceTestWithRouter routes to the correct service test implementation
func executeServiceTestWithRouter(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	serviceType string,
	placementType string,
) core.TestResult {
	startTime := time.Now()

	// Start test with structured logging
	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Create command executor for service tests with proper error handling
	executor := core.NewCommandExecutor(logger, "service-diagnostic", verbose)

	// Ensure resource names are generated
	if config.NetworkingConfig != nil && config.NetworkingConfig.ResourceNames == nil {
		config.NetworkingConfig.ResourceNames = generateResourceNames(config.TestId)
	}

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	// Emergency cleanup defer (like L3/L4/L7 tests have)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		logger.LogInfo("Service test cleanup: Starting resource cleanup")

		// Cleanup service resources
		if executor != nil {
			executor.ExecuteKubectlCommand(cleanupCtx, "delete", "deployment", deploymentName, "--ignore-not-found=true")
			executor.ExecuteKubectlCommand(cleanupCtx, "delete", "service", serviceName, "--ignore-not-found=true")
			executor.ExecuteKubectlCommand(cleanupCtx, "delete", "pod", testPodName, "--ignore-not-found=true")
		}
		logger.LogInfo("Service test cleanup: Resource cleanup completed")
	}()

	// Route to specific service test implementation
	var result core.TestResult
	switch serviceType {
	case "ClusterIP":
		if placementType == "cross-node" {
			result = executeCrossNodeServiceTestInternal(config, logger, t, ctx, verbose, executor)
		} else {
			result = executeClusterIPServiceTestInternal(config, logger, t, ctx, verbose, executor)
		}
	case "NodePort":
		result = executeNodePortServiceTestInternal(config, logger, t, ctx, verbose, executor)
	case "LoadBalancer":
		result = executeLoadBalancerServiceTestInternal(config, logger, t, ctx, verbose, executor)
	default:
		// Default to ClusterIP if service type not specified
		result = executeClusterIPServiceTestInternal(config, logger, t, ctx, verbose, executor)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Complete test logging
	if verbose {
		logger.LogTestComplete(config.TestTitle, 1, 1, result.Success, duration.Seconds(), result.Message)
	}

	return result
}

// =============================================================================
// INTERNAL SERVICE TEST IMPLEMENTATIONS (PHASE 2 INTEGRATION)
// =============================================================================

// executeClusterIPServiceTestInternal implements ClusterIP service testing using common framework patterns
func executeClusterIPServiceTestInternal(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	executor *core.CommandExecutor,
) core.TestResult {
	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	// Step 1: Create nginx deployment
	logger.LogStepName("Creating deployment", fmt.Sprintf("Deploying %s with nginx image", deploymentName))

	_, err := t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create nginx deployment: %v", err)}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err)}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", true, fmt.Sprintf("Deployment %s created and ready", deploymentName))

	// Step 2: Create ClusterIP service
	logger.LogStepName("Creating service", fmt.Sprintf("Creating ClusterIP service %s", serviceName))

	_, err = t.CreateNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", false, fmt.Sprintf("Failed to create service: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create service: %v", err)}
	}

	serviceIP, err := t.GetServiceIP(ctx, serviceName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", false, fmt.Sprintf("Failed to get service IP: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get service IP: %v", err)}
	}
	logger.LogInfo("ClusterIP service IP: %s", serviceIP)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", true, fmt.Sprintf("ClusterIP service %s created", serviceName))

	// Step 3: Create test pod
	logger.LogStepName("Creating test pod", fmt.Sprintf("Creating netshoot test pod: %s", testPodName))

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create test pod: %v", err)}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err)}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, fmt.Sprintf("Test pod %s created and ready", testPodName))

	// Step 4: Test HTTP connectivity
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity to ClusterIP service %s", serviceName))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		return core.TestResult{Success: false, Message: "ClusterIP HTTP connectivity failed"}
	}

	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ ClusterIP HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("ClusterIP connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("WARNING: ClusterIP HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("ClusterIP HTTP connectivity issue - %s", message))
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("ClusterIP service connectivity test - Status: %s", message),
	}
}

// executeCrossNodeServiceTestInternal implements cross-node service testing
func executeCrossNodeServiceTestInternal(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	executor *core.CommandExecutor,
) core.TestResult {
	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 2 worker nodes for cross-node testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	if len(workerNodes) < 2 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 2 worker nodes, found %d", len(workerNodes)))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Cross-node service test requires at least 2 worker nodes, found %d", len(workerNodes))}
	}
	logger.LogInfo("Found %d worker nodes: %s", len(workerNodes), strings.Join(workerNodes, ", "))
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for cross-node testing", len(workerNodes)))

	// Step 2: Setup service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and service for cross-node testing")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create nginx deployment: %v", err)}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err)}
	}

	_, err = t.CreateNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create service: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create service: %v", err)}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", true, "Cross-node service environment ready")

	// Step 3: Create test pod on different node
	logger.LogStepName("Creating test pod", fmt.Sprintf("Creating netshoot pod on node %s for cross-node testing", workerNodes[1]))

	_, err = t.CreateNetshootPod(ctx, testPodName, workerNodes[1])
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod on node %s: %v", workerNodes[1], err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create test pod on node %s: %v", workerNodes[1], err)}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err)}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, "Cross-node test pod created and ready")

	// Step 4: Test connectivity
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity from test pod to service %s", serviceName))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		return core.TestResult{Success: false, Message: "Cross-node service HTTP connectivity failed"}
	}

	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ Cross-node HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("Cross-node service connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ Cross-node HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("Cross-node HTTP connectivity issue - %s", message))
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("Cross-node service connectivity test - Status: %s", message),
	}
}

// executeNodePortServiceTestInternal implements NodePort service testing
func executeNodePortServiceTestInternal(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	executor *core.CommandExecutor,
) core.TestResult {
	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 1 worker node for NodePort testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	if len(workerNodes) < 1 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 1 worker node, found %d", len(workerNodes)))
		return core.TestResult{Success: false, Message: fmt.Sprintf("NodePort test requires at least 1 worker node, found %d", len(workerNodes))}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for NodePort testing", len(workerNodes)))

	// Step 2: Setup NodePort service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and NodePort service")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create nginx deployment: %v", err)}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err)}
	}

	createdService, err := t.CreateNginxServiceWithType(ctx, serviceName, deploymentName, core.ServiceTypeNodePort)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create NodePort service: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create NodePort service: %v", err)}
	}

	nodePort := int(createdService.Spec.Ports[0].NodePort)
	logger.LogInfo("NodePort assigned: %d", nodePort)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", true, "NodePort service environment ready")

	// Step 3: Get node IP and create test pod
	logger.LogStepName("Setting up test environment", fmt.Sprintf("Getting node IP address for NodePort %d access", nodePort))

	node, err := t.GetClientset().CoreV1().Nodes().Get(ctx, workerNodes[0], metav1.GetOptions{})
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Failed to get node information: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get node information: %v", err)}
	}

	var nodeIP string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			nodeIP = address.Address
			break
		}
	}

	if nodeIP == "" {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, "Could not determine node IP address")
		return core.TestResult{Success: false, Message: "Could not determine node IP address"}
	}

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Failed to create test pod: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create test pod: %v", err)}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Test pod did not become ready: %v", err)}
	}

	logger.LogInfo("Node IP for NodePort access: %s", nodeIP)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", true, "NodePort test environment configured")

	// Step 4: Test NodePort connectivity
	nodePortURL := fmt.Sprintf("%s:%d", nodeIP, nodePort)
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity to NodePort %s", nodePortURL))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, nodePortURL)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity to NodePort failed: %v", err))
		return core.TestResult{Success: false, Message: "NodePort HTTP connectivity failed"}
	}

	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ NodePort HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("NodePort connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ NodePort HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("NodePort HTTP connectivity issue - %s", message))
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("NodePort service connectivity test - Status: %s", message),
	}
}

// executeLoadBalancerServiceTestInternal implements LoadBalancer service testing
func executeLoadBalancerServiceTestInternal(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	executor *core.CommandExecutor,
) core.TestResult {
	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 1 worker node for LoadBalancer testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	if len(workerNodes) < 1 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 1 worker node, found %d", len(workerNodes)))
		return core.TestResult{Success: false, Message: fmt.Sprintf("LoadBalancer test requires at least 1 worker node, found %d", len(workerNodes))}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for LoadBalancer testing", len(workerNodes)))

	// Step 2: Setup LoadBalancer service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and LoadBalancer service")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create nginx deployment: %v", err)}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err)}
	}

	createdService, err := t.CreateNginxServiceWithType(ctx, serviceName, deploymentName, core.ServiceTypeLoadBalancer)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create LoadBalancer service: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create LoadBalancer service: %v", err)}
	}

	clusterIP := createdService.Spec.ClusterIP
	logger.LogInfo("LoadBalancer service ClusterIP: %s", clusterIP)
	logger.LogInfo("Note: In cloud environments, the service would be assigned an external IP")

	if len(createdService.Status.LoadBalancer.Ingress) > 0 {
		externalIP := createdService.Status.LoadBalancer.Ingress[0].IP
		if externalIP != "" {
			logger.LogInfo("External IP assigned: %s", externalIP)
		}
	} else {
		logger.LogInfo("No external IP assigned (expected in local environments)")
	}

	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", true, "LoadBalancer service environment ready")

	// Step 3: Create and setup test pod
	logger.LogStepName("Creating test pod", "Creating netshoot pod for LoadBalancer connectivity testing")

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create test pod: %v", err)}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		return core.TestResult{Success: false, Message: fmt.Sprintf("Test pod did not become ready: %v", err)}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, "LoadBalancer test pod created and ready")

	// Step 4: Test LoadBalancer connectivity
	logger.LogStepName("Testing connectivity", "Testing HTTP connectivity via ClusterIP (fallback for local environments)")

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		return core.TestResult{Success: false, Message: "LoadBalancer HTTP connectivity failed"}
	}

	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ LoadBalancer HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("LoadBalancer connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ LoadBalancer HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("LoadBalancer HTTP connectivity issue - %s", message))
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("LoadBalancer service connectivity test - Status: %s", message),
	}
}

// executeDNSTest handles DNS resolution tests with real nslookup commands
func executeDNSTest(
	config PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	// Create command executor for real command execution and verbose logging
	executor := core.NewCommandExecutor(logger, "diagnostic-test", verbose)

	// Start test with structured logging
	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	testPodName := fmt.Sprintf("dns-test-pod-%d", time.Now().Unix())
	serviceName := "kubernetes" // Test resolution of the kubernetes service

	// Step 1: Create DNS test pod using corrected retry logic
	logger.LogStepName("Creating DNS test pod", fmt.Sprintf("Creating netshoot test pod: %s", testPodName))

	// Step 1: Create DNS pod ONCE (no retries for creation itself)
	logger.LogInfo("Creating DNS test pod: %s", testPodName)
	_, err := executor.ExecuteKubectlCommand(ctx, "run", testPodName, "--image=nicolaka/netshoot",
		"--restart=Never", "--command", "--", "sleep", "3600")
	if err != nil {
		// If DNS creation actually fails, return immediately with enhanced error
		expectedBehavior := getExpectedBehaviorForTest(config.TestId, config.NetworkingConfig)
		failureReason := fmt.Sprintf("DNS test pod creation failed - kubectl run command failed")
		failedCommand := fmt.Sprintf("kubectl run %s --image=nicolaka/netshoot --restart=Never --command -- sleep 3600", testPodName)
		commandOutput := err.Error()

		enhancedError := formatEnhancedFailure(expectedBehavior, failureReason, failedCommand, commandOutput)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating DNS test pod", false, enhancedError)
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), failureReason)
		return core.TestResult{
			Success: false,
			Message: enhancedError,
			Details: details,
		}
	}

	// Step 2: Retry ONLY verification with exponential backoff
	const maxVerifyRetries = 5
	var podCreated bool = false
	var lastVerifyError error
	var lastCommandOutput string

	for attempt := 1; attempt <= maxVerifyRetries; attempt++ {
		// Exponential backoff: 1s, 2s, 3s, 4s, 5s
		backoffDelay := time.Duration(attempt) * time.Second
		logger.LogInfo("DNS pod verification attempt %d/%d for %s (waiting %s)", attempt, maxVerifyRetries, testPodName, backoffDelay)
		time.Sleep(backoffDelay)

		// Verify DNS pod exists
		verifyOutput, verifyErr := executor.ExecuteKubectlCommand(ctx, "get", "pod", testPodName, "-o", "name")
		if verifyErr == nil {
			podCreated = true
			logger.LogInfo("DNS pod %s successfully verified after %d attempts", testPodName, attempt)
			break
		}

		// Store error details for enhanced failure message
		lastVerifyError = verifyErr
		lastCommandOutput = verifyOutput
		logger.LogError("DNS pod verification attempt %d failed: %v", attempt, verifyErr)
	}

	if !podCreated {
		// Enhanced failure output with diagnostic information
		expectedBehavior := getExpectedBehaviorForTest(config.TestId, config.NetworkingConfig)
		failureReason := fmt.Sprintf("DNS test pod creation failed - verification timeout after %d attempts", maxVerifyRetries)
		failedCommand := fmt.Sprintf("kubectl get pod %s -o name", testPodName)
		commandOutput := lastCommandOutput
		if lastVerifyError != nil && commandOutput == "" {
			commandOutput = lastVerifyError.Error()
		}

		enhancedError := formatEnhancedFailure(expectedBehavior, failureReason, failedCommand, commandOutput)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating DNS test pod", false, enhancedError)
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), failureReason)
		return core.TestResult{
			Success: false,
			Message: enhancedError,
			Details: details,
		}
	}

	// Wait for DNS test pod to be ready with increased timeout and better error handling
	logger.LogInfo("Waiting for DNS pod %s to be ready...", testPodName)
	_, err = executor.ExecuteKubectlCommand(ctx, "wait", "--for=condition=ready", "pod", testPodName, "--timeout=60s")
	if err != nil {
		// Get detailed pod status for debugging
		podStatus, _ := executor.ExecuteKubectlCommand(ctx, "get", "pod", testPodName, "-o", "wide")
		podEvents, _ := executor.ExecuteKubectlCommand(ctx, "describe", "pod", testPodName)

		logger.LogError("DNS pod ready wait failed. Status: %s", podStatus)
		if verbose {
			logger.LogInfo("DNS pod events: %s", podEvents)
		}

		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating DNS test pod", false, fmt.Sprintf("DNS test pod not ready: %v, status: %s", err, podStatus))
		// Cleanup pod
		executor.ExecuteKubectlCommand(ctx, "delete", "pod", testPodName, "--ignore-not-found=true")
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS test pod not ready")
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("DNS test pod not ready: %v", err),
			Details: details,
		}
	}

	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating DNS test pod", true, fmt.Sprintf("DNS test pod %s ready", testPodName))

	// Step 2: Test DNS resolution using real nslookup commands
	logger.LogStepName("Testing DNS resolution", fmt.Sprintf("Testing DNS resolution of %s service", serviceName))

	// Execute real nslookup command from test pod
	nslookupOutput, err := executor.ExecuteNslookupFromPod(ctx, testPodName, fmt.Sprintf("%s.default.svc.cluster.local", serviceName))
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing DNS resolution", false, fmt.Sprintf("DNS resolution failed: %v, output: %s", err, nslookupOutput))
		// Cleanup pod
		executor.ExecuteKubectlCommand(ctx, "delete", "pod", testPodName, "--ignore-not-found=true")
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS resolution test failed")
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("DNS resolution failed: %v", err),
			Details: details,
		}
	}

	// Check if DNS resolution was successful (nslookup output should contain an IP address)
	dnsSuccess := strings.Contains(nslookupOutput, "Address:") && !strings.Contains(nslookupOutput, "NXDOMAIN")
	if !dnsSuccess {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing DNS resolution", false, fmt.Sprintf("DNS resolution failed: no IP address found in output: %s", nslookupOutput))
		// Cleanup pod
		executor.ExecuteKubectlCommand(ctx, "delete", "pod", testPodName, "--ignore-not-found=true")
		duration := time.Since(startTime)
		logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS resolution test failed")
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("DNS resolution failed: no IP address found"),
			Details: details,
		}
	}

	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing DNS resolution", true, fmt.Sprintf("DNS resolution successful for %s service", serviceName))

	// Step 3: Cleanup DNS test pod
	logger.LogStepName("Cleanup", fmt.Sprintf("Removing DNS test pod %s", testPodName))
	_, err = executor.ExecuteKubectlCommand(ctx, "delete", "pod", testPodName, "--ignore-not-found=true")
	if err != nil {
		logger.LogInfo("Warning: Failed to cleanup DNS test pod: %v", err)
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Cleanup", true, fmt.Sprintf("DNS test pod %s removed", testPodName))

	// Complete test with final result
	duration := time.Since(startTime)

	// Store enhanced connectivity data in result details for FormatEnhancedTestSummary to use
	enhancedMessage := fmt.Sprintf("CONNECTIVITY_DATA:DNS resolved from testpod → service.namespace.svc.cluster.local (%.3fs)", duration.Seconds())
	details = append(details, enhancedMessage)

	logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "DNS resolution test completed")

	return core.TestResult{
		Success: true,
		Message: "DNS resolution test passed",
		Details: details,
	}
}

// checkCiliumStatus validates if Cilium CNI is healthy in the cluster
func checkCiliumStatus(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context) (bool, string) {
	logger.LogInfo("Querying Cilium pods in kube-system namespace...")

	// Check if Cilium pods are running
	pods, err := t.GetClientset().CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=cilium",
	})

	if err != nil {
		return false, fmt.Sprintf("Failed to check Cilium pod status: %v", err)
	}

	if len(pods.Items) == 0 {
		return false, "No Cilium pods found in kube-system namespace"
	}

	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}
	logger.LogInfo("Found %d Cilium pods: %s", len(pods.Items), strings.Join(podNames, ", "))

	// Count pods in various states and print details
	var running, failing int
	var failingPodNames []string

	for _, pod := range pods.Items {
		status := "Unknown"
		ready := isPodReady(&pod)

		if pod.Status.Phase == "Running" && ready {
			status = "Running"
			running++
		} else if pod.Status.Phase == "Failed" ||
			isPodInCrashLoop(&pod) ||
			(time.Since(pod.CreationTimestamp.Time) > time.Minute && pod.Status.Phase == "Pending") {
			status = "Failed"
			failing++
			failingPodNames = append(failingPodNames, pod.Name)
		} else {
			status = string(pod.Status.Phase)
		}

		readyStatus := "ready"
		if !ready {
			readyStatus = "not ready"
		}

		logger.LogInfo("Cilium pod %s status: %s (%s)", pod.Name, status, readyStatus)
	}

	// Check if all pods are running
	if running == len(pods.Items) {
		return true, ""
	}

	// Get Cilium config to report routing mode in the error message
	ciliumConfig, err := t.GetCiliumConfig(ctx)
	routingMode := "unknown"
	if err == nil && ciliumConfig["routing-mode"] != "" {
		routingMode = ciliumConfig["routing-mode"]
	}

	if failing > 0 {
		return false, fmt.Sprintf("Cilium is unhealthy: %d of %d pods failing, routing-mode=%s, failing pods: %s",
			failing, len(pods.Items), routingMode, strings.Join(failingPodNames, ", "))
	}

	return false, fmt.Sprintf("Cilium is not fully ready: %d of %d pods running, routing-mode=%s",
		running, len(pods.Items), routingMode)
}

// Helper functions for pod status checking
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isPodInCrashLoop(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil &&
			containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}

// Connectivity test implementations with real logic
func executeSameNodeConnectivityTest(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) core.TestResult {
	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	if len(workerNodes) < 1 {
		return core.TestResult{Success: false, Message: "Need at least 1 worker node for same-node testing"}
	}

	selectedNode := workerNodes[0]
	pod1Name := config.NetworkingConfig.ResourceNames["pod1"]
	pod2Name := config.NetworkingConfig.ResourceNames["pod2"]

	logger.LogInfo("Creating pods %s and %s on node: %s", pod1Name, pod2Name, selectedNode)

	// Create pods
	_, err = t.CreateNetshootPod(ctx, pod1Name, selectedNode)
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create pod %s: %v", pod1Name, err)}
	}

	pod2, err := t.CreateNetshootPod(ctx, pod2Name, selectedNode)
	if err != nil {
		t.CleanupPod(ctx, pod1Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create pod %s: %v", pod2Name, err)}
	}

	// Wait for pods to be ready
	err = t.WaitForPodReady(ctx, pod1Name, config.NetworkingConfig.Timeout)
	if err != nil {
		t.CleanupPods(ctx, pod1Name, pod2Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Pod %s did not become ready: %v", pod1Name, err)}
	}

	err = t.WaitForPodReady(ctx, pod2Name, config.NetworkingConfig.Timeout)
	if err != nil {
		t.CleanupPods(ctx, pod1Name, pod2Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Pod %s did not become ready: %v", pod2Name, err)}
	}

	// Test connectivity
	var details []string
	result := t.TestPodConnectivity(ctx, pod1Name, pod2Name, pod2, "same-node", &details)

	// Cleanup
	t.CleanupPods(ctx, pod1Name, pod2Name)

	return result
}

func executeCrossNodeConnectivityTest(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) core.TestResult {
	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	if len(workerNodes) < 2 {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Need at least 2 worker nodes for cross-node testing, found %d", len(workerNodes))}
	}

	pod1Name := config.NetworkingConfig.ResourceNames["pod1"]
	pod2Name := config.NetworkingConfig.ResourceNames["pod2"]

	logger.LogInfo("Creating pod %s on node %s", pod1Name, workerNodes[0])
	logger.LogInfo("Creating pod %s on node %s", pod2Name, workerNodes[1])

	// Create pods on different nodes
	_, err = t.CreateNetshootPod(ctx, pod1Name, workerNodes[0])
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create pod %s: %v", pod1Name, err)}
	}

	pod2, err := t.CreateNetshootPod(ctx, pod2Name, workerNodes[1])
	if err != nil {
		t.CleanupPod(ctx, pod1Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to create pod %s: %v", pod2Name, err)}
	}

	// Wait for pods to be ready
	err = t.WaitForPodReady(ctx, pod1Name, config.NetworkingConfig.Timeout)
	if err != nil {
		t.CleanupPods(ctx, pod1Name, pod2Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Pod %s did not become ready: %v", pod1Name, err)}
	}

	err = t.WaitForPodReady(ctx, pod2Name, config.NetworkingConfig.Timeout)
	if err != nil {
		t.CleanupPods(ctx, pod1Name, pod2Name)
		return core.TestResult{Success: false, Message: fmt.Sprintf("Pod %s did not become ready: %v", pod2Name, err)}
	}

	// Test connectivity
	var details []string
	result := t.TestPodConnectivity(ctx, pod1Name, pod2Name, pod2, "cross-node", &details)

	// Cleanup
	t.CleanupPods(ctx, pod1Name, pod2Name)

	return result
}

func executeBothPlacementConnectivityTests(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) core.TestResult {
	// Execute same-node test
	sameNodeConfig := config
	sameNodeConfig.NetworkingConfig.PlacementType = "same-node"
	sameNodeResult := executeSameNodeConnectivityTest(t, ctx, logger, sameNodeConfig)

	// Get worker nodes to check if cross-node test is possible
	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}
	}

	var crossNodeResult core.TestResult
	if len(workerNodes) >= 2 {
		// Execute cross-node test
		crossNodeConfig := config
		crossNodeConfig.NetworkingConfig.PlacementType = "cross-node"
		crossNodeResult = executeCrossNodeConnectivityTest(t, ctx, logger, crossNodeConfig)
	} else {
		logger.LogInfo("Skipping cross-node test: only %d worker node(s) available", len(workerNodes))
		crossNodeResult = core.TestResult{Success: true, Message: "Cross-node test skipped due to insufficient nodes"}
	}

	// Determine overall success
	bothSuccess := sameNodeResult.Success && crossNodeResult.Success
	var message string
	if len(workerNodes) >= 2 {
		if bothSuccess {
			message = "Both same-node and cross-node connectivity tests passed"
		} else if sameNodeResult.Success {
			message = "Same-node connectivity passed, cross-node failed"
		} else if crossNodeResult.Success {
			message = "Cross-node connectivity passed, same-node failed"
		} else {
			message = "Both same-node and cross-node connectivity tests failed"
		}
	} else {
		bothSuccess = sameNodeResult.Success
		message = sameNodeResult.Message
	}

	return core.TestResult{
		Success: bothSuccess,
		Message: message,
		Details: append(sameNodeResult.Details, crossNodeResult.Details...),
	}
}

// Enhanced connectivity test functions that return both result and connectivity details

// executeSameNodeConnectivityTestEnhanced returns both test result and connectivity details
func executeSameNodeConnectivityTestEnhanced(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) (core.TestResult, []core.ConnectivityResult) {
	startTime := time.Now()
	result := executeSameNodeConnectivityTest(t, ctx, logger, config)
	duration := time.Since(startTime)

	// Generate connectivity details from the test result
	var connectivityResults []core.ConnectivityResult
	if result.Success {
		pod1Name := config.NetworkingConfig.ResourceNames["pod1"]
		pod2Name := config.NetworkingConfig.ResourceNames["pod2"]

		connectivityResults = append(connectivityResults, core.ConnectivityResult{
			Source:     pod1Name,
			Target:     pod2Name,
			Protocol:   "ICMP",
			StatusCode: "ping-ok",
			Success:    true,
			Duration:   duration.Seconds(),
		})
	}

	return result, connectivityResults
}

// executeCrossNodeConnectivityTestEnhanced returns both test result and connectivity details
func executeCrossNodeConnectivityTestEnhanced(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) (core.TestResult, []core.ConnectivityResult) {
	startTime := time.Now()
	result := executeCrossNodeConnectivityTest(t, ctx, logger, config)
	duration := time.Since(startTime)

	// Generate connectivity details from the test result
	var connectivityResults []core.ConnectivityResult
	if result.Success {
		pod1Name := config.NetworkingConfig.ResourceNames["pod1"]
		pod2Name := config.NetworkingConfig.ResourceNames["pod2"]

		connectivityResults = append(connectivityResults, core.ConnectivityResult{
			Source:     pod1Name,
			Target:     pod2Name,
			Protocol:   "ICMP",
			StatusCode: "ping-ok",
			Success:    true,
			Duration:   duration.Seconds(),
		})
	}

	return result, connectivityResults
}

// executeBothPlacementConnectivityTestsEnhanced returns both test result and connectivity details
func executeBothPlacementConnectivityTestsEnhanced(t *core.Tester, ctx context.Context, logger *core.MultiChannelLogger, config PolicyTestConfig) (core.TestResult, []core.ConnectivityResult) {
	// Execute same-node test with connectivity details
	sameNodeConfig := config
	sameNodeConfig.NetworkingConfig.PlacementType = "same-node"
	sameNodeResult, sameNodeConnectivity := executeSameNodeConnectivityTestEnhanced(t, ctx, logger, sameNodeConfig)

	// Get worker nodes to check if cross-node test is possible
	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		return core.TestResult{Success: false, Message: fmt.Sprintf("Failed to get worker nodes: %v", err)}, []core.ConnectivityResult{}
	}

	var crossNodeResult core.TestResult
	var crossNodeConnectivity []core.ConnectivityResult
	if len(workerNodes) >= 2 {
		// Execute cross-node test with connectivity details
		crossNodeConfig := config
		crossNodeConfig.NetworkingConfig.PlacementType = "cross-node"
		crossNodeResult, crossNodeConnectivity = executeCrossNodeConnectivityTestEnhanced(t, ctx, logger, crossNodeConfig)
	} else {
		logger.LogInfo("Skipping cross-node test: only %d worker node(s) available", len(workerNodes))
		crossNodeResult = core.TestResult{Success: true, Message: "Cross-node test skipped due to insufficient nodes"}
		crossNodeConnectivity = []core.ConnectivityResult{}
	}

	// Determine overall success
	bothSuccess := sameNodeResult.Success && crossNodeResult.Success
	var message string
	if len(workerNodes) >= 2 {
		if bothSuccess {
			message = "Both same-node and cross-node connectivity tests passed"
		} else if sameNodeResult.Success {
			message = "Same-node connectivity passed, cross-node failed"
		} else if crossNodeResult.Success {
			message = "Cross-node connectivity passed, same-node failed"
		} else {
			message = "Both same-node and cross-node connectivity tests failed"
		}
	} else {
		bothSuccess = sameNodeResult.Success
		message = sameNodeResult.Message
	}

	finalResult := core.TestResult{
		Success: bothSuccess,
		Message: message,
		Details: append(sameNodeResult.Details, crossNodeResult.Details...),
	}

	// Combine connectivity results
	allConnectivityResults := append(sameNodeConnectivity, crossNodeConnectivity...)

	return finalResult, allConnectivityResults
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
func validateL4DataCapture(testId string, results []core.ConnectivityResult) error {
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
func validatePortConnectivity(testId string, results []core.ConnectivityResult) error {
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
func validateICMPConnectivity(testId string, results []core.ConnectivityResult) error {
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
func validateTLSConnectivity(testId string, results []core.ConnectivityResult) error {
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
func validateGenericL4Connectivity(testId string, results []core.ConnectivityResult) error {
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
func validateL7DataCapture(testId string, results []core.ConnectivityResult) error {
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
func validateHTTPConnectivity(testId string, results []core.ConnectivityResult) error {
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
func validateDNSConnectivity(testId string, results []core.ConnectivityResult) error {
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
func validateGenericL7Connectivity(testId string, results []core.ConnectivityResult) error {
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
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	connectivityDataCapture func(context.Context, string) []core.ConnectivityResult,
) ([]core.TimedTestResult, []string) {
	var timedResults []core.TimedTestResult
	var testNames []string

	// Use command logger singleton for JSONL logging
	logger := core.GetGlobalMultiChannelLogger()
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
			var result core.TestResult
			if config.GroupId == "networking" {
				// Use networking test framework for networking tests
				result = ExecuteNetworkingTest(config, logger, t, groupCtx, verbose, currentTestNumber, totalTests)
			} else {
				// Use policy test framework for policy tests
				result = ExecutePolicyTest(config, logger, t, groupCtx, false, verbose, currentTestNumber, totalTests)
			}
			testEnd := time.Now()
			testElapsed := core.ElapsedSeconds(testStart)
			currentTestNumber++

			if result.Success {
				// Capture REAL connectivity data during policy tests
				connectivityResults := connectivityDataCapture(groupCtx, config.TestId)

				// If no real results captured, show empty results rather than fake data
				if len(connectivityResults) == 0 {
					connectivityResults = []core.ConnectivityResult{} // Empty - no fake data
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
				enhancedResult := core.CreateEnhancedTestResultWithExpectation(result, config.TestId+"-policy", connectivityResults, nil)
				formattedResult := core.FormatEnhancedTestResultForHierarchy(config.TestId, enhancedResult, testElapsed, verbose)
				fmt.Print(formattedResult)
			} else {
				// For failed tests, print newline first to break from tree structure
				fmt.Print("\n")

				// For failed tests in verbose mode, get real command history and create proper enhanced result
				if verbose {
					executor := t.GetLastExecutor()
					if executor != nil {
						failurePoint := determineFailurePoint(config.TestId)
						errorDetails := core.ExtractErrorDetailsFromCommand(executor.GetCommandHistory(), fmt.Sprintf("%s policy validation", strings.ToUpper(config.GroupId[:2])))
						enhancedResult := core.CreateEnhancedTestResult(result, executor, failurePoint, errorDetails)

						// Use the verbose failure formatting that shows command history
						formattedResult := core.FormatVerboseTestFailure(config.TestId, testElapsed, enhancedResult.ExecutedCommands, enhancedResult.FailurePoint, enhancedResult.ErrorDetails)
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
			timedResult := core.TimedTestResult{
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
func performNetworkingEmergencyCleanup(ctx context.Context, config PolicyTestConfig, logger *core.MultiChannelLogger, t *core.Tester) {
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
		executor := core.NewCommandExecutor(logger, "diagnostic-test", false)
		_, err := executor.ExecuteKubectlCommand(ctx, "delete", "service", serviceName, "--ignore-not-found=true")
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("service %s: %v", serviceName, err))
		} else {
			logger.LogInfo("Emergency cleanup: Successfully cleaned up service %s", serviceName)
		}
	}

	// 3. Deployment cleanup
	if deploymentName, exists := resourceNames["deployment"]; exists && deploymentName != "" {
		executor := core.NewCommandExecutor(logger, "diagnostic-test", false)
		_, err := executor.ExecuteKubectlCommand(ctx, "delete", "deployment", deploymentName, "--ignore-not-found=true")
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("deployment %s: %v", deploymentName, err))
		} else {
			logger.LogInfo("Emergency cleanup: Successfully cleaned up deployment %s", deploymentName)
		}
	}

	// 4. Clean up any DNS test pods (dynamic names)
	if config.NetworkingConfig.TestType == "dns" {
		executor := core.NewCommandExecutor(logger, "diagnostic-test", false)
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
