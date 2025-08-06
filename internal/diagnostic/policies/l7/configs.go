package l7

import (
	"fmt"

	"k8s-diagnostic/internal/diagnostic/core"
)

// L7TestConfigs defines all L7 test configurations using the common framework
// PolicyName field removed - all policy names extracted dynamically from YAML metadata
var L7TestConfigs = []core.PolicyTestConfig{
	// HTTP Subgroup
	{
		PolicyPath:    "cilium-policies/9-l7-policies/http-policies/basic-http-get-policy.yaml",
		GroupId:       "l7-policies",
		SubgroupId:    "http",
		TestId:        "basic-http-get",
		TestTitle:     "Basic HTTP GET Policy Test",
		LogStepName:   "Deploying basic HTTP GET policy",
		LogStepFile:   "Policy file: basic-http-get-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/9-l7-policies/http-policies/http-with-headers-policy.yaml",
		GroupId:       "l7-policies",
		SubgroupId:    "http",
		TestId:        "http-with-headers",
		TestTitle:     "HTTP With Headers Policy Test",
		LogStepName:   "Deploying HTTP with headers policy",
		LogStepFile:   "Policy file: http-with-headers-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/9-l7-policies/http-policies/path-method-policy.yaml",
		GroupId:       "l7-policies",
		SubgroupId:    "http",
		TestId:        "path-method",
		TestTitle:     "Path Method Policy Test",
		LogStepName:   "Deploying path method policy",
		LogStepFile:   "Policy file: path-method-policy.yaml",
		ExpectSuccess: true,
	},

	// DNS Subgroup
	{
		PolicyPath:    "cilium-policies/9-l7-policies/dns-policies/dns-matchname-policy.yaml",
		GroupId:       "l7-policies",
		SubgroupId:    "dns",
		TestId:        "dns-matchname",
		TestTitle:     "DNS Match Name Policy Test",
		LogStepName:   "Deploying DNS matchName policy",
		LogStepFile:   "Policy file: dns-matchname-policy.yaml",
		ExpectSuccess: true,
	},
	{
		PolicyPath:    "cilium-policies/9-l7-policies/dns-policies/dns-matchpattern-policy.yaml",
		GroupId:       "l7-policies",
		SubgroupId:    "dns",
		TestId:        "dns-matchpattern",
		TestTitle:     "DNS Match Pattern Policy Test",
		LogStepName:   "Deploying DNS matchPattern policy",
		LogStepFile:   "Policy file: dns-matchpattern-policy.yaml",
		ExpectSuccess: true,
	},
}

// BuildL7TestGroups creates test groups based on requested subgroups using the common framework
func BuildL7TestGroups(requestedSubgroups []string) []core.PolicyTestGroup {
	groups := []core.PolicyTestGroup{}

	// If no specific subgroups are requested, use all available subgroups
	if len(requestedSubgroups) == 0 {
		for subgroup := range L7PolicySubgroups {
			requestedSubgroups = append(requestedSubgroups, subgroup)
		}
	}

	// Build groups based on requested subgroups
	for _, subgroupName := range requestedSubgroups {
		testIds, exists := L7PolicySubgroups[subgroupName]
		if !exists {
			fmt.Printf("Warning: Subgroup '%s' not found in L7 policies, skipping\n", subgroupName)
			continue
		}

		group := core.PolicyTestGroup{
			Name:        subgroupName,
			GroupId:     "l7-policies",
			TestConfigs: []core.PolicyTestConfig{},
		}

		// Add tests to the group
		for _, testId := range testIds {
			for _, config := range L7TestConfigs {
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
