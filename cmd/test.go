package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"
	"k8s-diagnostic/internal/diagnostic/networking"
	diagnostic "k8s-diagnostic/internal/diagnostic/policies"
	"k8s-diagnostic/internal/diagnostic/policies/l3"
	"k8s-diagnostic/internal/diagnostic/policies/l4"
	"k8s-diagnostic/internal/diagnostic/policies/l7"

	"github.com/spf13/cobra"
)

// InfrastructureStatus represents the cluster infrastructure status
type InfrastructureStatus struct {
	ClusterInfo    ClusterInfo         `json:"cluster_info"`
	CiliumStatus   CiliumStatus        `json:"cilium_status"`
	NamespaceInfo  NamespaceInfo       `json:"namespace_info"`
	PodDeployments []PodDeploymentInfo `json:"pod_deployments"`
	ServiceStatus  []ServiceStatusInfo `json:"service_status"`
	NodeInfo       []NodeInfo          `json:"node_info"`
}

// ClusterInfo contains basic cluster information
type ClusterInfo struct {
	KubernetesVersion string `json:"kubernetes_version"`
	ClusterNodes      int    `json:"cluster_nodes"`
	Context           string `json:"current_context"`
	Accessible        bool   `json:"accessible"`
	ErrorDetails      string `json:"error_details,omitempty"`
}

// CiliumStatus contains Cilium CNI health information
type CiliumStatus struct {
	Installed      bool     `json:"installed"`
	Healthy        bool     `json:"healthy"`
	Version        string   `json:"version,omitempty"`
	Agents         int      `json:"agents_running"`
	ConnectivityOK bool     `json:"connectivity_ok"`
	PolicyEnabled  bool     `json:"policy_enabled"`
	ErrorDetails   []string `json:"error_details,omitempty"`
	VerboseLogs    []string `json:"verbose_logs,omitempty"`
}

// NamespaceInfo contains test namespace information
type NamespaceInfo struct {
	Name         string `json:"name"`
	Created      bool   `json:"created_successfully"`
	Accessible   bool   `json:"accessible"`
	ErrorDetails string `json:"error_details,omitempty"`
}

// PodDeploymentInfo represents individual pod deployment status
type PodDeploymentInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Ready        bool     `json:"ready"`
	Status       string   `json:"status"`
	RestartCount int      `json:"restart_count"`
	Node         string   `json:"node,omitempty"`
	PodIP        string   `json:"pod_ip,omitempty"`
	CreationTime string   `json:"creation_time,omitempty"`
	ErrorDetails []string `json:"error_details,omitempty"`
	VerboseLogs  []string `json:"verbose_logs,omitempty"`
}

// ServiceStatusInfo represents service deployment and accessibility
type ServiceStatusInfo struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	ClusterIP    string   `json:"cluster_ip,omitempty"`
	ExternalIP   string   `json:"external_ip,omitempty"`
	Ports        []string `json:"ports"`
	Ready        bool     `json:"ready"`
	Accessible   bool     `json:"accessible"`
	ErrorDetails []string `json:"error_details,omitempty"`
}

// NodeInfo contains cluster node information
type NodeInfo struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	Roles             string `json:"roles"`
	KubernetesVersion string `json:"kubernetes_version"`
	CiliumReady       bool   `json:"cilium_ready"`
	ContainerRuntime  string `json:"container_runtime,omitempty"`
}

// TestSummaryEntry represents a single test result for JSON output
type TestSummaryEntry struct {
	Name             string             `json:"name"`
	Success          bool               `json:"success"`
	ExpectedOutcome  string             `json:"expected_outcome"`
	ActualOutcome    string             `json:"actual_outcome"`
	Message          string             `json:"message"`
	StartTime        time.Time          `json:"start_time"`
	EndTime          time.Time          `json:"end_time"`
	Duration         float64            `json:"duration_seconds"`
	Details          []string           `json:"details,omitempty"`
	Steps            []TestStep         `json:"steps,omitempty"`
	ErrorDetails     []string           `json:"error_details,omitempty"`
	VerboseErrorLogs []string           `json:"verbose_error_logs,omitempty"`
	CommandsExecuted []CommandExecution `json:"commands_executed,omitempty"`
}

// TestStep represents individual test steps
type TestStep struct {
	StepNumber   int      `json:"step_number"`
	Description  string   `json:"description"`
	Success      bool     `json:"success"`
	Duration     float64  `json:"duration_seconds"`
	Details      string   `json:"details,omitempty"`
	ErrorDetails []string `json:"error_details,omitempty"`
}

// CommandExecution represents executed commands during tests
type CommandExecution struct {
	Command  string  `json:"command"`
	ExitCode int     `json:"exit_code"`
	Duration float64 `json:"duration_seconds"`
	Stdout   string  `json:"stdout,omitempty"`
	Stderr   string  `json:"stderr,omitempty"`
	Success  bool    `json:"success"`
}

// TestSuiteResult represents the complete test suite results for JSON output
type TestSuiteResult struct {
	Timestamp           time.Time                   `json:"timestamp"`
	TestConfiguration   TestConfiguration           `json:"test_configuration"`
	Infrastructure      *core.ClusterInfrastructure `json:"infrastructure"`
	TotalTests          int                         `json:"total_tests"`
	PassedTests         int                         `json:"passed_tests"`
	FailedTests         int                         `json:"failed_tests"`
	SkippedTests        int                         `json:"skipped_tests"`
	SuccessRate         float64                     `json:"success_rate"`
	TotalTime           float64                     `json:"total_time_seconds"`
	Tests               []TestSummaryEntry          `json:"tests"`
	OverallHealthStatus string                      `json:"overall_health_status"`
}

// TestConfiguration contains test run configuration
type TestConfiguration struct {
	TestGroup      string   `json:"test_group,omitempty"`
	TestList       []string `json:"test_list,omitempty"`
	Namespace      string   `json:"namespace"`
	VerboseMode    bool     `json:"verbose_mode"`
	KeepNamespace  bool     `json:"keep_namespace"`
	KubeconfigPath string   `json:"kubeconfig_path,omitempty"`
}

// generateJSONSummary creates a comprehensive JSON summary file with test results and infrastructure status
func generateJSONSummary(timedResults []core.TimedTestResult, testNames []string, sharedTime *core.SharedTimestamp, tester *core.Tester, ctx context.Context, testConfig TestConfiguration) error {
	// Ensure test_results directory exists
	if err := os.MkdirAll("test_results", 0755); err != nil {
		return fmt.Errorf("failed to create test_results directory: %v", err)
	}

	// Collect real infrastructure data using InfrastructureCollector instead of static mock data
	infrastructureCollector := core.NewInfrastructureCollector(tester.GetClientset(), testConfig.VerboseMode)
	infrastructure := infrastructureCollector.CollectInfrastructure(ctx)

	// Prepare test entries with comprehensive details
	var tests []TestSummaryEntry
	var totalTime float64
	passed := 0
	failed := 0
	skipped := 0

	for i, timedResult := range timedResults {
		testName := testNames[i]
		duration := timedResult.EndTime.Sub(timedResult.StartTime).Seconds()
		totalTime += duration

		if timedResult.TestResult.Success {
			passed++
		} else {
			failed++
		}

		// Determine expected vs actual outcome
		expectedOutcome := "PASS"
		actualOutcome := "PASS"
		if !timedResult.TestResult.Success {
			actualOutcome = "FAIL"
		}

		// Collect verbose error logs for failed tests
		var verboseErrorLogs []string
		var errorDetails []string
		if !timedResult.TestResult.Success {
			errorDetails = append(errorDetails, timedResult.TestResult.Message)
			// Add details as verbose logs
			verboseErrorLogs = append(verboseErrorLogs, timedResult.TestResult.Details...)
		}

		entry := TestSummaryEntry{
			Name:             testName,
			Success:          timedResult.TestResult.Success,
			ExpectedOutcome:  expectedOutcome,
			ActualOutcome:    actualOutcome,
			Message:          timedResult.TestResult.Message,
			StartTime:        timedResult.StartTime,
			EndTime:          timedResult.EndTime,
			Duration:         duration,
			Details:          timedResult.TestResult.Details,
			ErrorDetails:     errorDetails,
			VerboseErrorLogs: verboseErrorLogs,
			// TODO: Steps and CommandsExecuted would need to be collected during test execution
		}
		tests = append(tests, entry)
	}

	// Calculate success rate
	successRate := 0.0
	if len(timedResults) > 0 {
		successRate = float64(passed) / float64(len(timedResults)) * 100.0
	}

	// Determine overall health status based on real infrastructure data
	overallHealthStatus := "HEALTHY"
	if failed > 0 {
		overallHealthStatus = "UNHEALTHY"
	} else if infrastructure.HasCriticalErrors() {
		overallHealthStatus = "WARNING"
	}

	// Create comprehensive summary result
	summary := TestSuiteResult{
		Timestamp:           sharedTime.GetTime(),
		TestConfiguration:   testConfig,
		Infrastructure:      infrastructure,
		TotalTests:          len(timedResults),
		PassedTests:         passed,
		FailedTests:         failed,
		SkippedTests:        skipped,
		SuccessRate:         successRate,
		TotalTime:           totalTime,
		Tests:               tests,
		OverallHealthStatus: overallHealthStatus,
	}

	// Create JSON file
	jsonPath := sharedTime.GetJSONFilePath()
	file, err := os.Create(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to create JSON summary file: %v", err)
	}
	defer file.Close()

	// Write JSON with pretty formatting
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("failed to write JSON summary: %v", err)
	}

	fmt.Printf("✅ Comprehensive JSON summary written to: %s\n", jsonPath)
	return nil
}

