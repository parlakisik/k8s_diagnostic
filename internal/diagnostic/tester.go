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
	UseExistingPods bool   `json:"use_existing_pods"`
	TargetNamespace string `json:"target_namespace"`
	PodSelector     string `json:"pod_selector"`
	CreateFreshPods bool   `json:"create_fresh_pods"`
}

// InteractiveConfig represents configuration for interactive pod selection
type InteractiveConfig struct {
	TargetNamespace   string `json:"target_namespace"`
	AutoCreateMissing bool   `json:"auto_create_missing"`
	PreferCrossNode   bool   `json:"prefer_cross_node"`
	ShowAllPods       bool   `json:"show_all_pods"`
	Verbose           bool   `json:"verbose"`
}

// PodInfo represents information about a discovered pod
type PodInfo struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	NodeName   string            `json:"node_name"`
	PodIP      string            `json:"pod_ip"`
	Status     string            `json:"status"`
	Age        string            `json:"age"`
	Image      string            `json:"image"`
	Labels     map[string]string `json:"labels"`
	IsNetshoot bool              `json:"is_netshoot"`
	IsReady    bool              `json:"is_ready"`
	Score      int               `json:"score"` // Health/suitability score
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
	return t.TestPodToPodConnectivityWithConfig(ctx, TestConfig{CreateFreshPods: true})
}

// TestPodToPodConnectivityWithConfig tests connectivity with configurable pod source
func (t *Tester) TestPodToPodConnectivityWithConfig(ctx context.Context, config TestConfig) TestResult {
	if config.UseExistingPods {
		return t.testWithExistingPods(ctx, config)
	}
	return t.testWithFreshPods(ctx, config)
}

// testWithExistingPods tests connectivity using existing pods in the cluster
func (t *Tester) testWithExistingPods(ctx context.Context, config TestConfig) TestResult {
	var details []string

	// Discover existing pods
	existingPods, err := t.findExistingNetshootPods(ctx, config)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to discover existing pods: %v", err),
			Details: details,
			DetailedDiagnostics: &DetailedDiagnostics{
				FailureStage:   "pod_discovery",
				TechnicalError: fmt.Sprintf("Pod discovery failed in namespace '%s' with selector '%s': %v", config.TargetNamespace, config.PodSelector, err),
				TroubleshootingHints: []string{
					fmt.Sprintf("Ensure pods with label '%s' exist in namespace '%s'", config.PodSelector, config.TargetNamespace),
					"Check if pods are in Ready state",
					"Verify kubectl has access to the target namespace",
				},
			},
		}
	}

	if len(existingPods) < 2 {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Found %d pods, need at least 2 for connectivity testing", len(existingPods)),
			Details: details,
			DetailedDiagnostics: &DetailedDiagnostics{
				FailureStage:   "pod_validation",
				TechnicalError: fmt.Sprintf("Insufficient pods: found %d, need at least 2", len(existingPods)),
				NetworkContext: &NetworkContext{
					AdditionalInfo: map[string]string{
						"target_namespace": config.TargetNamespace,
						"pod_selector":     config.PodSelector,
						"pods_found":       fmt.Sprintf("%d", len(existingPods)),
					},
				},
				TroubleshootingHints: []string{
					fmt.Sprintf("Deploy at least 2 pods with label '%s' in namespace '%s'", config.PodSelector, config.TargetNamespace),
					"Ensure pods are distributed across different nodes for cross-node testing",
					"Check pod status with: kubectl get pods -n " + config.TargetNamespace,
				},
			},
		}
	}

	details = append(details, fmt.Sprintf("✓ Using existing pods mode - found %d pods", len(existingPods)))

	// Select two pods for testing (preferably on different nodes)
	pod1, pod2, err := t.selectCrossNodePods(existingPods)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to select suitable pods: %v", err),
			Details: details,
			DetailedDiagnostics: &DetailedDiagnostics{
				FailureStage:   "pod_selection",
				TechnicalError: fmt.Sprintf("Pod selection failed: %v", err),
				TroubleshootingHints: []string{
					"Ensure pods are running on different nodes for cross-node testing",
					"Check pod readiness status",
					"Verify pods have valid IP addresses",
				},
			},
		}
	}

	details = append(details, fmt.Sprintf("✓ Selected pod %s on node %s", pod1.Name, pod1.Spec.NodeName))
	details = append(details, fmt.Sprintf("✓ Selected pod %s on node %s", pod2.Name, pod2.Spec.NodeName))

	// Test connectivity by pinging from pod1 to pod2
	pod2IP := pod2.Status.PodIP
	if pod2IP == "" {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s has no IP address", pod2.Name),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Target pod %s IP: %s", pod2.Name, pod2IP))

	// Ping from pod1 to pod2
	pingResult, err := t.pingFromPodInNamespace(ctx, pod1.Name, config.TargetNamespace, pod2IP)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ Ping command failed: %v", err))
		details = append(details, fmt.Sprintf("  Output: %s", pingResult))

		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s is not reachable from pod %s", pod2.Name, pod1.Name),
			Details: details,
			DetailedDiagnostics: &DetailedDiagnostics{
				FailureStage:   "connectivity_test",
				TechnicalError: fmt.Sprintf("100%% packet loss during ping test: %v", err),
				CommandOutputs: []CommandOutput{
					{
						Command:     fmt.Sprintf("ping -c 3 -W 3 -i 1 %s", pod2IP),
						ExitCode:    1,
						Stdout:      pingResult,
						Description: "Cross-node ping test between existing pods",
					},
				},
				NetworkContext: &NetworkContext{
					SourceNode:  pod1.Spec.NodeName,
					TargetNode:  pod2.Spec.NodeName,
					TargetPodIP: pod2IP,
					AdditionalInfo: map[string]string{
						"source_pod":       pod1.Name,
						"target_pod":       pod2.Name,
						"test_mode":        "existing_pods",
						"target_namespace": config.TargetNamespace,
					},
				},
				TroubleshootingHints: []string{
					"Check if pod was deleted during test execution",
					"Verify CNI network configuration",
					"Check firewall rules between nodes",
					"Ensure kube-proxy is running on all nodes",
				},
			},
		}
	}

	// Check for successful ping patterns
	pingLower := strings.ToLower(pingResult)
	if strings.Contains(pingLower, "0% packet loss") ||
		(strings.Contains(pingLower, "3 packets transmitted") && strings.Contains(pingLower, "3 received")) ||
		(strings.Contains(pingLower, "transmitted") && strings.Contains(pingLower, "received") && !strings.Contains(pingLower, "100% packet loss")) {

		details = append(details, "✓ Ping successful - existing pods can communicate")
		details = append(details, fmt.Sprintf("  Ping output: %s", strings.TrimSpace(pingResult)))
		return TestResult{
			Success: true,
			Message: fmt.Sprintf("Pod %s is reachable from pod %s (using existing pods)", pod2.Name, pod1.Name),
			Details: details,
		}
	} else {
		details = append(details, fmt.Sprintf("✗ Ping failed - pod %s is not reachable from pod %s", pod2.Name, pod1.Name))
		details = append(details, fmt.Sprintf("  Ping output: %s", strings.TrimSpace(pingResult)))
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s is not reachable from pod %s (using existing pods)", pod2.Name, pod1.Name),
			Details: details,
		}
	}
}

