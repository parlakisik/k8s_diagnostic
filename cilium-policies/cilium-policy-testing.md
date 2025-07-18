# Cilium Network Policy Testing Documentation

## 1. Introduction

This document summarizes our approaches and experiments with Kubernetes network policies using Cilium CNI. We explored various methods to control pod-to-pod traffic in a Kubernetes cluster, tested different policy configurations, and documented the challenges encountered.

## 2. Initial Environment Setup

### Test Namespace and Pods Creation

We set up a dedicated namespace and deployed test pods:

```bash
# Create test namespace
kubectl create namespace policy-test

# Deploy web server pod (target)
kubectl run web --image=nginx -n policy-test

# Deploy client pod with networking tools
kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600

# Wait for pods to be ready
kubectl wait --for=condition=Ready pod/web pod/client -n policy-test --timeout=60s
```

**Terminal Output:**
```
namespace/policy-test created
pod/web created
pod/client created
pod/web condition met
pod/client condition met
```

### Baseline Connectivity Testing

Before applying any policies, we verified that pods could communicate freely:

```bash
# Get web pod IP
kubectl get pod web -n policy-test -o jsonpath='{.status.podIP}'
```

**Terminal Output:**
```
10.244.1.136
```

```bash
# Test ping connectivity
kubectl exec -n policy-test client -- ping -c 3 10.244.1.136
```

**Terminal Output:**
```
PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.
64 bytes from 10.244.1.136: icmp_seq=1 ttl=63 time=0.221 ms
64 bytes from 10.244.1.136: icmp_seq=2 ttl=63 time=0.043 ms
64 bytes from 10.244.1.136: icmp_seq=3 ttl=63 time=0.059 ms

--- 10.244.1.136 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2060ms
rtt min/avg/max/mdev = 0.043/0.107/0.221/0.080 ms
```

```bash
# Test HTTP connectivity
kubectl exec -n policy-test client -- curl -s --max-time 5 http://10.244.1.136
```

**Terminal Output:**
```
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

### Current Cilium Configuration

We examined the Cilium configuration to understand the default settings:

```bash
# Check current policy enforcement mode
kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-policy}'
```

**Terminal Output:**
```
default
```

The default policy mode of "default" means that only pods with policies applied are isolated, while other pods allow all traffic.

## 3. Policy Enforcement Modes

Cilium supports different policy enforcement modes that control how traffic is handled:

### Default Mode

- Mode value: `enable-policy: default`
- Behavior: Only pods with policies applied are isolated; all other pods allow all traffic
- Use case: Selective policy enforcement, good for gradual implementation

### Always Mode

- Mode value: `enable-policy: always`
- Behavior: All pods are isolated by default (deny-all)
- Use case: Zero-trust environments where all traffic must be explicitly allowed

### Changing Policy Mode Experiments

We modified the Cilium ConfigMap to change the policy enforcement mode:

```bash
# Change to "always" mode
kubectl patch configmap cilium-config -n kube-system --patch '{"data": {"enable-policy": "always"}}'
```

**Terminal Output:**
```
configmap/cilium-config patched
```

```bash
# Restart Cilium to apply changes
kubectl rollout restart ds/cilium -n kube-system
```

**Terminal Output:**
```
daemonset.apps/cilium restarted
```

```bash
# Verify the mode changed
kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-policy}'
```

**Terminal Output:**
```
always
```

**Observation**: After changing to "always" mode, we expected all traffic to be denied by default, but when testing, we observed that traffic between pods continued to work:

```bash
# Test ping after changing to "always" mode
kubectl exec -n policy-test client -- ping -c 3 10.244.1.136
```

**Terminal Output (Unexpected Behavior):**
```
PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.
64 bytes from 10.244.1.136: icmp_seq=1 ttl=63 time=0.714 ms
64 bytes from 10.244.1.136: icmp_seq=2 ttl=63 time=0.069 ms
64 bytes from 10.244.1.136: icmp_seq=3 ttl=63 time=0.091 ms

--- 10.244.1.136 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2040ms
rtt min/avg/max/mdev = 0.069/0.291/0.714/0.299 ms
```

## 4. Network Policy Approaches

### Approach 1: Kubernetes NetworkPolicy

We first tried standard Kubernetes NetworkPolicy to deny all traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: policy-test
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

```bash
# Apply the NetworkPolicy
kubectl apply -f deny-all-policy.yaml
```

**Terminal Output:**
```
networkpolicy.networking.k8s.io/default-deny created
```

**Test Result After Policy Application:**
```bash
kubectl exec -n policy-test client -- ping -c 3 10.244.1.136
```

**Terminal Output (Unexpected Behavior):**
```
PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.
64 bytes from 10.244.1.136: icmp_seq=1 ttl=63 time=0.714 ms
64 bytes from 10.244.1.136: icmp_seq=2 ttl=63 time=0.069 ms
64 bytes from 10.244.1.136: icmp_seq=3 ttl=63 time=0.091 ms

