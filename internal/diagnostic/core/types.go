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

// =============================================================================
// ENHANCED DATA STRUCTURES FOR COMPREHENSIVE TEST DIAGNOSTICS AND USER FEEDBACK
// =============================================================================

// DetailedTestResult extends TestResult with comprehensive environment and execution data
type DetailedTestResult struct {
	TestResult
	EnvironmentSnapshot *EnvironmentSnapshot `json:"environment_snapshot,omitempty"`
	ExecutionData       *TestExecutionData   `json:"execution_data,omitempty"`
	UserContext         *UserTestContext     `json:"user_context,omitempty"`
}

// EnvironmentSnapshot captures the cluster state at test execution time
type EnvironmentSnapshot struct {
	Infrastructure *ClusterInfrastructure `json:"infrastructure"`
	TestNamespace  string                 `json:"test_namespace"`
	StartTime      time.Time              `json:"start_time"`
	NodeStates     []NodeState            `json:"node_states,omitempty"`
	CNIHealth      CNIHealthSnapshot      `json:"cni_health"`
}

// NodeState represents the state of a single node during test execution
type NodeState struct {
	Name               string            `json:"name"`
	Ready              bool              `json:"ready"`
	SchedulingDisabled bool              `json:"scheduling_disabled"`
	PodCount           int               `json:"pod_count"`
	PodCapacity        int               `json:"pod_capacity"`
	Labels             map[string]string `json:"labels,omitempty"`
	Taints             []string          `json:"taints,omitempty"`
}

// CNIHealthSnapshot captures CNI status at test time
type CNIHealthSnapshot struct {
	Provider           string `json:"provider"`
	Version            string `json:"version,omitempty"`
	PodsRunning        int    `json:"pods_running"`
	PodsTotal          int    `json:"pods_total"`
	ConnectivityCheck  string `json:"connectivity_check"` // "healthy", "unhealthy", "unknown"
	HealthCheckDetails string `json:"health_check_details,omitempty"`
}

// TestExecutionData tracks all runtime test execution details
type TestExecutionData struct {
	PodsCreated       []PodCreationResult       `json:"pods_created"`
	ServicesCreated   []ServiceCreationResult   `json:"services_created"`
	PoliciesApplied   []PolicyApplicationResult `json:"policies_applied,omitempty"`
	ConnectivityTests []ConnectivityTestResult  `json:"connectivity_tests"`
	FailurePoints     []FailurePoint            `json:"failure_points,omitempty"`
	CleanupResults    CleanupResult             `json:"cleanup_results"`
}

// PodCreationResult tracks individual pod deployment with detailed status
type PodCreationResult struct {
	PodName       string     `json:"pod_name"`
	RequestedNode string     `json:"requested_node,omitempty"`
	ActualNode    string     `json:"actual_node"`
	PodIP         string     `json:"pod_ip,omitempty"`
	Status        string     `json:"status"` // "created", "failed", "pending", "running", "timeout"
	CreationTime  time.Time  `json:"creation_time"`
	ReadyTime     *time.Time `json:"ready_time,omitempty"`
	Error         string     `json:"error,omitempty"`
	RestartCount  int        `json:"restart_count"`
	Image         string     `json:"image,omitempty"`
	Resources     string     `json:"resources,omitempty"`
}

