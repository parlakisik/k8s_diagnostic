package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FeatureTest represents a test associated with a feature
type FeatureTest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
}

// FeatureDefinition represents a Cilium feature from the JSON config
type FeatureDefinition struct {
	ConfigKeys    []string      `json:"configKeys"`
	DisplayName   string        `json:"displayName"`
	Category      string        `json:"category"`
	Priority      string        `json:"priority"`
	Description   string        `json:"description"`
	UseCase       string        `json:"useCase"`
	Complexity    string        `json:"complexity"`
	Dependencies  []string      `json:"dependencies"`
	Tests         []FeatureTest `json:"tests"`
	Documentation string        `json:"documentation"`
}

// ValidationFeatureDefinition represents validation-specific feature metadata
type ValidationFeatureDefinition struct {
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
	UseCase     string `json:"useCase"`
	Complexity  string `json:"complexity"`
}

// CiliumFeaturesConfig represents the entire JSON configuration
type CiliumFeaturesConfig struct {
	Features           map[string]FeatureDefinition           `json:"features"`
	ValidationFeatures map[string]ValidationFeatureDefinition `json:"validationFeatures"`
	TestDescriptions   map[string]string                      `json:"testDescriptions"`
}

var (
	// Cache the loaded configuration
	cachedConfig    *CiliumFeaturesConfig
	configLoadError error
)

// LoadFeatureDefinitions loads feature definitions from the shared JSON config file
func LoadFeatureDefinitions() (*CiliumFeaturesConfig, error) {
	// Return cached config if already loaded
	if cachedConfig != nil {
		return cachedConfig, nil
	}
	if configLoadError != nil {
		return nil, configLoadError
	}

	// Find the JSON config file
	configPath, err := findConfigFile()
	if err != nil {
		configLoadError = fmt.Errorf("failed to find config file: %w", err)
		return nil, configLoadError
	}

	// Read and parse the JSON file
	data, err := os.ReadFile(configPath)
	if err != nil {
		configLoadError = fmt.Errorf("failed to read config file %s: %w", configPath, err)
		return nil, configLoadError
	}

	var config CiliumFeaturesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		configLoadError = fmt.Errorf("failed to parse config JSON: %w", err)
		return nil, configLoadError
	}

	cachedConfig = &config
	return cachedConfig, nil
}

// findConfigFile locates the cilium-features.json file
func findConfigFile() (string, error) {
	// Get the current executable directory
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// Search paths relative to the executable
	searchPaths := []string{
		filepath.Join(execDir, "web", "config", "cilium-features.json"),       // ./web/config/cilium-features.json
		filepath.Join(execDir, "..", "web", "config", "cilium-features.json"), // ../web/config/cilium-features.json
		filepath.Join(execDir, "config", "cilium-features.json"),              // ./config/cilium-features.json
		"web/config/cilium-features.json",                                     // Relative to working directory
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("cilium-features.json not found in any expected location")
}

// ValidateFeatureFromConfig validates a feature using the JSON-based configuration
func ValidateFeatureFromConfig(featureName string, configData map[string]string) error {
	config, err := LoadFeatureDefinitions()
	if err != nil {
		// Fallback to hardcoded validation if config loading fails
		return validateFeatureFallback(featureName, configData)
	}

	// First check validation features (for CLI validation)
	if validationFeature, exists := config.ValidationFeatures[featureName]; exists {
		return validateFromValidationFeature(featureName, validationFeature, configData)
	}

	// Then check regular features (for config-based validation)
	if feature, exists := config.Features[featureName]; exists {
		return validateFromFeature(featureName, feature, configData)
	}

	// Feature not found in config, but don't fail - just return nil
	return nil
}

// validateFromFeature validates using regular feature definitions
func validateFromFeature(featureName string, feature FeatureDefinition, configData map[string]string) error {
	// Special handling for encryption features
	if featureName == "wireguard" {
		return validateEncryption("wireguard", configData)
	}
	if featureName == "ipsec" {
		return validateEncryption("ipsec", configData)
	}

	// Standard validation: check if any of the config keys is truthy
	for _, key := range feature.ConfigKeys {
		if truthy(configData[key]) {
			return nil // Feature is enabled
		}
	}

	return fmt.Errorf("not enabled")
}

// validateFromValidationFeature validates using validation-specific feature definitions
func validateFromValidationFeature(featureName string, feature ValidationFeatureDefinition, configData map[string]string) error {
	// Use the hardcoded validation logic for validation features since they have complex prerequisites
	return validateFeatureFallback(featureName, configData)
}

// validateEncryption handles encryption feature validation
func validateEncryption(encryptionType string, configData map[string]string) error {
	encType := strings.ToLower(strings.TrimSpace(configData["encryption-type"]))
	encMode := strings.ToLower(strings.TrimSpace(configData["encryption"]))
	specificKey := configData[fmt.Sprintf("enable-%s", encryptionType)]

	if truthy(specificKey) || encType == encryptionType || encMode == encryptionType {
		return nil
	}

	return fmt.Errorf("not enabled")
}

// validateFeatureFallback provides fallback validation using the existing hardcoded logic
func validateFeatureFallback(featureName string, configData map[string]string) error {
	switch featureName {
	case "gateway-api":
		return validateGatewayAPIFromConfig(configData)
	case "dns-policies":
		return validateDNSPoliciesFromConfig(configData)
	case "host-firewall":
		return validateHostFirewallFromConfig(configData)
	case "egress-gateway":
		return validateEgressGatewayFromConfig(configData)
	case "bgp-control-plane":
		return validateBGPControlPlaneFromConfig(configData)
	case "wireguard":
		return validateEncryption("wireguard", configData)
	case "ipsec":
		return validateEncryption("ipsec", configData)
	case "nodeport":
		return validateNodePortFromConfig(configData)
	case "kube-proxy-replacement-strict":
		return validateKubeProxyReplacementStrictFromConfig(configData)
	case "l2-announcements":
		return validateL2AnnouncementsFromConfig(configData)
	default:
		return nil // Unknown feature, don't fail
	}
}

// Individual validation functions that work with config data directly
func validateGatewayAPIFromConfig(configData map[string]string) error {
	// Check NodePort or kube-proxy replacement
	nodePortEnabled := truthy(configData["enable-node-port"]) || truthy(configData["nodePort.enabled"])
	kubeProxyEnabled := truthy(configData["kube-proxy-replacement"]) || truthy(configData["kubeProxyReplacement"])

	if !nodePortEnabled && !kubeProxyEnabled {
		return fmt.Errorf("requires NodePort or kube-proxy replacement")
	}

	// Check L7 proxy
	l7ProxyEnabled := truthy(configData["enable-l7-proxy"]) || truthy(configData["l7Proxy"])
	if !l7ProxyEnabled {
		return fmt.Errorf("requires L7 proxy")
	}

	return nil
}

func validateDNSPoliciesFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-dnsproxy"]) {
		return fmt.Errorf("requires DNS proxy")
	}
	return nil
}

func validateHostFirewallFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-host-firewall"]) {
		return fmt.Errorf("not enabled")
	}
	return nil
}

func validateEgressGatewayFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-egress-gateway"]) && !truthy(configData["egressGateway.enabled"]) {
		return fmt.Errorf("not enabled")
	}
	return nil
}

func validateBGPControlPlaneFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-bgp-control-plane"]) {
		return fmt.Errorf("not enabled")
	}
	return nil
}

func validateNodePortFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-node-port"]) && !truthy(configData["nodePort.enabled"]) {
		return fmt.Errorf("not enabled")
	}
	return nil
}

func validateKubeProxyReplacementStrictFromConfig(configData map[string]string) error {
	kpr := strings.ToLower(strings.TrimSpace(configData["kube-proxy-replacement"]))
	helm := strings.ToLower(strings.TrimSpace(configData["kubeProxyReplacement"]))
	if !(kpr == "strict" || helm == "strict") {
		return fmt.Errorf("requires strict mode")
	}
	return nil
}

func validateL2AnnouncementsFromConfig(configData map[string]string) error {
	if !truthy(configData["enable-l2-announcements"]) && !truthy(configData["l2announcements.enabled"]) {
		return fmt.Errorf("not enabled")
	}
	return nil
}

// GetFeatureDisplayName returns the display name for a feature
func GetFeatureDisplayName(featureName string) string {
	config, err := LoadFeatureDefinitions()
	if err != nil {
		// Fallback to default naming
		return strings.Title(strings.ReplaceAll(featureName, "-", " "))
	}

	if feature, exists := config.Features[featureName]; exists {
		return feature.DisplayName
	}

	if validationFeature, exists := config.ValidationFeatures[featureName]; exists {
		return validationFeature.DisplayName
	}

	// Fallback to default naming
	return strings.Title(strings.ReplaceAll(featureName, "-", " "))
}

// ListAllFeatures returns all feature names from the JSON config
func ListAllFeatures() []string {
	config, err := LoadFeatureDefinitions()
	if err != nil {
		// Fallback to hardcoded list
		return []string{
			"gateway-api", "dns-policies", "host-firewall", "egress-gateway",
			"bgp-control-plane", "wireguard", "ipsec", "nodeport",
			"kube-proxy-replacement-strict", "l2-announcements",
		}
	}

	var features []string
	for name := range config.Features {
		features = append(features, name)
	}
	for name := range config.ValidationFeatures {
		features = append(features, name)
	}

	return features
}
