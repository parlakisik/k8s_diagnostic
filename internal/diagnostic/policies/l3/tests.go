package l3

import (
	"context"
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// Type aliases for compatibility
type TestResult = core.TestResult

// Function aliases
var ElapsedSeconds = core.ElapsedSeconds

// L3PolicySubgroups defines test subgroups for organization and concurrent execution
// Exported so it can be accessed from cmd/test.go for validation
var L3PolicySubgroups = map[string][]string{
	"ip-cidr":  {"cidr-ingress", "cidr-egress", "cidr-except"},
	"endpoint": {"endpoints-label"},
	"entities": {"entities-based"},
	"dns":      {"dns-based"},
	"node":     {"node-selector", "pod-node-name", "node-cidr", "node-based"},
	"service":  {"kubernetes-service"},
	"security": {"allow-all", "deny-all"},
}

// Map of test names to test keys (for CLI reference)
var L3PolicyTestNameToKey = map[string]string{
	"cidr-ingress":              "cidr-ingress",
	"cidr-egress":               "cidr-egress",
	"cidr-with-except":          "cidr-except",
	"endpoints-label":           "endpoints-label",
	"entities-based":            "entities-based",
	"dns-based":                 "dns-based",
	"traditional-node-selector": "node-selector",
	"pod-node-name":             "pod-node-name",
	"node-cidr":                 "node-cidr",
	"node-based":                "node-based",
	"kubernetes-service":        "kubernetes-service",
	"allow-all":                 "allow-all",
	"deny-all":                  "deny-all",
}

// TestCIDRIngressPolicy tests the CIDR-based ingress policy using common framework
func TestCIDRIngressPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "cidr-ingress" {
			config = cfg
			break
		}
	}

	// Use common framework for execution (real data capture handled at group level)
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestCIDREgressPolicy tests the CIDR-based egress policy using common framework
func TestCIDREgressPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "cidr-egress" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestCIDRWithExceptPolicy tests the CIDR with except policy using common framework
func TestCIDRWithExceptPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "cidr-except" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestEndpointsLabelPolicy tests the endpoints label selector policy using common framework
func TestEndpointsLabelPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "endpoints-label" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestEntitiesBasedPolicy tests the entities-based policy using common framework
func TestEntitiesBasedPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "entities-based" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestDNSBasedPolicy tests the DNS-based policy using common framework
func TestDNSBasedPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "dns-based" {
			config = cfg
			break
		}
	}

	// Use core framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestTraditionalNodeSelectorPolicy tests the traditional node selector policy using core framework
func TestTraditionalNodeSelectorPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "node-selector" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestPodNodeNamePolicy tests the pod node name policy using common framework
func TestPodNodeNamePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "pod-node-name" {
			config = cfg
			break
		}
	}

	// Use core framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestNodeCIDRPolicy tests the node CIDR policy using core framework
func TestNodeCIDRPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "node-cidr" {
			config = cfg
			break
		}
	}

	// Use core framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestNodeBasedPolicy tests the node based policy clusterwide using core framework
func TestNodeBasedPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "node-based" {
			config = cfg
			break
		}
	}

	// Use core framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestKubernetesServicePolicy tests the Kubernetes service policy using core framework
func TestKubernetesServicePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "kubernetes-service" {
			config = cfg
			break
		}
	}

	// Use core framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestAllowAllPolicy tests the allow-all policy using common framework
func TestAllowAllPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "allow-all" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestDenyAllPolicy tests the deny-all policy using common framework
func TestDenyAllPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L3TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L3TestConfigs {
		if cfg.TestId == "deny-all" {
			config = cfg
			break
		}
	}

	// Use common framework for execution
	return core.ExecutePolicyTest(
		config,
		logger,
		t,
		ctx,
		reuseResources,
		verbose,
		testNumber,
		totalTests,
	)
}

// TestL3PoliciesSequential runs L3 policy tests with sequential execution for clean policy isolation
func TestL3PoliciesSequential(t *core.Tester, ctx context.Context, requestedSubgroups []string, verbose ...bool) []TestResult {
	// Determine verbose mode
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	allResults := []TestResult{}

	if isVerbose {
		fmt.Printf("\n===== L3 POLICIES SEQUENTIAL EXECUTION - VERBOSE MODE ENABLED =====\n")
		fmt.Printf("Verbose output will be shown for each test\n")
		fmt.Printf("============================================================\n\n")
	}

	fmt.Printf("Starting L3 policies tests...\n")

	// Create test groups using the new core framework
	testGroups := BuildL3TestGroups(requestedSubgroups)

	// Execute test groups using the core framework (preserves all real data functionality)
	timedResults, testNames := core.ExecutePolicyTestGroups(
		testGroups,
		t,
		ctx,
		isVerbose,
		t.CaptureRealL3ConnectivityData, // Preserve real data capture functionality
	)

	// Extract basic results for backward compatibility
	for _, timedResult := range timedResults {
		allResults = append(allResults, timedResult.TestResult)
	}

	// Calculate passed/failed counts
	passedTests := 0
	failedTests := 0
	for _, result := range allResults {
		if result.Success {
			passedTests++
		} else {
			failedTests++
		}
	}

	// Create display names mapping for L3 tests
	displayNames := map[string]string{
		"cidr-ingress":       "CIDR Ingress Policy",
		"cidr-egress":        "CIDR Egress Policy",
		"cidr-except":        "CIDR With Except Policy",
		"endpoints-label":    "Endpoints Label Selector Policy",
		"entities-based":     "Entities Based Policy",
		"dns-based":          "DNS Based Policy",
		"node-selector":      "Traditional Node Selector Policy",
		"pod-node-name":      "Pod Node Name Policy",
		"node-cidr":          "Node CIDR Policy",
		"node-based":         "Node Based Policy Clusterwide",
		"kubernetes-service": "Kubernetes Service Policy",
		"allow-all":          "Allow All Policy",
		"deny-all":           "Deny All Policy",
	}

	// Display enhanced verbose summary with Expected vs Received details
	core.FormatEnhancedTestSummary(timedResults, testNames, displayNames, isVerbose)

	return allResults
}
