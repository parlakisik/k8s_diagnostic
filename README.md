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

### Key Capabilities
- **Real Pod Testing**: Uses actual Kubernetes pods, not simulated connections
- **Cross-Node Communication**: Tests networking between different worker nodes
- **Automatic Cleanup**: Creates and removes test resources automatically
- **Namespace Management**: Isolated testing environment with proper cleanup
- **Verbose Reporting**: Detailed test steps and ping output
- **Multiple Test Support**: Architecture ready for additional test types

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
🚀 Running pod-to-pod diagnostic test in namespace 'diagnostic-test'

🔧 Setting up test environment...
✓ Namespace diagnostic-test ready

🧪 Running diagnostic tests...
📋 Test 1: Pod-to-Pod Connectivity
✅ Test 1 PASSED: Pod netshoot-test-2 is reachable from pod netshoot-test-1

🧹 Cleaning up test environment...
✓ Namespace diagnostic-test cleaned up
✅ Test Result: All diagnostic tests passed
💡 Run with --verbose for detailed test steps
```

### Verbose Output
Includes detailed information about:
- Configuration settings
- Worker node discovery
- Pod creation and scheduling
- Pod readiness status
- IP address assignment
- Actual ping command output
- Cleanup operations

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