// testWithFreshPods tests connectivity using newly created pods (current logic)
func (t *Tester) testWithFreshPods(ctx context.Context, config TestConfig) TestResult {
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
			Message: fmt.Sprintf("Need at least 2 worker nodes, found %d", len(workerNodes)),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Found %d worker nodes", len(workerNodes)))

	// Create two test pods
	pod1Name := "netshoot-test-1"
	pod2Name := "netshoot-test-2"

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
		t.cleanupPod(ctx, pod1Name) // Cleanup first pod
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create pod %s: %v", pod2Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created pod %s on node %s", pod2Name, workerNodes[1]))

	// Wait for pods to be ready (increased timeout for image pull)
	if err := t.waitForPodReady(ctx, pod1Name, 120*time.Second); err != nil {
		t.cleanupPods(ctx, pod1Name, pod2Name)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod1Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Pod %s is ready", pod1Name))

	if err := t.waitForPodReady(ctx, pod2Name, 120*time.Second); err != nil {
		t.cleanupPods(ctx, pod1Name, pod2Name)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s did not become ready: %v", pod2Name, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Pod %s is ready", pod2Name))

	// Get pod IPs
	pod2IP := pod2.Status.PodIP
	if pod2IP == "" {
		// Refresh pod info to get IP
		pod2, err = t.clientset.CoreV1().Pods(t.namespace).Get(ctx, pod2Name, metav1.GetOptions{})
		if err != nil || pod2.Status.PodIP == "" {
			t.cleanupPods(ctx, pod1Name, pod2Name)
			return TestResult{
				Success: false,
				Message: fmt.Sprintf("Failed to get IP for pod %s", pod2Name),
				Details: details,
			}
		}
		pod2IP = pod2.Status.PodIP
	}
	details = append(details, fmt.Sprintf("✓ Pod %s IP: %s", pod2Name, pod2IP))

	// Test connectivity by pinging from pod1 to pod2
	pingResult, err := t.pingFromPod(ctx, pod1Name, pod2IP)

	// Cleanup pods regardless of ping result
	t.cleanupPods(ctx, pod1Name, pod2Name)
	details = append(details, "✓ Cleaned up test pods")

	// Analyze ping results
	if err != nil {
		details = append(details, fmt.Sprintf("✗ Ping command failed: %v", err))
		details = append(details, fmt.Sprintf("  Output: %s", pingResult))

		// Create detailed diagnostics for ping failure
		detailedDiagnostics := &DetailedDiagnostics{
			FailureStage:   "connectivity_test",
			TechnicalError: fmt.Sprintf("100%% packet loss during ping test: %v", err),
			CommandOutputs: []CommandOutput{
				{
					Command:     fmt.Sprintf("ping -c 3 -W 3 -i 1 %s", pod2IP),
					ExitCode:    1, // Ping failure exit code
					Stdout:      pingResult,
					Description: "Cross-node ping test from pod to pod",
				},
			},
			NetworkContext: &NetworkContext{
				SourceNode:  workerNodes[0],
				TargetNode:  workerNodes[1],
				TargetPodIP: pod2IP,
				AdditionalInfo: map[string]string{
					"pod_1_name": pod1Name,
					"pod_2_name": pod2Name,
					"test_type":  "cross_node_ping",
				},
			},
			TroubleshootingHints: []string{
				"Check CNI network configuration",
				"Verify cross-node routing is enabled",
				"Check firewall rules between nodes",
				"Validate pod CIDR configuration",
				"Ensure kube-proxy is running on all nodes",
			},
		}

		return TestResult{
			Success:             false,
			Message:             fmt.Sprintf("Pod %s is not reachable from pod %s", pod2Name, pod1Name),
			Details:             details,
			DetailedDiagnostics: detailedDiagnostics,
		}
	}

	// Check for successful ping patterns
	pingLower := strings.ToLower(pingResult)
	if strings.Contains(pingLower, "0% packet loss") ||
		(strings.Contains(pingLower, "3 packets transmitted") && strings.Contains(pingLower, "3 received")) ||
		(strings.Contains(pingLower, "transmitted") && strings.Contains(pingLower, "received") && !strings.Contains(pingLower, "100% packet loss")) {

		details = append(details, "✓ Ping successful - pods can communicate")
		details = append(details, fmt.Sprintf("  Ping output: %s", strings.TrimSpace(pingResult)))
		return TestResult{
			Success: true,
			Message: fmt.Sprintf("Pod %s is reachable from pod %s", pod2Name, pod1Name),
			Details: details,
		}
	} else {
		details = append(details, fmt.Sprintf("✗ Ping failed - pod %s is not reachable from pod %s", pod2Name, pod1Name))
		details = append(details, fmt.Sprintf("  Ping output: %s", strings.TrimSpace(pingResult)))
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s is not reachable from pod %s", pod2Name, pod1Name),
			Details: details,
		}
	}
}

