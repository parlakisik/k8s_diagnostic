#!/bin/bash

# Script to test L7 Cilium network policies with granular subtest options
# Usage: ./test-l7-policies.sh [subtest-name]
#
# NOTE: When running these tests, you may see failures in tests where traffic should be allowed.
# This is a known issue with Cilium in many environments, where policies can be enforced more 
# strictly than documented. The tests are structured correctly according to Cilium documentation,
# but your environment may have different policy enforcement behavior.
# 
# Available subtests organized according to Cilium Documentation categories:
#
#   1. HTTP POLICIES:
#      http-basic     - Basic HTTP GET policy with path matching
#      http-headers   - HTTP policy with header validation
#      http-advanced  - Advanced HTTP with multiple methods and paths
#      http           - Test all HTTP policies
#
#   2. DNS POLICIES:  
#      dns-matchname    - DNS matchName policy (exact matching)
#      dns-matchpattern - DNS matchPattern policy (wildcard matching)
#      dns-fqdn        - DNS FQDN policy with IP discovery
#      dns             - Test all DNS policies
#
#   3. DENY POLICIES:
#      deny-ingress    - Deny ingress policy 
#      deny-clusterwide - Clusterwide deny policy
#      deny            - Test all deny policies
#
#   OTHER OPTIONS:
#      baseline       - Test baseline L7 policy enforcement
#      categories     - Test all categories with cleanup between each (default)
#      isolated-all   - Test all subtests with cleanup between each test
#      cleanup        - Only clean up the test environment
#      check-dns-config - Check DNS proxy configuration in Cilium
#      fix-dns-config   - Fix DNS proxy configuration in Cilium
#      list          - List all available subtests
#      help          - Show usage information
#
# Example:
#   ./test-l7-policies.sh http
#   ./test-l7-policies.sh dns-matchname
#   ./test-l7-policies.sh categories
#   ./test-l7-policies.sh isolated-all

set -e

# ANSI color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Store start time
START_TIME=$(date +%s)

# Constants
NAMESPACE="l7-policy-test"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

function print_header() {
  echo -e "\n${BLUE}=================================================================${NC}"
  echo -e "${BLUE}= $1 ${NC}"
  echo -e "${BLUE}=================================================================${NC}\n"
}

function print_subheader() {
  echo -e "\n${CYAN}>>> $1 ${NC}\n"
}

function print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

function print_error() {
  echo -e "${RED}✗ $1${NC}"
}