// generateRecommendationsFromRealInfrastructure creates actionable recommendations based on real infrastructure data
func generateRecommendationsFromRealInfrastructure(infrastructure *core.ClusterInfrastructure, failed, passed int) []string {
	var recommendations []string

	if failed > 0 {
		recommendations = append(recommendations, "Review failed test error details for specific connectivity issues")
		recommendations = append(recommendations, "Check pod logs and events for deployment issues")

		// Use real CNI provider information
		if infrastructure.CNIProvider == "cilium" {
			recommendations = append(recommendations, "Verify Cilium agent status on all nodes")
		} else if infrastructure.CNIProvider != "" {
			recommendations = append(recommendations, fmt.Sprintf("Check %s CNI health and configuration", infrastructure.CNIProvider))
		}
	}

	if infrastructure.HasCriticalErrors() {
		recommendations = append(recommendations, "Investigate cluster infrastructure issues")
		for _, err := range infrastructure.CollectionErrors {
			if strings.Contains(err, "cilium") || strings.Contains(err, "CNI") {
				recommendations = append(recommendations, "Check CNI configuration and policies")
				break
			}
		}
	}

	if infrastructure.NodeCount < 2 {
		recommendations = append(recommendations, "Consider adding more nodes for high availability testing")
	}

	// Platform-specific recommendations
	switch infrastructure.Platform {
	case "kind":
		recommendations = append(recommendations, "Kind cluster detected - ensure LoadBalancer support if testing external services")
	case "eks":
		recommendations = append(recommendations, "AWS EKS detected - verify VPC CNI and security group configurations")
	case "gke":
		recommendations = append(recommendations, "Google GKE detected - check GKE network policy settings")
	case "aks":
		recommendations = append(recommendations, "Azure AKS detected - verify Azure CNI and network security groups")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "All tests passed successfully - cluster networking is healthy")
		recommendations = append(recommendations, fmt.Sprintf("Cluster running %s with %s CNI is functioning optimally", infrastructure.KubernetesVersion, infrastructure.CNIProvider))
	}

	return recommendations
}

// generateTroubleshootingTipsFromRealInfrastructure provides specific troubleshooting guidance based on real infrastructure
func generateTroubleshootingTipsFromRealInfrastructure(infrastructure *core.ClusterInfrastructure, tests []TestSummaryEntry) []string {
	var tips []string

	for _, test := range tests {
		if !test.Success {
			switch test.Name {
			case "Pod-to-Pod Connectivity":
				if infrastructure.CNIProvider == "cilium" {
					tips = append(tips, "Check Cilium pod logs: kubectl logs -n kube-system -l k8s-app=cilium")
					tips = append(tips, "Verify Cilium network policies: kubectl get ciliumnetworkpolicies --all-namespaces")
				} else {
					tips = append(tips, fmt.Sprintf("Check %s CNI pod logs and configuration", infrastructure.CNIProvider))
				}
				tips = append(tips, "Verify node network connectivity and firewall rules")
			case "DNS Resolution":
				tips = append(tips, "Check CoreDNS status: kubectl get pods -n kube-system -l k8s-app=kube-dns")
				if infrastructure.CNIProvider == "cilium" {
					tips = append(tips, "Verify DNS policy configuration in Cilium")
				}
			case "Service-to-Pod Connectivity":
				tips = append(tips, "Check service endpoints: kubectl get endpoints")
				tips = append(tips, "Verify kube-proxy configuration and iptables rules")
			}
		}
	}

	// Add infrastructure-specific troubleshooting tips
	if infrastructure.HasCriticalErrors() {
		tips = append(tips, "Infrastructure collection errors detected:")
		for _, err := range infrastructure.CollectionErrors {
			tips = append(tips, fmt.Sprintf("  - %s", err))
		}
	}

	// Platform-specific troubleshooting
	switch infrastructure.Platform {
	case "kind":
		tips = append(tips, "Kind cluster troubleshooting: docker ps, kind get clusters")
	case "eks":
		tips = append(tips, "AWS EKS troubleshooting: check VPC, security groups, and IAM roles")
	case "gke":
		tips = append(tips, "Google GKE troubleshooting: gcloud container clusters describe")
	case "aks":
		tips = append(tips, "Azure AKS troubleshooting: az aks show, check NSGs")
	}

	if len(tips) == 0 {
		tips = append(tips, "All tests passed - no troubleshooting required")
		tips = append(tips, "For ongoing monitoring, consider setting up regular connectivity tests")
	}

	// Add general troubleshooting tips
	tips = append(tips, "Use 'kubectl describe' commands to get detailed resource information")
	tips = append(tips, "Check cluster events: kubectl get events --sort-by=.metadata.creationTimestamp")
	tips = append(tips, fmt.Sprintf("Cluster info: %s", infrastructure.GetInfrastructureSummary()))

	return tips
}

// Global logger instance
var logger *core.Logger

// Test registry - maps test names to their functions
type TestEntry struct {
	Name     string
	Function func(context.Context) core.TestResult
}

type TestEntryWithConfig struct {
	Name     string
	Function func(context.Context, core.TestConfig) core.TestResult
}

// Comprehensive test registry with all available individual tests
var availableTests = map[string]TestEntry{
	// Networking tests (from networking/configs.go)
	"pod-to-pod-same-node":  {"Pod-to-Pod Same-Node Connectivity", nil},
	"pod-to-pod-cross-node": {"Pod-to-Pod Cross-Node Connectivity", nil},
	"service-clusterip":     {"Service ClusterIP Connectivity", nil},
	"service-nodeport":      {"Service NodePort Connectivity", nil},
	"service-loadbalancer":  {"Service LoadBalancer Connectivity", nil},
	"service-cross-node":    {"Cross-Node Service Connectivity", nil},
	"dns-resolution":        {"DNS Resolution", nil},

	// L3 Policies (from l3/configs.go)
	"cidr-ingress":       {"CIDR Ingress Policy Test", nil},
	"cidr-egress":        {"CIDR Egress Policy Test", nil},
	"cidr-except":        {"CIDR With Except Policy Test", nil},
	"endpoints-label":    {"Endpoints Label Selector Policy Test", nil},
	"entities-based":     {"Entities Based Policy Test", nil},
	"dns-based":          {"DNS Based Policy Test", nil},
	"node-selector":      {"Traditional Node Selector Policy Test", nil},
	"pod-node-name":      {"Pod Node Name Policy Test", nil},
	"node-cidr":          {"Node CIDR Policy Test", nil},
	"node-based":         {"Node Based Policy Clusterwide Test", nil},
	"kubernetes-service": {"Kubernetes Service Policy Test", nil},
	"allow-all":          {"Allow All Policy Test", nil},
	"deny-all":           {"Deny All Policy Test", nil},

	// L4 Policies (from l4/configs.go)
	"tcp-port-ingress": {"TCP Port Ingress Policy Test", nil},
	"tcp-port-egress":  {"TCP Port Egress Policy Test", nil},
	"port-range":       {"Port Range Policy Test", nil},
	"multiple-port":    {"Multiple Port Policy Test", nil},
	"icmp-type":        {"ICMP Type Policy Test", nil},
	"icmpv6-type":      {"ICMPv6 Type Policy Test", nil},
	"mixed-icmp":       {"Mixed ICMP Policy Test", nil},
	"basic-sni":        {"Basic SNI Policy Test", nil},
	"multi-domain-sni": {"Multi Domain SNI Policy Test", nil},
	"combined-l4-sni":  {"Combined L4 SNI Policy Test", nil},

	// L7 Policies (from l7/configs.go)
	"basic-http-get":    {"Basic HTTP GET Policy Test", nil},
	"http-with-headers": {"HTTP With Headers Policy Test", nil},
	"path-method":       {"Path Method Policy Test", nil},
	"dns-matchname":     {"DNS Match Name Policy Test", nil},
	"dns-matchpattern":  {"DNS Match Pattern Policy Test", nil},
}

