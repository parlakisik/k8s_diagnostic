package core

import (
	"fmt"
	"time"
)

// SharedTimestamp holds a timestamp that can be used across log and JSON files
// to ensure they have matching names
type SharedTimestamp struct {
	timestamp string
	time      time.Time
}

// NewSharedTimestamp creates a new shared timestamp for this test execution
func NewSharedTimestamp() *SharedTimestamp {
	now := time.Now()
	return &SharedTimestamp{
		timestamp: now.Format("20060102-150405"),
		time:      now,
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
	return fmt.Sprintf("k8s-diagnostic-%s.log", st.timestamp)
}

// GetJSONFilename returns the JSON filename with consistent naming
func (st *SharedTimestamp) GetJSONFilename() string {
	return fmt.Sprintf("k8s-diagnostic-%s.json", st.timestamp)
}

// GetLogFilePath returns the full log file path
func (st *SharedTimestamp) GetLogFilePath() string {
	return fmt.Sprintf("test_results/logs/%s", st.GetLogFilename())
}

// GetJSONFilePath returns the full JSON file path
func (st *SharedTimestamp) GetJSONFilePath() string {
	return fmt.Sprintf("test_results/%s", st.GetJSONFilename())
}

// GetFrontendLogFilename returns the frontend JSON Lines filename
func (st *SharedTimestamp) GetFrontendLogFilename() string {
	return fmt.Sprintf("k8s-diagnostic-%s.frontend.jsonl", st.timestamp)
}

// GetFrontendLogFilePath returns the full frontend log file path
func (st *SharedTimestamp) GetFrontendLogFilePath() string {
	return fmt.Sprintf("test_results/logs/%s", st.GetFrontendLogFilename())
}
