# Implementing Label-Based Network Policy with Cilium

This guide provides a step-by-step approach to implementing network policies in Kubernetes that allow traffic between pods based on shared labels, regardless of their namespace. We'll use Cilium's powerful label-based selectors to create precise, label-driven security policies.

## Introduction

Label-based network policies are essential in several scenarios:
- Implementing microservices architectures where services need to communicate based on their function
- Creating security boundaries based on application components rather than namespaces
- Enabling cross-namespace communication for specific application tiers
- Supporting multi-tenant environments with shared services
- Implementing zero-trust security models with fine-grained access control

This approach ensures pods can only communicate with other pods sharing the same labels, creating strong security boundaries based on application roles rather than namespace boundaries.

## Prerequisites

- A Kubernetes cluster with Cilium CNI installed
- kubectl command-line tool configured to interact with your cluster
- Basic understanding of Kubernetes networking concepts and labels

## Implementation Steps with Real Outputs

Below is a detailed walkthrough of implementing a label-based policy, including actual commands executed and outputs observed.

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
kubectl create namespace ns1
kubectl create namespace ns2
```

**Output:**
```
namespace/ns1 created
namespace/ns2 created
```

Deploy API pods with the same label (`app=api`) in both namespaces:

```bash
kubectl run api-pod1 --image=nginx --labels="app=api" -n ns1
kubectl run api-pod2 --image=nginx --labels="app=api" -n ns2
```

**Output:**
```
pod/api-pod1 created
pod/api-pod2 created
```

Deploy web pods with a different label (`app=web`):

```bash
kubectl run web-pod1 --image=nginx --labels="app=web" -n ns1
kubectl run web-pod2 --image=nginx --labels="app=web" -n ns2
```

**Output:**
```
pod/web-pod1 created
pod/web-pod2 created
```

Deploy client pods with another label set (`role=client`):

```bash
kubectl run client1 --image=nicolaka/netshoot --labels="role=client" -n ns1 -- sleep 3600
kubectl run client2 --image=nicolaka/netshoot --labels="role=client" -n ns2 -- sleep 3600
```

**Output:**
```
pod/client1 created
pod/client2 created
```

Also deploy client pods with the API label to demonstrate label-based connectivity:

```bash
kubectl run api-client1 --image=nicolaka/netshoot --labels="app=api" -n ns1 -- sleep 3600
kubectl run api-client2 --image=nicolaka/netshoot --labels="app=api" -n ns2 -- sleep 3600
```

**Output:**
```
pod/api-client1 created
pod/api-client2 created
```

Wait for all pods to be ready:

```bash
kubectl wait --for=condition=Ready pod/api-pod1 pod/web-pod1 pod/client1 pod/api-client1 -n ns1 \
  pod/api-pod2 pod/web-pod2 pod/client2 pod/api-client2 -n ns2 --timeout=60s
```

### Step 3: Test Baseline Connectivity (Before Policy)

Get the API pods' IP addresses:

```bash
kubectl get pod api-pod1 -n ns1 -o jsonpath='{.status.podIP}' && echo " (api-pod1)"
kubectl get pod api-pod2 -n ns2 -o jsonpath='{.status.podIP}' && echo " (api-pod2)"
```

**Output:**
```
10.244.1.253 (api-pod1)
10.244.1.117 (api-pod2)
```

Test connectivity from regular clients to API pods:

```bash
kubectl exec -n ns1 client1 -- curl -s --max-time 5 10.244.1.253 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

```bash
kubectl exec -n ns2 client2 -- curl -s --max-time 5 10.244.1.117 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

Test connectivity from API-labeled clients:

```bash
kubectl exec -n ns1 api-client1 -- curl -s --max-time 5 10.244.1.117 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

As expected, in a cluster with no network policies, all pods can freely communicate regardless of labels or namespaces. This confirms our baseline connectivity is working correctly.

### Step 4: Create the Label-Based Policy