// testAliases maps user-friendly test names to their actual test IDs
// This allows users to use intuitive names like "pod-to-pod" or "tcp-port"
var testAliases = map[string][]string{
	// Networking Test Aliases
	"pod-to-pod":       {"pod-to-pod-cross-node"}, // Default to cross-node for broader testing
	"pod-connectivity": {"pod-to-pod-same-node", "pod-to-pod-cross-node"},
	"service":          {"service-clusterip"},
	"clusterip":        {"service-clusterip"},
	"nodeport":         {"service-nodeport"},
	"loadbalancer":     {"service-loadbalancer"},
	"dns":              {"dns-resolution"},

	// L3 Policy Test Aliases
	"cidr":      {"cidr-ingress", "cidr-egress"},
	"endpoints": {"endpoints-label"},
	"entities":  {"entities-based"},
	"node":      {"node-selector", "node-based", "node-cidr"},

	// L4 Policy Test Aliases
	"tcp-port": {"tcp-port-ingress"},
	"tcp":      {"tcp-port-ingress", "tcp-port-egress"},
	"port":     {"tcp-port-ingress", "port-range", "multiple-port"},
	"icmp":     {"icmp-type", "icmpv6-type"},
	"sni":      {"basic-sni"},

	// L7 Policy Test Aliases
	"http":      {"basic-http-get"},
	"http-get":  {"basic-http-get"},
	"dns-match": {"dns-matchname"},

	// Cross-category aliases
	"connectivity": {"pod-to-pod-cross-node", "service-clusterip", "dns-resolution"},
	"basic":        {"pod-to-pod-cross-node", "service-clusterip", "dns-resolution", "basic-http-get"},
}

// Test groups updated to match actual config structures
var testGroups = map[string][]string{
	"networking": {
		"pod-to-pod-same-node", "pod-to-pod-cross-node", "service-clusterip",
		"service-nodeport", "service-loadbalancer", "service-cross-node", "dns-resolution",
	},
	"l3-policies": {
		"cidr-ingress", "cidr-egress", "cidr-except",
		"endpoints-label", "entities-based", "dns-based",
		"node-selector", "pod-node-name", "node-cidr", "node-based",
		"kubernetes-service", "allow-all", "deny-all",
	},
	"l4-policies": {
		"tcp-port-ingress", "tcp-port-egress", "port-range", "multiple-port",
		"icmp-type", "icmpv6-type", "mixed-icmp",
		"basic-sni", "multi-domain-sni", "combined-l4-sni",
	},
	"l7-policies": {
		"basic-http-get", "http-with-headers", "path-method",
		"dns-matchname", "dns-matchpattern",
	},
}

// Default test list when no --test-list or --test-group is specified
var defaultTests = []string{"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"}

// groupNameAliases maps user-friendly group names to internal group names
var groupNameAliases = map[string]string{
	"l3":         "l3-policies",
	"l4":         "l4-policies",
	"l7":         "l7-policies",
	"networking": "networking",
	// Also allow the full names
	"l3-policies": "l3-policies",
	"l4-policies": "l4-policies",
	"l7-policies": "l7-policies",
}

// resolveGroupName converts user-friendly group names to internal group names
func resolveGroupName(userGroupName string) string {
	if internalName, exists := groupNameAliases[userGroupName]; exists {
		return internalName
	}
	return userGroupName // Return as-is if no alias found
}

