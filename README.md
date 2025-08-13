# k8s-diagnostic

A CLI tool for testing Kubernetes network connectivity and validating Cilium network policies.

## Features

- **Core Connectivity Tests**: Pod-to-Pod, Service, DNS, Cross-Node, NodePort, LoadBalancer
- **Policy Testing**: L3 (CIDR, node), L4 (port-based), L7 (HTTP, DNS) policies
- **Visual Output**: Clear test results with progress indicators
- **Concurrent Testing**: Run multiple policy tests with resource isolation
- **Reporting**: JSON reports and structured logs for analysis
- **Production Ready**: Reliable connectivity validation with cleanup

## Tools Provided

- `k8s-diagnostic`: Main connectivity testing tool
- `build_test_k8s.sh`: Creates test cluster with Cilium CNI
- `delete_test_k8s.sh`: Removes test clusters
- `cilium-policies/`: Ready-to-use policy examples

## Cilium Network Policies

The repository includes organized Cilium network policies by category:

| Category | Description | Examples |
|----------|-------------|---------|
| Basic | Baseline policies | allow-all, deny-all |
| Namespace | Namespace filtering | same-namespace, deny-namespace |
| Label | Pod label filtering | same-label, deny-label |
| L3 | Network layer (IP) | CIDR-based, node-based |
| L4 | Transport layer | Port-based, protocol-specific |
| L7 | Application layer | HTTP, DNS filtering |

### Testing Policies

```bash
# Test L3 policies
./k8s-diagnostic test --test-group l3-policies

# Test L4 policies  
./k8s-diagnostic test --test-group l4-policies

# Test L7 policies
./k8s-diagnostic test --test-group l7-policies
```

## Available Tests

### Core Network Tests
- **Pod-to-Pod Connectivity**: Tests basic pod networking across nodes
- **Service-to-Pod**: Verifies service discovery and load balancing
- **Cross-Node Service**: Validates kube-proxy inter-node routing
- **DNS Resolution**: Tests DNS functionality for service discovery
- **NodePort Service**: Validates external access through node ports
- **LoadBalancer Service**: Tests cloud load balancer integration

### Policy Tests
- **Basic Policies**: Allow-all and deny-all baseline tests
- **L3 Policies**: Tests for network layer (IP-based) filtering
- **L4 Policies**: Tests for transport layer (port-based) filtering
- **L7 Policies**: Tests for application layer (HTTP, DNS) filtering

## Quick Start

### Prerequisites
- Docker (running)
- kind, kubectl, helm
- Go 1.21+ (for building from source)

### 1. Setup and Build
```bash
# Clone repository
git clone https://github.com/parlakisik/k8s_diagnostic.git
cd k8s_diagnostic

# Build tool
make build

# Create test cluster (optional)
./build_test_k8s.sh
```

### 2. Run Tests
```bash
# Basic connectivity tests
./k8s-diagnostic test

# Test L3 policies
./k8s-diagnostic test --test-group l3-policies

# Test L4 policies
./k8s-diagnostic test --test-group l4-policies

# Test L7 policies
./k8s-diagnostic test --test-group l7-policies
```

## Command Options

```bash
# Basic usage
./k8s-diagnostic test

# Test flags
--namespace string     Namespace for tests (default: "diagnostic-test")
--kubeconfig string    Custom kubeconfig path
--verbose              Detailed output with DEBUG logs
--test-group string    Run tests by group: networking, l3-policies, l4-policies, l7-policies
--test-list string     Run specific tests: pod-to-pod,service-to-pod (Limited support - see Test Execution Modes below)
--keep-namespace       Preserve namespace after tests
--l3-subgroups         Specify L3 policy subgroups to run
--l4-subgroups         Specify L4 policy subgroups to run
--l7-subgroups         Specify L7 policy subgroups to run
```

### Script Options

```bash
# Test cluster creation
./build_test_k8s.sh -n custom-cluster-name

# Test cluster deletion
./delete_test_k8s.sh -f
```

## L3 Policy Tests

L3 policy tests validate Cilium's Layer 3 network policies using sequential execution:

### L3 Test Subgroups

The L3 tests are organized into 7 subgroups:

| Subgroup | Tests | Description |
|----------|-------|-------------|
| ip-cidr | 3 tests | CIDR-based ingress/egress filtering with exceptions |
| endpoint | 1 test | Endpoint label selector policies |
| entities | 1 test | Built-in entity selectors (world, host, cluster) |
| dns | 1 test | DNS-based filtering policies |
| node | 4 tests | Node selectors, pod node filtering, node CIDR targeting |
| service | 1 test | Kubernetes service targeting |
| security | 2 tests | Baseline security policies (allow-all, deny-all) |

### Running L3 Tests

```bash
# CLI tool
./k8s-diagnostic test --test-group l3-policies
./k8s-diagnostic test --test-group l3-policies --l3-subgroups ip-cidr,entities
```

### Subgroup Combinations

You can run any combination of subgroups together:
- `ip-cidr` + `entities`: Runs CIDR-based and entity-based tests
- `entities` + `endpoint` + `service`: Runs label-based, entity-based, and service-based tests  
- `node` + `ip-cidr`: Runs all node-based and CIDR-based tests
- Any other combination based on your testing needs

## L3 Policy Test Guide

This section provides a focused guide to all **13 individual L3 policy tests**, explaining what each test validates and how to run them.

### IP-CIDR Subgroup (3 tests)

#### 1. **cidr-ingress** - CIDR Ingress Policy Test
**What it tests:** Validates that pods can receive traffic from specific IP address ranges using `fromCIDRSet`  
**Why we test it:** Ensures IP-based ingress filtering works correctly for network segmentation  
**How to run:**
```bash
# Run only CIDR tests (includes this one)
./k8s-diagnostic test --test-group l3-policies --l3-subgroups ip-cidr

# Run all L3 tests (includes this one)  
./k8s-diagnostic test --test-group l3-policies
```

#### 2. **cidr-egress** - CIDR Egress Policy Test
**What it tests:** Validates that pods can send traffic to specific IP address ranges using `toCIDR`  
**Why we test it:** Ensures IP-based egress filtering works for controlling outbound traffic  
**How to run:** Same commands as `cidr-ingress` (part of ip-cidr subgroup)

#### 3. **cidr-except** - CIDR with Exceptions Policy Test
**What it tests:** Validates complex CIDR rules with IP exclusions using `fromCIDRSet` with `except`  
**Why we test it:** Ensures granular IP filtering works (allow range but block specific IPs)  
**How to run:** Same commands as `cidr-ingress` (part of ip-cidr subgroup)

### Endpoint Subgroup (1 test)

#### 4. **endpoints-label** - Endpoint Label Selector Policy Test
**What it tests:** Validates pod-to-pod communication filtering based on source pod labels using `fromEndpoints`  
**Why we test it:** Core functionality for label-based micro-segmentation in Kubernetes  
**How to run:**
```bash
# Run only endpoint tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups endpoint

# Combine with other subgroups
./k8s-diagnostic test --test-group l3-policies --l3-subgroups endpoint,entities
```

### Entities Subgroup (1 test)

#### 5. **entities-based** - Predefined Entities Policy Test
**What it tests:** Validates filtering using Cilium's built-in entities (`host`, `cluster`, `world`) with `fromEntities`  
**Why we test it:** Ensures entity-based filtering works for common traffic patterns  
**How to run:**
```bash
# Run only entities tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups entities

# Combine with endpoint (both are 1 test each)
./k8s-diagnostic test --test-group l3-policies --l3-subgroups entities,endpoint
```

### DNS Subgroup (1 test)

#### 6. **dns-based** - DNS-Based Policy Test  
**What it tests:** Validates egress filtering based on domain names using `toFQDNs`  
**Why we test it:** Ensures DNS-based policies work for controlling external service access  
**How to run:**
```bash
# Run only DNS tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups dns

# Combine with service tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups dns,service
```

### Node Subgroup (4 tests)

#### 7. **node-selector** - Traditional Node Selector Policy Test
**What it tests:** Validates node-based filtering using `fromNodes` selector  
**Why we test it:** Tests node-based policies (requires `enable-node-selector-labels: true`)  
**How to run:**
```bash
# Run all node tests (4 tests total)
./k8s-diagnostic test --test-group l3-policies --l3-subgroups node

# Can combine with other subgroups as needed
```

