Running tests from: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies
Available subtests by Cilium L3 Policy Categories:

1. ENDPOINTS-BASED POLICIES:
  endpoints      - Test endpoints-based policy with label selectors

2. SERVICES-BASED POLICIES:
  services       - Test services-based policy with Kubernetes services

3. ENTITIES-BASED POLICIES:
  entities       - Test entities-based policy (host, world, cluster)

4. NODE-BASED POLICIES:
  node-name      - Test pod node name policy (formerly pod-node-name)
  node-selector  - Test node selector policy (formerly node-cidr)
  from-nodes     - Test fromNodes selector policy (l3-node-policy)
  node-entities  - Test node entities (remote-node, host) policy
  node           - Test all node-based policies

5. IP/CIDR-BASED POLICIES:
  cidr-ingress   - Test CIDR ingress policy
  cidr-egress    - Test CIDR egress policy
  cidr-except    - Test CIDR with exceptions
  cidr           - Test all CIDR-based policies

6. DNS-BASED POLICIES:
  dns            - Test DNS-based policies

OTHER OPTIONS:
  baseline       - Test baseline policy enforcement (simplest possible policy)
  categories     - Test all categories with cleanup between each category (default)
  cleanup        - Only clean up the test environment (delete namespace and policies)
  list           - Show this list
  help           - Show usage information
daryakut@b0f1d8773a3c 7-l3-policies % cd "/Users/daryakut/Desktop/k8s_diagnostic"
daryakut@b0f1d8773a3c k8s_diagnostic % cd cilium-policies/7-l3-policies/ && ./test-l3-policies.sh categories
Running tests from: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.181 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.156 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.20 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED

=================================================================
= RUNNING TESTS BY CATEGORY WITH CLEANUP BETWEEN CATEGORIES 
=================================================================

ℹ️  This will run tests by Cilium documentation categories with cleanup between each category
ℹ️  to prevent policy interference between categories.


=================================================================
= [1/6] RUNNING CATEGORY: endpoints 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: endpoints

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.118 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.4 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.22 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: endpoints

=================================================================
= RUNNING ALL ENDPOINTS-BASED POLICY TESTS (CILIUM CATEGORY 1) 
=================================================================


=================================================================
= TESTING ENDPOINTS-BASED POLICY (CILIUM CATEGORY 1) 
=================================================================

ℹ️  This policy type uses label selectors to select endpoints managed by Cilium
ℹ️  This is the most common type of Cilium policy and is completely decoupled from addressing
NAME                       AGE   VALID
endpoints-label-selector   10s   True
Testing connectivity from client1 (same namespace, should work)...
✓ Connectivity from client1 works as expected (label matching)
ℹ️  Policy 'endpoints-label-selector' has been left applied for further testing
✓ Endpoints-based policy test completed
✓ All endpoints-based policy tests completed

>>> ENDPOINTS-BASED POLICY TESTS SUMMARY 

Endpoints Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= [2/6] RUNNING CATEGORY: services 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: endpoints-label-selector
ciliumnetworkpolicy.cilium.io "endpoints-label-selector" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/endpoints-policies/endpoints-label-selector.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: services

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.201 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.75 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.233 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: services

=================================================================
= RUNNING ALL SERVICES-BASED POLICY TESTS (CILIUM CATEGORY 2) 
=================================================================


=================================================================
= TESTING SERVICES-BASED POLICY (CILIUM CATEGORY 2) 
=================================================================

ℹ️  This policy type targets Kubernetes services rather than pods directly
ℹ️  It allows decoupling from direct pod IPs while still controlling traffic flow
Creating Kubernetes service pointing to API pod...
service/api-svc created
error: 'app' already has a value (api-svc), and --overwrite is false
service/api-svc patched
API Service IP: 10.96.147.41
Testing connectivity to the Kubernetes service...
✓ Connectivity to service works as expected
ℹ️  Policy 'kubernetes-service-policy' has been left applied for further testing
✓ Services-based policy test completed
✓ All services-based policy tests completed

