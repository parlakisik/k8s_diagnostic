# k8s-diagnostic

A CLI tool for testing network connectivity within Kubernetes clusters using real pod-to-pod communication tests with enhanced visual feedback and comprehensive logging.

## **Latest Enhancements**

Recent updates have significantly improved the user experience and diagnostic capabilities:

- **🔒 Comprehensive Cilium Network Policies**: Complete library of Cilium NetworkPolicies from basic allow/deny to advanced L3/L4 filtering
- **🧪 Network Policy Testing Framework**: Testing tools for validating Cilium policy behavior in real clusters
- **📊 Enhanced Logging System**: Multi-level logging (DEBUG, INFO, WARNING, ERROR) with colored output and comprehensive log files
- **📝 Detailed Diagnostics**: Improved error reporting with context-aware logging and file/line tracking
- **📁 Structured Logs**: All terminal output captured in timestamped log files for later analysis

## **Main Branch - Core Foundation**

This is the **main/base branch** containing the foundational connectivity tests and network policies that form the core of k8s-diagnostic. This branch provides reliable, battle-tested network diagnostics for any Kubernetes cluster.


### **Core Features (This Branch):**
- **6 Comprehensive Tests**: Pod-to-Pod, Service-to-Pod, Cross-Node Service, DNS Resolution, NodePort Service, LoadBalancer Service
- **Cilium Network Policies Library**: Complete collection of Cilium CNI network policies organized by type and use case
- **Enhanced Visual Output**: Emoji-based UI for clearer, more engaging test results 🎨
- **Comprehensive Logging**: Multi-level logging with DEBUG, INFO, WARNING, ERROR levels
- **Log File Generation**: All output captured in timestamped log files for debugging
- **Production Ready**: Stable, reliable connectivity testing
- **Educational Output**: Detailed explanations and equivalent kubectl commands
- **JSON Reporting**: Structured results for automation and monitoring
- **Clean Architecture**: Well-organized, maintainable codebase
- **Namespace Persistence**: Optional preservation of test namespace between runs for efficient testing

## Overview

This project provides:
- **`build_test_k8s.sh`** - Script to create a test Kubernetes cluster using kind with Cilium CNI
- **`delete_test_k8s.sh`** - Script to delete test clusters
- **`k8s-diagnostic`** - CLI tool for running comprehensive diagnostic tests in any Kubernetes cluster
- **`cilium-policies/`** - Library of Cilium network policies with documentation and testing tools

## Cilium Network Policies Library

The repository now includes an extensive collection of Cilium network policies organized by type and complexity. These policies serve as both educational examples and practical templates for securing Kubernetes clusters.

### Policies Collection Overview

1. **Basic Allow/Deny Policies**
   - `1-allow-all`: Allow all traffic (baseline policy)
   - `2-deny-all`: Deny all traffic (zero-trust baseline)

2. **Namespace-based Policies**
   - `3-same-namespace`: Allow traffic within the same namespace
   - `4-deny-namespace`: Deny traffic from specific namespaces

3. **Label-based Policies**
   - `5-same-label`: Allow traffic between pods with matching labels
   - `6-deny-label`: Deny traffic based on pod labels

4. **L3 (Network Layer) Policies**
   - `7-l3-policies/cidr-policies`: CIDR-based ingress and egress filtering
   - `7-l3-policies/node-policies`: Node-based selectors and node CIDR targeting

5. **L4 (Transport Layer) Policies**
   - `8-l4-policies/basic-port-policies`: TCP/UDP port-based filtering
   - `8-l4-policies/http-api-policies`: HTTP-specific policies (methods, paths, headers)
   - `8-l4-policies/advanced-l4-policies`: Combined L3/L4 policies and service targeting

### Using the Cilium Policies

Each policy directory contains:
- README.md with detailed explanations
- YAML policy definitions ready to apply
- Testing instructions and expected behavior

To test policies:
```bash
# Test specific L4 policy types
./cilium-policies/8-l4-policies/test-l4-policies.sh basic-port
./cilium-policies/8-l4-policies/test-l4-policies.sh http-api
./cilium-policies/8-l4-policies/test-l4-policies.sh advanced

# Test all L4 policy types
./cilium-policies/8-l4-policies/test-l4-policies.sh all
```

