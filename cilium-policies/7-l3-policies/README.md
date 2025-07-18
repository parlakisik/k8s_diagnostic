# Comprehensive Cilium L3 Network Policies Guide

This document provides a consolidated reference for all Layer 3 (L3) network policies in Cilium, including test implementations and validation results.

## Overview of Cilium L3 Network Policies

Cilium supports several types of L3 network policies, each targeting different aspects of network communication:

1. **Label-based Policies**: Filter traffic based on pod labels
2. **Entity-based Policies**: Use predefined entities like "cluster", "host", etc.
3. **Node-based Policies**: Restrict traffic based on node identity
4. **CIDR-based Policies**: Control traffic using IP address ranges
5. **DNS-based Policies**: Manage egress traffic to specific domain names
6. **Service-based Policies**: Control traffic to Kubernetes Services

## 1. Label-Based Policies

Label-based policies are the most common type and form the foundation of Cilium's network policy model.

### Implementation Example:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "label-based-policy"
  namespace: test-namespace
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: frontend
```

### Test Results:

✅ **Behavior**: Traffic from pods with the label `app: frontend` is allowed to pods with the label `app: api`.  
✅ **Validation**: All other traffic to the target pods is blocked.

## 2. Entity-Based Policies

Entity-based policies use predefined entities for greater abstraction.

### Implementation Example:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "l3-entities-policy"
  namespace: entities-test
spec:
  description: "Allow ingress traffic from cluster entities"
  endpointSelector:
    matchLabels:
      app: web
  ingress:
  - fromEntities:
    - "cluster"
```

### Test Results:

✅ **Behavior**: Traffic from any pod in the cluster is allowed to pods with the label `app: web`.  
✅ **Validation**: The policy correctly allows all cluster traffic regardless of source pod location.

## 3. Node-Based Policies

Node-based policies filter traffic based on which node the source pods are running on.

### Implementation Approaches

There are three distinct ways to implement node-based policies in Cilium:

#### Option 1: Using `fromNodes` Selector (Requires Configuration Change)

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "traditional-node-selector"
  namespace: node-test
spec:
  description: "Allow traffic from cluster-2-worker only"
  endpointSelector:
    matchLabels:
      app: database
  ingress:
  - fromNodes:
    - matchLabels:
        kubernetes.io/hostname: cluster-2-worker
```

**⚠️ Important Configuration Requirement:**

For the `fromNodes` selector to work, Cilium must be configured with:
```
enable-node-selector-labels: "true"
```

You can verify your current configuration with:
```bash
kubectl -n kube-system get configmap cilium-config -o yaml | grep enable-node-selector-labels
```

If this is set to `false` (the default), you will see this error in the logs:
```
Unable to add CiliumNetworkPolicy: Invalid CiliumNetworkPolicy spec: 
FromNodes/ToNodes rules can only be applied when the "enable-node-selector-labels" flag is set
```

#### Option 2: Using Pod Node Name Label (Works with Default Configuration)

This approach uses the `io.kubernetes.pod.nodeName` label that Kubernetes automatically adds to every pod:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "pod-nodename-selector"
spec:
  description: "Allow traffic only from pods on specific node"
  endpointSelector:
    matchLabels:
      app: database
      k8s:io.kubernetes.pod.namespace: node-policy-test
  ingress:
  - fromEndpoints:
    - matchExpressions:
      - key: k8s:io.kubernetes.pod.nodeName
        operator: In
        values:
        - cluster-2-worker
      matchLabels:
        k8s:io.kubernetes.pod.namespace: node-policy-test
```

#### Option 3: Using CIDR Ranges for Node Pod Subnets

This approach uses the CIDR ranges that correspond to the pod subnets on specific nodes:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "cidr-node-selector"
  namespace: node-policy-test
spec:
  description: "Allow traffic only from pods on cluster-2-worker node using CIDR range"
  endpointSelector:
    matchLabels:
      app: database
  ingress:
  - fromCIDR:
    - "10.244.1.0/24"  # This CIDR covers all pods on the cluster-2-worker node
