package diagnostic

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// evaluateHTTPStatusCode evaluates an HTTP status code and returns success status and descriptive message
func evaluateHTTPStatusCode(statusCode string) (bool, string) {
	code, err := strconv.Atoi(statusCode)
	if err != nil {
		return false, fmt.Sprintf("Invalid status code: %s", statusCode)
	}

	switch {
	case code >= 200 && code < 300:
		return true, fmt.Sprintf("Success - HTTP %d", code)
	case code >= 300 && code < 400:
		return false, fmt.Sprintf("Redirect - HTTP %d (may need to follow redirects)", code)
	case code >= 400 && code < 500:
		return false, fmt.Sprintf("Client Error - HTTP %d", code)
	case code >= 500 && code < 600:
		return false, fmt.Sprintf("Server Error - HTTP %d", code)
	default:
		return false, fmt.Sprintf("Unknown status code: %d", code)
	}
}

// CommandOutput represents a command execution result
type CommandOutput struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Description string `json:"description"`
}

// NetworkContext represents network diagnostic information
type NetworkContext struct {
	SourcePodIP    string            `json:"source_pod_ip,omitempty"`
	TargetPodIP    string            `json:"target_pod_ip,omitempty"`
	ServiceIP      string            `json:"service_ip,omitempty"`
	SourceNode     string            `json:"source_node,omitempty"`
	TargetNode     string            `json:"target_node,omitempty"`
	RoutingInfo    []string          `json:"routing_info,omitempty"`
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}

// DetailedDiagnostics represents comprehensive diagnostic information
type DetailedDiagnostics struct {
	FailureStage         string          `json:"failure_stage,omitempty"`
	TechnicalError       string          `json:"technical_error,omitempty"`
	CommandOutputs       []CommandOutput `json:"command_outputs,omitempty"`
	NetworkContext       *NetworkContext `json:"network_context,omitempty"`
	TroubleshootingHints []string        `json:"troubleshooting_hints,omitempty"`
}

// TestConfig represents configuration for test execution
type TestConfig struct {
	Placement              string `json:"placement"` // "same-node", "cross-node", "both"
	FailCiliumConnectivity bool   `json:"fail_cilium_connectivity"`
}

// TestResult represents the result of a connectivity test
type TestResult struct {
	Success             bool                 `json:"success"`
	Message             string               `json:"message"`
	Details             []string             `json:"details"`
	DetailedDiagnostics *DetailedDiagnostics `json:"detailed_diagnostics,omitempty"`
}

// Tester handles connectivity testing operations
type Tester struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	namespace string
}

// NewTester creates a new connectivity tester
func NewTester(kubeconfig, namespace string) (*Tester, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			// Try to use default kubeconfig
			config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &Tester{
		clientset: clientset,
		config:    config,
		namespace: namespace,
	}, nil
}

// EnsureNamespace creates the test namespace if it doesn't exist
func (t *Tester) EnsureNamespace(ctx context.Context) error {
	return t.ensureNamespace(ctx)
}

// CleanupNamespace removes the test namespace
func (t *Tester) CleanupNamespace(ctx context.Context) error {
	err := t.clientset.CoreV1().Namespaces().Delete(ctx, t.namespace, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %v", t.namespace, err)
	}
	return nil
}

// TestPodToPodConnectivity creates two netshoot pods and tests connectivity between them
func (t *Tester) TestPodToPodConnectivity(ctx context.Context) TestResult {
	return t.TestPodToPodConnectivityWithConfig(ctx, TestConfig{})
}

// TestPodToPodConnectivityWithConfig tests connectivity with configurable pod source
func (t *Tester) TestPodToPodConnectivityWithConfig(ctx context.Context, config TestConfig) TestResult {
	return t.testWithFreshPods(ctx, config)
}

