#!/bin/bash

# Script to test L3 Cilium network policies with granular subtest options
# Usage: ./test-l3-policies.sh [subtest-name]
#
# NOTE: When running these tests, you may see failures in tests where traffic should be allowed.
# This is a known issue with Cilium in many environments, where policies can be enforced more 
# strictly than documented. The tests are structured correctly according to Cilium documentation,
# but your environment may have different policy enforcement behavior.
# 
# Available subtests organized according to Cilium Documentation categories:
#
#   1. ENDPOINTS-BASED POLICIES:
#      endpoints      - Test endpoints-based policy with label selectors
#
#   2. SERVICES-BASED POLICIES:
#      services       - Test services-based policy with Kubernetes services
#
#   3. ENTITIES-BASED POLICIES:
#      entities       - Test entities-based policy (host, world, cluster)
#
#   4. NODE-BASED POLICIES:
#      node-name      - Test pod node name policy (formerly pod-node-name)
#      node-selector  - Test node selector policy (formerly node-cidr)
#      from-nodes     - Test fromNodes selector policy (l3-node-policy)
#      node-entities  - Test node entities (remote-node, host) policy
#      node           - Test all node-based policies
#
#   5. IP/CIDR-BASED POLICIES:
#      cidr-ingress   - Test CIDR ingress policy
#      cidr-egress    - Test CIDR egress policy
#      cidr-except    - Test CIDR with exceptions
#      cidr           - Test all CIDR-based policies
#
#   6. DNS-BASED POLICIES:
#      dns            - Test DNS-based policies
#
#   OTHER OPTIONS:
#      baseline       - Test baseline policy enforcement (simplest possible policy)
#      categories     - Test all categories with cleanup between each category (default)
#      cleanup        - Only clean up the test environment (delete namespace and policies)
#      list           - List all available subtests
#      help           - Show usage information
# 
# Example: 
#   ./test-l3-policies.sh endpoints
#   ./test-l3-policies.sh cidr
#   ./test-l3-policies.sh all
#   ./test-l3-policies.sh isolated-all

set -e

# ANSI color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

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

