package cmd

import (
	"context"
	"fmt"
	"time"

	"k8s-diagnostic/internal/diagnostic"

	"github.com/spf13/cobra"
)

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

// Default test list when no --test-list is specified
var defaultTests = []string{"pod-to-pod", "service-to-pod", "cross-node", "dns"}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run diagnostic tests in Kubernetes cluster",
	Long: `Run comprehensive connectivity diagnostic tests within a Kubernetes cluster.

Tests include:
- Pod-to-Pod Connectivity: Creates two netshoot pods on different worker nodes and tests ping connectivity
- Service-to-Pod Connectivity: Creates nginx deployment + service and tests HTTP connectivity and load balancing
- Cross-Node Service Connectivity: Tests service connectivity from a remote node to validate kube-proxy inter-node routing
- DNS Resolution: Tests service DNS resolution including FQDN, short names, and pod-to-pod DNS

The tool will use the current kubectl context unless --kubeconfig is specified.
All test resources will be created in the specified namespace (default: diagnostic-test).`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")
		placement, _ := cmd.Flags().GetString("placement")
		testAll, _ := cmd.Flags().GetBool("test-all")
		testList, _ := cmd.Flags().GetStringSlice("test-list")

		// Create tester
		ctx := context.Background()
		tester, err := diagnostic.NewTester(kubeconfig, namespace)
		if err != nil {
			fmt.Printf("ERROR: Failed to create diagnostic tester: %v\n", err)
			return
		}

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
		fmt.Printf("Setting up test environment...\n")
		if err := tester.EnsureNamespace(ctx); err != nil {
			fmt.Printf("ERROR: Failed to create namespace %s: %v\n", namespace, err)
			return
		}
		fmt.Printf("Namespace %s ready\n\n", namespace)

		// Run all diagnostic tests
		fmt.Printf("Running diagnostic tests...\n")

		// Store timed test results for JSON output
		var timedResults []diagnostic.TimedTestResult
		var testNames []string

		// Determine which tests to run
		testsToRun := defaultTests
		if testAll {
			// --test-all flag takes priority: run all available tests
			testsToRun = []string{"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"}
		} else if len(testList) > 0 {
			// Handle special case: "all" means run all available tests (backwards compatibility)
			if len(testList) == 1 && testList[0] == "all" {
				testsToRun = []string{"pod-to-pod", "service-to-pod", "cross-node", "dns", "nodeport", "loadbalancer"}
			} else {
				testsToRun = testList
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

		// Get the keep-namespace flag
		keepNamespace, _ := cmd.Flags().GetBool("keep-namespace")

		// Determine if we should clean up the namespace
		// - Only clean up if running all tests AND not explicitly keeping namespace
		// - For selective tests, always keep namespace by default
		shouldCleanup := testAll && !keepNamespace

		if shouldCleanup {
			// Clean up namespace after tests
			fmt.Printf("\nCleaning up test environment...\n")
			if err := tester.CleanupNamespace(ctx); err != nil {
				fmt.Printf("WARNING: Failed to cleanup namespace %s: %v\n", namespace, err)
			} else {
				fmt.Printf("Namespace %s cleaned up\n", namespace)
			}
		} else {
			fmt.Printf("\nKeeping namespace %s for future test runs\n", namespace)
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

		if err := diagnostic.SaveJSONReport(&jsonReport); err != nil {
			fmt.Printf("WARNING: Failed to save JSON report: %v\n", err)
		} else {
			fmt.Printf("JSON report saved: test_results/%s\n", jsonReport.ExecutionInfo.Filename)
		}

		// Display test summary
		fmt.Printf("\nTest Summary:\n")
		fmt.Printf("  Total Tests: %d, Passed: %d, Failed: %d\n", totalTests, passedTests, failedTests)

		if len(passedTestNames) > 0 {
			fmt.Printf("  Passed Tests:\n")
			for _, testName := range passedTestNames {
				fmt.Printf("    ✓ %s\n", testName)
			}
		}

		if len(failedTestNames) > 0 {
			fmt.Printf("  Failed Tests:\n")
			for _, testName := range failedTestNames {
				fmt.Printf("    ✗ %s\n", testName)
			}
		}

		// Display detailed results in verbose mode
		if verbose {
			fmt.Printf("\nDetailed Test Results:\n")
			for _, detail := range result.Details {
				fmt.Printf("  %s\n", detail)
			}
		}

		// Display final result
		fmt.Printf("\n")
		if result.Success {
			fmt.Printf("✓ Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("Run with --verbose for detailed test steps\n")
			}
		} else {
			fmt.Printf("✗ Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("Individual Test Results:\n")
				for _, detail := range result.Details {
					fmt.Printf("  %s\n", detail)
				}
			}
		}

		// Final reminder about JSON file availability
		fmt.Printf("\nDetailed results are stored in JSON file in the test_results/ folder for further analysis\n")
	},
}

// executeTimedTestWithConfig is a helper function that captures timing information for tests that need configuration
func executeTimedTestWithConfig(testNum int, testName string, testFunc func(context.Context, diagnostic.TestConfig) diagnostic.TestResult,
	ctx context.Context, verbose bool, config diagnostic.TestConfig, timedResults *[]diagnostic.TimedTestResult, testNames *[]string) {

	fmt.Printf("Test %d: %s\n", testNum, testName)

	// Capture start time
	startTime := time.Now()

	// Execute test with config
	result := testFunc(ctx, config)

	// Capture end time
	endTime := time.Now()

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
		fmt.Printf("✓ Test %d PASSED: %s\n", testNum, result.Message)
	} else {
		fmt.Printf("✗ Test %d FAILED: %s\n", testNum, result.Message)
	}

	// Show verbose details if enabled
	if verbose && len(result.Details) > 0 {
		fmt.Printf("  Details:\n")
		for _, detail := range result.Details {
			fmt.Printf("    %s\n", detail)
		}
	}
	fmt.Printf("\n")
}

// executeTimedTest is a helper function that captures timing information for each test
func executeTimedTest(testNum int, testName string, testFunc func(context.Context) diagnostic.TestResult,
	ctx context.Context, verbose bool, timedResults *[]diagnostic.TimedTestResult, testNames *[]string) {

	fmt.Printf("Test %d: %s\n", testNum, testName)

	// Capture start time
	startTime := time.Now()

	// Execute test
	result := testFunc(ctx)

	// Capture end time
	endTime := time.Now()

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
		fmt.Printf("✓ Test %d PASSED: %s\n", testNum, result.Message)
	} else {
		fmt.Printf("✗ Test %d FAILED: %s\n", testNum, result.Message)
	}

	// Show verbose details if enabled
	if verbose && len(result.Details) > 0 {
		fmt.Printf("  Details:\n")
		for _, detail := range result.Details {
			fmt.Printf("    %s\n", detail)
		}
	}
	fmt.Printf("\n")
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Local flags for the test command
	testCmd.Flags().StringP("namespace", "n", "diagnostic-test", "namespace to run diagnostic tests in")
	testCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
	testCmd.Flags().String("placement", "both", "pod placement strategy for pod-to-pod connectivity: same-node|cross-node|both")
	testCmd.Flags().Bool("test-all", false, "run all available tests")
	testCmd.Flags().Bool("keep-namespace", false, "keep the test namespace after tests complete (useful for running multiple test sequences)")
	testCmd.Flags().StringSlice("test-list", nil, "comma-separated list of tests to run: pod-to-pod,service-to-pod,cross-node,dns,nodeport,loadbalancer (default: pod-to-pod,service-to-pod,cross-node,dns)")
}