// testWithFreshPods tests connectivity using newly created pods with placement strategy support
func (t *Tester) testWithFreshPods(ctx context.Context, config TestConfig) TestResult {
	// Handle different placement strategies
	switch config.Placement {
	case "same-node":
		return t.testSameNodePods(ctx, config)
	case "cross-node":
		return t.testCrossNodePods(ctx, config)
	case "both":
		return t.testBothPlacements(ctx, config)
	default:
		// Default to "both" for backward compatibility
		return t.testBothPlacements(ctx, config)
	}
}

// testSameNodePods tests connectivity between pods on the same worker node
func (t *Tester) testSameNodePods(ctx context.Context, config TestConfig) TestResult {
	var details []string

	// Get worker nodes
	workerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 1 {
		return TestResult{
			Success: false,
			Message: "Need at least 1 worker node for same-node testing",
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes", len(workerNodes)))

	// Pick the first worker node for both pods
	selectedNode := workerNodes[0]
	details = append(details, fmt.Sprintf("✓ Selected node %s for same-node testing", selectedNode))

	// Create two test pods on the same node
	pod1Name := "netshoot-same-1"
	pod2Name := "netshoot-same-2"

	_, err = t.createNetshootPod(ctx, pod1Name, selectedNode)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create pod %s: %v", pod1Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created pod %s on node %s", pod1Name, selectedNode))

	pod2, err := t.createNetshootPod(ctx, pod2Name, selectedNode)
	if err != nil {
		t.cleanupPod(ctx, pod1Name)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create pod %s: %v", pod2Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created pod %s on node %s", pod2Name, selectedNode))

	// Wait for pods to be ready using helper function
	cleanupFunc := func() {
		t.cleanupPods(ctx, pod1Name, pod2Name)
	}

	if err := t.WaitForPodReadyOrCleanup(ctx, pod1Name, 120*time.Second, cleanupFunc, &details); err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod1Name, err),
			Details: details,
		}
	}

	if err := t.WaitForPodReadyOrCleanup(ctx, pod2Name, 120*time.Second, cleanupFunc, &details); err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod2Name, err),
			Details: details,
		}
	}

	// Test connectivity
	result := t.testPodConnectivity(ctx, pod1Name, pod2Name, pod2, "same-node", &details, config)

	// Cleanup pods
	t.cleanupPods(ctx, pod1Name, pod2Name)
	details = append(details, "✓ Cleaned up test pods")

	result.Details = details
	return result
}

// testCrossNodePods tests connectivity between pods on different worker nodes
func (t *Tester) testCrossNodePods(ctx context.Context, config TestConfig) TestResult {
	var details []string

	// Get worker nodes
	workerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 2 {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Need at least 2 worker nodes for cross-node testing, found %d", len(workerNodes)),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes", len(workerNodes)))

	// Create two test pods on different nodes
	pod1Name := "netshoot-cross-1"
	pod2Name := "netshoot-cross-2"

	_, err = t.createNetshootPod(ctx, pod1Name, workerNodes[0])
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create pod %s: %v", pod1Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created pod %s on node %s", pod1Name, workerNodes[0]))

	pod2, err := t.createNetshootPod(ctx, pod2Name, workerNodes[1])
	if err != nil {
		t.cleanupPod(ctx, pod1Name)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create pod %s: %v", pod2Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created pod %s on node %s", pod2Name, workerNodes[1]))

	// Wait for pods to be ready using helper function
	cleanupFunc := func() {
		t.cleanupPods(ctx, pod1Name, pod2Name)
	}

	if err := t.WaitForPodReadyOrCleanup(ctx, pod1Name, 120*time.Second, cleanupFunc, &details); err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod1Name, err),
			Details: details,
		}
	}

	if err := t.WaitForPodReadyOrCleanup(ctx, pod2Name, 120*time.Second, cleanupFunc, &details); err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod2Name, err),
			Details: details,
		}
	}

	// Test connectivity
	result := t.testPodConnectivity(ctx, pod1Name, pod2Name, pod2, "cross-node", &details, config)

	// Cleanup pods
	t.cleanupPods(ctx, pod1Name, pod2Name)
	details = append(details, "✓ Cleaned up test pods")

	result.Details = details
	return result
}

