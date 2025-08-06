package diagnostic

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// L3PolicyDisplayNames maps L3 test keys to user-friendly display names
var L3PolicyDisplayNames = map[string]string{
	"cidr-ingress":       "CIDR Ingress Policy",
	"cidr-egress":        "CIDR Egress Policy",
	"cidr-except":        "CIDR With Except Policy",
	"endpoints-label":    "Endpoints Label Policy",
	"entities-based":     "Entities Based Policy",
	"dns-based":          "DNS Based Policy",
	"node-selector":      "Traditional Node Selector Policy",
	"pod-node-name":      "Pod Node Name Policy",
	"node-cidr":          "Node CIDR Policy",
	"node-based":         "Node Based Policy",
	"kubernetes-service": "Kubernetes Service Policy",
}

// L4PolicyDisplayNames maps L4 test keys to user-friendly display names
var L4PolicyDisplayNames = map[string]string{
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

// printInfo prints an informational message with a timestamp
func printInfo(message string) {
	fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
}

// PolicyGroup represents a group of related policy tests that should be cleaned up together
type PolicyGroup struct {
	PolicyFile   string
	AppliedName  string
	TestsInGroup []string // Test function names that use this policy
	TestOrder    []string // Order of tests in the group (for tracking what's run)
	CurrentTest  int      // Index of current test being run
}

// PolicyTracker tracks applied policies and their cleanup status
type PolicyTracker struct {
	mu           sync.Mutex
	policyFiles  map[string]*PolicyGroup // Maps policy file paths to their policy groups
	testSchedule []string                // Ordered list of all tests to be run
	testToPolicy map[string]string       // Maps test names to their policy files
	currentTest  int                     // Current position in test schedule
}

// NewPolicyTracker creates a new policy tracker
func NewPolicyTracker() *PolicyTracker {
	return &PolicyTracker{
		policyFiles:  make(map[string]*PolicyGroup),
		testSchedule: make([]string, 0),
		testToPolicy: make(map[string]string),
		currentTest:  0,
	}
}

// InitTestSchedule initializes the test schedule with an ordered list of tests
func (pt *PolicyTracker) InitTestSchedule(tests []string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.testSchedule = tests
	pt.currentTest = 0
}

// GetCurrentTest returns the current test name
func (pt *PolicyTracker) GetCurrentTest() string {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.currentTest < len(pt.testSchedule) {
		return pt.testSchedule[pt.currentTest]
	}
	return ""
}

// AdvanceTest moves to the next test in the schedule
func (pt *PolicyTracker) AdvanceTest() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.currentTest < len(pt.testSchedule) {
		pt.currentTest++
	}
}

// HasMoreTests checks if there are more tests to run
func (pt *PolicyTracker) HasMoreTests() bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	return pt.currentTest < len(pt.testSchedule)
}

// RegisterTest registers a test with a specific policy file
// This helps in grouping tests by policy file for efficient cleanup
func (pt *PolicyTracker) RegisterTest(testName, policyFile string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Map test to policy file
	pt.testToPolicy[testName] = policyFile

	if group, exists := pt.policyFiles[policyFile]; exists {
		// Add this test to the existing group if not already there
		found := false
		for _, test := range group.TestsInGroup {
			if test == testName {
				found = true
				break
			}
		}
		if !found {
			group.TestsInGroup = append(group.TestsInGroup, testName)
			group.TestOrder = append(group.TestOrder, testName)
		}
	} else {
		// Create a new group for this policy file
		pt.policyFiles[policyFile] = &PolicyGroup{
			PolicyFile:   policyFile,
			TestsInGroup: []string{testName},
			TestOrder:    []string{testName},
			CurrentTest:  0,
		}
	}
}

// RecordAppliedPolicy records the applied policy name for a policy file
func (pt *PolicyTracker) RecordAppliedPolicy(policyFile, appliedName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if group, exists := pt.policyFiles[policyFile]; exists {
		group.AppliedName = appliedName
	}
}

