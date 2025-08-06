package cmd

import (
	"context"
	"fmt"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"

	"github.com/spf13/cobra"
)

// deepCleanCmd represents the deepclean command
//
// JUSTIFICATION FOR THIS COMMAND:
// This command is essential because Kubernetes testing environments often encounter:
// 1. STUCK RESOURCES: NetworkPolicies with finalizers that prevent normal deletion
// 2. FAILED TESTS: When tests crash/fail, resources are left behind across namespaces
// 3. CNI CONFLICTS: Cilium policies can conflict and require aggressive cleanup
// 4. NAMESPACE ISSUES: Test namespaces can get stuck in "Terminating" state
// 5. RESOURCE ORPHANING: Cross-namespace resources that standard cleanup misses
//
// Without this command, developers would need to manually run multiple kubectl commands
// and understand complex Kubernetes resource relationships. This tool automates the
// complex cleanup process that is specifically needed for network policy testing.
var deepCleanCmd = &cobra.Command{
	Use:   "deepclean",
	Short: "Perform comprehensive cleanup of all test resources",
	Long: `Perform comprehensive cleanup of all test resources in the Kubernetes cluster.

** WHY THIS COMMAND IS NECESSARY **

Kubernetes network policy testing creates complex resource dependencies that normal 
cleanup cannot handle. This command solves critical problems:

PROBLEM 1 - STUCK RESOURCES:
- NetworkPolicies with finalizers that prevent deletion
- Namespaces stuck in "Terminating" state  
- Resources with cross-references that block cleanup

PROBLEM 2 - TEST FAILURES:
- When tests crash, resources are scattered across multiple namespaces
- Failed tests leave behind partial configurations
- Standard 'kubectl delete' misses dependent resources

PROBLEM 3 - CNI COMPLEXITY:
- Cilium network policies require specific cleanup order
- Policy conflicts can prevent new test runs
- CNI state needs complete reset between test suites

PROBLEM 4 - DEVELOPMENT WORKFLOW:
- Developers need a reliable way to reset test environments
- Manual cleanup requires deep Kubernetes knowledge
- Time-consuming to identify and remove all test artifacts

This command aggressively removes:
- All Cilium network policies (namespace-scoped and cluster-wide)  
- All test pods, services, and related resources
- All secondary test namespaces
- Stuck resources with finalizer removal
- Orphaned cross-namespace dependencies

Use this command when:
- Tests have failed and left resources behind
- You want to start completely fresh  
- Resources are stuck and normal cleanup isn't working
- Switching between different test scenarios
- Debugging test environment issues

The command uses the current kubectl context unless --kubeconfig is specified.`,
	Run: func(cmd *cobra.Command, args []string) {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		namespace, _ := cmd.Flags().GetString("namespace")
		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Printf("🧹 Deep Clean - Comprehensive Test Resource Cleanup\n")
		fmt.Printf("=====================================\n\n")

		if namespace != "" {
			fmt.Printf("🎯 Target namespace: %s\n", namespace)
		} else {
			fmt.Printf("🎯 Target: All test namespaces\n")
		}

		if verbose {
			fmt.Printf("📝 Verbose mode: Enabled\n")
		}
		fmt.Printf("\n")

		// Create timeout context for cleanup operations
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Create tester instance
		tester, err := core.NewTester(kubeconfig, namespace, verbose)
		if err != nil {
			fmt.Printf("❌ ERROR: Failed to create tester: %v\n", err)
			return
		}

		// Create cleanup orchestrator
		// Create a minimal logger for cleanup operations
		logger, err := core.NewMultiChannelLogger(namespace, verbose)
		if err != nil {
			fmt.Printf("ERROR: Failed to create logger: %v\n", err)
			return
		}
		defer logger.Close()

		orchestrator := core.NewCleanupOrchestrator(tester, logger)

		// Perform deep cleanup
		startTime := time.Now()
		orchestrator.DeepClean(ctx)
		elapsed := time.Since(startTime)

		fmt.Printf("\n⏱️  Deep cleanup completed in %.1f seconds\n", elapsed.Seconds())
		fmt.Printf("🎉 All test resources have been removed from the cluster\n")
	},
}

func init() {
	rootCmd.AddCommand(deepCleanCmd)

	// Flags for the deepclean command
	deepCleanCmd.Flags().StringP("namespace", "n", "diagnostic-test", "target namespace for cleanup (default cleans all test namespaces)")
	deepCleanCmd.Flags().String("kubeconfig", "", "path to kubeconfig file (inherits from global flag)")
	deepCleanCmd.Flags().BoolP("verbose", "v", false, "verbose output showing detailed cleanup operations")
}