Create a file named `label-based-policy.yaml` with the following content:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "same-label-traffic"
spec:
  description: "Allow traffic only between pods with matching labels"
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: api
```

This policy has several important elements:

- **CiliumClusterwideNetworkPolicy**: We use this rather than the namespaced policy for more reliable enforcement
- **endpointSelector**: Targets pods with the label `app: api` (our API servers and clients)
- **ingress rule with label selector**: Allows traffic only from pods with the same `app: api` label
- **No namespace specification**: By not specifying a namespace, the policy works across all namespaces
- **No port/protocol specifications**: By omitting `toPorts`, we allow ALL traffic types (TCP, UDP, ICMP, etc.)

### Step 5: Apply the Policy

Apply the policy:

```bash
kubectl apply -f label-based-policy.yaml
```

**Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/same-label-traffic created
```

Verify the policy was created and is valid:

```bash
kubectl get ciliumclusterwidenetworkpolicies same-label-traffic
```

**Output:**
```
NAME                 VALID
same-label-traffic   True
```

The `VALID: True` status confirms that Cilium has successfully processed and activated our policy.

### Step 6: Test Connectivity After Policy Application

Let's verify that the policy only permits communication between pods with the same label, regardless of namespace:

#### Test 1: Regular client to API pod (should fail)

```bash
kubectl exec -n ns1 client1 -- curl -s --max-time 5 10.244.1.253
```

**Output:**
```
command terminated with exit code 28
```

The connection timed out because the client1 pod (with label `role=client`) is not allowed to communicate with api-pod1 (with label `app=api`).

#### Test 2: API client to API pod in same namespace (should succeed)

```bash
kubectl exec -n ns1 api-client1 -- curl -s --max-time 5 10.244.1.253 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

The connection succeeds because both pods have the same `app=api` label.

#### Test 3: API client to API pod in different namespace (should succeed)

```bash
kubectl exec -n ns1 api-client1 -- curl -s --max-time 5 10.244.1.117 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

The connection succeeds despite being in different namespaces, demonstrating that the policy works based on labels rather than namespace boundaries.

#### Test 4: Regular client in different namespace to API pod (should fail)

```bash
kubectl exec -n ns2 client2 -- curl -s --max-time 5 10.244.1.117
```

**Output:**
```
command terminated with exit code 28
```

Again, the connection times out due to label mismatch.

#### Test 5: API client to API pod across namespaces (should succeed)

```bash
kubectl exec -n ns2 api-client2 -- curl -s --max-time 5 10.244.1.253 | head -n 5
```

**Output:**
```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
```

This confirms cross-namespace communication works as long as the labels match.

**Result Analysis**: The tests confirm that our label-based policy is working as intended:

1. Traffic is allowed between pods with matching `app=api` labels, regardless of namespace
2. Traffic is blocked from all pods without the `app=api` label
3. Both same-namespace and cross-namespace communication works when labels match
4. The policy applies consistently for all network traffic

### Step 7: Understanding the Policy

Let's examine the details of the policy:

```bash
kubectl describe ciliumclusterwidenetworkpolicy same-label-traffic
```

**Output:**
```
Name:         same-label-traffic
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  cilium.io/v2
Kind:         CiliumClusterwideNetworkPolicy
Metadata:
  Creation Timestamp:  2025-07-18T20:24:46Z
  Generation:          1
  Resource Version:    40877
  UID:                 c6b14a83-f277-4fdf-b3ad-d56e9fbfdbb4
Spec:
  Description:  Allow traffic only between pods with matching labels
  Endpoint Selector:
    Match Labels:
      App:  api
  Ingress:
    From Endpoints:
      Match Labels:
        App:  api
Status:
  Conditions:
    Last Transition Time:  2025-07-18T20:24:46Z
    Message:               Policy validation succeeded
    Status:                True
    Type:                  Valid
Events:                    <none>
```

## How Label-Based Policies Work

### Key Concepts

1. **Label-Based Selection**: Cilium policies use Kubernetes label selectors to identify both the target pods (`endpointSelector`) and the allowed sources (`fromEndpoints`).

