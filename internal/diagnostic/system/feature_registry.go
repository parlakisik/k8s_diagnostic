package system

import "context"

// FeatureCheck defines a function that validates prerequisites for a feature.
type FeatureCheck func(ctx context.Context) error

// featureChecks holds known Cilium features mapped to their validators.
var featureChecks = map[string]FeatureCheck{
	"gateway-api":                   IsValidGatewayAPI,
	"dns-policies":                  IsValidDNSPolicies,
	"host-firewall":                 IsValidHostFirewall,
	"egress-gateway":                IsValidEgressGateway,
	"bgp-control-plane":             IsValidBGPControlPlane,
	"wireguard":                     IsValidWireGuard,
	"ipsec":                         IsValidIPsec,
	"nodeport":                      IsValidNodePort,
	"kube-proxy-replacement-strict": IsValidKubeProxyReplacementStrict,
	"l2-announcements":              IsValidL2Announcements,
}

// ValidateFeatureByName runs the validator for a named feature, if known.
func ValidateFeatureByName(ctx context.Context, feature string) error {
	if fn, ok := featureChecks[feature]; ok {
		return fn(ctx)
	}
	return nil
}

// ListKnownFeatures returns feature keys known to this validator registry.
func ListKnownFeatures() []string {
	keys := make([]string, 0, len(featureChecks))
	for k := range featureChecks {
		keys = append(keys, k)
	}
	return keys
}