// ServiceCreationResult tracks service deployment and accessibility
type ServiceCreationResult struct {
	ServiceName  string     `json:"service_name"`
	ServiceType  string     `json:"service_type"` // "ClusterIP", "NodePort", "LoadBalancer"
	ClusterIP    string     `json:"cluster_ip,omitempty"`
	ExternalIP   string     `json:"external_ip,omitempty"`
	NodePort     int        `json:"node_port,omitempty"`
	Ports        []string   `json:"ports"`
	Status       string     `json:"status"` // "created", "failed", "ready"
	CreationTime time.Time  `json:"creation_time"`
	ReadyTime    *time.Time `json:"ready_time,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// PolicyApplicationResult tracks network policy deployment and status
type PolicyApplicationResult struct {
	PolicyName     string     `json:"policy_name"`
	PolicyType     string     `json:"policy_type"` // "NetworkPolicy", "CiliumNetworkPolicy"
	Namespace      string     `json:"namespace"`
	AppliedTime    time.Time  `json:"applied_time"`
	Status         string     `json:"status"` // "applied", "failed", "verified"
	Error          string     `json:"error,omitempty"`
	VerifiedTime   *time.Time `json:"verified_time,omitempty"`
	EnforcementLog string     `json:"enforcement_log,omitempty"`
}

// ConnectivityTestResult tracks individual connectivity test attempts with comprehensive data
type ConnectivityTestResult struct {
	SourcePod      string    `json:"source_pod"`
	SourcePodIP    string    `json:"source_pod_ip,omitempty"`
	TargetPod      string    `json:"target_pod,omitempty"`
	TargetPodIP    string    `json:"target_pod_ip,omitempty"`
	TargetService  string    `json:"target_service,omitempty"`
	TestType       string    `json:"test_type"` // "http", "dns", "ping", "tcp"
	StartTime      time.Time `json:"start_time"`
	Duration       float64   `json:"duration_seconds"`
	Success        bool      `json:"success"`
	HTTPStatusCode string    `json:"http_status_code,omitempty"`
	ResponseBody   string    `json:"response_body,omitempty"`
	Error          string    `json:"error,omitempty"`
	NetworkPath    []string  `json:"network_path,omitempty"` // IPs traversed
	DNSResponse    string    `json:"dns_response,omitempty"`
	Command        string    `json:"command,omitempty"` // The actual command executed
}

// FailurePoint represents a specific point of failure during test execution
type FailurePoint struct {
	Phase       string                 `json:"phase"`     // "environment", "setup", "execution", "verification", "cleanup"
	Component   string                 `json:"component"` // "pod", "service", "policy", "network", "dns"
	Error       string                 `json:"error"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Remediation []string               `json:"remediation_suggestions,omitempty"`
}

// CleanupResult tracks cleanup operations success/failure
type CleanupResult struct {
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	PodsDeleted      int       `json:"pods_deleted"`
	ServicesDeleted  int       `json:"services_deleted"`
	PoliciesDeleted  int       `json:"policies_deleted"`
	NamespaceDeleted bool      `json:"namespace_deleted"`
	CleanupErrors    []string  `json:"cleanup_errors,omitempty"`
	CompletionStatus string    `json:"completion_status"` // "complete", "partial", "failed"
}

// UserTestContext provides user-friendly context and messaging
type UserTestContext struct {
	Summary      string   `json:"summary"`      // High-level result summary
	Details      string   `json:"details"`      // What happened during the test
	Implications string   `json:"implications"` // What this means for their cluster
	Hints        []string `json:"hints"`        // Actionable next steps
}

// =============================================================================
// REUSABLE VALIDATION RESULT TYPES FOR COMMON TEST PATTERNS
// =============================================================================

// ValidationResult represents the result of a common validation check (environment, resource, connectivity)
type ValidationResult struct {
	Success        bool                   `json:"success"`
	UserMessage    UserMessage            `json:"user_message"`    // User-friendly message with context and hints
	TechnicalData  map[string]interface{} `json:"technical_data"`  // Technical details for customer service
	FailureHints   []string               `json:"failure_hints"`   // Specific remediation suggestions
	Duration       float64                `json:"duration"`        // Time taken for this validation
	ComponentType  string                 `json:"component_type"`  // "environment", "resource", "connectivity"
	ComponentName  string                 `json:"component_name"`  // "worker-nodes", "pod", "http-connectivity"
	ValidationType string                 `json:"validation_type"` // "node-count", "pod-creation", "http-test"
}

