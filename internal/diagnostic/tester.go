package diagnostic

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// TestResult represents the result of a connectivity test
type TestResult struct {
	Success bool
	Message string
	Details []string
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
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Pod %s is not reachable from pod %s", pod2Name, pod1Name),
			Details: details,
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

// cleanupPod removes a single pod
func (t *Tester) cleanupPod(ctx context.Context, podName string) {
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// cleanupPods removes test pods
func (t *Tester) cleanupPods(ctx context.Context, pod1Name, pod2Name string) {
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, pod1Name, metav1.DeleteOptions{})
	t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, pod2Name, metav1.DeleteOptions{})
}