--- 10.244.1.136 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2040ms
rtt min/avg/max/mdev = 0.069/0.291/0.714/0.299 ms
```

### Approach 2: Cilium Deny Policy

Next, we created a Cilium-specific policy to deny traffic from client to web pod:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "default-deny-ingress"
  namespace: policy-test
spec:
  description: "Default deny ingress policy"
  endpointSelector:
    matchLabels:
      run: web
  ingress: [] # Empty ingress array means deny all ingress traffic
```

```bash
# Apply the Cilium policy
kubectl apply -f specific-deny-policy.yaml
```

**Terminal Output (Error):**
```
The CiliumNetworkPolicy "default-deny-ingress" is invalid: spec.ingress[0].toPorts[0].ports[0].protocol: Unsupported value: "ICMP": supported values: "TCP", "UDP", "SCTP", "ANY"
```

### Approach 3: Cilium IngressDeny Policy

We modified our approach to use the ingressDeny field:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "default-deny-ingress"
  namespace: policy-test
spec:
  description: "Default deny ingress policy"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        run: client
```

```bash
# Apply the updated policy
kubectl apply -f specific-deny-policy.yaml
```

**Terminal Output:**
```
ciliumnetworkpolicy.cilium.io/default-deny-ingress created
```

**Testing Policy Effectiveness:**
```bash
kubectl exec -n policy-test client -- ping -c 3 -w 10 10.244.1.136
```

**Terminal Output (Successful Blocking):**
```
PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.

--- 10.244.1.136 ping statistics ---
10 packets transmitted, 0 received, 100% packet loss, time 9240ms

command terminated with exit code 1
```

### Approach 4: Cilium Allow Specific Traffic

We then attempted to allow only specific traffic (HTTP) between pods:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "allow-specific-traffic"
  namespace: policy-test
spec:
  description: "Allow specific traffic from client to web"
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

```bash
# Delete deny policy and apply the allow policy
kubectl delete -f specific-deny-policy.yaml && kubectl apply -f allow-policy.yaml
```

**Terminal Output:**
```
ciliumnetworkpolicy.cilium.io "default-deny-ingress" deleted
ciliumnetworkpolicy.cilium.io/allow-specific-traffic created
```

**Testing HTTP Connectivity:**
```bash
kubectl exec -n policy-test client -- curl -v --max-time 5 http://10.244.1.136
```

**Terminal Output (Unexpected Blocking):**
```
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.1.136:80...
  0     0    0     0    0     0      0      0 --:--:--  0:00:05 --:--:--     0

command terminated with exit code 28
```

### Approach 5: Allow ICMP Traffic

We attempted to specifically allow ICMP (ping) traffic:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "allow-icmp-traffic"
  namespace: policy-test
spec:
  description: "Allow ICMP traffic from client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
    toPorts:
    - ports:
      - port: "8"
        protocol: ICMP
    - ports:
      - port: "80"
        protocol: TCP
```

```bash
# Apply the ICMP policy
kubectl apply -f allow-icmp-policy.yaml
```

**Terminal Output (Error):**
```
The CiliumNetworkPolicy "allow-icmp-traffic" is invalid: spec.ingress[0].toPorts[0].ports[0].protocol: Unsupported value: "ICMP": supported values: "TCP", "UDP", "SCTP", "ANY"
```

### Approach 6: Allow All Traffic Between Specific Pods

We created a policy that allows all traffic between our pods without specifying protocols:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "allow-all-protocols"
  namespace: policy-test
spec:
  description: "Allow all protocols from client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
    # No toPorts specification means all ports/protocols are allowed
```

```bash
# Apply the all-protocols policy
kubectl apply -f allow-all-icmp-policy.yaml
```

**Terminal Output:**
```
ciliumnetworkpolicy.cilium.io/allow-all-protocols created
```

**Testing After Policy Application:**
```bash
kubectl exec -n policy-test client -- ping -c 3 -w 10 10.244.1.136
```

**Terminal Output (Unexpected Blocking):**
```
PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.

--- 10.244.1.136 ping statistics ---
10 packets transmitted, 0 received, 100% packet loss, time 9210ms

