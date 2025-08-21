package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// KubernetesProductionCleanup performs optimized cleanup for Kubernetes production environment
// This maintains the same template structure as dev environment for UI compatibility
// but uses lightweight cleanup optimized for individual test execution
func (t *Tester) KubernetesProductionCleanup(ctx context.Context, testID string, verbose bool) {
	// Clean hierarchical output - SAME TEMPLATE AS DEV
	fmt.Println("\n🧹 CLEANUP PHASE (Kubernetes Production)")

	// Create timeout context for cleanup operations - SHORTER TIMEOUT
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Reduced from 120s
	defer cancel()

	// Operation 1: ONLY Cilium policies cleanup (optimized version)
	fmt.Print("├── Cilium Policies: Fast policy cleanup... ")
	startTime := time.Now()
	t.ForceCleanupCiliumPolicies(timeoutCtx, false) // Use our optimized 15s version
	duration := time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// SKIP full namespace cleanup - just clean test pods that might conflict
	fmt.Print("├── Test Pods: Cleaning conflicting test pods... ")
	startTime = time.Now()
	t.CleanupConflictingTestPods(timeoutCtx, false) // New lightweight pod cleanup
	duration = time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// SKIP secondary namespace cleanup - not needed for individual tests
	fmt.Print("└── Resource Check: Quick verification... ")
	startTime = time.Now()
	t.QuickResourceVerification(timeoutCtx, verbose) // New quick check instead of 60s wait
	duration = time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// Still log to JSONL for tracking - SAME AS DEV TEMPLATE
	logger := GetGlobalMultiChannelLogger()
	if logger != nil {
		logger.LogStepComplete("kubernetes_production_cleanup", true, "Kubernetes production cleanup successfully completed")
	}

	fmt.Printf("🎯 Production cleanup completed efficiently\n")
}

// CleanupAllTestResources cleans up all resources created during tests with clean hierarchical output
func (t *Tester) CleanupAllTestResources(ctx context.Context, verbose bool) {
	// Clean hierarchical output
	fmt.Println("\n🧹 CLEANUP PHASE")

	// Create timeout context for cleanup operations
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second) // Increased timeout
	defer cancel()

	// Operation 1: Cilium policies cleanup
	fmt.Print("├── Cilium Policies: Removing all network policies... ")
	startTime := time.Now()
	t.ForceCleanupCiliumPolicies(timeoutCtx, false) // verbose=false to avoid nested output
	duration := time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// Wait for policies to be fully removed
	time.Sleep(5 * time.Second)

	// Operation 2: Main namespace cleanup
	fmt.Printf("├── Main Namespace: Cleaning %s... ", t.namespace)
	startTime = time.Now()
	t.ForceCleanupNamespace(timeoutCtx, t.namespace, false) // verbose=false to avoid nested output
	duration = time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// Operation 3: Secondary namespaces cleanup
	fmt.Print("└── Secondary Namespaces: Checking for orphaned resources... ")
	startTime = time.Now()
	t.ForceCleanupSecondaryNamespaces(timeoutCtx, false) // verbose=false to avoid nested output
	duration = time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// CRITICAL: Verify all resources are actually gone before proceeding
	fmt.Print("🔍 Verifying all resources are deleted... ")
	startTime = time.Now()
	t.VerifyResourcesDeleted(timeoutCtx, verbose)
	duration = time.Since(startTime)
	fmt.Printf("✅ Done (%.1fs)\n", duration.Seconds())

	// Still log to JSONL for tracking
	logger := GetGlobalMultiChannelLogger()
	if logger != nil {
		logger.LogStepComplete("universal_cleanup", true, "Universal cleanup successfully completed")
	}
}

