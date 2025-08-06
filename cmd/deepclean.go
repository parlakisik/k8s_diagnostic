package cmd

import (
	"context"
	"fmt"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"

	"github.com/spf13/cobra"
)

// deepCleanCmd represents the deepclean command
var deepCleanCmd = &cobra.Command{
	Use:   "deepclean",
	Short: "Perform comprehensive cleanup of all test resources",
	Long: `Perform comprehensive cleanup of all test resources in the Kubernetes cluster.

This command aggressively removes:
- All Cilium network policies (namespace-scoped and cluster-wide)
- All test pods, services, and related resources
- All secondary test namespaces
- Stuck resources with finalizer removal

Use this command when:
- Tests have failed and left resources behind
- You want to start completely fresh
- Resources are stuck and normal cleanup isn't working

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