### L4 Policy Types

#### Basic Port Policies
Control traffic based on TCP/UDP port numbers:
```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "tcp-port-ingress-policy"
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - matchLabels:
        app: frontend
    toPorts:
    - ports:
      - port: "80"
        protocol: TCP
```

#### HTTP API Policies
Fine-grained control over HTTP traffic by inspecting URL paths, methods, and headers:
```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - toPorts:
    - ports:
      - port: "80"
        protocol: TCP
      rules:
        http:
        - method: "GET"
          path: "/api/v1/health"
```

#### Combined L3/L4 Policies
Merge IP and port-based filtering for defense in depth:
```yaml
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
spec:
  endpointSelector:
    matchLabels:
      app: database
  ingress:
  - fromCIDR:
    - "10.244.0.0/16"
    toPorts:
    - ports:
      - port: "3306"
        protocol: TCP
```

## New User Guide: Getting Started

This step-by-step guide will help you get started with k8s-diagnostic from scratch. Follow these instructions to set up your environment, run diagnostics, and understand the results.

### Prerequisites

Before you begin, ensure you have the following installed:
- Docker (running)
- kind (Kubernetes in Docker)
- kubectl
- helm
- Go 1.21+ (for building from source)

### Step 1: Clone the Repository

```bash
git clone https://github.com/parlakisik/k8s_diagnostic.git
cd k8s_diagnostic
```

### Step 2: Create a Test Kubernetes Cluster

```bash
# Create a 3-node kind cluster with Cilium CNI
./build_test_k8s.sh

# Verify your cluster is running
kubectl get nodes
```

This creates a Kubernetes cluster with 1 control-plane and 2 worker nodes, running the Cilium CNI.

### Step 3: Build the Diagnostic Tool

```bash
# Build using Make
make build

# Verify the binary was created
ls -la build/k8s-diagnostic
```

### Step 4: Run the Network Diagnostics

```bash
# Run all network tests with verbose output
./build/k8s-diagnostic test --test-group networking --verbose
```

This will:
1. Create a `diagnostic-test` namespace
2. Run 6 comprehensive network connectivity tests
3. Generate detailed logs and a JSON report
4. Clean up resources automatically

### Step 5: Understanding the Results

After running the tests, you'll see:
- A summary of all test results with pass/fail status
- Detailed steps that were performed (in verbose mode)
- Path to the JSON report for further analysis

Example successful output will look like:
```
📊 Test Summary:
  Total Tests: 6, Passed: 6, Failed: 0
  ✅ Passed Tests:
    ✅ Pod-to-Pod Connectivity
    ✅ Service to Pod Connectivity
    ✅ Cross-Node Service Connectivity
    ✅ DNS Resolution
    ✅ NodePort Service Connectivity
    ✅ LoadBalancer Service Connectivity

🎉 Overall Result: All 6 diagnostic tests passed
```

### Step 6: Cleaning Up

When you're done testing, clean up your resources:

```bash
# Delete the test Kubernetes cluster
./delete_test_k8s.sh -f
```

### Next Steps

- Try running individual tests with `--test-list pod-to-pod,dns`
- Test against your production cluster by pointing to your kubeconfig
- Review the JSON reports in `test_results/` for detailed analysis
- Experiment with different Cilium network policies in the `cilium-policies/` directory

## Features

### Current Tests
- **Pod-to-Pod Connectivity**: Creates two `nicolaka/netshoot` pods on different worker nodes and tests connectivity using real ping commands
- **Service-to-Pod Connectivity**: Creates nginx deployment + service and tests HTTP connectivity and load balancing (DNS testing separated)
- **Cross-Node Service Connectivity**: Tests service connectivity from remote nodes to validate kube-proxy inter-node routing
- **DNS Resolution**: Dedicated DNS testing including service FQDN resolution, short names, and pod-to-pod DNS validation
- **NodePort Service Connectivity**: Tests external access to services through node ports, validating access from outside the cluster
- **LoadBalancer Service Connectivity**: Tests LoadBalancer service type functionality for cloud or on-premise deployments