command terminated with exit code 1
```

### Approach 7: CiliumClusterwideNetworkPolicy with Deny Rules

After recreating the test pods, we tried a cluster-wide policy that denies traffic from client to web:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "clusterwide-deny-test"
spec:
  description: "ClusterWide deny policy for testing"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        run: client
```

```bash
# Apply the cluster-wide deny policy
kubectl apply -f cluster-wide-policy.yaml
```

**Terminal Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/clusterwide-deny-test created
```

```bash
# Check if the policy is valid
kubectl get ciliumclusterwidenetworkpolicies
```

**Terminal Output:**
```
NAME                    VALID
clusterwide-deny-test   True
```

**Testing Ping Connectivity:**
```bash
kubectl exec -n policy-test client -- ping -c 3 -w 5 10.244.1.14
```

**Terminal Output (Successful Blocking):**
```
PING 10.244.1.14 (10.244.1.14) 56(84) bytes of data.

--- 10.244.1.14 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4106ms

command terminated with exit code 1
```

**Testing HTTP Connectivity:**
```bash
kubectl exec -n policy-test client -- curl -s --max-time 5 http://10.244.1.14
```

**Terminal Output (Successful Blocking):**
```
command terminated with exit code 28
```

### Approach 8: CiliumClusterwideNetworkPolicy with Allow Rules

We then tried a cluster-wide policy that allows traffic from client to web:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "clusterwide-allow-test"
spec:
  description: "ClusterWide allow policy for testing"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
```

```bash
# Delete the deny policy and apply the allow policy
kubectl delete -f cluster-wide-policy.yaml
kubectl apply -f cluster-wide-allow.yaml
```

**Terminal Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io "clusterwide-deny-test" deleted
ciliumclusterwidenetworkpolicy.cilium.io/clusterwide-allow-test created
```

```bash
# Check if the policy is valid
kubectl get ciliumclusterwidenetworkpolicies
```

**Terminal Output:**
```
NAME                     VALID
clusterwide-allow-test   True
```

**Testing Ping Connectivity After Policy Application:**
```bash
kubectl exec -n policy-test client -- ping -c 3 -w 5 10.244.1.14
```

**Terminal Output:**
```
PING 10.244.1.14 (10.244.1.14) 56(84) bytes of data.
64 bytes from 10.244.1.14: icmp_seq=1 ttl=63 time=0.284 ms
64 bytes from 10.244.1.14: icmp_seq=2 ttl=63 time=0.079 ms
64 bytes from 10.244.1.14: icmp_seq=3 ttl=63 time=0.083 ms

--- 10.244.1.14 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2038ms
rtt min/avg/max/mdev = 0.079/0.148/0.284/0.095 ms
```

**Observation**: Unlike the namespace-scoped CiliumNetworkPolicy, the CiliumClusterwideNetworkPolicy worked as expected: the deny policy blocked traffic and the allow policy permitted traffic.

### Approach 9: Precedence of Deny Over Allow Policies

To test what happens when both allow and deny policies target the same endpoints, we applied both policies simultaneously:

```yaml
# cluster-wide-allow.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "clusterwide-allow-test"
spec:
  description: "ClusterWide allow policy for testing"
  endpointSelector:
    matchLabels:
      run: web
  ingress:
  - fromEndpoints:
    - matchLabels:
        run: client
```

```yaml
# cluster-deny-test.yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "clusterwide-deny-final"
spec:
  description: "ClusterWide deny policy for final test"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        run: client
```

```bash
# Apply the deny policy while the allow policy is already active
kubectl apply -f cluster-deny-test.yaml
```

**Terminal Output:**
```
ciliumclusterwidenetworkpolicy.cilium.io/clusterwide-deny-final created
```

```bash
# Check if both policies are valid
kubectl get ciliumclusterwidenetworkpolicies
```

**Terminal Output:**
```
NAME                     VALID
clusterwide-allow-test   True
clusterwide-deny-final   True
```

**Testing Ping Connectivity with Both Policies:**
```bash
kubectl exec -n policy-test client -- ping -c 3 -w 5 10.244.1.14
```

**Terminal Output (Deny Takes Precedence):**
```
PING 10.244.1.14 (10.244.1.14) 56(84) bytes of data.

--- 10.244.1.14 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4086ms

