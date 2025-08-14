package system

import (
	"fmt"
	"strings"
)

// getCiliumConfigData returns Cilium's config data map from cilium-config
func getCiliumConfigData() (map[string]string, error) {
	cm, err := GetConfigMapData("kube-system", "cilium-config")
	if err != nil {
		return nil, fmt.Errorf("failed to read cilium-config: %w", err)
	}
	if cm == nil || cm.Data == nil {
		return nil, fmt.Errorf("cilium-config ConfigMap has no data")
	}
	return cm.Data, nil
}

// isValidGatewayAPI validates Cilium prerequisites for Gateway API support.
// Prerequisites per docs:
// - NodePort enabled (enable-node-port=true) OR kube-proxy replacement enabled (kube-proxy-replacement=true/strict/partial/probe)
// - L7 proxy enabled (enable-l7-proxy=true)
// Docs: https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/
func isValidGatewayAPI() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}

	// Check NodePort or kube-proxy replacement
	nodePortEnabled := truthy(data["enable-node-port"]) || truthy(data["nodePort.enabled"]) // helm value mirror

	kubeProxyReplacementRaw := strings.TrimSpace(data["kube-proxy-replacement"]) // common key
	kubeProxyReplacementHelm := strings.TrimSpace(data["kubeProxyReplacement"])  // helm style
	kubeProxyEnabled := truthy(kubeProxyReplacementRaw) || truthy(kubeProxyReplacementHelm)

	if !nodePortEnabled && !kubeProxyEnabled {
		return fmt.Errorf("requires NodePort or kube-proxy replacement")
	}

	// Check L7 proxy
	l7ProxyEnabled := truthy(data["enable-l7-proxy"]) || truthy(data["l7Proxy"]) // helm value mirror
	if !l7ProxyEnabled {
		return fmt.Errorf("requires L7 proxy")
	}

	return nil
}

// truthy interprets a variety of boolean-like strings as true.
// Returns true for: true, 1, yes, y, on, enabled, enable, strict, partial, probe
func truthy(val string) bool {
	if val == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "true", "1", "yes", "y", "on", "enabled", "enable", "strict", "partial", "probe":
		return true
	case "false", "0", "no", "n", "off", "disabled", "disable":
		return false
	default:
		return false
	}
}

// Note: Individual validation functions have been moved to the JSON-based system
// in feature_definitions.go. This file now only contains shared utility functions.
