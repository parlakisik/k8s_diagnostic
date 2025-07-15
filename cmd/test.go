package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s-diagnostic/internal/diagnostic"

	"github.com/spf13/cobra"
)

// Global logger instance
var logger *diagnostic.Logger

// Test registry - maps test names to their functions
type TestEntry struct {
	Name     string
	Function func(context.Context) diagnostic.TestResult
}

type TestEntryWithConfig struct {
	Name     string
	Function func(context.Context, diagnostic.TestConfig) diagnostic.TestResult
}

// Available tests registry
var availableTests = map[string]TestEntry{
	"pod-to-pod":     {"Pod-to-Pod Connectivity", nil}, // Special handling with config
	"service-to-pod": {"Service to Pod Connectivity", nil},
	"cross-node":     {"Cross-Node Service Connectivity", nil},
	"dns":            {"DNS Resolution", nil},
	"nodeport":       {"NodePort Service Connectivity", nil},
	"loadbalancer":   {"LoadBalancer Service Connectivity", nil},
}

// Test groups for logical organization
var testGroups = map[string][]string{
	"networking": {"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"},
	// Future groups will be added here, e.g.:
	// "firewall": {"ingress-policy", "egress-policy"},
	// "storage": {"pv-binding", "pvc-access"},
}

// Default test list when no --test-list or --test-group is specified
var defaultTests = []string{"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run diagnostic tests in Kubernetes cluster",
	Long: `Run comprehensive diagnostic tests within a Kubernetes cluster.

Available test groups:
- networking: All network connectivity tests

Networking tests include:
- Pod-to-Pod Connectivity: Creates two netshoot pods on different worker nodes and tests ping connectivity
- Service-to-Pod Connectivity: Creates nginx deployment + service and tests HTTP connectivity and load balancing
- Cross-Node Service Connectivity: Tests service connectivity from a remote node to validate kube-proxy inter-node routing
- DNS Resolution: Tests service DNS resolution including FQDN, short names, and pod-to-pod DNS
- NodePort Service Connectivity: Tests external access to services through node ports
- LoadBalancer Service Connectivity: Tests LoadBalancer service functionality

The tool will use the current kubectl context unless --kubeconfig is specified.
All test resources will be created in the specified namespace (default: diagnostic-test).`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")
		placement, _ := cmd.Flags().GetString("placement")
		testList, _ := cmd.Flags().GetStringSlice("test-list")
		testGroup, _ := cmd.Flags().GetString("test-group")

		// Initialize logger with debug level when verbose mode is enabled
		var err error
		if verbose {
			logger, err = diagnostic.NewLoggerWithLevel(true, diagnostic.DEBUG) // true = console output enabled
		} else {
			logger, err = diagnostic.NewLoggerWithLevel(true, diagnostic.INFO)
		}

		if err != nil {
			fmt.Printf("ERROR: Failed to initialize logger: %v\n", err)
			return
		}
		defer logger.Close()

		logger.LogInfo("Starting Kubernetes connectivity diagnostic tests")
		logger.LogInfo("Configuration: namespace=%s, verbose=%t", namespace, verbose)
		if testGroup != "" {
			logger.LogInfo("Using test group: %s", testGroup)
		}
		if kubeconfig != "" {
			logger.LogInfo("Using kubeconfig file: %s", kubeconfig)
		} else {
			logger.LogInfo("Using default kubectl context")
		}

		// Create tester
		ctx := context.Background()
		logger.LogDebug("Creating diagnostic tester with kubeconfig: %s, namespace: %s", kubeconfig, namespace)
		tester, err := diagnostic.NewTester(kubeconfig, namespace)
		if err != nil {
			logger.LogError("Failed to create diagnostic tester: %v", err)
			return
		}
		logger.LogDebug("Tester created successfully")

		// Record overall start time
		overallStartTime := time.Now()

		if verbose {
			fmt.Printf("Configuration:\n")
			fmt.Printf("  - Namespace: %s\n", namespace)
			if kubeconfig != "" {
				fmt.Printf("  - Kubeconfig: %s\n", kubeconfig)
			} else {
				fmt.Printf("  - Using default kubectl context\n")
			}
			fmt.Printf("\n")
		}

		fmt.Printf("Running connectivity diagnostic tests in namespace '%s'\n\n", namespace)

		// Create namespace before running tests
		fmt.Printf("🔍 Setting up test environment...\n")
		if err := tester.EnsureNamespace(ctx); err != nil {
			fmt.Printf("ERROR: Failed to create namespace %s: %v\n", namespace, err)
			return
		}
		fmt.Printf("✅ Namespace %s ready\n\n", namespace)

		// Run all diagnostic tests
		fmt.Printf("🧪 Running diagnostic tests...\n")

		// Store timed test results for JSON output
		var timedResults []diagnostic.TimedTestResult
		var testNames []string

		// Determine which tests to run
		testsToRun := defaultTests

		// Check for test group first
		if testGroup != "" {
			if group, exists := testGroups[testGroup]; exists {
				testsToRun = group
				logger.LogInfo("Running tests in group: %s", testGroup)
			} else {
				fmt.Printf("WARNING: Unknown test group '%s' - using defaults\n", testGroup)
				logger.LogWarning("Unknown test group '%s' - using defaults", testGroup)
			}
		} else if len(testList) > 0 {
			// Handle special case: "all" means run all available tests (backwards compatibility)
			if len(testList) == 1 && testList[0] == "all" {
				testsToRun = defaultTests
			} else {
				testsToRun = testList
			}
		}

		// Get the block-pod-connectivity flag
		blockPodConnectivity, _ := cmd.Flags().GetBool("block-pod-connectivity")

		// Log when the block connectivity flag is enabled and apply policy if requested
		if blockPodConnectivity {
			fmt.Printf("\n⚠️  BLOCKING MODE: A Kubernetes NetworkPolicy will be applied to block pod connectivity\n\n")
			logger.LogWarning("Pod connectivity blocking enabled via --block-pod-connectivity flag")

			logger.LogInfo("Applying NetworkPolicy to block pod-to-pod traffic")
			if err := tester.ApplyNetworkPolicy(ctx); err != nil {
				logger.LogError("Failed to apply NetworkPolicy: %v", err)
				fmt.Printf("❌ Failed to apply NetworkPolicy: %v\n\n", err)
				fmt.Printf("Continuing with tests, but connectivity may not be blocked as requested.\n\n")
			} else {
				logger.LogInfo("Successfully applied NetworkPolicy to block pod-to-pod traffic")
				fmt.Printf("✅ Successfully applied NetworkPolicy to block pod-to-pod traffic\n\n")
			}
		}

		// Execute tests based on test registry
		testConfig := diagnostic.TestConfig{
			Placement: placement,
		}

		testNum := 1
		for _, testName := range testsToRun {
			testEntry, exists := availableTests[testName]
			if !exists {
				fmt.Printf("WARNING: Unknown test '%s' - skipping\n", testName)
				continue
			}

			// Special handling for tests that require config
			switch testName {
			case "pod-to-pod":
				executeTimedTestWithConfig(testNum, testEntry.Name, tester.TestPodToPodConnectivityWithConfig, ctx, verbose, testConfig, &timedResults, &testNames)
			case "service-to-pod":
				executeTimedTest(testNum, testEntry.Name, tester.TestServiceToPodConnectivity, ctx, verbose, &timedResults, &testNames)
			case "cross-node":
				executeTimedTest(testNum, testEntry.Name, tester.TestCrossNodeServiceConnectivity, ctx, verbose, &timedResults, &testNames)
			case "dns":
				executeTimedTest(testNum, testEntry.Name, tester.TestDNSResolution, ctx, verbose, &timedResults, &testNames)
			case "nodeport":
				executeTimedTest(testNum, testEntry.Name, tester.TestNodePortServiceConnectivity, ctx, verbose, &timedResults, &testNames)
			case "loadbalancer":
				executeTimedTest(testNum, testEntry.Name, tester.TestLoadBalancerServiceConnectivity, ctx, verbose, &timedResults, &testNames)
			}
			testNum++
		}

		// Record overall end time
		overallEndTime := time.Now()

		// Extract basic test results for summary calculations
		var testResults []diagnostic.TestResult
		for _, timedResult := range timedResults {
			testResults = append(testResults, timedResult.TestResult)
		}

		// Calculate test statistics
		totalTests := len(testResults)
		passedTests := 0
		failedTests := 0
		var passedTestNames []string
		var failedTestNames []string

		for i, result := range testResults {
			if result.Success {
				passedTests++
				passedTestNames = append(passedTestNames, testNames[i])
			} else {
				failedTests++
				failedTestNames = append(failedTestNames, testNames[i])
			}
		}

		// Determine overall result
		allTestsPassed := failedTests == 0
		var overallResult diagnostic.TestResult
		if allTestsPassed {
			overallResult = diagnostic.TestResult{
				Success: true,
				Message: fmt.Sprintf("All %d diagnostic tests passed", totalTests),
				Details: []string{},
			}
		} else {
			overallResult = diagnostic.TestResult{
				Success: false,
				Message: fmt.Sprintf("%d of %d diagnostic tests failed", failedTests, totalTests),
				Details: []string{},
			}
		}

		// Add individual test results to details
		for i, result := range testResults {
			if result.Success {
				overallResult.Details = append(overallResult.Details, fmt.Sprintf("✓ PASS: %s: %s", testNames[i], result.Message))
			} else {
				overallResult.Details = append(overallResult.Details, fmt.Sprintf("✗ FAIL: %s: %s", testNames[i], result.Message))
			}
		}

		result := overallResult

		// Clean up NetworkPolicy if it was applied, regardless of keep-namespace flag
		if blockPodConnectivity {
			logger.LogInfo("Removing NetworkPolicy")
			if err := tester.RemoveNetworkPolicy(ctx); err != nil {
				logger.LogWarning("Failed to remove NetworkPolicy: %v", err)
				fmt.Printf("⚠️ Warning: Failed to remove NetworkPolicy: %v\n", err)
				fmt.Printf("You may need to manually remove it: kubectl delete networkpolicy block-pod-ping -n %s\n\n", namespace)
			} else {
				logger.LogInfo("Successfully removed NetworkPolicy")
				fmt.Printf("✅ NetworkPolicy removed\n\n")
			}
		}

		// Get the keep-namespace flag
		keepNamespace, _ := cmd.Flags().GetBool("keep-namespace")

		// Determine if we should clean up the namespace
		// - Only clean up if running all default tests AND not explicitly keeping namespace
		// - For selective tests or specific groups, always keep namespace by default
		isRunningAllTests := len(testsToRun) == len(defaultTests)
		for i, test := range testsToRun {
			if i >= len(defaultTests) || test != defaultTests[i] {
				isRunningAllTests = false
				break
			}
		}
		shouldCleanup := isRunningAllTests && !keepNamespace

		if shouldCleanup {
			// Clean up namespace after tests
			logger.LogInfo("\n🧹 Cleaning up test environment...")
			logger.SetContext("Cleanup")
			if err := tester.CleanupNamespace(ctx); err != nil {
				logger.LogWarning("Failed to cleanup namespace %s: %v", namespace, err)
			} else {
				logger.LogInfo("Namespace %s cleaned up", namespace)
			}
			logger.ClearContext()
		} else {
			fmt.Printf("\n📝 Keeping namespace %s for future test runs\n", namespace)
			fmt.Printf("To delete the namespace manually: kubectl delete namespace %s\n", namespace)
		}

		// Generate and save JSON report
		kubeconfigSource := "default"
		if kubeconfig != "" {
			kubeconfigSource = kubeconfig
		}

		jsonReport := diagnostic.CreateJSONReport(
			namespace,
			kubeconfigSource,
			verbose,
			timedResults,
			testNames,
			overallStartTime,
			overallEndTime,
		)

		// Add log file information to the JSON report
		jsonReport.ExecutionInfo.LogFile = logger.GetLogFilename()

		// Save the JSON report
		if err := diagnostic.SaveJSONReport(&jsonReport); err != nil {
			logger.LogWarning("Failed to save JSON report: %v", err)
		} else {
			logger.LogInfo("JSON report saved: test_results/%s", jsonReport.ExecutionInfo.Filename)
		}

		// Display test summary
		fmt.Printf("\n📊 Test Summary:\n")
		fmt.Printf("  Total Tests: %d, Passed: %d, Failed: %d\n", totalTests, passedTests, failedTests)

		if len(passedTestNames) > 0 {
			fmt.Printf("  ✅ Passed Tests:\n")
			for _, testName := range passedTestNames {
				fmt.Printf("    ✅ %s\n", testName)
			}
		}

		if len(failedTestNames) > 0 {
			fmt.Printf("  ❌ Failed Tests:\n")
			for _, testName := range failedTestNames {
				fmt.Printf("    ❌ %s\n", testName)
			}
		}

		// Display detailed results in verbose mode
		if verbose {
			fmt.Printf("\n📋 Detailed Test Results:\n")
			for _, detail := range result.Details {
				fmt.Printf("  %s\n", detail)
			}
		}

		// Display final result
		fmt.Printf("\n")
		if result.Success {
			fmt.Printf("🎉 Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("💡 Run with --verbose for detailed test steps\n")
			}
		} else {
			fmt.Printf("🛑 Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("📋 Individual Test Results:\n")
				for _, detail := range result.Details {
					fmt.Printf("  %s\n", detail)
				}
			}
		}

		// Final reminder about JSON file availability
		fmt.Printf("\n📁 Detailed results are stored in JSON file in the test_results/ folder for further analysis\n")
	},
}