// parseCommaSeparatedValues parses comma-separated values and trims whitespace
func parseCommaSeparatedValues(input string) []string {
	var result []string

	// Split by comma and clean up each value
	parts := strings.Split(input, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// validateIndividualTests validates individual test names and returns only valid ones
// This function now supports both exact test IDs and user-friendly aliases
func validateIndividualTests(testNames []string) []string {
	var validTests []string

	for _, testName := range testNames {
		// CRITICAL FIX: Always prioritize exact test ID matches over aliases
		// This prevents "pod-node-name" from being confused with "node" alias
		if _, exists := availableTests[testName]; exists {
			fmt.Printf("DEBUG: Found direct test match: %s\n", testName)
			validTests = append(validTests, testName)
		} else if aliasedTests, isAlias := testAliases[testName]; isAlias {
			// Only expand aliases for names that are NOT direct test IDs
			fmt.Printf("INFO: Expanding alias '%s' to tests: %v\n", testName, aliasedTests)

			// Validate each aliased test exists
			for _, aliasedTest := range aliasedTests {
				if _, exists := availableTests[aliasedTest]; exists {
					validTests = append(validTests, aliasedTest)
				} else {
					fmt.Printf("WARNING: Aliased test '%s' (from alias '%s') not found, skipping\n", aliasedTest, testName)
				}
			}
		} else {
			fmt.Printf("WARNING: Unknown test '%s', skipping\n", testName)
		}
	}

	return validTests
}

// executeIndividualTest executes a single test by finding and running only that specific test
func executeIndividualTest(testName string, tester *core.Tester, ctx context.Context, verbose bool) core.TestResult {
	// Emit test start event for real-time UI updates
	emitSSEEvent(map[string]interface{}{
		"type":      "test_start",
		"testName":  testName,
		"timestamp": time.Now().Format(time.RFC3339),
	})

	startTime := time.Now()

	var result core.TestResult

	// Try to find the test in networking configs first
	if networkingConfig := getNetworkingTestConfig(testName); networkingConfig != nil {
		result = executeSingleNetworkingTest(*networkingConfig, tester, ctx, verbose)
	} else if l3Config := getL3TestConfig(testName); l3Config != nil {
		// Try to find the test in L3 configs
		result = executeSinglePolicyTest(*l3Config, tester, ctx, verbose)
	} else if l4Config := getL4TestConfig(testName); l4Config != nil {
		// Try to find the test in L4 configs
		result = executeSinglePolicyTest(*l4Config, tester, ctx, verbose)
	} else if l7Config := getL7TestConfig(testName); l7Config != nil {
		// Try to find the test in L7 configs
		result = executeSinglePolicyTest(*l7Config, tester, ctx, verbose)
	} else {
		result = core.TestResult{
			Success: false,
			Message: fmt.Sprintf("Test '%s' not found in any test configuration", testName),
		}
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime).Seconds()

	// Generate user-friendly message for the test result
	userMessage := generateUserMessageForTest(testName, result, duration)

	// Emit test complete event with user message for real-time UI updates
	emitSSEEvent(map[string]interface{}{
		"type":        "test_complete",
		"testName":    testName,
		"success":     result.Success,
		"duration":    duration,
		"summary":     result.Message,
		"userMessage": userMessage,
		"timestamp":   endTime.Format(time.RFC3339),
	})

	return result
}

// getNetworkingTestConfig finds a networking test config by test ID
func getNetworkingTestConfig(testId string) *core.PolicyTestConfig {
	for _, config := range networking.NetworkingTestConfigs {
		if config.TestId == testId {
			return &config
		}
	}
	return nil
}

// getL3TestConfig finds an L3 test config by test ID
func getL3TestConfig(testId string) *core.PolicyTestConfig {
	for _, config := range l3.L3TestConfigs {
		if config.TestId == testId {
			return &config
		}
	}
	return nil
}

// getL4TestConfig finds an L4 test config by test ID
func getL4TestConfig(testId string) *core.PolicyTestConfig {
	// Check if testId is actually in the L4 test group
	for _, testInGroup := range testGroups["l4-policies"] {
		if testInGroup == testId {
			// Only create config if it's actually an L4 test
			return &core.PolicyTestConfig{
				GroupId:    "l4-policies",
				TestId:     testId,
				TestTitle:  fmt.Sprintf("L4 Policy Test: %s", testId),
				PolicyPath: fmt.Sprintf("cilium-policies/8-l4-policies/*/%s*.yaml", testId),
			}
		}
	}
	return nil // Return nil if not found, allowing L7 check to proceed
}

// getL7TestConfig finds an L7 test config by test ID
func getL7TestConfig(testId string) *core.PolicyTestConfig {
	// Use the exported L7TestConfigs from l7 package
	for _, config := range l7.L7TestConfigs {
		if config.TestId == testId {
			return &config
		}
	}
	return nil
}

// executeSingleNetworkingTest executes a single networking test using real test execution
func executeSingleNetworkingTest(config core.PolicyTestConfig, tester *core.Tester, ctx context.Context, verbose bool) core.TestResult {
	// Run ONLY the specific networking test by passing the test ID as requestedTests parameter
	results := networking.TestNetworkingPoliciesSequential(tester, ctx, []string{config.TestId}, verbose)

	// Return the result from the single test execution
	if len(results) > 0 {
		result := results[0]
		return core.TestResult{
			Success: result.Success,
			Message: fmt.Sprintf("Networking test '%s': %s", config.TestId, result.Message),
			Details: result.Details,
		}
	}

	return core.TestResult{
		Success: false,
		Message: fmt.Sprintf("Failed to execute networking test '%s'", config.TestId),
	}
}

// executeSinglePolicyTest executes a single policy test using real policy framework
func executeSinglePolicyTest(config core.PolicyTestConfig, tester *core.Tester, ctx context.Context, verbose bool) core.TestResult {
	// Create a custom test group with only this single test config
	// This ensures we run ONLY the requested test, not the entire subgroup
	singleTestGroup := core.PolicyTestGroup{
		Name:        fmt.Sprintf("individual-%s", config.TestId),
		GroupId:     config.GroupId,
		TestConfigs: []core.PolicyTestConfig{config},
	}

	// Execute the single test using the common policy framework
	switch config.GroupId {
	case "l3-policies":
		// Use the L3 framework to execute only this single test
		results := executeSingleL3PolicyTest(singleTestGroup, tester, ctx, verbose)
		if len(results) > 0 {
			result := results[0]
			return core.TestResult{
				Success: result.Success,
				Message: fmt.Sprintf("L3 policy test '%s': %s", config.TestId, result.Message),
				Details: result.Details,
			}
		}

	case "l4-policies":
		// Use the L4 framework to execute only this single test
		results := executeSingleL4PolicyTest(singleTestGroup, tester, ctx, verbose)
		if len(results) > 0 {
			result := results[0]
			return core.TestResult{
				Success: result.Success,
				Message: fmt.Sprintf("L4 policy test '%s': %s", config.TestId, result.Message),
				Details: result.Details,
			}
		}

	case "l7-policies":
		// Use the L7 framework to execute only this single test
		results := executeSingleL7PolicyTest(singleTestGroup, tester, ctx, verbose)
		if len(results) > 0 {
			result := results[0]
			return core.TestResult{
				Success: result.Success,
				Message: fmt.Sprintf("L7 policy test '%s': %s", config.TestId, result.Message),
				Details: result.Details,
			}
		}
	}

	return core.TestResult{
		Success: false,
		Message: fmt.Sprintf("Failed to execute policy test '%s' from group '%s'", config.TestId, config.GroupId),
	}
}

// executeSingleL3PolicyTest executes a single L3 policy test using the L3 framework
func executeSingleL3PolicyTest(testGroup core.PolicyTestGroup, tester *core.Tester, ctx context.Context, verbose bool) []core.TestResult {
	// Create a temporary subgroup map with only our single test
	tempSubgroups := map[string][]string{
		"single-test": {testGroup.TestConfigs[0].TestId},
	}

	// Temporarily override the L3PolicySubgroups for this execution
	originalSubgroups := l3.L3PolicySubgroups
	l3.L3PolicySubgroups = tempSubgroups
	defer func() {
		l3.L3PolicySubgroups = originalSubgroups
	}()

	// Run the single test using the L3 framework
	return l3.TestL3PoliciesSequential(tester, ctx, []string{"single-test"}, verbose)
}

// executeSingleL4PolicyTest executes a single L4 policy test using the L4 framework
func executeSingleL4PolicyTest(testGroup core.PolicyTestGroup, tester *core.Tester, ctx context.Context, verbose bool) []core.TestResult {
	// Create a temporary subgroup map with only our single test
	tempSubgroups := map[string][]string{
		"single-test": {testGroup.TestConfigs[0].TestId},
	}

	// Temporarily override the L4PolicySubgroups for this execution
	originalSubgroups := l4.L4PolicySubgroups
	l4.L4PolicySubgroups = tempSubgroups
	defer func() {
		l4.L4PolicySubgroups = originalSubgroups
	}()

	// Run the single test using the L4 framework
	return l4.TestL4PoliciesSequential(tester, ctx, []string{"single-test"}, verbose)
}

// executeSingleL7PolicyTest executes a single L7 policy test using the L7 framework
func executeSingleL7PolicyTest(testGroup core.PolicyTestGroup, tester *core.Tester, ctx context.Context, verbose bool) []core.TestResult {
	// Create a temporary subgroup map with only our single test
	tempSubgroups := map[string][]string{
		"single-test": {testGroup.TestConfigs[0].TestId},
	}

	// Temporarily override the L7PolicySubgroups for this execution
	originalSubgroups := l7.L7PolicySubgroups
	l7.L7PolicySubgroups = tempSubgroups
	defer func() {
		l7.L7PolicySubgroups = originalSubgroups
	}()

	// Run the single test using the L7 framework
	return l7.TestL7PoliciesSequential(tester, ctx, []string{"single-test"}, verbose)
}

// getL3SubgroupForTest determines which L3 subgroup a test belongs to
func getL3SubgroupForTest(testId string) string {
	for subgroup, tests := range l3.L3PolicySubgroups {
		for _, test := range tests {
			if test == testId {
				return subgroup
			}
		}
	}
	return ""
}

// getL4SubgroupForTest determines which L4 subgroup a test belongs to
func getL4SubgroupForTest(testId string) string {
	for subgroup, tests := range l4.L4PolicySubgroups {
		for _, test := range tests {
			if test == testId {
				return subgroup
			}
		}
	}
	return ""
}

// getL7SubgroupForTest determines which L7 subgroup a test belongs to
func getL7SubgroupForTest(testId string) string {
	for subgroup, tests := range l7.L7PolicySubgroups {
		for _, test := range tests {
			if test == testId {
				return subgroup
			}
		}
	}
	return ""
}

// ensureBinaryIsUpToDate checks if the k8s_diagnostic binary needs to be rebuilt
func ensureBinaryIsUpToDate() error {
	binaryPath := "k8s-diagnostic"

	// Check if binary exists
	binaryInfo, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("🔨 Binary not found, building...\n")
			return buildBinary()
		}
		return fmt.Errorf("failed to check binary status: %v", err)
	}

	// Get binary modification time
	binaryModTime := binaryInfo.ModTime()

	// Check if any Go source files are newer than the binary
	sourceModified := false
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only check .go files
		if filepath.Ext(path) == ".go" {
			if info.ModTime().After(binaryModTime) {
				sourceModified = true
				return filepath.SkipDir // Stop walking once we find a newer file
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("⚠️  Warning: Could not check source file timestamps: %v\n", err)
		fmt.Printf("🔨 Rebuilding binary to be safe...\n")
		return buildBinary()
	}

	if sourceModified {
		fmt.Printf("🔨 Source changes detected, rebuilding binary...\n")
		return buildBinary()
	}

	return nil // Binary is up to date
}

// buildBinary rebuilds the k8s_diagnostic binary
func buildBinary() error {
	cmd := exec.Command("go", "build", "-o", "k8s-diagnostic", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	fmt.Printf("✅ Binary updated successfully\n")
	return nil
}

// setupSignalHandling sets up signal handling for immediate termination
func setupSignalHandling() (context.Context, context.CancelFunc) {
	// Create a context that will be cancelled when we receive a signal
	ctx, cancel := context.WithCancel(context.Background())

	// Create channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start goroutine to handle signals
	go func() {
		<-sigChan
		fmt.Printf("\n🛑 Process terminated by user\n")
		os.Exit(130) // Exit code 130 = terminated by Ctrl+C
	}()

	return ctx, cancel
}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run diagnostic tests in Kubernetes cluster",
	Long: `Run comprehensive diagnostic tests within a Kubernetes cluster.

Available test groups:
- networking: All network connectivity tests
- policies: Basic network policy tests
- l3-policies (or l3): Layer 3 Cilium network policy tests
- l4-policies (or l4): Layer 4 Cilium network policy tests
- l7-policies (or l7): Layer 7 Cilium network policy tests

## Usage Patterns:

### Single Test Group:
  k8s-diagnostic test networking
  k8s-diagnostic test l3-policies
  k8s-diagnostic test l3            # Short alias for l3-policies
  k8s-diagnostic test l4            # Short alias for l4-policies

### Multiple Test Groups (sequential execution):
  k8s-diagnostic test groups: l3,l4,networking
  k8s-diagnostic test groups: networking,l7-policies
  k8s-diagnostic test "groups:" "l3,l4"

### Individual Tests (comma-separated):
  k8s-diagnostic test list: tcp-port-ingress,basic-sni,dns-based
  k8s-diagnostic test list: cidr-ingress,service-clusterip
  k8s-diagnostic test "list:" "dns-resolution,pod-to-pod-same-node"

### Flag-based Usage (legacy):
  k8s-diagnostic test --test-group networking
  k8s-diagnostic test --test-list tcp-port-ingress,basic-sni

The tool will use the current kubectl context unless --kubeconfig is specified.
All test resources will be created in the specified namespace (default: diagnostic-test).`,
	Run: func(cmd *cobra.Command, args []string) {
		// Auto-build check and execution
		if err := ensureBinaryIsUpToDate(); err != nil {
			fmt.Printf("❌ Build failed: %v\n", err)
			return
		}

		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")
		testList, _ := cmd.Flags().GetStringSlice("test-list")
		testGroup, _ := cmd.Flags().GetString("test-group")

		// Set up signal handling for graceful shutdown
		ctx, cancel := setupSignalHandling()
		defer cancel()

		// Enhanced positional argument parsing to support new patterns
		if len(args) > 0 {
			// Parse the argument patterns:
			// 1. Single group: "networking", "l3-policies", etc.
			// 2. Multiple groups: "groups: l3,l4,networking"
			// 3. Individual tests: "list: tcp-port-ingress,basic-sni,dns-based"

			firstArg := args[0]

			// Pattern 2: Multiple groups - "groups: l3,l4,networking"
			if len(args) >= 2 && firstArg == "groups:" {
				// Join remaining args and split by comma to handle spaces
				groupsStr := ""
				for i := 1; i < len(args); i++ {
					if i > 1 {
						groupsStr += " "
					}
					groupsStr += args[i]
				}

				// Parse comma-separated groups
				requestedGroups := parseCommaSeparatedValues(groupsStr)
				if len(requestedGroups) > 0 {
					fmt.Printf("Detected multiple test groups: %v\n", requestedGroups)

					// Validate and resolve group names
					var validGroups []string
					for _, group := range requestedGroups {
						resolvedGroupName := resolveGroupName(group)
						if _, exists := testGroups[resolvedGroupName]; exists {
							validGroups = append(validGroups, resolvedGroupName)
						} else {
							fmt.Printf("WARNING: Unknown test group '%s', skipping\n", group)
						}
					}

					if len(validGroups) > 0 {
						if len(validGroups) == 1 {
							testGroup = validGroups[0]
							fmt.Printf("Running single valid group: %s\n", testGroup)
						} else {
							// Set flag to run multiple groups sequentially
							fmt.Printf("Running multiple groups sequentially: %v\n", validGroups)
							testGroup = "multi-group"
							testList = validGroups // Store groups in testList for multi-group execution
						}
					} else {
						fmt.Printf("ERROR: No valid test groups found\n")
						return
					}
				}
				args = []string{} // Clear processed args

				// Pattern 3: Individual tests - "list: tcp-port-ingress,basic-sni,dns-based"
			} else if len(args) >= 2 && firstArg == "list:" {
				// Join remaining args and split by comma to handle spaces
				testsStr := ""
				for i := 1; i < len(args); i++ {
					if i > 1 {
						testsStr += " "
					}
					testsStr += args[i]
				}

				// Parse comma-separated test names
				requestedTests := parseCommaSeparatedValues(testsStr)
				if len(requestedTests) > 0 {
					fmt.Printf("Detected individual tests: %v\n", requestedTests)
					testList = requestedTests

					// Validate individual test names
					validTests := validateIndividualTests(requestedTests)
					if len(validTests) != len(requestedTests) {
						fmt.Printf("WARNING: Some test names may be invalid. Valid tests will be executed.\n")
					}
					testList = validTests
				}
				args = []string{} // Clear processed args

				// Pattern 1: Single group or single test - "networking", "tcp-port-ingress"
			} else if len(args) == 1 {
				argValue := args[0]

				// Try to resolve group name aliases first (l3 -> l3-policies, etc.)
				resolvedGroupName := resolveGroupName(argValue)

				// Check if it's a known test group (using resolved name)
				if _, exists := testGroups[resolvedGroupName]; exists {
					testGroup = resolvedGroupName
					if resolvedGroupName != argValue {
						fmt.Printf("Detected test group: %s (resolved from %s)\n", resolvedGroupName, argValue)
					} else {
						fmt.Printf("Detected test group: %s\n", argValue)
					}
					args = []string{}
					testList = []string{}
				} else if _, exists := availableTests[argValue]; exists {
					// It's a single individual test
					testList = []string{argValue}
					fmt.Printf("Detected individual test: %s\n", argValue)
					args = []string{}
				} else {
					// Unknown argument, treat as potential test name and let validation handle it
					testList = args
					fmt.Printf("Unknown argument '%s', treating as individual test name\n", argValue)
				}

			} else {
				// Multiple arguments without "groups:" or "list:" prefix
				// Detect common mistake patterns before treating as individual tests
				if len(args) >= 2 {
					// Common pattern: "group l3" or "test l3"
					if args[0] == "group" && len(args) == 2 {
						suggestedGroup := resolveGroupName(args[1])
						if _, exists := testGroups[suggestedGroup]; exists {
							fmt.Printf("❌ ERROR: Invalid syntax 'group %s'\n", args[1])
							fmt.Printf("💡 Did you mean: './k8s_diagnostic test %s' ?\n", args[1])
							fmt.Printf("💡 Or try: './k8s_diagnostic test groups: %s'\n", args[1])
							return
						} else {
							fmt.Printf("❌ ERROR: Invalid syntax 'group %s' with unknown group\n", args[1])
							showAvailableOptions()
							return
						}
					}
					// Pattern: "test something" (likely meant as group)
					if args[0] == "test" && len(args) == 2 {
						fmt.Printf("❌ ERROR: Invalid nested 'test' command\n")
						fmt.Printf("💡 Try: './k8s_diagnostic test %s' (without extra 'test')\n", args[1])
						return
					}
				}

				// Treat as individual test names for backward compatibility
				testList = args
				fmt.Printf("Detected multiple arguments as individual tests: %v\n", args)
			}
		}

		// CRITICAL: Early validation before any expensive operations
		if err := performEarlyValidation(testGroup, testList); err != nil {
			fmt.Printf("❌ %s\n", err.Error())
			return
		}

		// PHASE 2: Initialize command-level singleton logger
		// This ensures all events (cleanup, setup, tests, results) go to the same JSONL file
		if err := core.InitializeCommandLogger(namespace, verbose); err != nil {
			fmt.Printf("ERROR: Failed to initialize command logger: %v\n", err)
			return
		}
		defer core.CloseCommandLogger()

		// Get the singleton command logger for legacy compatibility
		commandLogger := core.GetCommandLogger()
		if commandLogger == nil {
			fmt.Printf("ERROR: Failed to get command logger\n")
			return
		}

		// Get the underlying multi-channel logger for existing code compatibility
		logger := commandLogger.GetMultiChannelLogger().GetVerboseLogger()
		sharedTime := commandLogger.GetSharedTimestamp()

		// Set global verbosity flag
		if verbose {
			core.SetVerbosity(true)
		} else {
			core.SetVerbosity(false)
		}

		logger.LogInfo("Starting Kubernetes connectivity diagnostic tests")
		logger.LogInfo("Configuration: namespace=%s, verbose=%t", namespace, verbose)
		if testGroup != "" {
			logger.LogInfo("Using test group: %s", testGroup)
		}

		// Create tester with timeout context, combining signal handling with timeout
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 3*time.Minute)
		defer timeoutCancel()

		tester, err := core.NewTester(kubeconfig, namespace, verbose)
		if err != nil {
			logger.LogError("Failed to create diagnostic tester: %v", err)
			return
		}

		fmt.Printf("Running connectivity diagnostic tests in namespace '%s'\n\n", namespace)

		// Create namespace before running tests
		fmt.Printf("🔍 Setting up test environment...\n")
		if err := tester.EnsureNamespace(timeoutCtx); err != nil {
			// Check if we were interrupted by signal
			if timeoutCtx.Err() == context.Canceled {
				fmt.Printf("\n⚠️  Test setup interrupted by user signal\n")
				return
			}
			fmt.Printf("ERROR: Failed to create namespace %s: %v\n", namespace, err)
			return
		}
		fmt.Printf("✅ Namespace %s ready\n", namespace)

		// INFRASTRUCTURE COLLECTION: Run once before ANY tests start
		fmt.Printf("🔍 Collecting cluster infrastructure information...\n")
		var infrastructure *core.ClusterInfrastructure

		infrastructureCollector := core.NewInfrastructureCollector(tester.GetClientset(), verbose)
		infrastructure = infrastructureCollector.CollectInfrastructure(timeoutCtx)

		// Handle infrastructure collection errors gracefully
		if infrastructure.HasCriticalErrors() {
			fmt.Printf("⚠️  Infrastructure collection completed with issues: couldn't verify infrastructure settings\n")
			if verbose {
				fmt.Printf("Issues encountered:\n")
				for _, err := range infrastructure.CollectionErrors {
					fmt.Printf("  - %s\n", err)
				}
			}
		} else {
			fmt.Printf("✅ Infrastructure collection completed: %s\n", infrastructure.GetInfrastructureSummary())
		}

		// Universal pre-test cleanup before ANY testing begins - uses clean hierarchical format
		fmt.Printf("🧹 Pre-test cleanup phase...\n")
		tester.CleanupAllTestResources(timeoutCtx, false)
		fmt.Printf("✅ Pre-test cleanup completed\n")

		// Run all diagnostic tests
		fmt.Printf("🧪 Running diagnostic tests...\n")

		// Log suite start with infrastructure context to JSONL
		if commandLogger != nil {
			multiChannelLogger := commandLogger.GetMultiChannelLogger()
			if multiChannelLogger != nil {
				// Determine total test count and groups for logging
				var totalTestsForLogging int
				var groupsForLogging []string

				if testGroup != "" {
					if testGroup == "multi-group" {
						groupsForLogging = testList
						// Estimate total tests across multiple groups
						for _, groupName := range testList {
							if tests, exists := testGroups[groupName]; exists {
								totalTestsForLogging += len(tests)
							}
						}
					} else {
						groupsForLogging = []string{testGroup}
						if tests, exists := testGroups[testGroup]; exists {
							totalTestsForLogging = len(tests)
						}
					}
				} else if len(testList) > 0 {
					groupsForLogging = []string{"individual-tests"}
					totalTestsForLogging = len(testList)
				}

				// Log suite start with infrastructure context
				multiChannelLogger.LogSuiteStartWithInfrastructure(totalTestsForLogging, groupsForLogging, infrastructure)
			}
		}

		// Store timed test results for JSON output
		var timedResults []core.TimedTestResult
		var testNames []string

		// Check for test group first
		if testGroup != "" {
			if testGroup == "multi-group" {
				// Execute multiple groups sequentially
				fmt.Printf("Executing multiple test groups sequentially: %v\n", testList)

				for i, groupName := range testList {
					fmt.Printf("\n🔄 Running group %d/%d: %s\n", i+1, len(testList), groupName)
					fmt.Printf("================================================================================\n")

					switch groupName {
					case "networking":
						netCtx, netCancel := context.WithTimeout(ctx, 30*time.Minute)
						networkingResults := networking.TestNetworkingPoliciesSequential(tester, netCtx, nil, verbose)
						netCancel()

						// Convert results to timed results for reporting
						for j, result := range networkingResults {
							timedResult := core.TimedTestResult{
								TestResult: result,
								StartTime:  time.Now(),
								EndTime:    time.Now(),
							}
							timedResults = append(timedResults, timedResult)
							testNames = append(testNames, fmt.Sprintf("Networking-Test-%d", j+1))
						}

					case "l3-policies":
						l3Ctx, l3Cancel := context.WithTimeout(timeoutCtx, 45*time.Minute)
						l3Results := l3.TestL3PoliciesSequential(tester, l3Ctx, nil, verbose)
						l3Cancel()

						// Convert results to timed results for reporting
						for j, result := range l3Results {
							timedResult := core.TimedTestResult{
								TestResult: result,
								StartTime:  time.Now(),
								EndTime:    time.Now(),
							}
							timedResults = append(timedResults, timedResult)
							testNames = append(testNames, fmt.Sprintf("L3-Test-%d", j+1))
						}

					case "l4-policies":
						l4Ctx, l4Cancel := context.WithTimeout(ctx, 60*time.Minute)
						l4Results := l4.TestL4PoliciesSequential(tester, l4Ctx, nil, verbose)
						l4Cancel()

						// Convert results to timed results for reporting
						for j, result := range l4Results {
							timedResult := core.TimedTestResult{
								TestResult: result,
								StartTime:  time.Now(),
								EndTime:    time.Now(),
							}
							timedResults = append(timedResults, timedResult)
							testNames = append(testNames, fmt.Sprintf("L4-Test-%d", j+1))
						}

					case "l7-policies":
						l7Ctx, l7Cancel := context.WithTimeout(ctx, 30*time.Minute)
						l7Results := l7.TestL7PoliciesSequential(tester, l7Ctx, nil, verbose)
						l7Cancel()

						// Convert results to timed results for reporting
						for j, result := range l7Results {
							timedResult := core.TimedTestResult{
								TestResult: result,
								StartTime:  time.Now(),
								EndTime:    time.Now(),
							}
							timedResults = append(timedResults, timedResult)
							testNames = append(testNames, fmt.Sprintf("L7-Test-%d", j+1))
						}

					default:
						fmt.Printf("WARNING: Unknown group '%s' in multi-group execution\n", groupName)
					}

					// Add separator between groups (except for last group)
					if i < len(testList)-1 {
						fmt.Printf("\n⏳ Completed group: %s. Proceeding to next group...\n", groupName)
					}
				}

				fmt.Printf("\n✅ Completed all %d test groups\n", len(testList))

			} else if testGroup == "l3-policies" {
				// Special handling for L3 policies
				l3Subgroups, _ := cmd.Flags().GetStringSlice("l3-subgroups")
				fmt.Printf("Running L3 policy tests\n")

				l3Ctx, l3Cancel := context.WithTimeout(timeoutCtx, 45*time.Minute)
				defer l3Cancel()

				l3Results := l3.TestL3PoliciesSequential(tester, l3Ctx, l3Subgroups, verbose)

				// Check if we were interrupted by signal during L3 tests
				if l3Ctx.Err() == context.Canceled {
					fmt.Printf("\n⚠️  L3 policy tests interrupted by user signal\n")
					return
				}

				// Convert results to timed results for reporting
				for i, result := range l3Results {
					timedResult := core.TimedTestResult{
						TestResult: result,
						StartTime:  time.Now(),
						EndTime:    time.Now(),
					}
					timedResults = append(timedResults, timedResult)
					testNames = append(testNames, fmt.Sprintf("L3-Test-%d", i+1))
				}

			} else if testGroup == "l4-policies" {
				// Special handling for L4 policies
				l4Subgroups, _ := cmd.Flags().GetStringSlice("l4-subgroups")
				fmt.Printf("Running L4 policy tests\n")

				l4Ctx, l4Cancel := context.WithTimeout(ctx, 60*time.Minute)
				defer l4Cancel()

				l4Results := l4.TestL4PoliciesSequential(tester, l4Ctx, l4Subgroups, verbose)

				// Convert results to timed results for reporting
				for i, result := range l4Results {
					timedResult := core.TimedTestResult{
						TestResult: result,
						StartTime:  time.Now(),
						EndTime:    time.Now(),
					}
					timedResults = append(timedResults, timedResult)
					testNames = append(testNames, fmt.Sprintf("L4-Test-%d", i+1))
				}
			} else if testGroup == "l7-policies" {
				// Special handling for L7 policies
				l7Subgroups, _ := cmd.Flags().GetStringSlice("l7-subgroups")
				fmt.Printf("Running L7 policy tests\n")

				l7Ctx, l7Cancel := context.WithTimeout(ctx, 30*time.Minute)
				defer l7Cancel()

				l7Results := l7.TestL7PoliciesSequential(tester, l7Ctx, l7Subgroups, verbose)

				// Convert results to timed results for reporting
				for i, result := range l7Results {
					timedResult := core.TimedTestResult{
						TestResult: result,
						StartTime:  time.Now(),
						EndTime:    time.Now(),
					}
					timedResults = append(timedResults, timedResult)
					testNames = append(testNames, fmt.Sprintf("L7-Test-%d", i+1))
				}
			} else if testGroup == "networking" {
				// Special handling for networking tests
				fmt.Printf("Running networking tests\n")

				netCtx, netCancel := context.WithTimeout(ctx, 30*time.Minute)
				defer netCancel()

				networkingResults := networking.TestNetworkingPoliciesSequential(tester, netCtx, nil, verbose)

				// Check if we were interrupted by signal during networking tests
				if netCtx.Err() == context.Canceled {
					fmt.Printf("\n⚠️  Networking tests interrupted by user signal\n")
					return
				}

				// Convert results to timed results for reporting
				for i, result := range networkingResults {
					timedResult := core.TimedTestResult{
						TestResult: result,
						StartTime:  time.Now(),
						EndTime:    time.Now(),
					}
					timedResults = append(timedResults, timedResult)
					testNames = append(testNames, fmt.Sprintf("Networking-Test-%d", i+1))
				}
			} else {
				fmt.Printf("WARNING: Unknown test group '%s'\n", testGroup)
				logger.LogWarning("Unknown test group '%s'", testGroup)
			}
		} else if len(testList) > 0 {
			// Individual test execution - execute each test individually
			fmt.Printf("Running individual tests: %v\n", testList)

			for i, testName := range testList {
				fmt.Printf("Running test: %s\n", testName)

				// Record start time for this test
				startTime := time.Now()

				// Determine which test group this test belongs to and execute accordingly
				testResult := executeIndividualTest(testName, tester, ctx, verbose)

				// Record end time for this test
				endTime := time.Now()
				duration := endTime.Sub(startTime)

				// Display test result with timing
				if testResult.Success {
					fmt.Printf("✅ %s: PASSED (%.1fs)\n", testName, duration.Seconds())
				} else {
					fmt.Printf("❌ %s: FAILED (%.1fs)\n", testName, duration.Seconds())
				}

				// Add cleanup between tests (except after the last test)
				if i < len(testList)-1 {
					fmt.Printf("🧹 Cleaning up between tests...\n")
					tester.CleanupAllTestResources(ctx, false)
					fmt.Printf("✅ Inter-test cleanup completed\n")
				}

				// Convert result to timed result for reporting
				timedResult := core.TimedTestResult{
					TestResult: testResult,
					StartTime:  startTime,
					EndTime:    endTime,
				}
				timedResults = append(timedResults, timedResult)
				testNames = append(testNames, testName)
			}
		}

		// Extract basic test results for summary calculations
		var testResults []core.TestResult
		for _, timedResult := range timedResults {
			testResults = append(testResults, timedResult.TestResult)
		}

		// Calculate test statistics
		totalTests := len(testResults)
		passedTests := 0
		failedTests := 0

		for _, result := range testResults {
			if result.Success {
				passedTests++
			} else {
				failedTests++
			}
		}

		// Display detailed test summary only for non-L3/L4/L7/networking groups/subgroups to avoid duplication
		// L3, L4, L7, and networking tests handle their own detailed summaries with enhanced format
		if (testGroup != "" && testGroup != "l3-policies" && testGroup != "l4-policies" && testGroup != "l7-policies" && testGroup != "networking") || (len(testList) > 1) {
			// Import the summary package to use enhanced formatting

			// Create display names mapping for individual tests
			displayNames := createDisplayNamesForIndividualTests(testNames)

			// Use the enhanced summary format that shows Expected/Received/Result
			diagnostic.FormatDetailedTestSummary(timedResults, testNames, displayNames)
		}

		// Display overall test summary only for non-L3 groups to avoid duplication
		// L3 policies handle their own complete summaries internally
		if testGroup == "" || testGroup != "l3-policies" {
			fmt.Printf("\n📊 Test Summary:\n")
			fmt.Printf("  Total Tests: %d, Passed: %d, Failed: %d\n", totalTests, passedTests, failedTests)

			// Display final result
			fmt.Printf("\n")
			if failedTests == 0 {
				fmt.Printf("🎉 Overall Result: All %d diagnostic tests passed\n", totalTests)
			} else {
				fmt.Printf("🛑 Overall Result: %d of %d diagnostic tests failed\n", failedTests, totalTests)
			}
		}

		// Generate JSON summary file (only if there are test results)
		if len(timedResults) > 0 {
			// Create shared timestamp for consistent file naming
			if sharedTime == nil {
				sharedTime = core.NewSharedTimestamp()
			}

			// Create test configuration for JSON summary
			testConfig := TestConfiguration{
				TestGroup:      testGroup,
				TestList:       testList,
				Namespace:      namespace,
				VerboseMode:    verbose,
				KeepNamespace:  cmd.Flags().Changed("keep-namespace"),
				KubeconfigPath: kubeconfig,
			}

			// Generate JSON summary with test results
			if err := generateJSONSummary(timedResults, testNames, sharedTime, tester, ctx, testConfig); err != nil {
				fmt.Printf("Warning: Failed to generate JSON summary: %v\n", err)
			}
		}

		// Final cleanup - only if not L3 policies (they handle their own cleanup)
		keepNamespace, _ := cmd.Flags().GetBool("keep-namespace")
		if !keepNamespace && testGroup != "l3-policies" {
			// Check if we were interrupted before cleanup
			if timeoutCtx.Err() == context.Canceled {
				fmt.Printf("\n⚠️  Final cleanup skipped due to user interruption\n")
			} else {
				tester.CleanupAllTestResources(timeoutCtx, false)
			}
		}

		fmt.Printf("\n📁 Detailed results are stored in JSON file in the test_results/ folder for further analysis\n")
	},
}