### Key Capabilities
- **Real Pod Testing**: Uses actual Kubernetes pods, not simulated connections
- **Cross-Node Communication**: Tests networking between different worker nodes
- **Service Mesh Validation**: Comprehensive service discovery and load balancing testing
- **kube-proxy Testing**: Validates inter-node service routing and load balancing
- **Dedicated DNS Testing**: Separated DNS resolution testing for focused validation
- **Flexible HTTP Status Validation**: Accepts 2xx range status codes, not just 200
- **Load Balancing Verification**: Confirms traffic distribution across multiple replicas
- **Clean Architecture**: Separated concerns with single responsibility per test
- **Code Quality**: Zero duplication with reusable helper functions
- **Honest Output**: Accurate descriptions of actual implementation, no fake commands
- **Automatic Cleanup**: Creates and removes test resources automatically
- **Namespace Management**: Isolated testing environment with proper cleanup
- **Verbose Reporting**: Detailed test steps with equivalent kubectl commands
- **Educational Output**: Shows manual kubectl equivalents for learning
- **Network Policy Library**: Comprehensive collection of ready-to-use Cilium network policies

## Detailed Test Walkthroughs

### Test 1: Pod-to-Pod Connectivity

**Purpose:** Validates direct pod communication across different worker nodes, testing CNI networking and inter-node communication.

**Step-by-Step Process:**

1. **Validate Cluster Prerequisites**
   - Gets all worker nodes by filtering out control-plane/master nodes
   - Requires at least 2 worker nodes - fails immediately if fewer than 2
   - Reports: "✓ Found X worker nodes"

2. **Create Test Pods on Different Nodes**
   - Creates `netshoot-test-1` on `workerNodes[0]`
   - Creates `netshoot-test-2` on `workerNodes[1]`
   - Uses `nicolaka/netshoot` image (network troubleshooting toolkit)
   - Enforces node placement using `NodeName` field for cross-node testing
   - Sets 1-hour sleep command to keep pods running during test

3. **Wait for Pod Readiness**
   - 120-second timeout for each pod (allows time for image pull)
   - Polls every 2 seconds checking for `PodReady` condition
   - Reports: "✓ Pod netshoot-test-X is ready"

4. **Get Target Pod IP**
   - Retrieves Pod IP from `pod2.Status.PodIP`
   - Refreshes pod info if IP is initially empty
   - Reports: "✓ Pod netshoot-test-2 IP: X.X.X.X"

5. **Execute Cross-Node Ping Test**
   - Runs: `ping -c 3 -W 3 -i 1 [target_IP]`
   - 3 ping packets with 3-second timeout and 1-second intervals
   - Uses Kubernetes exec API to run command inside container

6. **Cleanup and Analyze Results**
   - Deletes both test pods immediately after test
   - Reports: "✓ Cleaned up test pods"
   - **Success patterns:** "0% packet loss", "3 packets transmitted" AND "3 received"
   - **Success message:** "Pod netshoot-test-2 is reachable from pod netshoot-test-1"

**What This Validates:**
- CNI networking functionality
- Inter-node pod communication
- Pod routing across cluster network
- Basic network connectivity between worker nodes

---

### Test 2: Service-to-Pod HTTP Connectivity

**Purpose:** Validates Kubernetes service discovery, HTTP connectivity, and load balancing across multiple pod replicas.

**Step-by-Step Process:**

1. **Create Nginx Deployment**
   - Creates deployment named `"web"` with 2 replicas
   - Uses `nginx:alpine` image (lightweight, 7MB)
   - Exposes port 80 on each container
   - Labels pods with `app: web`

2. **Wait for Deployment Readiness**
   - 120-second timeout for deployment to become ready
   - Ensures both nginx pods are running before proceeding
   - Polls every 2 seconds checking `deployment.Status.ReadyReplicas >= 2`

3. **Create ClusterIP Service**
   - Creates service named `"web"`
   - Type: `ClusterIP` (internal cluster access)
   - Selector: `app: web` (targets nginx pods)
   - Port mapping: Service port 80 → Target port 80

