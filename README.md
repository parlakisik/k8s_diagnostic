# k8s-diagnostic

A CLI tool for testing network connectivity within Kubernetes clusters using real pod-to-pod communication tests.

## Overview

This project provides:
- **`build_test_k8s.sh`** - Script to create a test Kubernetes cluster using kind with Cilium CNI
- **`delete_test_k8s.sh`** - Script to delete test clusters
- **`k8s-diagnostic`** - CLI tool for running comprehensive diagnostic tests in any Kubernetes cluster

## Features

### Current Tests
- **Pod-to-Pod Connectivity**: Creates two `nicolaka/netshoot` pods on different worker nodes and tests connectivity using real ping commands
- **Service-to-Pod Connectivity**: Creates nginx deployment + service and tests HTTP connectivity and load balancing (DNS testing separated)
- **Cross-Node Service Connectivity**: Tests service connectivity from remote nodes to validate kube-proxy inter-node routing
- **DNS Resolution**: Dedicated DNS testing including service FQDN resolution, short names, and pod-to-pod DNS validation

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
   - Common result: "⚠️ ICMP ping to service IP failed (some clusters block ping)"

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

1. **Create Nginx Deployment**
   - Creates deployment named `"web-cross"` with 2 replicas
   - Uses `nginx:alpine` image
   - Labels pods with `app: web-cross`

2. **Discover Nginx Pod Node Locations**
   - Lists all pods with label `app=web-cross`
   - Maps each pod to its worker node using `pod.Spec.NodeName`
   - Deduplicates node names
   - Reports: "✓ Nginx pods running on nodes: [node1, node2]"

3. **Find Different Worker Node**
   - Gets all worker nodes in cluster
   - Excludes nodes where nginx pods are running
   - Selects different node for cross-node testing
   - **Fallback:** If all nodes have nginx pods, uses first worker node
   - Reports: "✓ Selected different node 'worker-node-X' for cross-node test"

4. **Create Service and Get IP**
   - Creates ClusterIP service named `"web-cross"`
   - Selector: `app: web-cross`
   - Port 80 → 80 mapping
   - Retrieves auto-assigned ClusterIP

5. **Create Test Pod on Remote Node**
   - Creates `netshoot-cross-node-test` pod
   - **Critical:** Uses `NodeName` to force placement on different node
   - Ensures guaranteed cross-node service access
   - 120-second timeout for readiness

6. **Test Cross-Node HTTP Connectivity**
   - Service name test: `curl -s -o /dev/null -w "%{http_code}" http://web-cross`
   - Direct IP test: `curl -s -o /dev/null -w "%{http_code}" http://[service_IP]`
   - Validates kube-proxy routes traffic from remote node to nginx pods
   - Verifies nginx welcome page in responses

7. **Cleanup**
   - Deletes deployment, service, and test pod
   - **Success message:** "Cross-node service connectivity validated - kube-proxy inter-node routing confirmed"

**What This Validates:**
- kube-proxy inter-node routing
- Cross-node load balancing
- Service IP routing across nodes
- DNS + cross-node functionality
- Network policies don't block cross-node traffic

---

### Test 4: DNS Resolution

**Purpose:** Comprehensively validates Kubernetes DNS infrastructure including service discovery, FQDN resolution, DNS search domains, and pod-to-pod DNS.

**Step-by-Step Process:**

1. **Create DNS Test Environment**
   - Creates nginx deployment named `"web-dns"` with 2 replicas
   - Creates ClusterIP service named `"web-dns"`
   - Creates `netshoot-dns-test` pod with DNS tools (nslookup, dig)

2. **Test Service FQDN Resolution**
   - Constructs FQDN: `"web-dns.diagnostic-test.svc.cluster.local"`
   - Format: `[service].[namespace].svc.cluster.local`
   - Runs: `nslookup web-dns.diagnostic-test.svc.cluster.local`
   - **Success criteria:** Response contains both "Name:" and "Address:" fields
   - Shows actual nslookup output for verification

3. **Test Short Name Resolution**
   - Uses just service name: `"web-dns"`
   - Runs: `nslookup web-dns`
   - Relies on Kubernetes DNS search domains in `/etc/resolv.conf`
   - Tests DNS search domain configuration
   - Shows actual resolution output

4. **Test Pod-to-Pod DNS Resolution**
   - Gets one nginx pod IP from web-dns deployment
   - Runs reverse lookup: `nslookup [pod_IP]`
   - Tests whether pod IPs can be resolved via DNS
   - Often inconclusive but provides additional DNS validation