// getAllSubgroupNames returns a list of all available L3 subgroup names
func getAllSubgroupNames() []string {
	var subgroups []string
	for subgroup := range l3.L3PolicySubgroups {
		subgroups = append(subgroups, subgroup)
	}
	return subgroups
}

// countTestsInSubgroups counts the total number of tests in the given L3 subgroups
func countTestsInSubgroups(subgroups []string) int {
	totalTests := 0
	for _, subgroup := range subgroups {
		if tests, exists := l3.L3PolicySubgroups[subgroup]; exists {
			totalTests += len(tests)
		}
	}
	return totalTests
}

// getAllL4SubgroupNames returns a list of all available L4 subgroup names
func getAllL4SubgroupNames() []string {
	var subgroups []string
	for subgroup := range l4.L4PolicySubgroups {
		subgroups = append(subgroups, subgroup)
	}
	return subgroups
}

// countTestsInL4Subgroups counts the total number of tests in the given L4 subgroups
func countTestsInL4Subgroups(subgroups []string) int {
	totalTests := 0
	for _, subgroup := range subgroups {
		if tests, exists := l4.L4PolicySubgroups[subgroup]; exists {
			totalTests += len(tests)
		}
	}
	return totalTests
}

// createDisplayNamesForIndividualTests creates display name mappings for individual tests
func createDisplayNamesForIndividualTests(testNames []string) map[string]string {
	displayNames := make(map[string]string)

	for _, testName := range testNames {
		if entry, exists := availableTests[testName]; exists {
			displayNames[testName] = entry.Name
		} else {
			// Convert test key to a readable display name as fallback
			displayNames[testName] = strings.Title(strings.ReplaceAll(testName, "-", " "))
		}
	}

	return displayNames
}

