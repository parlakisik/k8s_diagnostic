package networking

import (
	"time"

	"k8s-diagnostic/internal/diagnostic/core"
)

// NetworkingTestConfigs contains all networking test configurations
var NetworkingTestConfigs = []core.PolicyTestConfig{
	// Pod Connectivity Tests
	{
		GroupId:       "networking",
		SubgroupId:    "pod-connectivity",
		TestId:        "pod-to-pod-same-node",
		TestTitle:     "Pod-to-Pod Same-Node Connectivity Test",
		LogStepName:   "Testing pod connectivity",
		LogStepFile:   "Test: pod-to-pod same-node connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "Pod-to-pod connectivity on same node (CNI networking validation)",
		ResultConfirmation: "Network connectivity confirmed - same-node pod communication",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "connectivity",
			PlacementType: "same-node",
			RequiredNodes: 1,
			Timeout:       120 * time.Second,
		},
	},
	{
		GroupId:       "networking",
		SubgroupId:    "pod-connectivity",
		TestId:        "pod-to-pod-cross-node",
		TestTitle:     "Pod-to-Pod Cross-Node Connectivity Test",
		LogStepName:   "Testing pod connectivity",
		LogStepFile:   "Test: pod-to-pod cross-node connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "Pod-to-pod connectivity across nodes (inter-node networking validation)",
		ResultConfirmation: "Network connectivity confirmed - cross-node pod communication",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "connectivity",
			PlacementType: "cross-node",
			RequiredNodes: 2,
			Timeout:       120 * time.Second,
		},
	},

	// Service Tests
	{
		GroupId:       "networking",
		SubgroupId:    "services",
		TestId:        "service-clusterip",
		TestTitle:     "Service ClusterIP Connectivity Test",
		LogStepName:   "Testing service connectivity",
		LogStepFile:   "Test: ClusterIP service connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "ClusterIP service connectivity (internal cluster networking)",
		ResultConfirmation: "Service connectivity confirmed - ClusterIP access validated",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "service",
			ServiceType:   "ClusterIP",
			RequiredNodes: 1,
			Timeout:       120 * time.Second,
		},
	},
	{
		GroupId:       "networking",
		SubgroupId:    "services",
		TestId:        "service-nodeport",
		TestTitle:     "Service NodePort Connectivity Test",
		LogStepName:   "Testing service connectivity",
		LogStepFile:   "Test: NodePort service connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "NodePort service connectivity (external cluster access)",
		ResultConfirmation: "Service connectivity confirmed - NodePort access validated",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "service",
			ServiceType:   "NodePort",
			RequiredNodes: 1,
			Timeout:       120 * time.Second,
		},
	},
	{
		GroupId:       "networking",
		SubgroupId:    "services",
		TestId:        "service-loadbalancer",
		TestTitle:     "Service LoadBalancer Connectivity Test",
		LogStepName:   "Testing service connectivity",
		LogStepFile:   "Test: LoadBalancer service connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "LoadBalancer service connectivity (cloud provider load balancing)",
		ResultConfirmation: "Service connectivity confirmed - LoadBalancer access validated",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "service",
			ServiceType:   "LoadBalancer",
			RequiredNodes: 1,
			Timeout:       120 * time.Second,
		},
	},
	{
		GroupId:       "networking",
		SubgroupId:    "services",
		TestId:        "service-cross-node",
		TestTitle:     "Cross-Node Service Connectivity Test",
		LogStepName:   "Testing service connectivity",
		LogStepFile:   "Test: cross-node service connectivity",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "Cross-node service connectivity (inter-node service routing)",
		ResultConfirmation: "Service connectivity confirmed - cross-node service access validated",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "service",
			ServiceType:   "ClusterIP",
			PlacementType: "cross-node",
			RequiredNodes: 2,
			Timeout:       120 * time.Second,
		},
	},

	// DNS Test
	{
		GroupId:       "networking",
		SubgroupId:    "dns",
		TestId:        "dns-resolution",
		TestTitle:     "DNS Resolution Test",
		LogStepName:   "Testing DNS resolution",
		LogStepFile:   "Test: service FQDN DNS resolution",
		ExpectSuccess: true,
		// Enhanced formatting fields
		ExpectedBehavior:   "Service FQDN DNS resolution (cluster DNS functionality)",
		ResultConfirmation: "DNS resolution confirmed - service discovery working",
		NetworkingConfig: &core.NetworkingTestConfig{
			TestType:      "dns",
			RequiredNodes: 1,
			Timeout:       120 * time.Second,
		},
	},
}

// GetNetworkingTestGroups returns networking test groups organized by subgroup
func GetNetworkingTestGroups() []core.PolicyTestGroup {
	// Group tests by subgroup
	subgroups := make(map[string][]core.PolicyTestConfig)

	for _, config := range NetworkingTestConfigs {
		subgroups[config.SubgroupId] = append(subgroups[config.SubgroupId], config)
	}

	// Create test groups
	var groups []core.PolicyTestGroup

	// Pod Connectivity group
	if podTests, exists := subgroups["pod-connectivity"]; exists {
		groups = append(groups, core.PolicyTestGroup{
			Name:        "pod-connectivity",
			GroupId:     "networking",
			TestConfigs: podTests,
		})
	}

	// Services group
	if serviceTests, exists := subgroups["services"]; exists {
		groups = append(groups, core.PolicyTestGroup{
			Name:        "services",
			GroupId:     "networking",
			TestConfigs: serviceTests,
		})
	}

	// DNS group
	if dnsTests, exists := subgroups["dns"]; exists {
		groups = append(groups, core.PolicyTestGroup{
			Name:        "dns",
			GroupId:     "networking",
			TestConfigs: dnsTests,
		})
	}

	return groups
}

// GetNetworkingTestByID returns a specific networking test configuration by ID
func GetNetworkingTestByID(testId string) *core.PolicyTestConfig {
	for _, config := range NetworkingTestConfigs {
		if config.TestId == testId {
			return &config
		}
	}
	return nil
}