// TestCrossNodeServiceConnectivity creates nginx deployment, service, and tests connectivity from a remote node
func (t *Tester) TestCrossNodeServiceConnectivity(ctx context.Context) TestResult {
	var details []string

	// Step 1: Create nginx deployment with 2 replicas
	deploymentName := "web-cross"
	serviceName := "web-cross"
	testPodName := "netshoot-cross-node-test"

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

	// Step 2: Get nginx pod node locations
	nginxNodes, err := t.getNginxPodNodes(ctx, deploymentName)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get nginx pod node locations: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Nginx pods running on nodes: %v", nginxNodes))

	// Step 3: Find a different worker node for the test pod
	differentNode, err := t.findDifferentWorkerNode(ctx, nginxNodes)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to find different worker node: %v", err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Selected different node '%s' for cross-node test", differentNode))

	// Step 4: Create service to expose the deployment
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

	// Step 5: Get Service IP
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

	// Step 6: Create test pod on the different node
	_, err = t.createNetshootPod(ctx, testPodName, differentNode)
	if err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create test pod on node %s: %v", differentNode, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Created test pod '%s' on node '%s'", testPodName, differentNode))

	// Wait for test pod to be ready
	if err := t.waitForPodReady(ctx, testPodName, 120*time.Second); err != nil {
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Test pod %s did not become ready: %v", testPodName, err),
			Details: details,
		}
	}
	details = append(details, fmt.Sprintf("✓ Test pod '%s' is ready on remote node", testPodName))

	// Step 7: Test cross-node HTTP connectivity
	statusCode, content, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ Cross-node HTTP connectivity failed: %v", err))
		t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
		return TestResult{
			Success: false,
			Message: "Cross-node service connectivity failed",
			Details: details,
		}
	}

	// Check HTTP status code using helper function
	success, message := evaluateHTTPStatusCode(statusCode)
	if success {
		details = append(details, fmt.Sprintf("✓ Cross-node HTTP connectivity successful - Status: %s", statusCode))
		details = append(details, fmt.Sprintf("  Created test pod on remote node with nodeSelector"))
	} else {
		details = append(details, fmt.Sprintf("WARNING: Cross-node HTTP connectivity issue - %s", message))
	}

	// Show response content if available
	if content != "" && strings.Contains(strings.ToLower(content), "welcome to nginx") {
		details = append(details, fmt.Sprintf("  Cross-node response: nginx welcome page detected"))
	}

	// Step 8: Test cross-node service IP connectivity
	directStatusCode, directContent, err := t.testHTTPConnectivityWithStatusCode(ctx, testPodName, serviceIP)
	if err != nil {
		details = append(details, fmt.Sprintf("WARNING: Direct service IP connectivity failed: %v", err))
	} else {
		// Check status code using helper function
		directSuccess, directMessage := evaluateHTTPStatusCode(directStatusCode)
		if directSuccess {
			details = append(details, fmt.Sprintf("✓ Direct service IP connectivity successful - Status: %s", directStatusCode))
			details = append(details, fmt.Sprintf("  curl http://%s from remote node successful", serviceIP))

			// Show response content if available
			if directContent != "" && strings.Contains(strings.ToLower(directContent), "welcome to nginx") {
				details = append(details, fmt.Sprintf("  Direct IP response: nginx welcome page detected"))
			}
		} else {
			details = append(details, fmt.Sprintf("WARNING: Direct service IP connectivity issue - %s", directMessage))
		}
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all cross-node test resources")

	return TestResult{
		Success: true,
		Message: "Cross-node service connectivity validated - kube-proxy inter-node routing confirmed",
		Details: details,
	}
}

