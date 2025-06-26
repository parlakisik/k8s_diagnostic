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
	Long: `Run pod-to-pod diagnostic tests within a Kubernetes cluster.

The tool will create two netshoot pods on different worker nodes and test 
connectivity between them by pinging from one pod to another.

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

		fmt.Printf("🚀 Running pod-to-pod diagnostic test in namespace '%s'\n\n", namespace)

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

		// Test 1: Pod-to-Pod Connectivity
		fmt.Printf("📋 Test 1: Pod-to-Pod Connectivity\n")
		result1 := tester.TestPodToPodConnectivity(ctx)
		testResults = append(testResults, result1)
		testNames = append(testNames, "Pod-to-Pod Connectivity")

		// Display result for test 1
		if result1.Success {
			fmt.Printf("✅ Test 1 PASSED: %s\n", result1.Message)
		} else {
			fmt.Printf("❌ Test 1 FAILED: %s\n", result1.Message)
		}

		if verbose && len(result1.Details) > 0 {
			fmt.Printf("  Details:\n")
			for _, detail := range result1.Details {
				fmt.Printf("    %s\n", detail)
			}
		}
		fmt.Printf("\n")

		// TODO: Add more tests here in the future
		// Test 2: DNS Resolution
		// Test 3: Service Connectivity
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

func init() {
	rootCmd.AddCommand(testCmd)

	// Local flags for the test command
	testCmd.Flags().StringP("namespace", "n", "diagnostic-test", "namespace to run diagnostic tests in")
	testCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
}