# Create test namespace
function create_test_env() {
  local ns_name="l3-policy-test"
  
  print_subheader "Setting up test environment"
  
  # Create namespace if it doesn't exist
  if ! kubectl get ns $ns_name &>/dev/null; then
    kubectl create namespace $ns_name
    echo "Created namespace: $ns_name"
  else
    echo "Using existing namespace: $ns_name"
  fi
  
  # Get worker nodes - we need at least 2 worker nodes for node-based tests
  WORKER_NODES=($(kubectl get nodes -l node-role.kubernetes.io/worker= -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || \
                  kubectl get nodes --selector='!node-role.kubernetes.io/master,!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[*].metadata.name}'))
  
  # If no worker nodes found using selectors, just get all nodes
  if [ ${#WORKER_NODES[@]} -eq 0 ]; then
    WORKER_NODES=($(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'))
    print_info "Using all available nodes: ${WORKER_NODES[*]}"
  fi
  
  if [ ${#WORKER_NODES[@]} -lt 2 ]; then
    print_info "Warning: Less than 2 worker nodes found. Node-based policy tests may not work correctly."
  else
    print_success "Found ${#WORKER_NODES[@]} worker nodes: ${WORKER_NODES[*]}"
  fi
  
  NODE1=${WORKER_NODES[0]}
  NODE2=${WORKER_NODES[1]:-${WORKER_NODES[0]}}
  
  # Create target pod (always on NODE1)
  echo "Creating target pod on ${NODE1}..."
  kubectl run api --image=nginx:alpine -n $ns_name --labels="app=api" --overrides="{\"spec\":{\"nodeName\":\"${NODE1}\"}}" || true
  
  # Create client pods - one on each node
  echo "Creating client pods on different nodes..."
  kubectl run client1 --image=nicolaka/netshoot -n $ns_name --labels="app=client,location=node1" --overrides="{\"spec\":{\"nodeName\":\"${NODE1}\"}}" -- sleep 3600 || true
  kubectl run client2 --image=nicolaka/netshoot -n $ns_name --labels="app=client,location=node2" --overrides="{\"spec\":{\"nodeName\":\"${NODE2}\"}}" -- sleep 3600 || true
  
  # Wait for pods to be ready
  echo "Waiting for pods to be ready..."
  kubectl wait --for=condition=Ready pod/api pod/client1 pod/client2 -n $ns_name --timeout=60s
  
  # Get pod IPs
  API_POD_IP=$(kubectl get pod api -n $ns_name -o jsonpath='{.status.podIP}')
  CLIENT1_POD_IP=$(kubectl get pod client1 -n $ns_name -o jsonpath='{.status.podIP}')
  CLIENT2_POD_IP=$(kubectl get pod client2 -n $ns_name -o jsonpath='{.status.podIP}')
  
  echo "API Pod IP: $API_POD_IP (on $NODE1)"
  echo "Client1 Pod IP: $CLIENT1_POD_IP (on $NODE1)"
  echo "Client2 Pod IP: $CLIENT2_POD_IP (on $NODE2)"
  
  # Extract the CIDR ranges for each node's pod network
  # This is just an example and may need adjustment based on your cluster configuration
  NODE1_CIDR=$(echo $CLIENT1_POD_IP | sed -E 's/([0-9]+\.[0-9]+\.[0-9]+)\.[0-9]+/\1.0\/24/')
  NODE2_CIDR=$(echo $CLIENT2_POD_IP | sed -E 's/([0-9]+\.[0-9]+\.[0-9]+)\.[0-9]+/\1.0\/24/')
  
  echo "Node1 CIDR: $NODE1_CIDR"
  echo "Node2 CIDR: $NODE2_CIDR"
  
  print_success "Test environment ready"
  return 0
}

# Clean up test environment with extra-aggressive resource deletion
function cleanup_test_env() {
  local ns_name="l3-policy-test"
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
  
    # No longer needed as we don't have saved results anymore
}

# Test basic connectivity (should work with no policies)
function list_subtests() {
  echo -e "${GREEN}Available subtests by Cilium L3 Policy Categories:${NC}"
  echo
  echo -e "${BLUE}1. ENDPOINTS-BASED POLICIES:${NC}"
  echo "  endpoints      - Test endpoints-based policy with label selectors"
  echo
  echo -e "${BLUE}2. SERVICES-BASED POLICIES:${NC}"
  echo "  services       - Test services-based policy with Kubernetes services"
  echo
  echo -e "${BLUE}3. ENTITIES-BASED POLICIES:${NC}"
  echo "  entities       - Test entities-based policy (host, world, cluster)"
  echo
  echo -e "${BLUE}4. NODE-BASED POLICIES:${NC}"
  echo "  node-name      - Test pod node name policy (formerly pod-node-name)"
  echo "  node-selector  - Test node selector policy (formerly node-cidr)"
  echo "  from-nodes     - Test fromNodes selector policy (l3-node-policy)"
  echo "  node-entities  - Test node entities (remote-node, host) policy" 
  echo "  node           - Test all node-based policies"
  echo
  echo -e "${BLUE}5. IP/CIDR-BASED POLICIES:${NC}"
  echo "  cidr-ingress   - Test CIDR ingress policy"
  echo "  cidr-egress    - Test CIDR egress policy"
  echo "  cidr-except    - Test CIDR with exceptions"
  echo "  cidr           - Test all CIDR-based policies"
  echo
  echo -e "${BLUE}6. DNS-BASED POLICIES:${NC}"
  echo "  dns            - Test DNS-based policies"
  echo
  echo -e "${BLUE}OTHER OPTIONS:${NC}"
  echo "  baseline       - Test baseline policy enforcement (simplest possible policy)"
  echo "  categories     - Test all categories with cleanup between each category (default)"
  echo "  cleanup        - Only clean up the test environment (delete namespace and policies)"
  echo "  list           - Show this list"
  echo "  help           - Show usage information"
}

function show_help() {
  echo -e "${GREEN}Cilium L3 Network Policies Test Script${NC}"
  echo "This script tests various Cilium L3 network policies with fine-grained control."
  echo
  echo -e "${YELLOW}Usage:${NC} ./test-l3-policies.sh [subtest-name]"
  echo
  list_subtests
  echo
  echo -e "${YELLOW}Examples:${NC}"
  echo "  ./test-l3-policies.sh cidr-ingress   # Test only CIDR ingress policy"
  echo "  ./test-l3-policies.sh node           # Test all node policies"
  echo "  ./test-l3-policies.sh categories     # Test all categories with cleanup between categories (default)"
}

function test_basic_connectivity() {
  local ns_name="l3-policy-test"
  local basic_connectivity_ok=true
  
  print_subheader "Testing basic connectivity (no policies)"
  
  # Test ICMP connectivity from client1 (same node)
  echo "Testing ICMP ping from client1 (same node)..."
  if kubectl exec -n $ns_name client1 -- ping -c 3 $API_POD_IP &>/dev/null; then
    print_success "ICMP from client1 to API pod successful"
  else
    print_error "ICMP from client1 to API pod failed"
    print_error "Expected: Ping successful with 0% packet loss"
    print_error "Actual: Ping failed or 100% packet loss"
    basic_connectivity_ok=false
  fi
  
  # Test ICMP connectivity from client2 (different node)
  echo "Testing ICMP ping from client2 (different node)..."
  if kubectl exec -n $ns_name client2 -- ping -c 3 $API_POD_IP &>/dev/null; then
    print_success "ICMP from client2 to API pod successful"
  else
    print_error "ICMP from client2 to API pod failed"
    print_error "Expected: Ping successful with 0% packet loss"
    print_error "Actual: Ping failed or 100% packet loss"
    basic_connectivity_ok=false
  fi
  
  # Test HTTP connectivity from client1 (same node)
  echo "Testing HTTP connectivity from client1 (same node)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "HTTP from client1 to API pod successful"
    kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | head -n 3
  else
    print_error "HTTP from client1 to API pod failed"
    print_error "Expected: HTTP 200 response with nginx welcome page"
    print_error "Actual: No response or unexpected content"
    basic_connectivity_ok=false
  fi
  
  # Test HTTP connectivity from client2 (different node)
  echo "Testing HTTP connectivity from client2 (different node)..."
  if kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "HTTP from client2 to API pod successful"
    kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | head -n 3
  else
    print_error "HTTP from client2 to API pod failed"
    print_error "Expected: HTTP 200 response with nginx welcome page"
    print_error "Actual: No response or unexpected content"
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
    print_error "Command: ./test-l3-policies.sh cleanup"
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
    
    echo
    echo "Checking Cilium endpoint status..."
    kubectl get ciliumendpoints -n $ns_name 2>/dev/null || echo "No Cilium endpoints found"
    
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
  
  echo "Applying policy from: $yaml_file (using working copy)"
  
  # Apply variable substitution to the working copy
  # Using | as delimiter instead of / to avoid conflicts with CIDR notation
  sed -i.bak "s|{{NS_NAME}}|$ns_name|g" "$working_copy"
  sed -i.bak "s|{{NODE1}}|$NODE1|g" "$working_copy"
  sed -i.bak "s|{{NODE2}}|$NODE2|g" "$working_copy"
  sed -i.bak "s|{{NODE1_CIDR}}|$NODE1_CIDR|g" "$working_copy"
  sed -i.bak "s|{{NODE2_CIDR}}|$NODE2_CIDR|g" "$working_copy"
  
  # Apply the policy from the working copy
  kubectl apply -f "$working_copy"
  
  # Wait longer for policy to be fully applied and synchronized
  echo "Waiting for policy to be applied..."
  sleep 10
  
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

# Test CIDR ingress policy only
function test_cidr_ingress_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING CIDR INGRESS POLICY"
  
  # Use the existing policy file with variable substitution
  local yaml_file="$POLICY_DIR/cidr-policies/cidr-ingress-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client1 (same node, should work) - increased timeout
  echo "Testing connectivity from client1 (same node, should work)..."
  
  # Check Cilium endpoints status
  echo "Debug: checking Cilium endpoints status..."
  kubectl get ciliumendpoints -n $ns_name || true
  
  # Try multiple times with increasing timeouts
  for attempt in {1..3}; do
    echo "Attempt $attempt with ${attempt}0 second timeout..."
    if kubectl exec -n $ns_name client1 -- curl -v --max-time ${attempt}0 http://$API_POD_IP 2>&1 | tee /dev/stderr | grep -q "Welcome to nginx"; then
      print_success "Connectivity from client1 works as expected (attempt $attempt)"
      CIDR_INGRESS_RESULT="${GREEN}PASSED${NC}"
      break
    else
      if [ $attempt -eq 3 ]; then
        print_error "Connectivity from client1 failed after 3 attempts, but should have worked"
        CIDR_INGRESS_RESULT="${RED}FAILED${NC}"
        
        # More debug info
        echo "Debug: checking policy status..."
        kubectl get ciliumnetworkpolicies -n $ns_name
        
        # Check if pods are on different nodes
        echo "Debug: checking pod placement..."
        kubectl get pods -n $ns_name -o wide
        
        # Check Cilium agent logs
        echo "Debug: checking Cilium logs..."
        kubectl -n kube-system logs -l k8s-app=cilium --tail=20 || echo "No Cilium logs available"
      else
        echo "Connectivity test failed, trying again..."
      fi
    fi
  done
  
  # Test connectivity from client2 (different node, should fail if nodes have different CIDRs)
  echo "Testing connectivity from client2 (different node, should fail if nodes have different CIDRs)..."
  if [ "$NODE1_CIDR" != "$NODE2_CIDR" ]; then
    if ! kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
      print_success "Connection from client2 blocked as expected"
    else
      print_error "Connection from client2 succeeded, but should have been blocked"
    fi
  else
    print_info "Node1 and Node2 have the same CIDR ($NODE1_CIDR), skipping this test"
  fi
  
  # Clean up temporary directory but keep the policy applied
  # kubectl delete ciliumnetworkpolicy cidr-ingress-policy -n $ns_name 2>/dev/null || true
  cleanup_temp_files "$tmp_file"
  sleep 3
  print_info "Policy 'cidr-ingress-policy' has been left applied for further testing"
  
  print_success "CIDR ingress policy test completed"
}

# Test CIDR egress policy only
function test_cidr_egress_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING CIDR EGRESS POLICY"
  
  # Use the existing policy file with variable substitution
  local yaml_file="$POLICY_DIR/cidr-policies/cidr-egress-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test egress connectivity from client2 to API pod (should work as API is in NODE1_CIDR)
  echo "Testing egress from client2 to API pod (should work)..."
  
  # Try multiple times with increasing timeouts
  for attempt in {1..3}; do
    echo "Attempt $attempt with ${attempt}0 second timeout..."
    if kubectl exec -n $ns_name client2 -- curl -v --max-time ${attempt}0 http://$API_POD_IP 2>&1 | tee /dev/stderr | grep -q "Welcome to nginx"; then
      print_success "Egress from client2 to API pod works as expected (attempt $attempt)"
      CIDR_EGRESS_RESULT="${GREEN}PASSED${NC}"
      break
    else
      if [ $attempt -eq 3 ]; then
        print_error "Egress from client2 to API pod failed after 3 attempts, but should have worked"
        CIDR_EGRESS_RESULT="${RED}FAILED${NC}"
        
        # More debug info
        echo "Debug: checking policy status..."
        kubectl get ciliumnetworkpolicies -n $ns_name
        
        # Check if pods can resolve the API pod IP
        echo "Debug: checking DNS resolution..."
        kubectl exec -n $ns_name client2 -- nslookup $API_POD_IP || echo "No DNS resolution"
        
        # Check if ping works (ICMP)
        echo "Debug: checking if ping works..."
        kubectl exec -n $ns_name client2 -- ping -c 2 $API_POD_IP || echo "Ping failed"
      else
        echo "Connectivity test failed, trying again..."
      fi
    fi
  done
  
  # Clean up temporary directory but keep the policy applied
  # kubectl delete ciliumnetworkpolicy cidr-egress-policy -n $ns_name 2>/dev/null || true
  cleanup_temp_files "$tmp_file"
  sleep 3
  print_info "Policy 'cidr-egress-policy' has been left applied for further testing"
  
  print_success "CIDR egress policy test completed"
}

# Test CIDR with exceptions policy
function test_cidr_except_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING CIDR WITH EXCEPTIONS POLICY"
  
  print_info "This policy type uses CIDR blocks with exceptions (except CIDR)"
  print_info "It allows specifying IP ranges while excluding specific subnets"
  
  # Use the existing policy file with variable substitution
  local yaml_file="$POLICY_DIR/cidr-policies/cidr-with-except-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client1 (in the allowed CIDR but not in the exception)
  echo "Testing connectivity from client1 (should be allowed by CIDR rules)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client1 works as expected (in allowed CIDR)"
    CIDR_EXCEPT_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client1 failed, but should have worked"
    CIDR_EXCEPT_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'cidr-with-except-policy' has been left applied for further testing"
  
  print_success "CIDR with exceptions policy test completed"
}

# Function to cleanup only policies without destroying the namespace
function cleanup_policies_only() {
  local ns_name="l3-policy-test"
  local max_retries=3
  
  print_subheader "Cleaning up only Cilium policies (preserving test environment)"
  
  # First pass: Delete each policy individually by name
  for policy in $(kubectl get ciliumnetworkpolicies -n $ns_name --no-headers -o custom-columns=":metadata.name" 2>/dev/null); do
    echo "Explicitly deleting policy: $policy"
    kubectl delete ciliumnetworkpolicies -n $ns_name $policy --grace-period=0 --force 2>/dev/null || true
    sleep 3  # Increased wait time
  done
  
  # Check for and delete cluster-wide policies
  for policy in $(kubectl get ciliumclusterwidenetworkpolicies --no-headers -o custom-columns=":metadata.name" 2>/dev/null); do
    echo "Explicitly deleting cluster-wide policy: $policy"
    kubectl delete ciliumclusterwidenetworkpolicies $policy --grace-period=0 --force 2>/dev/null || true
    sleep 3  # Increased wait time
  done
  
  # Multiple retries for bulk deletion
  for attempt in $(seq 1 $max_retries); do
    # List remaining policies to debug
    echo "Checking for remaining policies (attempt $attempt)..."
    kubectl get ciliumnetworkpolicies -n $ns_name -o name 2>/dev/null || echo "No namespace policies found"
    kubectl get ciliumclusterwidenetworkpolicies -o name 2>/dev/null || echo "No cluster-wide policies found"
    
    # Remove finalizers from any remaining policies
    for policy in $(kubectl get ciliumnetworkpolicies -n $ns_name -o name 2>/dev/null); do
      echo "Removing finalizers from: $policy"
      kubectl patch $policy -n $ns_name -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
    done
    
    for policy in $(kubectl get ciliumclusterwidenetworkpolicies -o name 2>/dev/null); do
      echo "Removing finalizers from cluster-wide policy: $policy"
      kubectl patch $policy -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
    done
    
    # Forcefully delete everything again
    kubectl delete ciliumnetworkpolicies --all -n $ns_name --force --grace-period=0 2>/dev/null || true
    kubectl delete ciliumclusterwidenetworkpolicies --all --force --grace-period=0 2>/dev/null || true
    
    # If no policies remain, break the loop
    if ! kubectl get ciliumnetworkpolicies -n $ns_name &>/dev/null && ! kubectl get ciliumclusterwidenetworkpolicies &>/dev/null; then
      break
    fi
    
    sleep 5  # Wait longer between attempts
  done
  
  # Final verification
  echo "Performing final verification of policy cleanup..."
  local has_ns_policies=$(kubectl get ciliumnetworkpolicies -n $ns_name -o name 2>/dev/null || echo "")
  local has_cw_policies=$(kubectl get ciliumclusterwidenetworkpolicies -o name 2>/dev/null || echo "")
  
  if [ -z "$has_ns_policies" ] && [ -z "$has_cw_policies" ]; then
    print_success "All policies successfully removed"
    return 0
  else
    print_error "Some policies could not be removed"
    echo "Remaining namespace policies:"
    kubectl get ciliumnetworkpolicies -n $ns_name 2>/dev/null || echo "None"
    echo "Remaining cluster-wide policies:"
    kubectl get ciliumclusterwidenetworkpolicies 2>/dev/null || echo "None"
    
    # One last attempt with label selectors as last resort
    echo "Attempting deletion by label selectors..."
    kubectl delete ciliumnetworkpolicies -l app=api -n $ns_name --force --grace-period=0 2>/dev/null || true
    kubectl delete ciliumnetworkpolicies -l app=client -n $ns_name --force --grace-period=0 2>/dev/null || true
    kubectl delete ciliumclusterwidenetworkpolicies --all --force --grace-period=0 2>/dev/null || true
    sleep 3
  fi
  
  # Wait for policies to be fully removed before continuing
  print_info "Waiting for policy finalization to complete..."
  sleep 8
}

# Combined function to test all CIDR policies
function test_cidr_policies() {
  print_header "RUNNING ALL IP/CIDR-BASED POLICY TESTS (CILIUM CATEGORY 5)"
  echo "The following CIDR policy tests will be executed:"
  echo "1. CIDR Ingress Policy (cidr-ingress-policy.yaml)"
  echo "2. CIDR Egress Policy (cidr-egress-policy.yaml)"
  echo "3. CIDR with Exceptions Policy (cidr-with-except-policy.yaml)"
  echo
  
  # Run CIDR ingress policy test
  test_cidr_ingress_policy
  
  # Clean up previous policies before running next test
  cleanup_policies_only
  
  # Run CIDR egress policy test
  test_cidr_egress_policy
  
  # Clean up previous policies before running next test
  cleanup_policies_only
  
  # Run CIDR with exceptions policy test
  test_cidr_except_policy
  
  print_success "All CIDR policy tests completed"
  
  # Display summary of CIDR-based test results
  print_subheader "IP/CIDR-BASED POLICY TESTS SUMMARY"
  echo -e "CIDR Ingress Policy Test: ${CIDR_INGRESS_RESULT}"
  echo -e "CIDR Egress Policy Test: ${CIDR_EGRESS_RESULT}"
  echo -e "CIDR with Exceptions Policy Test: ${CIDR_EXCEPT_RESULT}"
}

# Combined function to test endpoints-based policies
function test_all_endpoints_policies() {
  print_header "RUNNING ALL ENDPOINTS-BASED POLICY TESTS (CILIUM CATEGORY 1)"
  
  test_endpoints_policy
  
  print_success "All endpoints-based policy tests completed"
  
  print_subheader "ENDPOINTS-BASED POLICY TESTS SUMMARY"
  echo -e "Endpoints Policy Test: ${ENDPOINTS_POLICY_RESULT}"
}

# Combined function to test services-based policies
function test_all_services_policies() {
  print_header "RUNNING ALL SERVICES-BASED POLICY TESTS (CILIUM CATEGORY 2)"
  
  test_services_policy
  
  print_success "All services-based policy tests completed"
  
  print_subheader "SERVICES-BASED POLICY TESTS SUMMARY"
  echo -e "Services Policy Test: ${SERVICES_POLICY_RESULT}"
}

# Combined function to test entities-based policies
function test_all_entities_policies() {
  print_header "RUNNING ALL ENTITIES-BASED POLICY TESTS (CILIUM CATEGORY 3)"
  
  test_entities_policy
  
  print_success "All entities-based policy tests completed"
  
  print_subheader "ENTITIES-BASED POLICY TESTS SUMMARY"
  echo -e "Entities Policy Test: ${ENTITIES_POLICY_RESULT}"
}

# Combined function to test DNS-based policies
function test_all_dns_policies() {
  print_header "RUNNING ALL DNS-BASED POLICY TESTS (CILIUM CATEGORY 6)"
  
  test_dns_policy
  
  print_success "All DNS-based policy tests completed"
  
  print_subheader "DNS-BASED POLICY TESTS SUMMARY"
  echo -e "DNS Policy Test: ${DNS_POLICY_RESULT}"
}

# Test pod node name policy only
function test_pod_node_name_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING POD NODE NAME POLICY"
  
  print_subheader "Testing pod node name policy"
  
  # Check if node name is available
  if [ -z "$NODE1" ]; then
    print_info "Node name not available, getting it from running pod..."
    NODE1=$(kubectl get pod client1 -n $ns_name -o jsonpath='{.spec.nodeName}')
    echo "Using node name from client1 pod: $NODE1"
  fi

  if [ -z "$NODE1" ]; then
    print_error "Could not determine node name, skipping node name policy test"
    return
  fi

  # Use the existing policy file with variable substitution
  local yaml_file="$POLICY_DIR/node-policies/pod-node-name-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumclusterwidenetworkpolicies
  
  # Test connectivity from client1 (same node, should work)
  echo "Testing connectivity from client1 (same node, should work)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client1 works as expected"
    POD_NODE_NAME_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client1 failed, but should have worked"
    POD_NODE_NAME_RESULT="${RED}FAILED${NC}"
  fi
  
  # Test connectivity from client2 (different node, should fail)
  if [ "$NODE1" != "$NODE2" ] && [ -n "$NODE1" ] && [ -n "$NODE2" ]; then
    echo "Testing connectivity from client2 (different node, should fail)..."
    if ! kubectl exec -n $ns_name client2 -- curl -s --max-time 10 http://$API_POD_IP | grep -q "Welcome to nginx"; then
      print_success "Connection from client2 blocked as expected"
    else
      print_error "Connection from client2 succeeded, but should have been blocked"
    fi
  else
    print_info "Only one node available or node names not properly detected, skipping cross-node test"
  fi
  
  # Clean up temporary directory but keep the policy applied
  # kubectl delete ciliumclusterwidenetworkpolicy pod-node-name-policy 2>/dev/null || true
  cleanup_temp_files "$tmp_file"
  sleep 3
  print_info "Policy 'pod-node-name-policy' has been left applied for further testing"
  
  print_success "Pod node name policy test completed"
}

# Test node CIDR policy only
function test_node_cidr_policy() {
  local ns_name="l3-policy-test"
  local client2_test_passed=true
  
  print_header "TESTING NODE CIDR POLICY"
  
  print_subheader "Testing node CIDR policy"
  
  # Use the existing policy file with variable substitution
  local yaml_file="$POLICY_DIR/node-policies/node-cidr-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client1 (same node, should work)
  echo "Testing connectivity from client1 (same node, should work)..."
  local client1_test_passed=false
  
  # Try multiple times with increasing timeouts
  for attempt in {1..3}; do
    echo "Attempt $attempt with ${attempt}0 second timeout..."
    if kubectl exec -n $ns_name client1 -- curl -v --max-time ${attempt}0 http://$API_POD_IP 2>&1 | tee /dev/stderr | grep -q "Welcome to nginx"; then
      print_success "Connectivity from client1 works as expected (attempt $attempt)"
      client1_test_passed=true
      break
    else
      if [ $attempt -eq 3 ]; then
        print_error "Connectivity from client1 failed after 3 attempts, but should have worked"
        client1_test_passed=false
        
        # More debug info
        echo "Debug: checking Cilium status..."
        kubectl -n kube-system exec -it $(kubectl -n kube-system get pods -l k8s-app=cilium -o jsonpath='{.items[0].metadata.name}') -- cilium status || echo "Couldn't get Cilium status"
      else
        echo "Connectivity test failed, trying again..."
      fi
    fi
  done
  
  # Test connectivity from client2 (different node, should fail if nodes have different CIDRs)
  if [ "$NODE1_CIDR" != "$NODE2_CIDR" ]; then
    echo "Testing connectivity from client2 (different node, should fail)..."
    if ! kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
      print_success "Connection from client2 blocked as expected"
      client2_test_passed=true
    else
      print_error "Connection from client2 succeeded, but should have been blocked"
      client2_test_passed=false
    fi
  else
    print_info "Nodes have the same CIDR, skipping cross-node test"
  fi
  
  # Set the overall result based on both tests
  if [ "$client1_test_passed" = true ] && [ "$client2_test_passed" = true ]; then
    NODE_CIDR_RESULT="${GREEN}PASSED${NC}"
  else
    NODE_CIDR_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  sleep 3
  print_info "Policy 'node-cidr-policy' has been left applied for further testing"
  
  print_success "Node CIDR policy test completed"
}

# Function to display summary of test results
function display_test_summary() {
  print_header "TEST RESULTS SUMMARY"
  
  echo -e "Basic Connectivity Test: ${BASIC_CONNECTIVITY_RESULT}"
  echo -e "Baseline Policy Test: ${BASELINE_RESULT}"
  echo -e "CIDR Ingress Policy Test: ${CIDR_INGRESS_RESULT}"
  echo -e "CIDR Egress Policy Test: ${CIDR_EGRESS_RESULT}"
  echo -e "Pod Node Name Policy Test: ${POD_NODE_NAME_RESULT}"
  echo -e "Node CIDR Policy Test: ${NODE_CIDR_RESULT}"
  
  echo
  local passed=0
  local failed=0
  local skipped=0
  
  for result in "$BASIC_CONNECTIVITY_RESULT" "$BASELINE_RESULT" "$CIDR_INGRESS_RESULT" "$CIDR_EGRESS_RESULT" "$POD_NODE_NAME_RESULT" "$NODE_CIDR_RESULT"; do
    if [ "$result" == "${GREEN}PASSED${NC}" ]; then
      ((passed++))
    elif [ "$result" == "${RED}FAILED${NC}" ]; then
      ((failed++))
    elif [ "$result" == "${YELLOW}SKIPPED${NC}" ]; then
      ((skipped++))
    fi
  done
  
  echo -e "Tests Passed: ${GREEN}$passed${NC}"
  echo -e "Tests Failed: ${RED}$failed${NC}"
  echo -e "Tests Skipped: ${YELLOW}$skipped${NC}"
  echo -e "Tests Not Run: ${YELLOW}$((5 - passed - failed - skipped))${NC}"
}

# Test baseline policy - simplest possible policy that should work in any Cilium environment
function test_baseline_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING BASELINE CILIUM POLICY ENFORCEMENT"
  
  print_info "This test applies the simplest possible policy that should work in any"
  print_info "properly configured Cilium environment. If this test fails, it indicates"
  print_info "that your Cilium configuration has non-standard policy enforcement settings."
  
  # Create a temporary policy file with completely open ingress
  TMP_POLICY_FILE=$(mktemp)
  cat > $TMP_POLICY_FILE << EOF
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "baseline-allow-all-ingress"
  namespace: $ns_name
spec:
  description: "Baseline test - allow all ingress to API pod"
  endpointSelector:
    matchLabels:
      app: api
  ingress:
  - fromEndpoints:
    - {}  # Empty fromEndpoints selector allows all traffic
EOF
  
  # Apply the policy
  echo "Applying baseline allow-all-ingress policy..."
  kubectl apply -f $TMP_POLICY_FILE
  
  # Also try creating a policy with 'run' label which may be used in some environments
  echo "Creating alternative policy with different label selector (for some environments)..."
  TMP_POLICY_FILE_ALT=$(mktemp)
  cat > $TMP_POLICY_FILE_ALT << EOF
apiVersion: "cilium.io/v2"
kind: CiliumNetworkPolicy
metadata:
  name: "baseline-allow-all-ingress-alt"
  namespace: $ns_name
spec:
  description: "Baseline test - allow all ingress (alternative label selector)"
  endpointSelector:
    matchLabels:
      run: api
  ingress:
  - fromEndpoints:
    - {}  # Empty fromEndpoints selector allows all traffic
EOF
  kubectl apply -f $TMP_POLICY_FILE_ALT
  
  # Wait longer for policy to be fully applied and synchronized
  echo "Waiting for policy to be applied..."
  sleep 10
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Check Cilium agent mode
  echo "Checking Cilium configuration..."
  
  # Try to get policy enforcement mode
  CILIUM_POD=$(kubectl -n kube-system get pods -l k8s-app=cilium -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [ -n "$CILIUM_POD" ]; then
    echo "Cilium agent pod found: $CILIUM_POD"
    echo "Checking policy enforcement mode..."
    kubectl -n kube-system exec $CILIUM_POD -- cilium config | grep PolicyEnforcement || echo "Could not determine policy enforcement mode"
  else
    echo "Could not find Cilium pod to check configuration"
  fi
  
  # Test connectivity from client1 (same node, should work with any policy)
  echo "Testing connectivity from client1 with baseline policy (should always work)..."
  
  # Try multiple times with increasing timeouts
  for attempt in {1..3}; do
    echo "Attempt $attempt with ${attempt}0 second timeout..."
    if kubectl exec -n $ns_name client1 -- curl -v --max-time ${attempt}0 http://$API_POD_IP 2>&1 | tee /dev/stderr | grep -q "Welcome to nginx"; then
      print_success "BASELINE TEST PASSED: Connectivity works as expected (attempt $attempt)"
      echo "✓ Your Cilium environment enforces policies as documented in standard configurations"
      BASELINE_RESULT="${GREEN}PASSED${NC}"
      break
    else
      if [ $attempt -eq 3 ]; then
        print_error "BASELINE TEST FAILED: Even with the simplest policy, connectivity failed"
        echo "✗ Your Cilium environment has non-standard policy enforcement settings"
        BASELINE_RESULT="${RED}FAILED${NC}"
        echo "This indicates that your Cilium configuration differs from standard documentation."
        echo "Possible causes:"
        echo "  1. Policy enforcement mode is set to 'always' instead of 'default'"
        echo "  2. Custom CNI configurations affecting Cilium behavior"
        echo "  3. Network plugin conflicts"
        echo "  4. Additional security plugins or admission controllers"
        
        # Try to get more debug info
        echo "Additional debugging information:"
        kubectl -n kube-system get cm cilium-config -o yaml 2>/dev/null || echo "Could not retrieve Cilium ConfigMap"
        
        # Check Cilium status
        echo "Checking Cilium status..."
        if [ -n "$CILIUM_POD" ]; then
          kubectl -n kube-system exec $CILIUM_POD -- cilium status || echo "Could not get Cilium status"
        fi
      else
        echo "Connectivity test failed, trying again with longer timeout..."
      fi
    fi
  done
  
  # Remove temporary files but keep the policies applied
  # kubectl delete -f $TMP_POLICY_FILE
  rm -f $TMP_POLICY_FILE
  # kubectl delete ciliumnetworkpolicy baseline-allow-all-ingress-alt -n $ns_name 2>/dev/null || true
  rm -f $TMP_POLICY_FILE_ALT 2>/dev/null || true
  sleep 3
  print_info "Baseline policies have been left applied for further testing"
  
  print_success "Baseline policy test completed"
}

# Test fromNodes selector policy (l3-node-policy)
function test_from_nodes_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING FROM-NODES POLICY (CILIUM CATEGORY 4)"
  
  print_info "This policy type uses fromNodes selector to specify nodes by hostname"
  print_info "It allows controlling traffic from specific nodes without pod details"
  
  # Apply the fromNodes policy
  local yaml_file="$POLICY_DIR/node-policies/l3-node-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client2 (should work as it's from NODE2)
  echo "Testing connectivity from client2 (should work as it's from NODE2)..."
  if kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client2 works as expected (fromNodes matching)"
    FROM_NODES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client2 failed, but should have worked"
    FROM_NODES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'l3-node-policy' has been left applied for further testing"
  
  print_success "FromNodes policy test completed"
}

# Test node entities policy (remote-node, host)
function test_node_entities_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING NODE ENTITIES POLICY (CILIUM CATEGORY 4)"
  
  print_info "This policy type uses remote-node and host entities"
  print_info "It allows controlling traffic from all nodes without specifying each one"
  
  # Apply the node entities policy
  local yaml_file="$POLICY_DIR/node-policies/node-based-policy-clusterwide.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumclusterwidenetworkpolicies
  
  # Test connectivity from client2 (should work as all nodes are allowed)
  echo "Testing connectivity from client2 (should work as remote-node entity allows it)..."
  if kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client2 works as expected (remote-node entity)"
    NODE_ENTITIES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client2 failed, but should have worked"
    NODE_ENTITIES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'node-based-policy-clusterwide' has been left applied for further testing"
  
  print_success "Node entities policy test completed"
}

# Combined function to test all node policies
function test_node_policies() {
  print_header "RUNNING ALL NODE-BASED POLICY TESTS (CILIUM CATEGORY 4)"
  echo "The following node policy tests will be executed:"
  echo "1. Node Name Policy (pod-node-name-policy.yaml)"
  echo "2. Node Selector Policy (node-cidr-policy.yaml)"
  echo "3. FromNodes Policy (l3-node-policy.yaml)"
  echo "4. Node Entities Policy (node-based-policy-clusterwide.yaml)"
  echo
  
  # Run all node policy tests
  test_pod_node_name_policy   # Renamed but keeping the same function for backward compatibility
  test_node_cidr_policy       # Renamed but keeping the same function for backward compatibility
  test_from_nodes_policy
  test_node_entities_policy
  
  print_success "All node policy tests completed"
  
  # Display summary of node-based test results
  print_subheader "NODE-BASED POLICY TESTS SUMMARY"
  echo -e "Pod Node Name Policy Test: ${POD_NODE_NAME_RESULT}"
  echo -e "Node Selector Policy Test: ${NODE_CIDR_RESULT}"
  echo -e "FromNodes Policy Test: ${FROM_NODES_POLICY_RESULT}"
  echo -e "Node Entities Policy Test: ${NODE_ENTITIES_POLICY_RESULT}"
}

# Process command line arguments
SUBTEST_TYPE=${1:-all}

# Variables to track test results
BASIC_CONNECTIVITY_RESULT="NOT_RUN"
BASELINE_RESULT="NOT_RUN"
CIDR_INGRESS_RESULT="NOT_RUN"
CIDR_EGRESS_RESULT="NOT_RUN"
POD_NODE_NAME_RESULT="NOT_RUN"
NODE_CIDR_RESULT="NOT_RUN"

# Important: Make sure errors don't cause early termination when running categories tests
if [ "$SUBTEST_TYPE" == "categories" ]; then
  set +e
fi

# Create or use an existing kind cluster for testing if no kubernetes cluster is available
if ! kubectl get nodes &> /dev/null; then
    print_info "No Kubernetes cluster detected, checking for kind..."
    if command -v kind &> /dev/null; then
        print_info "Creating a test kind cluster for Cilium policy tests..."
        kind create cluster --name cilium-policy-test
        print_success "Kind cluster created"
    else
        print_error "No Kubernetes cluster available and kind not installed. Please create a cluster first."
        exit 1
    fi
fi

# Special cases that don't need test environment
if [ "$SUBTEST_TYPE" == "list" ]; then
    list_subtests
    exit 0
elif [ "$SUBTEST_TYPE" == "help" ]; then
    show_help
    exit 0
elif [ "$SUBTEST_TYPE" == "cleanup" ]; then
    print_info "Running cleanup operation only..."
    cleanup_test_env
    exit 0
fi

# Set up test environment for all other tests
create_test_env
test_basic_connectivity

# Test endpoints-based policy (label selectors)
function test_endpoints_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING ENDPOINTS-BASED POLICY (CILIUM CATEGORY 1)"
  
  print_info "This policy type uses label selectors to select endpoints managed by Cilium"
  print_info "This is the most common type of Cilium policy and is completely decoupled from addressing"
  
  # Use the endpoints policy file (previously called traditional-node-selector)
  local yaml_file="$POLICY_DIR/endpoints-policies/endpoints-label-selector.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client1 (same namespace, should work based on label matching)
  echo "Testing connectivity from client1 (same namespace, should work)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client1 works as expected (label matching)"
    ENDPOINTS_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client1 failed, but should have worked"
    ENDPOINTS_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'endpoints-label-selector' has been left applied for further testing"
  
  print_success "Endpoints-based policy test completed"
}