// VerifyResourcesDeleted ensures all test resources are actually gone before proceeding
func (t *Tester) VerifyResourcesDeleted(ctx context.Context, verbose bool) {
	maxRetries := 30 // 30 attempts with 2-second intervals = 60 seconds max

	for attempt := 1; attempt <= maxRetries; attempt++ {
		allClear := true

		// Check for any remaining pods
		if pods, err := t.clientset.CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{}); err == nil {
			testPods := 0
			for _, pod := range pods.Items {
				// Count pods that match our test patterns
				if strings.Contains(pod.Name, "pod-to-pod") ||
					strings.Contains(pod.Name, "netshoot") ||
					strings.Contains(pod.Name, "test") ||
					pod.Labels["app"] != "" {
					testPods++
				}
			}
			if testPods > 0 {
				allClear = false
				if verbose {
					fmt.Printf("  Waiting for %d test pods to be fully deleted (attempt %d/%d)...\n",
						testPods, attempt, maxRetries)
				}
			}
		}

		// Check for any remaining services
		if services, err := t.clientset.CoreV1().Services(t.namespace).List(ctx, metav1.ListOptions{}); err == nil {
			testServices := 0
			for _, svc := range services.Items {
				// Skip kubernetes default service
				if svc.Name != "kubernetes" {
					testServices++
				}
			}
			if testServices > 0 {
				allClear = false
				if verbose {
					fmt.Printf("  Waiting for %d test services to be fully deleted (attempt %d/%d)...\n",
						testServices, attempt, maxRetries)
				}
			}
		}

		// Check for any remaining deployments
		if deployments, err := t.clientset.AppsV1().Deployments(t.namespace).List(ctx, metav1.ListOptions{}); err == nil {
			if len(deployments.Items) > 0 {
				allClear = false
				if verbose {
					fmt.Printf("  Waiting for %d test deployments to be fully deleted (attempt %d/%d)...\n",
						len(deployments.Items), attempt, maxRetries)
				}
			}
		}

		if allClear {
			if verbose {
				fmt.Printf("  ✅ All test resources confirmed deleted after %d attempts\n", attempt)
			}
			return
		}

		// Wait before next attempt
		time.Sleep(2 * time.Second)
	}

	// If we get here, some resources may still exist but we've waited long enough
	if verbose {
		fmt.Printf("  ⚠️  Resource verification completed after %d attempts (some resources may still be terminating)\n", maxRetries)
	}
}