// executeTimedTestUnified is a unified helper function that captures timing information for tests with or without config
func executeTimedTestUnified(
	testNum int,
	testName string,
	ctx context.Context,
	verbose bool,
	timedResults *[]diagnostic.TimedTestResult,
	testNames *[]string,
	execute func() diagnostic.TestResult,
	logStartMessage string,
) {
	// Select emoji based on test name
	var testEmoji string
	switch {
	case strings.Contains(testName, "Pod-to-Pod"):
		testEmoji = "🔄"
	case strings.Contains(testName, "Service to Pod"):
		testEmoji = "🌐"
	case strings.Contains(testName, "Cross-Node"):
		testEmoji = "📡"
	case strings.Contains(testName, "DNS"):
		testEmoji = "🔤"
	case strings.Contains(testName, "NodePort"):
		testEmoji = "🚪"
	case strings.Contains(testName, "LoadBalancer"):
		testEmoji = "⚖️"
	default:
		testEmoji = "🧪"
	}
	fmt.Printf("Test %d: %s %s\n", testNum, testEmoji, testName)

	// Set test context in logger
	testContext := fmt.Sprintf("Test %d: %s", testNum, testName)
	logger.SetContext(testContext)

	// Log start message
	logger.LogInfo("%s", logStartMessage)

	// Capture start time
	startTime := time.Now()

	// Execute test function
	logger.LogDebug("Executing test function")
	result := execute()

	// Capture end time
	endTime := time.Now()
	executionTime := endTime.Sub(startTime)
	logger.LogInfo("Test completed in %.2f seconds", executionTime.Seconds())

	// Log test result details
	if result.Success {
		logger.LogInfo("Test PASSED: %s", result.Message)
	} else {
		logger.LogError("Test FAILED: %s", result.Message)
	}

	// Log detailed results
	for _, detail := range result.Details {
		logger.LogDebug("Detail: %s", detail)
	}

	// Log diagnostic info if available
	if result.DetailedDiagnostics != nil {
		if result.DetailedDiagnostics.FailureStage != "" {
			logger.LogWarning("Failure stage: %s", result.DetailedDiagnostics.FailureStage)
		}
		if result.DetailedDiagnostics.TechnicalError != "" {
			logger.LogError("Technical error: %s", result.DetailedDiagnostics.TechnicalError)
		}

		// Log command outputs
		for _, cmd := range result.DetailedDiagnostics.CommandOutputs {
			logger.CaptureCommandOutput(cmd)
		}

		// Log network context if available
		if result.DetailedDiagnostics.NetworkContext != nil {
			netContext := result.DetailedDiagnostics.NetworkContext
			logger.LogDebug("Network context: source=%s, target=%s",
				netContext.SourcePodIP, netContext.TargetPodIP)
		}

		// Log troubleshooting hints
		for _, hint := range result.DetailedDiagnostics.TroubleshootingHints {
			logger.LogInfo("Troubleshooting hint: %s", hint)
		}
	}

	// Create timed result
	timedResult := diagnostic.TimedTestResult{
		TestResult: result,
		StartTime:  startTime,
		EndTime:    endTime,
	}

	*timedResults = append(*timedResults, timedResult)
	*testNames = append(*testNames, testName)

	// Display result
	if result.Success {
		fmt.Printf("✅ Test %d PASSED: %s\n", testNum, result.Message)
	} else {
		fmt.Printf("❌ Test %d FAILED: %s\n", testNum, result.Message)
	}

	// Show verbose details if enabled
	if verbose && len(result.Details) > 0 {
		fmt.Printf("  Details:\n")
		for _, detail := range result.Details {
			fmt.Printf("    %s\n", detail)
		}
	}
	fmt.Printf("\n")

	// Clear test context
	logger.ClearContext()
}