4. **Get Service ClusterIP**
   - Retrieves auto-assigned ClusterIP from `service.Spec.ClusterIP`
   - Equivalent to: `kubectl get svc web -n diagnostic-test -o jsonpath='{.spec.clusterIP}'`
   - Reports: "✓ Service IP is X.X.X.X"

5. **Create Test Pod and Test ICMP**
   - Creates `netshoot-service-test` pod (no specific node placement)
   - Tests: `ping -c 3 -W 3 [service_IP]`
   - **Expected:** Many clusters block ICMP to service IPs
   - Common result: "WARNING: ICMP ping to service IP failed (some clusters block ping)"

6. **Test HTTP Connectivity**
   - Primary test: `curl -s -o /dev/null -w "%{http_code}" http://web`
   - Uses service name (not IP) to test DNS resolution
   - Accepts 2xx status codes (200, 201, 202, etc.)
   - Verifies nginx welcome page in response content

7. **Test Load Balancing**
   - Makes 5 consecutive HTTP requests to service
   - 200ms delay between requests
   - Success criteria: At least 3/5 requests succeed
   - Validates service distributes requests across 2 nginx replicas

8. **Cleanup**
   - Deletes deployment, service, and test pod
   - Reports: "✓ Cleaned up all test resources"
   - **Success message:** "Service to Pod connectivity test passed - HTTP connectivity and load balancing working"

**What This Validates:**
- Service discovery (service name → ClusterIP)
- HTTP connectivity to services
- Load balancing across replicas
- DNS resolution (service names work)
- ClusterIP functionality

---

### Test 3: Cross-Node Service Connectivity

**Purpose:** Validates kube-proxy inter-node routing by ensuring services work when accessed from pods on different nodes than where target pods run.

**Step-by-Step Process:**

1. **Find Worker Nodes for Cross-Node Testing**
   - Gets all worker nodes by filtering out control-plane/master nodes
   - Requires at least 2 worker nodes - fails immediately if fewer than 2
   - Reports: "✓ Found 2 worker nodes for cross-node testing"

2. **Create Nginx Deployment**
   - Creates deployment named `"web-cross-node"` with 2 replicas
   - Uses `nginx:alpine` image
   - Labels pods with `app: web-cross-node`

3. **Wait for Deployment Readiness**
   - 120-second timeout for deployment to become ready
   - Ensures both nginx pods are running before proceeding
   - Reports: "✓ Deployment 'web-cross-node' is ready"

4. **Create Service and Get IP**
   - Creates ClusterIP service named `"web-cross-node"`
   - Selector: `app: web-cross-node`
   - Port 80 → 80 mapping
   - Retrieves auto-assigned ClusterIP
   - Reports: "✓ Service IP is X.X.X.X"

5. **Create Test Pod on Specific Node**
   - Creates `netshoot-cross-node-test` pod
   - **Critical:** Uses `NodeName` to force placement on specific node (worker node 2)
   - Ensures guaranteed cross-node service access
   - Reports: "✓ Created test pod 'netshoot-cross-node-test' on node X for cross-node testing"

6. **Test Cross-Node HTTP Connectivity**
   - Service name test: `curl -s -o /dev/null -w "%{http_code}" http://web-cross-node`
   - Validates kube-proxy routes traffic from remote node to nginx pods
   - Reports: "✓ Cross-node HTTP connectivity successful - Status: 200"

7. **Cleanup**
   - Deletes deployment, service, and test pod
   - Reports: "✓ Cleaned up all cross-node test resources"
   - **Success message:** "Cross-node service connectivity test passed - HTTP connectivity working across nodes"

**What This Validates:**
- kube-proxy inter-node routing
- Cross-node load balancing
- Service IP routing across nodes
- DNS + cross-node functionality
- Network policies don't block cross-node traffic

---

### Test 4: DNS Resolution

**Purpose:** Validates Kubernetes DNS infrastructure by testing service FQDN resolution.

**Step-by-Step Process:**

1. **Create DNS Test Environment**
   - Creates nginx deployment named `"web-dns"` with 2 replicas
   - Creates ClusterIP service named `"web-dns"`
   - Creates `netshoot-dns-test` pod with DNS tools (nslookup, dig)

