# Cilium Network Policy: Correct Implementation Guide

Based on our extensive testing with Cilium v1.17.5, this guide provides instructions for correctly implementing network policies while avoiding unexpected behaviors. We'll focus on practical, step-by-step guidance for both allow and deny policies.

## Understanding Cilium Policy Behavior

Our testing revealed critical insights into how Cilium policies actually behave:

1. When a pod is targeted by a deny policy's `endpointSelector`, **ALL** traffic to that pod is blocked by default, regardless of source labels.
2. The `fromEndpoints` selector in an `ingressDeny` rule does not limit which sources are affected.
3. Deny rules always take precedence over allow rules.

## Recommended Implementation Approach

### Step 1: Set Up Test Environment

Start with a clean test environment to properly validate policy behavior:

```bash
# Create test namespace
kubectl create namespace policy-test

# Deploy test pods
kubectl run web --image=nginx -n policy-test
kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600

# Wait for pods to be ready
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
```

### Step 2: Verify Baseline Connectivity

```bash
# Get web pod IP
WEB_POD_IP=$(kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}')
echo "Web Pod IP: $WEB_POD_IP"

# Test connectivity
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
kubectl exec -n policy-test client -- curl -s $WEB_POD_IP
```

### Step 3: For Allow-Only Approach (Recommended)

Given our findings, the most predictable approach is to implement allow policies without deny policies:

1. First, set up a default deny rule for the entire namespace:

```yaml
# default-deny-namespace.yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "default-deny-namespace"
  namespace: policy-test
spec:
  endpointSelector: {}  # Matches all pods in namespace
  ingress: []  # Empty ingress array = deny all ingress traffic
```

2. Then, add specific allow rules for needed connectivity:

```yaml
# allow-client-to-web.yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "allow-client-to-web"
  namespace: policy-test
spec:
  description: "Allow client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
```

Apply these policies:

```bash
kubectl apply -f default-deny-namespace.yaml
kubectl apply -f allow-client-to-web.yaml
```

This approach follows the principle of "deny by default, allow explicitly" which is more secure and predictable.

### Step 4: Using Deny Policies Correctly

If you must use deny policies, be aware that they will block ALL traffic to the targeted pod. The correct approach is:

1. Start with a baseline where all traffic is allowed.
2. Add deny policies with the understanding they will block ALL traffic to targeted pods.
3. Add explicit allow policies to restore needed connectivity.

```yaml
# deny-to-web.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-to-web"
spec:
  description: "Deny all traffic to web"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints: [{}]  # Empty = all sources
```

```yaml
# allow-specific-to-web.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "allow-specific-to-web"
spec:
  description: "Allow specific traffic to web"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: allowed-source
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

### Step 5: Verification Testing

Always test your policies with multiple source pods and different protocols:

```bash
# Create additional test pods with different labels
kubectl run test-pod --image=nicolaka/netshoot -n policy-test --labels="app=testing" -- sleep 3600
kubectl wait --for=condition=Ready pod/test-pod -n policy-test --timeout=60s

# Test connectivity from different sources
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
kubectl exec -n policy-test test-pod -- ping -c 3 $WEB_POD_IP
kubectl exec -n policy-test test-pod -- curl -s --max-time 5 $WEB_POD_IP
```

## Best Practices for Avoiding Unexpected Behavior

1. **Use Namespace Policies First**: Start with namespace-scoped policies before using cluster-wide policies.

2. **Prefer Allow over Deny**: Build policies using allow rules rather than deny rules for more predictable behavior.

3. **Test with Multiple Sources**: Always test with different source pods to verify policy behavior.

4. **Layer Policies Carefully**: Remember that deny takes precedence over allow.

5. **Verify Before and After**: Test connectivity before applying policies and after to confirm expected behavior.

6. **Check Policy Validity**: Always verify that policies are valid after applying:
   ```bash
   kubectl get ciliumnetworkpolicies -n policy-test
   kubectl get ciliumclusterwidenetworkpolicies
   ```

7. **Monitor Policy Events**: Watch Cilium logs for policy-related events:
   ```bash
   kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system | grep -i policy
   ```

## Example Correct Implementation Workflow

Here's a complete workflow for correctly implementing Cilium network policies:

1. **Start with documentation**: Review the Cilium documentation while understanding its actual behavior may differ.

2. **Create test environment**: Set up pods with different labels for testing.

3. **Establish baseline**: Confirm all pods can communicate before applying policies.

4. **Apply minimally**: Start with the simplest policy that achieves your goal.

5. **Test thoroughly**: Test from all relevant source pods and with different protocols.

6. **Add incrementally**: Add more complex policies one at a time, testing after each addition.

7. **Document exceptions**: Keep track of any unexpected behaviors for future reference.

## Common Errors and Solutions

| Error | Solution |
|-------|----------|
| Traffic blocked when it should be allowed | Add explicit allow rules that take source labels into account |
| Traffic allowed when it should be denied | Remember deny rules affect ALL traffic to the target, regardless of source |
| Policy shows valid but doesn't work | Check Cilium agent logs and ensure there are no conflicts with other policies |
| Inconsistent behavior after policy change | Restart the Cilium daemon: `kubectl rollout restart ds/cilium -n kube-system` |

By following these guidelines and understanding Cilium's actual behavior patterns, you can implement network policies that reliably enforce your security boundaries without unexpected connectivity issues.