# Test services-based policy
function test_services_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING SERVICES-BASED POLICY (CILIUM CATEGORY 2)"
  
  print_info "This policy type targets Kubernetes services rather than pods directly"
  print_info "It allows decoupling from direct pod IPs while still controlling traffic flow"
  
  # Check if service exists, if not create it
  if ! kubectl get service api-svc -n $ns_name &>/dev/null; then
    echo "Creating Kubernetes service pointing to API pod..."
    kubectl create service clusterip api-svc --tcp=80:80 -n $ns_name || true
    kubectl label service api-svc app=api -n $ns_name || true
    kubectl patch service api-svc -n $ns_name -p "{\"spec\":{\"selector\":{\"app\":\"api\"}}}" || true
    sleep 2
  fi
  
  # Get the service IP
  API_SVC_IP=$(kubectl get service api-svc -n $ns_name -o jsonpath='{.spec.clusterIP}')
  if [ -z "$API_SVC_IP" ]; then
    print_error "Failed to get service IP, skipping test"
    SERVICES_POLICY_RESULT="${YELLOW}SKIPPED${NC}"
    return
  fi
  
  echo "API Service IP: $API_SVC_IP"
  
  # Apply the services policy
  local yaml_file="$POLICY_DIR/services-policies/kubernetes-service-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Test connectivity to the service
  echo "Testing connectivity to the Kubernetes service..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_SVC_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity to service works as expected"
    SERVICES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity to service failed, but should have worked"
    SERVICES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'kubernetes-service-policy' has been left applied for further testing"
  
  print_success "Services-based policy test completed"
}

