# Cilium Network Policy: Final Testing Results and Conclusions

This document summarizes the final findings from our comprehensive testing of Cilium network policies, with a focus on the unexpected behavior observed when implementing deny policies.

## Test Environment

Our testing was conducted in a Kubernetes cluster with Cilium v1.17.5 as the CNI, with the following test pods:

```
NAME           READY   STATUS    RESTARTS   AGE     LABELS
client         1/1     Running   0          12m     run=client
other-client   1/1     Running   0          11m     run=other-client
test-pod       1/1     Running   0          69s     app=testing
third-client   1/1     Running   0          2m15s   run=third-client
web            1/1     Running   0          12m     run=web
```

## Applied Policy

We applied a CiliumClusterwideNetworkPolicy intended to deny traffic only from the `client` pod to the `web` pod:

```yaml
apiVersion: "cilium.io/v2"
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: "deny-all-traffic"
spec:
  description: "Deny all traffic from client to web"
  endpointSelector:
    matchLabels:
      run: web
  ingressDeny:
  - fromEndpoints:
    - matchLabels:
        run: client
```

## Observed Terminal Outputs

### 1. Blocking Traffic from Original Client Pod (Expected Behavior)

```bash
kubectl exec -n policy-test client -- ping -c 3 -w 5 10.244.1.26
```

**Output:**
```
PING 10.244.1.26 (10.244.1.26) 56(84) bytes of data.

--- 10.244.1.26 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4076ms

command terminated with exit code 1
```

```bash
kubectl exec -n policy-test client -- curl -s --max-time 5 10.244.1.26
```

**Output:**
```
command terminated with exit code 28
```

### 2. Blocking Traffic from Other Client Pod (Unexpected Behavior)

```bash
kubectl exec -n policy-test other-client -- ping -c 3 10.244.1.26
```

**Output:**
```
PING 10.244.1.26 (10.244.1.26) 56(84) bytes of data.

--- 10.244.1.26 ping statistics ---
3 packets transmitted, 0 received, 100% packet loss, time 2082ms

command terminated with exit code 1
```

```bash
kubectl exec -n policy-test other-client -- curl -v --max-time 5 10.244.1.26
```

**Output:**
```
% Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.1.26:80...
  0     0    0     0    0     0      0      0 --:--:--  0:00:01 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:02 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:03 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:04 --:--:--     0* Connection timed out after 5011 milliseconds
  0     0    0     0    0     0      0      0 --:--:--  0:00:05 --:--:--     0
* closing connection #0
curl: (28) Connection timed out after 5011 milliseconds
command terminated with exit code 28
```

### 3. Blocking Traffic from Test Pod (Different Label Scheme)

```bash
kubectl exec -n policy-test test-pod -- ping -c 3 -w 5 10.244.1.26
```

**Output:**
```
PING 10.244.1.26 (10.244.1.26) 56(84) bytes of data.

--- 10.244.1.26 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4075ms

command terminated with exit code 1
```

```bash
kubectl exec -n policy-test test-pod -- curl -v --max-time 5 10.244.1.26
```

**Output:**
```
% Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.1.26:80...
  0     0    0     0    0     0      0      0 --:--:--  0:00:01 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:02 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:03 --:--:--     0  0     0    0     0    0     0      0      0 --:--:--  0:00:04 --:--:--     0* Connection timed out after 5002 milliseconds
  0     0    0     0    0     0      0      0 --:--:--  0:00:05 --:--:--     0
* closing connection #0
curl: (28) Connection timed out after 5002 milliseconds
command terminated with exit code 28
```

### 4. Blocking Traffic from Third Client Pod (Unexpected Behavior)

```bash
kubectl exec -n policy-test third-client -- ping -c 3 -w 5 10.244.1.26
```

**Output:**
```
PING 10.244.1.26 (10.244.1.26) 56(84) bytes of data.

--- 10.244.1.26 ping statistics ---
5 packets transmitted, 0 received, 100% packet loss, time 4122ms

command terminated with exit code 1
```

### 5. Verification Steps

We verified that no other policies were in effect:

```bash
kubectl get networkpolicies --all-namespaces
```
**Output:**
```
No resources found
```

```bash
kubectl get ciliumnetworkpolicies --all-namespaces
```
**Output:**
```
No resources found
```

```bash
kubectl get ciliumclusterwidenetworkpolicies
```
**Output:**
```
NAME              VALID
deny-all-traffic  True
```

We confirmed the Cilium version:

```bash
kubectl -n kube-system get pods -l k8s-app=cilium -o jsonpath='{.items[0].spec.containers[0].image}'
```
**Output:**
```
quay.io/cilium/cilium:v1.17.5@sha256:baf8541723ee0b72d6c489c741c81a6fdc5228940d66cb76ef5ea2ce3c639ea6
```

## Final Conclusions

1. **Unexpected Default Behavior**: 
   - When a pod becomes a policy endpoint target (via endpointSelector), all traffic to that pod is affected
   - The fromEndpoints selector in ingressDeny doesn't limit the policy application as expected
   - This results in a "deny by default" behavior for any pod targeted by a policy

2. **Security Implications**:
   - The behavior creates a stricter security posture than what's explicitly defined
   - From a security perspective, this could be considered beneficial (fail-closed)
   - However, it's contrary to the documented behavior and could cause unexpected connectivity issues

3. **Production Recommendations**:
   - When implementing Cilium deny policies, expect all traffic to be blocked to targeted endpoints
   - Use specific allow policies to restore needed connectivity
   - Always test thoroughly with multiple traffic sources and protocols
   - Be cautious when migrating existing network policies to Cilium

4. **Documentation Discrepancy**:
   - Cilium documentation suggests that fromEndpoints selectors should limit which traffic sources are affected
   - Our testing shows that any pod targeted with a deny policy blocks all incoming traffic
   - This behavior may be specific to Cilium v1.17.5 or related to the ClusterwideNetworkPolicy implementation

These findings underscore the importance of thorough testing when implementing network policies in production environments. The observed behavior demonstrates that Cilium network policies may implement stricter controls than their specifications suggest, which could impact application connectivity in unexpected ways.
