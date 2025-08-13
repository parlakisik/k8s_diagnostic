package core

import (
	"fmt"
	"strings"
)

// UserMessageGenerator provides intelligent analysis of test scenarios and generates user-friendly messages
type UserMessageGenerator struct {
	testType       string
	infrastructure *ClusterInfrastructure
}

// NewUserMessageGenerator creates a new user message generator with infrastructure context
func NewUserMessageGenerator(testType string, infrastructure *ClusterInfrastructure) *UserMessageGenerator {
	return &UserMessageGenerator{
		testType:       testType,
		infrastructure: infrastructure,
	}
}

// AnalyzeEnvironmentForNetworking analyzes cluster environment for networking test readiness
func (g *UserMessageGenerator) AnalyzeEnvironmentForNetworking() UserMessage {
	if g.infrastructure == nil {
		return UserMessage{
			Phase:       "environment",
			Status:      "warning",
			Title:       "Could not verify cluster environment",
			Description: "Unable to collect cluster infrastructure information",
			Context:     "Proceeding with basic assumptions about cluster setup",
			Hints:       []string{"Verify kubectl connectivity to cluster", "Check cluster permissions"},
		}
	}

	nodeCount := g.infrastructure.NodeCount
	cniProvider := g.infrastructure.CNIProvider

	// Check for insufficient nodes for cross-node testing
	if nodeCount < 2 {
		return UserMessage{
			Phase:       "environment",
			Status:      "failure",
			Title:       "Insufficient worker nodes",
			Description: fmt.Sprintf("Found only %d node - cross-node testing requires at least 2 worker nodes", nodeCount),
			Context:     "Cannot validate distributed workload capability",
			Hints: []string{
				"Add more worker nodes to your cluster",
				"Or test single-node connectivity instead",
				"Consider using single-node test configurations",
			},
		}
	}

	// Analyze CNI provider and version
	cniDetails := ""
	if cniProvider != "" && cniProvider != "unknown" {
		cniDetails = fmt.Sprintf("using %s CNI", cniProvider)
		if g.infrastructure.CNIVersion != "" && g.infrastructure.CNIVersion != "detected" {
			cniDetails = fmt.Sprintf("using %s CNI version %s", cniProvider, g.infrastructure.CNIVersion)
		}
	}

	// Generate success message with infrastructure insights
	title := fmt.Sprintf("Found %d worker nodes - cross-node testing ready", nodeCount)
	description := fmt.Sprintf("Your cluster has %d nodes %s", nodeCount, cniDetails)
	context := "Your cluster can handle distributed workloads and cross-node connectivity testing"

	hints := []string{}
	if cniProvider != "" {
		hints = append(hints, fmt.Sprintf("%s CNI detected and ready for testing", cniProvider))
	}

	// Add platform-specific insights
	if g.infrastructure.Platform != "" && g.infrastructure.Platform != "unknown" {
		hints = append(hints, fmt.Sprintf("Running on %s platform", g.infrastructure.Platform))
	}

	return UserMessage{
		Phase:       "environment",
		Status:      "success",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}

// AnalyzeEnvironmentFailure analyzes environment collection failures
func (g *UserMessageGenerator) AnalyzeEnvironmentFailure(err error) UserMessage {
	errorStr := strings.ToLower(err.Error())
	hints := []string{}

	title := "Environment check failed"
	description := "Could not verify cluster readiness"
	context := "Unable to collect cluster information for test optimization"

	switch {
	case strings.Contains(errorStr, "connection") || strings.Contains(errorStr, "timeout"):
		description = "Cannot connect to Kubernetes cluster"
		context = "Cluster connectivity issue preventing test execution"
		hints = append(hints, "Verify kubectl is configured correctly")
		hints = append(hints, "Check cluster connectivity and credentials")
		hints = append(hints, "Ensure cluster is running and accessible")

	case strings.Contains(errorStr, "permission") || strings.Contains(errorStr, "forbidden"):
		description = "Insufficient permissions to check cluster"
		context = "Access denied when trying to collect cluster information"
		hints = append(hints, "Verify kubectl permissions and RBAC settings")
		hints = append(hints, "Ensure service account has cluster-admin or sufficient permissions")

	case strings.Contains(errorStr, "not found") || strings.Contains(errorStr, "unknown"):
		description = "Kubernetes cluster not found or not ready"
		context = "Cluster may not be running or properly configured"
		hints = append(hints, "Verify cluster is running and kubectl context is correct")
		hints = append(hints, "Check if cluster nodes are ready")

	default:
		description = fmt.Sprintf("Cluster check failed: %s", err.Error())
		hints = append(hints, "Check cluster logs for detailed error information")
		hints = append(hints, "Verify cluster is running and accessible")
	}

	return UserMessage{
		Phase:       "environment",
		Status:      "failure",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}

// AnalyzeConnectivityFailure provides intelligent analysis of connectivity test failures
func (g *UserMessageGenerator) AnalyzeConnectivityFailure(err error, testType string) UserMessage {
	errorStr := strings.ToLower(err.Error())
	hints := []string{}

	title := "Connectivity test failed"
	description := fmt.Sprintf("%s communication blocked", strings.ToUpper(testType))
	context := "Network connectivity issue detected"

	// Analyze specific error patterns
	switch {
	case strings.Contains(errorStr, "connection refused"):
		title = "Connection refused"
		description = "Target service is not accepting connections"
		context = "Network traffic is reaching the target but being rejected"
		hints = g.generateConnectionRefusedHints(testType)

	case strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "timed out"):
		title = "Connection timeout"
		description = "Network communication is timing out"
		context = "Traffic may be blocked or network is slow/unreliable"
		hints = g.generateTimeoutHints(testType)

	case strings.Contains(errorStr, "no route") || strings.Contains(errorStr, "unreachable"):
		title = "Network unreachable"
		description = "Cannot find route to target"
		context = "Routing issue between source and destination"
		hints = g.generateRoutingHints()

	case strings.Contains(errorStr, "dns") || strings.Contains(errorStr, "resolve"):
		title = "DNS resolution failed"
		description = "Cannot resolve target hostname"
		context = "DNS configuration or service discovery issue"
		hints = g.generateDNSHints()

	case strings.Contains(errorStr, "permission") || strings.Contains(errorStr, "forbidden"):
		title = "Network policy blocked"
		description = "Traffic blocked by network policies"
		context = "Security policies are preventing communication"
		hints = g.generatePolicyHints()

	default:
		description = fmt.Sprintf("Unknown connectivity issue: %s", err.Error())
		context = "Unexpected network problem detected"
		hints = g.generateGenericConnectivityHints()
	}

	// Add CNI-specific context
	if g.infrastructure != nil && g.infrastructure.CNIProvider != "" {
		context = g.addCNIContext(context, g.infrastructure.CNIProvider)
	}

	return UserMessage{
		Phase:       "execution",
		Status:      "failure",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}

// generateConnectionRefusedHints provides hints for connection refused errors
func (g *UserMessageGenerator) generateConnectionRefusedHints(testType string) []string {
	hints := []string{
		"Verify target service is running and healthy",
		"Check service port configuration and exposure",
	}

	switch testType {
	case "http":
		hints = append(hints, "Verify HTTP server is listening on expected port")
		hints = append(hints, "Check application logs for startup errors")
	case "dns":
		hints = append(hints, "Verify CoreDNS pods are running")
		hints = append(hints, "Check DNS service configuration")
	}

	if g.infrastructure != nil && g.infrastructure.NodeCount > 1 {
		hints = append(hints, "Verify service is accessible from all nodes")
	}

	return hints
}

// generateTimeoutHints provides hints for timeout errors
func (g *UserMessageGenerator) generateTimeoutHints(testType string) []string {
	hints := []string{
		"Check network latency between nodes",
		"Verify no network policies are blocking traffic",
		"Review cluster resource constraints",
	}

	if g.infrastructure != nil {
		switch g.infrastructure.CNIProvider {
		case "cilium":
			hints = append(hints, "Check Cilium connectivity test results")
			hints = append(hints, "Verify Cilium agent is healthy on all nodes")
		case "calico":
			hints = append(hints, "Check Calico network policies")
			hints = append(hints, "Verify Calico node status")
		case "flannel":
			hints = append(hints, "Check Flannel daemon set status")
			hints = append(hints, "Verify overlay network configuration")
		}

		if g.infrastructure.NodeCount > 1 {
			hints = append(hints, "Test connectivity between specific nodes")
		}
	}

	return hints
}

// generateRoutingHints provides hints for routing issues
func (g *UserMessageGenerator) generateRoutingHints() []string {
	hints := []string{
		"Check pod network routing configuration",
		"Verify CNI bridge and routing setup",
		"Review cluster network configuration",
	}

	if g.infrastructure != nil && g.infrastructure.NodeCount > 1 {
		hints = append(hints, "Verify cross-node routing is properly configured")
		hints = append(hints, "Check node-to-node connectivity")
	}

	return hints
}

// generateDNSHints provides hints for DNS resolution issues
func (g *UserMessageGenerator) generateDNSHints() []string {
	return []string{
		"Check CoreDNS pod status and configuration",
		"Verify cluster DNS settings and service",
		"Review DNS policy configurations",
		"Test DNS resolution from different pods",
		"Check DNS service endpoints and load balancing",
	}
}

// generatePolicyHints provides hints for network policy issues
func (g *UserMessageGenerator) generatePolicyHints() []string {
	hints := []string{
		"Review network policies in test namespace",
		"Check for conflicting security policies",
		"Verify pod security context and policies",
	}

	if g.infrastructure != nil {
		switch g.infrastructure.CNIProvider {
		case "cilium":
			hints = append(hints, "Check Cilium network policies: kubectl get ciliumnetworkpolicies --all-namespaces")
		case "calico":
			hints = append(hints, "Check Calico network policies: kubectl get networkpolicies --all-namespaces")
		}
	}

	return hints
}

// generateGenericConnectivityHints provides generic connectivity troubleshooting hints
func (g *UserMessageGenerator) generateGenericConnectivityHints() []string {
	hints := []string{
		"Check cluster network policies",
		"Verify CNI configuration and health",
		"Review firewall rules between nodes",
		"Check cluster events for additional context",
	}

	if g.infrastructure != nil && g.infrastructure.CNIProvider != "" {
		hints = append(hints, fmt.Sprintf("Verify %s CNI agent status", g.infrastructure.CNIProvider))
	}

	return hints
}

// addCNIContext adds CNI-specific context to error messages
func (g *UserMessageGenerator) addCNIContext(context, cniProvider string) string {
	switch cniProvider {
	case "cilium":
		return context + " - check Cilium configuration and policies"
	case "calico":
		return context + " - check Calico configuration and policies"
	case "flannel":
		return context + " - check Flannel configuration"
	case "weave":
		return context + " - check Weave Net configuration"
	default:
		return context + fmt.Sprintf(" - check %s CNI configuration", cniProvider)
	}
}

// AnalyzePodFailure provides intelligent analysis of pod deployment failures
func (g *UserMessageGenerator) AnalyzePodFailure(err error) UserMessage {
	errorStr := strings.ToLower(err.Error())
	hints := []string{}

	title := "Pod deployment failed"
	description := "Cannot create test pod"
	context := "Test environment setup blocked"

	switch {
	case strings.Contains(errorStr, "insufficient") || strings.Contains(errorStr, "resource"):
		title = "Insufficient resources"
		description = "Cluster lacks resources to create pod"
		context = "Node capacity or resource limits preventing pod scheduling"
		hints = []string{
			"Check node CPU and memory availability",
			"Review resource quotas and limits",
			"Consider scaling cluster or reducing resource requests",
		}

		if g.infrastructure != nil && g.infrastructure.NodeCount > 1 {
			hints = append(hints, "Consider distributing workload across multiple nodes")
		}

	case strings.Contains(errorStr, "image") || strings.Contains(errorStr, "pull"):
		title = "Image pull failed"
		description = "Cannot download container image"
		context = "Container registry access or image availability issue"
		hints = []string{
			"Verify container image exists and is accessible",
			"Check image pull policies and secrets",
			"Verify network connectivity to registry",
		}

	case strings.Contains(errorStr, "schedule") || strings.Contains(errorStr, "node"):
		title = "Pod scheduling failed"
		description = "Cannot find suitable node for pod"
		context = "Node constraints or affinity rules preventing placement"
		hints = []string{
			"Check node taints and tolerations",
			"Review pod affinity and anti-affinity rules",
			"Verify nodes have sufficient resources",
		}

		if g.infrastructure != nil {
			hints = append(hints, fmt.Sprintf("Cluster has %d nodes available", g.infrastructure.NodeCount))
		}

	case strings.Contains(errorStr, "security") || strings.Contains(errorStr, "policy"):
		title = "Security policy blocked"
		description = "Pod blocked by security policies"
		context = "Security constraints preventing pod creation"
		hints = []string{
			"Check pod security policies and contexts",
			"Verify RBAC permissions for test namespace",
			"Review admission controllers and security configurations",
		}

	default:
		description = fmt.Sprintf("Pod creation failed: %s", err.Error())
		hints = []string{
			"Check cluster events for detailed error information",
			"Verify cluster has sufficient resources",
			"Review pod specifications and constraints",
		}
	}

	return UserMessage{
		Phase:       "setup",
		Status:      "failure",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}

// GenerateTestSummary creates a comprehensive test summary with user insights
func (g *UserMessageGenerator) GenerateTestSummary(testName string, success bool, duration float64, executionData *TestExecutionData) UserMessage {
	if success {
		return g.generateSuccessSummary(testName, duration, executionData)
	}
	return g.generateFailureSummary(testName, duration, executionData)
}

// generateSuccessSummary creates user-friendly success summary
func (g *UserMessageGenerator) generateSuccessSummary(testName string, duration float64, executionData *TestExecutionData) UserMessage {
	title := "Test completed successfully"
	description := "All connectivity checks passed"
	context := "Your cluster networking is functioning correctly"
	hints := []string{}

	// Customize based on test specifics
	if strings.Contains(testName, "cross-node") {
		title = "Cross-node networking working perfectly"
		description = "Pods can communicate seamlessly across worker nodes"
		context = "Your cluster can handle distributed workloads"
		hints = append(hints, "Your cluster is ready for distributed applications")
	}

	// Add performance insights
	if duration > 0 {
		if duration < 10 {
			hints = append(hints, "Test completed quickly - good cluster performance")
		} else if duration > 30 {
			hints = append(hints, "Test took longer than expected - consider checking cluster performance")
		}
	}

	// Add infrastructure-specific insights
	if g.infrastructure != nil && g.infrastructure.CNIProvider != "" {
		context += fmt.Sprintf(" - %s CNI is properly configured", g.infrastructure.CNIProvider)
		hints = append(hints, fmt.Sprintf("%s CNI is functioning correctly", g.infrastructure.CNIProvider))
	}

	// Add execution insights if available
	if executionData != nil && len(executionData.ConnectivityTests) > 0 {
		successfulTests := 0
		for _, test := range executionData.ConnectivityTests {
			if test.Success {
				successfulTests++
			}
		}
		if successfulTests > 1 {
			hints = append(hints, fmt.Sprintf("All %d connectivity tests passed", successfulTests))
		}
	}

	return UserMessage{
		Phase:       "result",
		Status:      "success",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}

// generateFailureSummary creates user-friendly failure summary with actionable insights
func (g *UserMessageGenerator) generateFailureSummary(testName string, duration float64, executionData *TestExecutionData) UserMessage {
	title := "Test failed"
	description := "Connectivity issues detected"
	context := "Your cluster networking needs attention"
	hints := []string{}

	// Analyze failure points for more specific guidance
	if executionData != nil && len(executionData.FailurePoints) > 0 {
		lastFailure := executionData.FailurePoints[len(executionData.FailurePoints)-1]

		switch lastFailure.Phase {
		case "setup":
			title = "Test environment setup failed"
			description = "Could not create required test resources"
			context = "Cluster resource or configuration issue"

		case "execution":
			title = "Network connectivity blocked"
			description = "Pods cannot communicate properly"
			context = "CNI or network policy configuration issue"
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
		if g.infrastructure != nil && g.infrastructure.CNIProvider != "" {
			hints = append(hints, fmt.Sprintf("Review %s CNI configuration", g.infrastructure.CNIProvider))
		}
	}

	return UserMessage{
		Phase:       "result",
		Status:      "failure",
		Title:       title,
		Description: description,
		Context:     context,
		Hints:       hints,
	}
}