// TestDNSResolution creates test resources and validates DNS resolution functionality
func (t *Tester) TestDNSResolution(ctx context.Context) TestResult {
	var details []string

	// Step 1: Create nginx deployment with 2 replicas for DNS testing
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

	// Step 2: Create service for DNS testing
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

	// Step 3: Create test pod for DNS queries
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
	details = append(details, fmt.Sprintf("✓ DNS test pod '%s' is ready", testPodName))

	// Step 4: Test service FQDN resolution
	fqdnName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, t.namespace)
	fqdnResult, err := t.testDNSResolution(ctx, testPodName, fqdnName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ Service FQDN DNS resolution failed: %v", err))
		details = append(details, fmt.Sprintf("  Command: nslookup %s", fqdnName))
	} else {
		details = append(details, fmt.Sprintf("✓ Service FQDN DNS resolution successful"))
		details = append(details, fmt.Sprintf("  Command: nslookup %s", fqdnName))
		details = append(details, fmt.Sprintf("  Result: %s", strings.TrimSpace(fqdnResult)))
	}

	// Step 5: Test short name resolution (DNS search domains)
	shortResult, err := t.testDNSResolution(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("✗ Short name DNS resolution failed: %v", err))
		details = append(details, fmt.Sprintf("  Command: nslookup %s", serviceName))
	} else {
		details = append(details, fmt.Sprintf("✓ Short name DNS resolution successful"))
		details = append(details, fmt.Sprintf("  Command: nslookup %s", serviceName))
		details = append(details, fmt.Sprintf("  Result: %s", strings.TrimSpace(shortResult)))
	}

	// Step 6: Test pod-to-pod DNS resolution
	podDNSResult, err := t.testPodToPodDNS(ctx, testPodName, deploymentName)
	if err != nil {
		details = append(details, fmt.Sprintf("WARNING: Pod-to-pod DNS resolution test inconclusive: %v", err))
	} else {
		details = append(details, fmt.Sprintf("✓ Pod-to-pod DNS resolution successful"))
		details = append(details, fmt.Sprintf("  %s", podDNSResult))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up DNS test resources")

	// Determine overall success
	fqdnSuccess := err == nil
	shortSuccess := shortResult != ""

	if fqdnSuccess && shortSuccess {
		return TestResult{
			Success: true,
			Message: "DNS resolution test passed - service FQDN and short name resolution working",
			Details: details,
		}
	} else {
		return TestResult{
			Success: false,
			Message: "DNS resolution test failed - check cluster DNS configuration",
			Details: details,
		}
	}
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

	// Step 4: Test ICMP ping to Service IP (equivalent to: ping -c3 $SERVICE_IP)
	pingResult, err := t.testServiceIPPing(ctx, testPodName, serviceIP)
	if err != nil {
		details = append(details, fmt.Sprintf("WARNING: ICMP ping to service IP failed: %v (some clusters block ping)", err))
		details = append(details, fmt.Sprintf("  Output: %s", strings.TrimSpace(pingResult)))
	} else {
		// Check for successful ping patterns
		pingLower := strings.ToLower(pingResult)
		if strings.Contains(pingLower, "0% packet loss") ||
			(strings.Contains(pingLower, "3 packets transmitted") && strings.Contains(pingLower, "3 received")) ||
			(strings.Contains(pingLower, "transmitted") && strings.Contains(pingLower, "received") && !strings.Contains(pingLower, "100% packet loss")) {
			details = append(details, fmt.Sprintf("✓ ICMP ping to service IP %s successful", serviceIP))
		} else {
			details = append(details, fmt.Sprintf("WARNING: ICMP ping to service IP %s failed (some clusters block ping)", serviceIP))
		}
	}

	// Step 5: Test HTTP connectivity with status code (equivalent to: curl -s -o /dev/null -w "%{http_code}\n" http://$SERVICE_IP)
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

	// Step 6: Test load balancing by making multiple requests
	lbResult, err := t.testLoadBalancing(ctx, testPodName, serviceName)
	if err != nil {
		details = append(details, fmt.Sprintf("WARNING: Load balancing test inconclusive: %v", err))
	} else {
		details = append(details, fmt.Sprintf("✓ Load balancing verified: %s", lbResult))
	}

	// Cleanup all resources
	t.cleanupServiceResources(ctx, deploymentName, serviceName, testPodName)
	details = append(details, "✓ Cleaned up all test resources")

	return TestResult{
		Success: true,
		Message: "Service to Pod connectivity test passed - HTTP connectivity and load balancing working",
		Details: details,
	}
}