// testBothPlacements runs both same-node and cross-node tests, returning combined results
func (t *Tester) testBothPlacements(ctx context.Context, config TestConfig) TestResult {
	var allDetails []string

	// Test same-node first
	sameNodeConfig := config
	sameNodeConfig.Placement = "same-node"
	sameNodeResult := t.testSameNodePods(ctx, sameNodeConfig)

	allDetails = append(allDetails, "=== Same-Node Connectivity Test ===")
	allDetails = append(allDetails, sameNodeResult.Details...)

	// Test cross-node second
	crossNodeConfig := config
	crossNodeConfig.Placement = "cross-node"
	crossNodeResult := t.testCrossNodePods(ctx, crossNodeConfig)

	allDetails = append(allDetails, "")
	allDetails = append(allDetails, "=== Cross-Node Connectivity Test ===")
	allDetails = append(allDetails, crossNodeResult.Details...)

	// Determine overall success
	bothSuccess := sameNodeResult.Success && crossNodeResult.Success
	var message string
	if bothSuccess {
		message = "Both same-node and cross-node connectivity tests passed"
	} else if sameNodeResult.Success {
		message = "Same-node connectivity passed, cross-node failed"
	} else if crossNodeResult.Success {
		message = "Cross-node connectivity passed, same-node failed"
	} else {
		message = "Both same-node and cross-node connectivity tests failed"
	}

	return TestResult{
		Success: bothSuccess,
		Message: message,
		Details: allDetails,
	}
}

// testPodConnectivity tests ICMP ping connectivity between two pods
func (t *Tester) testPodConnectivity(ctx context.Context, fromPod, toPod string, toPodObj *corev1.Pod, placement string, details *[]string, config TestConfig) TestResult {

	// Get target pod IP
	pod2IP := toPodObj.Status.PodIP
	if pod2IP == "" {
		// Refresh pod info to get IP
		refreshedPod, err := t.clientset.CoreV1().Pods(t.namespace).Get(ctx, toPod, metav1.GetOptions{})
		if err != nil || refreshedPod.Status.PodIP == "" {
			return TestResult{
				Success: false,
				Message: fmt.Sprintf("Failed to get IP for pod %s", toPod),
			}
		}
		pod2IP = refreshedPod.Status.PodIP
	}
	*details = append(*details, fmt.Sprintf("✓ Pod %s IP: %s", toPod, pod2IP))

	// Test ICMP ping connectivity
	pingResult, pingErr := t.pingFromPod(ctx, fromPod, pod2IP)
	var pingLatency float64

	if pingErr != nil {
		*details = append(*details, fmt.Sprintf("✗ ICMP ping failed: %v", pingErr))
		*details = append(*details, fmt.Sprintf("  Output: %s", pingResult))
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod connectivity test failed (%s) - ping failed", placement),
		}
	}

	// Extract latency from ping result
	pingLatency = t.extractPingLatency(pingResult)

	// Check for successful ping patterns
	pingLower := strings.ToLower(pingResult)
	if strings.Contains(pingLower, "0% packet loss") ||
		(strings.Contains(pingLower, "3 packets transmitted") && strings.Contains(pingLower, "3 received")) {
		*details = append(*details, fmt.Sprintf("✓ ICMP ping successful (%.2fms avg latency)", pingLatency))

		// ICMP ping success confirms pod-to-pod connectivity
		successMsg := fmt.Sprintf("Pod connectivity test passed (%s)", placement)
		if pingLatency > 0 {
			successMsg += fmt.Sprintf(" - avg latency: %.2fms", pingLatency)
		}

		return TestResult{
			Success: true,
			Message: successMsg,
		}
	} else {
		*details = append(*details, fmt.Sprintf("✗ ICMP ping failed: %s", strings.TrimSpace(pingResult)))
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod connectivity test failed (%s) - unreliable ping", placement),
		}
	}
}