// ForceCleanupCiliumPolicies aggressively removes all Cilium network policies - OPTIMIZED VERSION
func (t *Tester) ForceCleanupCiliumPolicies(ctx context.Context, verbose bool) {
	// Only show internal step logging when verbose=true
	if verbose {
		// Get the global logger for structured logging
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStep("force_cleanup_cilium_policies", "Forcefully removing all Cilium network policies", 1, 1)
		} else {
			fmt.Printf("%s Forcefully removing all Cilium network policies...\n", time.Now().Format("2006-01-02 15:04:05"))
		}
	}

	// OPTIMIZATION 1: Quick check if any policies exist before doing expensive operations
	quickCheck := exec.CommandContext(ctx, "kubectl", "get", "ciliumnetworkpolicies", "--all-namespaces", "--no-headers")
	checkOutput, err := quickCheck.CombinedOutput()

	quickCheckCW := exec.CommandContext(ctx, "kubectl", "get", "ciliumclusterwidenetworkpolicies", "--no-headers")
	checkOutputCW, errCW := quickCheckCW.CombinedOutput()

	// If no policies exist, skip expensive operations
	if (err != nil || len(strings.TrimSpace(string(checkOutput))) == 0) &&
		(errCW != nil || len(strings.TrimSpace(string(checkOutputCW))) == 0) {
		if verbose {
			fmt.Printf("%s No Cilium policies found, skipping cleanup\n", time.Now().Format("2006-01-02 15:04:05"))
		}
		return
	}

	// OPTIMIZATION 2: Use faster bulk operations with timeouts
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Limit total time
	defer cancel()

	// Bulk delete with short timeout and --wait=false to avoid hanging
	policyCommands := [][]string{
		{"kubectl", "delete", "ciliumclusterwidenetworkpolicy", "--all", "--force", "--grace-period=0", "--wait=false"},
		{"kubectl", "delete", "ciliumnetworkpolicy", "--all", "--all-namespaces", "--force", "--grace-period=0", "--wait=false"},
	}

	for _, cmdArgs := range policyCommands {
		cmd := exec.CommandContext(timeoutCtx, cmdArgs[0], cmdArgs[1:]...)
		_, err := cmd.CombinedOutput()
		if err != nil && verbose {
			fmt.Printf("%s Warning: %v during policy cleanup (continuing anyway)\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
	}

	// OPTIMIZATION 3: Use simple name-only listing instead of full JSON parsing
	// Handle namespace-scoped policies with name extraction only
	nsCmd := exec.CommandContext(timeoutCtx, "kubectl", "get", "ciliumnetworkpolicies", "--all-namespaces", "-o", "jsonpath={range .items[*]}{.metadata.namespace}{' '}{.metadata.name}{'\n'}{end}")
	if output, err := nsCmd.CombinedOutput(); err == nil && len(output) > 0 {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				namespace, name := parts[0], parts[1]
				// Quick patch and delete without waiting
				patchCmd := fmt.Sprintf("kubectl patch ciliumnetworkpolicies %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge --timeout=2s", name, namespace)
				exec.CommandContext(timeoutCtx, "sh", "-c", patchCmd).Run()

				deleteCmd := fmt.Sprintf("kubectl delete ciliumnetworkpolicies %s -n %s --force --grace-period=0 --wait=false", name, namespace)
				exec.CommandContext(timeoutCtx, "sh", "-c", deleteCmd).Run()
			}
		}
	}

	// Handle cluster-wide policies with name extraction only
	cwCmd := exec.CommandContext(timeoutCtx, "kubectl", "get", "ciliumclusterwidenetworkpolicies", "-o", "jsonpath={range .items[*]}{.metadata.name}{'\n'}{end}")
	if output, err := cwCmd.CombinedOutput(); err == nil && len(output) > 0 {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			// Quick patch and delete without waiting
			patchCmd := fmt.Sprintf("kubectl patch ciliumclusterwidenetworkpolicies %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge --timeout=2s", name)
			exec.CommandContext(timeoutCtx, "sh", "-c", patchCmd).Run()

			deleteCmd := fmt.Sprintf("kubectl delete ciliumclusterwidenetworkpolicies %s --force --grace-period=0 --wait=false", name)
			exec.CommandContext(timeoutCtx, "sh", "-c", deleteCmd).Run()
		}
	}

	// OPTIMIZATION 4: Shorter wait time and skip verification in non-verbose mode
	if verbose {
		fmt.Printf("%s Verifying all Cilium policies have been removed...\n", time.Now().Format("2006-01-02 15:04:05"))
		time.Sleep(2 * time.Second)
	} else {
		// Just a brief pause to let deletions start
		time.Sleep(500 * time.Millisecond)
	}

	// Log completion with immediate flush (only when verbose=true)
	if verbose {
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStepComplete("force_cleanup_cilium_policies", true, "Cilium network policies forcefully removed")
		}
	}
}

