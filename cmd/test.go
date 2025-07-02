package cmd

import (
	"context"
	"fmt"
	"time"

	"k8s-diagnostic/internal/diagnostic"

	"github.com/spf13/cobra"
)

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

		// Record overall start time
		overallStartTime := time.Now()

		if verbose {
			fmt.Printf("🔍 Configuration:\n")
			fmt.Printf("  - Namespace: %s\n", namespace)
			if kubeconfig != "" {
				fmt.Printf("  - Kubeconfig: %s\n", kubeconfig)
			} else {
				fmt.Printf("  - Using default kubectl context\n")
			}
			fmt.Printf("\n")
		}

		fmt.Printf("🚀 Running connectivity diagnostic tests in namespace '%s'\n\n", namespace)

		// Create tester with default context (no timeout)
		ctx := context.Background()

		tester, err := diagnostic.NewTester(kubeconfig, namespace)
		if err != nil {
			fmt.Printf("❌ Failed to create diagnostic tester: %v\n", err)
			return
		}

		// Create namespace before running tests
		fmt.Printf("🔧 Setting up test environment...\n")
		if err := tester.EnsureNamespace(ctx); err != nil {
			fmt.Printf("❌ Failed to create namespace %s: %v\n", namespace, err)
			return
		}
		fmt.Printf("✓ Namespace %s ready\n\n", namespace)

		// Run all diagnostic tests
		fmt.Printf("🧪 Running diagnostic tests...\n")

		// Store timed test results for JSON output
		var timedResults []diagnostic.TimedTestResult
		var testNames []string

		// Execute all tests with timing
		executeTimedTest(1, "Pod-to-Pod Connectivity", tester.TestPodToPodConnectivity, ctx, verbose, &timedResults, &testNames)
		executeTimedTest(2, "Service to Pod Connectivity", tester.TestServiceToPodConnectivity, ctx, verbose, &timedResults, &testNames)
		executeTimedTest(3, "Cross-Node Service Connectivity", tester.TestCrossNodeServiceConnectivity, ctx, verbose, &timedResults, &testNames)
		executeTimedTest(4, "DNS Resolution", tester.TestDNSResolution, ctx, verbose, &timedResults, &testNames)

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
				overallResult.Details = append(overallResult.Details, fmt.Sprintf("✅ %s: %s", testNames[i], result.Message))
			} else {
				overallResult.Details = append(overallResult.Details, fmt.Sprintf("❌ %s: %s", testNames[i], result.Message))
			}
		}

		result := overallResult

		// Clean up namespace after all tests
		fmt.Printf("\n🧹 Cleaning up test environment...\n")
		if err := tester.CleanupNamespace(ctx); err != nil {
			fmt.Printf("⚠️  Warning: Failed to cleanup namespace %s: %v\n", namespace, err)
		} else {
			fmt.Printf("✓ Namespace %s cleaned up\n", namespace)
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
			fmt.Printf("⚠️  Warning: Failed to save JSON report: %v\n", err)
		} else {
			fmt.Printf("📄 JSON report saved: test_results/%s\n", jsonReport.ExecutionInfo.Filename)
		}

		// Display test summary
		fmt.Printf("\n📊 Test Summary:\n")
		fmt.Printf("  Total Tests: %d, Passed: %d, Failed: %d\n", totalTests, passedTests, failedTests)

		if len(passedTestNames) > 0 {
			fmt.Printf("  ✅ Passed Tests:\n")
			for _, testName := range passedTestNames {
				fmt.Printf("    • %s\n", testName)
			}
		}

		if len(failedTestNames) > 0 {
			fmt.Printf("  ❌ Failed Tests:\n")
			for _, testName := range failedTestNames {
				fmt.Printf("    • %s\n", testName)
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
			fmt.Printf("✅ Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("💡 Run with --verbose for detailed test steps\n")
			}
		} else {
			fmt.Printf("❌ Overall Result: %s\n", result.Message)
			if !verbose && len(result.Details) > 0 {
				fmt.Printf("📋 Individual Test Results:\n")
				for _, detail := range result.Details {
					fmt.Printf("  %s\n", detail)
				}
			}
		}
	},
}

// executeTimedTest is a helper function that captures timing information for each test
func executeTimedTest(testNum int, testName string, testFunc func(context.Context) diagnostic.TestResult,
	ctx context.Context, verbose bool, timedResults *[]diagnostic.TimedTestResult, testNames *[]string) {

	fmt.Printf("📋 Test %d: %s\n", testNum, testName)

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
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Local flags for the test command
	testCmd.Flags().StringP("namespace", "n", "diagnostic-test", "namespace to run diagnostic tests in")
	testCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
}
