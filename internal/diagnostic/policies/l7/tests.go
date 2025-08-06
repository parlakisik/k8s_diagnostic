package l7

import (
	"context"
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// Type aliases for compatibility
type TestResult = core.TestResult

// Function aliases
var ElapsedSeconds = core.ElapsedSeconds

// L7PolicySubgroups defines test subgroups for organization and concurrent execution
// Exported so it can be accessed from cmd/test.go for validation
var L7PolicySubgroups = map[string][]string{
	"http": {"basic-http-get", "http-with-headers", "path-method"},
	"dns":  {"dns-matchname", "dns-matchpattern"},
}

// Map of test names to test keys (for CLI reference)
var L7PolicyTestNameToKey = map[string]string{
	"basic-http-get-policy":    "basic-http-get",
	"http-with-headers-policy": "http-with-headers",
	"path-method-policy":       "path-method",
	"dns-matchname-policy":     "dns-matchname",
	"dns-matchpattern-policy":  "dns-matchpattern",
}

// TestBasicHTTPGetPolicy tests the basic HTTP GET policy using common framework
func TestBasicHTTPGetPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L7TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L7TestConfigs {
		if cfg.TestId == "basic-http-get" {
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

// TestHTTPWithHeadersPolicy tests the HTTP policy with headers using common framework
func TestHTTPWithHeadersPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L7TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L7TestConfigs {
		if cfg.TestId == "http-with-headers" {
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

// TestPathMethodPolicy tests the path and method based policy using common framework
func TestPathMethodPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L7TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L7TestConfigs {
		if cfg.TestId == "path-method" {
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

// TestDNSMatchNamePolicy tests the DNS matchName policy (Pure L7) using common framework
func TestDNSMatchNamePolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L7TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L7TestConfigs {
		if cfg.TestId == "dns-matchname" {
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

// TestDNSMatchPatternPolicy tests the DNS matchPattern policy (Pure L7) using common framework
func TestDNSMatchPatternPolicy(logger *core.MultiChannelLogger, t *core.Tester, ctx context.Context, reuseResources bool, verbose bool, testNumber int, totalTests int) TestResult {
	// Find config from L7TestConfigs (no more hardcoded values)
	var config core.PolicyTestConfig
	for _, cfg := range L7TestConfigs {
		if cfg.TestId == "dns-matchpattern" {
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

// TestL7PoliciesSequential runs L7 policy tests with sequential execution for clean policy isolation
func TestL7PoliciesSequential(t *core.Tester, ctx context.Context, requestedSubgroups []string, verbose ...bool) []TestResult {
	// Determine verbose mode
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	allResults := []TestResult{}

	if isVerbose {
		fmt.Printf("\n===== L7 POLICIES SEQUENTIAL EXECUTION - VERBOSE MODE ENABLED =====\n")
		fmt.Printf("Verbose output will be shown for each test\n")
		fmt.Printf("============================================================\n\n")
	}

	fmt.Printf("Starting L7 policies tests...\n")

	// Create test groups using the new common framework
	testGroups := BuildL7TestGroups(requestedSubgroups)

	// Execute test groups using the common framework (preserves all real data functionality)
	timedResults, testNames := core.ExecutePolicyTestGroups(
		testGroups,
		t,
		ctx,
		isVerbose,
		t.CaptureRealL7ConnectivityData, // Preserve real data capture functionality
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

	// Create display names mapping for L7 tests
	displayNames := map[string]string{
		"basic-http-get":    "Basic HTTP GET Policy",
		"http-with-headers": "HTTP With Headers Policy",
		"path-method":       "Path Method Policy",
		"dns-matchname":     "DNS Match Name Policy",
		"dns-matchpattern":  "DNS Match Pattern Policy",
	}

	// Display enhanced verbose summary with Expected vs Received details
	core.FormatEnhancedTestSummary(timedResults, testNames, displayNames, isVerbose)

	return allResults
}

// TestL7Policies runs all L7 policy tests (legacy function - uses TestL7PoliciesSequential)
func TestL7Policies(t *core.Tester, ctx context.Context, verbose bool) []TestResult {
	// Use the main sequential function with all subgroups
	allSubgroups := []string{"http", "dns"}
	return TestL7PoliciesSequential(t, ctx, allSubgroups, verbose)
}
