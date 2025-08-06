package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// KubectlCommandInfo holds parsed kubectl command information
type KubectlCommandInfo struct {
	IsKubectl    bool
	Operation    string
	Resource     string
	ResourceName string
	Namespace    string
	Args         []string
}

// CommandExecutor provides enhanced command execution with real-time logging
type CommandExecutor struct {
	logger           *MultiChannelLogger
	namespace        string
	verbose          bool
	executedCommands []VerboseCommandExecution // Track all executed commands for verbose mode
}

// NewCommandExecutor creates a new command executor with logging
func NewCommandExecutor(logger *MultiChannelLogger, namespace string, verbose bool) *CommandExecutor {
	return &CommandExecutor{
		logger:           logger,
		namespace:        namespace,
		verbose:          verbose,
		executedCommands: make([]VerboseCommandExecution, 0),
	}
}

// GetCommandHistory returns the history of executed commands for verbose reporting
func (ce *CommandExecutor) GetCommandHistory() []VerboseCommandExecution {
	return ce.executedCommands
}

// ClearCommandHistory clears the command execution history
func (ce *CommandExecutor) ClearCommandHistory() {
	ce.executedCommands = make([]VerboseCommandExecution, 0)
}

// AddConnectivityCommand adds a connectivity test command (curl, ping, etc.) to the command history
func (ce *CommandExecutor) AddConnectivityCommand(command VerboseCommandExecution) {
	ce.executedCommands = append(ce.executedCommands, command)
}

// trackCommand records a command execution for verbose reporting
func (ce *CommandExecutor) trackCommand(command string, exitCode int, stdout, stderr string, duration float64, success bool) {
	execution := VerboseCommandExecution{
		Command:   command,
		ExitCode:  exitCode,
		Stdout:    stdout,
		Stderr:    stderr,
		Duration:  duration,
		Timestamp: time.Now(),
		Success:   success,
	}
	ce.executedCommands = append(ce.executedCommands, execution)
}

// ExecuteCommand executes a command with comprehensive logging
func (ce *CommandExecutor) ExecuteCommand(ctx context.Context, command string, workingDir string) (string, error) {
	startTime := time.Now()

	// Log command start
	cmdID, err := ce.logger.LogCommand(command, workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to log command: %v", err)
	}

	// Parse command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create command
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Calculate execution time
	duration := time.Since(startTime).Seconds()

	// Determine exit code
	exitCode := 0
	success := true
	if err != nil {
		success = false
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Split stdout and stderr (combined output goes to stdout for logging)
	stdout := outputStr
	stderr := ""
	if exitCode != 0 && err != nil {
		stderr = err.Error()
	}

	// Track command execution for verbose reporting
	ce.trackCommand(command, exitCode, stdout, stderr, duration, success)

	// Log command result
	logErr := ce.logger.LogCommandResult(cmdID, exitCode, duration, stdout, stderr)
	if logErr != nil {
		// Don't fail the command execution due to logging errors, just note it
		fmt.Printf("Warning: Failed to log command result: %v\n", logErr)
	}

	return outputStr, err
}

// ExecuteKubectlCommand executes a kubectl command with proper context
func (ce *CommandExecutor) ExecuteKubectlCommand(ctx context.Context, args ...string) (string, error) {
	// Build kubectl command with namespace
	kubectlArgs := []string{"kubectl"}
	kubectlArgs = append(kubectlArgs, args...)

	// Add namespace if not already specified and it's needed
	hasNamespace := false
	for i, arg := range kubectlArgs {
		if arg == "-n" || arg == "--namespace" {
			hasNamespace = true
			break
		}
		if strings.HasPrefix(arg, "--namespace=") {
			hasNamespace = true
			break
		}
		// Check if this is a command that doesn't use namespaces
		if i == 1 && (arg == "get" || arg == "delete" || arg == "apply" || arg == "describe") {
			// Look ahead to see if the resource type requires namespace
			if i+1 < len(kubectlArgs) {
				resourceType := kubectlArgs[i+1]
				if resourceType == "nodes" || resourceType == "namespaces" || resourceType == "clusterroles" {
					hasNamespace = true // These don't use namespaces, so skip adding it
					break
				}
			}
		}
	}

	if !hasNamespace && ce.namespace != "" {
		kubectlArgs = append(kubectlArgs, "-n", ce.namespace)
	}

	command := strings.Join(kubectlArgs, " ")
	return ce.ExecuteCommand(ctx, command, "")
}

// ExecutePodCommand executes a command inside a pod
func (ce *CommandExecutor) ExecutePodCommand(ctx context.Context, podName, containerName string, command []string) (string, error) {
	// Build kubectl exec command
	kubectlArgs := []string{"exec", "-n", ce.namespace, podName}
	if containerName != "" {
		kubectlArgs = append(kubectlArgs, "-c", containerName)
	}
	kubectlArgs = append(kubectlArgs, "--")
	kubectlArgs = append(kubectlArgs, command...)

	return ce.ExecuteKubectlCommand(ctx, kubectlArgs...)
}

// ExecutePingFromPod executes a ping command from within a pod
func (ce *CommandExecutor) ExecutePingFromPod(ctx context.Context, podName, targetIP string) (string, error) {
	return ce.ExecutePodCommand(ctx, podName, "netshoot", []string{"ping", "-c", "3", "-W", "3", "-i", "1", targetIP})
}

// ExecuteCurlFromPod executes a curl command from within a pod
func (ce *CommandExecutor) ExecuteCurlFromPod(ctx context.Context, podName, target string) (string, error) {
	return ce.ExecutePodCommand(ctx, podName, "netshoot", []string{"curl", "-s", "--connect-timeout", "3", "--max-time", "5", "-o", "/dev/null", "-w", "%{http_code}", fmt.Sprintf("http://%s", target)})
}

// ExecuteNslookupFromPod executes a DNS lookup from within a pod
func (ce *CommandExecutor) ExecuteNslookupFromPod(ctx context.Context, podName, hostname string) (string, error) {
	return ce.ExecutePodCommand(ctx, podName, "netshoot", []string{"nslookup", hostname})
}

// ApplyPolicyFile applies a Kubernetes policy file
func (ce *CommandExecutor) ApplyPolicyFile(ctx context.Context, policyPath string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "apply", "-f", policyPath)
}