// performEarlyValidation performs validation before expensive operations
func performEarlyValidation(testGroup string, testList []string) error {
	// Skip validation if both are empty (will use defaults)
	if testGroup == "" && len(testList) == 0 {
		return nil
	}

	// Validate test group if specified
	if testGroup != "" && testGroup != "multi-group" {
		if _, exists := testGroups[testGroup]; !exists {
			// Check if it might be an unresolved alias
			resolvedGroup := resolveGroupName(testGroup)
			if _, exists := testGroups[resolvedGroup]; !exists {
				return fmt.Errorf("Unknown test group '%s'\n💡 Available groups: %s\n💡 Try: './k8s_diagnostic test --help' for usage examples",
					testGroup, strings.Join(getAvailableGroups(), ", "))
			}
		}
	}

	// Validate individual tests if specified
	if len(testList) > 0 && testGroup != "multi-group" {
		invalidTests := findInvalidTests(testList)
		if len(invalidTests) > 0 {
			suggestions := generateTestSuggestions(invalidTests)
			return fmt.Errorf("Unknown test(s): %s\n%s\n💡 Use './k8s_diagnostic test --help' for available options",
				strings.Join(invalidTests, ", "), suggestions)
		}
	}

	return nil
}

// findInvalidTests returns a list of test names that don't exist
func findInvalidTests(testNames []string) []string {
	var invalidTests []string

	for _, testName := range testNames {
		// Check if it's a direct test ID or valid alias
		if _, exists := availableTests[testName]; !exists {
			if _, isAlias := testAliases[testName]; !isAlias {
				invalidTests = append(invalidTests, testName)
			}
		}
	}

	return invalidTests
}