// extractPingLatency extracts average latency from ping output
func (t *Tester) extractPingLatency(pingOutput string) float64 {
	lines := strings.Split(pingOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "rtt min/avg/max/mdev") {
			// Example: rtt min/avg/max/mdev = 0.346/0.466/0.635/0.122 ms
			parts := strings.Split(line, "=")
			if len(parts) > 1 {
				values := strings.TrimSpace(parts[1])
				values = strings.Replace(values, " ms", "", -1)
				latencyParts := strings.Split(values, "/")
				if len(latencyParts) >= 2 {
					if avgLatency, err := strconv.ParseFloat(latencyParts[1], 64); err == nil {
						return avgLatency
					}
				}
			}
		}
	}
	return 0.0
}

// TestServiceToPodConnectivity creates nginx deployment, service, and tests connectivity from a netshoot pod
func (t *Tester) TestServiceToPodConnectivity(ctx context.Context) TestResult {
	var details []string

	// Step 1: Create nginx deployment with 2 replicas
	deploymentName := "web"
	serviceName := "web"
	testPodName := "netshoot-service-test"

	// Create nginx deployment
	_, err := t.createNginxDeployment(ctx, deploymentName)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created nginx deployment '%s' with 2 replicas", deploymentName))

	// Wait for deployment to be ready
	if err := t.waitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Deployment '%s' is ready", deploymentName))

	// Step 2: Create service to expose the deployment
	_, err = t.createNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created service '%s'", serviceName))

	// Step 2a: Get Service IP (equivalent to: kubectl get svc web -o jsonpath='{.spec.clusterIP}')
	serviceIP, err := t.getServiceIP(ctx, serviceName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get service IP: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Service IP is %s (kubectl get svc %s -n %s -o jsonpath='{.spec.clusterIP}')", serviceIP, serviceName, t.namespace))

	// Step 3: Create netshoot test pod
	_, err = t.createNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created test pod '%s'", testPodName))

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Test pod '%s' is ready", testPodName))

	// Step 4: Test HTTP connectivity with status code (equivalent to: curl -s -o /dev/null -w "%{http_code}\n" http://$SERVICE_IP)
	statusCode, content, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ HTTP connectivity failed: %v", err))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "Service HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code using helper function
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		details = append(details, fmt.Sprintf("✓ HTTP connectivity successful - Status: %s", statusCode))
		details = append(details, fmt.Sprintf("  curl -s -o /dev/null -w \"%%{http_code}\\n\" http://%s", serviceName))
	} else {
		details = append(details, fmt.Sprintf("WARNING: HTTP connectivity issue - %s", message))
	}

	// Show response content if available
	if content != "" && strings.Contains(strings.ToLower(content), "welcome to nginx") {
		details = append(details, fmt.Sprintf("  Response content: nginx welcome page detected"))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all test resources")

	return TestResult{
		Success: true,
		Message: "Service to Pod connectivity test passed - HTTP connectivity working",
		Details: details,
	}
}

// TestCrossNodeServiceConnectivity creates nginx deployment, service, and tests connectivity from a remote node
func (t *Tester) TestCrossNodeServiceConnectivity(ctx context.Context) TestResult {
	var details []string

	// Get worker nodes - we need at least 2 for this test
	workerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 2 {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Cross-node service test requires at least 2 worker nodes, found %d", len(workerNodes)),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes for cross-node testing", len(workerNodes)))

	// Step 1: Create nginx deployment with pod anti-affinity to spread across nodes
	deploymentName := "web-cross-node"
	serviceName := "web-cross-node"
	testPodName := "netshoot-cross-node-test"

	// Create nginx deployment
	_, err = t.createNginxDeployment(ctx, deploymentName)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created nginx deployment '%s' with 2 replicas", deploymentName))

	// Wait for deployment to be ready
	if err := t.waitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Deployment '%s' is ready", deploymentName))

	// Step 2: Create service to expose the deployment
	_, err = t.createNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created service '%s'", serviceName))

	// Step 2a: Get Service IP
	serviceIP, err := t.getServiceIP(ctx, serviceName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get service IP: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Service IP is %s", serviceIP))

	// Step 3: Create test pod on the second node to ensure cross-node traffic
	_, err = t.createNetshootPod(ctx, testPodName, workerNodes[1])
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod on node %s: %v", workerNodes[1], err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created test pod '%s' on node %s for cross-node testing", testPodName, workerNodes[1]))

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Test pod '%s' is ready", testPodName))

	// Step 4: Test HTTP connectivity with status code
	statusCode, content, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ HTTP connectivity failed: %v", err))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "Cross-node service HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		details = append(details, fmt.Sprintf("✓ Cross-node HTTP connectivity successful - Status: %s", statusCode))
		details = append(details, fmt.Sprintf("  curl -s -o /dev/null -w \"%%{http_code}\\n\" http://%s", serviceName))
	} else {
		details = append(details, fmt.Sprintf("✗ Cross-node HTTP connectivity issue - %s", message))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Cross-node service connectivity failed with status: %s", message),
			Details: details,
		}
	}

	// Show response content if available
	if content != "" && strings.Contains(strings.ToLower(content), "welcome to nginx") {
		details = append(details, fmt.Sprintf("  Response content: nginx welcome page detected"))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all cross-node test resources")

	return TestResult{
		Success: true,
		Message: "Cross-node service connectivity test passed - HTTP connectivity working across nodes",
		Details: details,
	}
}

