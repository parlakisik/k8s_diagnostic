package networking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Type aliases to avoid having to change all the existing code
type TestConfig = core.TestConfig

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// isPodInCrashLoop checks if a pod is in crash loop
func isPodInCrashLoop(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil &&
			containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			return true
		}
	}
	return false
}

// TestPodToPodConnectivity creates two netshoot pods and tests connectivity between them
func TestPodToPodConnectivity(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context) core.TestResult {
	return TestPodToPodConnectivityWithConfig(logger, t, ctx, TestConfig{})
}

// TestPodToPodConnectivityWithConfig tests connectivity with configurable pod source
func TestPodToPodConnectivityWithConfig(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, config TestConfig) core.TestResult {
	// Determine the appropriate test ID based on placement strategy
	testId := determineTestId(config.Placement)

	// Get the networking test configuration
	testConfig := GetNetworkingTestByID(testId)
	if testConfig == nil {
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Unknown test ID: %s", testId),
		}
	}

	// Override placement if specified in config
	if config.Placement != "" {
		testConfig.NetworkingConfig.PlacementType = config.Placement
	}

	// Execute the test using the common framework
	return core.ExecuteNetworkingTest(*testConfig, logger, t, ctx, false, 1, 1)
}

// determineTestId converts placement strategy to test ID
func determineTestId(placement string) string {
	switch placement {
	case "same-node":
		return "pod-to-pod-same-node"
	case "cross-node":
		return "pod-to-pod-cross-node"
	case "both", "":
		return "pod-to-pod-cross-node" // Use cross-node as default for "both"
	default:
		return "pod-to-pod-cross-node"
	}
}