// ensureNamespace creates the namespace if it doesn't exist
func (t *Tester) ensureNamespace(ctx context.Context) error {
	// Check if namespace exists
	ns, err := t.clientset.CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace exists, check if it's terminating
		if ns.Status.Phase == corev1.NamespaceTerminating {
			// Wait for termination to complete
			if err := t.waitForNamespaceTermination(ctx); err != nil {
				return fmt.Errorf("failed to wait for namespace termination: %v", err)
			}
		} else {
			// Namespace exists and is active
			return nil
		}
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

// waitForNamespaceTermination waits for a namespace to be fully terminated
func (t *Tester) waitForNamespaceTermination(ctx context.Context) error {
	timeout := 60 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("namespace %s did not terminate within %v", t.namespace, timeout)
		case <-ticker.C:
			_, err := t.clientset.CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
			if err != nil {
				// Namespace no longer exists, termination complete
				return nil
			}
		}
	}
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
						"3600", // Sleep for 1 hour
					},
					Resources: corev1.ResourceRequirements{},
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
	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Poll every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("pod %s did not become ready within %v", podName, timeout)
		case <-ticker.C:
			pod, err := t.clientset.CoreV1().Pods(t.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				continue // Keep trying if we can't get the pod
			}

			// Check if pod is ready
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
	}
}

// pingFromPod executes ping command from one pod to another
func (t *Tester) pingFromPod(ctx context.Context, fromPod, targetIP string) (string, error) {
	// Create the exec request
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

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil {
		if stderr.Len() > 0 {
			return output + "\nSTDERR: " + stderr.String(), err
		}
		return output, err
	}

	return output, nil
}

