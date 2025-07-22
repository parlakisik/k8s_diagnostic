# Implementing Namespace-Based Network Policy with Cilium

This guide provides a step-by-step approach to implementing a network policy in Kubernetes that allows traffic only from pods within the same namespace. We'll use Cilium's powerful network policy capabilities to achieve precise namespace-level isolation.

## Introduction

Namespace-based network policies are crucial for:
- Multi-tenant Kubernetes clusters
- Isolation between development, staging, and production environments
- Implementing security boundaries between different application components
- Preventing cross-namespace communication for regulatory compliance
- Creating zero-trust security models within your cluster

This approach ensures pods can only communicate with other pods in their own namespace, creating strong security boundaries between different applications or tenants sharing the same Kubernetes cluster.

## Prerequisites

- A Kubernetes cluster with Cilium CNI installed
- kubectl command-line tool configured to interact with your cluster
- Basic understanding of Kubernetes networking concepts and namespaces

## Implementation Steps with Real Outputs

Below is a detailed walkthrough of implementing a namespace-based policy, including actual commands executed and outputs observed.

### Step 1: Verify Cilium Policy Enforcement Mode

First, check the current policy enforcement mode:

```bash
kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-policy}'
```

**Output:**
```
default
```

Our cluster is in the recommended `default` mode, which means policies only affect pods with policies specifically applied to them.

### Step 2: Set Up Test Environment

Create test namespaces:

```bash
kubectl create namespace policy-test
kubectl create namespace other-namespace
```

**Output:**
```
namespace/policy-test created
namespace/other-namespace created
```

Deploy pods in both namespaces:

```bash
# Deploy pods in the policy-test namespace
kubectl run web --image=nginx -n policy-test
kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600

# Deploy a pod in the other-namespace
kubectl run external-client --image=nicolaka/netshoot -n other-namespace -- sleep 3600
```

**Output:**
```
pod/web created
pod/client created
pod/external-client created
```

Wait for pods to be ready:

```bash
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
kubectl wait --for=condition=Ready pod/external-client -n other-namespace --timeout=60s
```

**Output:**
```
pod/web condition met
pod/client condition met
pod/external-client condition met
```

### Step 3: Test Baseline Connectivity (Before Policy)

Get the web pod's IP address:

```bash
WEB_POD_IP=$(kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}') && echo "Web Pod IP: $WEB_POD_IP"
```

**Output:**
```
Web Pod IP: 10.244.1.218
```

Test connectivity from a pod in the same namespace:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.
64 bytes from 10.244.1.218: icmp_seq=1 ttl=63 time=0.226 ms
64 bytes from 10.244.1.218: icmp_seq=2 ttl=63 time=0.060 ms
64 bytes from 10.244.1.218: icmp_seq=3 ttl=63 time=0.116 ms

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2059ms
rtt min/avg/max/mdev = 0.060/0.134/0.226/0.068 ms
```

Test connectivity from a pod in a different namespace:

```bash
kubectl exec -n other-namespace external-client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.
64 bytes from 10.244.1.218: icmp_seq=1 ttl=60 time=0.379 ms
64 bytes from 10.244.1.218: icmp_seq=2 ttl=60 time=0.109 ms
64 bytes from 10.244.1.218: icmp_seq=3 ttl=60 time=0.133 ms

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2085ms
rtt min/avg/max/mdev = 0.109/0.207/0.379/0.122 ms
```

As expected, in a cluster with no network policies, pods can freely communicate across namespace boundaries. This confirms our baseline connectivity is working correctly.

### Step 4: Create the Namespace-Based Policy

Create a file named `same-namespace-policy.yaml` with the following content:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "same-namespace-traffic"
spec:
  description: "Allow traffic only from pods in the same namespace"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        io.kubernetes.pod.namespace: policy-test  # This selects all pods in the policy-test namespace
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: We use this rather than the namespaced policy for more reliable enforcement
- **endpointSelector**: Targets pods with the label `run: web` (our web server)
- **ingress rule with namespace selector**: Allows traffic only from pods in the `policy-test` namespace
- **No port/protocol specifications**: By omitting `toPorts`, we allow ALL traffic types (TCP, UDP, ICMP, etc.)

### Step 5: Apply the Policy

Apply the policy:

```bash
kubectl apply -f same-namespace-policy.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/same-namespace-traffic created
```

### Step 6: Test Connectivity After Policy Application

Now that the policy is in place, let's test connectivity again.

Test from a pod in the same namespace:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.
64 bytes from 10.244.1.218: icmp_seq=1 ttl=63 time=0.221 ms
64 bytes from 10.244.1.218: icmp_seq=2 ttl=63 time=0.062 ms
64 bytes from 10.244.1.218: icmp_seq=3 ttl=63 time=0.047 ms

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2066ms
rtt min/avg/max/mdev = 0.047/0.110/0.221/0.078 ms
```