// checkCiliumStatus validates if Cilium CNI is healthy in the cluster
func checkCiliumStatus(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context) (bool, string) {
	// Use logger for consistent output to terminal and file
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

		if pod.Status.Phase == corev1.PodRunning && ready {
			status = "Running"
			running++
		} else if pod.Status.Phase == corev1.PodFailed ||
			isPodInCrashLoop(&pod) ||
			(time.Since(pod.CreationTimestamp.Time) > time.Minute && pod.Status.Phase == corev1.PodPending) {
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

// ExecuteNetworkingTestFromConfig is a wrapper that calls the common framework
func ExecuteNetworkingTestFromConfig(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
	testNumber int,
	totalTests int,
) core.TestResult {
	return core.ExecuteNetworkingTest(config, logger, t, ctx, verbose, testNumber, totalTests)
}

// NetworkingSubgroups defines test subgroups for organization and concurrent execution
// Exported so it can be accessed from cmd/test.go for validation
var NetworkingSubgroups = map[string][]string{
	"pod-connectivity": {"pod-to-pod-same-node", "pod-to-pod-cross-node"},
	"services":         {"service-clusterip", "service-nodeport", "service-loadbalancer", "service-cross-node"},
	"dns":              {"dns-resolution"},
}

// Map of test names to test keys (for CLI reference)
var NetworkingTestNameToKey = map[string]string{
	"pod-to-pod-same-node-connectivity-test":  "pod-to-pod-same-node",
	"pod-to-pod-cross-node-connectivity-test": "pod-to-pod-cross-node",
	"service-clusterip-connectivity-test":     "service-clusterip",
	"service-nodeport-connectivity-test":      "service-nodeport",
	"service-loadbalancer-connectivity-test":  "service-loadbalancer",
	"cross-node-service-connectivity-test":    "service-cross-node",
	"dns-resolution-test":                     "dns-resolution",
}

// TestNetworkingPoliciesSequential runs networking tests with enhanced formatting like L3/L4/L7 tests
func TestNetworkingPoliciesSequential(t *core.Tester, ctx context.Context, requestedTests []string, verbose ...bool) []core.TestResult {
	// Determine verbose mode
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	allResults := []core.TestResult{}

	if isVerbose {
		fmt.Printf("\n===== NETWORKING TESTS SEQUENTIAL EXECUTION - VERBOSE MODE ENABLED =====\n")
		fmt.Printf("Verbose output will be shown for each test\n")
		fmt.Printf("============================================================\n\n")
	}

	fmt.Printf("Starting networking tests...\n")

	// Create test group with all networking tests as individual tests (no subgroups)
	testGroup := BuildNetworkingTestGroup(requestedTests)

	// Define networking-specific connectivity data capture function
	connectivityDataCapture := func(ctx context.Context, testId string) []core.ConnectivityResult {
		// Return empty for networking tests since connectivity data is captured within the test functions themselves
		// The enhanced data is embedded in result.Details by executeConnectivityTest
		return []core.ConnectivityResult{}
	}

	// Execute tests using the common framework with enhanced data collection
	timedResults, testNames := core.ExecutePolicyTestGroups(
		[]core.PolicyTestGroup{testGroup},
		t,
		ctx,
		isVerbose,
		connectivityDataCapture,
	)

	// Extract basic results for backward compatibility
	for _, timedResult := range timedResults {
		allResults = append(allResults, timedResult.TestResult)
	}

	// Calculate passed/failed counts
	passedTests := 0
	failedTests := 0
	for _, result := range allResults {
		if result.Success {
			passedTests++
		} else {
			failedTests++
		}
	}

	// Create display names mapping for networking tests
	displayNames := map[string]string{
		"pod-to-pod-same-node":  "Pod-to-Pod Same-Node Connectivity",
		"pod-to-pod-cross-node": "Pod-to-Pod Cross-Node Connectivity",
		"service-clusterip":     "Service ClusterIP Connectivity",
		"service-nodeport":      "Service NodePort Connectivity",
		"service-loadbalancer":  "Service LoadBalancer Connectivity",
		"service-cross-node":    "Cross-Node Service Connectivity",
		"dns-resolution":        "DNS Resolution",
	}

	// Display enhanced verbose summary with Expected vs Received details
	core.FormatEnhancedTestSummary(timedResults, testNames, displayNames, isVerbose)

	return allResults
}

// BuildNetworkingTestGroup builds a single test group with all networking tests as individual tests
func BuildNetworkingTestGroup(requestedTests []string) core.PolicyTestGroup {
	// Get all available networking test configurations
	allNetworkingConfigs := NetworkingTestConfigs

	// Filter based on requested tests, or use all if none specified
	var selectedConfigs []core.PolicyTestConfig
	if len(requestedTests) == 0 {
		// Use all networking tests if none specified
		selectedConfigs = allNetworkingConfigs
	} else {
		// Filter to only requested tests
		for _, config := range allNetworkingConfigs {
			for _, requestedTest := range requestedTests {
				if config.TestId == requestedTest {
					selectedConfigs = append(selectedConfigs, config)
					break
				}
			}
		}
	}

	// Create a single test group containing all selected networking tests
	return core.PolicyTestGroup{
		Name:        "networking",
		GroupId:     "networking",
		TestConfigs: selectedConfigs,
	}
}

// =============================================================================
// SERVICE TEST IMPLEMENTATIONS (RESTORED FROM BACKUP)
// =============================================================================

// executeClusterIPTest handles ClusterIP service connectivity testing
func executeClusterIPTest(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Create nginx deployment with robust error handling
	logger.LogStepName("Creating deployment", fmt.Sprintf("Deploying %s with nginx image", deploymentName))

	_, err := t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP deployment setup failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP deployment not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating deployment", true, fmt.Sprintf("Deployment %s created and ready", deploymentName))

	// Step 2: Create ClusterIP service
	logger.LogStepName("Creating service", fmt.Sprintf("Creating ClusterIP service %s", serviceName))

	_, err = t.CreateNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", false, fmt.Sprintf("Failed to create service: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP service creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service: %v", err),
			Details: details,
		}
	}

	serviceIP, err := t.GetServiceIP(ctx, serviceName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", false, fmt.Sprintf("Failed to get service IP: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP service IP retrieval failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get service IP: %v", err),
			Details: details,
		}
	}
	logger.LogInfo("ClusterIP service IP: %s", serviceIP)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating service", true, fmt.Sprintf("ClusterIP service %s created", serviceName))

	// Step 3: Create test pod with proper error handling
	logger.LogStepName("Creating test pod", fmt.Sprintf("Creating netshoot test pod: %s", testPodName))

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP test pod creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP test pod not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, fmt.Sprintf("Test pod %s created and ready", testPodName))

	// Step 4: Test HTTP connectivity
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity to ClusterIP service %s", serviceName))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP HTTP connectivity failed")
		}
		return core.TestResult{
			Success: false,
			Message: "ClusterIP HTTP connectivity failed",
			Details: details,
		}
	}

	// Evaluate HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ ClusterIP HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("ClusterIP connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("WARNING: ClusterIP HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("ClusterIP HTTP connectivity issue - %s", message))
	}

	// Cleanup all resources
	t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)

	// Complete test with final result
	duration := time.Since(startTime)
	if verbose {
		if success {
			logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "ClusterIP service connectivity test passed - HTTP connectivity working")
		} else {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "ClusterIP service connectivity test failed - HTTP connectivity issues")
		}
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("ClusterIP service connectivity test - Status: %s", message),
		Details: details,
	}
}