>>> SERVICES-BASED POLICY TESTS SUMMARY 

Services Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= [3/6] RUNNING CATEGORY: entities 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: service-based-policy
ciliumnetworkpolicy.cilium.io "service-based-policy" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
namespace "l3-policy-test" force deleted
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/services-policies/kubernetes-service-policy.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: entities

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.80 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.251 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.20 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: entities

=================================================================
= RUNNING ALL ENTITIES-BASED POLICY TESTS (CILIUM CATEGORY 3) 
=================================================================


=================================================================
= TESTING ENTITIES-BASED POLICY (CILIUM CATEGORY 3) 
=================================================================

ℹ️  This policy type uses predefined entities like 'host', 'world', 'cluster'
ℹ️  It allows specifying remote peers without knowing their IP addresses
NAME                    AGE   VALID
entities-based-policy   11s   True
Testing connectivity from client1 (should work due to 'cluster' entity)...
✓ Connectivity from client1 works as expected (cluster entity)
ℹ️  Policy 'entities-based-policy' has been left applied for further testing
✓ Entities-based policy test completed
✓ All entities-based policy tests completed

>>> ENTITIES-BASED POLICY TESTS SUMMARY 

Entities Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= [4/6] RUNNING CATEGORY: node 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: entities-based-policy
ciliumnetworkpolicy.cilium.io "entities-based-policy" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/entities-policies/entities-based-policy.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: node

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.48 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.130 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.97 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: node

=================================================================
= RUNNING ALL NODE-BASED POLICY TESTS (CILIUM CATEGORY 4) 
=================================================================

The following node policy tests will be executed:
1. Node Name Policy (pod-node-name-policy.yaml)
2. Node Selector Policy (node-cidr-policy.yaml)
3. FromNodes Policy (l3-node-policy.yaml)
4. Node Entities Policy (node-based-policy-clusterwide.yaml)


=================================================================
= TESTING POD NODE NAME POLICY 
=================================================================


>>> Testing pod node name policy 

NAME                   VALID
pod-node-name-policy   True
Testing connectivity from client1 (same node, should work)...
✓ Connectivity from client1 works as expected
Testing connectivity from client2 (different node, should fail)...
command terminated with exit code 28
✓ Connection from client2 blocked as expected
ℹ️  Policy 'pod-node-name-policy' has been left applied for further testing
✓ Pod node name policy test completed

=================================================================
= TESTING NODE CIDR POLICY 
=================================================================


>>> Testing node CIDR policy 