5. **Determine Overall Success**
   - **Primary criteria:** Pod-to-pod DNS test must succeed AND short name resolution must return results
   - **Success condition:** `(pod-to-pod DNS err == nil) && (shortResult != "")`
   - **Success message:** "DNS resolution test passed - service FQDN and short name resolution working"
   - **Failure message:** "DNS resolution test failed - check cluster DNS configuration"
   - **Note:** Current implementation has a bug where FQDN success isn't properly tracked in final determination

**What This Validates:**
- CoreDNS/kube-dns functionality
- Service discovery via DNS
- FQDN resolution (full kubernetes DNS names)
- DNS search domains (short names resolve)
- DNS configuration (`/etc/resolv.conf` setup)
- Pod DNS functionality

**Common Issues Detected:**
- CoreDNS not running or misconfigured
- DNS service not accessible from pods
- Search domain misconfiguration
- Network policies blocking DNS traffic
- DNS service discovery broken

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
./k8s-connectivity test [OPTIONS]

OPTIONS:
    -n, --namespace string    Namespace to run tests in (default: "diagnostic-test")
    --kubeconfig string       Path to kubeconfig file
    -v, --verbose            Verbose output with detailed test steps

Global Options:
    --config string          Config file (default: $HOME/.k8s-diagnostic.yaml)
```

## Test Output

### Standard Output
```
🚀 Running connectivity diagnostic tests in namespace 'diagnostic-test'

🔧 Setting up test environment...
✓ Namespace diagnostic-test ready

🧪 Running diagnostic tests...
📋 Test 1: Pod-to-Pod Connectivity
✅ Test 1 PASSED: Pod netshoot-test-2 is reachable from pod netshoot-test-1

📋 Test 2: Service to Pod Connectivity
✅ Test 2 PASSED: Service to Pod connectivity test passed - HTTP connectivity and load balancing working

📋 Test 3: Cross-Node Service Connectivity
✅ Test 3 PASSED: Cross-node service connectivity validated - kube-proxy inter-node routing confirmed

📋 Test 4: DNS Resolution
✅ Test 4 PASSED: DNS resolution test passed - service FQDN and short name resolution working

🧹 Cleaning up test environment...
✓ Namespace diagnostic-test cleaned up
📄 JSON report saved: test_results/k8s-diagnostic-results-20250702-101146.json

📊 Test Summary:
  Total Tests: 4, Passed: 4, Failed: 0
  ✅ Passed Tests:
    • Pod-to-Pod Connectivity
    • Service to Pod Connectivity
    • Cross-Node Service Connectivity
    • DNS Resolution

✅ Overall Result: All 4 diagnostic tests passed
💡 Run with --verbose for detailed test steps
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

### JSON Result Files

**Every test execution automatically generates a comprehensive JSON report** saved to the `test_results/` directory. These files provide structured data perfect for monitoring dashboards, CI/CD integration, and historical analysis.

#### File Naming and Location
```
test_results/k8s-diagnostic-results-YYYYMMDD-HHMMSS.json
```
- **New file per execution** - no overwriting
- **Timestamped filenames** prevent conflicts
- **Organized storage** in dedicated directory

#### Smart Detail Strategy

The JSON logging uses an intelligent approach to balance file size with debugging capability:

**✅ Successful Tests (Clean Format):**
```json
{
  "test_number": 1,
  "test_name": "Pod-to-Pod Connectivity",
  "status": "PASSED",
  "success_message": "Pod netshoot-test-2 is reachable from pod netshoot-test-1",
  "details": [],
  "execution_time_seconds": 6.12
}
```

**❌ Failed Tests (Full Debug Details):**
```json
{
  "test_number": 1,
  "test_name": "Pod-to-Pod Connectivity", 
  "status": "FAILED",
  "error_message": "Pod netshoot-test-2 is not reachable from pod netshoot-test-1",
  "details": [
    "✓ Found 2 worker nodes",
    "✓ Created pod netshoot-test-1 on node diag-sandbox-worker",
    "✓ Pod netshoot-test-2 IP: 10.0.1.160",
    "✗ Ping failed - pod netshoot-test-2 is not reachable from pod netshoot-test-1",
    "  Ping output: PING 10.0.1.160 (10.0.1.160) 56(84) bytes of data..."
  ],
  "execution_time_seconds": 6.17
}
```

