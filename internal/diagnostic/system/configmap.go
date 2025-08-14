package system

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"k8s-diagnostic/internal/diagnostic/core"
)

// ConfigMap represents the structure of a Kubernetes ConfigMap
type ConfigMap struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   Metadata          `json:"metadata"`
	Data       map[string]string `json:"data"`
}

// Metadata represents the metadata structure of a ConfigMap
type Metadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// GetConfigMapData retrieves configmap data by running kubectl command
func GetConfigMapData(namespace, configMapName string) (*ConfigMap, error) {
	// Default to kube-system if namespace is empty
	effectiveNamespace := namespace
	if effectiveNamespace == "" {
		effectiveNamespace = "kube-system"
	}

	logger := core.GetGlobalMultiChannelLogger()
	ce := core.NewCommandExecutor(logger, effectiveNamespace, false)
	output, err := ce.ExecuteKubectlCommand(
		context.Background(),
		"get", "cm", configMapName, "-o", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute kubectl command: %w", err)
	}

	var configMap ConfigMap
	if err := json.Unmarshal([]byte(output), &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap data: %w", err)
	}

	return &configMap, nil
}

// GetConfigMapValue returns the value of a specific key from the configmap
func GetConfigMapValue(configMap *ConfigMap, key string) (string, error) {
	if configMap == nil {
		return "", fmt.Errorf("configmap is nil")
	}

	value, exists := configMap.Data[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found in configmap", key)
	}

	return value, nil
}

// GetConfigMapKeys returns all keys from the configmap
func GetConfigMapKeys(configMap *ConfigMap) ([]string, error) {
	if configMap == nil {
		return nil, fmt.Errorf("configmap is nil")
	}

	keys := make([]string, 0, len(configMap.Data))
	for key := range configMap.Data {
		keys = append(keys, key)
	}

	return keys, nil
}

// GetConfigMapDataMap returns all keys and values from the configmap in a map
func GetConfigMapDataMap(configMap *ConfigMap) (map[string]string, error) {
	if configMap == nil {
		return nil, fmt.Errorf("configmap is nil")
	}

	// Create a copy of the data map to avoid external modifications
	dataMap := make(map[string]string, len(configMap.Data))
	for key, value := range configMap.Data {
		dataMap[key] = value
	}

	return dataMap, nil
}

func TestGetConfigMapValue(t *testing.T) {
	// Create a test configmap
	testConfigMap := &ConfigMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: Metadata{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	tests := []struct {
		name        string
		configMap   *ConfigMap
		key         string
		expected    string
		expectError bool
	}{
		{
			name:        "Valid key",
			configMap:   testConfigMap,
			key:         "key1",
			expected:    "value1",
			expectError: false,
		},
		{
			name:        "Another valid key",
			configMap:   testConfigMap,
			key:         "key2",
			expected:    "value2",
			expectError: false,
		},
		{
			name:        "Non-existent key",
			configMap:   testConfigMap,
			key:         "nonexistent",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Nil configmap",
			configMap:   nil,
			key:         "key1",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetConfigMapValue(tt.configMap, tt.key)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestGetConfigMapKeys(t *testing.T) {
	// Create a test configmap
	testConfigMap := &ConfigMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: Metadata{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	tests := []struct {
		name        string
		configMap   *ConfigMap
		expectError bool
		expectedLen int
	}{
		{
			name:        "Valid configmap",
			configMap:   testConfigMap,
			expectError: false,
			expectedLen: 3,
		},
		{
			name: "Empty configmap",
			configMap: &ConfigMap{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Metadata: Metadata{
					Name:      "empty-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{},
			},
			expectError: false,
			expectedLen: 0,
		},
		{
			name:        "Nil configmap",
			configMap:   nil,
			expectError: true,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetConfigMapKeys(tt.configMap)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != tt.expectedLen {
					t.Errorf("Expected %d keys, got %d", tt.expectedLen, len(result))
				}
			}
		})
	}
}

func TestGetConfigMapDataMap(t *testing.T) {
	// Create a test configmap
	testConfigMap := &ConfigMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: Metadata{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	tests := []struct {
		name        string
		configMap   *ConfigMap
		expectError bool
		expectedLen int
	}{
		{
			name:        "Valid configmap",
			configMap:   testConfigMap,
			expectError: false,
			expectedLen: 3,
		},
		{
			name: "Empty configmap",
			configMap: &ConfigMap{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Metadata: Metadata{
					Name:      "empty-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{},
			},
			expectError: false,
			expectedLen: 0,
		},
		{
			name:        "Nil configmap",
			configMap:   nil,
			expectError: true,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetConfigMapDataMap(tt.configMap)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != tt.expectedLen {
					t.Errorf("Expected %d items, got %d", tt.expectedLen, len(result))
				}

				// Verify the returned map is a copy and not the original
				if tt.configMap != nil && len(result) > 0 {
					// Modify the original to ensure we have a copy
					originalKey := "key1"
					if _, exists := tt.configMap.Data[originalKey]; exists {
						tt.configMap.Data[originalKey] = "modified"
						if result[originalKey] == "modified" {
							t.Errorf("Returned map is not a copy - modification affected result")
						}
					}
				}
			}
		})
	}
}

func TestConfigMapStructures(t *testing.T) {
	// Test ConfigMap structure
	configMap := &ConfigMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: Metadata{
			Name:      "test-config",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}

	if configMap.APIVersion != "v1" {
		t.Errorf("Expected APIVersion 'v1', got %s", configMap.APIVersion)
	}

	if configMap.Kind != "ConfigMap" {
		t.Errorf("Expected Kind 'ConfigMap', got %s", configMap.Kind)
	}

	if configMap.Metadata.Name != "test-config" {
		t.Errorf("Expected Name 'test-config', got %s", configMap.Metadata.Name)
	}

	if configMap.Metadata.Namespace != "test-namespace" {
		t.Errorf("Expected Namespace 'test-namespace', got %s", configMap.Metadata.Namespace)
	}

	if configMap.Metadata.Labels["app"] != "test" {
		t.Errorf("Expected label 'app' to be 'test', got %s", configMap.Metadata.Labels["app"])
	}

	if configMap.Data["key1"] != "value1" {
		t.Errorf("Expected Data['key1'] to be 'value1', got %s", configMap.Data["key1"])
	}
}