2. **Test Service FQDN Resolution**
   - Constructs FQDN: `"web-dns.diagnostic-test.svc.cluster.local"`
   - Format: `[service].[namespace].svc.cluster.local`
   - Runs: `nslookup web-dns.diagnostic-test.svc.cluster.local`
   - Shows actual nslookup output for verification
   - **Success criteria:** Command completes without errors and returns the service IP

3. **Cleanup and Analyze Results**
   - Deletes deployment, service, and test pod
   - Reports: "✓ Cleaned up DNS test resources"
   - **Success message:** "DNS resolution test completed"

**What This Validates:**
- CoreDNS/kube-dns functionality
- Service discovery via DNS
- FQDN resolution (full kubernetes DNS names)
- DNS configuration in the cluster


---

### Test 5: NodePort Service Connectivity

**Purpose:** Validates the functionality of NodePort-type services, which expose applications externally via node ports accessible from outside the cluster.

**Step-by-Step Process:**

1. **Find Worker Nodes for NodePort Testing**
   - Gets all worker nodes by filtering out control-plane/master nodes
   - Requires at least 1 worker node with external IP/hostname
   - Reports: "✓ Found X worker nodes for NodePort testing"

2. **Create Nginx Deployment**
   - Creates deployment named `"web-nodeport"` with 2 replicas
   - Uses `nginx:alpine` image
   - Labels pods with `app: web-nodeport`
   - Reports: "✓ Created nginx deployment 'web-nodeport' with 2 replicas"

3. **Wait for Deployment Readiness**
   - 120-second timeout for deployment to become ready
   - Ensures all nginx pods are running before proceeding
   - Reports: "✓ Deployment 'web-nodeport' is ready"

4. **Create NodePort Service**
   - Creates service named `"web-nodeport"` with type NodePort
   - Selector: `app: web-nodeport`
   - Port 80 → 80 mapping, with dynamic NodePort assignment
   - Reports: "✓ Created NodePort service 'web-nodeport'"

5. **Get Assigned NodePort**
   - Retrieves auto-assigned NodePort from service specification
   - Typically in the 30000-32767 range
   - Reports: "✓ NodePort assigned: XXXX"

6. **Find Node IP for NodePort Access**
   - Gets External IP or hostname of a worker node
   - Selects the first available worker node
   - Reports: "✓ Found node IP for NodePort access: X.X.X.X"

7. **Create Test Pod to Access NodePort**
   - Creates `netshoot` pod to test NodePort access
   - Test pod simulates external client access
   - Reports: "✓ Created test pod to access NodePort service"

8. **Test HTTP Connectivity via NodePort**
   - Uses curl to access service via NodeIP:NodePort
   - Command: `curl -s -o /dev/null -w "%{http_code}" http://[node-ip]:[node-port]`
   - Validates external access works correctly
   - Reports: "✓ NodePort HTTP connectivity successful - Status: 200"

9. **Cleanup**
   - Deletes deployment, service, and test pod
   - Reports: "✓ Cleaned up all NodePort test resources"
   - **Success message:** "NodePort service connectivity test passed - HTTP connectivity working through node port"

**What This Validates:**
- NodePort service type functions correctly
- External access to services works
- kube-proxy configures host network correctly
- Node ports are properly exposed
- Traffic routing from external sources to pods works
- iptables/ipvs rules are configured correctly

---

### Test 6: LoadBalancer Service Connectivity

**Purpose:** Tests functionality of LoadBalancer-type services, which expose applications externally through cloud provider load balancers or on-premise equivalents.

**Step-by-Step Process:**

1. **Find Worker Nodes for LoadBalancer Testing**
   - Gets all worker nodes by filtering out control-plane/master nodes
   - Reports: "✓ Found X worker nodes for LoadBalancer testing"

2. **Create Nginx Deployment**
   - Creates deployment named `"web-loadbalancer"` with 2 replicas
   - Uses `nginx:alpine` image
   - Labels pods with `app: web-loadbalancer`
   - Reports: "✓ Created nginx deployment 'web-loadbalancer' with 2 replicas"

3. **Wait for Deployment Readiness**
   - 120-second timeout for deployment to become ready
   - Ensures all nginx pods are running before proceeding
   - Reports: "✓ Deployment 'web-loadbalancer' is ready"