// executeCrossNodeServiceTest handles cross-node service connectivity testing
func executeCrossNodeServiceTest(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 2 worker nodes for cross-node testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Failed to get worker nodes")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 2 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 2 worker nodes, found %d", len(workerNodes)))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Insufficient worker nodes for cross-node testing")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Cross-node service test requires at least 2 worker nodes, found %d", len(workerNodes)),
			Details: details,
		}
	}
	logger.LogInfo("Found %d worker nodes: %s", len(workerNodes), strings.Join(workerNodes, ", "))
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for cross-node testing", len(workerNodes)))

	// Step 2: Setup cross-node service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and service for cross-node testing")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service deployment setup failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service deployment not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}

	_, err = t.CreateNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create service: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service: %v", err),
			Details: details,
		}
	}

	serviceIP, err := t.GetServiceIP(ctx, serviceName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to get service IP: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service IP retrieval failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get service IP: %v", err),
			Details: details,
		}
	}
	logger.LogInfo("Cross-node service IP: %s", serviceIP)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", true, "Cross-node service environment ready")

	// Step 3: Create test pod on different node
	logger.LogStepName("Creating test pod", fmt.Sprintf("Creating netshoot pod on node %s for cross-node testing", workerNodes[1]))

	_, err = t.CreateNetshootPod(ctx, testPodName, workerNodes[1])
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod on node %s: %v", workerNodes[1], err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node test pod creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod on node %s: %v", workerNodes[1], err),
			Details: details,
		}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node test pod not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	logger.LogInfo("Cross-node test pod created successfully on node %s", workerNodes[1])
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, "Cross-node test pod created and ready")

	// Step 4: Test cross-node HTTP connectivity
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity from test pod to service %s", serviceName))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service HTTP connectivity failed")
		}
		return core.TestResult{
			Success: false,
			Message: "Cross-node service HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ Cross-node HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("Cross-node service connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ Cross-node HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("Cross-node HTTP connectivity issue - %s", message))
	}

	// Cleanup all resources
	t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)

	// Complete test with final result
	duration := time.Since(startTime)
	if verbose {
		if success {
			logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "Cross-node service connectivity test passed - HTTP connectivity working across nodes")
		} else {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Cross-node service connectivity test failed - HTTP connectivity issues")
		}
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("Cross-node service connectivity test - Status: %s", message),
		Details: details,
	}
}

// executeNodePortTest handles NodePort service connectivity testing
func executeNodePortTest(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 1 worker node for NodePort testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Failed to get worker nodes")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 1 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 1 worker node, found %d", len(workerNodes)))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Insufficient worker nodes for NodePort testing")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("NodePort test requires at least 1 worker node, found %d", len(workerNodes)),
			Details: details,
		}
	}
	logger.LogInfo("Found %d worker nodes for NodePort testing", len(workerNodes))
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for NodePort testing", len(workerNodes)))

	// Step 2: Setup NodePort service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and NodePort service")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort deployment setup failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort deployment not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}

	createdService, err := t.CreateNginxServiceWithType(ctx, serviceName, deploymentName, core.ServiceTypeNodePort)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create NodePort service: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort service creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create NodePort service: %v", err),
			Details: details,
		}
	}

	nodePort := int(createdService.Spec.Ports[0].NodePort)
	logger.LogInfo("NodePort assigned: %d", nodePort)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", true, "NodePort service environment ready")

	// Step 3: Setup test pod and get node IP
	logger.LogStepName("Setting up test environment", fmt.Sprintf("Getting node IP address for NodePort %d access", nodePort))

	node, err := t.GetClientset().CoreV1().Nodes().Get(ctx, workerNodes[0], metav1.GetOptions{})
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Failed to get node information: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Node information retrieval failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get node information: %v", err),
			Details: details,
		}
	}

	var nodeIP string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			nodeIP = address.Address
			break
		}
	}

	if nodeIP == "" {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, "Could not determine node IP address")
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Node IP address not found")
		}
		return core.TestResult{
			Success: false,
			Message: "Could not determine node IP address",
			Details: details,
		}
	}

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Failed to create test pod: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort test pod creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort test pod not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod did not become ready: %v", err),
			Details: details,
		}
	}

	logger.LogInfo("Node IP for NodePort access: %s", nodeIP)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up test environment", true, "NodePort test environment configured")

	// Step 4: Test NodePort connectivity
	nodePortURL := fmt.Sprintf("%s:%d", nodeIP, nodePort)
	logger.LogStepName("Testing connectivity", fmt.Sprintf("Testing HTTP connectivity to NodePort %s", nodePortURL))

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, nodePortURL)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity to NodePort failed: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort HTTP connectivity failed")
		}
		return core.TestResult{
			Success: false,
			Message: "NodePort HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ NodePort HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("NodePort connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ NodePort HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("NodePort HTTP connectivity issue - %s", message))
	}

	// Cleanup all resources
	t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)

	// Complete test with final result
	duration := time.Since(startTime)
	if verbose {
		if success {
			logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "NodePort service connectivity test passed - HTTP connectivity working through node port")
		} else {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "NodePort service connectivity test failed - HTTP connectivity issues")
		}
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("NodePort service connectivity test - Status: %s", message),
		Details: details,
	}
}