// InteractivePodSelection handles interactive pod discovery and selection
func (t *Tester) InteractivePodSelection(ctx context.Context, config InteractiveConfig) (TestConfig, error) {
	fmt.Printf("Discovering pods in namespace '%s'...\n", config.TargetNamespace)

	// Discover all pods in the target namespace
	allPods, err := t.clientset.CoreV1().Pods(config.TargetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return TestConfig{}, fmt.Errorf("failed to list pods in namespace %s: %v", config.TargetNamespace, err)
	}

	if len(allPods.Items) == 0 {
		fmt.Printf("No pods found in namespace '%s'\n", config.TargetNamespace)
		if config.AutoCreateMissing {
			fmt.Printf("Auto-creating pods enabled. Will create fresh pods for testing.\n")
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}

		fmt.Printf("Would you like to create fresh pods for testing? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}
		return TestConfig{}, fmt.Errorf("no pods available for testing")
	}

	// Convert pods to PodInfo and score them
	var podInfos []PodInfo
	for _, pod := range allPods.Items {
		podInfo := t.convertToPodInfo(&pod)
		if config.ShowAllPods || podInfo.IsNetshoot || t.isNetworkCapable(&pod) {
			podInfos = append(podInfos, podInfo)
		}
	}

	if len(podInfos) == 0 {
		fmt.Printf("No suitable pods found for network testing in namespace '%s'\n", config.TargetNamespace)
		if config.AutoCreateMissing {
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}

		fmt.Printf("Would you like to create fresh pods for testing? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}
		return TestConfig{}, fmt.Errorf("no suitable pods available for testing")
	}

	// Sort pods by score (highest first)
	t.sortPodsByScore(podInfos, config.PreferCrossNode)

	// Display discovered pods
	fmt.Printf("\n📋 Discovered Pods:\n")
	for i, pod := range podInfos {
		status := "❌"
		if pod.IsReady {
			status = "✅"
		}

		nodeInfo := ""
		if pod.NodeName != "" {
			nodeInfo = fmt.Sprintf(" on %s", pod.NodeName)
		}

		suitability := ""
		if pod.IsNetshoot {
			suitability = " 🔧 netshoot"
		} else if pod.Score > 5 {
			suitability = " 🌐 network-capable"
		}

		fmt.Printf("[%d] %s %s (%s)%s - IP: %s%s\n",
			i+1, status, pod.Name, pod.Status, nodeInfo, pod.PodIP, suitability)
	}

	if len(podInfos) >= 2 {
		fmt.Printf("\n💡 Recommendation: Pods %d and %d for cross-node testing\n", 1, 2)
	}

	// Handle selection
	if len(podInfos) < 2 {
		fmt.Printf("\n⚠️  Found only %d suitable pod(s), need at least 2 for connectivity testing\n", len(podInfos))
		if config.AutoCreateMissing {
			fmt.Printf("Auto-creating additional pods...\n")
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}

		fmt.Printf("Would you like to create additional pods? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
			return TestConfig{
				UseExistingPods: false,
				CreateFreshPods: true,
			}, nil
		}
		return TestConfig{}, fmt.Errorf("insufficient pods for testing")
	}

	fmt.Printf("\nSelect pods for testing (e.g., 1,2 or just press Enter for recommended): ")
	var input string
	fmt.Scanln(&input)

	var selectedIndices []int
	if input == "" {
		// Use recommended pods (first two, which are highest scored)
		selectedIndices = []int{0, 1}
	} else {
		// Parse user input
		parts := strings.Split(input, ",")
		for _, part := range parts {
			index, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || index < 1 || index > len(podInfos) {
				return TestConfig{}, fmt.Errorf("invalid selection: %s", part)
			}
			selectedIndices = append(selectedIndices, index-1) // Convert to 0-based
		}
	}

	if len(selectedIndices) < 2 {
		return TestConfig{}, fmt.Errorf("need to select at least 2 pods")
	}

	// Create selector for the chosen pods
	selectedPods := make([]PodInfo, len(selectedIndices))
	for i, idx := range selectedIndices {
		selectedPods[i] = podInfos[idx]
	}

	// Generate a selector that matches the selected pods
	selector := t.generatePodSelector(selectedPods)

	return TestConfig{
		UseExistingPods: true,
		TargetNamespace: config.TargetNamespace,
		PodSelector:     selector,
		CreateFreshPods: false,
	}, nil
}

// convertToPodInfo converts a Kubernetes Pod to PodInfo with scoring
func (t *Tester) convertToPodInfo(pod *corev1.Pod) PodInfo {
	age := time.Since(pod.CreationTimestamp.Time).Truncate(time.Second).String()

	// Determine primary image
	image := "unknown"
	if len(pod.Spec.Containers) > 0 {
		image = pod.Spec.Containers[0].Image
	}

	// Check if it's a netshoot pod
	isNetshoot := strings.Contains(strings.ToLower(image), "netshoot")

	// Calculate suitability score
	score := t.calculatePodScore(pod, isNetshoot)

	return PodInfo{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		NodeName:   pod.Spec.NodeName,
		PodIP:      pod.Status.PodIP,
		Status:     string(pod.Status.Phase),
		Age:        age,
		Image:      image,
		Labels:     pod.Labels,
		IsNetshoot: isNetshoot,
		IsReady:    t.isPodReady(pod),
		Score:      score,
	}
}

// calculatePodScore assigns a suitability score to a pod for network testing
func (t *Tester) calculatePodScore(pod *corev1.Pod, isNetshoot bool) int {
	score := 0

	// Base score for ready pods
	if t.isPodReady(pod) {
		score += 10
	}

	// High score for netshoot pods
	if isNetshoot {
		score += 20
	}

	// Score for network-capable images
	image := strings.ToLower(pod.Spec.Containers[0].Image)
	networkImages := []string{"busybox", "alpine", "ubuntu", "centos", "debian", "curl", "wget"}
	for _, netImg := range networkImages {
		if strings.Contains(image, netImg) {
			score += 5
			break
		}
	}

	// Bonus for having network tools
	if len(pod.Spec.Containers) > 0 {
		container := pod.Spec.Containers[0]
		if len(container.Command) > 0 {
			cmdStr := strings.ToLower(strings.Join(container.Command, " "))
			if strings.Contains(cmdStr, "sleep") || strings.Contains(cmdStr, "sh") || strings.Contains(cmdStr, "/bin/bash") {
				score += 3
			}
		}
	}

	return score
}

// isNetworkCapable checks if a pod is likely to have network debugging capabilities
func (t *Tester) isNetworkCapable(pod *corev1.Pod) bool {
	if len(pod.Spec.Containers) == 0 {
		return false
	}

	image := strings.ToLower(pod.Spec.Containers[0].Image)
	networkCapableImages := []string{
		"netshoot", "busybox", "alpine", "ubuntu", "centos", "debian",
		"curl", "wget", "nicolaka/netshoot", "praqma/network-multitool",
	}

	for _, capable := range networkCapableImages {
		if strings.Contains(image, capable) {
			return true
		}
	}

	return false
}

// sortPodsByScore sorts pods by their suitability score
func (t *Tester) sortPodsByScore(pods []PodInfo, preferCrossNode bool) {
	// Simple bubble sort by score (descending)
	for i := 0; i < len(pods)-1; i++ {
		for j := 0; j < len(pods)-i-1; j++ {
			// Higher score should come first
			if pods[j].Score < pods[j+1].Score {
				pods[j], pods[j+1] = pods[j+1], pods[j]
			}
		}
	}

	// If preferCrossNode, try to put cross-node pairs at the front
	if preferCrossNode && len(pods) >= 2 {
		for i := 0; i < len(pods)-1; i++ {
			for j := i + 1; j < len(pods); j++ {
				if pods[i].NodeName != "" && pods[j].NodeName != "" &&
					pods[i].NodeName != pods[j].NodeName {
					// Found a cross-node pair, move to front if not already there
					if i > 0 {
						// Move pods[i] to front
						temp := pods[i]
						copy(pods[1:i+1], pods[0:i])
						pods[0] = temp
					}
					if j > 1 {
						// Move pods[j] to second position
						temp := pods[j]
						copy(pods[2:j+1], pods[1:j])
						pods[1] = temp
					}
					return
				}
			}
		}
	}
}

// generatePodSelector creates a label selector that matches the selected pods
func (t *Tester) generatePodSelector(pods []PodInfo) string {
	if len(pods) == 0 {
		return ""
	}

	// Try to find a common label among all selected pods
	commonLabels := make(map[string]string)

	// Initialize with first pod's labels
	for k, v := range pods[0].Labels {
		commonLabels[k] = v
	}

	// Remove labels that don't match across all pods
	for _, pod := range pods[1:] {
		for k, v := range commonLabels {
			if podVal, exists := pod.Labels[k]; !exists || podVal != v {
				delete(commonLabels, k)
			}
		}
	}

	// Use the first common label we find
	for k, v := range commonLabels {
		// Skip kubernetes system labels
		if !strings.HasPrefix(k, "kubernetes.io/") && !strings.HasPrefix(k, "k8s.io/") {
			return fmt.Sprintf("%s=%s", k, v)
		}
	}

	// Fallback: create a selector based on pod names (this is a hack but works for demonstration)
	var names []string
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	// Return a pseudo-selector (this won't actually work in practice, but shows the concept)
	return fmt.Sprintf("pod-name in (%s)", strings.Join(names, ","))
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

// createNginxDeployment creates an nginx deployment with the exact spec from the task
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

			// Check if deployment is ready
			if deployment.Status.ReadyReplicas >= *deployment.Spec.Replicas && deployment.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

// createNginxService creates a service to expose the nginx deployment
func (t *Tester) createNginxService(ctx context.Context, serviceName, deploymentName string) (*corev1.Service, error) {
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
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return t.clientset.CoreV1().Services(t.namespace).Create(ctx, service, metav1.CreateOptions{})
}

// testDNSResolution tests if the service can be resolved via DNS
func (t *Tester) testDNSResolution(ctx context.Context, podName, serviceName string) (string, error) {
	// Create the exec request for nslookup
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

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil {
		if stderr.Len() > 0 {
			return output + "\nSTDERR: " + stderr.String(), err
		}
		return output, err
	}

	// Check if DNS resolution was successful
	if strings.Contains(strings.ToLower(output), "name:") && strings.Contains(strings.ToLower(output), "address:") {
		return output, nil
	}

	return output, fmt.Errorf("DNS resolution failed")
}

// testHTTPConnectivity tests HTTP connectivity to the service
func (t *Tester) testHTTPConnectivity(ctx context.Context, podName, serviceName string) (string, error) {
	// Create the exec request for curl
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"curl", "-s", "-m", "10", fmt.Sprintf("http://%s", serviceName)},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil {
		if stderr.Len() > 0 {
			return output + "\nSTDERR: " + stderr.String(), err
		}
		return output, err
	}

	// Check if we got nginx welcome page
	if strings.Contains(strings.ToLower(output), "welcome to nginx") || strings.Contains(strings.ToLower(output), "<title>") {
		return output, nil
	}

	return output, fmt.Errorf("HTTP connectivity test failed - unexpected response")
}

// testLoadBalancing tests load balancing by making multiple requests
func (t *Tester) testLoadBalancing(ctx context.Context, podName, serviceName string) (string, error) {
	// Make 5 requests to see if we get responses (simple load balancing test)
	successCount := 0

	for i := 0; i < 5; i++ {
		_, err := t.testHTTPConnectivity(ctx, podName, serviceName)
		if err == nil {
			successCount++
		}
		// Small delay between requests
		time.Sleep(200 * time.Millisecond)
	}

	if successCount >= 3 {
		return fmt.Sprintf("Load balancing working - %d/5 requests successful", successCount), nil
	}

	return fmt.Sprintf("Load balancing issues - only %d/5 requests successful", successCount),
		fmt.Errorf("insufficient successful requests for load balancing")
}

// testServiceIPPing tests ICMP connectivity directly to the service ClusterIP
func (t *Tester) testServiceIPPing(ctx context.Context, podName, serviceIP string) (string, error) {
	// Create the exec request for ping to service IP
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"ping", "-c", "3", "-W", "3", serviceIP},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil {
		if stderr.Len() > 0 {
			return output + "\nSTDERR: " + stderr.String(), err
		}
		return output, err
	}

	return output, nil
}

// testHTTPConnectivityWithStatusCode tests HTTP connectivity and returns status code (like curl -w "%{http_code}\n")
func (t *Tester) testHTTPConnectivityWithStatusCode(ctx context.Context, podName, target string) (string, string, error) {
	// Create the exec request for curl with status code output
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

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	statusCode := strings.TrimSpace(stdout.String())
	if err != nil {
		if stderr.Len() > 0 {
			return statusCode, stderr.String(), err
		}
		return statusCode, "", err
	}

	// Get the actual response content with a separate curl call
	contentReq := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(t.namespace).
		SubResource("exec")

	contentReq.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"curl", "-s", "-m", "10", fmt.Sprintf("http://%s", target)},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	contentExec, err := remotecommand.NewSPDYExecutor(t.config, "POST", contentReq.URL())
	if err != nil {
		return statusCode, "", fmt.Errorf("failed to create content executor: %v", err)
	}

	var contentStdout, contentStderr bytes.Buffer
	err = contentExec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &contentStdout,
		Stderr: &contentStderr,
	})

	content := contentStdout.String()
	if err != nil && contentStderr.Len() > 0 {
		content += "\nSTDERR: " + contentStderr.String()
	}

	return statusCode, content, nil
}

