package core

import (
	"fmt"
	"os"
	"time"
)

// SharedTimestamp holds a timestamp that can be used across log and JSON files
// to ensure they have matching names
type SharedTimestamp struct {
	timestamp string
	time      time.Time
	useTestID bool // Flag to indicate if using testID instead of timestamp
}

// NewSharedTimestamp creates a new shared timestamp for this test execution
func NewSharedTimestamp() *SharedTimestamp {
	now := time.Now()

	// 🛡️ CRITICAL FIX: Check for BATCH_TEST_ID environment variable from UI
	if testID := os.Getenv("BATCH_TEST_ID"); testID != "" {
		return &SharedTimestamp{
			timestamp: testID, // Use testID from UI instead of timestamp
			time:      now,
			useTestID: true,
		}
	}

	// Fallback to timestamp-based naming for non-batch executions
	return &SharedTimestamp{
		timestamp: now.Format("20060102-150405"),
		time:      now,
		useTestID: false,
	}
}

// GetTimestamp returns the timestamp string (YYYYMMDD-HHMMSS format)
func (st *SharedTimestamp) GetTimestamp() string {
	return st.timestamp
}

// GetTime returns the original time.Time
func (st *SharedTimestamp) GetTime() time.Time {
	return st.time
}

// GetLogFilename returns the log filename with consistent naming
func (st *SharedTimestamp) GetLogFilename() string {
	if st.useTestID {
		// Use testID directly for batch executions from UI
		return fmt.Sprintf("%s.log", st.timestamp)
	}
	// Use traditional timestamp format for standalone executions
	return fmt.Sprintf("k8s-diagnostic-%s.log", st.timestamp)
}

// GetJSONFilename returns the JSON filename with consistent naming
func (st *SharedTimestamp) GetJSONFilename() string {
	if st.useTestID {
		// Use testID directly for batch executions from UI
		return fmt.Sprintf("%s.json", st.timestamp)
	}
	// Use traditional timestamp format for standalone executions
	return fmt.Sprintf("k8s-diagnostic-%s.json", st.timestamp)
}

// GetLogFilePath returns the full log file path
func (st *SharedTimestamp) GetLogFilePath() string {
	basePath := getBasePath()
	return fmt.Sprintf("%s/logs/%s", basePath, st.GetLogFilename())
}

// GetJSONFilePath returns the full JSON file path
func (st *SharedTimestamp) GetJSONFilePath() string {
	basePath := getBasePath()
	return fmt.Sprintf("%s/%s", basePath, st.GetJSONFilename())
}

// GetFrontendLogFilename returns the frontend JSON Lines filename
func (st *SharedTimestamp) GetFrontendLogFilename() string {
	if st.useTestID {
		// Use testID directly for batch executions from UI
		return fmt.Sprintf("%s.frontend.jsonl", st.timestamp)
	}
	// Use traditional timestamp format for standalone executions
	return fmt.Sprintf("k8s-diagnostic-%s.frontend.jsonl", st.timestamp)
}

// GetFrontendLogFilePath returns the full frontend log file path
func (st *SharedTimestamp) GetFrontendLogFilePath() string {
	basePath := getBasePath()
	return fmt.Sprintf("%s/logs/%s", basePath, st.GetFrontendLogFilename())
}

// getBasePath returns the appropriate base path for test results
// Uses SHARED_VOLUME_PATH in Kubernetes deployments, fallback to "test_results" for local dev
func getBasePath() string {
	if sharedPath := os.Getenv("SHARED_VOLUME_PATH"); sharedPath != "" {
		return sharedPath
	}
	return "test_results"
}