4. **Create LoadBalancer Service**
   - Creates service named `"web-loadbalancer"` with type LoadBalancer
   - Selector: `app: web-loadbalancer`
   - Port 80 → 80 mapping
   - Reports: "✓ Created LoadBalancer service 'web-loadbalancer'"

5. **Get Service ClusterIP and Check External IP**
   - Retrieves auto-assigned ClusterIP
   - Checks for External IP (may be pending in local environments)
   - Reports: "✓ Service ClusterIP: X.X.X.X"
   - Reports: "ℹ️ No external IP assigned (expected in local environments)" OR
   - Reports: "✓ External IP assigned: X.X.X.X"

6. **Create Test Pod to Access LoadBalancer Service**
   - Creates `netshoot` pod to test service access
   - Reports: "✓ Created test pod to access LoadBalancer service"

7. **Test HTTP Connectivity**
   - In local environments without external IPs:
     - Reports: "ℹ️ Testing connectivity via ClusterIP (fallback for local environments)"
     - Uses service name: `curl -s -o /dev/null -w "%{http_code}" http://web-loadbalancer`
   - In cloud environments with external IPs:
     - Uses external IP: `curl -s -o /dev/null -w "%{http_code}" http://[external-ip]`
   - Reports: "✓ LoadBalancer HTTP connectivity successful - Status: 200"

8. **Cleanup**
   - Deletes deployment, service, and test pod
   - Reports: "✓ Cleaned up all LoadBalancer test resources"
   - **Success message:** "LoadBalancer service connectivity test passed - HTTP connectivity working via service"

**What This Validates:**
- LoadBalancer service type functions correctly
- Cloud provider integration works (if applicable)
- External IP assignment process works (in cloud environments)
- Graceful fallback to ClusterIP in local environments
- Traffic routing through load balancer to pods works
- Service discovery via service name works

---

## Quick Start

### 1. Create Test Cluster (Optional)

If you need a test Kubernetes cluster:

```bash
# Create test cluster with default settings
./build_test_k8s.sh

# Create test cluster with custom name
./build_test_k8s.sh -n my-test-cluster

# Delete test cluster
./delete_test_k8s.sh k8s-diagnostic-test
```

**Prerequisites for test cluster:**
- Docker (running)
- kind
- kubectl
- helm

The script creates a 3-node kind cluster (1 control-plane + 2 workers) with Cilium CNI.

### 2. Build the CLI Tool

```bash
# Using Makefile
make build

# Manual build
go build -o k8s-diagnostic .

# Build and install
make install
```

### 3. Run Connectivity Tests

```bash
# Test with default namespace (diagnostic-test)
./k8s-diagnostic test

# Test with custom namespace
./k8s-diagnostic test --namespace my-test-ns

# Test with verbose output
./k8s-diagnostic test --verbose

# Test with specific kubeconfig
./k8s-diagnostic test --kubeconfig /path/to/kubeconfig
```

### 4. Test Cilium Network Policies

```bash
# Apply a basic allow-all policy
kubectl apply -f cilium-policies/1-allow-all/allow-all-policy.yaml

# Apply a deny-all policy
kubectl apply -f cilium-policies/2-deny-all/deny-all-policy.yaml

# Test L4 policies
./cilium-policies/8-l4-policies/test-l4-policies.sh all
```

---

## Usage

### Build Script Options

```bash
./build_test_k8s.sh [OPTIONS]

OPTIONS:
    -n, --name NAME        Cluster name (default: k8s-connectivity-test)
    -h, --help             Show this help message

Features:
    • 3-node cluster (1 control-plane + 2 workers)
    • Cilium CNI (latest version)
    • Automatic system configuration (macOS/Linux)
    • Prerequisite checking
```

### Delete Script Options

```bash
./delete_test_k8s.sh [CLUSTER_NAME] [OPTIONS]

OPTIONS:
    -f, --force            Skip confirmation prompts
    -h, --help             Show this help message
```

### CLI Tool Options

