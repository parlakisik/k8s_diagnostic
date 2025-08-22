package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"k8s-diagnostic/internal/diagnostic/core"

	"github.com/spf13/cobra"
)

// TestExecutionRequest represents a request to execute a CLI test
type TestExecutionRequest struct {
	TestID     string   `json:"testId"`
	CLICommand string   `json:"cliCommand"`
	Args       []string `json:"args"`
}

// TestExecutionResponse represents the response from test execution
type TestExecutionResponse struct {
	Success  bool        `json:"success"`
	TestID   string      `json:"testId"`
	Message  string      `json:"message"`
	Duration *float64    `json:"duration,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for Kubernetes container communication",
	Long: `Start an HTTP server that handles test execution requests from the UI container.
This command is used in Kubernetes deployments where the CLI container needs to
receive HTTP requests from the UI container to execute diagnostic tests.

The server runs on port 8080 by default and provides endpoints for:
- Test execution: POST /api/execute-test
- Health check: GET /api/health
- Status check: GET /api/status`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		if err := runHTTPServer(port); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 8080, "Port to run HTTP server on")
}

// runHTTPServer starts the HTTP server for container communication
func runHTTPServer(port int) error {
	// Perform startup validation before starting server
	log.Printf("🔍 Performing startup validation...")
	if err := performStartupChecks(port); err != nil {
		return fmt.Errorf("startup validation failed: %w", err)
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/api/health", handleHealth)

	// Status endpoint
	mux.HandleFunc("/api/status", handleStatus)

	// Test execution endpoint
	mux.HandleFunc("/api/execute-test", handleTestExecution)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsMiddleware(loggingMiddleware(mux)),
	}

	// Graceful shutdown handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("🚀 CLI HTTP server starting on port %d", port)
	log.Printf("📝 Endpoints available:")
	log.Printf("   GET  /api/health - Health check")
	log.Printf("   GET  /api/status - Server status")
	log.Printf("   POST /api/execute-test - Execute diagnostic tests")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server failed to start: %w", err)
	}

	log.Println("HTTP server stopped")
	return nil
}

// performStartupChecks validates the environment before starting the HTTP server
func performStartupChecks(port int) error {
	log.Printf("✅ Starting CLI container startup validation...")

	// Validate environment variables
	if err := validateEnvironment(); err != nil {
		log.Printf("❌ Environment validation failed: %v", err)
		return err
	}

	// Test port availability
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("❌ Port %d is not available: %v", err)
		return fmt.Errorf("port %d not available: %w", port, err)
	}
	listener.Close()
	log.Printf("✅ Port %d is available", port)

	// Test shared volume access
	sharedPath := os.Getenv("SHARED_VOLUME_PATH")
	if sharedPath == "" {
		sharedPath = "/app/shared/repository/test_results"
	}

	if stat, err := os.Stat(sharedPath); err != nil || !stat.IsDir() {
		log.Printf("⚠️ Shared volume not accessible at %s: %v", sharedPath, err)
		log.Printf("⚠️ This may cause test result storage issues")
	} else {
		log.Printf("✅ Shared volume accessible at %s", sharedPath)

		// Test write access
		testFile := filepath.Join(sharedPath, "startup-test.tmp")
		if err := os.WriteFile(testFile, []byte("startup test"), 0644); err != nil {
			log.Printf("⚠️ Cannot write to shared volume: %v", err)
		} else {
			os.Remove(testFile)
			log.Printf("✅ Shared volume write access confirmed")
		}
	}

	log.Printf("✅ Startup validation completed successfully")
	return nil
}

// validateEnvironment checks required environment variables and configuration
func validateEnvironment() error {
	log.Printf("🔧 Validating container environment...")

	// Check Kubernetes mode
	kubernetesMode := os.Getenv("KUBERNETES_MODE")
	if kubernetesMode == "true" {
		log.Printf("✅ Running in Kubernetes mode")
	} else {
		log.Printf("ℹ️ Running in local development mode")
	}

	// Check CLI port configuration
	cliPort := os.Getenv("CLI_PORT")
	if cliPort == "" {
		log.Printf("ℹ️ CLI_PORT not set, using default")
	} else {
		log.Printf("✅ CLI_PORT configured: %s", cliPort)
	}

	// Check shared volume path
	sharedPath := os.Getenv("SHARED_VOLUME_PATH")
	if sharedPath == "" {
		log.Printf("ℹ️ SHARED_VOLUME_PATH not set, using default: /app/shared/repository/test_results")
	} else {
		log.Printf("✅ SHARED_VOLUME_PATH configured: %s", sharedPath)
	}

	// Check if kubectl is available (for test execution)
	if _, err := exec.LookPath("kubectl"); err != nil {
		log.Printf("⚠️ kubectl not found in PATH: %v", err)
		log.Printf("⚠️ This may affect Kubernetes diagnostic tests")
	} else {
		log.Printf("✅ kubectl is available")
	}

	return nil
}

// handleHealth returns server health status
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "k8s-diagnostic-cli",
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus returns detailed server status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we can access shared volume
	sharedPath := os.Getenv("SHARED_VOLUME_PATH")
	if sharedPath == "" {
		sharedPath = "/app/shared/repository/test_results"
	}

	volumeAccessible := false
	if stat, err := os.Stat(sharedPath); err == nil && stat.IsDir() {
		volumeAccessible = true
	}

	response := map[string]interface{}{
		"status":            "running",
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
		"service":           "k8s-diagnostic-cli",
		"kubernetes_mode":   os.Getenv("KUBERNETES_MODE") == "true",
		"shared_volume":     sharedPath,
		"volume_accessible": volumeAccessible,
		"environment": map[string]string{
			"KUBERNETES_MODE":    os.Getenv("KUBERNETES_MODE"),
			"CLI_PORT":           os.Getenv("CLI_PORT"),
			"SHARED_VOLUME_PATH": os.Getenv("SHARED_VOLUME_PATH"),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Global request counter for tracking HTTP requests from UI
var httpRequestCounter int64 = 0

// handleTestExecution processes test execution requests from UI container
func handleTestExecution(w http.ResponseWriter, r *http.Request) {
	// Increment request counter immediately to track all incoming requests
	httpRequestCounter++
	requestID := httpRequestCounter

	log.Printf("🚨 [CLI HTTP] === HTTP REQUEST #%d RECEIVED === ", requestID)
	log.Printf("🎯 [CLI HTTP] Request #%d: Received test execution request from %s", requestID, r.RemoteAddr)
	log.Printf("🕐 [CLI HTTP] Request #%d: Timestamp: %s", requestID, time.Now().UTC().Format(time.RFC3339))
	log.Printf("🌐 [CLI HTTP] Request #%d: Method: %s, URL: %s", requestID, r.Method, r.URL.String())
	log.Printf("🔍 [CLI HTTP] Request #%d: User-Agent: %s", requestID, r.Header.Get("User-Agent"))
	log.Printf("📏 [CLI HTTP] Request #%d: Content-Length: %s", requestID, r.Header.Get("Content-Length"))

	if r.Method != http.MethodPost {
		log.Printf("❌ [CLI HTTP] Request #%d: Method not allowed: %s", requestID, r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enhanced request headers logging
	log.Printf("🔍 [CLI HTTP] Request #%d: Complete request headers:", requestID)
	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("    %s: %s", name, value)
		}
	}

	// Parse request body with enhanced error handling
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ [CLI HTTP] Request #%d: Failed to read request body: %v", requestID, err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("📄 [CLI HTTP] Request #%d: Request body received (%d bytes)", requestID, len(body))
	if len(body) > 0 {
		// Log the first 500 characters of the body to avoid overwhelming logs
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		log.Printf("📝 [CLI HTTP] Request #%d: Body content: %s", requestID, bodyPreview)
	} else {
		log.Printf("⚠️ [CLI HTTP] Request #%d: Empty request body!", requestID)
	}

	var req TestExecutionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("❌ [CLI HTTP] Request #%d: Invalid JSON format: %v", requestID, err)
		log.Printf("📄 [CLI HTTP] Request #%d: Raw body that failed to parse: %s", requestID, string(body))
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("✅ [CLI HTTP] Request #%d: Successfully parsed JSON request:", requestID)
	log.Printf("    TestID: '%s'", req.TestID)
	log.Printf("    CLICommand: '%s'", req.CLICommand)
	log.Printf("    Args: %v", req.Args)

	// Validate request with detailed logging
	if req.TestID == "" {
		log.Printf("❌ [CLI HTTP] Request #%d: Validation failed - TestID is required but empty", requestID)
		http.Error(w, "testId is required", http.StatusBadRequest)
		return
	}

	if req.CLICommand == "" {
		log.Printf("❌ [CLI HTTP] Request #%d: Validation failed - CLICommand is required but empty", requestID)
		http.Error(w, "cliCommand is required", http.StatusBadRequest)
		return
	}

	log.Printf("✅ [CLI HTTP] Request #%d: Validation passed - proceeding with test execution", requestID)
	log.Printf("📨 [CLI HTTP] Request #%d: Processing test execution: TestID='%s', Command='%s'", requestID, req.TestID, req.CLICommand)

	// Execute the test with enhanced logging and request tracking
	success, message, err := executeTestWithRequestTracking(requestID, req.TestID, req.CLICommand, req.Args)

	response := TestExecutionResponse{
		Success: success,
		TestID:  req.TestID,
		Message: message,
	}

	if err != nil {
		log.Printf("❌ [CLI HTTP] Request #%d: Test execution failed with error: %v", requestID, err)
		response.Message = fmt.Sprintf("Test execution failed: %v", err)
	} else {
		log.Printf("✅ [CLI HTTP] Request #%d: Test execution completed successfully", requestID)
		log.Printf("    Success: %t", success)
		log.Printf("    Message: '%s'", message)
	}

	// Enhanced response logging
	log.Printf("📤 [CLI HTTP] Request #%d: Preparing response:", requestID)
	log.Printf("    Success: %t", response.Success)
	log.Printf("    TestID: '%s'", response.TestID)
	log.Printf("    Message: '%s'", response.Message)
	log.Printf("    Message Length: %d characters", len(response.Message))

	w.Header().Set("Content-Type", "application/json")
	if !success {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("⚠️ [CLI HTTP] Request #%d: Sending HTTP 500 due to test failure", requestID)
	} else {
		log.Printf("✅ [CLI HTTP] Request #%d: Sending HTTP 200 for successful test", requestID)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("❌ [CLI HTTP] Request #%d: Failed to encode JSON response: %v", requestID, err)
	} else {
		log.Printf("📡 [CLI HTTP] Request #%d: Response sent successfully to UI container", requestID)
	}

	log.Printf("🏁 [CLI HTTP] === REQUEST #%d COMPLETED === ", requestID)
}

// executeTestWithRequestTracking is a wrapper around executeTest with request tracking
func executeTestWithRequestTracking(requestID int64, testID, cliCommand string, args []string) (bool, string, error) {
	log.Printf("🔄 [CLI HTTP] Request #%d: Starting executeTest for TestID='%s'", requestID, testID)

	success, message, err := executeTest(testID, cliCommand, args)

	log.Printf("📊 [CLI HTTP] Request #%d: executeTest completed:", requestID)
	log.Printf("    Success: %t", success)
	log.Printf("    Message: '%s'", message)
	log.Printf("    Error: %v", err)

	return success, message, err
}

// executeTest runs the CLI test command with proper cleanup synchronization and progress reporting
func executeTest(testID, cliCommand string, args []string) (bool, string, error) {
	log.Printf("🚀 [CLI EXEC] Starting test execution for TestID: %s", testID)
	log.Printf("📋 [CLI EXEC] Input parameters:")
	log.Printf("  TestID: %s", testID)
	log.Printf("  CLICommand: %s", cliCommand)
	log.Printf("  Args: %v", args)

	// Send test_start event immediately
	sendTestEvent(testID, "test_start", fmt.Sprintf("Starting test: %s", testID), true)

	// Use optimized Kubernetes production cleanup instead of comprehensive deepclean
	log.Printf("🧹 [CLI EXEC] Starting optimized Kubernetes production cleanup...")

	// First wait for any ongoing cleanup operations with progress reporting
	if err := waitForCleanupCompletionWithProgress(testID); err != nil {
		log.Printf("⚠️ [CLI EXEC] Cleanup completion check had issues: %v", err)
		// Continue anyway as the function allows test to proceed
	}

	// Use lightweight cleanup optimized for individual test execution
	if err := executeOptimizedKubernetesCleanup(testID); err != nil {
		log.Printf("❌ [CLI EXEC] Optimized cleanup execution failed: %v", err)
		sendProgressUpdate(testID, "cleanup_failed", fmt.Sprintf("❌ Optimized cleanup execution failed: %v", err))
		return false, fmt.Sprintf("Failed to execute optimized cleanup: %v", err), err
	}
	log.Printf("✅ [CLI EXEC] Optimized cleanup completed - proceeding with test execution")
	sendTestEvent(testID, "test_progress", "🧪 Executing diagnostic test...", false)

	// Parse the CLI command to extract the actual command and arguments
	commandParts := strings.Fields(cliCommand)
	if len(commandParts) == 0 {
		log.Printf("❌ [CLI EXEC] Empty command provided")
		return false, "Empty command", fmt.Errorf("empty command")
	}

	log.Printf("🔍 [CLI EXEC] Parsed command parts: %v", commandParts)

	// For Kubernetes deployment, we're already running inside the CLI container
	// so we can execute the test command directly
	var cmdArgs []string

	// If the command starts with ./k8s-diagnostic, we need to use the test subcommand
	if strings.Contains(commandParts[0], "k8s-diagnostic") {
		// Extract arguments after the binary name
		if len(commandParts) > 1 {
			cmdArgs = commandParts[1:] // Skip the binary name
		}
		// Add any additional args passed in the request
		cmdArgs = append(cmdArgs, args...)
		log.Printf("✅ [CLI EXEC] Using k8s-diagnostic binary, args: %v", cmdArgs)
	} else {
		// Use the command as-is
		cmdArgs = commandParts[1:]
		cmdArgs = append(cmdArgs, args...)
		log.Printf("✅ [CLI EXEC] Using command as-is, args: %v", cmdArgs)
	}

	// Set up environment for test execution
	env := os.Environ()
	log.Printf("🌍 [CLI EXEC] Base environment variables count: %d", len(env))

	// Ensure shared volume path is set
	sharedPath := os.Getenv("SHARED_VOLUME_PATH")
	if sharedPath == "" {
		sharedPath = "/app/shared/repository/test_results"
		log.Printf("⚠️ [CLI EXEC] SHARED_VOLUME_PATH not set, using default: %s", sharedPath)
	} else {
		log.Printf("✅ [CLI EXEC] Using SHARED_VOLUME_PATH: %s", sharedPath)
	}

	// Add test ID to environment
	env = append(env, fmt.Sprintf("BATCH_TEST_ID=%s", testID))
	env = append(env, fmt.Sprintf("SHARED_VOLUME_PATH=%s", sharedPath))
	log.Printf("➕ [CLI EXEC] Added environment variables:")
	log.Printf("  BATCH_TEST_ID=%s", testID)
	log.Printf("  SHARED_VOLUME_PATH=%s", sharedPath)

	// Determine the correct binary path
	var binaryPath string

	// Try multiple binary path options for Kubernetes deployment
	binaryOptions := []string{
		"/app/k8s-diagnostic",
		"/usr/local/bin/k8s-diagnostic",
		"/app/k8s_diagnostic",
		os.Args[0], // Current process path as fallback
	}

	for _, path := range binaryOptions {
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			binaryPath = path
			log.Printf("✅ [CLI EXEC] Found binary at: %s", binaryPath)
			break
		} else {
			log.Printf("⚠️ [CLI EXEC] Binary not found at: %s (error: %v)", path, err)
		}
	}

	if binaryPath == "" {
		log.Printf("❌ [CLI EXEC] No valid binary found, trying fallback")
		binaryPath = os.Args[0]
		log.Printf("🔄 [CLI EXEC] Using current process as fallback: %s", binaryPath)
	}

	// Build the final command - avoid duplicating "test" subcommand
	var finalCmd []string
	if len(cmdArgs) > 0 && cmdArgs[0] == "test" {
		// Command already includes "test" subcommand
		finalCmd = cmdArgs
		log.Printf("✅ [CLI EXEC] Command already includes 'test' subcommand")
	} else {
		// Need to prepend "test" subcommand
		finalCmd = append([]string{"test"}, cmdArgs...)
		log.Printf("✅ [CLI EXEC] Prepended 'test' subcommand")
	}
	log.Printf("🔧 [CLI EXEC] Final command to execute:")
	log.Printf("  Binary: %s", binaryPath)
	log.Printf("  Args: %v", finalCmd)

	// Create the command
	cmd := exec.Command(binaryPath, finalCmd...)
	cmd.Env = env

	// Set appropriate working directory
	var workingDir string
	if stat, err := os.Stat(sharedPath); err == nil && stat.IsDir() {
		workingDir = filepath.Dir(sharedPath) // Go up one level to repository root
		log.Printf("✅ [CLI EXEC] Using working directory: %s", workingDir)
	} else {
		workingDir = "/app" // Default to /app in container
		log.Printf("⚠️ [CLI EXEC] Shared volume not accessible, using default working directory: %s", workingDir)
	}
	cmd.Dir = workingDir

	// Log complete command execution details
	log.Printf("📋 [CLI EXEC] Complete command execution setup:")
	log.Printf("  Command: %s %s", binaryPath, strings.Join(finalCmd, " "))
	log.Printf("  Working Directory: %s", workingDir)
	log.Printf("  Environment Variables: %d total", len(env))

	// Execute the command with timeout
	log.Printf("⏳ [CLI EXEC] Starting command execution...")
	startTime := time.Now()

	// Capture both stdout and stderr separately for better debugging
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	combinedOutput := stdoutStr + stderrStr

	log.Printf("⏱️ [CLI EXEC] Command execution completed in %v", duration)
	log.Printf("📊 [CLI EXEC] Execution results:")
	log.Printf("  Exit Code: %v", err)
	log.Printf("  Stdout Length: %d bytes", len(stdoutStr))
	log.Printf("  Stderr Length: %d bytes", len(stderrStr))

	if len(stdoutStr) > 0 {
		log.Printf("📄 [CLI EXEC] STDOUT:")
		// Log stdout in chunks to avoid overwhelming logs
		if len(stdoutStr) > 2000 {
			log.Printf("%s... (truncated, full length: %d bytes)", stdoutStr[:2000], len(stdoutStr))
		} else {
			log.Printf("%s", stdoutStr)
		}
	}

	if len(stderrStr) > 0 {
		log.Printf("📄 [CLI EXEC] STDERR:")
		// Log stderr in chunks to avoid overwhelming logs
		if len(stderrStr) > 2000 {
			log.Printf("%s... (truncated, full length: %d bytes)", stderrStr[:2000], len(stderrStr))
		} else {
			log.Printf("%s", stderrStr)
		}
	}

	if err != nil {
		log.Printf("❌ [CLI EXEC] Command failed with error: %v", err)
		log.Printf("🔍 [CLI EXEC] Error analysis:")
		log.Printf("  Error Type: %T", err)
		log.Printf("  Error String: %s", err.Error())

		// Check for common error patterns
		if strings.Contains(err.Error(), "no such file or directory") {
			log.Printf("💡 [CLI EXEC] Suggestion: Binary path issue - check if %s exists and is executable", binaryPath)
		}
		if strings.Contains(err.Error(), "permission denied") {
			log.Printf("💡 [CLI EXEC] Suggestion: Permission issue - check file permissions for %s", binaryPath)
		}

		return false, fmt.Sprintf("Command execution failed: %v\nStdout: %s\nStderr: %s", err, stdoutStr, stderrStr), err
	}

	log.Printf("✅ [CLI EXEC] Test execution completed successfully for TestID=%s", testID)

	// Parse actual test results from JSON file instead of just checking command execution
	success, resultMessage := parseTestResults(workingDir, combinedOutput, testID)

	if success {
		log.Printf("🎯 [CLI EXEC] Test results: SUCCESS")
	} else {
		log.Printf("❌ [CLI EXEC] Test results: FAILED")
	}

	return success, resultMessage, nil
}

// waitForCleanupCompletion waits for any ongoing cleanup operations to complete before starting test execution
func waitForCleanupCompletion(testID string) error {
	log.Printf("🧹 [CLI CLEANUP] Starting cleanup completion check for TestID: %s", testID)

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		log.Printf("⚠️ [CLI CLEANUP] kubectl not available, skipping cleanup verification: %v", err)
		return nil
	}

	// Maximum wait time for cleanup operations
	maxWaitTime := 60 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	// Check for ongoing cleanup processes and stuck resources
	for attempt := 1; time.Since(startTime) < maxWaitTime; attempt++ {
		log.Printf("🔍 [CLI CLEANUP] Cleanup verification attempt %d...", attempt)

		allClear := true
		issues := []string{}

		// Check for terminating pods (these indicate ongoing cleanup)
		if terminatingPods := checkTerminatingPods(); terminatingPods > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d terminating pods", terminatingPods))
		}

		// Check for stuck deployments
		if stuckDeployments := checkStuckDeployments(); stuckDeployments > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d stuck deployments", stuckDeployments))
		}

		// Check for pending policy deletions
		if pendingPolicies := checkPendingPolicyDeletions(); pendingPolicies > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d pending policy deletions", pendingPolicies))
		}

		if allClear {
			log.Printf("✅ [CLI CLEANUP] All cleanup operations completed after %v (attempt %d)",
				time.Since(startTime), attempt)
			return nil
		}

		log.Printf("⏳ [CLI CLEANUP] Waiting for cleanup to complete... Issues: %v", issues)
		log.Printf("    Elapsed: %v, Max wait: %v", time.Since(startTime), maxWaitTime)

		// Wait before next check
		time.Sleep(checkInterval)
	}

	// Timeout reached - log warning but allow test to proceed
	log.Printf("⚠️ [CLI CLEANUP] Cleanup verification timeout reached after %v", maxWaitTime)
	log.Printf("⚠️ [CLI CLEANUP] Proceeding with test execution despite potential ongoing cleanup")

	// Don't return error - allow test to proceed even if cleanup is taking longer
	return nil
}

// checkTerminatingPods returns the number of pods in Terminating state
func checkTerminatingPods() int {
	cmd := exec.Command("kubectl", "get", "pods", "--all-namespaces",
		"--field-selector=status.phase!=Running,status.phase!=Succeeded",
		"-o", "jsonpath={.items[*].status.phase}")

	output, err := cmd.Output()
	if err != nil {
		return 0 // Assume no issues if we can't check
	}

	terminatingCount := strings.Count(string(output), "Terminating")
	if terminatingCount > 0 {
		log.Printf("🔍 [CLI CLEANUP] Found %d terminating pods", terminatingCount)
	}

	return terminatingCount
}

// checkStuckDeployments returns the number of deployments that might be stuck
func checkStuckDeployments() int {
	cmd := exec.Command("kubectl", "get", "deployments", "--all-namespaces",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{' '}{.status.replicas}{' '}{.status.readyReplicas}{'\\n'}{end}")

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	stuckCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 3 {
			// Check if deployment has replicas but no ready replicas (potentially stuck)
			if parts[1] != "0" && parts[2] == "0" {
				stuckCount++
			}
		}
	}

	if stuckCount > 0 {
		log.Printf("🔍 [CLI CLEANUP] Found %d potentially stuck deployments", stuckCount)
	}

	return stuckCount
}

// checkPendingPolicyDeletions returns the number of network policies pending deletion
func checkPendingPolicyDeletions() int {
	// Check for Cilium network policies with deletion timestamps
	cmd := exec.Command("kubectl", "get", "ciliumnetworkpolicies", "--all-namespaces",
		"-o", "jsonpath={.items[*].metadata.deletionTimestamp}")

	output, err := cmd.Output()
	if err == nil {
		pendingCount := len(strings.Fields(string(output)))
		if pendingCount > 0 {
			log.Printf("🔍 [CLI CLEANUP] Found %d Cilium network policies pending deletion", pendingCount)
			return pendingCount
		}
	}

	// Check for cluster-wide policies
	cmd = exec.Command("kubectl", "get", "ciliumclusterwidenetworkpolicies",
		"-o", "jsonpath={.items[*].metadata.deletionTimestamp}")

	output, err = cmd.Output()
	if err == nil {
		pendingCount := len(strings.Fields(string(output)))
		if pendingCount > 0 {
			log.Printf("🔍 [CLI CLEANUP] Found %d Cilium cluster-wide policies pending deletion", pendingCount)
			return pendingCount
		}
	}

	return 0
}

// parseTestResults analyzes the test output and JSON file to determine actual test success/failure
func parseTestResults(workingDir, output, testID string) (bool, string) {
	log.Printf("🔍 [CLI EXEC] Parsing test results for %s", testID)

	// Look for JSON results file in the output
	if strings.Contains(output, "JSON summary written to:") {
		// Extract JSON file path from output
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "JSON summary written to:") {
				jsonPath := strings.TrimSpace(strings.Split(line, "JSON summary written to:")[1])

				log.Printf("📄 [CLI EXEC] Found JSON results file path in output: %s", jsonPath)
				log.Printf("🔍 [CLI EXEC] Working directory: %s", workingDir)

				// Let parseJSONResults handle path construction
				// This prevents path duplication by passing the original path
				// parseJSONResults will handle absolute vs relative path logic correctly
				if success, message := parseJSONResults(jsonPath, testID); message != "" {
					return success, message
				}
			}
		}
	}

	// Fallback: Parse output text for test results
	log.Printf("📄 [CLI EXEC] No JSON file found, parsing output text")

	// Look for test summary in output
	if strings.Contains(output, "Overall Result:") {
		if strings.Contains(output, "failed") || strings.Contains(output, "FAILED") {
			log.Printf("❌ [CLI EXEC] Found failure indicator in output")
			// Extract failure details
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if strings.Contains(line, "Overall Result:") {
					return false, fmt.Sprintf("Test failed: %s", strings.TrimSpace(line))
				}
			}
			return false, "Test failed based on output analysis"
		}

		if strings.Contains(output, "passed") || strings.Contains(output, "PASSED") {
			log.Printf("✅ [CLI EXEC] Found success indicator in output")
			return true, "Test passed based on output analysis"
		}
	}

	// Check for specific test result patterns
	if strings.Contains(output, "❌") && strings.Contains(output, "FAILED") {
		log.Printf("❌ [CLI EXEC] Found failure symbols in output")
		return false, "Test failed - found failure indicators in output"
	}

	if strings.Contains(output, "✅") && (strings.Contains(output, "PASSED") || strings.Contains(output, "passed")) {
		log.Printf("✅ [CLI EXEC] Found success symbols in output")
		return true, "Test passed - found success indicators in output"
	}

	// Default fallback - if no clear indicators, assume failure for safety
	log.Printf("⚠️ [CLI EXEC] No clear success/failure indicators found, defaulting to failure")
	return false, "Unable to determine test result from output"
}

// parseJSONResults reads and parses the JSON results file
func parseJSONResults(jsonPath, testID string) (bool, string) {
	log.Printf("📖 [CLI EXEC] Reading JSON results from: %s", jsonPath)

	// 🛡️ CRITICAL FIX: Handle absolute vs relative paths correctly
	var finalPath string
	if filepath.IsAbs(jsonPath) {
		// Use absolute path directly
		finalPath = jsonPath
		log.Printf("✅ [CLI EXEC] Using absolute path: %s", finalPath)
	} else {
		// For relative paths, join with current working directory
		if workingDir, err := os.Getwd(); err == nil {
			finalPath = filepath.Join(workingDir, jsonPath)
			log.Printf("✅ [CLI EXEC] Constructed relative path: %s", finalPath)
		} else {
			finalPath = jsonPath
			log.Printf("⚠️ [CLI EXEC] Cannot get working directory, using path as-is: %s", finalPath)
		}
	}

	log.Printf("🎯 [CLI EXEC] Final JSON path to read: %s", finalPath)

	// Check if file exists and wait a bit for it to be written
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(finalPath); err == nil {
			log.Printf("✅ [CLI EXEC] JSON file found on attempt %d", i+1)
			break
		}
		log.Printf("⏳ [CLI EXEC] Waiting for JSON file to be written... (%d/5)", i+1)
		time.Sleep(500 * time.Millisecond)
	}

	data, err := os.ReadFile(finalPath)
	if err != nil {
		log.Printf("❌ [CLI EXEC] Failed to read JSON file: %v", err)

		// 🛡️ ENHANCED: Try alternative path construction as fallback
		if !filepath.IsAbs(jsonPath) {
			// Try using shared volume path directly
			sharedPath := os.Getenv("SHARED_VOLUME_PATH")
			if sharedPath == "" {
				sharedPath = "/app/shared/repository/test_results"
			}

			// Extract just the filename and construct path directly
			fileName := filepath.Base(jsonPath)
			fallbackPath := filepath.Join(sharedPath, fileName)
			log.Printf("🔄 [CLI EXEC] Trying fallback path: %s", fallbackPath)

			if data, err = os.ReadFile(fallbackPath); err == nil {
				log.Printf("✅ [CLI EXEC] Successfully read from fallback path")
			} else {
				log.Printf("❌ [CLI EXEC] Fallback path also failed: %v", err)
				return false, ""
			}
		} else {
			return false, ""
		}
	}

	// Parse JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("❌ [CLI EXEC] Failed to parse JSON: %v", err)
		return false, ""
	}

	log.Printf("📊 [CLI EXEC] Parsed JSON results: %d keys found", len(result))

	// Check overall success
	if overallSuccess, ok := result["overall_success"].(bool); ok {
		log.Printf("🎯 [CLI EXEC] Found overall_success: %t", overallSuccess)

		// Get test summary
		passedTests := 0
		totalTests := 0

		if passed, ok := result["passed_tests"].(float64); ok {
			passedTests = int(passed)
		}
		if total, ok := result["total_tests"].(float64); ok {
			totalTests = int(total)
		}

		message := fmt.Sprintf("Test execution completed: %d/%d tests passed", passedTests, totalTests)

		if overallSuccess {
			log.Printf("✅ [CLI EXEC] Overall test result: SUCCESS (%s)", message)
		} else {
			log.Printf("❌ [CLI EXEC] Overall test result: FAILED (%s)", message)
		}

		return overallSuccess, message
	}

	// Fallback: check individual test results
	if tests, ok := result["tests"].([]interface{}); ok {
		log.Printf("🔍 [CLI EXEC] Checking individual test results (%d tests)", len(tests))

		allPassed := true
		passedCount := 0

		for _, test := range tests {
			if testMap, ok := test.(map[string]interface{}); ok {
				if success, exists := testMap["success"]; exists {
					if successBool, ok := success.(bool); ok {
						if successBool {
							passedCount++
						} else {
							allPassed = false
						}
					}
				}
			}
		}

		message := fmt.Sprintf("Individual test results: %d/%d tests passed", passedCount, len(tests))
		log.Printf("📊 [CLI EXEC] %s", message)

		return allPassed, message
	}

	log.Printf("⚠️ [CLI EXEC] No recognizable test results in JSON")
	return false, "No test results found in JSON file"
}

// sendProgressUpdate sends progress updates to the UI via SSE_EVENT format
func sendProgressUpdate(testID, phase, message string) {
	// 🛡️ CRITICAL FIX: Validate inputs to prevent nil formatting issues
	if testID == "" {
		log.Printf("⚠️ [CLI PROGRESS] Empty testID provided, skipping progress update")
		return
	}
	if phase == "" {
		phase = "unknown"
	}
	if message == "" {
		message = "Progress update"
	}

	log.Printf("📡 [CLI PROGRESS] TestID=%s, Phase=%s: %s", testID, phase, message)

	// Create SSE event for UI consumption with proper nil checking
	sseEvent := map[string]interface{}{
		"type":      "progress_update",
		"testName":  testID,
		"phase":     phase,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Output SSE_EVENT that UI can capture from stdout
	if eventJSON, err := json.Marshal(sseEvent); err == nil {
		fmt.Printf("SSE_EVENT:%s\n", string(eventJSON))
		fmt.Fprintf(os.Stdout, "SSE_EVENT:%s\n", string(eventJSON))

		// CRITICAL FIX: Forward SSE event to UI container's log-events API
		forwardEventToUI(testID, sseEvent)
	} else {
		log.Printf("❌ [CLI PROGRESS] Failed to marshal progress event: %v", err)
	}
}

// sendTestEvent sends test lifecycle events to the UI via SSE_EVENT format
func sendTestEvent(testID, eventType, message string, success bool) {
	// 🛡️ CRITICAL FIX: Validate inputs to prevent nil formatting issues
	if testID == "" {
		log.Printf("⚠️ [CLI TEST EVENT] Empty testID provided, skipping test event")
		return
	}
	if eventType == "" {
		eventType = "unknown"
	}
	if message == "" {
		message = "Test event"
	}

	log.Printf("📡 [CLI TEST EVENT] TestID=%s, Type=%s: %s", testID, eventType, message)

	// Create SSE event for UI consumption with proper nil checking
	sseEvent := map[string]interface{}{
		"type":      eventType,
		"testName":  testID,
		"message":   message,
		"success":   success,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Output SSE_EVENT that UI can capture from stdout
	if eventJSON, err := json.Marshal(sseEvent); err == nil {
		fmt.Printf("SSE_EVENT:%s\n", string(eventJSON))
		fmt.Fprintf(os.Stdout, "SSE_EVENT:%s\n", string(eventJSON))

		// CRITICAL FIX: Forward SSE event to UI container's log-events API
		forwardEventToUI(testID, sseEvent)
	} else {
		log.Printf("❌ [CLI TEST EVENT] Failed to marshal test event: %v", err)
	}
}

// forwardEventToUI forwards SSE events to UI container with robust retry logic and fallback mechanisms
func forwardEventToUI(testID string, sseEvent map[string]interface{}) {
	// Skip forwarding if not in Kubernetes mode
	if os.Getenv("KUBERNETES_MODE") != "true" {
		log.Printf("🔍 [SSE FORWARD] Skipping - not in Kubernetes mode")
		return
	}

	// 🚀 ENHANCED: Robust event forwarding with multiple reliability mechanisms
	success := false

	// Primary mechanism: HTTP forwarding with retry logic
	if forwardEventViaHTTP(testID, sseEvent) {
		success = true
	} else {
		// Fallback mechanism: Shared volume event logging
		log.Printf("🔄 [SSE FORWARD] HTTP forwarding failed, trying shared volume fallback...")
		if forwardEventViaSharedVolume(testID, sseEvent) {
			success = true
		}
	}

	if !success {
		log.Printf("❌ [SSE FORWARD] All forwarding mechanisms failed for event: %s", sseEvent["type"])
	}
}

// forwardEventViaHTTP forwards events via HTTP with robust retry logic
func forwardEventViaHTTP(testID string, sseEvent map[string]interface{}) bool {
	// Get UI container URL (localhost since containers share network)
	uiURL := "http://localhost:3000/api/log-events"
	log.Printf("🔍 [SSE HTTP] Forwarding to URL: %s", uiURL)

	// Prepare event data for UI
	eventData := map[string]interface{}{
		"testId":    testID,
		"type":      sseEvent["type"],
		"message":   sseEvent["message"],
		"timestamp": sseEvent["timestamp"],
		"phase":     sseEvent["phase"],   // May be nil for non-progress events
		"success":   sseEvent["success"], // May be nil for non-test events
		"testName":  sseEvent["testName"],
		"container": "cli", // Mark as coming from CLI container
		"line":      fmt.Sprintf("[SSE] %s", sseEvent["message"]),
	}

	log.Printf("🔍 [SSE HTTP] Event data prepared: %+v", eventData)

	// Convert to JSON
	jsonData, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("❌ [SSE HTTP] Failed to marshal event data: %v", err)
		return false
	}

	log.Printf("🔍 [SSE HTTP] JSON data (%d bytes): %s", len(jsonData), string(jsonData))

	// 🚀 ENHANCED: Retry logic with exponential backoff
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("🔄 [SSE HTTP] Attempt %d/%d", attempt, maxRetries)

		// Create HTTP request to UI's log-events API
		req, err := http.NewRequest("POST", uiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("❌ [SSE HTTP] Failed to create request on attempt %d: %v", attempt, err)
			if attempt == maxRetries {
				return false
			}
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "k8s-diagnostic-cli-event-forwarder")

		// Add connection keep-alive header for better reliability
		req.Header.Set("Connection", "keep-alive")

		// 🚀 ENHANCED: Adaptive timeout based on attempt number
		timeoutDuration := time.Duration(5+attempt*2) * time.Second
		client := &http.Client{
			Timeout: timeoutDuration,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		}

		startTime := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(startTime)

		if err != nil {
			log.Printf("❌ [SSE HTTP] HTTP request failed on attempt %d after %v: %v", attempt, duration, err)
			log.Printf("❌ [SSE HTTP] Error type: %T", err)

			// Check if this is a network-level error that might be temporary
			if isTemporaryNetworkError(err) {
				if attempt < maxRetries {
					delay := baseDelay * time.Duration(1<<(attempt-1)) // Exponential backoff
					log.Printf("⏳ [SSE HTTP] Temporary network error, retrying in %v...", delay)
					time.Sleep(delay)
					continue
				}
			}

			if attempt == maxRetries {
				log.Printf("❌ [SSE HTTP] All %d HTTP attempts failed", maxRetries)
				return false
			}
			continue
		}
		defer resp.Body.Close()

		log.Printf("🔍 [SSE HTTP] Response received on attempt %d after %v: Status=%d", attempt, duration, resp.StatusCode)

		// Read response body for detailed debugging
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("⚠️ [SSE HTTP] Failed to read response body: %v", readErr)
		} else {
			log.Printf("🔍 [SSE HTTP] Response body (%d bytes): %s", len(body), string(body))
		}

		// Check response status
		if resp.StatusCode == http.StatusOK {
			log.Printf("✅ [SSE HTTP] Event forwarded successfully on attempt %d: %s (status=%d, duration=%v)", attempt, sseEvent["type"], resp.StatusCode, duration)
			return true
		} else if resp.StatusCode >= 500 {
			// Server error - might be temporary, retry if we have attempts left
			log.Printf("⚠️ [SSE HTTP] Server error %d on attempt %d, will retry if attempts remain", resp.StatusCode, attempt)
			if attempt < maxRetries {
				delay := baseDelay * time.Duration(1<<(attempt-1))
				time.Sleep(delay)
				continue
			}
		} else {
			// Client error - don't retry
			log.Printf("❌ [SSE HTTP] Client error %d on attempt %d: %s", resp.StatusCode, attempt, string(body))
			return false
		}
	}

	return false
}

// isTemporaryNetworkError checks if an error might be temporary and worth retrying
func isTemporaryNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	temporaryIndicators := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"no route to host",
		"network unreachable",
		"temporary failure",
	}

	for _, indicator := range temporaryIndicators {
		if strings.Contains(strings.ToLower(errStr), indicator) {
			return true
		}
	}

	return false
}

// forwardEventViaSharedVolume writes events to shared volume as fallback mechanism
func forwardEventViaSharedVolume(testID string, sseEvent map[string]interface{}) bool {
	// Get shared volume path
	sharedPath := os.Getenv("SHARED_VOLUME_PATH")
	if sharedPath == "" {
		sharedPath = "/app/shared/repository/test_results"
	}

	// Create events directory if it doesn't exist
	eventsDir := filepath.Join(sharedPath, "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		log.Printf("❌ [SSE VOLUME] Failed to create events directory: %v", err)
		return false
	}

	// Create event file name with timestamp
	timestamp := time.Now().UTC().Format("20060102-150405.000")
	eventFile := filepath.Join(eventsDir, fmt.Sprintf("%s-%s.json", testID, timestamp))

	// Prepare event data
	eventData := map[string]interface{}{
		"testId":    testID,
		"type":      sseEvent["type"],
		"message":   sseEvent["message"],
		"timestamp": sseEvent["timestamp"],
		"phase":     sseEvent["phase"],
		"success":   sseEvent["success"],
		"testName":  sseEvent["testName"],
		"container": "cli",
		"line":      fmt.Sprintf("[SSE] %s", sseEvent["message"]),
		"fallback":  "shared_volume",
	}

	// Write event to file
	jsonData, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		log.Printf("❌ [SSE VOLUME] Failed to marshal event data: %v", err)
		return false
	}

	if err := os.WriteFile(eventFile, jsonData, 0644); err != nil {
		log.Printf("❌ [SSE VOLUME] Failed to write event file: %v", err)
		return false
	}

	log.Printf("✅ [SSE VOLUME] Event written to shared volume: %s", eventFile)
	return true
}

// executeOptimizedKubernetesCleanup executes optimized cleanup for Kubernetes production environment
// This replaces the comprehensive deepclean with lightweight cleanup optimized for individual tests
func executeOptimizedKubernetesCleanup(testID string) error {
	log.Printf("🚀 [CLI CLEANUP] Starting optimized Kubernetes cleanup for TestID: %s", testID)

	// Create a tester instance with minimal configuration for cleanup
	namespace := "diagnostic-test" // Default namespace for diagnostic tests
	kubeconfig := ""               // Use default kubeconfig from environment
	verbose := false               // Keep cleanup output clean for performance

	tester, err := core.NewTester(kubeconfig, namespace, verbose)
	if err != nil {
		log.Printf("❌ [CLI CLEANUP] Failed to create tester instance: %v", err)
		return fmt.Errorf("failed to create tester for cleanup: %w", err)
	}

	// Create timeout context for cleanup operations (much shorter than comprehensive cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Reduced from 120s
	defer cancel()

	// Send cleanup start event
	sendProgressUpdate(testID, "cleanup_starting", "🚀 Starting optimized Kubernetes cleanup...")

	// Execute our optimized Kubernetes production cleanup
	startTime := time.Now()

	// Use our new KubernetesProductionCleanup method
	tester.KubernetesProductionCleanup(ctx, testID, verbose)

	duration := time.Since(startTime)

	log.Printf("⏱️ [CLI CLEANUP] Optimized cleanup completed in %v", duration)
	log.Printf("📊 [CLI CLEANUP] Performance improvement: ~%dx faster than comprehensive cleanup",
		int(120/duration.Seconds())) // Estimate improvement over 2-minute comprehensive cleanup

	// Send cleanup completion event
	sendProgressUpdate(testID, "cleanup_completed",
		fmt.Sprintf("✅ Optimized cleanup completed in %.1fs", duration.Seconds()))

	log.Printf("✅ [CLI CLEANUP] Optimized Kubernetes cleanup completed successfully for TestID=%s", testID)
	return nil
}

// waitForCleanupCompletionWithProgress is enhanced version with progress reporting
func waitForCleanupCompletionWithProgress(testID string) error {
	log.Printf("🧹 [CLI CLEANUP] Starting cleanup completion check for TestID: %s", testID)

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		log.Printf("⚠️ [CLI CLEANUP] kubectl not available, skipping cleanup verification: %v", err)
		sendProgressUpdate(testID, "cleanup_skip", "⚠️ kubectl not available, skipping cleanup verification")
		return nil
	}

	// Maximum wait time for cleanup operations
	maxWaitTime := 60 * time.Second
	checkInterval := 2 * time.Second
	startTime := time.Now()

	// Check for ongoing cleanup processes and stuck resources
	for attempt := 1; time.Since(startTime) < maxWaitTime; attempt++ {
		log.Printf("🔍 [CLI CLEANUP] Cleanup verification attempt %d...", attempt)

		// Send progress update about ongoing verification
		sendProgressUpdate(testID, "cleanup_checking", fmt.Sprintf("🔍 Checking for ongoing cleanup operations (attempt %d)...", attempt))

		allClear := true
		issues := []string{}

		// Check for terminating pods (these indicate ongoing cleanup)
		if terminatingPods := checkTerminatingPods(); terminatingPods > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d terminating pods", terminatingPods))
		}

		// Check for stuck deployments
		if stuckDeployments := checkStuckDeployments(); stuckDeployments > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d stuck deployments", stuckDeployments))
		}

		// Check for pending policy deletions
		if pendingPolicies := checkPendingPolicyDeletions(); pendingPolicies > 0 {
			allClear = false
			issues = append(issues, fmt.Sprintf("%d pending policy deletions", pendingPolicies))
		}

		if allClear {
			log.Printf("✅ [CLI CLEANUP] All cleanup operations completed after %v (attempt %d)",
				time.Since(startTime), attempt)
			sendProgressUpdate(testID, "cleanup_completed", fmt.Sprintf("✅ Cleanup completed after %v", time.Since(startTime)))
			return nil
		}

		log.Printf("⏳ [CLI CLEANUP] Waiting for cleanup to complete... Issues: %v", issues)
		log.Printf("    Elapsed: %v, Max wait: %v", time.Since(startTime), maxWaitTime)

		// Send progress update about waiting
		sendProgressUpdate(testID, "cleanup_waiting", fmt.Sprintf("⏳ Waiting for cleanup: %s (elapsed: %v)", strings.Join(issues, ", "), time.Since(startTime)))

		// Wait before next check
		time.Sleep(checkInterval)
	}

	// Timeout reached - log warning but allow test to proceed
	log.Printf("⚠️ [CLI CLEANUP] Cleanup verification timeout reached after %v", maxWaitTime)
	log.Printf("⚠️ [CLI CLEANUP] Proceeding with test execution despite potential ongoing cleanup")

	sendProgressUpdate(testID, "cleanup_timeout", fmt.Sprintf("⚠️ Cleanup verification timeout after %v, proceeding anyway", maxWaitTime))

	// Don't return error - allow test to proceed even if cleanup is taking longer
	return nil
}

// corsMiddleware adds CORS headers for cross-origin requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("� %s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status codes
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
