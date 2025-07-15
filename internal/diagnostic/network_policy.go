package diagnostic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Constants for the Cilium NetworkPolicy
const (
	PolicyName = "block-diagnostic-pods"
	PolicyFile = "block-diagnostic-pods.yaml"
)

// NetworkPolicyYAML is the YAML for a standard Kubernetes NetworkPolicy
// that blocks pod-to-pod connectivity while allowing all other traffic
const NetworkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: block-pod-ping
spec:
  podSelector:
    matchLabels:
      app: netshoot-test
  policyTypes:
  - Ingress
  - Egress
  # Default deny all ingress to simulate pod connectivity issues
  # This will cause ping to fail while still allowing other tests to work
  ingress: []
  # But allow all egress traffic for DNS and other services
  egress:
  - {}
`

// ApplyNetworkPolicy creates and applies a Kubernetes NetworkPolicy that blocks ICMP traffic between test pods
func (t *Tester) ApplyNetworkPolicy(ctx context.Context) error {
	// Create temporary directory if it doesn't exist
	tempDir := "temp"
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		err := os.Mkdir(tempDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %v", err)
		}
	}

	// Create policy file
	policyFile := filepath.Join(tempDir, PolicyFile)
	err := os.WriteFile(policyFile, []byte(NetworkPolicyYAML), 0644)
	if err != nil {
		return fmt.Errorf("failed to create policy file: %v", err)
	}

	// Apply the policy using kubectl
	cmd := exec.Command("kubectl", "apply", "-f", policyFile, "-n", t.namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply network policy: %v, output: %s", err, string(output))
	}

	return nil
}

// RemoveNetworkPolicy removes the NetworkPolicy
func (t *Tester) RemoveNetworkPolicy(ctx context.Context) error {
	// Delete the policy using kubectl
	cmd := exec.Command("kubectl", "delete", "networkpolicy", "block-pod-ping", "-n", t.namespace, "--ignore-not-found")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove network policy: %v, output: %s", err, string(output))
	}

	return nil
}