#### Complete JSON Structure

Each JSON file contains:

```json
{
  "execution_info": {
    "timestamp": "2025-07-02T10:11:19-07:00",
    "filename": "k8s-diagnostic-results-20250702-101146.json", 
    "namespace": "diagnostic-test",
    "kubeconfig_source": "default",
    "verbose_mode": false
  },
  "tests": [
    {
      "test_number": 1,
      "test_name": "Pod-to-Pod Connectivity",
      "description": "Validates direct pod communication across different worker nodes...",
      "status": "PASSED",
      "success_message": "Pod netshoot-test-2 is reachable from pod netshoot-test-1",
      "details": [],
      "start_time": "2025-07-02T10:11:19-07:00",
      "end_time": "2025-07-02T10:11:25-07:00", 
      "execution_time_seconds": 6.12
    }
  ],
  "summary": {
    "total_tests": 4,
    "passed": 4,
    "failed": 0,
    "overall_status": "PASSED",
    "total_execution_time_seconds": 27.16,
    "errors_encountered": null,
    "completion_time": "2025-07-02T10:11:46-07:00"
  }
}
```

#### Example Files

Two example JSON files demonstrate the different output formats:

**1. All Tests Successful** (`test_results/k8s-diagnostic-results-20250702-101146.json`)
- **Size:** ~1.5KB (compact)
- **All tests:** `"status": "PASSED"` with `"details": []`
- **Perfect for:** Monitoring dashboards, CI/CD success validation, routine health checks

**2. Mixed Results with Failure** (`test_results/k8s-diagnostic-results-20250702-101515.json`)
- **Size:** ~5KB (includes debug details)
- **Failed test:** Complete debugging information in `"details"` array
- **Successful tests:** Still clean with `"details": []`
- **Perfect for:** Error investigation, troubleshooting, post-incident analysis

#### Use Cases

**Monitoring and Dashboards:**
```bash
# Extract key metrics
cat test_results/k8s-diagnostic-results-*.json | jq '.summary'

# Check overall status
cat test_results/k8s-diagnostic-results-*.json | jq '.summary.overall_status'

# Get execution time
cat test_results/k8s-diagnostic-results-*.json | jq '.summary.total_execution_time_seconds'
```

**CI/CD Integration:**
```bash
# Run tests and parse JSON results
./k8s-diagnostic test
LATEST_RESULT=$(ls -t test_results/*.json | head -1)
STATUS=$(cat "$LATEST_RESULT" | jq -r '.summary.overall_status')

if [[ "$STATUS" == "PASSED" ]]; then
  echo "✅ All connectivity tests passed"
  exit 0
else
  echo "❌ Connectivity tests failed"
  cat "$LATEST_RESULT" | jq '.summary.errors_encountered'
  exit 1
fi
```

**Error Analysis:**
```bash
# Find failed tests with full details
cat test_results/*.json | jq '.tests[] | select(.status == "FAILED") | {test_name, error_message, details}'

# Get timing analysis
cat test_results/*.json | jq '.tests[] | {test_name, execution_time_seconds}' 
```

#### Key Benefits

- **Compact successful runs:** ~80% smaller JSON files when all tests pass
- **Rich failure debugging:** Complete diagnostic information when needed
- **Historical tracking:** Every execution preserved with timestamps
- **Integration ready:** Structured format perfect for automation
- **Zero information loss:** All debugging data available for failed tests

## Use Cases

### Testing Kind Cluster
```bash
# Create cluster
./build_test_k8s.sh

# Run tests
./k8s-diagnostic test --verbose

# Cleanup
./delete_test_k8s.sh k8s-diagnostic-test
```

### Testing Production Cluster
```bash
# Point to your cluster
export KUBECONFIG=/path/to/your/kubeconfig

# Run tests in custom namespace
./k8s-diagnostic test --namespace prod-diagnostic-test

# Or specify kubeconfig directly
./k8s-diagnostic test --kubeconfig /path/to/kubeconfig --namespace test-env
```

### CI/CD Integration
```bash
# Non-interactive testing
./k8s-diagnostic test --namespace ci-test-$(date +%s)
echo "Exit code: $?"
```

## Development

### Building

```bash
# Install dependencies
go mod tidy

# Build binary
go build -o k8s-diagnostic .

# Run tests
go test ./...

# Build with make
make build
make test
make clean
```

### Available Make Targets

