# Implementing Namespace-Specific Deny Policies with Cilium

This guide provides a step-by-step approach to implementing network policies in Kubernetes that specifically deny traffic from certain namespaces while allowing all other traffic. We'll demonstrate two methods to achieve this with Cilium's powerful network policy capabilities.

## Introduction

Namespace-specific deny policies are valuable in several scenarios:
- Isolating sensitive workloads from specific untrusted namespaces
- Establishing security boundaries between environments (e.g., blocking dev from accessing prod)
- Implementing zero-trust security models with selective namespace blocking
- Enforcing regulatory compliance by preventing access from non-compliant workloads
- Creating DMZs or quarantine zones in multi-tenant clusters

This approach gives you fine-grained control to explicitly block traffic from specific namespaces while still allowing communication from approved sources.

## Prerequisites

- A Kubernetes cluster with Cilium CNI installed
- kubectl command-line tool configured to interact with your cluster
- Basic understanding of Kubernetes networking concepts and namespaces

## Two Approaches to Namespace-Specific Denial

We'll explore two different methods to deny traffic from specific namespaces:

1. **Direct Namespace Denial** - Using `ingressDeny` with namespace selector
2. **Allowlist Approach** - Using `ingress` with a NotIn selector

Both approaches achieve similar outcomes but have subtle differences in behavior and flexibility.

## Implementation Steps with Real Outputs

Below is a detailed walkthrough of implementing namespace-specific denial policies, including actual commands executed and outputs observed.

### Step 1: Set Up Test Environment

First, let's create test namespaces and deploy pods in each:

```bash
# Create test namespaces
kubectl create namespace policy-test
kubectl create namespace other-namespace

# Deploy pods in the policy-test namespace
kubectl run web --image=nginx -n policy-test
kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600

# Deploy a pod in the other-namespace
kubectl run external-client --image=nicolaka/netshoot -n other-namespace -- sleep 3600

# Wait for pods to be ready
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
kubectl wait --for=condition=Ready pod/external-client -n other-namespace --timeout=60s
```

Let's check our pods:

```bash
kubectl get pods -n policy-test -o wide
```

**Output:**
```
NAME     READY   STATUS    RESTARTS   AGE     IP             NODE               NOMINATED NODE   READINESS GATES
client   1/1     Running   0          5m44s   10.244.1.105   cluster-2-worker   <none>           <none>
web      1/1     Running   0          5m44s   10.244.1.218   cluster-2-worker   <none>           <none>
```

```bash
kubectl get pods -n other-namespace -o wide
```

**Output:**
```
NAME              READY   STATUS    RESTARTS   AGE     IP             NODE                NOMINATED NODE   READINESS GATES
external-client   1/1     Running   0          5m38s   10.244.2.186   cluster-2-worker2   <none>           <none>
```

### Step 2: Test Baseline Connectivity

Let's check connectivity between all pods before applying any policies:

```bash
# Get the web pod's IP
WEB_POD_IP=$(kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}') && echo "Web Pod IP: $WEB_POD_IP"
```

**Output:**
```
Web Pod IP: 10.244.1.218
```

Testing connectivity from same namespace:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.
64 bytes from 10.244.1.218: icmp_seq=1 ttl=63 time=0.311 ms
64 bytes from 10.244.1.218: icmp_seq=2 ttl=63 time=0.073 ms
64 bytes from 10.244.1.218: icmp_seq=3 ttl=63 time=0.057 ms

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2087ms
rtt min/avg/max/mdev = 0.057/0.147/0.311/0.116 ms
```

Testing connectivity from other-namespace:

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

With no policies applied, both pods can communicate with the web pod.

### Step 3: Approach 1 - Direct Namespace Denial

#### Creating the Direct Deny Policy

Create a file named `deny-namespace-policy.yaml` with the following content:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-other-namespace-traffic"
spec:
  description: "Deny traffic specifically from other-namespace"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        io.kubernetes.pod.namespace: other-namespace  # This specifically denies pods from other-namespace
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: We use this rather than the namespaced policy for more reliable enforcement
- **endpointSelector**: Targets pods with the label `run: web` (our web server)
- **ingressDeny**: Explicitly denies traffic from pods in the `other-namespace` namespace
- **Namespace Selector**: Uses the automatically-applied `io.kubernetes.pod.namespace` label

Apply the policy:

```bash
kubectl apply -f deny-namespace-policy.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/deny-other-namespace-traffic created
```

#### Testing Direct Deny Policy

Test connectivity from same namespace:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output (unexpected result - connection failed):**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 0 received, 100% packet loss, time 2045ms

command terminated with exit code 1
```

This is an interesting finding! Even though we only specified to deny traffic from `other-namespace`, traffic from `policy-test` namespace is also blocked. This is consistent with our findings from the deny-all policy test, where Cilium's deny policies behave more restrictively than expected.

Let's delete this policy and try a different approach:

```bash
kubectl delete ciliumclusterwidenetworkpolicies deny-other-namespace-traffic
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io "deny-other-namespace-traffic" deleted
```

### Step 4: Approach 2 - Allowlist with NotIn Operator

Since we discovered that the direct deny approach blocks all traffic, we'll use an alternative approach - an allow policy with a NotIn operator to exclude specific namespaces.

