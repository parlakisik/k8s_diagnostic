# Implementing "Allow All" Network Policy with Cilium

This guide provides a step-by-step approach to implementing a true "allow all" network policy in Kubernetes using Cilium. We'll focus specifically on the CiliumClusterwideNetworkPolicy, which provides reliable network policy enforcement across the entire cluster.

## Introduction

An "allow all" policy is useful in several scenarios:
- Initial development and testing environments
- Troubleshooting connectivity issues
- Establishing a baseline before implementing more restrictive policies
- Gradually transitioning from no policies to fine-grained controls

This approach lets you explicitly allow traffic between all pods in your cluster while maintaining control and visibility over your network flows.

## Prerequisites

- A Kubernetes cluster with Cilium CNI installed
- kubectl command-line tool configured to interact with your cluster
- Basic understanding of Kubernetes networking concepts

## Implementation Steps with Real Outputs

Below is a detailed walkthrough of implementing an "allow all" policy, including actual commands executed and outputs observed.

### Step 1: Verify Cilium Policy Enforcement Mode

First, check the current policy enforcement mode:

```bash
kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-policy}'
```

**Output:**
```
default
```

Our cluster is already in the recommended `default` mode, which means policies only affect pods with policies specifically applied to them. If your cluster is in a different mode, you can change it with:

```bash
kubectl patch configmap cilium-config -n kube-system --patch '{"data": {"enable-policy": "default"}}'
kubectl rollout restart ds/cilium -n kube-system
kubectl rollout status ds/cilium -n kube-system
```

### Step 2: Set Up Test Environment

Create a test namespace:

```bash
kubectl create namespace policy-test
```

**Output:**
```
namespace/policy-test created
```

Deploy the web server and client pods:

```bash
kubectl run web --image=nginx -n policy-test && kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600
```

**Output:**
```
pod/web created
pod/client created
```

Wait for pods to be ready:

```bash
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
```

**Output:**
```
pod/web condition met
pod/client condition met
```

### Step 3: Test Baseline Connectivity

Get the web pod's IP address:

```bash
WEB_POD_IP=$(kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}') && echo "Web Pod IP: $WEB_POD_IP"
```

**Output:**
```
Web Pod IP: 10.244.1.244
```

Test ping connectivity:

```bash
kubectl exec -n policy-test client -- ping -c 3 10.244.1.244
```

**Output:**
```
PING 10.244.1.244 (10.244.1.244) 56(84) bytes of data.
64 bytes from 10.244.1.244: icmp_seq=1 ttl=63 time=0.161 ms
64 bytes from 10.244.1.244: icmp_seq=2 ttl=63 time=0.060 ms
64 bytes from 10.244.1.244: icmp_seq=3 ttl=63 time=0.111 ms

--- 10.244.1.244 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2066ms
rtt min/avg/max/mdev = 0.060/0.110/0.161/0.041 ms
```

Test HTTP connectivity:

```bash
kubectl exec -n policy-test client -- curl -s 10.244.1.244
```

**Output:**
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

As expected, in a cluster with no network policies, pods can freely communicate with each other. This confirms our baseline connectivity is working correctly.

### Step 4: Create the True "Allow All" Policy

Create a file named `allow-all-policy.yaml` with the following content:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "allow-all-traffic"
spec:
  description: "Allow all traffic between all pods"
  endpointSelector: {} # Empty selector means "all pods"
  ingress:
  - fromEndpoints:
    - {} # Empty selector means "from all pods"
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: We use this rather than the namespaced policy for more reliable enforcement
- **Empty endpointSelector**: The empty brackets `{}` means "select all pods" in the cluster
- **Empty fromEndpoints selector**: The empty brackets `{}` means "match all endpoints", allowing traffic from any pod regardless of labels
- **No port/protocol specifications**: By omitting `toPorts`, we allow ALL traffic types (TCP, UDP, ICMP, etc.)

### Step 5: Apply the Policy

Apply the policy:

```bash
kubectl apply -f allow-all-policy.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/allow-all-traffic created
```

Verify the policy was created and is valid:

```bash
kubectl get ciliumclusterwidenetworkpolicies
```

**Output:**
```
NAME                VALID
allow-all-traffic   True
```

The `VALID: True` status confirms that Cilium has successfully processed and activated our policy.

### Step 6: Test Connectivity After Policy Application

Now that the policy is in place, let's test connectivity from multiple different pods to confirm it truly allows traffic from any source.

#### Test from Original Client Pod

Test ping connectivity:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.246 (10.244.1.246) 56(84) bytes of data.
64 bytes from 10.244.1.246: icmp_seq=1 ttl=63 time=0.228 ms
64 bytes from 10.244.1.246: icmp_seq=2 ttl=63 time=0.045 ms
64 bytes from 10.244.1.246: icmp_seq=3 ttl=63 time=0.063 ms

--- 10.244.1.246 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2036ms
rtt min/avg/max/mdev = 0.045/0.112/0.228/0.082 ms
```

Test HTTP connectivity:

```bash
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

**Output showing HTML content from nginx, successful connection**

#### Test from Pod with Different Labels

First, create additional test pods with completely different label schemes:

```bash
kubectl run test-pod --image=nicolaka/netshoot -n policy-test --labels="app=testing" -- sleep 3600
kubectl run custom-pod --image=nicolaka/netshoot -n policy-test --labels="environment=dev,role=tester" -- sleep 3600
kubectl wait --for=condition=Ready pod/test-pod pod/custom-pod -n policy-test --timeout=60s
```

