package core

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CommandOutput represents a command execution result
type CommandOutput struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Description string `json:"description"`
}

// NetworkContext represents network diagnostic information
type NetworkContext struct {
	SourcePodIP    string            `json:"source_pod_ip,omitempty"`
	TargetPodIP    string            `json:"target_pod_ip,omitempty"`
	ServiceIP      string            `json:"service_ip,omitempty"`
	SourceNode     string            `json:"source_node,omitempty"`
	TargetNode     string            `json:"target_node,omitempty"`
	RoutingInfo    []string          `json:"routing_info,omitempty"`
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}

// DetailedDiagnostics represents comprehensive diagnostic information
type DetailedDiagnostics struct {
	FailureStage         string          `json:"failure_stage,omitempty"`
	TechnicalError       string          `json:"technical_error,omitempty"`
	CommandOutputs       []CommandOutput `json:"command_outputs,omitempty"`
	NetworkContext       *NetworkContext `json:"network_context,omitempty"`
	TroubleshootingHints []string        `json:"troubleshooting_hints,omitempty"`
}

// TestConfig represents configuration for test execution
type TestConfig struct {
	Placement string `json:"placement"` // "same-node", "cross-node", "both"
}

// TestResult represents the result of a connectivity test
type TestResult struct {
	Success             bool                 `json:"success"`
	Message             string               `json:"message"`
	Details             []string             `json:"details"`
	DetailedDiagnostics *DetailedDiagnostics `json:"detailed_diagnostics,omitempty"`
}

// SubgroupTestResult represents a group of test results with timing information
type SubgroupTestResult struct {
	Name        string        `json:"name"`
	Results     []TestResult  `json:"results"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	ElapsedTime time.Duration `json:"elapsed_time"`
}

// TimedTestResult represents a test result with timing information
type TimedTestResult struct {
	TestResult
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ServiceType is a type for supported Kubernetes service types
type ServiceType int

const (
	// ServiceTypeClusterIP is for ClusterIP services
	ServiceTypeClusterIP ServiceType = iota
	// ServiceTypeNodePort is for NodePort services
	ServiceTypeNodePort
	// ServiceTypeLoadBalancer is for LoadBalancer services
	ServiceTypeLoadBalancer
)

// PolicyTracker tracks applied network policies for cleanup
type PolicyTracker struct {
	appliedPolicies []string
}

// NewPolicyTracker creates a new policy tracker
func NewPolicyTracker() *PolicyTracker {
	return &PolicyTracker{
		appliedPolicies: make([]string, 0),
	}
}

// ResourceCache caches resources for reuse across tests
type ResourceCache struct {
	Pods     map[string]interface{}
	Services map[string]interface{}
	Policies map[string]interface{}
}

// Tester handles connectivity testing operations
type Tester struct {
	clientset       *kubernetes.Clientset
	config          *rest.Config
	namespace       string
	policyTracker   *PolicyTracker
	nodeInfo        map[string]string
	resourceCache   *ResourceCache   // For caching and reusing resources across tests
	namespaceSuffix string           // Used to create unique secondary namespaces for concurrent tests
	verbose         bool             // Controls verbose output for all test operations
	lastExecutor    *CommandExecutor // Track the last command executor used for error details
}

// Enhanced verbose mode types for detailed command tracking and error reporting

// VerboseCommandExecution tracks individual command execution with full details
type VerboseCommandExecution struct {
	Command   string    `json:"command"`
	ExitCode  int       `json:"exit_code"`
	Stdout    string    `json:"stdout"`
	Stderr    string    `json:"stderr"`
	Duration  float64   `json:"duration"` // Duration in seconds
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// VerboseErrorDetails provides detailed error information for failed tests
type VerboseErrorDetails struct {
	Command string `json:"command"`
	Output  string `json:"output"`
	Stage   string `json:"stage"`
}

// ConnectivityResult represents individual connection test results
type ConnectivityResult struct {
	Source     string  `json:"source"`      // client1, client2, etc.
	Target     string  `json:"target"`      // target pod IP or service
	Protocol   string  `json:"protocol"`    // HTTP, ICMP, DNS
	StatusCode string  `json:"status_code"` // 200, timeout, etc.
	Duration   float64 `json:"duration"`    // Response time in seconds
	Success    bool    `json:"success"`     // Whether this specific connection succeeded
}

// PolicyExpectation defines what a policy should do
type PolicyExpectation struct {
	Expected    string `json:"expected"`     // Expected outcome description
	Explanation string `json:"explanation"`  // Why this outcome is expected
	Protocol    string `json:"protocol"`     // Primary protocol being tested
	ShouldAllow bool   `json:"should_allow"` // Whether policy should allow or deny
}

// EnhancedTestResult extends TestResult with comprehensive verbose tracking and expected vs received details
type EnhancedTestResult struct {
	TestResult
	// Verbose command tracking
	ExecutedCommands []VerboseCommandExecution `json:"executed_commands"`
	FailurePoint     string                    `json:"failure_point"`
	ErrorDetails     *VerboseErrorDetails      `json:"error_details"`
	// Expected vs Received details
	ExpectedOutcome string               `json:"expected_outcome"` // What we expected to happen
	ReceivedOutcome string               `json:"received_outcome"` // What actually happened
	TestDetails     []ConnectivityResult `json:"test_details"`     // Individual connection results
	PolicyBehavior  string               `json:"policy_behavior"`  // How the policy should behave
}

// VerboseTestSummary provides formatted output for verbose test failures
type VerboseTestSummary struct {
	TestName     string
	Duration     float64
	Success      bool
	CommandTrace []VerboseCommandExecution
	FailurePoint string
	ErrorDetails *VerboseErrorDetails
}
