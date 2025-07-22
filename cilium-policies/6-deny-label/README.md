# Implementing Label-Specific Denial Policies with Cilium

This guide provides a step-by-step approach to implementing network policies in Kubernetes that reject traffic from pods with specific labels. We'll use Cilium's powerful label-based selectors to create precise security policies that deny access from specific workloads based on their labels.

## Introduction

Label-specific denial policies are valuable in several scenarios:
- Isolating sensitive workloads from specific untrusted services
- Creating security boundaries between application tiers
- Implementing zero-trust security models with selective blocking
- Enforcing traffic separation between components with different security classifications
- Establishing security boundaries for multi-tenant environments

This approach gives you fine-grained control to explicitly block traffic from specific workload types while still allowing communication from all other sources.

## Prerequisites

- A Kubernetes cluster with Cilium CNI installed
- kubectl command-line tool configured to interact with your cluster
- Basic understanding of Kubernetes networking concepts and labels

## Implementation Steps with Real Outputs

Below is a detailed walkthrough of implementing a label-specific denial policy, including actual commands executed and outputs observed.

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

Create test namespaces and deploy pods with different labels:

```bash
# Create test namespaces
kubectl create namespace test1
kubectl create namespace test2
```

**Output:**
```
namespace/test1 created
namespace/test2 created
```

Deploy target pods with the "red" label:

```bash
kubectl run red-pod1 --image=nginx --labels="app=red" -n test1
kubectl run red-pod2 --image=nginx --labels="app=red" -n test2
```

**Output:**
```
pod/red-pod1 created
pod/red-pod2 created
```