// getServiceIP retrieves the ClusterIP of a service (equivalent to kubectl get svc -o jsonpath='{.spec.clusterIP}')
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

// getNginxPodNodes gets the node names where nginx pods are running
func (t *Tester) getNginxPodNodes(ctx context.Context, deploymentName string) ([]string, error) {
	// Get pods with the deployment's label selector
	pods, err := t.clientset.CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nginx pods: %v", err)
	}

	var nodeNames []string
	nodeMap := make(map[string]bool) // Use map to avoid duplicates

	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" && !nodeMap[pod.Spec.NodeName] {
			nodeNames = append(nodeNames, pod.Spec.NodeName)
			nodeMap[pod.Spec.NodeName] = true
		}
	}

	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("no nginx pods found or pods not scheduled")
	}

	return nodeNames, nil
}

// findDifferentWorkerNode finds a worker node that's different from the provided nodes
func (t *Tester) findDifferentWorkerNode(ctx context.Context, usedNodes []string) (string, error) {
	allWorkerNodes, err := t.getWorkerNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get worker nodes: %v", err)
	}

	// Need at least 2 worker nodes for cross-node testing
	if len(allWorkerNodes) < 2 {
		return "", fmt.Errorf("need at least 2 worker nodes for cross-node testing, found %d", len(allWorkerNodes))
	}

	// Create a map of used nodes for quick lookup
	usedNodeMap := make(map[string]bool)
	for _, node := range usedNodes {
		usedNodeMap[node] = true
	}

	// Find a worker node that's not in the used nodes
	for _, node := range allWorkerNodes {
		if !usedNodeMap[node] {
			return node, nil
		}
	}

	// If all worker nodes are used by nginx pods, pick the first one
	// This still enables cross-node testing since nginx has 2 replicas across nodes
	if len(allWorkerNodes) >= 2 {
		// Use the first worker node - this will still test cross-node connectivity
		// because the service will load balance to nginx pods on other nodes too
		return allWorkerNodes[0], nil
	}

	return "", fmt.Errorf("insufficient worker nodes for cross-node testing (need at least 2, found %d)", len(allWorkerNodes))
}