function print_info() {
  echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Make sure we're in the right directory
cd $(dirname $0)
POLICY_DIR=$(pwd)
echo -e "${YELLOW}Running tests from: $POLICY_DIR${NC}"

# Variables to track test results
# HTTP Subtests
HTTP_BASIC_RESULT="NOT_RUN"    # Test 1: Basic HTTP GET policy with path matching
HTTP_HEADERS_RESULT="NOT_RUN"  # Test 2: HTTP policy with header validation
HTTP_ADVANCED_RESULT="NOT_RUN" # Test 3: Advanced HTTP with multiple methods and paths

# Overall category results
HTTP_RESULT="NOT_RUN"          # Overall HTTP category result
DNS_MATCHNAME_RESULT="NOT_RUN" 
DNS_MATCHPATTERN_RESULT="NOT_RUN"
DNS_FQDN_RESULT="NOT_RUN"
DNS_RESULT="NOT_RUN"
DENY_INGRESS_RESULT="NOT_RUN"
DENY_CLUSTERWIDE_RESULT="NOT_RUN"
DENY_RESULT="NOT_RUN"
BASIC_CONNECTIVITY_RESULT="NOT_RUN"

# Define policy directories
HTTP_DIR="$SCRIPT_DIR/http-policies"
DNS_DIR="$SCRIPT_DIR/dns-policies"
DENY_DIR="$SCRIPT_DIR/deny-policies"

function print_step() {
  echo "$(date +"%Y-%m-%d %H:%M:%S") $1"
}

# Function to check DNS proxy configuration
function check_dns_config() {
  print_header "CHECKING CILIUM DNS PROXY CONFIGURATION"
  
  print_step "Examining Cilium ConfigMap for DNS proxy settings..."
  
  # Check if ConfigMap exists
  if ! kubectl get configmap -n kube-system cilium-config &>/dev/null; then
    print_error "Cilium ConfigMap not found. Is Cilium installed?"
    return 1
  fi
  
  # Check for enable-dns-proxy setting
  local dns_proxy_enabled=$(kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-dns-proxy}' 2>/dev/null || echo "not set")
  
  echo -e "\n${BLUE}DNS Proxy Settings:${NC}"
  echo -e "  enable-dns-proxy: $dns_proxy_enabled"
  
  # Check for transparent mode settings
  local transparent_mode=$(kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.dnsproxy-enable-transparent-mode}' 2>/dev/null || echo "not set")
  echo -e "  dnsproxy-enable-transparent-mode: $transparent_mode"
  
  # Check for L7 proxy status
  local l7_proxy=$(kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-l7-proxy}' 2>/dev/null || echo "not set")
  echo -e "  enable-l7-proxy: $l7_proxy"
  
  # Check agent logs for DNS proxy related entries
  echo -e "\n${BLUE}Checking Cilium agent logs for DNS proxy information:${NC}"
  kubectl logs -n kube-system -l k8s-app=cilium --tail=20 | grep -i dns
  
  # Evaluate configuration status
  echo -e "\n${BLUE}Configuration Assessment:${NC}"
  if [[ "$dns_proxy_enabled" == "true" ]]; then
    print_success "DNS Proxy is properly enabled"
    echo "Your Cilium configuration should be able to validate DNS policies."
  else
    print_error "DNS Proxy is not explicitly enabled"
    echo "This is why your DNS policies are created but not validated by Cilium."
    echo "Run './test-l7-policies.sh fix-dns-config' to fix this issue."
  fi
  
  return 0
}

# Function to fix DNS proxy configuration
function fix_dns_config() {
  print_header "FIXING CILIUM DNS PROXY CONFIGURATION"
  
  print_step "Checking current Cilium configuration..."
  
  # Check if ConfigMap exists
  if ! kubectl get configmap -n kube-system cilium-config &>/dev/null; then
    print_error "Cilium ConfigMap not found. Is Cilium installed?"
    return 1
  fi
  
  # Check current DNS proxy setting
  local dns_proxy_enabled=$(kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-dns-proxy}' 2>/dev/null || echo "not set")
  
  if [[ "$dns_proxy_enabled" == "true" ]]; then
    print_info "DNS Proxy is already enabled. No changes needed."
    return 0
  fi
  
  print_step "Creating DNS proxy patch file..."
  
  # Create a temporary patch file
  cat > /tmp/cilium-dns-proxy-patch.yaml << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: cilium-config
  namespace: kube-system
data:
  # Enable DNS proxy to allow DNS-based policies
  enable-dns-proxy: "true"
  # Port range for transparent DNS proxy
  proxy-dns-port-range: "53:53"
EOF
  
  print_step "Applying patch to Cilium ConfigMap..."
  
  # Apply the patch
  if ! kubectl patch configmap -n kube-system cilium-config --patch-file /tmp/cilium-dns-proxy-patch.yaml; then
    print_error "Failed to patch Cilium ConfigMap"
    return 1
  fi
  
  print_success "Cilium ConfigMap successfully patched"
  
  print_step "Restarting Cilium pods to apply new configuration..."
  
  # Delete Cilium pods to apply new configuration
  kubectl delete pods -n kube-system -l k8s-app=cilium --wait=false
  
  print_info "Waiting for Cilium pods to restart..."
  sleep 10
  
  # Wait for pods to be ready
  kubectl wait --for=condition=ready pod -l k8s-app=cilium -n kube-system --timeout=120s
  
  print_success "Cilium pods restarted with new configuration"
  
  print_step "Verifying updated configuration..."
  
  # Check if the setting was applied
  dns_proxy_enabled=$(kubectl get configmap -n kube-system cilium-config -o jsonpath='{.data.enable-dns-proxy}' 2>/dev/null)
  
  if [[ "$dns_proxy_enabled" == "true" ]]; then
    print_success "DNS Proxy is now enabled"
    echo -e "\n${GREEN}Configuration successfully updated.${NC}"
    echo "Your Cilium configuration should now be able to validate DNS policies."
    echo "Please try running the DNS policy tests again."
  else
    print_error "Failed to enable DNS Proxy"
    echo "Manual intervention may be required."
  fi
  
  return 0
}

# Clean up test environment with extra-aggressive resource deletion
function cleanup_test_env() {
  local ns_name="$NAMESPACE"
  local max_attempts=3
  local attempt=1
  
  print_subheader "Cleaning up test environment"
  
  # Step 1: Force delete all Cilium policies first, before anything else
  echo "Deleting all Cilium policies (explicit deletion)..."
  for policy in $(kubectl get ciliumnetworkpolicies -n $ns_name --no-headers -o custom-columns=":metadata.name" 2>/dev/null); do
    echo "Explicitly deleting policy: $policy"
    kubectl delete ciliumnetworkpolicies -n $ns_name $policy --grace-period=0 --force 2>/dev/null || true
    sleep 1
  done
  
  # Also check for cluster-wide policies
  for policy in $(kubectl get ciliumclusterwidenetworkpolicies --no-headers -o custom-columns=":metadata.name" 2>/dev/null); do
    echo "Explicitly deleting cluster-wide policy: $policy"
    kubectl delete ciliumclusterwidenetworkpolicies $policy --grace-period=0 --force 2>/dev/null || true
    sleep 1
  done
  
  # Check if namespace exists
  if ! kubectl get namespace $ns_name &>/dev/null; then
    print_info "Namespace $ns_name does not exist, skipping cleanup"
    
    # Still clean up any .applied files
    echo "Cleaning up .applied files..."
    find "$POLICY_DIR" -name "*.applied" -print -delete | while read file; do
      echo "Removed: $file"
    done
    
    print_success "Cleanup complete - no namespace to delete"
    return 0
  fi
  
  print_info "Performing ultra-thorough cleanup of test environment..."
  
  # Delete the pods first with forced removal
  echo "Deleting pods in namespace: $ns_name"
  kubectl delete pods --all -n $ns_name --grace-period=0 --force --wait=false 2>/dev/null || true
  
  # Wait briefly to let pod deletion start
  sleep 3
  
  # Force terminate any stuck pods by removing finalizers
  for pod in $(kubectl get pods -n $ns_name --no-headers -o custom-columns=":metadata.name" 2>/dev/null); do
    echo "Removing finalizers from pod: $pod"
    kubectl patch pod -n $ns_name $pod -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
  done
  
  # Delete all Cilium policies from Kubernetes using multiple approaches
  echo "Deleting all Cilium policies (bulk deletion)..."
  kubectl delete ciliumnetworkpolicies --all -n $ns_name --grace-period=0 --force --wait=false 2>/dev/null || true
  kubectl delete ciliumclusterwidenetworkpolicies --all --grace-period=0 --force --wait=false 2>/dev/null || true
  
  # Double-check for any remaining policies and explicitly delete them
  remaining_policies=$(kubectl get ciliumnetworkpolicies -n $ns_name -o name 2>/dev/null)
  if [ -n "$remaining_policies" ]; then
    echo "Found remaining policies, forcing deletion..."
    echo "$remaining_policies" | xargs -r kubectl delete --grace-period=0 --force 2>/dev/null || true
  fi
  
  # Remove any finalizers from the namespace if present (to avoid stuck namespaces)
  echo "Removing finalizers from namespace if present..."
  kubectl patch namespace $ns_name -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
  
  # Delete the namespace with increasing force
  while [ $attempt -le $max_attempts ]; do
    echo "Attempt $attempt: Deleting namespace: $ns_name"
    
    if [ $attempt -eq 1 ]; then
      # First try: Normal delete with short timeout
      kubectl delete namespace $ns_name --wait=false
    elif [ $attempt -eq 2 ]; then
      # Second try: Force delete with zero grace period
      kubectl delete namespace $ns_name --force --grace-period=0
    else
      # Final try: Force delete with patch to remove finalizers
      kubectl patch namespace $ns_name -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
      kubectl delete namespace $ns_name --force --grace-period=0
    fi
    
    # Check if namespace is gone
    if ! kubectl get namespace $ns_name &>/dev/null; then
      echo "Namespace successfully deleted!"
      break
    fi
    
    echo "Namespace still exists, waiting before next attempt..."
    sleep 5
    ((attempt++))
  done
  
  # Verify namespace is truly gone with extra wait
  wait_count=0
  while kubectl get namespace $ns_name &>/dev/null && [ $wait_count -lt 6 ]; do
    echo "Waiting for namespace to be fully deleted... ($((wait_count+1))/6)"
    sleep 5
    ((wait_count++))
  done
  
  # Final status check
  if kubectl get namespace $ns_name &>/dev/null; then
    print_error "WARNING: Could not delete namespace $ns_name after multiple attempts"
    print_error "Will attempt to recreate namespace for clean state"
    
    # Last resort - try to delete with kubectl directly with debug output
    kubectl delete namespace $ns_name --grace-period=0 --force --v=6 2>/dev/null || true
    sleep 5
  else
    echo "Namespace $ns_name has been successfully deleted"
  fi
  
  # Wait a moment for resources to be fully cleaned up
  echo "Waiting for resources to be fully cleaned up..."
  sleep 5
  
  # Find and remove all .applied files
  echo "Cleaning up .applied files..."
  find "$POLICY_DIR" -name "*.applied" -print -delete | while read file; do
    echo "Removed: $file"
  done
  
  print_success "Cleanup complete - Original YAML files preserved, .applied files removed"
}

# Create test namespace and environment setup
function create_test_env() {
  local ns_name="$NAMESPACE"
  
  print_subheader "Setting up test environment"
  
  # Create namespace if it doesn't exist
  if ! kubectl get ns $ns_name &>/dev/null; then
    kubectl create namespace $ns_name
    echo "Created namespace: $ns_name"
  else
    echo "Using existing namespace: $ns_name"
  fi
  
  # Find worker nodes - we need at least 2 worker nodes for node-based tests
  WORKER_NODES=($(kubectl get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || \
                kubectl get nodes --selector='!node-role.kubernetes.io/master,!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[*].metadata.name}'))
  
  # If no worker nodes found using selectors, just get all nodes
  if [ ${#WORKER_NODES[@]} -eq 0 ]; then
    WORKER_NODES=($(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'))
    print_info "Using all available nodes: ${WORKER_NODES[*]}"
  fi
  
  if [ ${#WORKER_NODES[@]} -lt 2 ]; then
    print_info "Warning: Less than 2 worker nodes found. Cross-node tests may not work correctly."
  else
    print_success "Found ${#WORKER_NODES[@]} worker nodes: ${WORKER_NODES[*]}"
  fi
  
  NODE1=${WORKER_NODES[0]}
  NODE2=${WORKER_NODES[1]:-${WORKER_NODES[0]}}
  
  print_step "Creating HTTP server pod on $NODE1..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: http-server
  namespace: $ns_name
  labels:
    app: service
    api-server: "true"
    service-type: myService
spec:
  nodeName: ${NODE1}
  containers:
  - name: http
    image: nginx:alpine
    ports:
    - containerPort: 80
    - containerPort: 8080
EOF
    
  # Create a service to expose the HTTP server
  print_step "Creating service for HTTP server..."
  kubectl -n $ns_name expose pod http-server --port=80 --target-port=80 --name=http-service
    
  # Create DNS server pod on first node
  print_step "Creating DNS server pod on $NODE1..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: dns-server
  namespace: $ns_name
  labels:
    app: dns
    role: server
spec:
  nodeName: ${NODE1}
  containers:
  - name: dns
    image: coredns/coredns:latest
    ports:
    - containerPort: 53
      protocol: UDP
    - containerPort: 53
      protocol: TCP
EOF
    
  # Create client pods on different nodes with different roles
  print_step "Creating client pods on different nodes..."
  # Client with 'env: prod' label
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: client-prod
  namespace: $ns_name
  labels:
    app: client
    env: prod
    role: frontend
spec:
  nodeName: ${NODE1}
  containers:
  - name: client
    image: curlimages/curl:latest
    command: ["sleep", "3600"]
EOF

  # Client with 'role: untrusted' label
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: client-untrusted
  namespace: $ns_name
  labels:
    app: client
    role: untrusted
spec:
  nodeName: ${NODE2}
  containers:
  - name: client
    image: curlimages/curl:latest
    command: ["sleep", "3600"]
EOF

  # Client on second node
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: client-dns
  namespace: $ns_name
  labels:
    app: client
    role: internal
spec:
  nodeName: ${NODE2}
  containers:
  - name: client
    image: nicolaka/netshoot:latest
    command: ["sleep", "3600"]
EOF

  # Wait for pods to be ready with multiple retries if needed
  echo "Waiting for pods to be ready (timeout: 60s)..."
  local max_retries=3
  local retry=0
  local all_ready=false
  
  while [ $retry -lt $max_retries ] && [ "$all_ready" != "true" ]; do
    ((retry++))
    echo "Attempt $retry of $max_retries to check pod readiness..."
    
    # Try to wait for all pods
    if kubectl wait --for=condition=ready pod/http-server pod/dns-server pod/client-prod pod/client-untrusted pod/client-dns -n $ns_name --timeout=30s &>/dev/null; then
      all_ready=true
      break
    fi
    
    # If we get here, at least one pod is not ready
    echo "Not all pods are ready yet. Checking individual pods..."
    
    # Check each pod individually to see which ones are ready
    for pod in http-server dns-server client-prod client-untrusted client-dns; do
      if kubectl get pod $pod -n $ns_name | grep -q "Running"; then
        echo "✓ Pod $pod is running"
      else
        echo "✗ Pod $pod is not running. Status: $(kubectl get pod $pod -n $ns_name -o jsonpath='{.status.phase}')"
      fi
    done
    
    if [ $retry -lt $max_retries ]; then
      echo "Waiting 10 seconds before retrying..."
      sleep 10
    fi
  done
  
  if [ "$all_ready" != "true" ]; then
    print_error "Not all pods are ready after $max_retries attempts"
    echo "Continuing anyway, but some tests may fail"
  else
    print_success "All pods are ready"
  fi
  
  # Get pod IPs
  HTTP_POD_IP=$(kubectl get pod http-server -n $ns_name -o jsonpath='{.status.podIP}')
  DNS_POD_IP=$(kubectl get pod dns-server -n $ns_name -o jsonpath='{.status.podIP}')
  CLIENT_PROD_IP=$(kubectl get pod client-prod -n $ns_name -o jsonpath='{.status.podIP}')
  CLIENT_UNTRUSTED_IP=$(kubectl get pod client-untrusted -n $ns_name -o jsonpath='{.status.podIP}')
  CLIENT_DNS_IP=$(kubectl get pod client-dns -n $ns_name -o jsonpath='{.status.podIP}')
    
  print_step "HTTP Server Pod IP: $HTTP_POD_IP (on $NODE1)"
  print_step "DNS Server Pod IP: $DNS_POD_IP (on $NODE1)"
  print_step "Client-Prod Pod IP: $CLIENT_PROD_IP (on $NODE1)"
  print_step "Client-Untrusted Pod IP: $CLIENT_UNTRUSTED_IP (on $NODE2)"
  print_step "Client-DNS Pod IP: $CLIENT_DNS_IP (on $NODE2)"
    
  # Create test files for HTTP server
  print_step "Creating test files on HTTP server..."
  kubectl exec -n $ns_name http-server -- sh -c "echo 'This is public content' > /usr/share/nginx/html/public"
  kubectl exec -n $ns_name http-server -- sh -c "mkdir -p /usr/share/nginx/html/api/public"
  kubectl exec -n $ns_name http-server -- sh -c "echo 'Public API' > /usr/share/nginx/html/api/public/index.html"
  kubectl exec -n $ns_name http-server -- sh -c "mkdir -p /usr/share/nginx/html/api/users/123"
  kubectl exec -n $ns_name http-server -- sh -c "echo 'User 123 Data' > /usr/share/nginx/html/api/users/123/index.html"
  kubectl exec -n $ns_name http-server -- sh -c "mkdir -p /usr/share/nginx/html/path1"
  kubectl exec -n $ns_name http-server -- sh -c "echo 'Path1 Content' > /usr/share/nginx/html/path1/index.html"
  kubectl exec -n $ns_name http-server -- sh -c "mkdir -p /usr/share/nginx/html/path2"
  kubectl exec -n $ns_name http-server -- sh -c "echo 'Path2 Content' > /usr/share/nginx/html/path2/index.html"
    
  print_success "Test environment ready"
  return 0
}

# Test basic connectivity (should work with no policies)
function test_basic_connectivity() {
  local ns_name="$NAMESPACE"
  local basic_connectivity_ok=true
  
  print_subheader "Testing basic connectivity (no policies)"
  
  # Test HTTP connectivity from client-prod
  print_step "Testing HTTP connectivity from client-prod..."
  if kubectl exec -n $ns_name client-prod -- curl -s --max-time 5 http://$HTTP_POD_IP/ | grep -q "Welcome to nginx"; then
    print_success "HTTP from client-prod to HTTP server successful"
    kubectl exec -n $ns_name client-prod -- curl -s --max-time 5 http://$HTTP_POD_IP/ | head -n 3
  else
    print_error "HTTP from client-prod to HTTP server failed"
    print_error "Expected: HTTP 200 response with nginx welcome page"
    basic_connectivity_ok=false
  fi
  
  # Test HTTP connectivity from client-untrusted
  print_step "Testing HTTP connectivity from client-untrusted..."
  if kubectl exec -n $ns_name client-untrusted -- curl -s --max-time 5 http://$HTTP_POD_IP/ | grep -q "Welcome to nginx"; then
    print_success "HTTP from client-untrusted to HTTP server successful"
    kubectl exec -n $ns_name client-untrusted -- curl -s --max-time 5 http://$HTTP_POD_IP/ | head -n 3
  else
    print_error "HTTP from client-untrusted to HTTP server failed"
    print_error "Expected: HTTP 200 response with nginx welcome page"
    basic_connectivity_ok=false
  fi
  
  # Test DNS lookup from client-dns
  print_step "Testing DNS lookup from client-dns..."
  if kubectl exec -n $ns_name client-dns -- dig kubernetes.default.svc.cluster.local @kube-dns.kube-system +short &>/dev/null; then
    print_success "DNS resolution from client-dns successful"
    kubectl exec -n $ns_name client-dns -- dig kubernetes.default.svc.cluster.local @kube-dns.kube-system +short | head -n 2
  else
    print_error "DNS resolution from client-dns failed"
    basic_connectivity_ok=false
  fi
  
  if [ "$basic_connectivity_ok" = true ]; then
    print_success "Basic connectivity test PASSED"
    BASIC_CONNECTIVITY_RESULT="${GREEN}PASSED${NC}"
    return 0
  else
    print_error "Basic connectivity test FAILED"
    print_error "Please check your network setup or cluster configuration"
    print_error "Consider running 'cleanup' first to reset the environment"
    print_error "Command: ./test-l7-policies.sh cleanup"
    BASIC_CONNECTIVITY_RESULT="${RED}FAILED${NC}"
    
    # Debug information
    echo
    echo "Debug information:"
    echo "Checking pod status and placement..."
    kubectl get pods -n $ns_name -o wide
    
    echo
    echo "Checking for existing Cilium policies..."
    kubectl get ciliumnetworkpolicies --all-namespaces 2>/dev/null || echo "No Cilium network policies found"
    kubectl get ciliumclusterwidenetworkpolicies 2>/dev/null || echo "No Cilium cluster-wide network policies found"
    
    return 1
  fi
}

# Function to apply policy YAML files with variable substitution
function apply_policy_yaml() {
  local yaml_file=$1
  local ns_name=$2
  
  # Create a working copy in a temporary directory - leave original untouched
  local tmp_dir=$(mktemp -d)
  local tmp_yaml=$(basename "$yaml_file")
  local working_copy="$tmp_dir/$tmp_yaml"
  
  # Copy the original template to our working directory
  cp "$yaml_file" "$working_copy"
  
  print_info "Applying policy from: $yaml_file (using working copy)"
  
  # Apply variable substitution to the working copy
  # Using | as delimiter instead of / to avoid conflicts with CIDR notation
  sed -i.bak "s|{{NS_NAME}}|$ns_name|g" "$working_copy"
  sed -i.bak "s|{{NODE1}}|$NODE1|g" "$working_copy"
  sed -i.bak "s|{{NODE2}}|$NODE2|g" "$working_copy"
  
  # Add namespace to the policy if it doesn't exist
  if ! grep -q "namespace:" "$working_copy"; then
    # Add namespace to metadata (using a different approach)
    awk '/metadata:/ {print; print "  namespace: '$ns_name'"; next} 1' "$working_copy" > "${working_copy}.tmp" && mv "${working_copy}.tmp" "$working_copy"
  fi
  
  # Apply the policy
  kubectl apply -f "$working_copy"
  
  # Get the policy name
  local policy_name=$(grep -m1 "name:" "$working_copy" | awk '{print $2}' | tr -d '"' | tr -d "'")
  local policy_kind=$(grep -m1 "kind:" "$working_copy" | awk '{print $2}' | tr -d '"' | tr -d "'")
  
  sleep 10 # Give Cilium time to process the policy
  
  # Check if policy is valid based on kind
  if [[ "$policy_kind" == "CiliumClusterwideNetworkPolicy" ]]; then
    kubectl get ccnp $policy_name -o wide 2>/dev/null || echo "Policy $policy_name applied but not immediately available"
  else
    kubectl get cnp -n $ns_name $policy_name -o wide 2>/dev/null || echo "Policy $policy_name applied but not immediately available"
  fi
  
  # Save the applied version for reference, but don't modify the original
  cp "$working_copy" "${yaml_file}.applied"
  
  # Return the temporary directory path for cleanup
  echo "$tmp_dir"
}

# Clean up temporary files created during policy application
function cleanup_temp_files() {
  local temp_dir=$1
  
  if [ -d "$temp_dir" ]; then
    echo "Cleaning up temporary files in $temp_dir"
    rm -rf "$temp_dir"
  fi
}

# Function to display usage information
function show_usage() {
  print_header "CILIUM L7 NETWORK POLICIES TEST SCRIPT - HELP"
  echo "Usage: $0 [subtest-name]"
  echo
  echo "Available subtests organized by Cilium Documentation categories:"
  echo
  echo "  1. HTTP POLICIES:"
  echo "     http-basic     - Basic HTTP GET policy with path matching"
  echo "     http-headers   - HTTP policy with header validation"
  echo "     http-advanced  - Advanced HTTP with multiple methods and paths"
  echo "     http           - Test all HTTP policies"
  echo
  echo "  2. DNS POLICIES:"
  echo "     dns-matchname    - DNS matchName policy (exact matching)"
  echo "     dns-matchpattern - DNS matchPattern policy (wildcard matching)"
  echo "     dns-fqdn         - DNS FQDN policy with IP discovery"
  echo "     dns              - Test all DNS policies"
  echo
  echo "  3. DENY POLICIES:"
  echo "     deny-ingress    - Deny ingress policy"
  echo "     deny-clusterwide - Clusterwide deny policy"
  echo "     deny            - Test all deny policies"
  echo
  echo "  OTHER OPTIONS:"
  echo "     baseline        - Test baseline L7 policy enforcement"
  echo "     categories      - Test all categories with cleanup between each (default)"
  echo "     isolated-all    - Test all subtests with cleanup between each test"
  echo "     cleanup         - Only clean up the test environment"
  echo "     check-dns-config - Check DNS proxy configuration in Cilium"
  echo "     fix-dns-config   - Fix DNS proxy configuration in Cilium"
  echo "     list           - List all available subtests"
  echo "     help           - Show this usage information"
  echo
  echo "Example:"
  echo "  $0 http"
  echo "  $0 dns-matchname"
  echo "  $0 categories"
  echo "  $0 isolated-all"
  return 0
}

# Function to test the DNS matchName policy
function test_dns_matchname() {
  local ns_name="$NAMESPACE"
  local policy_file="$DNS_DIR/dns-matchname-policy.yaml"
  
  print_subheader "Testing DNS matchName Policy"
  
  # Apply the policy
  echo
  local tmp_dir=$(apply_policy_yaml "$policy_file" "$ns_name")
  echo "Applied policy: $(basename $policy_file)"
  echo
  echo "Policy content:"
  grep -A15 "spec:" "$policy_file" | head -n 15
  
  # Check if the policy is valid
  print_step "DNS matchName policy status: "
  local policy_valid=$(kubectl get cnp -n $ns_name dns-matchname-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].status}' 2>/dev/null)
  local policy_message=$(kubectl get cnp -n $ns_name dns-matchname-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].message}' 2>/dev/null)
  
  echo "Policy valid status: $policy_valid"
  echo "Policy message: $policy_message"
  
  if [[ "$policy_valid" == "True" ]]; then
    print_success "DNS matchName policy successfully applied and validated"
    DNS_MATCHNAME_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "DNS matchName policy not valid: $policy_message"
    kubectl describe cnp -n $ns_name dns-matchname-policy
    kubectl delete -f "${policy_file}.applied" 2>/dev/null || true
    DNS_MATCHNAME_RESULT="${RED}FAIL${NC}"
  fi
  
  # Clean up temp files
  cleanup_temp_files "$tmp_dir"
  return 0
}

# Function to test the DNS matchPattern policy
function test_dns_matchpattern() {
  local ns_name="$NAMESPACE"
  local policy_file="$DNS_DIR/dns-matchpattern-policy.yaml"
  
  print_subheader "Testing DNS matchPattern Policy"
  
  # Apply the policy
  echo
  local tmp_dir=$(apply_policy_yaml "$policy_file" "$ns_name")
  echo "Applied policy: $(basename $policy_file)"
  echo
  echo "Policy content:"
  grep -A20 "spec:" "$policy_file" | head -n 20
  
  # Check if the policy is valid
  print_step "DNS matchPattern policy status: "
  local policy_valid=$(kubectl get cnp -n $ns_name dns-matchpattern-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].status}' 2>/dev/null)
  local policy_message=$(kubectl get cnp -n $ns_name dns-matchpattern-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].message}' 2>/dev/null)
  
  echo "Policy valid status: $policy_valid"
  echo "Policy message: $policy_message"
  
  if [[ "$policy_valid" == "True" ]]; then
    print_success "DNS matchPattern policy successfully applied and validated"
    DNS_MATCHPATTERN_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "DNS matchPattern policy not valid: $policy_message"
    kubectl describe cnp -n $ns_name dns-matchpattern-policy
    kubectl delete -f "${policy_file}.applied" 2>/dev/null || true
    DNS_MATCHPATTERN_RESULT="${RED}FAIL${NC}"
  fi
  
  # Clean up temp files
  cleanup_temp_files "$tmp_dir"
  return 0
}

# Function to test the DNS FQDN policy with IP discovery
function test_dns_fqdn() {
  local ns_name="$NAMESPACE"
  local policy_file="$DNS_DIR/dns-fqdn-policy.yaml"
  
  print_subheader "Testing DNS FQDN Policy with IP Discovery"
  
  # Apply the policy
  echo
  local tmp_dir=$(apply_policy_yaml "$policy_file" "$ns_name")
  echo "Applied policy: $(basename $policy_file)"
  echo
  echo "Policy content:"
  grep -A30 "spec:" "$policy_file" | head -n 30
  
  # Check if the policy is valid
  print_step "DNS FQDN policy status: "
  local policy_valid=$(kubectl get cnp -n $ns_name dns-fqdn-visibility-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].status}' 2>/dev/null)
  local policy_message=$(kubectl get cnp -n $ns_name dns-fqdn-visibility-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].message}' 2>/dev/null)
  
  echo "Policy valid status: $policy_valid"
  echo "Policy message: $policy_message"
  
  if [[ "$policy_valid" == "True" ]]; then
    print_success "DNS FQDN policy successfully applied and validated"
    DNS_FQDN_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "DNS FQDN policy not valid: $policy_message"
    kubectl describe cnp -n $ns_name dns-fqdn-visibility-policy
    kubectl delete -f "${policy_file}.applied" 2>/dev/null || true
    DNS_FQDN_RESULT="${RED}FAIL${NC}"
  fi
  
  # Clean up temp files
  cleanup_temp_files "$tmp_dir"
  return 0
}

# Test all DNS policy types
function test_dns_policies() {
  print_header "DNS POLICIES AND IP DISCOVERY (CILIUM L7 CATEGORY 2)"
  echo "This policy type controls DNS traffic and can populate IP allow-lists"
  
  echo "[TEST 1/3] Testing DNS matchName Policy (exact matching)"
  test_dns_matchname
  
  echo "[TEST 2/3] Testing DNS matchPattern Policy (wildcard matching)"
  test_dns_matchpattern
  
  echo "[TEST 3/3] Testing DNS FQDN Policy with IP Discovery"
  test_dns_fqdn
  
  # Set overall DNS category result
  if [[ "$DNS_MATCHNAME_RESULT" == "${GREEN}PASSED${NC}" && 
        "$DNS_MATCHPATTERN_RESULT" == "${GREEN}PASSED${NC}" && 
        "$DNS_FQDN_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    DNS_RESULT="${GREEN}PASSED${NC}"
  else
    DNS_RESULT="${RED}FAIL${NC} Some DNS policy tests failed"
  fi
  
  print_success "DNS policies test completed"
}

# Function to test the deny ingress policy
function test_deny_ingress() {
  local ns_name="$NAMESPACE"
  local policy_file="$DENY_DIR/deny-ingress-policy.yaml"
  
  print_subheader "Testing Deny Ingress Policy"
  
  # Apply the policy
  echo
  local tmp_dir=$(apply_policy_yaml "$policy_file" "$ns_name")
  echo "Applied policy: $(basename $policy_file)"
  echo
  echo "Policy content:"
  grep -A15 "spec:" "$policy_file" | head -n 15
  
  # Check if the policy is valid
  print_step "Deny Ingress policy status: "
  local policy_valid=$(kubectl get cnp -n $ns_name deny-ingress-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].status}' 2>/dev/null)
  local policy_message=$(kubectl get cnp -n $ns_name deny-ingress-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].message}' 2>/dev/null)
  
  echo "Policy valid status: $policy_valid"
  echo "Policy message: $policy_message"
  
  if [[ "$policy_valid" == "True" ]]; then
    print_success "Deny Ingress policy successfully applied and validated"
    DENY_INGRESS_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Deny Ingress policy not valid: $policy_message"
    kubectl describe cnp -n $ns_name deny-ingress-policy
    kubectl delete -f "${policy_file}.applied" 2>/dev/null || true
    DENY_INGRESS_RESULT="${RED}FAIL${NC}"
  fi
  
  # Clean up temp files
  cleanup_temp_files "$tmp_dir"
  return 0
}

# Function to test the clusterwide deny policy
function test_deny_clusterwide() {
  local policy_file="$DENY_DIR/deny-with-allow-policy.yaml"
  
  print_subheader "Testing Clusterwide Deny Policy"
  
  # Apply the policy (use empty namespace for clusterwide policies)
  echo
  local tmp_dir=$(apply_policy_yaml "$policy_file" "")
  echo "Applied policy: $(basename $policy_file)"
  echo
  echo "Policy content:"
  grep -A15 "spec:" "$policy_file" | head -n 15
  
  # Check if the policy is valid - special handling for clusterwide policy
  print_step "Clusterwide Deny policy status: "
  local policy_valid=$(kubectl get ccnp deny-with-allow-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].status}' 2>/dev/null)
  local policy_message=$(kubectl get ccnp deny-with-allow-policy -o jsonpath='{.status.conditions[?(@.type=="Valid")].message}' 2>/dev/null)
  
  echo "Policy valid status: $policy_valid"
  echo "Policy message: $policy_message"
  
  if [[ "$policy_valid" == "True" ]]; then
    print_success "Clusterwide Deny policy successfully applied and validated"
    DENY_CLUSTERWIDE_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Clusterwide Deny policy not valid: $policy_message"
    kubectl describe ccnp deny-with-allow-policy
    kubectl delete -f "${policy_file}.applied" 2>/dev/null || true
    DENY_CLUSTERWIDE_RESULT="${RED}FAIL${NC}"
  fi
  
  # Clean up temp files
  cleanup_temp_files "$tmp_dir"
  return 0
}