```bash
make help        # Show available targets
make build       # Build the binary
make test        # Run Go tests
make clean       # Clean build files
make deps        # Download dependencies
make install     # Build and install to $GOPATH/bin
```

### Adding New Tests

The architecture supports multiple test types. To add a new test:

#### 1. Add Test Method to Tester

```go
// internal/diagnostic/tester.go

// TestDNSResolution tests DNS resolution within the cluster
func (t *Tester) TestDNSResolution(ctx context.Context) TestResult {
    var details []string
    
    // Your test implementation here
    // ...
    
    return TestResult{
        Success: true,
        Message: "DNS resolution test passed",
        Details: details,
    }
}
```

#### 2. Add Test to Command

```go
// cmd/test.go

// Add after Test 1: Pod-to-Pod Connectivity
fmt.Printf("📋 Test 2: DNS Resolution\n")
result2 := tester.TestDNSResolution(ctx)

if result2.Success {
    fmt.Printf("✅ Test 2 PASSED: %s\n", result2.Message)
} else {
    fmt.Printf("❌ Test 2 FAILED: %s\n", result2.Message)
}

// Update overall result logic
allTestsPassed := result1.Success && result2.Success
```

#### 3. Test Structure Guidelines

- **Namespace agnostic**: Don't create/delete namespaces in test methods
- **Resource cleanup**: Clean up any resources created during the test
- **Detailed logging**: Provide step-by-step details for verbose mode
- **Error handling**: Return clear error messages
- **Context awareness**: Respect context cancellation

### Example Test Implementation

```go
func (t *Tester) TestServiceConnectivity(ctx context.Context) TestResult {
    var details []string
    
    // Create a test service
    service, err := t.createTestService(ctx)
    if err != nil {
        return TestResult{
            Success: false,
            Message: fmt.Sprintf("Failed to create test service: %v", err),
            Details: details,
        }
    }
    details = append(details, "✓ Created test service")
    
    // Test diagnostic connectivity to service
    // ... test implementation
    
    // Cleanup
    t.cleanupService(ctx, service.Name)
    details = append(details, "✓ Cleaned up test service")
    
    return TestResult{
        Success: true,
        Message: "Service diagnostic test passed",
        Details: details,
    }
}
```

## Project Structure

```
k8s_diagnostic/
├── build_test_k8s.sh           # Create test cluster script
├── delete_test_k8s.sh          # Delete test cluster script
├── main.go                     # CLI entry point
├── cmd/                        # CLI commands
│   ├── root.go                 # Root command with global flags
│   └── test.go                 # Test command implementation
├── internal/                   # Internal packages
│   ├── diagnostic/           # Diagnostic testing logic
│   │   └── tester.go          # Test implementations
│   └── config/                # Configuration handling
│       └── config.go          # Config management
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
├── Makefile                   # Build automation
└── README.md                  # This documentation
```

## Requirements

### System Requirements
- Go 1.21+
- Docker (for kind clusters)
- kubectl
- Access to a Kubernetes cluster

### Kubernetes Requirements
- At least 2 worker nodes (for pod-to-pod tests)
- Ability to create namespaces
- Ability to create pods
- Container runtime that supports `nicolaka/netshoot` image

### Permissions Required
The tool needs the following Kubernetes permissions:
- Create/delete namespaces
- Create/delete pods
- List nodes
- Execute commands in pods (for ping tests)

## Troubleshooting

### Common Issues

**"Need at least 2 worker nodes"**
- Ensure your cluster has multiple worker nodes
- Check with `kubectl get nodes`

**"Pod did not become ready"**
- Check if the cluster can pull `nicolaka/netshoot` image
- Verify cluster has sufficient resources
- Check pod events: `kubectl describe pod -n diagnostic-test`

**"Namespace is being terminated"**
- The tool automatically waits for namespace termination
- If stuck, manually delete: `kubectl delete ns diagnostic-test --force --grace-period=0`

**Permission Denied**
- Ensure your kubeconfig has sufficient permissions
- Check RBAC policies if using service accounts

### Debug Mode

For debugging issues:
```bash
# Enable verbose output
./k8s-diagnostic test --verbose

# Check cluster status
kubectl get nodes
kubectl get pods --all-namespaces

# Check events
kubectl get events --sort-by='.lastTimestamp'
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `make test`
5. Submit a pull request

### Code Style
- Follow standard Go conventions
- Add comments for exported functions
- Include error handling
- Write tests for new functionality

## License

This project is licensed under the MIT License - see the LICENSE file for details.