// executeLoadBalancerTest handles LoadBalancer service connectivity testing
func executeLoadBalancerTest(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Check node availability
	logger.LogStepName("Checking node availability", "Verifying at least 1 worker node for LoadBalancer testing")

	workerNodes, err := t.GetWorkerNodes(ctx)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Failed to get worker nodes: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Failed to get worker nodes")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 1 {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", false, fmt.Sprintf("Need at least 1 worker node, found %d", len(workerNodes)))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "Insufficient worker nodes for LoadBalancer testing")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("LoadBalancer test requires at least 1 worker node, found %d", len(workerNodes)),
			Details: details,
		}
	}
	logger.LogInfo("Found %d worker nodes for LoadBalancer testing", len(workerNodes))
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Checking node availability", true, fmt.Sprintf("Found %d worker nodes for LoadBalancer testing", len(workerNodes)))

	// Step 2: Setup LoadBalancer service environment
	logger.LogStepName("Setting up service environment", "Creating nginx deployment and LoadBalancer service")

	_, err = t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer deployment setup failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Deployment did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer deployment not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}

	createdService, err := t.CreateNginxServiceWithType(ctx, serviceName, deploymentName, core.ServiceTypeLoadBalancer)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up service environment", false, fmt.Sprintf("Failed to create LoadBalancer service: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer service creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create LoadBalancer service: %v", err),
			Details: details,
		}
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
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create test pod: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer test pod creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Test pod did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer test pod not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod did not become ready: %v", err),
			Details: details,
		}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, "LoadBalancer test pod created and ready")

	// Step 4: Test LoadBalancer connectivity
	logger.LogStepName("Testing connectivity", "Testing HTTP connectivity via ClusterIP (fallback for local environments)")

	statusCode, err := t.TestHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("HTTP connectivity failed: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer HTTP connectivity failed")
		}
		return core.TestResult{
			Success: false,
			Message: "LoadBalancer HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		logger.LogInfo("✓ LoadBalancer HTTP connectivity successful - Status: %s", statusCode)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", true, fmt.Sprintf("LoadBalancer connectivity successful - Status: %s", statusCode))
	} else {
		logger.LogInfo("✗ LoadBalancer HTTP connectivity issue - %s", message)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing connectivity", false, fmt.Sprintf("LoadBalancer HTTP connectivity issue - %s", message))
	}

	// Cleanup all resources
	t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)

	// Complete test with final result
	duration := time.Since(startTime)
	if verbose {
		if success {
			logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "LoadBalancer service connectivity test passed - HTTP connectivity working via service")
		} else {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "LoadBalancer service connectivity test failed - HTTP connectivity issues")
		}
	}

	return core.TestResult{
		Success: success,
		Message: fmt.Sprintf("LoadBalancer service connectivity test - Status: %s", message),
		Details: details,
	}
}

