package diagnostic

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TestResultJSON represents a single test result for JSON output
type TestResultJSON struct {
	TestNumber           int      `json:"test_number"`
	TestName             string   `json:"test_name"`
	Description          string   `json:"description"`
	Status               string   `json:"status"`
	SuccessMessage       string   `json:"success_message,omitempty"`
	ErrorMessage         string   `json:"error_message,omitempty"`
	Details              []string `json:"details"`
	StartTime            string   `json:"start_time"`
	EndTime              string   `json:"end_time"`
	ExecutionTimeSeconds float64  `json:"execution_time_seconds"`
}

// ExecutionInfoJSON represents execution metadata
type ExecutionInfoJSON struct {
	Timestamp        string `json:"timestamp"`
	Filename         string `json:"filename"`
	Namespace        string `json:"namespace"`
	KubeconfigSource string `json:"kubeconfig_source"`
	VerboseMode      bool   `json:"verbose_mode"`
}

// SummaryJSON represents the overall test summary
type SummaryJSON struct {
	TotalTests                int      `json:"total_tests"`
	Passed                    int      `json:"passed"`
	Failed                    int      `json:"failed"`
	OverallStatus             string   `json:"overall_status"`
	TotalExecutionTimeSeconds float64  `json:"total_execution_time_seconds"`
	ErrorsEncountered         []string `json:"errors_encountered"`
	CompletionTime            string   `json:"completion_time"`
}

// DiagnosticReportJSON represents the complete JSON output structure
type DiagnosticReportJSON struct {
	ExecutionInfo ExecutionInfoJSON `json:"execution_info"`
	Tests         []TestResultJSON  `json:"tests"`
	Summary       SummaryJSON       `json:"summary"`
}

// TestDescriptions maps test names to their descriptions
var TestDescriptions = map[string]string{
	"Pod-to-Pod Connectivity":         "Validates direct pod communication across different worker nodes, testing CNI networking and inter-node communication",
	"Service to Pod Connectivity":     "Validates Kubernetes service discovery, HTTP connectivity, and load balancing across multiple pod replicas",
	"Cross-Node Service Connectivity": "Validates kube-proxy inter-node routing by ensuring services work when accessed from pods on different nodes",
	"DNS Resolution":                  "Comprehensively validates Kubernetes DNS infrastructure including service discovery, FQDN resolution, and DNS search domains",
}

// TimedTestResult represents a test result with timing information
type TimedTestResult struct {
	TestResult
	StartTime time.Time
	EndTime   time.Time
}

// SaveJSONReport saves the diagnostic report to a timestamped JSON file
func SaveJSONReport(report *DiagnosticReportJSON) error {
	// Create test_results directory if it doesn't exist
	testResultsDir := "test_results"
	if err := os.MkdirAll(testResultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create test_results directory: %v", err)
	}

	// Create filename with timestamp
	filename := fmt.Sprintf("k8s-diagnostic-results-%s.json",
		time.Now().Format("20060102-150405"))

	// Full path including directory
	fullPath := fmt.Sprintf("%s/%s", testResultsDir, filename)

	// Update filename in the report (just the filename, not the full path)
	report.ExecutionInfo.Filename = filename

	// Marshal to JSON with proper indentation
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Write to file
	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %v", fullPath, err)
	}

	return nil
}

// CreateJSONReport creates a DiagnosticReportJSON from test results
func CreateJSONReport(
	namespace string,
	kubeconfigSource string,
	verbose bool,
	timedResults []TimedTestResult,
	testNames []string,
	startTime time.Time,
	endTime time.Time,
) DiagnosticReportJSON {

	// Create execution info
	executionInfo := ExecutionInfoJSON{
		Timestamp:        startTime.Format(time.RFC3339),
		Namespace:        namespace,
		KubeconfigSource: kubeconfigSource,
		VerboseMode:      verbose,
	}

	// Create test results
	var jsonTests []TestResultJSON
	var errorsEncountered []string
	passedCount := 0
	failedCount := 0

	for i, result := range timedResults {
		testName := testNames[i]

		// Determine status and messages
		status := "FAILED"
		successMessage := ""
		errorMessage := ""
		var testDetails []string

		if result.Success {
			status = "PASSED"
			successMessage = result.Message
			passedCount++
			// For successful tests, only include minimal details or none
			testDetails = []string{} // Empty details for successful tests
		} else {
			errorMessage = result.Message
			errorsEncountered = append(errorsEncountered, fmt.Sprintf("Test %d (%s): %s", i+1, testName, result.Message))
			failedCount++
			// For failed tests, include full details for debugging
			testDetails = result.Details
		}

		// Get description
		description := TestDescriptions[testName]
		if description == "" {
			description = fmt.Sprintf("Diagnostic test: %s", testName)
		}

		// Calculate execution time
		executionTime := result.EndTime.Sub(result.StartTime).Seconds()

		jsonTest := TestResultJSON{
			TestNumber:           i + 1,
			TestName:             testName,
			Description:          description,
			Status:               status,
			SuccessMessage:       successMessage,
			ErrorMessage:         errorMessage,
			Details:              testDetails,
			StartTime:            result.StartTime.Format(time.RFC3339),
			EndTime:              result.EndTime.Format(time.RFC3339),
			ExecutionTimeSeconds: executionTime,
		}

		jsonTests = append(jsonTests, jsonTest)
	}

	// Determine overall status
	overallStatus := "PASSED"
	if failedCount > 0 {
		overallStatus = "FAILED"
	}

	// Calculate total execution time
	totalExecutionTime := endTime.Sub(startTime).Seconds()

	// Create summary
	summary := SummaryJSON{
		TotalTests:                len(timedResults),
		Passed:                    passedCount,
		Failed:                    failedCount,
		OverallStatus:             overallStatus,
		TotalExecutionTimeSeconds: totalExecutionTime,
		ErrorsEncountered:         errorsEncountered,
		CompletionTime:            endTime.Format(time.RFC3339),
	}

	return DiagnosticReportJSON{
		ExecutionInfo: executionInfo,
		Tests:         jsonTests,
		Summary:       summary,
	}
}