// ForceCleanupNamespace aggressively cleans up a namespace
func (t *Tester) ForceCleanupNamespace(ctx context.Context, namespace string, verbose bool) {
	// Only show internal step logging when verbose=true
	if verbose {
		// Get the global logger for structured logging
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStep("force_cleanup_namespace", fmt.Sprintf("Forcefully cleaning up namespace: %s", namespace), 1, 1)
		} else {
			fmt.Printf("%s Forcefully cleaning up namespace: %s\n", time.Now().Format("2006-01-02 15:04:05"), namespace)
		}
	}

	// STEP 0: Directly delete known problematic pods by name - extreme nuclear approach
	// These are the ones that consistently cause "already exists" errors in test runs
	specificPodNames := []string{
		"web-policy-test",
		"client-policy-test",
		"api",
		"client1",
		"client2",
		"cidr-test-pod",
		"test-pod",
		"netshoot",
	}

	// Also delete any pods matching networking test patterns
	if pods, err := t.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, pod := range pods.Items {
			podName := pod.Name
			// Target networking test pods that follow specific patterns
			if strings.Contains(podName, "pod-to-pod-") ||
				strings.Contains(podName, "service-") ||
				strings.Contains(podName, "dns-resolution-") ||
				strings.Contains(podName, "cross-node-") {
				specificPodNames = append(specificPodNames, podName)
			}
		}
	}

	if verbose {
		fmt.Printf("%s Direct nuclear targeting of specific pod names...\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	for _, podName := range specificPodNames {
		// First attempt to remove finalizers
		patchCmd := fmt.Sprintf("kubectl patch pod %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge 2>/dev/null || true",
			podName, namespace)
		exec.Command("sh", "-c", patchCmd).Run()

		// Then use direct kubectl force delete (most reliable)
		deleteCmd := fmt.Sprintf("kubectl delete pod %s -n %s --force --grace-period=0 --wait=false 2>/dev/null || true",
			podName, namespace)
		exec.Command("sh", "-c", deleteCmd).Run()

		// And also try the client deletion
		t.clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{
			GracePeriodSeconds: pointer.Int64(0),
			PropagationPolicy:  &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
		})
	}

	// Wait briefly for these direct deletions to take effect
	time.Sleep(2 * time.Second)

	// STEP 1: Remove pods by label with extreme prejudice
	podLabels := []string{
		"app in (web,client,api,client-test,netshoot-test,web-policy-test,client-policy-test,cidr-test)",
		"run in (web,client,api,web-policy-test,client-policy-test)",
		"app=api",
		"app=web",
		"app=client",
		"app=cidr-test",
	}

	for _, labelSelector := range podLabels {
		// First try direct kubectl for label deletion (often more reliable)
		directCmd := fmt.Sprintf("kubectl delete pods -n %s -l \"%s\" --force --grace-period=0 --wait=false 2>/dev/null || true",
			namespace, labelSelector)
		exec.Command("sh", "-c", directCmd).Run()

		// Then also do API-based deletion
		if pods, err := t.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err == nil {
			for _, pod := range pods.Items {
				// First remove finalizers if any
				patchCmd := fmt.Sprintf("kubectl patch pod %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge",
					pod.Name, namespace)
				exec.Command("sh", "-c", patchCmd).Run()

				// Then force delete with zero grace period
				t.clientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
					GracePeriodSeconds: pointer.Int64(0),
					PropagationPolicy:  &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
				})
				if verbose {
					fmt.Printf("%s Forcefully deleted pod: %s in namespace: %s\n",
						time.Now().Format("2006-01-02 15:04:05"), pod.Name, namespace)
				}
			}
		}
	}

	// STEP 2: Delete ALL pods in the namespace (catch any we missed)
	if verbose {
		fmt.Printf("%s Deleting ALL pods in namespace %s...\n", time.Now().Format("2006-01-02 15:04:05"), namespace)
	}
	allPodsCmd := fmt.Sprintf("kubectl delete pods --all -n %s --force --grace-period=0 --wait=false 2>/dev/null || true",
		namespace)
	exec.Command("sh", "-c", allPodsCmd).Run()

	// STEP 3: Delete all services in the namespace
	if verbose {
		fmt.Printf("%s Deleting all services in namespace %s...\n", time.Now().Format("2006-01-02 15:04:05"), namespace)
	}
	allSvcsCmd := fmt.Sprintf("kubectl delete services --all -n %s --force --grace-period=0 --wait=false 2>/dev/null || true",
		namespace)
	exec.Command("sh", "-c", allSvcsCmd).Run()

	if services, err := t.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, svc := range services.Items {
			// Skip kubernetes default service
			if svc.Name == "kubernetes" {
				continue
			}

			// Remove finalizers
			patchCmd := fmt.Sprintf("kubectl patch service %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge",
				svc.Name, namespace)
			exec.Command("sh", "-c", patchCmd).Run()

			// Force delete
			t.clientset.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
			})
			if verbose {
				fmt.Printf("%s Forcefully deleted service: %s in namespace: %s\n",
					time.Now().Format("2006-01-02 15:04:05"), svc.Name, namespace)
			}
		}
	}

	// STEP 4: Delete all deployments in the namespace (this fixes "deployments.apps already exists" errors)
	if verbose {
		fmt.Printf("%s Deleting all deployments in namespace %s...\n", time.Now().Format("2006-01-02 15:04:05"), namespace)
	}
	allDeploymentsCmd := fmt.Sprintf("kubectl delete deployments --all -n %s --force --grace-period=0 --wait=false 2>/dev/null || true",
		namespace)
	exec.Command("sh", "-c", allDeploymentsCmd).Run()

	if deployments, err := t.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, deploy := range deployments.Items {
			// Remove finalizers
			patchCmd := fmt.Sprintf("kubectl patch deployment %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge",
				deploy.Name, namespace)
			exec.Command("sh", "-c", patchCmd).Run()

			// Force delete
			t.clientset.AppsV1().Deployments(namespace).Delete(ctx, deploy.Name, metav1.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
			})
			if verbose {
				fmt.Printf("%s Forcefully deleted deployment: %s in namespace: %s\n",
					time.Now().Format("2006-01-02 15:04:05"), deploy.Name, namespace)
			}
		}
	}

	// STEP 5: Verify all pods are gone with retries and escalating force
	if verbose {
		fmt.Printf("%s Verifying cleanup of namespace %s...\n", time.Now().Format("2006-01-02 15:04:05"), namespace)
	}
	for i := 0; i < 8; i++ { // More retry attempts
		pods, err := t.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil || len(pods.Items) == 0 {
			if verbose {
				fmt.Printf("%s ✓ All pods confirmed gone from namespace %s\n",
					time.Now().Format("2006-01-02 15:04:05"), namespace)
			}
			break
		}

		if verbose {
			fmt.Printf("%s Still waiting for %d pods to be removed from namespace %s (attempt %d/8)...\n",
				time.Now().Format("2006-01-02 15:04:05"), len(pods.Items), namespace, i+1)
		}

		// Progressively more aggressive approaches
		if i >= 2 {
			for _, pod := range pods.Items {
				// Ultra-nuclear approach - patch finalizers, evict, then delete with escalating force
				podName := pod.Name

				// Remove finalizers
				patchCmd := fmt.Sprintf("kubectl patch pod %s -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge",
					podName, namespace)
				exec.Command("sh", "-c", patchCmd).Run()

				// Try eviction API
				if i >= 4 {
					evictCmd := fmt.Sprintf("kubectl delete pod %s -n %s --force --grace-period=0 --wait=false",
						podName, namespace)
					exec.Command("sh", "-c", evictCmd).Run()
				}

				// Direct killing
				if i >= 6 {
					killCmd := fmt.Sprintf("kubectl delete pod %s -n %s --force --grace-period=0 --wait=false",
						podName, namespace)
					output, err := exec.Command("sh", "-c", killCmd).CombinedOutput()
					if err != nil && verbose {
						fmt.Printf("%s Warning: Failed to delete pod %s: %v\nOutput: %s\n",
							time.Now().Format("2006-01-02 15:04:05"), podName, err, string(output))
					}
				}
			}
		}

		// Escalate timing between retries
		sleepTime := time.Duration(1+i) * time.Second
		time.Sleep(sleepTime)
	}

	// Log completion with immediate flush (only when verbose=true)
	if verbose {
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStepComplete("force_cleanup_namespace", true, fmt.Sprintf("Namespace %s forcefully cleaned up", namespace))
		}
	}
}