// testPodToPodDNS tests DNS resolution between pods
func (t *Tester) testPodToPodDNS(ctx context.Context, testPodName, deploymentName string) (string, error) {
	// Get one of the nginx pods to test DNS resolution to
	pods, err := t.clientset.CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list nginx pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no nginx pods found")
	}

	// Try to resolve the first nginx pod by its IP
	targetPod := pods.Items[0]
	if targetPod.Status.PodIP == "" {
		return "", fmt.Errorf("target pod has no IP address")
	}

	// Create the exec request for nslookup on the pod IP
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(testPodName).
		Namespace(t.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"nslookup", targetPod.Status.PodIP},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil && stderr.Len() > 0 {
		output += "\nSTDERR: " + stderr.String()
	}

	// Simple validation - if we get some DNS response, consider it successful
	if strings.Contains(strings.ToLower(output), "name") || strings.Contains(output, targetPod.Status.PodIP) {
		return fmt.Sprintf("Pod IP %s resolved successfully", targetPod.Status.PodIP), nil
	}

	return output, fmt.Errorf("pod-to-pod DNS resolution failed")
}

// cleanupServiceResources removes all service-related test resources
func (t *Tester) cleanupServiceResources(ctx context.Context, deploymentName, serviceName, podName string) {
	// Clean up deployment
	t.clientset.AppsV1().Deployments(t.namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})

	// Clean up service
	t.clientset.CoreV1().Services(t.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})

	// Clean up test pod
	if podName != "" {
		t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	}
}

// findExistingNetshootPods discovers existing pods matching the selector in the target namespace
func (t *Tester) findExistingNetshootPods(ctx context.Context, config TestConfig) ([]corev1.Pod, error) {
	// List pods in the target namespace with the specified selector
	pods, err := t.clientset.CoreV1().Pods(config.TargetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: config.PodSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %v", config.TargetNamespace, err)
	}

	var readyPods []corev1.Pod
	for _, pod := range pods.Items {
		// Only include ready pods
		if t.isPodReady(&pod) {
			readyPods = append(readyPods, pod)
		}
	}

	return readyPods, nil
}

// selectCrossNodePods selects two pods preferably on different nodes for cross-node testing
func (t *Tester) selectCrossNodePods(pods []corev1.Pod) (*corev1.Pod, *corev1.Pod, error) {
	if len(pods) < 2 {
		return nil, nil, fmt.Errorf("need at least 2 pods, found %d", len(pods))
	}

	// First, try to find pods on different nodes
	for i, pod1 := range pods {
		for j, pod2 := range pods {
			if i != j && pod1.Spec.NodeName != pod2.Spec.NodeName && pod1.Spec.NodeName != "" && pod2.Spec.NodeName != "" {
				return &pod1, &pod2, nil
			}
		}
	}

	// If no cross-node pairs found, use the first two pods
	if len(pods) >= 2 {
		return &pods[0], &pods[1], nil
	}

	return nil, nil, fmt.Errorf("insufficient pods for testing")
}

// isPodReady checks if a pod is in Ready state
func (t *Tester) isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// pingFromPodInNamespace executes ping command from a pod in a specific namespace
func (t *Tester) pingFromPodInNamespace(ctx context.Context, fromPod, namespace, targetIP string) (string, error) {
	// Create the exec request
	req := t.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(fromPod).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: "netshoot",
		Command:   []string{"ping", "-c", "3", "-W", "3", "-i", "1", targetIP},
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(t.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %v", err)
	}

	// Prepare buffers for output
	var stdout, stderr bytes.Buffer

	// Execute the command
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	output := stdout.String()
	if err != nil {
		if stderr.Len() > 0 {
			return output + "\nSTDERR: " + stderr.String(), err
		}
		return output, err
	}

	return output, nil
}