#### 8. **pod-node-name** - Pod Node Name Policy Test
**What it tests:** Validates filtering based on pod's node location using `io.kubernetes.pod.nodeName` label  
**Why we test it:** Tests node-based filtering that works with default Cilium configuration  
**How to run:** Same as `node-selector` (part of node subgroup)

#### 9. **node-cidr** - Node CIDR Policy Test  
**What it tests:** Validates node-based filtering using CIDR ranges that correspond to node pod subnets  
**Why we test it:** Tests alternative approach to node-based filtering using IP ranges  
**How to run:** Same as `node-selector` (part of node subgroup)

#### 10. **node-based** - Node-Based Clusterwide Policy Test
**What it tests:** Validates cross-namespace node policies using `CiliumClusterwideNetworkPolicy`  
**Why we test it:** Ensures node-based policies work across namespace boundaries  
**How to run:** Same as `node-selector` (part of node subgroup)

### Service Subgroup (1 test)

#### 11. **kubernetes-service** - Kubernetes Service Policy Test
**What it tests:** Validates service-to-service communication filtering using `toServices`  
**Why we test it:** Ensures policies can target Kubernetes services by name and namespace  
**How to run:**
```bash
# Run only service tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups service

# Combine with other single-test subgroups
./k8s-diagnostic test --test-group l3-policies --l3-subgroups service,dns,entities
```

### Security Subgroup (2 tests)

#### 12. **allow-all** - Allow All Policy Test
**What it tests:** Validates baseline allow-all policies that permit all ingress and egress traffic  
**Why we test it:** Establishes baseline connectivity and ensures policy infrastructure works before restrictive tests  
**How to run:**
```bash
# Run only security tests (includes allow-all and deny-all)
./k8s-diagnostic test --test-group l3-policies --l3-subgroups security

# Combine with other subgroups (security tests run last to prevent conflicts)
./k8s-diagnostic test --test-group l3-policies --l3-subgroups ip-cidr,security
```

#### 13. **deny-all** - Deny All Policy Test
**What it tests:** Validates restrictive deny-all policies that block all ingress and egress traffic  
**Why we test it:** Ensures policy enforcement works and tests policy isolation capabilities  
**How to run:** Same commands as `allow-all` (part of security subgroup)

**Note:** Security tests are automatically run last to prevent interference with other policy tests.

### Common Test Execution Options

#### Run All L3 Tests (11 tests)
```bash
./k8s-diagnostic test --test-group l3-policies
```

#### Run with Verbose Output
```bash  
./k8s-diagnostic test --test-group l3-policies --verbose
```

#### Subgroup Combination Examples
```bash
# Run CIDR and entity tests
./k8s-diagnostic test --test-group l3-policies --l3-subgroups ip-cidr,entities

# Run multiple single-test subgroups  
./k8s-diagnostic test --test-group l3-policies --l3-subgroups endpoint,entities,dns

# Run node and CIDR tests together
./k8s-diagnostic test --test-group l3-policies --l3-subgroups ip-cidr,node
```

This guide provides everything you need to understand and run each of the 11 L3 policy tests individually or in any combination.

## L4 Policy Tests

L4 policy tests validate Cilium's Layer 4 network policies using sequential execution:

### L4 Test Subgroups

The L4 tests are organized into 3 subgroups:

| Subgroup | Tests | Description |
|----------|-------|-------------|
| port | 4 tests | TCP port-based ingress/egress filtering with ranges and multiple ports |
| icmp | 3 tests | ICMP and ICMPv6 protocol-based filtering |
| tls-sni | 3 tests | TLS Server Name Indication (SNI) filtering |

### Running L4 Tests

```bash
# CLI tool
./k8s-diagnostic test --test-group l4-policies
./k8s-diagnostic test --test-group l4-policies --l4-subgroups port,icmp

```

### Subgroup Combinations

You can run any combination of subgroups together:
- `port` + `icmp`: Runs port-based and ICMP-based tests
- `port` + `tls-sni`: Runs port-based and TLS SNI tests
- `icmp` + `tls-sni`: Runs ICMP-based and TLS SNI tests
- Any other combination based on your testing needs

## L4 Policy Test Guide

This section provides a focused guide to all **10 individual L4 policy tests**, explaining what each test validates and how to run them.

### Port Subgroup (4 tests)