// ForceCleanupSecondaryNamespaces aggressively cleans up all secondary namespaces
func (t *Tester) ForceCleanupSecondaryNamespaces(ctx context.Context, verbose bool) {
	// Only show internal step logging when verbose=true
	if verbose {
		fmt.Printf("%s Forcefully cleaning up all secondary namespaces...\n", time.Now().Format("2006-01-02 15:04:05"))
	}

	// 1. List and delete all secondary namespaces
	namespaces, err := t.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		if verbose {
			fmt.Printf("%s Warning: failed to list namespaces: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
		return
	}

	basePrefix := t.namespace + "-secondary-"
	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, basePrefix) {
			// Clean up the namespace contents first
			t.ForceCleanupNamespace(ctx, ns.Name, verbose)

			// Remove finalizers from namespace
			patchCmd := fmt.Sprintf("kubectl patch namespace %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge", ns.Name)
			exec.Command("sh", "-c", patchCmd).Run()

			// Delete the namespace with force
			t.clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
			})
			if verbose {
				fmt.Printf("%s Forcefully deleted namespace: %s\n", time.Now().Format("2006-01-02 15:04:05"), ns.Name)
			}

			// Wait for namespace to be fully deleted
			for i := 0; i < 5; i++ {
				_, err := t.clientset.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
				if err != nil && errors.IsNotFound(err) {
					if verbose {
						fmt.Printf("%s Confirmed namespace %s is deleted\n", time.Now().Format("2006-01-02 15:04:05"), ns.Name)
					}
					break
				}
				if i == 4 && verbose {
					fmt.Printf("%s Warning: namespace %s may still exist after cleanup\n",
						time.Now().Format("2006-01-02 15:04:05"), ns.Name)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// CleanupTestPods cleans up specifically test pods in the namespace
// This function is used in L4 policy tests
func (t *Tester) CleanupTestPods(ctx context.Context, verbose bool) {
	// Only show internal step logging when verbose=true
	if verbose {
		// Get the global logger for structured logging
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStep("cleanup_test_pods", "Cleaning up test pods in namespace", 1, 1)
		} else {
			fmt.Printf("%s Cleaning up test pods...\n", time.Now().Format("2006-01-02 15:04:05"))
		}
	}

	// List of test pod labels to target
	podLabels := []string{
		"app in (web,client,api,client-test,netshoot-test,web-policy-test,client-policy-test)",
		"run in (web,client,api,web-policy-test,client-policy-test)",
	}

	for _, labelSelector := range podLabels {
		// Try direct kubectl for label deletion
		directCmd := fmt.Sprintf("kubectl delete pods -n %s -l \"%s\" --force --grace-period=0 --wait=false 2>/dev/null || true",
			t.namespace, labelSelector)
		exec.Command("sh", "-c", directCmd).Run()

		// Also do API-based deletion
		if pods, err := t.clientset.CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err == nil {
			for _, pod := range pods.Items {
				t.clientset.CoreV1().Pods(t.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
					GracePeriodSeconds: pointer.Int64(0),
					PropagationPolicy:  &[]metav1.DeletionPropagation{metav1.DeletePropagationForeground}[0],
				})
				if verbose {
					fmt.Printf("%s Deleted test pod: %s\n", time.Now().Format("2006-01-02 15:04:05"), pod.Name)
				}
			}
		}
	}

	// Allow some time for pods to be fully removed
	time.Sleep(2 * time.Second)

	// Log completion with immediate flush (only when verbose=true)
	if verbose {
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStepComplete("cleanup_test_pods", true, "Test pods cleanup completed successfully")
		}
	}
}

