package l3

import (
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// L3TestConfigs defines all L3 test configurations using the common framework
// PolicyName field removed - all policy names extracted dynamically from YAML metadata
var L3TestConfigs = []core.PolicyTestConfig{
	// IP/CIDR Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/cidr-policies/cidr-ingress-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "ip-cidr",
		TestId:        "cidr-ingress",
		TestTitle:     "CIDR Ingress Policy Test",
		LogStepName:   "Deploying CIDR ingress policy",
		LogStepFile:   "Policy file: cidr-ingress-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/7-l3-policies/cidr-policies/cidr-egress-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "ip-cidr",
		TestId:        "cidr-egress",
		TestTitle:     "CIDR Egress Policy Test",
		LogStepName:   "Deploying CIDR egress policy",
		LogStepFile:   "Policy file: cidr-egress-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/7-l3-policies/cidr-policies/cidr-with-except-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "ip-cidr",
		TestId:        "cidr-except",
		TestTitle:     "CIDR With Except Policy Test",
		LogStepName:   "Deploying CIDR with except policy",
		LogStepFile:   "Policy file: cidr-with-except-policy.yaml",
		ExpectSuccess: true,
	},

	// Endpoint Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/endpoints-policies/endpoints-label-selector.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "endpoint",
		TestId:        "endpoints-label",
		TestTitle:     "Endpoints Label Selector Policy Test",
		LogStepName:   "Deploying endpoints label selector policy",
		LogStepFile:   "Policy file: endpoints-label-selector.yaml",
		ExpectSuccess: true,
	},

	// Entities Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/entities-policies/entities-based-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "entities",
		TestId:        "entities-based",
		TestTitle:     "Entities Based Policy Test",
		LogStepName:   "Deploying entities based policy",
		LogStepFile:   "Policy file: entities-based-policy.yaml",
		ExpectSuccess: true,
	},

	// DNS Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/dns-policies/dns-based-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "dns",
		TestId:        "dns-based",
		TestTitle:     "DNS Based Policy Test",
		LogStepName:   "Deploying DNS based policy",
		LogStepFile:   "Policy file: dns-based-policy.yaml",
		ExpectSuccess: true,
	},

	// Node Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/node-policies/traditional-node-selector.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "node",
		TestId:        "node-selector",
		TestTitle:     "Traditional Node Selector Policy Test",
		LogStepName:   "Deploying traditional node selector policy",
		LogStepFile:   "Policy file: traditional-node-selector.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/7-l3-policies/node-policies/pod-node-name-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "node",
		TestId:        "pod-node-name",
		TestTitle:     "Pod Node Name Policy Test",
		LogStepName:   "Deploying pod node name policy",
		LogStepFile:   "Policy file: pod-node-name-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/7-l3-policies/node-policies/node-cidr-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "node",
		TestId:        "node-cidr",
		TestTitle:     "Node CIDR Policy Test",
		LogStepName:   "Deploying node CIDR policy",
		LogStepFile:   "Policy file: node-cidr-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/7-l3-policies/node-policies/node-based-policy-clusterwide.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "node",
		TestId:        "node-based",
		TestTitle:     "Node Based Policy Clusterwide Test",
		LogStepName:   "Deploying node based policy clusterwide",
		LogStepFile:   "Policy file: node-based-policy-clusterwide.yaml",
		ExpectSuccess: true,
	},

	// Service Subgroup
	{
		PolicyPath:    "cilium-policies/7-l3-policies/services-policies/kubernetes-service-policy.yaml",
		GroupId:       "l3-policies",
		SubgroupId:    "service",
		TestId:        "kubernetes-service",
		TestTitle:     "Kubernetes Service Policy Test",
		LogStepName:   "Deploying Kubernetes service policy",
		LogStepFile:   "Policy file: kubernetes-service-policy.yaml",
		ExpectSuccess: true,
	},
}

// BuildL3TestGroups creates test groups based on requested subgroups using the common framework
func BuildL3TestGroups(requestedSubgroups []string) []core.PolicyTestGroup {
	groups := []core.PolicyTestGroup{}

	// If no specific subgroups are requested, use all available subgroups
	if len(requestedSubgroups) == 0 {
		for subgroup := range L3PolicySubgroups {
			requestedSubgroups = append(requestedSubgroups, subgroup)
		}
	}

	// Build groups based on requested subgroups
	for _, subgroupName := range requestedSubgroups {
		testIds, exists := L3PolicySubgroups[subgroupName]
		if !exists {
			fmt.Printf("Warning: Subgroup '%s' not found in L3 policies, skipping\n", subgroupName)
			continue
		}

		group := core.PolicyTestGroup{
			Name:        subgroupName,
			GroupId:     "l3-policies",
			TestConfigs: []core.PolicyTestConfig{},
		}

		// Add tests to the group
		for _, testId := range testIds {
			for _, config := range L3TestConfigs {
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