// executeTimedTestWithConfig is a helper function that captures timing information for tests that need configuration
func executeTimedTestWithConfig(testNum int, testName string, testFunc func(context.Context, diagnostic.TestConfig) diagnostic.TestResult,
	ctx context.Context, verbose bool, config diagnostic.TestConfig, timedResults *[]diagnostic.TimedTestResult, testNames *[]string) {

	executeTimedTestUnified(
		testNum,
		testName,
		ctx,
		verbose,
		timedResults,
		testNames,
		func() diagnostic.TestResult {
			return testFunc(ctx, config)
		},
		fmt.Sprintf("Starting test with configuration: %+v", config),
	)
}

// executeTimedTest is a helper function that captures timing information for each test
func executeTimedTest(testNum int, testName string, testFunc func(context.Context) diagnostic.TestResult,
	ctx context.Context, verbose bool, timedResults *[]diagnostic.TimedTestResult, testNames *[]string) {

	executeTimedTestUnified(
		testNum,
		testName,
		ctx,
		verbose,
		timedResults,
		testNames,
		func() diagnostic.TestResult {
			return testFunc(ctx)
		},
		"Starting test",
	)
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Local flags for the test command
	testCmd.Flags().StringP("namespace", "n", "diagnostic-test", "namespace to run diagnostic tests in")
	testCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
	testCmd.Flags().String("placement", "both", "pod placement strategy for pod-to-pod connectivity: same-node|cross-node|both")
	testCmd.Flags().String("test-group", "", "run tests by group: networking (more groups coming soon)")
	testCmd.Flags().Bool("keep-namespace", false, "keep the test namespace after tests complete (useful for running multiple test sequences)")
	testCmd.Flags().StringSlice("test-list", nil, "comma-separated list of tests to run: pod-to-pod,service-to-pod,cross-node,dns,nodeport,loadbalancer")
	testCmd.Flags().Bool("block-pod-connectivity", false, "apply a Kubernetes NetworkPolicy to block pod-to-pod connectivity for demonstration purposes")
}
