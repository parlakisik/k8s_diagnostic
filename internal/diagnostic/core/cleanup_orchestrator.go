package core

import (
	"context"
	"fmt"
	"time"
)

// CleanupOrchestrator provides standardized cleanup operations across all test groups
type CleanupOrchestrator struct {
	tester *Tester
	logger *MultiChannelLogger
}

// NewCleanupOrchestrator creates a new cleanup orchestrator
func NewCleanupOrchestrator(tester *Tester, logger *MultiChannelLogger) *CleanupOrchestrator {
	return &CleanupOrchestrator{
		tester: tester,
		logger: logger,
	}
}

// PreTestCleanup performs cleanup before each individual test
func (co *CleanupOrchestrator) PreTestCleanup(ctx context.Context, testName string) {
	if co.logger != nil {
		co.logger.LogStep("pre_test_cleanup", fmt.Sprintf("Pre-test cleanup for %s", testName), 1, 1)
	}

	// Clean up test pods to ensure fresh environment
	co.tester.CleanupTestPods(ctx, co.logger.IsVerbose())

	if co.logger != nil {
		co.logger.LogStepComplete("pre_test_cleanup", true, "Pre-test cleanup completed")
	}
}

// OptimizedPreTestCleanup performs lightweight cleanup before each test (policies only)
// This is optimized for L4 policy tests where pod reuse is beneficial
func (co *CleanupOrchestrator) OptimizedPreTestCleanup(ctx context.Context, testName string) {
	if co.logger != nil {
		co.logger.LogStepName("optimized_pre_test_cleanup", fmt.Sprintf("Policy cleanup for %s", testName))
	}

	// Only clean up policies between tests - keep pods alive for reuse
	co.tester.CleanupPoliciesOnly(ctx, false) // verbose=false to avoid nested output

	if co.logger != nil {
		co.logger.LogStepComplete("optimized_pre_test_cleanup", true, "Policy cleanup completed")
	}
}

// OptimizedPostSubgroupCleanup performs cleanup after subgroup with minimal pod disruption
func (co *CleanupOrchestrator) OptimizedPostSubgroupCleanup(ctx context.Context, subgroupName string) {
	if co.logger != nil {
		co.logger.LogStep("optimized_post_subgroup_cleanup", fmt.Sprintf("Post-subgroup cleanup for %s", subgroupName), 1, 1)
	}

	// Clean up policies after subgroup (critical for isolation between subgroups)
	co.tester.CleanupPoliciesOnly(ctx, false)

	// Only clean pods between different subgroups (not between tests in same subgroup)
	co.tester.CleanupTestPods(ctx, false)

	// Allow policies to be fully cleaned up
	time.Sleep(2 * time.Second) // Reduced from 3s

	if co.logger != nil {
		co.logger.LogStepComplete("optimized_post_subgroup_cleanup", true, "Optimized post-subgroup cleanup completed")
	}
}

// PostSubgroupCleanup performs cleanup after each subgroup completes
func (co *CleanupOrchestrator) PostSubgroupCleanup(ctx context.Context, subgroupName string) {
	if co.logger != nil {
		co.logger.LogStep("post_subgroup_cleanup", fmt.Sprintf("Post-subgroup cleanup for %s", subgroupName), 1, 1)
	}

	// Clean up policies and test resources after subgroup
	co.tester.CleanupPoliciesOnly(ctx, co.logger.IsVerbose())
	co.tester.CleanupTestPods(ctx, co.logger.IsVerbose())

	// Allow resources to be fully cleaned up
	time.Sleep(3 * time.Second)

	if co.logger != nil {
		co.logger.LogStepComplete("post_subgroup_cleanup", true, "Post-subgroup cleanup completed")
	}
}

// PostGroupCleanup performs cleanup after each test group completes
func (co *CleanupOrchestrator) PostGroupCleanup(ctx context.Context, groupName string) {
	if co.logger != nil {
		co.logger.LogStep("post_group_cleanup", fmt.Sprintf("Post-group cleanup for %s", groupName), 1, 1)
	}

	// Comprehensive cleanup after entire group
	co.tester.CleanupAllTestResources(ctx, co.logger.IsVerbose())

	// Longer wait for complete resource cleanup
	time.Sleep(5 * time.Second)

	if co.logger != nil {
		co.logger.LogStepComplete("post_group_cleanup", true, "Post-group cleanup completed")
	}
}

// PreTestGroupCleanup performs cleanup before starting a test group
func (co *CleanupOrchestrator) PreTestGroupCleanup(ctx context.Context, groupName string) {
	if co.logger != nil {
		co.logger.LogStep("pre_group_cleanup", fmt.Sprintf("Pre-group cleanup for %s", groupName), 1, 1)
	}

	// Ensure completely clean environment before starting group
	co.tester.CleanupAllTestResources(ctx, co.logger.IsVerbose())

	// Allow time for cleanup to complete
	time.Sleep(5 * time.Second)

	if co.logger != nil {
		co.logger.LogStepComplete("pre_group_cleanup", true, "Pre-group cleanup completed")
	}
}

// EmergencyCleanup performs aggressive cleanup when tests fail
func (co *CleanupOrchestrator) EmergencyCleanup(ctx context.Context, reason string) {
	if co.logger != nil {
		co.logger.LogStep("emergency_cleanup", fmt.Sprintf("Emergency cleanup triggered: %s", reason), 1, 1)
	}

	// Use the most aggressive cleanup available
	co.tester.CleanupAllTestResources(ctx, true) // Force verbose for emergency

	if co.logger != nil {
		co.logger.LogStepComplete("emergency_cleanup", true, "Emergency cleanup completed")
	}
}

// DeepClean performs comprehensive nuclear cleanup - used by deepclean command
func (co *CleanupOrchestrator) DeepClean(ctx context.Context) {
	fmt.Printf("🧹 Starting deep cleanup of all test resources...\n")

	if co.logger != nil {
		co.logger.LogStep("deep_cleanup", "Deep cleanup: Removing all test resources", 1, 1)
	} else {
		fmt.Printf("  🔄 Removing all test resources...\n")
	}

	// Use aggressive cleanup with verbose output
	co.tester.CleanupAllTestResources(ctx, true)

	if co.logger != nil {
		co.logger.LogStepComplete("deep_cleanup", true, "Deep cleanup completed")
	} else {
		fmt.Printf("  ✅ Deep cleanup completed\n")
	}

	fmt.Printf("🎯 Deep cleanup finished - all test resources removed\n")
}