#### 1. **tcp-port-ingress** - TCP Port Ingress Policy Test
**What it tests:** Validates that pods can receive TCP traffic on specific ports using `toPorts` ingress rules  
**Why we test it:** Ensures port-based ingress filtering works correctly for service access control  
**How to run:**
```bash
# Run only port tests (includes this one)
./k8s-diagnostic test --test-group l4-policies --l4-subgroups port

# Run all L4 tests (includes this one)
./k8s-diagnostic test --test-group l4-policies
```

#### 2. **tcp-port-egress** - TCP Port Egress Policy Test
**What it tests:** Validates that pods can send TCP traffic on specific ports using `toPorts` egress rules  
**Why we test it:** Ensures port-based egress filtering works for controlling outbound service connections  
**How to run:** Same commands as `tcp-port-ingress` (part of port subgroup)

#### 3. **port-range** - Port Range Policy Test
**What it tests:** Validates filtering using port ranges (e.g., `8080-8090`) instead of individual ports  
**Why we test it:** Ensures port range specifications work for services using multiple consecutive ports  
**How to run:** Same commands as `tcp-port-ingress` (part of port subgroup)

#### 4. **multiple-port** - Multiple Port Policy Test
**What it tests:** Validates policies that allow traffic on multiple specific ports (e.g., `80`, `443`, `8080`)  
**Why we test it:** Ensures multiple port specifications work for services using different ports  
**How to run:** Same commands as `tcp-port-ingress` (part of port subgroup)

### ICMP Subgroup (3 tests)

#### 5. **icmp-type** - ICMP Type Policy Test
**What it tests:** Validates ICMP protocol filtering using specific ICMP types (e.g., echo request/reply)  
**Why we test it:** Ensures ICMP-based network diagnostics can be controlled at the policy level  
**How to run:**
```bash
# Run only ICMP tests
./k8s-diagnostic test --test-group l4-policies --l4-subgroups icmp

# Combine with other subgroups
./k8s-diagnostic test --test-group l4-policies --l4-subgroups icmp,port
```

#### 6. **icmpv6-type** - ICMPv6 Type Policy Test
**What it tests:** Validates ICMPv6 protocol filtering for IPv6 networks using specific ICMPv6 types  
**Why we test it:** Ensures IPv6 ICMP traffic can be controlled separately from IPv4 ICMP  
**How to run:** Same commands as `icmp-type` (part of icmp subgroup)

#### 7. **mixed-icmp** - Mixed ICMP Policy Test
**What it tests:** Validates policies that handle both ICMP and ICMPv6 traffic simultaneously  
**Why we test it:** Ensures dual-stack (IPv4/IPv6) ICMP policies work correctly  
**How to run:** Same commands as `icmp-type` (part of icmp subgroup)

### TLS-SNI Subgroup (3 tests)

#### 8. **basic-sni** - Basic SNI Policy Test
**What it tests:** Validates TLS Server Name Indication (SNI) filtering for specific domain names  
**Why we test it:** Ensures TLS traffic can be filtered based on the requested domain name  
**How to run:**
```bash
# Run only TLS SNI tests
./k8s-diagnostic test --test-group l4-policies --l4-subgroups tls-sni

# Combine with port tests
./k8s-diagnostic test --test-group l4-policies --l4-subgroups tls-sni,port
```

#### 9. **multi-domain-sni** - Multi-Domain SNI Policy Test
**What it tests:** Validates SNI policies that allow TLS connections to multiple domains  
**Why we test it:** Ensures applications can connect to multiple external services with SNI filtering  
**How to run:** Same commands as `basic-sni` (part of tls-sni subgroup)

#### 10. **combined-l4-sni** - Combined L4 and SNI Policy Test
**What it tests:** Validates policies combining both port restrictions and SNI domain filtering  
**Why we test it:** Ensures complex L4 policies with both port and TLS domain restrictions work together  
**How to run:** Same commands as `basic-sni` (part of tls-sni subgroup)

### Common Test Execution Options

#### Run All L4 Tests (10 tests)
```bash
./k8s-diagnostic test --test-group l4-policies
```

#### Run with Verbose Output
```bash
./k8s-diagnostic test --test-group l4-policies --verbose
```

#### Subgroup Combination Examples
```bash
# Run port and ICMP tests
./k8s-diagnostic test --test-group l4-policies --l4-subgroups port,icmp

# Run TLS SNI tests only
./k8s-diagnostic test --test-group l4-policies --l4-subgroups tls-sni

# Run all three subgroups
./k8s-diagnostic test --test-group l4-policies --l4-subgroups port,icmp,tls-sni
```

