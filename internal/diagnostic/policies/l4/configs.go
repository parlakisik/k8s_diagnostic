package l4

import (
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// L4TestConfigs defines all L4 test configurations using the common framework
// PolicyName field removed - all policy names extracted dynamically from YAML metadata
var L4TestConfigs = []core.PolicyTestConfig{
	// Port Subgroup
	{
		PolicyPath:    "cilium-policies/8-l4-policies/basic-port-policies/tcp-port-ingress-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "port",
		TestId:        "tcp-port-ingress",
		TestTitle:     "TCP Port Ingress Policy Test",
		LogStepName:   "Deploying TCP port ingress policy",
		LogStepFile:   "Policy file: tcp-port-ingress-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/basic-port-policies/tcp-port-egress-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "port",
		TestId:        "tcp-port-egress",
		TestTitle:     "TCP Port Egress Policy Test",
		LogStepName:   "Deploying TCP port egress policy",
		LogStepFile:   "Policy file: tcp-port-egress-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/basic-port-policies/port-range-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "port",
		TestId:        "port-range",
		TestTitle:     "Port Range Policy Test",
		LogStepName:   "Deploying port range policy",
		LogStepFile:   "Policy file: port-range-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/basic-port-policies/multiple-port-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "port",
		TestId:        "multiple-port",
		TestTitle:     "Multiple Port Policy Test",
		LogStepName:   "Deploying multiple port policy",
		LogStepFile:   "Policy file: multiple-port-policy.yaml",
		ExpectSuccess: true,
	},

	// ICMP Subgroup
	{
		PolicyPath:    "cilium-policies/8-l4-policies/icmp-policies/icmp-type-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "icmp",
		TestId:        "icmp-type",
		TestTitle:     "ICMP Type Policy Test",
		LogStepName:   "Deploying ICMP type policy",
		LogStepFile:   "Policy file: icmp-type-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/icmp-policies/icmpv6-type-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "icmp",
		TestId:        "icmpv6-type",
		TestTitle:     "ICMPv6 Type Policy Test",
		LogStepName:   "Deploying ICMPv6 type policy",
		LogStepFile:   "Policy file: icmpv6-type-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/icmp-policies/mixed-icmp-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "icmp",
		TestId:        "mixed-icmp",
		TestTitle:     "Mixed ICMP Policy Test",
		LogStepName:   "Deploying mixed ICMP policy",
		LogStepFile:   "Policy file: mixed-icmp-policy.yaml",
		ExpectSuccess: true,
	},

	// TLS-SNI Subgroup
	{
		PolicyPath:    "cilium-policies/8-l4-policies/tls-sni-policies/basic-sni-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "tls-sni",
		TestId:        "basic-sni",
		TestTitle:     "Basic SNI Policy Test",
		LogStepName:   "Deploying basic SNI policy",
		LogStepFile:   "Policy file: basic-sni-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/tls-sni-policies/multi-domain-sni-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "tls-sni",
		TestId:        "multi-domain-sni",
		TestTitle:     "Multi-Domain SNI Policy Test",
		LogStepName:   "Deploying multi-domain SNI policy",
		LogStepFile:   "Policy file: multi-domain-sni-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/8-l4-policies/tls-sni-policies/combined-l4-sni-policy.yaml",
		GroupId:       "l4-policies",
		SubgroupId:    "tls-sni",
		TestId:        "combined-l4-sni",
		TestTitle:     "Combined L4 SNI Policy Test",
		LogStepName:   "Deploying combined L4 SNI policy",
		LogStepFile:   "Policy file: combined-l4-sni-policy.yaml",
		ExpectSuccess: true,
	},
}

// BuildL4TestGroups creates test groups based on requested subgroups using the common framework
func BuildL4TestGroups(requestedSubgroups []string) []core.PolicyTestGroup {
	groups := []core.PolicyTestGroup{}

	// If no specific subgroups are requested, use all available subgroups
	if len(requestedSubgroups) == 0 {
		for subgroup := range L4PolicySubgroups {
			requestedSubgroups = append(requestedSubgroups, subgroup)
		}
	}

	// Build groups based on requested subgroups
	for _, subgroupName := range requestedSubgroups {
		testIds, exists := L4PolicySubgroups[subgroupName]
		if !exists {
			fmt.Printf("Warning: Subgroup '%s' not found in L4 policies, skipping\n", subgroupName)
			continue
		}

		group := core.PolicyTestGroup{
			Name:        subgroupName,
			GroupId:     "l4-policies",
			TestConfigs: []core.PolicyTestConfig{},
		}

		// Add tests to the group
		for _, testId := range testIds {
			for _, config := range L4TestConfigs {
				if config.TestId == testId {
					group.TestConfigs = append(group.TestConfigs, config)
					break
				}
			}
		}

		if len(group.TestConfigs) > 0 {
			groups = append(groups, group)
		}
	}

	return groups
}