// EnvironmentValidationResult specifically for environment checks (node count, cluster access, CNI health)
type EnvironmentValidationResult struct {
	ValidationResult
	NodeCount          int                    `json:"node_count,omitempty"`
	CNIProvider        string                 `json:"cni_provider,omitempty"`
	CNIVersion         string                 `json:"cni_version,omitempty"`
	ClusterVersion     string                 `json:"cluster_version,omitempty"`
	EnvironmentDetails map[string]interface{} `json:"environment_details,omitempty"`
}

// ResourceValidationResult specifically for resource creation/management checks
type ResourceValidationResult struct {
	ValidationResult
	ResourceName   string     `json:"resource_name"`
	ResourceType   string     `json:"resource_type"`   // "pod", "service", "deployment"
	ResourceStatus string     `json:"resource_status"` // "created", "failed", "pending", "running", "timeout"
	CreationTime   time.Time  `json:"creation_time"`
	ReadyTime      *time.Time `json:"ready_time,omitempty"`
	ActualNode     string     `json:"actual_node,omitempty"`    // For pod placement validation
	RequestedNode  string     `json:"requested_node,omitempty"` // For cross-node test validation
	ResourceIP     string     `json:"resource_ip,omitempty"`    // Pod IP or Service IP
	Error          string     `json:"error,omitempty"`
}

// ConnectivityValidationResult specifically for connectivity test checks
type ConnectivityValidationResult struct {
	ValidationResult
	SourcePod     string   `json:"source_pod"`
	TargetPod     string   `json:"target_pod,omitempty"`
	TargetService string   `json:"target_service,omitempty"`
	TestType      string   `json:"test_type"`              // "http", "dns", "ping", "tcp"
	StatusCode    string   `json:"status_code"`            // HTTP status or test result code
	ResponseTime  float64  `json:"response_time"`          // Response time in seconds
	NetworkPath   []string `json:"network_path,omitempty"` // IPs traversed
	DNSResponse   string   `json:"dns_response,omitempty"`
	Command       string   `json:"command,omitempty"` // Actual command executed
	ResponseBody  string   `json:"response_body,omitempty"`
}

// TestExecutionConfig defines configuration for reusable test execution patterns
type TestExecutionConfig struct {
	TestName          string             `json:"test_name"`
	TestType          string             `json:"test_type"`          // "cross-node", "same-node", "service", "dns"
	MinWorkerNodes    int                `json:"min_worker_nodes"`   // Minimum nodes required
	RequiredResources []ResourceSpec     `json:"required_resources"` // Resources to create
	ConnectivityTests []ConnectivitySpec `json:"connectivity_tests"` // Connectivity tests to run
	Timeout           time.Duration      `json:"timeout"`            // Test timeout
	RetryCount        int                `json:"retry_count"`        // Number of retries for failed operations
	CleanupOnFailure  bool               `json:"cleanup_on_failure"` // Whether to cleanup on test failure
	UserContext       map[string]string  `json:"user_context"`       // Additional user context for messaging
}

// ResourceSpec defines a resource that needs to be created for a test
type ResourceSpec struct {
	Type          string `json:"type"`                   // "pod", "service", "deployment"
	Name          string `json:"name"`                   // Resource name
	NodePlacement string `json:"node_placement"`         // "node-0", "node-1", "any"
	Image         string `json:"image,omitempty"`        // Container image
	ServiceType   string `json:"service_type,omitempty"` // For services: "ClusterIP", "NodePort"
	Port          int    `json:"port,omitempty"`         // Service port
}

// ConnectivitySpec defines a connectivity test to perform
type ConnectivitySpec struct {
	Source   string `json:"source"`         // Source resource name
	Target   string `json:"target"`         // Target resource name or IP
	Protocol string `json:"protocol"`       // "http", "dns", "ping", "tcp"
	Port     int    `json:"port,omitempty"` // Port to test
	Expected string `json:"expected"`       // Expected result: "success", "failure", "blocked"
}