// executeDNSTest handles DNS resolution testing (modernized from backup)
func executeDNSTest(
	config core.PolicyTestConfig,
	logger *core.MultiChannelLogger,
	t *core.Tester,
	ctx context.Context,
	verbose bool,
) core.TestResult {
	startTime := time.Now()
	var details []string

	deploymentName := config.NetworkingConfig.ResourceNames["deployment"]
	serviceName := config.NetworkingConfig.ResourceNames["service"]
	testPodName := config.NetworkingConfig.ResourceNames["testpod"]

	if verbose {
		logger.LogTestStart(config.TestTitle, 1, 1, config.GroupId)
	}

	// Step 1: Create DNS test environment
	logger.LogStepName("Setting up DNS test environment", "Creating nginx deployment and service for DNS testing")

	_, err := t.CreateNginxDeployment(ctx, deploymentName)
	if err != nil {
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up DNS test environment", false, fmt.Sprintf("Failed to create nginx deployment: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS test environment setup failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment for DNS test: %v", err),
			Details: details,
		}
	}

	_, err = t.CreateNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up DNS test environment", false, fmt.Sprintf("Failed to create service: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS test service creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service for DNS test: %v", err),
			Details: details,
		}
	}
	logger.LogInfo("Created deployment '%s' and service '%s' for DNS testing", deploymentName, serviceName)
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Setting up DNS test environment", true, "DNS test environment ready")

	// Step 2: Create and setup DNS test pod
	logger.LogStepName("Creating test pod", "Creating netshoot pod for DNS resolution testing")

	_, err = t.CreateNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("Failed to create DNS test pod: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS test pod creation failed")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create DNS test pod: %v", err),
			Details: details,
		}
	}

	if err := t.WaitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", false, fmt.Sprintf("DNS test pod did not become ready: %v", err))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS test pod not ready")
		}
		return core.TestResult{
			Success: false,
			Message: fmt.Sprintf("DNS test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Creating test pod", true, "DNS test pod created and ready")

	// Step 3: Test DNS resolution
	fqdnName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, t.GetNamespace())
	logger.LogStepName("Testing DNS resolution", fmt.Sprintf("Testing FQDN resolution for %s", fqdnName))

	fqdnResult, fqdnErr := t.TestDNSResolution(ctx, testPodName, fqdnName)
	if fqdnErr != nil {
		t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing DNS resolution", false, fmt.Sprintf("Service FQDN DNS resolution failed: %v", fqdnErr))
		duration := time.Since(startTime)
		if verbose {
			logger.LogTestComplete(config.TestTitle, 1, 1, false, duration.Seconds(), "DNS resolution test failed")
		}
		return core.TestResult{
			Success: false,
			Message: "DNS resolution test failed",
			Details: details,
		}
	} else {
		logger.LogInfo("✓ Service FQDN DNS resolution successful")
		logger.LogInfo("  Result: %s", strings.TrimSpace(fqdnResult))
		logger.GetFrontendLogger().LogStepCompleteWithForcedFlush("Testing DNS resolution", true, "DNS resolution successful")
	}

	// Cleanup all resources
	t.CleanupServiceResources(ctx, deploymentName, serviceName, testPodName)

	// Complete test with final result
	duration := time.Since(startTime)
	if verbose {
		logger.LogTestComplete(config.TestTitle, 1, 1, true, duration.Seconds(), "DNS resolution test passed - Service FQDN resolution working")
	}
	return core.TestResult{
		Success: true,
		Message: "DNS resolution test passed - Service FQDN resolution working",
		Details: details,
	}
}

// evaluateHTTPStatusCode evaluates HTTP status codes and returns success status with message
// This is a local copy to avoid circular import issues with common package
func evaluateHTTPStatusCode(statusCode string) (bool, string) {
	// Handle empty status code
	if statusCode == "" {
		return false, "Empty status code"
	}

	// Try to parse as integer
	if strings.Contains(statusCode, "200") || statusCode == "200" {
		return true, "Success (200)"
	}

	// Handle common HTTP status codes
	switch statusCode {
	case "404":
		return false, "Not Found (404)"
	case "403":
		return false, "Forbidden (403)"
	case "500":
		return false, "Internal Server Error (500)"
	case "timeout":
		return false, "Connection timeout"
	case "connection_refused":
		return false, "Connection refused"
	default:
		// If it contains "200" somewhere, treat as success
		if strings.Contains(statusCode, "200") {
			return true, "Success (200)"
		}
		return false, fmt.Sprintf("HTTP error or unknown status: %s", statusCode)
	}
}