Deploy client pods with "green" labels (which we'll deny access from):

```bash
kubectl run green-pod1 --image=nicolaka/netshoot --labels="app=green" -n test1 -- sleep 3600
kubectl run green-pod2 --image=nicolaka/netshoot --labels="app=green" -n test2 -- sleep 3600
```

**Output:**
```
pod/green-pod1 created
pod/green-pod2 created
```

Deploy additional client pods with "blue" labels (which we'll allow access from):

```bash
kubectl run blue-pod1 --image=nicolaka/netshoot --labels="app=blue" -n test1 -- sleep 3600
kubectl run blue-pod2 --image=nicolaka/netshoot --labels="app=blue" -n test2 -- sleep 3600
```

**Output:**
```
pod/blue-pod1 created
pod/blue-pod2 created
```

Wait for all pods to be ready:

```bash
kubectl wait --for=condition=Ready pod/red-pod1 pod/green-pod1 pod/blue-pod1 -n test1 \
  pod/red-pod2 pod/green-pod2 pod/blue-pod2 -n test2 --timeout=60s
```

### Step 3: Test Baseline Connectivity (Before Policy)

Get the pod IP addresses:

```bash
kubectl get pod red-pod1 -n test1 -o jsonpath='{.status.podIP}' && echo " (red-pod1)"
kubectl get pod red-pod2 -n test2 -o jsonpath='{.status.podIP}' && echo " (red-pod2)"
```

**Output:**
```
10.244.1.87 (red-pod1)
10.244.1.230 (red-pod2)
```

Test connectivity from green pods to red pods:

```bash
kubectl exec -n test1 green-pod1 -- curl -s --max-time 5 10.244.1.87 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

Test connectivity from blue pods to red pods:

```bash
kubectl exec -n test1 blue-pod1 -- curl -s --max-time 5 10.244.1.87 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

As expected, in a cluster with no network policies, all pods can freely communicate regardless of labels. This confirms our baseline connectivity is working correctly.

### Step 4: Choose the Right Approach

For implementing label-specific denial policies, we'll demonstrate two approaches:

#### Approach 1: Direct Denial with ingressDeny (Not Recommended)

First, we tried creating a policy with `ingressDeny`:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-specific-label"
spec:
  description: "Deny traffic from pods with label 'green' to pods with label 'red'"
  endpointSelector:
    matchLabels:
      app: red
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        app: green
```

However, when we tested this, we found that **all** traffic to red pods was blocked, including from blue pods, even though the policy only specified denial from green pods. This is consistent with our findings from other deny policies, where Cilium's implementation of `ingressDeny` might be more restrictive than expected.

#### Approach 2: Allowlist with NotIn Operator (Recommended)

Instead, we'll use an alternative approach with the `NotIn` operator:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-specific-label-alt"
spec:
  description: "Allow traffic from all pods except those with label 'green'"
  endpointSelector:
    matchLabels:
      app: red
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: app
        operator: NotIn
        values:
        - green
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: For cluster-wide enforcement
- **endpointSelector**: Targets pods with the label `app: red`
- **ingress with matchExpressions**: Uses the `NotIn` operator to create an allowlist
- **NotIn operator**: A Kubernetes label selector that matches everything EXCEPT the specified values
- **No namespace specification**: Policy applies across all namespaces

### Step 5: Apply the Policy

Apply the policy:

```bash
kubectl apply -f deny-specific-label-policy-approach2.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/deny-specific-label-alt created
```

### Step 6: Test Connectivity After Policy Application

Now, let's test connectivity after applying the policy:

#### Test 1: Green pod to Red pod (should fail)

```bash
kubectl exec -n test1 green-pod1 -- curl -s --max-time 5 10.244.1.87
```

**Output:**
```
command terminated with exit code 28
```

The connection timed out, confirming that traffic from green pods is now blocked.

#### Test 2: Blue pod to Red pod (should succeed)

```bash
kubectl exec -n test1 blue-pod1 -- curl -s --max-time 5 10.244.1.87 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

The connection succeeded, confirming that traffic from blue pods is still allowed.

#### Test 3: Cross-namespace tests

Let's also test the cross-namespace behavior:

```bash
kubectl exec -n test2 green-pod2 -- curl -s --max-time 5 10.244.1.87
```

**Output:**
```
command terminated with exit code 28
```

```bash
kubectl exec -n test2 blue-pod2 -- curl -s --max-time 5 10.244.1.87 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

This confirms that the policy behaves consistently across namespaces, blocking green pods and allowing blue pods regardless of which namespace they're in.

**Result Analysis**: Our policy is working as intended:

1. Traffic from pods with label `app=green` is blocked, regardless of namespace
2. Traffic from pods with different labels (e.g., `app=blue`) is allowed
3. The policy applies consistently across namespaces
4. Using the `NotIn` operator gives us more predictable behavior than `ingressDeny`

## How Label-Specific Denial Works

### The NotIn Operator

The key to making our policy work correctly is the `NotIn` operator, which is a powerful feature in Kubernetes label selectors:

```yaml
matchExpressions:
- key: app
  operator: NotIn
  values:
  - green
```

This expression creates an allowlist that matches:
1. Pods that don't have the `app` label at all
2. Pods that have the `app` label with any value other than "green"

The `NotIn` operator is more reliable than using `ingressDeny` because it expresses the policy as "allow everything except..." rather than trying to explicitly deny specific traffic.

### Cross-Namespace Functionality

By not specifying a namespace in the `fromEndpoints` selector, our policy automatically works across all namespaces. This makes the policy more powerful and easier to maintain as your cluster grows.

### Zero-Trust Model

This approach fits nicely into a zero-trust security model because it:

1. Explicitly defines what traffic is allowed (everything except from green pods)
2. Applies the rule consistently regardless of namespace boundaries
3. Blocks traffic based on workload identity (labels) rather than network location

## Advanced Configurations

### Blocking Multiple Labels

To block traffic from multiple different labels, simply add more values to the `NotIn` list:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "block-multiple-labels"
spec:
  description: "Allow traffic except from specified labels"
  endpointSelector:
    matchLabels:
      app: red
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: app
        operator: NotIn
        values:
        - green
        - yellow
        - orange
```

### Combining with Namespace Constraints

If you want to block specific labels but only within certain namespaces:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-label-in-namespace"
spec:
  description: "Block green-labeled pods in the prod namespace"
  endpointSelector:
    matchLabels:
      app: red
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: app
        operator: NotIn
        values:
        - green
      matchLabels:
        io.kubernetes.pod.namespace: prod
```

### Adding Protocol Restrictions

You can further refine the policy by adding protocol and port restrictions:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-label-http-only"
spec:
  description: "Block green-labeled pods for HTTP traffic only"
  endpointSelector:
    matchLabels:
      app: red
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: app
        operator: NotIn
        values:
        - green
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

## Troubleshooting Common Issues

1. **Verify Policy Validity**:
   ```bash
   kubectl get ciliumclusterwidenetworkpolicies
   ```
   Make sure the VALID column shows "True"

2. **Check Pod Labels**:
   ```bash
   kubectl get pods -n test1 --show-labels
   ```
   Ensure the labels match exactly what's specified in your policy

3. **Test with Different Traffic Types**:
   If HTTP traffic is blocked but other protocols work, it might be a port-specific issue

4. **Common Pitfalls**:
   - Remember that `ingressDeny` might have unexpected behavior
   - Label case sensitivity matters (`app: Green` and `app: green` are different)
   - Ensure your `NotIn` operator is correctly formatted

5. **Check Cilium Logs**:
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```

## Best Practices

1. **Prefer NotIn Over ingressDeny**: As shown in our testing, the `NotIn` operator gives more predictable behavior
2. **Test Thoroughly**: Always test both the traffic you want to block and the traffic you want to allow
3. **Be Specific**: Only block the specific labels you need to, rather than broad patterns
4. **Document Blocked Labels**: Keep clear documentation of which labels are blocked and why
5. **Consider Defense in Depth**: Layer multiple policies for critical workloads

## Cleanup

When finished testing, clean up resources:

```bash
# Delete the policy
kubectl delete ciliumclusterwidenetworkpolicies deny-specific-label-alt

# Delete the test namespaces
kubectl delete namespace test1 test2
```

## Key Takeaways

Based on our implementation and testing, here are the key takeaways:

1. **NotIn Operator is More Reliable**: Using the `NotIn` operator is more predictable than using `ingressDeny` for blocking specific labels
2. **Cross-Namespace by Default**: Label-based policies work across namespace boundaries unless specifically restricted
3. **Zero Trust Building Block**: This approach can be a key component in a zero-trust security model
4. **Defense in Depth**: Can be combined with other policy types for comprehensive protection

## Summary

This guide demonstrated how to implement network policies that reject traffic based on specific pod labels. By using the `NotIn` operator with label-based selectors, we can create precise security boundaries that align with application components rather than infrastructure boundaries.

This pattern is particularly powerful for isolating sensitive workloads from specific untrusted services while maintaining connectivity with everything else, providing a targeted security posture that doesn't disrupt legitimate traffic flows.