// GetAppliedPolicy returns the applied policy name for a policy file
func (pt *PolicyTracker) GetAppliedPolicy(policyFile string) (string, bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if group, exists := pt.policyFiles[policyFile]; exists && group.AppliedName != "" {
		return group.AppliedName, true
	}
	return "", false
}

// ClearAppliedPolicy clears the applied policy name for a policy file
func (pt *PolicyTracker) ClearAppliedPolicy(policyFile string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if group, exists := pt.policyFiles[policyFile]; exists {
		group.AppliedName = ""
	}
}

// PolicyCleanupDecision contains the decision and reasoning for policy cleanup
type PolicyCleanupDecision struct {
	ShouldCleanup     bool
	Reason            string
	CurrentPolicyFile string
	NextPolicyFile    string
	NextTestName      string
	IsLastTest        bool
}

// ShouldCleanupPolicy determines if we should clean up a policy after the current test
// It checks if the next test will use the same policy file and returns detailed reasoning
func (pt *PolicyTracker) ShouldCleanupPolicy(testName string) PolicyCleanupDecision {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	decision := PolicyCleanupDecision{}

	// Get the policy file for the current test
	currentPolicyFile, exists := pt.testToPolicy[testName]
	if !exists {
		decision.ShouldCleanup = true
		decision.Reason = fmt.Sprintf("Test '%s' not registered with a policy file, forcing cleanup", testName)
		return decision
	}
	decision.CurrentPolicyFile = currentPolicyFile

	// If this is the last test in the schedule, clean up
	if pt.currentTest >= len(pt.testSchedule)-1 {
		decision.ShouldCleanup = true
		decision.IsLastTest = true
		decision.Reason = "This is the last test in schedule, will clean up"
		return decision
	}

	// Get the next test in the schedule
	nextTestName := pt.testSchedule[pt.currentTest+1]
	decision.NextTestName = nextTestName

	// Check if the next test uses the same policy file
	nextPolicyFile, exists := pt.testToPolicy[nextTestName]
	if !exists {
		decision.ShouldCleanup = true
		decision.NextPolicyFile = ""
		decision.Reason = fmt.Sprintf("Next test '%s' not registered with a policy file, forcing cleanup", nextTestName)
		return decision
	}
	decision.NextPolicyFile = nextPolicyFile

	decision.ShouldCleanup = nextPolicyFile != currentPolicyFile
	if decision.ShouldCleanup {
		decision.Reason = "Next test uses a different policy file"
	} else {
		decision.Reason = "Next test uses the same policy file"
	}

	return decision
}

// GetNextTestForPolicy returns the name of the next test that will use the same policy file
// Useful for providing information about resource reuse
func (pt *PolicyTracker) GetNextTestForPolicy(policyFile string) string {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.currentTest >= len(pt.testSchedule)-1 {
		// No more tests
		return ""
	}

	// Check next test
	nextTestName := pt.testSchedule[pt.currentTest+1]
	nextPolicyFile, exists := pt.testToPolicy[nextTestName]

	if exists && nextPolicyFile == policyFile {
		return nextTestName
	}

	return ""
}

// PolicyTemplateResult contains the result of processing a policy template
type PolicyTemplateResult struct {
	ProcessedFilePath  string
	VariablesReplaced  map[string]string
	UnprocessedVars    []string
	WarningsGenerated  []string
	UsedFallbackValues bool
	ContentWasModified bool
}