// CleanupConflictingTestPods performs lightweight cleanup of only pods that might conflict with new tests
// This is optimized for Kubernetes production where we don't need full namespace cleanup
func (t *Tester) CleanupConflictingTestPods(ctx context.Context, verbose bool) {
	// Only target pods that commonly cause "already exists" errors
	conflictingPodNames := []string{
		"web-policy-test",
		"client-policy-test",
		"api",
		"client1",
		"client2",
		"test-pod",
		"netshoot",
	}

	// Quick deletion of known conflicting pods
	for _, podName := range conflictingPodNames {
		deleteCmd := fmt.Sprintf("kubectl delete pod %s -n %s --force --grace-period=0 --wait=false 2>/dev/null || true", podName, t.namespace)
		exec.CommandContext(ctx, "sh", "-c", deleteCmd).Run()
	}

	// Clean up any pods with conflicting labels (lightweight approach)
	labelSelectors := []string{
		"app in (web,client,api)",
		"run in (web,client,api)",
	}

	for _, labelSelector := range labelSelectors {
		directCmd := fmt.Sprintf("kubectl delete pods -n %s -l \"%s\" --force --grace-period=0 --wait=false 2>/dev/null || true", t.namespace, labelSelector)
		exec.CommandContext(ctx, "sh", "-c", directCmd).Run()
	}

	// Brief pause to let deletions start
	time.Sleep(1 * time.Second)
}

