package l4

import (
	"context"
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// Type aliases for compatibility
type TestResult = core.TestResult

// Function aliases
var ElapsedSeconds = core.ElapsedSeconds

// L4PolicySubgroups defines test subgroups for organization and concurrent execution
// Exported so it can be accessed from cmd/test.go for validation
var L4PolicySubgroups = map[string][]string{
	"port":    {"tcp-port-ingress", "tcp-port-egress", "port-range", "multiple-port"},
	"icmp":    {"icmp-type", "icmpv6-type", "mixed-icmp"},
	"tls-sni": {"basic-sni", "multi-domain-sni", "combined-l4-sni"},
}

// Map of test names to test keys (for CLI reference)
var L4PolicyTestNameToKey = map[string]string{
	"tcp-port-ingress-policy": "tcp-port-ingress",
	"tcp-port-egress-policy":  "tcp-port-egress",
	"port-range-policy":       "port-range",
	"multiple-port-policy":    "multiple-port",
	"icmp-type-policy":        "icmp-type",
	"icmpv6-type-policy":      "icmpv6-type",
	"mixed-icmp-policy":       "mixed-icmp",
	"basic-sni-policy":        "basic-sni",
	"multi-domain-sni-policy": "multi-domain-sni",
	"combined-l4-sni-policy":  "combined-l4-sni",
}

// TestTCPPortIngressPolicy tests the TCP port ingress policy using common framework
func TestTCPPortIngressPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "tcp-port-ingress" {
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

// TestTCPPortEgressPolicy tests the TCP port egress policy using common framework
func TestTCPPortEgressPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "tcp-port-egress" {
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

// TestPortRangePolicy tests the port range policy using common framework
func TestPortRangePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "port-range" {
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

// TestMultiplePortPolicy tests the multiple port policy using common framework
func TestMultiplePortPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "multiple-port" {
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

// TestICMPTypePolicy tests the ICMP type policy using common framework
func TestICMPTypePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "icmp-type" {
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

// TestICMPv6TypePolicy tests the ICMPv6 type policy using common framework
func TestICMPv6TypePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "icmpv6-type" {
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

// TestMixedICMPPolicy tests the mixed ICMP policy using common framework
func TestMixedICMPPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "mixed-icmp" {
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

// TestBasicSNIPolicy tests the basic SNI policy using common framework
func TestBasicSNIPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "basic-sni" {
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

// TestMultiDomainSNIPolicy tests the multi-domain SNI policy using common framework
func TestMultiDomainSNIPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "multi-domain-sni" {
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

// TestCombinedL4SNIPolicy tests the combined L4 and SNI policy using common framework
func TestCombinedL4SNIPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L4TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L4TestConfigs {
		if cfg.TestId == "combined-l4-sni" {
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

// TestL4PoliciesSequential runs L4 policy tests with sequential execution for clean policy isolation
func TestL4PoliciesSequential(t *core.Tester, ctx context.Context, requestedSubgroups []string, verbose ...bool) []TestResult {
	// Determine verbose mode
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	allResults := []TestResult{}

	if isVerbose {
		fmt.Printf("\n===== L4 POLICIES SEQUENTIAL EXECUTION - VERBOSE MODE ENABLED =====\n")
		fmt.Printf("Verbose output will be shown for each test\n")
		fmt.Printf("============================================================\n\n")
	}

	fmt.Printf("Starting L4 policies tests...\n")

	// Create test groups using the new common framework
	testGroups := BuildL4TestGroups(requestedSubgroups)

	// Execute test groups using the common framework (preserves all real data functionality)
	timedResults, testNames := core.ExecutePolicyTestGroups(
		testGroups,
		t,
		ctx,
		isVerbose,
		t.CaptureRealL4ConnectivityData, // Preserve real L4 data capture functionality
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

	// Create display names mapping for L4 tests
	displayNames := map[string]string{
		"tcp-port-ingress": "TCP Port Ingress Policy",
		"tcp-port-egress":  "TCP Port Egress Policy",
		"port-range":       "Port Range Policy",
		"multiple-port":    "Multiple Port Policy",
		"icmp-type":        "ICMP Type Policy",
		"icmpv6-type":      "ICMPv6 Type Policy",
		"mixed-icmp":       "Mixed ICMP Policy",
		"basic-sni":        "Basic SNI Policy",
		"multi-domain-sni": "Multi-Domain SNI Policy",
		"combined-l4-sni":  "Combined L4 SNI Policy",
	}

	// Display enhanced verbose summary with Expected vs Received details
	core.FormatEnhancedTestSummary(timedResults, testNames, displayNames, isVerbose)

	return allResults
}

// TestL4Policies runs all L4 policy tests (legacy function - uses TestL4PoliciesSequential)
func TestL4Policies(t *core.Tester, ctx context.Context, verbose bool) []TestResult {
	// Use the main sequential function with all subgroups
	allSubgroups := []string{"port", "icmp", "tls-sni"}
	return TestL4PoliciesSequential(t, ctx, allSubgroups, verbose)
}