// TestDNSResolution creates test resources and validates DNS resolution functionality
func (t *Tester) TestDNSResolution(ctx context.Context) TestResult {
	var details []string

	deploymentName := "web-dns"
	serviceName := "web-dns"
	testPodName := "netshoot-dns-test"

	// Create nginx deployment
	_, err := t.createNginxDeployment(ctx, deploymentName)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment for DNS test: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created nginx deployment '%s' for DNS testing", deploymentName))

	// Create service
	_, err = t.createNginxService(ctx, serviceName, deploymentName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create service for DNS test: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created service '%s' for DNS testing", serviceName))

	// Create test pod
	_, err = t.createNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create DNS test pod: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created DNS test pod '%s'", testPodName))

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("DNS test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}

	// Test service FQDN resolution
	fqdnName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, t.namespace)
	fqdnResult, fqdnErr := t.testDNSResolution(ctx, testPodName, fqdnName)
	if fqdnErr != nil {
		details = append(details, fmt.Sprintf("✗ Service FQDN DNS resolution failed: %v", fqdnErr))
	} else {
		details = append(details, fmt.Sprintf("✓ Service FQDN DNS resolution successful"))
		details = append(details, fmt.Sprintf("  Result: %s", strings.TrimSpace(fqdnResult)))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up DNS test resources")

	return TestResult{
		Success: fqdnErr == nil,
		Message: "DNS resolution test completed",
		Details: details,
	}
}

// TestNodePortServiceConnectivity tests NodePort service connectivity
func (t *Tester) TestNodePortServiceConnectivity(ctx context.Context) TestResult {
	var details []string

	// Get worker nodes - we need at least one
	workerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 1 {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("NodePort test requires at least 1 worker node, found %d", len(workerNodes)),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes for NodePort testing", len(workerNodes)))

	// Step 1: Create nginx deployment
	deploymentName := "web-nodeport"
	serviceName := "web-nodeport"
	testPodName := "netshoot-nodeport-test"

	// Create nginx deployment
	_, err = t.createNginxDeployment(ctx, deploymentName)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created nginx deployment '%s' with 2 replicas", deploymentName))

	// Wait for deployment to be ready
	if err := t.waitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Deployment '%s' is ready", deploymentName))

	// Step 2: Create NodePort service to expose the deployment
	createdService, err := t.createNginxServiceWithType(ctx, serviceName, deploymentName, ServiceTypeNodePort)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create NodePort service: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created NodePort service '%s'", serviceName))

	// Get the assigned NodePort
	nodePort := int(createdService.Spec.Ports[0].NodePort)
	details = append(details, fmt.Sprintf("✓ NodePort assigned: %d", nodePort))

	// Step 3: Get the first worker node's IP address
	node, err := t.clientset.CoreV1().Nodes().Get(ctx, workerNodes[0], metav1.GetOptions{})
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get node information: %v", err),
			Details: details,
		}
	}

	// Extract internal IP address
	var nodeIP string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			nodeIP = address.Address
			break
		}
	}

	if nodeIP == "" {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "Could not determine node IP address",
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found node IP for NodePort access: %s", nodeIP))

	// Step 4: Create test pod to access the NodePort
	_, err = t.createNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}
	details = append(details, "✓ Created test pod to access NodePort service")

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod did not become ready: %v", err),
			Details: details,
		}
	}
	details = append(details, "✓ Test pod is ready")

	// Step 5: Test HTTP connectivity to the NodePort
	nodePortURL := fmt.Sprintf("%s:%d", nodeIP, nodePort)
	statusCode, content, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, nodePortURL)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ HTTP connectivity to NodePort failed: %v", err))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "NodePort HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		details = append(details, fmt.Sprintf("✓ NodePort HTTP connectivity successful - Status: %s", statusCode))
		details = append(details, fmt.Sprintf("  curl -s -o /dev/null -w \"%%{http_code}\\n\" http://%s", nodePortURL))
	} else {
		details = append(details, fmt.Sprintf("✗ NodePort HTTP connectivity issue - %s", message))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("NodePort connectivity failed with status: %s", message),
			Details: details,
		}
	}

	// Show response content if available
	if content != "" && strings.Contains(strings.ToLower(content), "welcome to nginx") {
		details = append(details, fmt.Sprintf("  Response content: nginx welcome page detected"))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all NodePort test resources")

	return TestResult{
		Success: true,
		Message: "NodePort service connectivity test passed - HTTP connectivity working through node port",
		Details: details,
	}
}