# Test entities-based policy
function test_entities_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING ENTITIES-BASED POLICY (CILIUM CATEGORY 3)"
  
  print_info "This policy type uses predefined entities like 'host', 'world', 'cluster'"
  print_info "It allows specifying remote peers without knowing their IP addresses"
  
  # Apply the entities policy
  local yaml_file="$POLICY_DIR/entities-policies/entities-based-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client1 (should work due to cluster entity)
  echo "Testing connectivity from client1 (should work due to 'cluster' entity)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client1 works as expected (cluster entity)"
    ENTITIES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client1 failed, but should have worked"
    ENTITIES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'entities-based-policy' has been left applied for further testing"
  
  print_success "Entities-based policy test completed"
}

# Test DNS-based policy
function test_dns_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING DNS-BASED POLICY (CILIUM CATEGORY 6)"
  
  print_info "This policy type uses DNS names converted to IPs via DNS lookups"
  print_info "It requires a working DNS setup and only works for egress traffic"
  
  # Apply the DNS policy
  local yaml_file="$POLICY_DIR/dns-policies/dns-based-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Check if we can access external domains (example.com)
  echo "Testing DNS-based egress to example.com (should work)..."
  if kubectl exec -n $ns_name client1 -- curl -s --max-time 10 https://example.com | grep -q "Example Domain"; then
    print_success "DNS-based egress to example.com works as expected"
    DNS_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    # Try with ping instead - some clusters may block HTTPS egress
    echo "HTTPS test failed, trying ICMP ping instead..."
    if kubectl exec -n $ns_name client1 -- ping -c 3 example.com &>/dev/null; then
      print_success "DNS resolution works (ping successful)"
      DNS_POLICY_RESULT="${GREEN}PASSED${NC}"
    else
      print_error "DNS-based egress failed, this may be normal if:"
      print_error "1. Your cluster doesn't have external connectivity"
      print_error "2. DNS redirection isn't configured in Cilium"
      print_error "3. Outbound traffic is blocked by firewalls"
      DNS_POLICY_RESULT="${YELLOW}SKIPPED${NC}"
    fi
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'dns-based-policy' has been left applied for further testing"
  
  print_success "DNS-based policy test completed"
}