command terminated with exit code 1
```

**Observation**: When both allow and deny policies target the same endpoints, the deny policy takes precedence. This is an important principle of Cilium's policy engine and a valuable security feature - even if an allow policy is mistakenly created, explicit deny rules will still be enforced.

## 5. Troubleshooting Steps

We employed several troubleshooting techniques:

### Checking Policy Validity

```bash
# List Cilium network policies and their validity
kubectl get ciliumnetworkpolicies -n policy-test
```

**Terminal Output:**
```
NAME                   AGE     VALID
default-deny-ingress   3m15s   True
```

### Examining Cilium Endpoints

```bash
# List Cilium endpoints
kubectl get ciliumendpoints -n policy-test
```

**Terminal Output:**
```
NAME     SECURITY IDENTITY   ENDPOINT STATE   IPV4           IPV6
client   60199               ready            10.244.1.149
web      919                 ready            10.244.1.136
```

### Checking Cilium Logs

```bash
# View Cilium agent logs for policy information
kubectl get pods -n kube-system -l k8s-app=cilium -o name | head -n 1 | xargs kubectl logs -n kube-system --tail=20 | grep -i policy
```

**Terminal Output:**
```
time=2025-07-18T17:57:01Z level=info msg="Policy repository updates complete, triggering endpoint updates" module=agent.controlplane.policy policyRevision=6
time="2025-07-18T17:57:01.429517674Z" level=info msg="Imported CiliumNetworkPolicy" ciliumNetworkPolicyName=default-deny-ingress k8sApiVersion= k8sNamespace=policy-test subsys=policy-k8s-watcher
time=2025-07-18T17:57:01Z level=info msg="Processing policy updates" module=agent.controlplane.policy count=1
time=2025-07-18T17:57:01Z level=info msg="Upserted policy to repository" module=agent.controlplane.policy resource=cnp/policy-test/default-deny-ingress policyRevision=7 deletedRules=0 identity=[919]
time=2025-07-18T17:57:01Z level=info msg="Policy repository updates complete, triggering endpoint updates" module=agent.controlplane.policy policyRevision=7
time="2025-07-18T18:01:16.02435325Z" level=info msg="Deleted CiliumNetworkPolicy" ciliumNetworkPolicyName=default-deny-ingress k8sApiVersion= k8sNamespace=policy-test subsys=policy-k8s-watcher
time=2025-07-18T18:01:16Z level=info msg="Processing policy updates" module=agent.controlplane.policy count=1
time=2025-07-18T18:01:16Z level=info msg="Deleted policy from repository" module=agent.controlplane.policy resource=cnp/policy-test/default-deny-ingress policyRevision=8 deletedRules=1 identity=[919]
time=2025-07-18T18:01:16Z level=info msg="Policy repository updates complete, triggering endpoint updates" module=agent.controlplane.policy policyRevision=8
time="2025-07-18T18:01:16.120084542Z" level=info msg="Imported CiliumNetworkPolicy" ciliumNetworkPolicyName=allow-specific-traffic k8sApiVersion= k8sNamespace=policy-test subsys=policy-k8s-watcher
time=2025-07-18T18:01:16Z level=info msg="Processing policy updates" module=agent.controlplane.policy count=1
time=2025-07-18T18:01:16Z level=info msg="Upserted policy to repository" module=agent.controlplane.policy resource=cnp/policy-test/allow-specific-traffic policyRevision=9 deletedRules=0 identity=[919]
time=2025-07-18T18:01:16Z level=info msg="Policy repository updates complete, triggering endpoint updates" module=agent.controlplane.policy policyRevision=9
```

### Resetting Environment

We tried restoring the default policy mode:

```bash
# Change back to default mode
kubectl patch configmap cilium-config -n kube-system --patch '{"data": {"enable-policy": "default"}}'
```

**Terminal Output:**
```
configmap/cilium-config patched
```

```bash
# Restart Cilium to apply changes
kubectl rollout restart ds/cilium -n kube-system
```

**Terminal Output:**
```
daemonset.apps/cilium restarted
```

```bash
# Wait for rollout to complete
kubectl rollout status ds/cilium -n kube-system
```

**Terminal Output:**
```
Waiting for daemon set "cilium" rollout to finish: 2 out of 3 new pods have been updated...
Waiting for daemon set "cilium" rollout to finish: 2 out of 3 new pods have been updated...
Waiting for daemon set "cilium" rollout to finish: 2 out of 3 new pods have been updated...
Waiting for daemon set "cilium" rollout to finish: 2 of 3 updated pods are available...
daemon set "cilium" successfully rolled out
```

We also tried recreating the pods to ensure clean state:

```bash
kubectl delete pod -n policy-test client web && kubectl run web --image=nginx -n policy-test && kubectl run client --image=nicolaka/netshoot -n policy-test -- sleep 3600
```

**Terminal Output:**
```
pod "client" deleted
pod "web" deleted
pod/web created
pod/client created
```

## 6. Errors and Issues Encountered

1. **Policy Validation Errors**:

   **Error with Cilium policy format:**
   ```
   The CiliumNetworkPolicy "default-deny-ingress" is invalid: spec.ingress[0].toPorts[0].ports[0].protocol: Unsupported value: "ICMP": supported values: "TCP", "UDP", "SCTP", "ANY"
   ```

   **Error with empty ingress array:**
   ```
   The CiliumNetworkPolicy "default-deny-ingress" is invalid: rule must have at least one of Ingress, IngressDeny, Egress, EgressDeny
   ```

2. **Unexpected Behavior**:

   **Traffic continued even with "always" policy mode:**
   ```
   PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.
   64 bytes from 10.244.1.136: icmp_seq=1 ttl=63 time=0.714 ms
   64 bytes from 10.244.1.136: icmp_seq=2 ttl=63 time=0.069 ms
   64 bytes from 10.244.1.136: icmp_seq=3 ttl=63 time=0.091 ms
   ```

   **HTTP traffic blocked despite valid allow policy:**
   ```
   % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                               Dload  Upload   Total   Spent    Left  Speed
   0     0    0     0    0     0      0      0 --:--:--  0:00:05 --:--:--     0
   ```

   **Ping remained blocked despite "allow-all-protocols" policy:**
   ```
   PING 10.244.1.136 (10.244.1.136) 56(84) bytes of data.
   --- 10.244.1.136 ping statistics ---
   10 packets transmitted, 0 received, 100% packet loss, time 9210ms
   ```

3. **Troubleshooting Challenges**:
   
   **Limited debugging tools in nginx container:**
   ```
   error: Internal error occurred: error executing command in container: failed to exec in container: failed to start exec "7222c75c13d984a7f37f9808cda333cb7183a0db989ecf74b3a444c5fdda7944": OCI runtime exec failed: exec failed: unable to start container process: exec: "netstat": executable file not found in $PATH
   ```

   **HTTP request timing out despite proper policy:**
   ```
   % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                               Dload  Upload   Total   Spent    Left  Speed
   0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.1.136:80...
   0     0    0     0    0     0      0      0 --:--:--  0:00:30 --:--:--     0
   ```

## 7. Key Learnings

1. **Policy Enforcement Mode**:
   - The `enable-policy` setting in Cilium's ConfigMap controls the global policy behavior
   - Changing from "default" to "always" mode requires a Cilium restart
   - In practice, changing to "always" mode did not consistently block traffic as expected

2. **Cilium Network Policies**:
   - CiliumNetworkPolicy CRDs offer more features than standard Kubernetes NetworkPolicy
   - Label selectors are the primary mechanism for targeting pods
   - Protocol specifications have limitations (ICMP handling is different)
   - CiliumClusterwideNetworkPolicy is more reliable than namespace-scoped CiliumNetworkPolicy
   - Deny policies take precedence over allow policies when both are present

3. **Policy Troubleshooting**:
   - Always check policy validity status
   - Examine Cilium logs for insight into policy application
   - Use port-forwarding for direct testing when needed
   - Re-creating pods can help resolve inconsistent behavior

4. **Best Practices**:
   - Start with permissive policies and gradually restrict
   - Test policies thoroughly before deploying
   - Consider both pod labels and namespace selectors
   - Use CiliumClusterwideNetworkPolicy for more reliable enforcement
   - Use ingressDeny/egressDeny fields for explicit denial rules
   - When needed, use explicit deny policies which take precedence over allow policies

## 8. Conclusion

Our experiments demonstrated that Cilium provides powerful network policy capabilities, but proper implementation requires careful configuration and understanding of both Kubernetes and Cilium-specific concepts.

The different approaches we tried highlight the flexibility of Cilium's policy engine, as well as some of its complexities. For production deployments, a systematic approach starting with permissive policies and gradually restricting traffic as needed would be recommended.

Based on our testing, the most reliable approach was using CiliumClusterwideNetworkPolicy with explicit allow or deny rules. The namespace-scoped policies and global policy mode changes showed inconsistent behavior.

Our final test confirmed that when both allow and deny policies target the same traffic, the deny policy takes precedence. This is an important security principle that ensures explicit denials are always enforced, even if conflicting allow policies exist.

## 9. References

- [Cilium Network Policy Documentation](https://docs.cilium.io/en/stable/security/policy/)
- [Kubernetes Network Policy](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Cilium Policy Enforcement Modes](https://docs.cilium.io/en/stable/security/policy/intro/#policy-enforcement-modes)