```bash
./k8s-diagnostic test [OPTIONS]

OPTIONS:
    -n, --namespace string    Namespace to run tests in (default: "diagnostic-test")
    --kubeconfig string       Path to kubeconfig file
    -v, --verbose             Verbose output with detailed test steps and DEBUG level logs
    --test-group string       Run tests by group: networking (more groups coming soon)
    --test-list string        Comma-separated list of tests to run: pod-to-pod,service-to-pod,cross-node,dns,nodeport,loadbalancer
    --keep-namespace          Keep the test namespace after tests complete (useful for running multiple test sequences)
    
Global Options:
    --config string          Config file (default: $HOME/.k8s-diagnostic.yaml)
```

### Namespace Management

The tool includes intelligent namespace management to improve testing efficiency:

**Default Behavior:**
- When running **selective tests** (--test-list with subset of tests): The namespace is **preserved** after tests complete
- When running **all tests** (default or --test-group networking): The namespace is **cleaned up** after tests complete

**Override Options:**
- `--keep-namespace`: Forces namespace preservation regardless of test mode
- Manual cleanup: `kubectl delete namespace diagnostic-test`

**Benefits:**
- **Efficient Testing**: Run multiple test sequences without recreating the namespace each time
- **Resource Conservation**: Reduces API server load from repeated namespace creation/deletion
- **Faster Sequential Tests**: Significant time savings when running multiple selective tests
- **Automatic Cleanup**: Full test suite still performs proper cleanup

**Example Usage:**
```bash
# Run a single test - namespace persists for next test
./k8s-diagnostic test --test-list cross-node

# Run another test using the same namespace
./k8s-diagnostic test --test-list nodeport

# Run all tests - namespace automatically cleaned up
./k8s-diagnostic test --test-group networking

# Force namespace preservation even with all tests
./k8s-diagnostic test --test-group networking --keep-namespace

# Manual cleanup when done testing
kubectl delete namespace diagnostic-test
```

## Test Output

### Standard Output
```
Running connectivity diagnostic tests in namespace 'diagnostic-test'

🔍 Setting up test environment...
✅ Namespace diagnostic-test ready

🧪 Running diagnostic tests...
Test 1: 🔄 Pod-to-Pod Connectivity
✅ Test 1 PASSED: Both same-node and cross-node connectivity tests passed

Test 2: 🌐 Service to Pod Connectivity
✅ Test 2 PASSED: Service to Pod connectivity test passed - HTTP connectivity working

Test 3: 📡 Cross-Node Service Connectivity
✅ Test 3 PASSED: Cross-node service connectivity test passed - HTTP connectivity working across nodes

Test 4: 🔤 DNS Resolution
✅ Test 4 PASSED: DNS resolution test completed

Test 5: 🚪 NodePort Service Connectivity
✅ Test 5 PASSED: NodePort service connectivity test passed - HTTP connectivity working through node port

Test 6: ⚖️ LoadBalancer Service Connectivity
✅ Test 6 PASSED: LoadBalancer service connectivity test passed - HTTP connectivity working via service

🧹 Cleaning up test environment...
Namespace diagnostic-test cleaned up
[2025-07-10 14:51:39][INFO][test.go:271] JSON report saved: test_results/k8s-diagnostic-results-20250710-145139.json

📊 Test Summary:
  Total Tests: 6, Passed: 6, Failed: 0
  ✅ Passed Tests:
    ✅ Pod-to-Pod Connectivity
    ✅ Service to Pod Connectivity
    ✅ Cross-Node Service Connectivity
    ✅ DNS Resolution
    ✅ NodePort Service Connectivity
    ✅ LoadBalancer Service Connectivity

🎉 Overall Result: All 6 diagnostic tests passed
💡 Run with --verbose for detailed test steps

📁 Detailed results are stored in JSON file in the test_results/ folder for further analysis
```

### Verbose Output
Includes detailed information about:
- Configuration settings
- Worker node discovery and selection
- Pod creation and scheduling across nodes
- Deployment and service creation
- Pod readiness status and IP assignment
- DNS resolution testing (`nslookup` commands)
- Service IP retrieval (equivalent kubectl commands)
- HTTP connectivity testing with status codes
- Cross-node service routing validation
- Load balancing verification across replicas
- Real ping/curl command outputs
- Comprehensive cleanup operations
- Manual kubectl command equivalents for education

### Enhance
