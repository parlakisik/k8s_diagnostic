package system

import "context"

// FeatureCheck defines a function that validates prerequisites for a feature.
type FeatureCheck func(ctx context.Context) error

// ValidateFeatureByName runs the validator for a named feature using the JSON-based configuration.
// This now uses the shared JSON config instead of hardcoded mappings.
func ValidateFeatureByName(ctx context.Context, feature string) error {
	// Get config data from cilium-config ConfigMap
	configData, err := getCiliumConfigData()
	if err != nil {
		return err
	}

	// Use the new JSON-based validation
	return ValidateFeatureFromConfig(feature, configData)
}

// ListKnownFeatures returns feature keys known to this validator registry.
// Now dynamically loaded from the JSON configuration.
func ListKnownFeatures() []string {
	return ListAllFeatures()
}

// Legacy functions maintained for backward compatibility
// These now delegate to the JSON-based validation system

func IsValidGatewayAPI(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "gateway-api")
}

func IsValidDNSPolicies(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "dns-policies")
}

func IsValidHostFirewall(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "host-firewall")
}

func IsValidEgressGateway(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "egress-gateway")
}

func IsValidBGPControlPlane(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "bgp-control-plane")
}

func IsValidWireGuard(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "wireguard")
}

func IsValidIPsec(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "ipsec")
}

func IsValidNodePort(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "nodeport")
}

func IsValidKubeProxyReplacementStrict(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "kube-proxy-replacement-strict")
}

func IsValidL2Announcements(ctx context.Context) error {
	return ValidateFeatureByName(ctx, "l2-announcements")
}