# Test all deny policy types
function test_deny_policies() {
  print_header "DENY POLICIES (CILIUM L7 CATEGORY 3)"
  echo "This policy type demonstrates deny policies that take precedence over allow policies"
  
  echo "[TEST 1/2] Testing Deny Ingress Policy"
  test_deny_ingress
  
  echo "[TEST 2/2] Testing Clusterwide Deny Policy"
  test_deny_clusterwide
  
  # Set overall DENY category result
  if [[ "$DENY_INGRESS_RESULT" == "${GREEN}PASSED${NC}" && 
        "$DENY_CLUSTERWIDE_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    DENY_RESULT="${GREEN}PASSED${NC}"
  else
    DENY_RESULT="${RED}FAIL${NC} Some deny policy tests failed"
  fi
  
  print_success "Deny policies test completed"
}

# Print results summary
function print_results_summary() {
  local end_time=$(date +%s)
  local runtime=$((end_time - START_TIME))
  local mins=$((runtime / 60))
  local secs=$((runtime % 60))
  
  print_header "DETAILED TEST RESULTS SUMMARY"
  
  echo "Tests completed in ${mins}m ${secs}s"
  echo "Categories tested: $@"
  echo -e "\n"
  
  # Print category results
  echo "----- TEST RESULTS BY CATEGORY -----"
  echo
  printf "%-20s %-12s %-s\n" "CATEGORY" "RESULT" "DETAILS"
  echo "----------------------------------------------------------------"
  
  # Print results for each category
  if [[ "$HTTP_RESULT" != "NOT_RUN" ]]; then
    printf "%-20s %-12s %-s\n" "HTTP POLICIES" "$HTTP_RESULT" ""
  else
    printf "%-20s %-12s %-s\n" "HTTP POLICIES" "NOT_RUN" ""
  fi
  
  if [[ "$DNS_RESULT" != "NOT_RUN" ]]; then
    printf "%-20s %-12s %-s\n" "DNS POLICIES" "$DNS_RESULT" ""
  else
    printf "%-20s %-12s %-s\n" "DNS POLICIES" "NOT_RUN" ""
  fi
  
  if [[ "$DENY_RESULT" != "NOT_RUN" ]]; then
    printf "%-20s %-12s %-s\n" "DENY POLICIES" "$DENY_RESULT" ""
  else
    printf "%-20s %-12s %-s\n" "DENY POLICIES" "NOT_RUN" ""
  fi
  
  # If DNS tests were run, show detailed DNS test results
  if [[ "$DNS_RESULT" != "NOT_RUN" ]]; then
    echo -e "\n"
    echo "----- DNS POLICY SUBTESTS RESULTS -----"
    echo
    printf "%-20s %-12s\n" "DNS POLICY TEST" "RESULT"
    echo "----------------------------------------------------------------"
    printf "%-20s %-12s\n" "DNS matchName" "$DNS_MATCHNAME_RESULT"
    printf "%-20s %-12s\n" "DNS matchPattern" "$DNS_MATCHPATTERN_RESULT"
    printf "%-20s %-12s\n" "DNS FQDN" "$DNS_FQDN_RESULT"
  fi
  
  # Count passed, failed, partial, and not run categories
  local categories_passed=0
  local categories_failed=0
  local categories_partial=0
  local categories_not_run=0
  
  # Count tests passed, failed, partial, and not run
  local tests_passed=0
  local tests_failed=0
  local tests_partial=0
  local tests_not_run=0
  
  # Count passed/failed categories
  if [[ "$HTTP_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((categories_passed++))
  elif [[ "$HTTP_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((categories_failed++))
  elif [[ "$HTTP_RESULT" == "NOT_RUN" ]]; then
    ((categories_not_run++))
  else
    ((categories_partial++))
  fi
  
  if [[ "$DNS_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((categories_passed++))
  elif [[ "$DNS_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((categories_failed++))
  elif [[ "$DNS_RESULT" == "NOT_RUN" ]]; then
    ((categories_not_run++))
  else
    ((categories_partial++))
  fi
  
  if [[ "$DENY_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((categories_passed++))
  elif [[ "$DENY_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((categories_failed++))
  elif [[ "$DENY_RESULT" == "NOT_RUN" ]]; then
    ((categories_not_run++))
  else
    ((categories_partial++))
  fi
  
  # Count passed/failed individual tests for DNS
  if [[ "$DNS_MATCHNAME_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((tests_passed++))
  elif [[ "$DNS_MATCHNAME_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((tests_failed++))
  elif [[ "$DNS_MATCHNAME_RESULT" == "NOT_RUN" ]]; then
    ((tests_not_run++))
  else
    ((tests_partial++))
  fi
  
  if [[ "$DNS_MATCHPATTERN_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((tests_passed++))
  elif [[ "$DNS_MATCHPATTERN_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((tests_failed++))
  elif [[ "$DNS_MATCHPATTERN_RESULT" == "NOT_RUN" ]]; then
    ((tests_not_run++))
  else
    ((tests_partial++))
  fi
  
  if [[ "$DNS_FQDN_RESULT" == "${GREEN}PASSED${NC}" ]]; then
    ((tests_passed++))
  elif [[ "$DNS_FQDN_RESULT" == "${RED}FAIL${NC}"* ]]; then
    ((tests_failed++))
  elif [[ "$DNS_FQDN_RESULT" == "NOT_RUN" ]]; then
    ((tests_not_run++))
  else
    ((tests_partial++))
  fi
  
  # Add 2 for HTTP tests and 3 for DENY tests that are not being tracked individually
  if [[ "$HTTP_RESULT" == "NOT_RUN" ]]; then
    ((tests_not_run+=2))
  fi
  
  if [[ "$DENY_RESULT" == "NOT_RUN" ]]; then
    ((tests_not_run+=3))
  fi
  
  echo -e "\nSummary by Category:"
  echo "  Categories Passed: $categories_passed"
  echo "  Categories Failed: $categories_failed"
  echo "  Categories Partial: $categories_partial"
  echo "  Categories Not Run: $categories_not_run"
  
  echo -e "\nSummary by Individual Test:"
  echo "  Tests Passed: $tests_passed"
  echo "  Tests Failed: $tests_failed"
  echo "  Tests Partial: $tests_partial"
  echo "  Tests Not Run: $tests_not_run"
  
  if [ $tests_failed -gt 0 ]; then
    echo -e "\nSome tests failed. This may indicate:"
    echo "  - Network connectivity issues"
    echo "  - Cilium configuration differences"
    echo "  - Policy enforcement variations"
    echo "  - External connectivity restrictions (for DNS tests)"
  fi
}

# Main execution starts here
# Process command line arguments
function main() {
  # Default is to run all categories
  local run_mode="categories"
  local subtests=()
  
  # Parse arguments
  if [ $# -eq 0 ]; then
    run_mode="categories"
  else
    run_mode="$1"
  fi
  
  # Special cases for help, list, cleanup
  case "$run_mode" in
    "help")
      show_usage
      exit 0
      ;;
    "list")
      print_header "AVAILABLE L7 POLICY SUBTESTS"
      echo "Subtests are organized by Cilium Documentation categories:"
      echo
      echo "HTTP POLICY TESTS:"
      echo "  http-basic    - Test basic HTTP GET policy with path matching"
      echo "  http-headers  - Test HTTP policy with header validation"
      echo "  http-advanced - Test advanced HTTP with multiple methods and paths"
      echo "  http          - Run all HTTP policy tests"
      echo
      echo "DNS POLICY TESTS:"
      echo "  dns-matchname    - Test DNS matchName policy (exact matching)"
      echo "  dns-matchpattern - Test DNS matchPattern policy (wildcard matching)"
      echo "  dns-fqdn         - Test DNS FQDN policy with IP discovery"
      echo "  dns              - Run all DNS policy tests"
      echo
      echo "DENY POLICY TESTS:"
      echo "  deny-ingress    - Test deny ingress policy"
      echo "  deny-clusterwide - Test clusterwide deny policy"
      echo "  deny            - Run all deny policy tests"
      echo
      echo "OTHER OPTIONS:"
      echo "  cleanup         - Only clean up the test environment"
      echo "  categories      - Test all categories with cleanup between each"
      echo "  check-dns-config - Check DNS proxy configuration in Cilium"
      echo "  fix-dns-config   - Fix DNS proxy configuration in Cilium"
      exit 0
      ;;
    "cleanup")
      cleanup_test_env
      exit 0
      ;;
    "check-dns-config")
      check_dns_config
      exit 0
      ;;
    "fix-dns-config")
      fix_dns_config
      exit 0
      ;;
  esac
  
  print_header "CILIUM L7 NETWORK POLICIES TEST SCRIPT"
  
  # Handle specific test modes
  case "$run_mode" in
    "dns-matchname")
      print_step "Ensuring clean environment before testing $run_mode..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_dns_matchname
      subtests+=("DNS")
      ;;
    "dns-matchpattern")
      print_step "Ensuring clean environment before testing $run_mode..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_dns_matchpattern
      subtests+=("DNS")
      ;;
    "dns-fqdn")
      print_step "Ensuring clean environment before testing $run_mode..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_dns_fqdn
      subtests+=("DNS")
      ;;
    "dns")
      print_step "Ensuring clean environment before testing $run_mode category..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_dns_policies
      subtests+=("DNS")
      ;;
    "deny-ingress")
      print_step "Ensuring clean environment before testing $run_mode..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_deny_ingress
      subtests+=("DENY")
      ;;
    "deny-clusterwide")
      print_step "Ensuring clean environment before testing $run_mode..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_deny_clusterwide
      subtests+=("DENY")
      ;;
    "deny")
      print_step "Ensuring clean environment before testing $run_mode category..."
      cleanup_test_env
      create_test_env
      test_basic_connectivity
      test_deny_policies
      subtests+=("DENY")
      ;;
    "categories")
      # Function to test by category with cleanup between each category
      print_header "RUNNING TESTS BY CATEGORY WITH CLEANUP BETWEEN CATEGORIES"
      print_info "This will run tests by Cilium documentation categories with cleanup between each category"
      print_info "to prevent policy interference between categories."
      echo
      
      # Define all categories to run in sequence
      local categories=(
        "dns"
        "deny"
        # Add HTTP category when implemented
        # "http"
      )
      
      # Initialize results tracking
      local results=()
      local passed=0
      local failed=0
      local skipped=0
      local total=${#categories[@]}
      local current=1
      
      # Run each category with cleanup between them
      for category in "${categories[@]}"; do
        print_header "[$current/$total] RUNNING CATEGORY: $category"
        
        # First run cleanup to ensure we start with a clean slate
        print_info "Cleaning up previous test environment..."
        cleanup_test_env
        
        # Create fresh test environment
        print_info "Creating fresh test environment for category: $category"
        if ! create_test_env; then
          print_error "Failed to create test environment for $category, skipping category"
          results+=("$category: ${RED}ERROR - Environment creation failed${NC}")
          continue
        fi
        
        # Test basic connectivity
        if ! test_basic_connectivity; then
          print_error "Basic connectivity test failed for $category, skipping category"
          results+=("$category: ${RED}ERROR - Basic connectivity failed${NC}")
          cleanup_test_env
          continue
        fi
        
        # Run the specific category
        print_info "Running tests for category: $category"
        
        # Save original error handling and turn off error exit
        set +e
        
        # Execute the appropriate test function
        case $category in
          "dns")
            test_dns_policies
            result="${DNS_RESULT}"
            subtests+=("DNS")
            ;;
          "deny")
            test_deny_policies
            result="${DENY_RESULT}"
            subtests+=("DENY")
            ;;
          # Add HTTP when implemented
          # "http")
          #   test_http_policies
          #   result="${HTTP_RESULT}"
          #   subtests+=("HTTP")
          #   ;;
        esac
        
        # Track results
        if [[ "$result" == "${GREEN}PASSED${NC}" ]]; then
          results+=("Category $category: ${GREEN}PASSED${NC}")
          ((passed++))
        elif [[ "$result" == "${RED}FAIL${NC}"* ]]; then
          results+=("Category $category: ${RED}FAILED${NC}")
          ((failed++))
        elif [[ "$result" == "${YELLOW}SKIPPED${NC}" ]]; then
          results+=("Category $category: $result")
          ((skipped++))
        else
          results+=("Category $category: ${YELLOW}UNKNOWN${NC}")
          ((skipped++))
        fi
        
        # Increment test counter
        ((current++))
        
        echo ""
        echo "--------------------------------------------------------------"
        echo ""
      done
      
      # Display summary of all test results
      print_header "CATEGORY TESTS RESULTS SUMMARY"
      
      echo -e "Total categories executed: ${BLUE}$total${NC}"
      echo -e "Categories passed: ${GREEN}$passed${NC}"
      echo -e "Categories failed: ${RED}$failed${NC}"
      echo -e "Categories skipped: ${YELLOW}$skipped${NC}"
      echo ""
      
      echo "Individual category results:"
      for result in "${results[@]}"; do
        echo -e "$result"
      done
      
      print_success "All category tests completed"
      
      # The final results will be summarized by print_results_summary
      # outside this case statement with all the subtests
      ;;
  esac
  
  # Print results
  print_results_summary "${subtests[@]}"
  
  # Final cleanup
  print_step "Performing final cleanup..."
  cleanup_test_env
  
  print_header "L7 POLICY TESTS COMPLETED"
  echo "Run './test-l7-policies.sh list' to see other available subtests"
  exit 0
}

# Execute main function with all args
main "$@"