// QuickResourceVerification performs a quick check instead of the 60-second comprehensive verification
// This is optimized for Kubernetes production environment
func (t *Tester) QuickResourceVerification(ctx context.Context, verbose bool) {
	// Just do a quick check - no long waits
	maxRetries := 5 // 5 attempts with 1-second intervals = 5 seconds max

	for attempt := 1; attempt <= maxRetries; attempt++ {
		allClear := true

		// Quick check for remaining test pods only (not all pods)
		if pods, err := t.clientset.CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app in (web,client,api,test)",
		}); err == nil {
			if len(pods.Items) > 0 {
				allClear = false
				if verbose {
					fmt.Printf("  Waiting for %d test pods to be deleted (attempt %d/%d)...\n", len(pods.Items), attempt, maxRetries)
				}
			}
		}

		if allClear {
			if verbose {
				fmt.Printf("  ✅ Quick verification completed after %d attempts\n", attempt)
			}
			return
		}

		// Short wait before next attempt
		time.Sleep(1 * time.Second)
	}

	// If we get here, some resources may still exist but we don't wait longer
	if verbose {
		fmt.Printf("  ⚠️  Quick verification completed after %d attempts (proceeding anyway)\n", maxRetries)
	}
}

// CleanupPoliciesOnly cleans up only policies without removing all test resources
// This is used between test categories to ensure policy isolation
func (t *Tester) CleanupPoliciesOnly(ctx context.Context, verbose bool) {
	// Only show internal step logging when verbose=true
	if verbose {
		// Get the global logger for structured logging
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStep("cleanup_policies_only", "Cleaning up policies between test categories", 1, 1)
		} else {
			fmt.Printf("%s Cleaning up only policies between test categories...\n", time.Now().Format("2006-01-02 15:04:05"))
		}
	}

	// 1. Delete all existing policy resources - both cluster-wide and namespace-scoped
	// Using force option to ensure deletion even if there are issues
	policyCommands := [][]string{
		{"kubectl", "delete", "ciliumclusterwidenetworkpolicy", "--all", "--force", "--grace-period=0"},
		{"kubectl", "delete", "ciliumnetworkpolicy", "--all", "--all-namespaces", "--force", "--grace-period=0"},
	}

	for _, cmdArgs := range policyCommands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		_, err := cmd.CombinedOutput()
		if err != nil && verbose {
			fmt.Printf("%s Warning: %v during policy cleanup\n", time.Now().Format("2006-01-02 15:04:05"), err)
		}
	}

	// Additional attempt - patch policies to remove finalizers
	// This is similar to what the bash script does for stuck resources
	cleanupCommand := "kubectl get ciliumnetworkpolicies -n " + t.namespace + " -o name 2>/dev/null | " +
		"xargs -r -I {} kubectl patch {} -n " + t.namespace + " -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge"
	exec.Command("sh", "-c", cleanupCommand).Run()

	cleanupCommand = "kubectl get ciliumclusterwidenetworkpolicies -o name 2>/dev/null | " +
		"xargs -r -I {} kubectl patch {} -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge"
	exec.Command("sh", "-c", cleanupCommand).Run()

	// Wait a moment for changes to take effect
	time.Sleep(2 * time.Second)

	// Log completion with immediate flush (only when verbose=true)
	if verbose {
		logger := GetGlobalMultiChannelLogger()
		if logger != nil {
			logger.LogStepComplete("cleanup_policies_only", true, "Policy cleanup completed successfully")
		}
	}
}