2. **Cross-Namespace by Default**: When you don't specify a namespace label, the policy applies across all namespaces, allowing same-label communication regardless of namespace boundaries.

3. **Zero Trust Approach**: By default, once a pod has a policy targeting it (via `endpointSelector`), all ingress traffic is denied except what's explicitly allowed.

4. **Label Inheritance**: The policy doesn't change or modify pod labels - it simply uses the existing labels to make traffic decisions.

### Advantages of Label-Based Policies

1. **Service-Oriented Security**: Aligns network security with service architecture rather than infrastructure layout
2. **Portable Policies**: Works the same regardless of namespace structure or IP addressing
3. **Reduced Policy Count**: One policy can secure many pods across different namespaces
4. **Simplified Microservices Security**: Natural fit for microservices architecture

## Advanced Configurations

### Adding Namespace Constraints

If you want to allow same-label communication but only within specific namespaces, you can add a namespace selector:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "same-label-in-production"
spec:
  description: "Allow traffic between pods with matching labels only in production"
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: api
        io.kubernetes.pod.namespace: production
```

### Multiple Label Requirements

You can specify multiple label requirements to make more complex policies:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "multi-label-traffic"
spec:
  description: "Allow traffic from frontend api to backend api"
  endpointSelector:
    matchLabels:
      app: api
      tier: backend
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: api
        tier: frontend
```

### Combining with Protocol Restrictions

You can further refine the policy by adding protocol and port restrictions:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "api-http-only"
spec:
  description: "Allow HTTP traffic only between api components"
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: api
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

## Troubleshooting Common Issues

1. **Check Policy Validity**:
   ```bash
   kubectl get ciliumclusterwidenetworkpolicies
   ```
   Make sure the VALID column shows "True"

2. **Verify Pod Labels**:
   ```bash
   kubectl get pods -n ns1 --show-labels
   ```
   Ensure the labels match exactly what's specified in your policy

3. **Look for Policy Errors in Logs**:
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```
   Check for any policy-related errors

4. **Label Case Sensitivity**:
   Remember that Kubernetes labels are case-sensitive. `app: api` and `App: api` are different labels.

5. **Test Bidirectional Communication**:
   If pods need to communicate bidirectionally, ensure that both pods have matching labels and are covered by the policy.

## Best Practices

1. **Use Consistent Labeling**: Establish a clear labeling scheme for your applications
2. **Start with Broader Policies**: Begin with more permissive policies, then gradually restrict as needed
3. **Test Thoroughly**: Always test with pods in different namespaces to ensure cross-namespace behavior is as expected
4. **Combine with Namespace Policies**: Use label-based policies alongside namespace-based policies for defense in depth
5. **Document Your Label Schema**: Maintain clear documentation of your label schema for security teams

## Cleanup

When finished testing, clean up resources:

```bash
# Delete the policy
kubectl delete ciliumclusterwidenetworkpolicies same-label-traffic

# Delete the test namespaces
kubectl delete namespace ns1 ns2
```

## Key Takeaways

Based on our implementation and testing, here are the key takeaways:

1. **Labels Transcend Namespaces**: Label-based policies work across namespace boundaries by default
2. **Precise Traffic Control**: You can achieve fine-grained access control based on application roles
3. **Service-Oriented Security**: Aligns security boundaries with service boundaries rather than infrastructure
4. **Multiple Selection Criteria**: Can combine labels, namespaces, and network protocols for comprehensive policies
5. **Zero Trust Foundation**: Provides a building block for implementing zero-trust security models

## Summary

This guide demonstrated how to implement network policies that permit traffic between pods with matching labels, regardless of which namespace they're deployed in. This approach is particularly powerful for microservices architectures, where services often need to communicate with counterparts in different namespaces based on their function rather than their location.

By using label-based policies, you can create security boundaries that naturally align with your application architecture, making your security posture more intuitive and maintainable as your application evolves.