// ProcessPolicyTemplate reads a policy template file and replaces variables with actual values
// Returns detailed result information for the caller to handle output
func ProcessPolicyTemplate(policyFilePath, namespace string, nodeInfo map[string]string) (PolicyTemplateResult, error) {
	result := PolicyTemplateResult{
		VariablesReplaced: make(map[string]string),
		WarningsGenerated: make([]string, 0),
		UnprocessedVars:   make([]string, 0),
	}

	// Read the policy template file
	content, err := ioutil.ReadFile(policyFilePath)
	if err != nil {
		return result, fmt.Errorf("failed to read policy template file: %v", err)
	}

	// Create a temporary file to store the processed policy
	tmpDir, err := ioutil.TempDir("", "policy-*")
	if err != nil {
		return result, fmt.Errorf("failed to create temp directory: %v", err)
	}

	tmpFile := filepath.Join(tmpDir, filepath.Base(policyFilePath))
	result.ProcessedFilePath = tmpFile

	processedContent := string(content)
	originalContent := processedContent

	// Replace variables with their actual values
	processedContent = strings.ReplaceAll(processedContent, "{{NS_NAME}}", namespace)
	result.VariablesReplaced["NS_NAME"] = namespace

	// Replace node-related variables if they exist in nodeInfo
	if nodeName, ok := nodeInfo["NODE1"]; ok {
		processedContent = strings.ReplaceAll(processedContent, "{{NODE1}}", nodeName)
		result.VariablesReplaced["NODE1"] = nodeName
	} else if strings.Contains(processedContent, "{{NODE1}}") {
		defaultValue := "worker-node-1"
		processedContent = strings.ReplaceAll(processedContent, "{{NODE1}}", defaultValue)
		result.VariablesReplaced["NODE1"] = defaultValue
		result.UsedFallbackValues = true
		result.WarningsGenerated = append(result.WarningsGenerated, "NODE1 variable found but no node information available, using fallback")
	}

	if nodeName, ok := nodeInfo["NODE2"]; ok {
		processedContent = strings.ReplaceAll(processedContent, "{{NODE2}}", nodeName)
		result.VariablesReplaced["NODE2"] = nodeName
	} else if strings.Contains(processedContent, "{{NODE2}}") {
		defaultValue := "worker-node-2"
		processedContent = strings.ReplaceAll(processedContent, "{{NODE2}}", defaultValue)
		result.VariablesReplaced["NODE2"] = defaultValue
		result.UsedFallbackValues = true
		result.WarningsGenerated = append(result.WarningsGenerated, "NODE2 variable found but no node information available, using fallback")
	}

	// Always provide valid CIDR values for NODE1_CIDR and NODE2_CIDR
	if nodeCIDR, ok := nodeInfo["NODE1_CIDR"]; ok && nodeCIDR != "" {
		processedContent = strings.ReplaceAll(processedContent, "{{NODE1_CIDR}}", nodeCIDR)
		result.VariablesReplaced["NODE1_CIDR"] = nodeCIDR
	} else {
		fallbackCIDR := "10.0.0.0/16"
		processedContent = strings.ReplaceAll(processedContent, "{{NODE1_CIDR}}", fallbackCIDR)
		result.VariablesReplaced["NODE1_CIDR"] = fallbackCIDR
		result.UsedFallbackValues = true
		result.WarningsGenerated = append(result.WarningsGenerated, "Using fallback NODE1_CIDR: "+fallbackCIDR)
	}

	if nodeCIDR, ok := nodeInfo["NODE2_CIDR"]; ok && nodeCIDR != "" {
		processedContent = strings.ReplaceAll(processedContent, "{{NODE2_CIDR}}", nodeCIDR)
		result.VariablesReplaced["NODE2_CIDR"] = nodeCIDR
	} else {
		fallbackCIDR := "10.1.0.0/16"
		processedContent = strings.ReplaceAll(processedContent, "{{NODE2_CIDR}}", fallbackCIDR)
		result.VariablesReplaced["NODE2_CIDR"] = fallbackCIDR
		result.UsedFallbackValues = true
		result.WarningsGenerated = append(result.WarningsGenerated, "Using fallback NODE2_CIDR: "+fallbackCIDR)
	}

	// Check if there are any remaining unprocessed variables
	unprocessedVars := regexp.MustCompile(`\{\{[^}]+\}\}`).FindAllString(processedContent, -1)
	result.UnprocessedVars = unprocessedVars

	if len(unprocessedVars) > 0 {
		result.WarningsGenerated = append(result.WarningsGenerated, fmt.Sprintf("Found %d unprocessed variables", len(unprocessedVars)))

		// For unprocessed variables that might cause policy validation issues,
		// replace them with safe default values
		for _, v := range unprocessedVars {
			varName := strings.Trim(v, "{}")

			// Choose appropriate defaults based on variable name
			var defaultValue string
			if strings.Contains(strings.ToLower(varName), "cidr") {
				defaultValue = "10.0.0.0/16"
			} else if strings.Contains(strings.ToLower(varName), "node") {
				defaultValue = "default-node"
			} else if strings.Contains(strings.ToLower(varName), "namespace") || strings.Contains(strings.ToLower(varName), "ns") {
				defaultValue = "default"
			} else {
				defaultValue = "default-value"
			}

			processedContent = strings.ReplaceAll(processedContent, v, defaultValue)
			result.VariablesReplaced[varName] = defaultValue
			result.UsedFallbackValues = true
		}
	}

	// Write the processed content to the temporary file
	if err := ioutil.WriteFile(tmpFile, []byte(processedContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write processed policy to temporary file: %v", err)
	}

	// Check if the content was actually modified
	result.ContentWasModified = processedContent != originalContent

	return result, nil
}

// ApplyProcessedPolicy applies a processed policy file to the cluster and extracts its name
func ApplyProcessedPolicy(ctx context.Context, processedFilePath string) (string, error) {
	// Extract policy name pattern (metadata.name)
	namePattern := regexp.MustCompile(`metadata:\s*\n\s*name:\s*"?([^"\n]+)"?`)

	// Read the file to extract the policy name
	content, err := ioutil.ReadFile(processedFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read processed policy file: %v", err)
	}

	// Find the policy name
	matches := namePattern.FindStringSubmatch(string(content))
	policyName := ""
	if len(matches) > 1 {
		policyName = matches[1]
	}

	// Apply the policy using kubectl
	cmd := fmt.Sprintf("kubectl apply -f %s", processedFilePath)
	output, err := ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to apply policy: %v, output: %s", err, output)
	}

	// If policy name wasn't extracted from the file, try to extract it from the output
	if policyName == "" {
		outputPattern := regexp.MustCompile(`(ciliumnetworkpolicy|ciliumclusterwidenetworkpolicy)\.cilium\.io/([^\s]+)\s+created`)
		matches = outputPattern.FindStringSubmatch(output)
		if len(matches) > 2 {
			policyName = matches[2]
		}
	}

	// Wait for policy to be fully applied and synchronized
	// This is similar to the sleep 10 in the bash script
	fmt.Printf("Waiting for policy to be applied...\n")
	ExecuteCommand("sleep 10")

	return policyName, nil
}