This guide provides everything you need to understand and run each of the 10 L4 policy tests individually or in any combination.

## L7 Policy Tests

L7 policy tests validate Cilium's Layer 7 application-layer network policies using sequential execution:

### L7 Test Subgroups

The L7 tests are organized into 2 subgroups:

| Subgroup | Tests | Description |
|----------|-------|-------------|
| http | 3 tests | HTTP protocol filtering with methods, paths, and headers |
| dns | 2 tests | DNS query filtering with name matching and pattern matching |

### Running L7 Tests

```bash
# CLI tool
./k8s-diagnostic test --test-group l7-policies
./k8s-diagnostic test --test-group l7-policies --l7-subgroups http,dns
```

### Subgroup Combinations

You can run any combination of subgroups together:
- `http` + `dns`: Runs all HTTP-based and DNS-based tests
- `http` only: Runs HTTP method, path, and header filtering tests
- `dns` only: Runs DNS name and pattern matching tests
- Any other combination based on your testing needs

## L7 Policy Test Guide

This section provides a focused guide to all **5 individual L7 policy tests**, explaining what each test validates and how to run them.

### HTTP Subgroup (3 tests)

#### 1. **basic-http-get** - Basic HTTP GET Policy Test
**What it tests:** Validates HTTP method filtering allowing only GET requests using L7 HTTP policy rules  
**Why we test it:** Ensures application-layer HTTP method restrictions work correctly for API endpoint protection  
**How to run:**
```bash
# Run only HTTP tests (includes this one)
./k8s-diagnostic test --test-group l7-policies --l7-subgroups http

# Run all L7 tests (includes this one)
./k8s-diagnostic test --test-group l7-policies
```

#### 2. **http-with-headers** - HTTP With Headers Policy Test
**What it tests:** Validates HTTP header-based filtering using specific header requirements in L7 policies  
**Why we test it:** Ensures application-layer header validation works for authentication and API versioning  
**How to run:** Same commands as `basic-http-get` (part of http subgroup)

#### 3. **path-method** - Path Method Policy Test
**What it tests:** Validates HTTP path and method combination filtering using L7 path-based rules  
**Why we test it:** Ensures granular endpoint protection based on both URL paths and HTTP methods  
**How to run:** Same commands as `basic-http-get` (part of http subgroup)

### DNS Subgroup (2 tests)

#### 4. **dns-matchname** - DNS Match Name Policy Test
**What it tests:** Validates DNS query filtering for specific domain names using `matchName` rules  
**Why we test it:** Ensures L7 DNS policies can control access to specific external services by domain name  
**How to run:**
```bash
# Run only DNS tests (includes this one)
./k8s-diagnostic test --test-group l7-policies --l7-subgroups dns

# Combine with HTTP tests
./k8s-diagnostic test --test-group l7-policies --l7-subgroups dns,http
```

#### 5. **dns-matchpattern** - DNS Match Pattern Policy Test
**What it tests:** Validates DNS query filtering using pattern matching with `matchPattern` for domain wildcards  
**Why we test it:** Ensures L7 DNS policies can control access to domain patterns (e.g., *.example.com)  
**How to run:** Same commands as `dns-matchname` (part of dns subgroup)

### Common Test Execution Options

#### Run All L7 Tests (5 tests)
```bash
./k8s-diagnostic test --test-group l7-policies
```

#### Run with Verbose Output
```bash
./k8s-diagnostic test --test-group l7-policies --verbose
```

#### Subgroup Combination Examples
```bash
# Run HTTP and DNS tests
./k8s-diagnostic test --test-group l7-policies --l7-subgroups http,dns

# Run HTTP tests only
./k8s-diagnostic test --test-group l7-policies --l7-subgroups http

# Run DNS tests only
./k8s-diagnostic test --test-group l7-policies --l7-subgroups dns
```

This guide provides everything you need to understand and run each of the 5 L7 policy tests individually or in any combination.

## Output Format

Test results include:
- Visual indicators (✅/❌) for each test result
- Timestamped logs with test progress
- Detailed JSON reports in the `test_results/` folder
- Verbose option (--verbose) for detailed debug information