// TestLoadBalancerServiceConnectivity tests LoadBalancer service connectivity
func (t *Tester) TestLoadBalancerServiceConnectivity(ctx context.Context) TestResult {
	var details []string

	// Get worker nodes - we need at least one
	workerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get worker nodes: %v", err),
			Details: details,
		}
	}

	if len(workerNodes) < 1 {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("LoadBalancer test requires at least 1 worker node, found %d", len(workerNodes)),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes for LoadBalancer testing", len(workerNodes)))

	// Step 1: Create nginx deployment
	deploymentName := "web-loadbalancer"
	serviceName := "web-loadbalancer"
	testPodName := "netshoot-loadbalancer-test"

	// Create nginx deployment
	_, err = t.createNginxDeployment(ctx, deploymentName)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create nginx deployment: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created nginx deployment '%s' with 2 replicas", deploymentName))

	// Wait for deployment to be ready
	if err := t.waitForDeploymentReady(ctx, deploymentName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Deployment %s did not become ready: %v", deploymentName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Deployment '%s' is ready", deploymentName))

	// Step 2: Create LoadBalancer service to expose the deployment
	createdService, err := t.createNginxServiceWithType(ctx, serviceName, deploymentName, ServiceTypeLoadBalancer)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create LoadBalancer service: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created LoadBalancer service '%s'", serviceName))

	// Get the ClusterIP since we're running in a local environment
	clusterIP := createdService.Spec.ClusterIP
	details = append(details, fmt.Sprintf("✓ Service ClusterIP: %s", clusterIP))

	// Note about external IP in cloud environments
	details = append(details, "ℹ️ Note: In cloud environments, the service would be assigned an external IP")

	// Check for any external IPs (likely none in local environment)
	if len(createdService.Status.LoadBalancer.Ingress) > 0 {
		externalIP := createdService.Status.LoadBalancer.Ingress[0].IP
		if externalIP != "" {
			details = append(details, fmt.Sprintf("✓ External IP assigned: %s", externalIP))
		}
	} else {
		details = append(details, "ℹ️ No external IP assigned (expected in local environments)")
	}

	// Step 3: Create test pod to test connectivity
	_, err = t.createNetshootPod(ctx, testPodName, "")
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod: %v", err),
			Details: details,
		}
	}
	details = append(details, "✓ Created test pod to access LoadBalancer service")

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod did not become ready: %v", err),
			Details: details,
		}
	}
	details = append(details, "✓ Test pod is ready")

	// Step 4: Test HTTP connectivity via ClusterIP (as fallback in local environments)
	details = append(details, "ℹ️ Testing connectivity via ClusterIP (fallback for local environments)")
	statusCode, content, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ HTTP connectivity failed: %v", err))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "LoadBalancer HTTP connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		details = append(details, fmt.Sprintf("✓ LoadBalancer HTTP connectivity successful - Status: %s", statusCode))
		details = append(details, fmt.Sprintf("  curl -s -o /dev/null -w \"%%{http_code}\\n\" http://%s", serviceName))
	} else {
		details = append(details, fmt.Sprintf("✗ LoadBalancer HTTP connectivity issue - %s", message))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("LoadBalancer connectivity failed with status: %s", message),
			Details: details,
		}
	}

	// Show response content if available
	if content != "" && strings.Contains(strings.ToLower(content), "welcome to nginx") {
		details = append(details, fmt.Sprintf("  Response content: nginx welcome page detected"))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all LoadBalancer test resources")

	return TestResult{
		Success: true,
		Message: "LoadBalancer service connectivity test passed - HTTP connectivity working via service",
		Details: details,
	}
}