# Test fromNodes selector policy (l3-node-policy)
function test_from_nodes_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING FROM-NODES POLICY (CILIUM CATEGORY 4)"
  
  print_info "This policy type uses fromNodes selector to specify nodes by hostname"
  print_info "It allows controlling traffic from specific nodes without pod details"
  
  # Apply the fromNodes policy
  local yaml_file="$POLICY_DIR/node-policies/l3-node-policy.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumnetworkpolicies -n $ns_name
  
  # Test connectivity from client2 (should work as it's from NODE2)
  echo "Testing connectivity from client2 (should work as it's from NODE2)..."
  if kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client2 works as expected (fromNodes matching)"
    FROM_NODES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client2 failed, but should have worked"
    FROM_NODES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'l3-node-policy' has been left applied for further testing"
  
  print_success "FromNodes policy test completed"
}

# Test node entities policy (remote-node, host)
function test_node_entities_policy() {
  local ns_name="l3-policy-test"
  
  print_header "TESTING NODE ENTITIES POLICY (CILIUM CATEGORY 4)"
  
  print_info "This policy type uses remote-node and host entities"
  print_info "It allows controlling traffic from all nodes without specifying each one"
  
  # Apply the node entities policy
  local yaml_file="$POLICY_DIR/node-policies/node-based-policy-clusterwide.yaml"
  local tmp_file=$(apply_policy_yaml "$yaml_file" "$ns_name")
  
  # Show policy status
  kubectl get ciliumclusterwidenetworkpolicies
  
  # Test connectivity from client2 (should work as all nodes are allowed)
  echo "Testing connectivity from client2 (should work as remote-node entity allows it)..."
  if kubectl exec -n $ns_name client2 -- curl -s --max-time 5 http://$API_POD_IP | grep -q "Welcome to nginx"; then
    print_success "Connectivity from client2 works as expected (remote-node entity)"
    NODE_ENTITIES_POLICY_RESULT="${GREEN}PASSED${NC}"
  else
    print_error "Connectivity from client2 failed, but should have worked"
    NODE_ENTITIES_POLICY_RESULT="${RED}FAILED${NC}"
  fi
  
  # Clean up temporary directory but keep the policy applied
  cleanup_temp_files "$tmp_file"
  print_info "Policy 'node-based-policy-clusterwide' has been left applied for further testing"
  
  print_success "Node entities policy test completed"
}

