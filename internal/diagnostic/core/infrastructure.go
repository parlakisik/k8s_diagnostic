package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ClusterInfrastructure contains comprehensive cluster infrastructure information
type ClusterInfrastructure struct {
	KubernetesVersion string           `json:"kubernetesVersion"`
	NodeCount         int              `json:"nodeCount"`
	CNIProvider       string           `json:"cniProvider"`
	CNIVersion        string           `json:"cniVersion,omitempty"`
	Platform          string           `json:"platform"`
	Nodes             []NodeInfo       `json:"nodes"`
	ClusterResources  *ResourceSummary `json:"clusterResources,omitempty"`
	CollectionErrors  []string         `json:"collectionErrors,omitempty"`
	CollectedAt       time.Time        `json:"collectedAt"`
}

// NodeInfo contains information about a single node
type NodeInfo struct {
	Name          string            `json:"name"`
	Role          string            `json:"role"` // "control-plane", "worker"
	KernelVersion string            `json:"kernelVersion,omitempty"`
	OSImage       string            `json:"osImage,omitempty"`
	PodCIDR       string            `json:"podCIDR,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// ResourceSummary contains cluster resource information
type ResourceSummary struct {
	TotalCPU    string `json:"totalCPU,omitempty"`
	TotalMemory string `json:"totalMemory,omitempty"`
	PodCapacity int    `json:"podCapacity,omitempty"`
}

// InfrastructureCollector handles gathering cluster infrastructure information
type InfrastructureCollector struct {
	clientset *kubernetes.Clientset
	verbose   bool
}

// NewInfrastructureCollector creates a new infrastructure collector
func NewInfrastructureCollector(clientset *kubernetes.Clientset, verbose bool) *InfrastructureCollector {
	return &InfrastructureCollector{
		clientset: clientset,
		verbose:   verbose,
	}
}

// CollectInfrastructure gathers comprehensive cluster infrastructure information
func (ic *InfrastructureCollector) CollectInfrastructure(ctx context.Context) *ClusterInfrastructure {
	infrastructure := &ClusterInfrastructure{
		CollectedAt:      time.Now(),
		CollectionErrors: []string{},
	}

	if ic.verbose {
		fmt.Printf("🔍 Collecting cluster infrastructure information...\n")
	}

	// Collect with timeout to prevent hanging
	collectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. Collect Kubernetes version
	ic.collectKubernetesVersion(collectCtx, infrastructure)

	// 2. Collect node information
	ic.collectNodeInformation(collectCtx, infrastructure)

	// 3. Detect CNI provider and version
	ic.detectCNIProvider(collectCtx, infrastructure)

	// 4. Detect platform (kind, EKS, GKE, etc.)
	ic.detectPlatform(collectCtx, infrastructure)

	// 5. Collect cluster resources (optional, non-critical)
	ic.collectClusterResources(collectCtx, infrastructure)

	// Log collection summary
	if ic.verbose {
		ic.logCollectionSummary(infrastructure)
	}

	return infrastructure
}

// collectKubernetesVersion gets the Kubernetes API server version
func (ic *InfrastructureCollector) collectKubernetesVersion(ctx context.Context, infrastructure *ClusterInfrastructure) {
	version, err := ic.clientset.Discovery().ServerVersion()
	if err != nil {
		infrastructure.CollectionErrors = append(infrastructure.CollectionErrors,
			fmt.Sprintf("Failed to get Kubernetes version: %v", err))
		infrastructure.KubernetesVersion = "unknown"
		return
	}

	// Remove 'v' prefix from GitVersion if it already exists
	gitVersion := version.GitVersion
	if strings.HasPrefix(gitVersion, "v") {
		gitVersion = gitVersion[1:] // Remove the 'v' prefix
	}
	infrastructure.KubernetesVersion = fmt.Sprintf("v%s", gitVersion)
	if ic.verbose {
		fmt.Printf("  ✓ Kubernetes version: %s\n", infrastructure.KubernetesVersion)
	}
}

// collectNodeInformation gathers detailed information about cluster nodes
func (ic *InfrastructureCollector) collectNodeInformation(ctx context.Context, infrastructure *ClusterInfrastructure) {
	nodes, err := ic.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		infrastructure.CollectionErrors = append(infrastructure.CollectionErrors,
			fmt.Sprintf("Failed to get node information: %v", err))
		infrastructure.NodeCount = 0
		return
	}

	infrastructure.NodeCount = len(nodes.Items)
	infrastructure.Nodes = make([]NodeInfo, 0, len(nodes.Items))

	totalPodCapacity := 0

	for _, node := range nodes.Items {
		nodeInfo := NodeInfo{
			Name:   node.Name,
			Role:   ic.determineNodeRole(node.Labels),
			Labels: make(map[string]string),
		}

		// Extract key system information
		if node.Status.NodeInfo.KernelVersion != "" {
			nodeInfo.KernelVersion = node.Status.NodeInfo.KernelVersion
		}
		if node.Status.NodeInfo.OSImage != "" {
			nodeInfo.OSImage = node.Status.NodeInfo.OSImage
		}
		if node.Spec.PodCIDR != "" {
			nodeInfo.PodCIDR = node.Spec.PodCIDR
		}

		// Extract relevant labels (platform-specific, role-related)
		for key, value := range node.Labels {
			if ic.isRelevantLabel(key) {
				nodeInfo.Labels[key] = value
			}
		}

		// Extract pod capacity for resource summary
		if capacity, ok := node.Status.Allocatable["pods"]; ok {
			if podCap := capacity.Value(); podCap > 0 {
				totalPodCapacity += int(podCap)
			}
		}

		infrastructure.Nodes = append(infrastructure.Nodes, nodeInfo)
	}

	// Initialize cluster resources with pod capacity
	if infrastructure.ClusterResources == nil {
		infrastructure.ClusterResources = &ResourceSummary{}
	}
	infrastructure.ClusterResources.PodCapacity = totalPodCapacity

	if ic.verbose {
		fmt.Printf("  ✓ Nodes: %d (%d control-plane, %d worker)\n",
			infrastructure.NodeCount,
			ic.countNodesByRole(infrastructure.Nodes, "control-plane"),
			ic.countNodesByRole(infrastructure.Nodes, "worker"))
	}
}

// detectCNIProvider attempts to identify the CNI provider and version
func (ic *InfrastructureCollector) detectCNIProvider(ctx context.Context, infrastructure *ClusterInfrastructure) {
	// 1. Try Cilium first (most likely for our use case)
	if ic.detectCilium(ctx, infrastructure) {
		return
	}

	// 2. Try other common CNI providers
	if ic.detectFlannel(ctx, infrastructure) {
		return
	}

	if ic.detectCalico(ctx, infrastructure) {
		return
	}

	if ic.detectWeave(ctx, infrastructure) {
		return
	}

	// 3. Fallback detection from node annotations/status
	ic.detectCNIFromNodes(ctx, infrastructure)
}

// detectCilium detects Cilium CNI and version
func (ic *InfrastructureCollector) detectCilium(ctx context.Context, infrastructure *ClusterInfrastructure) bool {
	// Check for Cilium DaemonSet
	ciliumDS, err := ic.clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "cilium", metav1.GetOptions{})
	if err == nil && ciliumDS != nil {
		infrastructure.CNIProvider = "cilium"

		// Extract version from image tag
		for _, container := range ciliumDS.Spec.Template.Spec.Containers {
			if container.Name == "cilium-agent" && strings.Contains(container.Image, "cilium/cilium") {
				if parts := strings.Split(container.Image, ":"); len(parts) > 1 {
					infrastructure.CNIVersion = strings.TrimPrefix(parts[1], "v")
				}
				break
			}
		}

		if ic.verbose {
			fmt.Printf("  ✓ CNI: Cilium %s\n", infrastructure.CNIVersion)
		}
		return true
	}

	// Check for Cilium ConfigMap as secondary confirmation
	_, err = ic.clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "cilium-config", metav1.GetOptions{})
	if err == nil {
		infrastructure.CNIProvider = "cilium"
		infrastructure.CNIVersion = "detected"
		if ic.verbose {
			fmt.Printf("  ✓ CNI: Cilium (version unknown)\n")
		}
		return true
	}

	return false
}

// detectFlannel detects Flannel CNI
func (ic *InfrastructureCollector) detectFlannel(ctx context.Context, infrastructure *ClusterInfrastructure) bool {
	_, err := ic.clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "kube-flannel-ds", metav1.GetOptions{})
	if err == nil {
		infrastructure.CNIProvider = "flannel"
		if ic.verbose {
			fmt.Printf("  ✓ CNI: Flannel\n")
		}
		return true
	}
	return false
}

// detectCalico detects Calico CNI
func (ic *InfrastructureCollector) detectCalico(ctx context.Context, infrastructure *ClusterInfrastructure) bool {
	_, err := ic.clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "calico-node", metav1.GetOptions{})
	if err == nil {
		infrastructure.CNIProvider = "calico"
		if ic.verbose {
			fmt.Printf("  ✓ CNI: Calico\n")
		}
		return true
	}
	return false
}

// detectWeave detects Weave Net CNI
func (ic *InfrastructureCollector) detectWeave(ctx context.Context, infrastructure *ClusterInfrastructure) bool {
	_, err := ic.clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "weave-net", metav1.GetOptions{})
	if err == nil {
		infrastructure.CNIProvider = "weave"
		if ic.verbose {
			fmt.Printf("  ✓ CNI: Weave Net\n")
		}
		return true
	}
	return false
}

// detectCNIFromNodes attempts to detect CNI from node information
func (ic *InfrastructureCollector) detectCNIFromNodes(ctx context.Context, infrastructure *ClusterInfrastructure) {
	if len(infrastructure.Nodes) == 0 {
		infrastructure.CNIProvider = "unknown"
		infrastructure.CollectionErrors = append(infrastructure.CollectionErrors,
			"Unable to detect CNI provider")
		return
	}

	// Look for CNI-related annotations or labels on nodes
	for _, node := range infrastructure.Nodes {
		for key := range node.Labels {
			if strings.Contains(strings.ToLower(key), "cilium") {
				infrastructure.CNIProvider = "cilium"
				return
			}
			if strings.Contains(strings.ToLower(key), "flannel") {
				infrastructure.CNIProvider = "flannel"
				return
			}
			if strings.Contains(strings.ToLower(key), "calico") {
				infrastructure.CNIProvider = "calico"
				return
			}
		}
	}

	infrastructure.CNIProvider = "unknown"
	if ic.verbose {
		fmt.Printf("  ⚠️ CNI: Unable to detect CNI provider\n")
	}
}

// detectPlatform attempts to identify the Kubernetes platform (kind, EKS, GKE, etc.)
func (ic *InfrastructureCollector) detectPlatform(ctx context.Context, infrastructure *ClusterInfrastructure) {
	if len(infrastructure.Nodes) == 0 {
		infrastructure.Platform = "unknown"
		return
	}

	// Check first node's labels for platform indicators
	firstNode := infrastructure.Nodes[0]

	// Check for kind
	if strings.Contains(firstNode.Name, "kind") ||
		firstNode.Labels["node.kubernetes.io/instance-type"] == "kind" {
		infrastructure.Platform = "kind"
		if ic.verbose {
			fmt.Printf("  ✓ Platform: Kind (local development)\n")
		}
		return
	}

	// Check for AWS EKS
	if firstNode.Labels["eks.amazonaws.com/nodegroup"] != "" ||
		strings.Contains(firstNode.Name, "eks") ||
		firstNode.Labels["node.kubernetes.io/instance-type"] != "" &&
			strings.HasPrefix(firstNode.Labels["node.kubernetes.io/instance-type"], "m") {
		infrastructure.Platform = "eks"
		if ic.verbose {
			fmt.Printf("  ✓ Platform: Amazon EKS\n")
		}
		return
	}

	// Check for Google GKE
	if firstNode.Labels["cloud.google.com/gke-nodepool"] != "" ||
		strings.Contains(firstNode.Name, "gke") {
		infrastructure.Platform = "gke"
		if ic.verbose {
			fmt.Printf("  ✓ Platform: Google GKE\n")
		}
		return
	}

	// Check for Azure AKS
	if firstNode.Labels["agentpool"] != "" ||
		strings.Contains(firstNode.Name, "aks") {
		infrastructure.Platform = "aks"
		if ic.verbose {
			fmt.Printf("  ✓ Platform: Azure AKS\n")
		}
		return
	}

	// Check for other cloud providers
	if firstNode.Labels["node.kubernetes.io/instance-type"] != "" {
		instanceType := firstNode.Labels["node.kubernetes.io/instance-type"]
		switch {
		case strings.HasPrefix(instanceType, "Standard_"):
			infrastructure.Platform = "azure"
		case strings.HasPrefix(instanceType, "n1-") || strings.HasPrefix(instanceType, "e2-"):
			infrastructure.Platform = "gcp"
		case strings.Contains(instanceType, "."):
			infrastructure.Platform = "aws"
		default:
			infrastructure.Platform = "cloud-unknown"
		}
	} else {
		infrastructure.Platform = "on-premises"
	}

	if ic.verbose {
		fmt.Printf("  ✓ Platform: %s\n", infrastructure.Platform)
	}
}

// collectClusterResources gathers cluster resource information (non-critical)
func (ic *InfrastructureCollector) collectClusterResources(ctx context.Context, infrastructure *ClusterInfrastructure) {
	if infrastructure.ClusterResources == nil {
		infrastructure.ClusterResources = &ResourceSummary{}
	}

	// This is optional and non-critical, so we'll keep it simple
	// Could be extended to collect CPU/Memory totals if needed
	if ic.verbose && infrastructure.ClusterResources.PodCapacity > 0 {
		fmt.Printf("  ✓ Resources: %d total pod capacity\n", infrastructure.ClusterResources.PodCapacity)
	}
}

// Helper methods

// determineNodeRole determines if a node is control-plane or worker based on labels
func (ic *InfrastructureCollector) determineNodeRole(labels map[string]string) string {
	for key := range labels {
		if strings.Contains(key, "control-plane") ||
			strings.Contains(key, "master") ||
			key == "node-role.kubernetes.io/control-plane" ||
			key == "node-role.kubernetes.io/master" {
			return "control-plane"
		}
	}
	return "worker"
}

// isRelevantLabel determines if a node label should be included in the infrastructure data
func (ic *InfrastructureCollector) isRelevantLabel(key string) bool {
	relevantPrefixes := []string{
		"node.kubernetes.io/",
		"kubernetes.io/",
		"eks.amazonaws.com/",
		"cloud.google.com/",
		"agentpool",
		"node-role.kubernetes.io/",
		"topology.kubernetes.io/",
	}

	for _, prefix := range relevantPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	// Include platform-specific labels
	platformKeywords := []string{"cilium", "calico", "flannel", "weave", "kind", "eks", "gke", "aks"}
	keyLower := strings.ToLower(key)
	for _, keyword := range platformKeywords {
		if strings.Contains(keyLower, keyword) {
			return true
		}
	}

	return false
}

// countNodesByRole counts nodes with a specific role
func (ic *InfrastructureCollector) countNodesByRole(nodes []NodeInfo, role string) int {
	count := 0
	for _, node := range nodes {
		if node.Role == role {
			count++
		}
	}
	return count
}

// logCollectionSummary logs a summary of the collection process
func (ic *InfrastructureCollector) logCollectionSummary(infrastructure *ClusterInfrastructure) {
	if len(infrastructure.CollectionErrors) > 0 {
		fmt.Printf("  ⚠️ Infrastructure collection completed with %d warnings:\n", len(infrastructure.CollectionErrors))
		for _, err := range infrastructure.CollectionErrors {
			fmt.Printf("    - %s\n", err)
		}
	} else {
		fmt.Printf("  ✅ Infrastructure collection completed successfully\n")
	}
}

// GetInfrastructureSummary returns a human-readable summary
func (infrastructure *ClusterInfrastructure) GetInfrastructureSummary() string {
	if len(infrastructure.CollectionErrors) > 3 {
		return "couldn't verify infrastructure settings"
	}

	summary := fmt.Sprintf("%s on %s (%d nodes, %s CNI)",
		infrastructure.KubernetesVersion,
		infrastructure.Platform,
		infrastructure.NodeCount,
		infrastructure.CNIProvider)

	if infrastructure.CNIVersion != "" && infrastructure.CNIVersion != "detected" {
		summary = fmt.Sprintf("%s v%s",
			strings.Replace(summary, infrastructure.CNIProvider+" CNI", infrastructure.CNIProvider, 1),
			infrastructure.CNIVersion)
	}

	return summary
}

// HasCriticalErrors returns true if there were critical errors during collection
func (infrastructure *ClusterInfrastructure) HasCriticalErrors() bool {
	return len(infrastructure.CollectionErrors) > 3 ||
		infrastructure.KubernetesVersion == "unknown" ||
		infrastructure.NodeCount == 0
}