Create a file named `deny-specific-namespace-policy.yaml` with the following content:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-specific-namespace"
spec:
  description: "Allow all traffic except from other-namespace"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: io.kubernetes.pod.namespace
        operator: NotIn
        values:
        - other-namespace
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: For cluster-wide enforcement
- **endpointSelector**: Targets pods with the label `run: web` (our web server)
- **ingress rule with matchExpressions**: Uses the `NotIn` operator to specify namespaces to exclude
- **NotIn operator**: A Kubernetes label selector that matches everything EXCEPT the specified values

Apply the policy:

```bash
kubectl apply -f deny-specific-namespace-policy.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/deny-specific-namespace created
```

#### Testing the NotIn Policy

Test connectivity from same namespace:

```bash
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.
64 bytes from 10.244.1.218: icmp_seq=1 ttl=63 time=0.231 ms
64 bytes from 10.244.1.218: icmp_seq=2 ttl=63 time=0.093 ms
64 bytes from 10.244.1.218: icmp_seq=3 ttl=63 time=0.136 ms

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2061ms
rtt min/avg/max/mdev = 0.093/0.153/0.231/0.057 ms
```

Test connectivity from other-namespace:

```bash
kubectl exec -n other-namespace external-client -- ping -c 3 $WEB_POD_IP
```

**Output:**
```
PING 10.244.1.218 (10.244.1.218) 56(84) bytes of data.

--- 10.244.1.218 ping statistics ---
3 packets transmitted, 0 received, 100% packet loss, time 2025ms

command terminated with exit code 1
```

**Result Analysis**: Success! With the allowlist approach:

1. Traffic from pods in the policy-test namespace is allowed
2. Traffic from pods in the other-namespace is blocked
3. The policy correctly implements the namespace-specific denial we wanted

## How These Policies Work

### Understanding the Key Differences

1. **Direct Deny Approach**:
   - Uses `ingressDeny` field
   - Explicitly identifies traffic sources to block
   - Ended up blocking all traffic (unexpected behavior)

2. **Allowlist Approach**:
   - Uses standard `ingress` field with `matchExpressions` and `NotIn`
   - Explicitly identifies allowed sources (everything except blocked namespaces)
   - Works as expected, allowing selective namespace blocking

### The NotIn Operator

The `NotIn` operator is a powerful feature in Kubernetes label selectors. When used with the namespace label, it creates a policy that:

1. Matches all pods that DON'T have the namespace label value in the specified list
2. Effectively creates an allowlist for "everything except these namespaces"
3. Provides a more predictable behavior than direct deny policies

## Advanced Configurations

### Blocking Multiple Namespaces

To block traffic from multiple namespaces, simply add more values to the `NotIn` list:

```yaml
matchExpressions:
- key: io.kubernetes.pod.namespace
  operator: NotIn
  values:
  - other-namespace
  - another-blocked-namespace
  - third-blocked-namespace
```

### Combining with Protocol Restrictions

You can further refine the policy by adding protocol and port restrictions:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-specific-namespace-http-only"
spec:
  description: "Allow HTTP traffic except from other-namespace"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: io.kubernetes.pod.namespace
        operator: NotIn
        values:
        - other-namespace
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

This policy would only allow HTTP traffic (port 80 TCP) from namespaces other than `other-namespace`.

## Troubleshooting Common Issues

1. **Check Policy Validity**:
   ```bash
   kubectl get ciliumclusterwidenetworkpolicies
   ```
   Make sure the VALID column shows "True"

2. **Verify Namespace Labels**:
   ```bash
   kubectl get pod external-client -n other-namespace --show-labels
   ```
   Confirm the `io.kubernetes.pod.namespace: other-namespace` label is present

3. **Look for Policy Errors in Logs**:
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```
   Check for any policy-related errors

4. **When to Use Each Approach**:
   - Use the direct deny approach (with caution) when you want to explicitly deny specific traffic patterns
   - Use the allowlist approach (more reliable) when you want to block entire namespaces

5. **Common Gotchas**:
   - Remember that `ingressDeny` policies have unexpected behavior in current Cilium versions
   - Always test thoroughly after policy application
   - Consider the interaction with other existing policies

## Best Practices

1. **Start with permissive policies** then gradually restrict
2. **Test thoroughly** from multiple namespaces and with different protocols
3. **Use the NotIn operator** for more predictable behavior
4. **Document your namespace blocking strategy** for team awareness
5. **Consider using policy tiers** for more complex scenarios

## Cleanup

When finished testing, clean up resources:

```bash
# Delete the policy
kubectl delete ciliumclusterwidenetworkpolicies deny-specific-namespace

# Delete the test namespaces
kubectl delete namespace policy-test
kubectl delete namespace other-namespace
```

## Key Takeaways

Based on our implementation and testing, here are the key takeaways:

1. **NotIn Operator is Powerful**: It allows for effective namespace exclusion while still allowing other traffic
2. **Direct Deny Behaves Differently**: The `ingressDeny` approach blocks more traffic than expected
3. **Namespace Labels are Automatic**: Cilium automatically adds the `io.kubernetes.pod.namespace` label to every pod
4. **Testing is Critical**: Always verify policy behavior from multiple source namespaces
5. **CiliumClusterwideNetworkPolicy** provides the most reliable way to implement these policies

## Summary

This guide demonstrated how to implement network policies that block traffic from specific namespaces while allowing all other traffic. By using the allowlist approach with the NotIn operator, we achieved selective namespace blocking with predictable behavior.

This pattern is valuable for implementing security boundaries between environments, isolating untrusted workloads, and creating fine-grained network security in multi-tenant Kubernetes clusters.
