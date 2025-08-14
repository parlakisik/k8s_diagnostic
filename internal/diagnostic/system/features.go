package system

import (
	"context"
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
		return fmt.Errorf(
			"gateway api prerequisites not met: enable either NodePort (enable-node-port=true) or kube-proxy replacement (kube-proxy-replacement=true/strict). see docs: %s",
			"https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/",
		)
	}

	// Check L7 proxy
	l7ProxyEnabled := truthy(data["enable-l7-proxy"]) || truthy(data["l7Proxy"]) // helm value mirror
	if !l7ProxyEnabled {
		return fmt.Errorf(
			"gateway api prerequisite not met: L7 proxy must be enabled (enable-l7-proxy=true). see docs: %s",
			"https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/",
		)
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

// Optional exported wrapper for use by other packages, if needed.
func IsValidGatewayAPI(ctx context.Context) error { // ctx reserved for future use
	return isValidGatewayAPI()
}

// isValidDNSPolicies validates prerequisites for DNS-aware policies
// Requires DNS proxy enabled
// Docs: https://docs.cilium.io/en/stable/security/policy/l7/dns/
func isValidDNSPolicies() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-dnsproxy"]) {
		return fmt.Errorf(
			"dns policy prerequisite not met: DNS proxy must be enabled (enable-dnsproxy=true). see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidDNSPolicies(ctx context.Context) error { return isValidDNSPolicies() }

// isValidHostFirewall checks host firewall feature
// Requires enable-host-firewall=true
// Docs: https://docs.cilium.io/en/stable/security/host-firewall/
func isValidHostFirewall() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-host-firewall"]) {
		return fmt.Errorf(
			"host firewall prerequisite not met: host firewall must be enabled (enable-host-firewall=true). see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidHostFirewall(ctx context.Context) error { return isValidHostFirewall() }

// isValidEgressGateway checks egress gateway feature
// Requires enable-egress-gateway=true
// Docs: https://docs.cilium.io/en/stable/network/egress-gateway/
func isValidEgressGateway() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-egress-gateway"]) && !truthy(data["egressGateway.enabled"]) {
		return fmt.Errorf(
			"egress gateway prerequisite not met: enable egress gateway (enable-egress-gateway=true). see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidEgressGateway(ctx context.Context) error { return isValidEgressGateway() }

// isValidBGPControlPlane checks BGP control plane
// Requires enable-bgp-control-plane=true
// Docs: https://docs.cilium.io/en/stable/network/bgp/
func isValidBGPControlPlane() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-bgp-control-plane"]) {
		return fmt.Errorf(
			"bgp control plane prerequisite not met: enable BGP control plane (enable-bgp-control-plane=true). see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidBGPControlPlane(ctx context.Context) error { return isValidBGPControlPlane() }

// isValidWireGuard checks WireGuard encryption
// Requires enable-wireguard=true (or encryption-type=wireguard)
// Docs: https://docs.cilium.io/en/stable/security/transparent-encryption/
func isValidWireGuard() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	encType := strings.ToLower(strings.TrimSpace(data["encryption-type"]))
	encMode := strings.ToLower(strings.TrimSpace(data["encryption"]))
	if !truthy(data["enable-wireguard"]) && encType != "wireguard" && encMode != "wireguard" {
		return fmt.Errorf(
			"wireguard prerequisite not met: enable wireguard (enable-wireguard=true) or set encryption-type=wireguard. see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidWireGuard(ctx context.Context) error { return isValidWireGuard() }

// isValidIPsec checks IPsec encryption
// Requires enable-ipsec=true (or encryption-type=ipsec)
// Docs: https://docs.cilium.io/en/stable/security/transparent-encryption/
func isValidIPsec() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	encType := strings.ToLower(strings.TrimSpace(data["encryption-type"]))
	encMode := strings.ToLower(strings.TrimSpace(data["encryption"]))
	if !truthy(data["enable-ipsec"]) && encType != "ipsec" && encMode != "ipsec" {
		return fmt.Errorf(
			"ipsec prerequisite not met: enable ipsec (enable-ipsec=true) or set encryption-type=ipsec. see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidIPsec(ctx context.Context) error { return isValidIPsec() }

// isValidNodePort checks NodePort feature
// Requires enable-node-port=true
func isValidNodePort() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-node-port"]) && !truthy(data["nodePort.enabled"]) {
		return fmt.Errorf("nodeport prerequisite not met: enable-node-port=true")
	}
	return nil
}

func IsValidNodePort(ctx context.Context) error { return isValidNodePort() }

// isValidKubeProxyReplacementStrict checks strict replacement for kube-proxy-less setups
// Requires kube-proxy-replacement=strict (or truthy interpreted as strict in some deploys)
func isValidKubeProxyReplacementStrict() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	kpr := strings.ToLower(strings.TrimSpace(data["kube-proxy-replacement"]))
	helm := strings.ToLower(strings.TrimSpace(data["kubeProxyReplacement"]))
	if !(kpr == "strict" || helm == "strict") {
		return fmt.Errorf("kube-proxy replacement prerequisite not met: set kube-proxy-replacement=strict for kube-proxy-less mode")
	}
	return nil
}

func IsValidKubeProxyReplacementStrict(ctx context.Context) error {
	return isValidKubeProxyReplacementStrict()
}

// isValidL2Announcements checks L2 announcements feature
// Requires enable-l2-announcements=true (or l2announcements.enabled=true)
// Docs: https://docs.cilium.io/en/stable/network/l2-announcements/
func isValidL2Announcements() error {
	data, err := getCiliumConfigData()
	if err != nil {
		return err
	}
	if !truthy(data["enable-l2-announcements"]) && !truthy(data["l2announcements.enabled"]) {
		return fmt.Errorf(
			"l2 announcements prerequisite not met: enable L2 announcements (enable-l2-announcements=true). see docs: %s",
			"https://docs.cilium.io/en/stable/",
		)
	}
	return nil
}

func IsValidL2Announcements(ctx context.Context) error { return isValidL2Announcements() }
