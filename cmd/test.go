package cmd

import (
	"context"
	"fmt"

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
- Service-to-Pod Connectivity: Creates nginx deployment + service and tests DNS resolution and HTTP connectivity
- Cross-Node Service Connectivity: Tests service connectivity from a remote node to validate kube-proxy inter-node routing

The tool will use the current kubectl context unless --kubeconfig is specified.
All test resources will be created in the specified namespace (default: diagnostic-test).`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")

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

		// Store test results for summary
		var testResults []diagnostic.TestResult
		var testNames []string

		// Execute all tests using helper function
		executeTest(1, "Pod-to-Pod Connectivity", tester.TestPodToPodConnectivity, ctx, verbose, &testResults, &testNames)
		executeTest(2, "Service to Pod Connectivity", tester.TestServiceToPodConnectivity, ctx, verbose, &testResults, &testNames)
		executeTest(3, "Cross-Node Service Connectivity", tester.TestCrossNodeServiceConnectivity, ctx, verbose, &testResults, &testNames)

		// TODO: Add more tests here in the future
		// Test 4: DNS Resolution
		// Test 5: Ingress Connectivity
		// etc.

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

// executeTest is a helper function that eliminates repetitive test execution code
func executeTest(testNum int, testName string, testFunc func(context.Context) diagnostic.TestResult,
	ctx context.Context, verbose bool, testResults *[]diagnostic.TestResult, testNames *[]string) {

	fmt.Printf("📋 Test %d: %s\n", testNum, testName)
	result := testFunc(ctx)
	*testResults = append(*testResults, result)
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