// generateTestSuggestions provides helpful suggestions for invalid test names
func generateTestSuggestions(invalidTests []string) string {
	var suggestions []string

	for _, invalidTest := range invalidTests {
		// Look for partial matches in available tests
		var matches []string
		for testId := range availableTests {
			if strings.Contains(testId, invalidTest) || strings.Contains(invalidTest, testId) {
				matches = append(matches, testId)
			}
		}

		// Look for partial matches in aliases
		for alias := range testAliases {
			if strings.Contains(alias, invalidTest) || strings.Contains(invalidTest, alias) {
				matches = append(matches, alias)
			}
		}

		if len(matches) > 0 {
			if len(matches) > 3 {
				matches = matches[:3] // Limit suggestions
			}
			suggestions = append(suggestions, fmt.Sprintf("💡 Did you mean '%s'? Try: %s",
				invalidTest, strings.Join(matches, ", ")))
		}
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "💡 Available tests include: pod-to-pod-cross-node, service-clusterip, dns-resolution")
		suggestions = append(suggestions, "💡 Available groups: networking, l3-policies, l4-policies, l7-policies")
	}

	return strings.Join(suggestions, "\n")
}

// getAvailableGroups returns a list of available test group names
func getAvailableGroups() []string {
	var groups []string
	for group := range testGroups {
		groups = append(groups, group)
	}
	return groups
}