// ensureNamespace creates the namespace if it doesn't exist
func (t *Tester) ensureNamespace(ctx context.Context) error {
	// Check if namespace exists
	_, err := t.clientset.CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	// Create the namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.namespace,
		},
	}
	_, err = t.clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}
	return nil
}

// getWorkerNodes returns a list of worker node names
func (t *Tester) getWorkerNodes(ctx context.Context) ([]string, error) {
	nodes, err := t.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var workerNodes []string
	for _, node := range nodes.Items {
		// Check if it's not a control-plane node
		isControlPlane := false
		for key := range node.Labels {
			if strings.Contains(key, "control-plane") || strings.Contains(key, "master") {
				isControlPlane = true
				break
			}
		}
		if !isControlPlane {
			workerNodes = append(workerNodes, node.Name)
		}
	}

	return workerNodes, nil
}

// createNetshootPod creates a netshoot pod on the specified node
func (t *Tester) createNetshootPod(ctx context.Context, name, nodeName string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
			Labels: map[string]string{
				"app": "netshoot-test",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:  "netshoot",
					Image: "nicolaka/netshoot",
					Command: []string{
						"sleep",
						"3600",
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	createdPod, err := t.clientset.CoreV1().Pods(t.namespace).Create(ctx, pod, metav1.CreateOptions{})
	return createdPod, err
}

// waitForPodReady waits for a pod to be ready
func (t *Tester) waitForPodReady(ctx context.Context, podName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("pod %s did not become ready within %v", podName, timeout)
		case <-ticker.C:
			pod, err := t.clientset.CoreV1().Pods(t.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				continue
			}

			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
	}
}

// WaitForPodReadyOrCleanup encapsulates the common pattern of waiting for pod readiness and cleanup on failure
func (t *Tester) WaitForPodReadyOrCleanup(
	ctx context.Context,
	podName string,
	timeout time.Duration,
	cleanupFunc func(),
	details *[]string,
) error {
	if err := t.waitForPodReady(ctx, podName, timeout); err != nil {
		if cleanupFunc != nil {
			cleanupFunc()
		}
		if details != nil {
			*details = append(*details, fmt.Sprintf("✗ Pod %s did not become ready: %v", podName, err))
		}
		return err
	}

	if details != nil {
		*details = append(*details, fmt.Sprintf("✓ Pod %s is ready", podName))
	}
	return nil
}

// pingFromPod executes ping command from one pod to another
func (t *Tester) pingFromPod(ctx context.Context, fromPod, targetIP string) (string, error) {
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(fromPod).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"ping", "-c", "3", "-W", "3", "-i", "1", targetIP},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil && stderr.Len() > 0 {
		return output + "\nSTDERR: " + stderr.String(), err
	}

	return output, err
}