Test from a pod in a different namespace:

```bash
kubectl exec -n other-namespace external-client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 0 received, 100% packet loss, time 2052ms

command terminated with exit code 1
```

Test HTTP connectivity from the same namespace:

```bash
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

**Output (HTML content received successfully):**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

Test HTTP connectivity from a different namespace:

```bash
kubectl exec -n other-namespace external-client -- curl -s --max-time 5 $WEB_POD_IP
```

**Output:**
```
command terminated with exit code 28
```

**Result Analysis**: The tests confirm that our namespace-based policy is working as intended:

1. Traffic from pods in the same namespace (`policy-test`) is allowed
2. Traffic from pods in different namespaces (`other-namespace`) is blocked
3. Both ICMP (ping) and TCP (HTTP/curl) tests show consistent behavior
4. No packet loss for same-namespace communication
5. 100% packet loss for cross-namespace communication

### Step 7: Additional Verification (Optional)

For a deeper look at the policy details:

```bash
kubectl get ciliumclusterwidenetworkpolicy same-namespace-traffic -o yaml
```

To check endpoint details:

```bash
kubectl get ciliumendpoints -n policy-test
```

## How the Policy Works

Our namespace-based policy leverages several key Cilium features:

1. **Kubernetes Namespace Integration**: Cilium automatically adds the label `io.kubernetes.pod.namespace` to every pod with its namespace name as the value.

2. **Label Selectors**: Our policy uses two different selectors:
   - `endpointSelector`: Identifies which pods the policy applies to (pods with label `run: web`)
   - `fromEndpoints`: Identifies allowed sources (pods with label `io.kubernetes.pod.namespace: policy-test`)

3. **Implicit Deny**: Any traffic not explicitly allowed by the policy is denied, which is why cross-namespace traffic is blocked.

4. **All Traffic Types**: By not specifying protocols or ports, we allow all types of traffic within the namespace.

## Common Use Cases

Namespace-based policies are particularly useful in:

1. **Multi-tenant Clusters**: Where different teams or applications share the same cluster but need isolation

2. **Environment Separation**: Creating boundaries between development, staging, and production namespaces

3. **Regulatory Compliance**: Meeting requirements for traffic isolation between different application components

4. **Security Defense-in-Depth**: Adding namespace boundaries as an additional security layer

## Troubleshooting Common Issues

If you encounter issues with your namespace-based policy:

1. **Check Policy Validity**
   ```bash
   kubectl get ciliumclusterwidenetworkpolicies
   ```
   Ensure the VALID column shows "True"

2. **Verify Pod Namespace Labels**
   ```bash
   kubectl get pod web -n policy-test --show-labels
   ```
   Confirm the `io.kubernetes.pod.namespace` label is present

3. **Check Cilium Endpoints**
   ```bash
   kubectl get ciliumendpoints -n policy-test
   ```
   Ensure endpoints are in the "ready" state

4. **Look for Policy Errors in Logs**
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```

5. **Test with Different Traffic Types**
   Try both ping and curl to see if one works while the other doesn't

## Cleanup

When finished testing, clean up resources:

```bash
# Delete the policy
kubectl delete ciliumclusterwidenetworkpolicies same-namespace-traffic

# Delete the test namespaces
kubectl delete namespace policy-test
kubectl delete namespace other-namespace
```

## Key Takeaways

Based on our implementation and testing, here are the key takeaways:

1. **Namespace-Based Selectors**: Using the `io.kubernetes.pod.namespace` label is the key to creating namespace-based policies
2. **CiliumClusterwideNetworkPolicy** provides the most reliable way to implement network policies with Cilium
3. **No Port/Protocol Specification** allows all traffic types within the namespace
4. **Comprehensive Testing** across different namespaces confirms isolation is working correctly
5. **Zero Configuration on Clients**: This approach works without requiring any special configuration on client pods

## Summary

This guide demonstrated how to implement a namespace-based network policy using Cilium. By creating a policy that explicitly allows traffic only from within the same namespace, we established clear network boundaries between different namespaces.

The approach is powerful yet simple, requiring minimal configuration while providing strong security isolation. This pattern can be adapted for more complex scenarios, such as allowing specific cross-namespace traffic where needed, or combining namespace restrictions with additional protocol or port constraints.