```

### Test Results

We tested all three approaches in a cluster where `enable-node-selector-labels` was set to `false`:

1. **Using `fromNodes`**: ❌ Failed with configuration error
2. **Using Pod Node Name Label**: ✅ Successfully filtered traffic based on source node
3. **Using CIDR Ranges**: ✅ Successfully filtered traffic based on source node IP range

### Best Practices for Node-Based Policies

1. **Check your Cilium configuration** first to determine which approach to use
2. **Use Option 2 (Pod Node Name Label)** if you can't modify the Cilium configuration
3. **Combine with namespace selectors** for more precise control
4. **Use ClusterWide policies** when filtering across namespaces

## 4. CIDR-Based Policies

CIDR-based policies control traffic using IP address ranges.

### Implementation Example:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "cidr-ingress-policy"
  namespace: test-namespace
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromCIDR:
    - "10.244.1.0/24"
```

### Test Results:

✅ **Behavior**: Only traffic from IP addresses in the specified CIDR range is allowed.  
✅ **Validation**: Traffic from other IP ranges is effectively blocked.

### Advanced Example with CIDR Exceptions:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "cidr-with-except-policy"
  namespace: test-namespace
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromCIDR:
    - "10.244.0.0/16"
    fromCIDRSet:
    - cidr: "10.244.2.0/24"
      except:
      - "10.244.2.100/32"
```

## 5. DNS-Based Policies

DNS-based policies allow egress traffic control based on domain names.

### Implementation Example:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "dns-egress-policy"
  namespace: test-namespace
spec:
  endpointSelector:
    matchLabels:
      app: frontend
  egress:
  - toFQDNs:
      matchNames:
      - "api.example.com"
      - "*.api.example.org"
    toPorts:
    - ports:
      - port: "443"
        protocol: TCP
```

### Test Results:

✅ **Behavior**: Egress traffic is allowed only to the specified domain names.  
✅ **Validation**: Requests to other domains are blocked.

## 6. Service-Based Policies

Service-based policies control traffic to Kubernetes Services.

### Implementation Example:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "service-based-policy"
  namespace: test-namespace
spec:
  endpointSelector:
    matchLabels:
      app: frontend
  egress:
  - toServices:
    - k8sService:
        serviceName: api-service
        namespace: api-namespace
    toPorts:
    - ports:
      - port: "8080"
        protocol: TCP
```

### Test Results:

✅ **Behavior**: Traffic to the specified service is allowed.  
✅ **Validation**: Traffic to other services is blocked.

## Practical Implementation Tips

1. **Start with Deny-All Policy**: Begin with a default deny policy for all pods, then add specific allow rules
2. **Layer Policies**: Use multiple policy types together for comprehensive security
3. **Label Strategy**: Design a consistent labeling strategy for your workloads
4. **Monitor & Test**: Regularly test policy behavior with tools like `kubectl exec` and `curl`
5. **Node Configuration**: For node-based policies, be aware of the Cilium configuration requirements

## Troubleshooting Common Issues

1. **Policy Not Applied**: Check if the policy is correctly targeting the intended pods with matching labels
2. **Connection Timeouts**: This often indicates the policy is blocking traffic as expected
3. **Node-Based Policy Issues**: 
   - For `fromNodes` selector errors, verify the Cilium configuration has `enable-node-selector-labels: "true"`
   - For pod nodeName selector issues, check if pods have the correct `io.kubernetes.pod.nodeName` label
   - For CIDR-based approaches, verify the CIDR ranges match your node's pod subnet
4. **DNS Resolution Problems**: For DNS-based policies, check if DNS interception is working correctly
5. **Service Selection Errors**: Verify service selectors match the actual Kubernetes services
6. **Policy Logs Analysis**: Inspect the Cilium agent logs for specific error messages:
   ```bash
   kubectl -n kube-system logs -l k8s-app=cilium --tail=50 | grep -E 'policy|error'
   ```

## Validation Methods

To validate network policies, use the following approaches:

1. **Direct Connectivity Testing**:
   ```bash
   kubectl exec -n namespace pod-name -- curl -s --max-time 5 target-ip:port
   ```

2. **Policy Status Check**:
   ```bash
   kubectl describe ciliumnetworkpolicies -n namespace policy-name
   ```

3. **Cilium Policy Logs**:
   ```bash
   kubectl -n kube-system logs -l k8s-app=cilium --tail=50 | grep -E 'policy|error'
   ```

4. **Pod Network Information**:
   ```bash
   kubectl get pods -n namespace -o wide
   ```
   This shows which nodes pods are running on and their IP addresses, which is crucial for node-based and CIDR-based policies.

5. **Pod Label Inspection**:
   ```bash
   kubectl get pod -n namespace pod-name -o jsonpath='{.metadata.labels}'
   ```
   This helps verify if pods have the expected labels for label-based policies.