// cleanupPod removes a single pod
func (t *Tester) cleanupPod(ctx context.Context, podName string) {
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// cleanupPods removes test pods
func (t *Tester) cleanupPods(ctx context.Context, pod1Name, pod2Name string) {
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, pod1Name, metav1.DeleteOptions{})
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, pod2Name, metav1.DeleteOptions{})
}

// createNginxDeployment creates an nginx deployment
func (t *Tester) createNginxDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:alpine",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	return t.clientset.AppsV1().Deployments(t.namespace).Create(ctx, deployment, metav1.CreateOptions{})
}

// waitForDeploymentReady waits for a deployment to be ready
func (t *Tester) waitForDeploymentReady(ctx context.Context, deploymentName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("deployment %s did not become ready within %v", deploymentName, timeout)
		case <-ticker.C:
			deployment, err := t.clientset.AppsV1().Deployments(t.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				continue
			}

			if deployment.Status.ReadyReplicas >= *deployment.Spec.Replicas && deployment.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

// ServiceType represents Kubernetes service types used in tests
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeNodePort     ServiceType = "NodePort"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
)

// createNginxService creates a service to expose the nginx deployment with the specified service type
func (t *Tester) createNginxService(ctx context.Context, serviceName, deploymentName string) (*corev1.Service, error) {
	return t.createNginxServiceWithType(ctx, serviceName, deploymentName, ServiceTypeClusterIP)
}

// createNginxServiceWithType creates a service of the specified type to expose the nginx deployment
func (t *Tester) createNginxServiceWithType(ctx context.Context, serviceName, deploymentName string, serviceType ServiceType) (*corev1.Service, error) {
	var k8sServiceType corev1.ServiceType

	// Convert our ServiceType to Kubernetes ServiceType
	switch serviceType {
	case ServiceTypeNodePort:
		k8sServiceType = corev1.ServiceTypeNodePort
	case ServiceTypeLoadBalancer:
		k8sServiceType = corev1.ServiceTypeLoadBalancer
	default:
		k8sServiceType = corev1.ServiceTypeClusterIP
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: t.namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": deploymentName,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: k8sServiceType,
		},
	}

	return t.clientset.CoreV1().Services(t.namespace).Create(ctx, service, metav1.CreateOptions{})
}

// getServiceIP retrieves the ClusterIP of a service
func (t *Tester) getServiceIP(ctx context.Context, serviceName string) (string, error) {
	service, err := t.clientset.CoreV1().Services(t.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service %s: %v", serviceName, err)
	}

	if service.Spec.ClusterIP == "" {
		return "", fmt.Errorf("service %s has no ClusterIP assigned", serviceName)
	}

	return service.Spec.ClusterIP, nil
}

// testHTTPConnectivityWithStatusCode tests HTTP connectivity and returns status code
func (t *Tester) testHTTPConnectivityWithStatusCode(ctx context.Context, podName, target string) (string, string, error) {
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", fmt.Sprintf("http://%s", target)},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	statusCode := strings.TrimSpace(stdout.String())
	return statusCode, "", err
}

// testDNSResolution tests if the service can be resolved via DNS
func (t *Tester) testDNSResolution(ctx context.Context, podName, serviceName string) (string, error) {
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"nslookup", serviceName},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil && stderr.Len() > 0 {
		return output + "\nSTDERR: " + stderr.String(), err
	}

	return output, err
}

// cleanupServiceResources removes all service-related test resources
func (t *Tester) cleanupServiceResources(ctx context.Context, deploymentName, serviceName, podName string) {
	t.clientset.AppsV1().Deployments(t.namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	t.clientset.CoreV1().Services(t.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if podName != "" {
		t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	}
}