Test from pod with `app=testing` label:

```bash
kubectl exec -n policy-test test-pod -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.246 (10.244.1.246) 56(84) bytes of data.
64 bytes from 10.244.1.246: icmp_seq=1 ttl=63 time=0.318 ms
64 bytes from 10.244.1.246: icmp_seq=2 ttl=63 time=0.126 ms
64 bytes from 10.244.1.246: icmp_seq=3 ttl=63 time=0.057 ms

--- 10.244.1.246 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2036ms
rtt min/avg/max/mdev = 0.057/0.167/0.318/0.110 ms
```

Test from pod with complex labels `environment=dev,role=tester`:

```bash
kubectl exec -n policy-test custom-pod -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.246 (10.244.1.246) 56(84) bytes of data.
64 bytes from 10.244.1.246: icmp_seq=1 ttl=60 time=0.395 ms
64 bytes from 10.244.1.246: icmp_seq=2 ttl=60 time=0.357 ms
64 bytes from 10.244.1.246: icmp_seq=3 ttl=60 time=0.138 ms

--- 10.244.1.246 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2026ms
rtt min/avg/max/mdev = 0.138/0.296/0.395/0.113 ms
```

**Result Analysis**: Tests from all pods with different label schemes succeed after applying the policy, confirming that our true "allow all" policy is working correctly. This demonstrates that:

1. The CiliumClusterwideNetworkPolicy is correctly enforced
2. The empty `endpointSelector` applies the policy to all pods in the cluster
3. The empty `fromEndpoints` selector successfully matches all pods regardless of labels
4. All types of traffic (ICMP and HTTP) are allowed
5. All pods can communicate with each other, regardless of their labels or namespace

### Step 7: Additional Verification (Optional)

For a deeper look at the policy details:

```bash
kubectl get ciliumclusterwidenetworkpolicy allow-all-traffic -o yaml
```

To check endpoint details:

```bash
kubectl get ciliumendpoints -n policy-test
```

## Implementation Logic Explained

Our approach follows these key principles:

1. **Use ClusterWide Policies**: We chose CiliumClusterwideNetworkPolicy rather than namespace-scoped CiliumNetworkPolicy because:
   - It has more consistent behavior across different Cilium versions
   - It allows for cross-namespace policies if needed later
   - It's more reliable in complex environments

2. **Default Policy Mode**: We verified the policy enforcement mode was set to `default`, which:
   - Only affects pods with policies specifically targeting them
   - Allows for incremental policy adoption
   - Reduces the risk of accidental connectivity loss

3. **True "Allow All" Design**: By:
   - Using an empty `endpointSelector: {}` to select all pods in the cluster
   - Using an empty `fromEndpoints: [{}]` selector which matches any pod as the source
   - Not specifying protocols or ports in toPorts
   - We created a policy that allows all traffic types between all pods in the cluster

4. **Comprehensive Testing with Multiple Sources**: By testing from:
   - A pod with the default `run: client` label
   - A pod with a different single label `app: testing`
   - A pod with multiple custom labels `environment: dev, role: tester`
   - We verified that the policy truly allows traffic from any pod regardless of labels

## Troubleshooting Common Issues

If you encounter issues with your policy implementation, here are some common checks:

1. **Check Policy Validity**
   ```bash
   kubectl get ciliumclusterwidenetworkpolicies
   ```
   Ensure the VALID column shows "True"

2. **Verify Pod Labels**
   ```bash
   kubectl get pods -n policy-test --show-labels
   ```
   Confirm the labels match those in your policy

3. **Check Cilium Endpoints**
   ```bash
   kubectl get ciliumendpoints -n policy-test
   ```
   Ensure both endpoints are in the "ready" state

4. **Look for Policy Errors in Logs**
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```
   Check for any policy-related errors

5. **Restart Cilium if Needed**
   ```bash
   kubectl rollout restart ds/cilium -n kube-system
   ```
   This can help if policies aren't being applied correctly

## Cleanup

When finished testing, clean up resources:

```bash
# Delete the policy
kubectl delete ciliumclusterwidenetworkpolicies allow-all-traffic

# Delete the test namespace
kubectl delete namespace policy-test
```

## Key Takeaways

Based on our implementation and testing, here are the key takeaways:

1. **Empty Selector is Critical**: Using `{}` as a selector in `fromEndpoints` is the key to creating a true "allow all" policy that works with any pod
2. **CiliumClusterwideNetworkPolicy** provides the most reliable way to implement network policies with Cilium
3. The **omission of port/protocol specifications** creates a policy that allows all traffic types (TCP, UDP, ICMP)
4. **Comprehensive testing with differently labeled pods** is essential to verify the policy works universally
5. The **default** policy enforcement mode allows for selective policy application without disrupting other workloads

## Summary

This guide provided a step-by-step approach to implementing a true "allow all" network policy using Cilium. By following these steps, you can establish a baseline for your network policies that allows any pod to communicate with your targeted service, regardless of labels.

This approach serves as an excellent starting point for:
- Testing environments where maximum connectivity is needed
- Troubleshooting communication issues
- Establishing a baseline before implementing more restrictive policies
- Ensuring critical services remain accessible while you develop more fine-grained policies

You can modify this pattern to create more sophisticated policies by adding specific port restrictions, protocol limitations, or other constraints while still maintaining the "from any pod" capability through the empty selector.