# Function to test by category with cleanup between each category
function test_by_category() {
  print_header "RUNNING TESTS BY CATEGORY WITH CLEANUP BETWEEN CATEGORIES"
  print_info "This will run tests by Cilium documentation categories with cleanup between each category"
  print_info "to prevent policy interference between categories."
  echo
  
  # Define all categories to run in sequence
  local categories=(
    "endpoints"
    "services"
    "entities"
    "node"
    "cidr"
    "dns"
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
      "endpoints")
        test_all_endpoints_policies
        result="${ENDPOINTS_POLICY_RESULT}"
        ;;
      "services")
        test_all_services_policies
        result="${SERVICES_POLICY_RESULT}"
        ;;
      "entities")
        test_all_entities_policies
        result="${ENTITIES_POLICY_RESULT}"
        ;;
      "node")
        test_node_policies
        # For node policies, we need to check multiple results
        if [ "${POD_NODE_NAME_RESULT}" == "${GREEN}PASSED${NC}" ] && \
           [ "${NODE_CIDR_RESULT}" == "${GREEN}PASSED${NC}" ] && \
           [ "${FROM_NODES_POLICY_RESULT}" == "${GREEN}PASSED${NC}" ] && \
           [ "${NODE_ENTITIES_POLICY_RESULT}" == "${GREEN}PASSED${NC}" ]; then
          result="${GREEN}PASSED${NC}"
        elif [ "${POD_NODE_NAME_RESULT}" == "${RED}FAILED${NC}" ] || \
             [ "${NODE_CIDR_RESULT}" == "${RED}FAILED${NC}" ] || \
             [ "${FROM_NODES_POLICY_RESULT}" == "${RED}FAILED${NC}" ] || \
             [ "${NODE_ENTITIES_POLICY_RESULT}" == "${RED}FAILED${NC}" ]; then
          result="${RED}FAILED${NC}"
        else
          result="${YELLOW}PARTIAL${NC}"
        fi
        ;;
      "cidr")
        test_cidr_policies
        # For CIDR policies, check multiple results
        if [ "${CIDR_INGRESS_RESULT}" == "${GREEN}PASSED${NC}" ] && \
           [ "${CIDR_EGRESS_RESULT}" == "${GREEN}PASSED${NC}" ] && \
           [ "${CIDR_EXCEPT_RESULT}" == "${GREEN}PASSED${NC}" ]; then
          result="${GREEN}PASSED${NC}"
        elif [ "${CIDR_INGRESS_RESULT}" == "${RED}FAILED${NC}" ] || \
             [ "${CIDR_EGRESS_RESULT}" == "${RED}FAILED${NC}" ] || \
             [ "${CIDR_EXCEPT_RESULT}" == "${RED}FAILED${NC}" ]; then
          result="${RED}FAILED${NC}"
        else
          result="${YELLOW}PARTIAL${NC}"
        fi
        ;;
      "dns")
        test_all_dns_policies
        result="${DNS_POLICY_RESULT}"
        ;;
    esac
    
    # Track results
    if [ "$result" == "${GREEN}PASSED${NC}" ]; then
      results+=("Category $category: ${GREEN}PASSED${NC}")
      ((passed++))
    elif [ "$result" == "${RED}FAILED${NC}" ]; then
      results+=("Category $category: ${RED}FAILED${NC}")
      ((failed++))
    elif [ "$result" == "${YELLOW}SKIPPED${NC}" ] || [ "$result" == "${YELLOW}PARTIAL${NC}" ]; then
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
  echo -e "Categories partial/skipped: ${YELLOW}$skipped${NC}"
  echo ""
  
  echo "Individual category results:"
  for result in "${results[@]}"; do
    echo -e "$result"
  done
  
  print_success "All category tests completed"
  
  # Final cleanup
  print_info "Performing final cleanup..."
  cleanup_test_env
}