// ExecuteCommand executes a command and returns the combined output
func ExecuteCommand(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ExecuteCommandVerbose executes a command with guaranteed verbose output
// Useful for critical commands that should always show output in verbose mode
func ExecuteCommandVerbose(command string, verbose bool) (string, error) {
	if verbose {
		fmt.Printf("\n==== COMMAND EXECUTION ====\n")
		fmt.Printf("COMMAND: %s\n", command)
	}

	// Record start time
	startTime := time.Now()

	// Execute command
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()

	// Calculate duration
	duration := time.Since(startTime)

	if verbose {
		fmt.Printf("\n==== COMMAND OUTPUT ====\n")
		fmt.Printf("%s\n", string(output))
		fmt.Printf("\n==== COMMAND RESULT ====\n")
		if err != nil {
			fmt.Printf("STATUS: ERROR (%.2fs)\n", duration.Seconds())
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Printf("STATUS: SUCCESS (%.2fs)\n", duration.Seconds())
		}
		fmt.Printf("===========================\n\n")
	}

	return string(output), err
}

// NodeInfoResult contains the result of collecting node information
type NodeInfoResult struct {
	NodeInfo      map[string]string
	NodesFound    []string
	UsedFallbacks []string
	Warnings      []string
	Errors        []string
}

// GetNodeInfo collects node information from the cluster
// Returns detailed result information for the caller to handle output
func GetNodeInfo(ctx context.Context) (NodeInfoResult, error) {
	result := NodeInfoResult{
		NodeInfo:      make(map[string]string),
		NodesFound:    make([]string, 0),
		UsedFallbacks: make([]string, 0),
		Warnings:      make([]string, 0),
		Errors:        make([]string, 0),
	}

	// Get worker nodes
	cmd := "kubectl get nodes -o jsonpath='{.items[*].metadata.name}'"
	output, err := ExecuteCommand(cmd)
	if err != nil {
		result.Errors = append(result.Errors, "Failed to get worker nodes: "+err.Error())
		return result, fmt.Errorf("failed to get worker nodes: %v", err)
	}

	nodes := strings.Fields(output)
	if len(nodes) == 0 {
		result.Errors = append(result.Errors, "No nodes found in the cluster")
		return result, fmt.Errorf("no nodes found in the cluster")
	}

	result.NodesFound = nodes

	// Store node names
	result.NodeInfo["NODE1"] = nodes[0]
	if len(nodes) > 1 {
		result.NodeInfo["NODE2"] = nodes[1]
	} else {
		result.NodeInfo["NODE2"] = nodes[0]
		result.Warnings = append(result.Warnings, fmt.Sprintf("Using the same node for NODE1 and NODE2: %s", nodes[0]))
	}

	// Get a pod on each node to determine CIDR ranges
	for i := 1; i <= 2; i++ {
		nodeName := result.NodeInfo[fmt.Sprintf("NODE%d", i)]

		// Always set fallback CIDR values first, will be overwritten if successful detection occurs
		fallbackCIDR := fmt.Sprintf("10.%d.0.0/16", i-1)
		result.NodeInfo[fmt.Sprintf("NODE%d_CIDR", i)] = fallbackCIDR

		// Method 1: Run a test pod on the node
		cmd = fmt.Sprintf(`kubectl run cidr-test-%d --image=nginx:alpine --overrides='{"spec":{"nodeName":"%s"}}' --rm -i --restart=Never -- cat /etc/hosts | grep -v "127.0.0.1" | head -n 1 | awk '{print $1}'`, i, nodeName)
		podIP, err := ExecuteCommand(cmd)

		// Check if the output contains an error message
		if err != nil || strings.TrimSpace(podIP) == "" || strings.Contains(podIP, "Error") {
			// Method 2: Find an existing pod on the node
			cmd = fmt.Sprintf(`kubectl get pods -o wide --all-namespaces | grep %s | head -n 1 | awk '{print $7}'`, nodeName)
			podIP, err = ExecuteCommand(cmd)

			if err != nil || strings.TrimSpace(podIP) == "" || strings.Contains(podIP, "Error") {
				// Method 3: Get the internal IP of the node itself
				cmd = fmt.Sprintf(`kubectl get node %s -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}'`, nodeName)
				podIP, err = ExecuteCommand(cmd)

				if err != nil || strings.TrimSpace(podIP) == "" || strings.Contains(podIP, "Error") {
					result.UsedFallbacks = append(result.UsedFallbacks, fmt.Sprintf("NODE%d_CIDR (failed to determine from node %s)", i, nodeName))
					continue
				}
			}
		}

		podIP = strings.TrimSpace(podIP)
		if podIP != "" && !strings.Contains(podIP, "Error") && isValidIPAddress(podIP) {
			// Convert pod IP to CIDR by replacing the last octet with 0/24
			cidr := regexp.MustCompile(`(\d+\.\d+\.\d+)\.\d+`).ReplaceAllString(podIP, "${1}.0/24")
			if isValidCIDR(cidr) {
				result.NodeInfo[fmt.Sprintf("NODE%d_CIDR", i)] = cidr
			} else {
				result.UsedFallbacks = append(result.UsedFallbacks, fmt.Sprintf("NODE%d_CIDR (invalid CIDR generated from IP %s)", i, podIP))
			}
		} else {
			result.UsedFallbacks = append(result.UsedFallbacks, fmt.Sprintf("NODE%d_CIDR (could not extract valid IP from output: %s)", i, podIP))
		}
	}

	return result, nil
}

// isValidIPAddress checks if the string is a valid IPv4 address
func isValidIPAddress(ip string) bool {
	// Basic format check
	ipPattern := regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
	if !ipPattern.MatchString(ip) {
		return false
	}

	// Use net.ParseIP for strict validation
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() != nil
}

// isValidCIDR checks if the string is a valid CIDR notation
func isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}
