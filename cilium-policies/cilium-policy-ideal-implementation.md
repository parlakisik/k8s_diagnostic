# Cilium Network Policy: Ideal Implementation Guide

This document outlines the ideal approach for implementing Kubernetes network policies with Cilium, based on our previous testing and experiments. This approach focuses on using CiliumClusterwideNetworkPolicy, which proved to be the most reliable method.

## 1. Environment Setup

First, let's set up a clean testing environment:

```bash
# Create a dedicated namespace for testing
kubectl create namespace policy-test

# Deploy a web server pod
kubectl run web --image=nginx -n policy-test

# Deploy a client pod with networking tools
kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600

# Wait for pods to be ready
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
```

## 2. Baseline Connectivity Testing

Before applying any policies, we need to verify that the pods can communicate:

```bash
# Get web pod IP
WEB_POD_IP=$(kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}')
echo "Web Pod IP: $WEB_POD_IP"

# Test ping connectivity
kubectl exec -n policy-test client -- ping -c 3 $WEB_POD_IP

# Test HTTP connectivity
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

## 3. Policy Implementation

### Option 1: Deny Traffic

Let's implement a policy that denies traffic from the client to the web pod:

```yaml
# deny-policy.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-client-to-web"
spec:
  description: "Deny traffic from client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        run: client
```

Apply and test the policy:

```bash
kubectl apply -f deny-policy.yaml

# Verify policy is valid
kubectl get ciliumclusterwidenetworkpolicies

# Test connectivity (should be blocked)
kubectl exec -n policy-test client -- ping -c 3 -w 5 $WEB_POD_IP
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

### Option 2: Allow Specific Traffic

Let's implement a policy that allows only HTTP traffic from the client to the web pod:

```yaml
# allow-http-policy.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "allow-client-to-web-http"
spec:
  description: "Allow HTTP traffic from client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

Apply and test the policy:

```bash
# Remove the deny policy
kubectl delete -f deny-policy.yaml

# Apply the allow policy
kubectl apply -f allow-http-policy.yaml

# Verify policy is valid
kubectl get ciliumclusterwidenetworkpolicies

# Test ping connectivity (should be blocked)
kubectl exec -n policy-test client -- ping -c 3 -w 5 $WEB_POD_IP

# Test HTTP connectivity (should be allowed)
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

### Option 3: Combined Policies (Demonstrating Precedence)

Let's implement both allow and deny policies to demonstrate that deny takes precedence:

```bash
# Apply both policies
kubectl apply -f deny-policy.yaml

# Verify both policies are valid
kubectl get ciliumclusterwidenetworkpolicies

# Test connectivity (should be blocked due to deny precedence)
kubectl exec -n policy-test client -- ping -c 3 -w 5 $WEB_POD_IP
kubectl exec -n policy-test client -- curl -s --max-time 5 $WEB_POD_IP
```

## 4. Clean Up

```bash
# Delete the policies
kubectl delete -f deny-policy.yaml
kubectl delete -f allow-http-policy.yaml

# Delete the namespace
kubectl delete namespace policy-test
```

## 5. Key Takeaways

1. **Use CiliumClusterwideNetworkPolicy** - These are more reliable than namespace-scoped policies
2. **Explicit Deny Rules** - Using ingressDeny provides better control than empty ingress arrays
3. **Deny Takes Precedence** - Deny policies override allow policies when both apply to the same traffic
4. **Label-Based Selection** - Always use precise label selectors for targeting pods
5. **Validate Policies** - Always check that policies are valid after applying them

This approach ensures consistent network policy enforcement and avoids the issues we encountered in our earlier experiments.
