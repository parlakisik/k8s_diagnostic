// Universal Event Bridge for CLI-to-UI communication
// Implements the missing storeEventInLogAPI functionality
// Works in both Docker Compose and Kubernetes environments

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// EventBridge handles universal event forwarding to the UI API
type EventBridge struct {
	batchTestID string
	apiURL      string
	client      *http.Client
	maxRetries  int
	retryDelay  time.Duration
}

// EventPayload represents the structure sent to the log-events API
type EventPayload struct {
	Type        string                 `json:"type"`
	TestID      string                 `json:"testId"`
	BatchTestID string                 `json:"batchTestId,omitempty"`
	TestName    string                 `json:"testName,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Timestamp   string                 `json:"timestamp"`
	Success     *bool                  `json:"success,omitempty"`
	Duration    *float64               `json:"duration,omitempty"`
	Phase       string                 `json:"phase,omitempty"`
	Step        string                 `json:"step,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewUniversalEventBridge creates a new event bridge that works in both environments
func NewUniversalEventBridge(batchTestID string) *EventBridge {
	apiURL := getUniversalAPIURL()

	return &EventBridge{
		batchTestID: batchTestID,
		apiURL:      apiURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

// getUniversalAPIURL determines the correct API URL based on environment
func getUniversalAPIURL() string {
	// Check for explicit HTTP_LOG_URL environment variable first
	if apiURL := os.Getenv("HTTP_LOG_URL"); apiURL != "" {
		return apiURL
	}

	// Check if we're in Kubernetes mode
	if os.Getenv("KUBERNETES_MODE") == "true" {
		// In Kubernetes, communicate with UI container in same pod
		return "http://localhost:3000"
	}

	// Docker Compose mode - communicate with ui service
	return "http://ui:3000"
}

// StoreEventInLogAPI implements the missing function from tester.go
// This is the critical function that was causing the infinite polling loop
func (eb *EventBridge) StoreEventInLogAPI(event EventPayload) error {
	// Ensure event has required fields for batch context
	if event.TestID == "" {
		event.TestID = eb.batchTestID
	}
	if event.BatchTestID == "" {
		event.BatchTestID = eb.batchTestID
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().Format(time.RFC3339)
	}

	return eb.postEventWithRetry(event)
}

// EmitCleanupStart emits a cleanup start event
func (eb *EventBridge) EmitCleanupStart(testName, message string) error {
	event := EventPayload{
		Type:        "cleanup_start",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Phase:       "cleanup",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitCleanupProgress emits a cleanup progress event
func (eb *EventBridge) EmitCleanupProgress(testName, message string) error {
	event := EventPayload{
		Type:        "cleanup_progress",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Phase:       "cleanup",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitCleanupComplete emits a cleanup completion event
func (eb *EventBridge) EmitCleanupComplete(testName, message string, success bool) error {
	event := EventPayload{
		Type:        "cleanup_complete",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Phase:       "cleanup",
		Success:     &success,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitTestStart emits a test start event
func (eb *EventBridge) EmitTestStart(testName, message string) error {
	event := EventPayload{
		Type:        "test_start",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Phase:       "testing",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitTestProgress emits a test progress event
func (eb *EventBridge) EmitTestProgress(testName, message, output string) error {
	event := EventPayload{
		Type:        "test_progress",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Output:      output,
		Phase:       "testing",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitTestComplete emits a test completion event
func (eb *EventBridge) EmitTestComplete(testName, message string, success bool, duration float64) error {
	event := EventPayload{
		Type:        "test_complete",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		TestName:    testName,
		Message:     message,
		Success:     &success,
		Duration:    &duration,
		Phase:       "testing",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitLiveOutput emits live output for terminal display
func (eb *EventBridge) EmitLiveOutput(output string) error {
	event := EventPayload{
		Type:        "live_output",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		Output:      output,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// EmitStepComplete emits a step completion event (used for cleanup steps)
func (eb *EventBridge) EmitStepComplete(step, message string, success bool) error {
	event := EventPayload{
		Type:        "step_complete",
		TestID:      eb.batchTestID,
		BatchTestID: eb.batchTestID,
		Step:        step,
		Message:     message,
		Success:     &success,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	return eb.StoreEventInLogAPI(event)
}

// postEventWithRetry posts an event with retry logic
func (eb *EventBridge) postEventWithRetry(event EventPayload) error {
	var lastErr error

	for attempt := 0; attempt < eb.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(eb.retryDelay)
		}

		if err := eb.postEvent(event); err != nil {
			lastErr = err
			if attempt < eb.maxRetries-1 {
				continue // Retry
			}
		} else {
			return nil // Success
		}
	}

	return fmt.Errorf("failed to post event after %d attempts: %v", eb.maxRetries, lastErr)
}

// postEvent posts a single event to the API
func (eb *EventBridge) postEvent(event EventPayload) error {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	url := fmt.Sprintf("%s/api/log-events", eb.apiURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := eb.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post event: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// GetBatchTestID returns the batch test ID
func (eb *EventBridge) GetBatchTestID() string {
	return eb.batchTestID
}

// GetAPIURL returns the API URL being used
func (eb *EventBridge) GetAPIURL() string {
	return eb.apiURL
}

// Ping tests connectivity to the log-events API
func (eb *EventBridge) Ping() error {
	url := fmt.Sprintf("%s/api/log-events?testId=%s", eb.apiURL, eb.batchTestID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %v", err)
	}

	resp, err := eb.client.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}
	defer resp.Body.Close()

	// Both 200 (events found) and 404 (no events yet) are acceptable for ping
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ping returned unexpected status %d", resp.StatusCode)
	}

	return nil
}

// Global event bridge instance - initialized when needed
var globalEventBridge *EventBridge

// GetOrCreateGlobalEventBridge returns the global event bridge, creating it if necessary
func GetOrCreateGlobalEventBridge() *EventBridge {
	if globalEventBridge == nil {
		batchTestID := os.Getenv("BATCH_TEST_ID")
		if batchTestID == "" {
			batchTestID = fmt.Sprintf("batch_%d", time.Now().Unix())
		}
		globalEventBridge = NewUniversalEventBridge(batchTestID)
	}
	return globalEventBridge
}

// StoreEventInLogAPI - Global function that implements the missing function from tester.go
// This function was being called but didn't exist, causing the infinite polling loop
func StoreEventInLogAPI(eventData interface{}, testId string, testList []string) error {
	bridge := GetOrCreateGlobalEventBridge()

	// Convert eventData to EventPayload
	var event EventPayload

	// Handle different event data types
	switch data := eventData.(type) {
	case map[string]interface{}:
		// Convert map to EventPayload
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %v", err)
		}
		if err := json.Unmarshal(jsonBytes, &event); err != nil {
			return fmt.Errorf("failed to unmarshal to EventPayload: %v", err)
		}
	case EventPayload:
		event = data
	default:
		// Create a generic event
		event = EventPayload{
			Type:      "generic_event",
			TestID:    testId,
			Message:   fmt.Sprintf("%v", eventData),
			Timestamp: time.Now().Format(time.RFC3339),
		}
	}

	// Ensure testId consistency
	if event.TestID == "" {
		event.TestID = testId
	}
	if event.BatchTestID == "" {
		event.BatchTestID = testId
	}

	return bridge.StoreEventInLogAPI(event)
}