NAME               AGE   VALID
node-cidr-policy   10s   True
Testing connectivity from client1 (same node, should work)...
Attempt 1 with 10 second timeout...
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.0.48:80...
* Connected to 10.244.0.48 (10.244.0.48) port 80
* using HTTP/1.x
> GET / HTTP/1.1
> Host: 10.244.0.48
> User-Agent: curl/8.14.1
> Accept: */*
> 
* Request completely sent off
< HTTP/1.1 200 OK
< Server: nginx/1.29.0
< Date: Wed, 23 Jul 2025 20:10:32 GMT
< Content-Type: text/html
< Content-Length: 615
< Last-Modified: Tue, 24 Jun 2025 17:57:38 GMT
< Connection: keep-alive
< ETag: "685ae712-267"
< Accept-Ranges: bytes
< 
{ [615 bytes data]
100   615  100   615    0     0   406k      0 --:--:-- --:--:-- --:--:--  600k
* Connection #0 to host 10.244.0.48 left intact
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
✓ Connectivity from client1 works as expected (attempt 1)
Testing connectivity from client2 (different node, should fail)...
command terminated with exit code 28
✓ Connection from client2 blocked as expected
ℹ️  Policy 'node-cidr-policy' has been left applied for further testing
✓ Node CIDR policy test completed

=================================================================
= TESTING FROM-NODES POLICY (CILIUM CATEGORY 4) 
=================================================================

ℹ️  This policy type uses fromNodes selector to specify nodes by hostname
ℹ️  It allows controlling traffic from specific nodes without pod details
NAME               AGE   VALID
l3-node-policy     10s   True
node-cidr-policy   29s   True
Testing connectivity from client2 (should work as it's from NODE2)...
✓ Connectivity from client2 works as expected (fromNodes matching)
ℹ️  Policy 'l3-node-policy' has been left applied for further testing
✓ FromNodes policy test completed

=================================================================
= TESTING NODE ENTITIES POLICY (CILIUM CATEGORY 4) 
=================================================================

ℹ️  This policy type uses remote-node and host entities
ℹ️  It allows controlling traffic from all nodes without specifying each one
NAME                            VALID
node-based-policy-clusterwide   True
pod-node-name-policy            True
Testing connectivity from client2 (should work as remote-node entity allows it)...
✓ Connectivity from client2 works as expected (remote-node entity)
ℹ️  Policy 'node-based-policy-clusterwide' has been left applied for further testing
✓ Node entities policy test completed
✓ All node policy tests completed

>>> NODE-BASED POLICY TESTS SUMMARY 

Pod Node Name Policy Test: PASSED
Node Selector Policy Test: PASSED
FromNodes Policy Test: PASSED
Node Entities Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= [5/6] RUNNING CATEGORY: cidr 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: l3-node-policy
ciliumnetworkpolicy.cilium.io "l3-node-policy" force deleted
Explicitly deleting policy: node-cidr-policy
ciliumnetworkpolicy.cilium.io "node-cidr-policy" force deleted
Explicitly deleting cluster-wide policy: node-based-policy-clusterwide
ciliumclusterwidenetworkpolicy.cilium.io "node-based-policy-clusterwide" force deleted
Explicitly deleting cluster-wide policy: pod-node-name-policy
ciliumclusterwidenetworkpolicy.cilium.io "pod-node-name-policy" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/node-policies/node-based-policy-clusterwide.yaml.applied
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/node-policies/pod-node-name-policy.yaml.applied
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/node-policies/node-cidr-policy.yaml.applied
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/node-policies/l3-node-policy.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: cidr

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.60 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.134 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.108 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: cidr

=================================================================
= RUNNING ALL IP/CIDR-BASED POLICY TESTS (CILIUM CATEGORY 5) 
=================================================================

The following CIDR policy tests will be executed:
1. CIDR Ingress Policy (cidr-ingress-policy.yaml)
2. CIDR Egress Policy (cidr-egress-policy.yaml)
3. CIDR with Exceptions Policy (cidr-with-except-policy.yaml)


=================================================================
= TESTING CIDR INGRESS POLICY 
=================================================================

NAME                  AGE   VALID
cidr-ingress-policy   11s   True
Testing connectivity from client1 (same node, should work)...
Debug: checking Cilium endpoints status...
NAME      SECURITY IDENTITY   ENDPOINT STATE   IPV4           IPV6
api       19231               ready            10.244.0.60    
client1   38646               ready            10.244.0.134   
client2   15685               ready            10.244.1.108   
Attempt 1 with 10 second timeout...
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.0.60:80...
* Connected to 10.244.0.60 (10.244.0.60) port 80
* using HTTP/1.x
> GET / HTTP/1.1
> Host: 10.244.0.60
> User-Agent: curl/8.14.1
> Accept: */*
> 
* Request completely sent off
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
< HTTP/1.1 200 OK
< Server: nginx/1.29.0
< Date: Wed, 23 Jul 2025 20:11:38 GMT
< Content-Type: text/html
< Content-Length: 615
< Last-Modified: Tue, 24 Jun 2025 17:57:38 GMT
< Connection: keep-alive
< ETag: "685ae712-267"
< Accept-Ranges: bytes
< 
{ [615 bytes data]
100   615  100   615    0     0   486k      0 --:--:-- --:--:-- --:--:--  600k
* Connection #0 to host 10.244.0.60 left intact
✓ Connectivity from client1 works as expected (attempt 1)
Testing connectivity from client2 (different node, should fail if nodes have different CIDRs)...
command terminated with exit code 28
✓ Connection from client2 blocked as expected
ℹ️  Policy 'cidr-ingress-policy' has been left applied for further testing
✓ CIDR ingress policy test completed

>>> Cleaning up only Cilium policies (preserving test environment) 

Explicitly deleting policy: cidr-ingress-policy
ciliumnetworkpolicy.cilium.io "cidr-ingress-policy" force deleted
Checking for remaining policies (attempt 1)...
No resources found
No resources found
Checking for remaining policies (attempt 2)...
No resources found
No resources found
Checking for remaining policies (attempt 3)...
No resources found
No resources found
Performing final verification of policy cleanup...
✓ All policies successfully removed

=================================================================
= TESTING CIDR EGRESS POLICY 
=================================================================

NAME                 AGE   VALID
cidr-egress-policy   10s   True
Testing egress from client2 to API pod (should work)...
Attempt 1 with 10 second timeout...
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0*   Trying 10.244.0.60:80...
* Connected to 10.244.0.60 (10.244.0.60) port 80
* using HTTP/1.x
> GET / HTTP/1.1
> Host: 10.244.0.60
> User-Agent: curl/8.14.1
> Accept: */*
> 
* Request completely sent off
< HTTP/1.1 200 OK
< Server: nginx/1.29.0
< Date: Wed, 23 Jul 2025 20:12:16 GMT
< Content-Type: text/html
< Content-Length: 615
< Last-Modified: Tue, 24 Jun 2025 17:57:38 GMT
< Connection: keep-alive
< ETag: "685ae712-267"
< Accept-Ranges: bytes
< 
{ [615 bytes data]
100   615  100   615    0     0   281k      0 --:--:-- --:--:-- --:--:--  300k
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
* Connection #0 to host 10.244.0.60 left intact
✓ Egress from client2 to API pod works as expected (attempt 1)
ℹ️  Policy 'cidr-egress-policy' has been left applied for further testing
✓ CIDR egress policy test completed

>>> Cleaning up only Cilium policies (preserving test environment) 

Explicitly deleting policy: cidr-egress-policy
ciliumnetworkpolicy.cilium.io "cidr-egress-policy" force deleted
Checking for remaining policies (attempt 1)...
No resources found
No resources found
Checking for remaining policies (attempt 2)...
No resources found
No resources found
Checking for remaining policies (attempt 3)...
No resources found
No resources found
Performing final verification of policy cleanup...
✓ All policies successfully removed

=================================================================
= TESTING CIDR WITH EXCEPTIONS POLICY 
=================================================================

ℹ️  This policy type uses CIDR blocks with exceptions (except CIDR)
ℹ️  It allows specifying IP ranges while excluding specific subnets
NAME                      AGE   VALID
cidr-with-except-policy   10s   False
Testing connectivity from client1 (should be allowed by CIDR rules)...
✓ Connectivity from client1 works as expected (in allowed CIDR)
ℹ️  Policy 'cidr-with-except-policy' has been left applied for further testing
✓ CIDR with exceptions policy test completed
✓ All CIDR policy tests completed

>>> IP/CIDR-BASED POLICY TESTS SUMMARY 

CIDR Ingress Policy Test: PASSED
CIDR Egress Policy Test: PASSED
CIDR with Exceptions Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= [6/6] RUNNING CATEGORY: dns 
=================================================================

ℹ️  Cleaning up previous test environment...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: cidr-with-except-policy
ciliumnetworkpolicy.cilium.io "cidr-with-except-policy" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/cidr-policies/cidr-with-except-policy.yaml.applied
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/cidr-policies/cidr-ingress-policy.yaml.applied
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/cidr-policies/cidr-egress-policy.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed
ℹ️  Creating fresh test environment for category: dns

>>> Setting up test environment 

namespace/l3-policy-test created
Created namespace: l3-policy-test
ℹ️  Using all available nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
✓ Found 3 worker nodes: cluster-2-control-plane cluster-2-worker cluster-2-worker2
Creating target pod on cluster-2-control-plane...
pod/api created
Creating client pods on different nodes...
pod/client1 created
pod/client2 created
Waiting for pods to be ready...
pod/api condition met
pod/client1 condition met
pod/client2 condition met
API Pod IP: 10.244.0.2 (on cluster-2-control-plane)
Client1 Pod IP: 10.244.0.198 (on cluster-2-control-plane)
Client2 Pod IP: 10.244.1.41 (on cluster-2-worker)
Node1 CIDR: 10.244.0.0/24
Node2 CIDR: 10.244.1.0/24
✓ Test environment ready

>>> Testing basic connectivity (no policies) 

Testing ICMP ping from client1 (same node)...
✓ ICMP from client1 to API pod successful
Testing ICMP ping from client2 (different node)...
✓ ICMP from client2 to API pod successful
Testing HTTP connectivity from client1 (same node)...
✓ HTTP from client1 to API pod successful
<!DOCTYPE html>
<html>
<head>
Testing HTTP connectivity from client2 (different node)...
✓ HTTP from client2 to API pod successful
<!DOCTYPE html>
<html>
<head>
✓ Basic connectivity test PASSED
ℹ️  Running tests for category: dns

=================================================================
= RUNNING ALL DNS-BASED POLICY TESTS (CILIUM CATEGORY 6) 
=================================================================


=================================================================
= TESTING DNS-BASED POLICY (CILIUM CATEGORY 6) 
=================================================================

ℹ️  This policy type uses DNS names converted to IPs via DNS lookups
ℹ️  It requires a working DNS setup and only works for egress traffic
NAME               AGE   VALID
dns-based-policy   11s   True
Testing DNS-based egress to example.com (should work)...
✓ DNS-based egress to example.com works as expected
ℹ️  Policy 'dns-based-policy' has been left applied for further testing
✓ DNS-based policy test completed
✓ All DNS-based policy tests completed

>>> DNS-BASED POLICY TESTS SUMMARY 

DNS Policy Test: PASSED

--------------------------------------------------------------


=================================================================
= CATEGORY TESTS RESULTS SUMMARY 
=================================================================

Total categories executed: 6
Categories passed: 6
Categories failed: 0
Categories partial/skipped: 0

Individual category results:
Category endpoints: PASSED
Category services: PASSED
Category entities: PASSED
Category node: PASSED
Category cidr: PASSED
Category dns: PASSED
✓ All category tests completed
ℹ️  Performing final cleanup...

>>> Cleaning up test environment 

Deleting all Cilium policies (explicit deletion)...
Explicitly deleting policy: dns-based-policy
ciliumnetworkpolicy.cilium.io "dns-based-policy" force deleted
ℹ️  Performing ultra-thorough cleanup of test environment...
Deleting pods in namespace: l3-policy-test
pod "api" force deleted
pod "client1" force deleted
pod "client2" force deleted
Deleting all Cilium policies (bulk deletion)...
No resources found
No resources found
Removing finalizers from namespace if present...
namespace/l3-policy-test patched (no change)
Attempt 1: Deleting namespace: l3-policy-test
namespace "l3-policy-test" deleted
Namespace still exists, waiting before next attempt...
Attempt 2: Deleting namespace: l3-policy-test
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
Error from server (NotFound): namespaces "l3-policy-test" not found
Namespace successfully deleted!
Namespace l3-policy-test has been successfully deleted
Waiting for resources to be fully cleaned up...
Cleaning up .applied files...
Removed: /Users/daryakut/Desktop/k8s_diagnostic/cilium-policies/7-l3-policies/dns-policies/dns-based-policy.yaml.applied
✓ Cleanup complete - Original YAML files preserved, .applied files removed

=================================================================
= ALL TESTS COMPLETED 
=================================================================

Run './test-l3-policies.sh list' to see other available subtests