// DeletePolicyFile deletes a Kubernetes policy file
func (ce *CommandExecutor) DeletePolicyFile(ctx context.Context, policyPath string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "delete", "-f", policyPath, "--ignore-not-found=true")
}

// GetPods gets pods in the current namespace
func (ce *CommandExecutor) GetPods(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "pods", "-o", "wide")
}

// GetPodsByLabel gets pods by label selector
func (ce *CommandExecutor) GetPodsByLabel(ctx context.Context, labelSelector string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "pods", "-l", labelSelector, "-o", "wide")
}

// DeletePod deletes a specific pod
func (ce *CommandExecutor) DeletePod(ctx context.Context, podName string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "delete", "pod", podName, "--ignore-not-found=true")
}

// DeleteAllPods deletes all pods with a specific label
func (ce *CommandExecutor) DeleteAllPodsWithLabel(ctx context.Context, labelSelector string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "delete", "pods", "-l", labelSelector, "--ignore-not-found=true")
}

// GetNetworkPolicies gets all network policies in the namespace
func (ce *CommandExecutor) GetNetworkPolicies(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "networkpolicies", "-o", "wide")
}

// DeleteAllNetworkPolicies deletes all network policies in the namespace
func (ce *CommandExecutor) DeleteAllNetworkPolicies(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "delete", "networkpolicies", "--all", "--ignore-not-found=true")
}

// GetNodes gets all nodes in the cluster
func (ce *CommandExecutor) GetNodes(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "nodes", "-o", "wide")
}

// DescribePod describes a specific pod
func (ce *CommandExecutor) DescribePod(ctx context.Context, podName string) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "describe", "pod", podName)
}

// GetCiliumConfig gets the Cilium configuration
func (ce *CommandExecutor) GetCiliumConfig(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "configmaps", "-n", "kube-system", "cilium-config", "-o", "yaml")
}

// GetCiliumPods gets Cilium pods status
func (ce *CommandExecutor) GetCiliumPods(ctx context.Context) (string, error) {
	return ce.ExecuteKubectlCommand(ctx, "get", "pods", "-n", "kube-system", "-l", "k8s-app=cilium", "-o", "wide")
}

// WaitForPodReady waits for a pod to be ready
func (ce *CommandExecutor) WaitForPodReady(ctx context.Context, podName string, timeout time.Duration) (string, error) {
	timeoutStr := fmt.Sprintf("%.0fs", timeout.Seconds())
	return ce.ExecuteKubectlCommand(ctx, "wait", "--for=condition=ready", "pod", podName, fmt.Sprintf("--timeout=%s", timeoutStr))
}

// parseKubectlCommand parses a command string and extracts kubectl information
func parseKubectlCommand(command string) KubectlCommandInfo {
	parts := strings.Fields(command)
	info := KubectlCommandInfo{
		IsKubectl: false,
		Args:      parts,
	}

	if len(parts) == 0 {
		return info
	}

	// Check if this is a kubectl command (could be direct kubectl or within a shell command)
	kubectlIndex := -1
	for i, part := range parts {
		if strings.Contains(part, "kubectl") {
			kubectlIndex = i
			break
		}
	}

	if kubectlIndex == -1 {
		return info
	}

	info.IsKubectl = true

	// Parse kubectl arguments
	kubectlArgs := parts[kubectlIndex:]
	if len(kubectlArgs) > 1 {
		info.Operation = kubectlArgs[1]
	}
	if len(kubectlArgs) > 2 {
		info.Resource = kubectlArgs[2]
	}

	// Extract resource name if available
	for i := 3; i < len(kubectlArgs); i++ {
		arg := kubectlArgs[i]
		// Skip flags and their values
		if strings.HasPrefix(arg, "-") {
			if arg == "-n" || arg == "--namespace" || arg == "-l" || arg == "--selector" ||
				arg == "-o" || arg == "--output" || arg == "-f" || arg == "--filename" {
				// Skip the flag value as well
				i++
			}
			continue
		}
		// This should be the resource name
		if info.ResourceName == "" {
			info.ResourceName = arg
		}
	}

	// Extract namespace
	for i := 0; i < len(kubectlArgs)-1; i++ {
		if kubectlArgs[i] == "-n" || kubectlArgs[i] == "--namespace" {
			if i+1 < len(kubectlArgs) {
				info.Namespace = kubectlArgs[i+1]
			}
			break
		}
		if strings.HasPrefix(kubectlArgs[i], "--namespace=") {
			info.Namespace = strings.TrimPrefix(kubectlArgs[i], "--namespace=")
			break
		}
	}

	return info
}

// CreateNetshootPodYAML creates a netshoot pod using inline YAML
func (ce *CommandExecutor) CreateNetshootPodYAML(ctx context.Context, podName, nodeName string) (string, error) {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: netshoot-test
spec:
  nodeName: %s
  containers:
  - name: netshoot
    image: nicolaka/netshoot
    command: ["sleep", "3600"]
  restartPolicy: Never`, podName, ce.namespace, nodeName)

	// Use kubectl apply with stdin
	command := fmt.Sprintf("echo '%s' | kubectl apply -f -", yaml)
	return ce.ExecuteCommand(ctx, command, "")
}