// emitSSEEvent emits a structured JSON event to stdout for real-time API streaming
func emitSSEEvent(eventData map[string]interface{}) {
	// Only emit SSE events when running in batch mode (detected via environment variable)
	if os.Getenv("BATCH_TEST_ID") != "" {
		jsonData, err := json.Marshal(eventData)
		if err == nil {
			fmt.Printf("SSE_EVENT:%s\n", string(jsonData))
		}
	}
}

// generateUserMessageForTest creates context-aware user messages for individual test results
func generateUserMessageForTest(testName string, result core.TestResult, duration float64) map[string]interface{} {
	// Create infrastructure-aware user message generator
	var infrastructure *core.ClusterInfrastructure
	if tester, err := core.NewTester("", "diagnostic-test", false); err == nil {
		collector := core.NewInfrastructureCollector(tester.GetClientset(), false)
		infrastructure = collector.CollectInfrastructure(context.Background())
	}

	generator := core.NewUserMessageGenerator(testName, infrastructure)

	// Generate appropriate message based on test result
	var userMsg core.UserMessage
	if result.Success {
		userMsg = generator.GenerateTestSummary(testName, true, duration, nil)
	} else {
		userMsg = generator.GenerateTestSummary(testName, false, duration, nil)
	}

	// Convert to map for JSON serialization
	return map[string]interface{}{
		"title":       userMsg.Title,
		"description": userMsg.Description,
		"context":     userMsg.Context,
		"hints":       userMsg.Hints,
		"status":      userMsg.Status,
	}
}

// showAvailableOptions displays available test groups and examples
func showAvailableOptions() {
	fmt.Printf("\n📋 Available Test Groups:\n")
	for group, tests := range testGroups {
		fmt.Printf("  • %s (%d tests)\n", group, len(tests))
	}

	fmt.Printf("\n📋 Usage Examples:\n")
	fmt.Printf("  • Single group: './k8s_diagnostic test l3'\n")
	fmt.Printf("  • Multiple groups: './k8s_diagnostic test groups: l3,l4,networking'\n")
	fmt.Printf("  • Individual tests: './k8s_diagnostic test list: dns-resolution,service-clusterip'\n")

	fmt.Printf("\n📋 Popular Individual Tests:\n")
	popularTests := []string{"pod-to-pod-cross-node", "service-clusterip", "dns-resolution", "cidr-ingress", "tcp-port-ingress"}
	for _, test := range popularTests {
		if entry, exists := availableTests[test]; exists {
			fmt.Printf("  • %s (%s)\n", test, entry.Name)
		}
	}

	fmt.Printf("\n💡 For complete help: './k8s_diagnostic test --help'\n")
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Local flags for the test command
	testCmd.Flags().StringP("namespace", "n", "diagnostic-test", "namespace to run diagnostic tests in")
	testCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
	testCmd.Flags().String("test-group", "", "run tests by group: networking, l3-policies, l4-policies, l7-policies")
	testCmd.Flags().Bool("keep-namespace", false, "keep the test namespace after tests complete")
	testCmd.Flags().StringSlice("test-list", nil, "comma-separated list of tests to run")

	// L3 policy subgroup selection
	testCmd.Flags().StringSlice("l3-subgroups", nil, "L3 policy subgroups to run: ip-cidr,endpoint,entities,dns,node,service")

	// L4 policy subgroup selection
	testCmd.Flags().StringSlice("l4-subgroups", nil, "L4 policy subgroups to run: port,icmp,tls-sni")

	// L7 policy subgroup selection
	testCmd.Flags().StringSlice("l7-subgroups", nil, "L7 policy subgroups to run: http,dns")
}