# Run the specified subtest
case $SUBTEST_TYPE in
  "baseline")
    test_baseline_policy
    ;;
  # 1. ENDPOINTS-BASED POLICIES
  "endpoints")
    test_all_endpoints_policies
    ;;
  # 2. SERVICES-BASED POLICIES
  "services")
    test_all_services_policies
    ;;
  # 3. ENTITIES-BASED POLICIES
  "entities")
    test_all_entities_policies
    ;;
  # 4. NODE-BASED POLICIES
  "node-name")
    test_pod_node_name_policy  # Renamed but keeping the same function for backward compatibility
    ;;
  "node-selector")
    test_node_cidr_policy      # Renamed but keeping the same function for backward compatibility
    ;;
  "from-nodes")
    test_from_nodes_policy
    ;;
  "node-entities")
    test_node_entities_policy
    ;;
  "node")
    test_node_policies  # Run all node-based tests
    ;;
  # 5. CIDR-BASED POLICIES
  "cidr-ingress")
    test_cidr_ingress_policy
    ;;
  "cidr-egress")
    test_cidr_egress_policy
    ;;
  "cidr-except")
    test_cidr_except_policy
    ;;
  "cidr")
    test_cidr_policies  # Run all CIDR-based tests
    ;;
  # 6. DNS-BASED POLICIES
  "dns")
    test_all_dns_policies
    ;;
  "cleanup")
    # We handle cleanup as a special case above - this should never be reached
    print_info "Cleanup already handled"
    ;;
  "categories")
    # Run tests by category with cleanup between categories
    test_by_category
    ;;
  *)
    # Default behavior: if it's not a specific test or cleanup option, run categories
    if [ "$SUBTEST_TYPE" != "baseline" ] && \
       [ "$SUBTEST_TYPE" != "endpoints" ] && \
       [ "$SUBTEST_TYPE" != "services" ] && \
       [ "$SUBTEST_TYPE" != "entities" ] && \
       [ "$SUBTEST_TYPE" != "node-name" ] && \
       [ "$SUBTEST_TYPE" != "node-selector" ] && \
       [ "$SUBTEST_TYPE" != "from-nodes" ] && \
       [ "$SUBTEST_TYPE" != "node-entities" ] && \
       [ "$SUBTEST_TYPE" != "node" ] && \
       [ "$SUBTEST_TYPE" != "cidr-ingress" ] && \
       [ "$SUBTEST_TYPE" != "cidr-egress" ] && \
       [ "$SUBTEST_TYPE" != "cidr-except" ] && \
       [ "$SUBTEST_TYPE" != "cidr" ] && \
       [ "$SUBTEST_TYPE" != "dns" ] && \
       [ "$SUBTEST_TYPE" != "cleanup" ] && \
       [ "$SUBTEST_TYPE" != "list" ] && \
       [ "$SUBTEST_TYPE" != "help" ]; then
      # Run categories mode by default
      test_by_category
    else
      print_error "Unknown subtest type: $SUBTEST_TYPE"
      echo "Run './test-l3-policies.sh list' to see available subtests"
      exit 1
    fi
    ;;
esac

print_header "ALL TESTS COMPLETED"
echo -e "${YELLOW}Run './test-l3-policies.sh list' to see other available subtests${NC}"
