#!/bin/bash

# ==============================================================================
# Cilium L4 Network Policies Test Script
# ==============================================================================
#
# This script tests various types of Cilium L4 network policies with
# cleanup between tests to ensure isolation between test categories.
#
# Available subtests organized according to Cilium Documentation categories:
#
#   1. LIMIT INGRESS/EGRESS PORTS:
#      basic-ports   - Test basic port-based policies (TCP/UDP)
#
#   2. LIMIT ICMP/ICMPv6 TYPES:
#      icmp          - Test ICMP/ICMPv6 type-based policies
#
#   3. LIMIT TLS SERVER NAME INDICATION (SNI):
#      tls-sni       - Test TLS SNI-based policies
#
#
#   OTHER OPTIONS:
#      all           - Test all categories (default)
#      isolated-all  - Test all categories with cleanup between each test
#      cleanup       - Only clean up the test environment
#      list          - List all available subtests
#      help          - Show usage information
#

# Set strict shell options
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Store start time
START_TIME=$(date +%s)

# Constants
NAMESPACE="eks-a-l4-policies-test"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Variables to track test results (without associative arrays for compatibility)
BASIC_PORTS_RESULT=""
BASIC_PORTS_DESCRIPTION=""
ICMP_RESULT=""
ICMP_DESCRIPTION=""
TLS_SNI_RESULT=""
TLS_SNI_DESCRIPTION=""

# Define policy directories
BASIC_PORT_DIR="$SCRIPT_DIR/basic-port-policies"
ICMP_DIR="$SCRIPT_DIR/icmp-policies"
TLS_SNI_DIR="$SCRIPT_DIR/tls-sni-policies"

# Functions
function print_header() {
    echo -e "\n==================================================================="
    echo -e "= $1 "
    echo -e "===================================================================\n"
}

function print_subheader() {
    echo -e "\n==================================================================="
    echo -e "= TESTING $1 "
    echo -e "===================================================================\n"
}

function print_step() {
    echo "$(date +"%Y-%m-%d %H:%M:%S") $1"
}

function cleanup() {
    print_step "Cleaning up resources..."
    print_step "Deleting all Cilium policies (explicit deletion)..."

    # Find and delete all applied policies
    find "${SCRIPT_DIR}" -name "*.applied" | while read policy_file; do
        original_file="${policy_file%.applied}"
        policy_name=$(yq e '.metadata.name' "$original_file" 2>/dev/null || echo "unknown")
        if [[ -n "$policy_name" && "$policy_name" != "unknown" ]]; then
            print_step "Explicitly deleting policy: $policy_name"
            kubectl delete cnp -n $NAMESPACE $policy_name --force --grace-period=0 2>/dev/null || true
        fi
    done

    # Delete the namespace
    kubectl delete namespace $NAMESPACE --force --grace-period=0 2>/dev/null || true
    
    # Wait for namespace deletion to complete
    print_step "Waiting for namespace to be fully deleted..."
    while kubectl get namespace $NAMESPACE &>/dev/null; do
        sleep 1
    done
    print_step "Namespace $NAMESPACE successfully deleted"
    
    # Clean up applied files
    echo "Cleaning up .applied files..."
    find "${SCRIPT_DIR}" -name "*.applied" -delete
}

function setup_environment() {
    echo -e "\n>>> Setting up test environment \n"
    
    # Create namespace
    kubectl create namespace $NAMESPACE
    print_step "Created namespace: $NAMESPACE"
    
    # Find worker nodes (assuming at least 2)
    WORKER_NODES=$(kubectl get nodes -l node-role.kubernetes.io/worker= -o name | sed 's|node/||' | head -n 2)
    NODE1=$(echo "$WORKER_NODES" | head -n 1)
    NODE2=$(echo "$WORKER_NODES" | tail -n 1 | head -n 1)
    echo "✓ Found 2 worker nodes: $NODE1 $NODE2"
    
    # Create API pod on first node
    print_step "Creating target pod on $NODE1..."
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: api
  namespace: $NAMESPACE
  labels:
    app: api
spec:
  nodeName: $NODE1
  containers:
  - name: api
    image: nginx:alpine
    ports:
    - containerPort: 80
EOF
    
    # Create client pods on different nodes
    print_step "Creating client pods on different nodes..."
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: client1
  namespace: $NAMESPACE
  labels:
    app: client
spec:
  nodeName: $NODE1
  containers:
  - name: client
    image: curlimages/curl:latest
    command: ["sleep", "3600"]
EOF

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: client2
  namespace: $NAMESPACE
  labels:
    app: client
spec:
  nodeName: $NODE2
  containers:
  - name: client
    image: curlimages/curl:latest
    command: ["sleep", "3600"]
EOF

    # Wait for pods to be ready
    print_step "Waiting for pod api to be ready (timeout: 60s)..."
    kubectl wait --for=condition=ready pod/api -n $NAMESPACE --timeout=60s
    print_step "Pod api is ready"
    
    print_step "Waiting for pod client1 to be ready (timeout: 60s)..."
    kubectl wait --for=condition=ready pod/client1 -n $NAMESPACE --timeout=60s
    print_step "Pod client1 is ready"
    
    print_step "Waiting for pod client2 to be ready (timeout: 60s)..."
    kubectl wait --for=condition=ready pod/client2 -n $NAMESPACE --timeout=60s
    print_step "Pod client2 is ready"
    
    # Get pod IPs and node CIDRs
    API_POD_IP=$(kubectl get pod api -n $NAMESPACE -o jsonpath='{.status.podIP}')
    CLIENT1_POD_IP=$(kubectl get pod client1 -n $NAMESPACE -o jsonpath='{.status.podIP}')
    CLIENT2_POD_IP=$(kubectl get pod client2 -n $NAMESPACE -o jsonpath='{.status.podIP}')
    
    API_NODE=$(kubectl get pod api -n $NAMESPACE -o jsonpath='{.spec.nodeName}')
    CLIENT1_NODE=$(kubectl get pod client1 -n $NAMESPACE -o jsonpath='{.spec.nodeName}')
    CLIENT2_NODE=$(kubectl get pod client2 -n $NAMESPACE -o jsonpath='{.spec.nodeName}')
    
    NODE1_CIDR=$(kubectl get nodes $NODE1 -o jsonpath='{.spec.podCIDR}')
    NODE2_CIDR=$(kubectl get nodes $NODE2 -o jsonpath='{.spec.podCIDR}')
    
    print_step "API Pod IP: $API_POD_IP (on $API_NODE)"
    print_step "Client1 Pod IP: $CLIENT1_POD_IP (on $CLIENT1_NODE)"
    print_step "Client2 Pod IP: $CLIENT2_POD_IP (on $CLIENT2_NODE)"
    print_step "Node1 CIDR: $NODE1_CIDR"
    print_step "Node2 CIDR: $NODE2_CIDR"
    
    echo "✓ Test environment ready"
}

function test_connectivity() {
    echo -e "\n>>> Testing basic connectivity (no policies) \n"
    
    # Test ICMP ping from client1 (same node)
    print_step "Testing ICMP ping from client1 (same node)..."
    kubectl exec -n $NAMESPACE client1 -- ping -c 2 $API_POD_IP &>/dev/null
    if [ $? -eq 0 ]; then
        echo "✓ ICMP from client1 to API pod successful"
    else
        echo -e "${RED}✗ ICMP from client1 to API pod failed${NC}"
    fi
    
    # Test ICMP ping from client2 (different node)
    print_step "Testing ICMP ping from client2 (different node)..."
    kubectl exec -n $NAMESPACE client2 -- ping -c 2 $API_POD_IP &>/dev/null
    if [ $? -eq 0 ]; then
        echo "✓ ICMP from client2 to API pod successful"
    else
        echo -e "${RED}✗ ICMP from client2 to API pod failed${NC}"
    fi
    
    # Test HTTP from client1
    print_step "Testing HTTP connectivity from client1 (same node)..."
    kubectl exec -n $NAMESPACE client1 -- curl -s --max-time 5 http://$API_POD_IP | head -n 5
    if [ $? -eq 0 ]; then
        echo "✓ HTTP from client1 to API pod successful"
    else
        echo -e "${RED}✗ HTTP from client1 to API pod failed${NC}"
    fi
    
    # Test HTTP from client2
    print_step "Testing HTTP connectivity from client2 (different node)..."
    kubectl exec -n $NAMESPACE client2 -- curl -s --max-time 5 http://$API_POD_IP | head -n 5
    if [ $? -eq 0 ]; then
        echo "✓ HTTP from client2 to API pod successful"
    else
        echo -e "${RED}✗ HTTP from client2 to API pod failed${NC}"
    fi

    # Check if all tests passed
    if kubectl exec -n $NAMESPACE client1 -- ping -c 2 $API_POD_IP &>/dev/null && \
       kubectl exec -n $NAMESPACE client2 -- ping -c 2 $API_POD_IP &>/dev/null && \
       kubectl exec -n $NAMESPACE client1 -- curl -s --max-time 5 http://$API_POD_IP &>/dev/null && \
       kubectl exec -n $NAMESPACE client2 -- curl -s --max-time 5 http://$API_POD_IP &>/dev/null; then
        echo "✓ Basic connectivity test PASSED"
        echo "ACTUAL: Basic connectivity test passed"
        echo -e "RESULT: ${GREEN}✅ PASS (Network connectivity working as expected)${NC}"
    else
        echo -e "${RED}✗ Basic connectivity test FAILED${NC}"
        echo "ACTUAL: Some basic connectivity tests failed"
        echo -e "RESULT: ${RED}❌ FAIL (Network connectivity not working properly)${NC}"
    fi
}

function apply_policy() {
    local policy_file="$1"
    local ns_placeholder="{{NS_NAME}}"
    
    # Create temp file with namespace replaced
    local temp_file="${policy_file}.applied"
    cat "$policy_file" | sed "s/$ns_placeholder/$NAMESPACE/g" > "$temp_file"
    
    # Apply the policy
    kubectl apply -f "$temp_file"
    
    # Get the policy name and wait for it to become valid
    local policy_name=$(yq e '.metadata.name' "$temp_file")
    sleep 10 # Give Cilium time to process the policy
    
    # Check if policy is valid
    kubectl get cnp -n $NAMESPACE $policy_name -o wide
}

function test_basic_port_policies() {
    print_subheader "PORT-BASED POLICIES (CILIUM CATEGORY 1)"
    
    print_step "This policy type controls traffic based on L4 ports and protocols"
    local test_passed=true
    local test_description=""

    # Test TCP port ingress policy
    echo -e "\n>>> Testing TCP Port Ingress Policy\n"
    local policy_file="$BASIC_PORT_DIR/tcp-port-ingress-policy.yaml"
    echo "Applied policy: $(basename $policy_file)"
    echo -e "\nPolicy content:"
    cat "$policy_file" | sed -n '/spec:/,/---/p' | sed '/---/d'
    apply_policy "$policy_file"
    
    # Test connectivity
    print_step "Testing HTTP connectivity from client1 to API pod..."
    kubectl exec -n $NAMESPACE client1 -- curl -s --max-time 5 http://$API_POD_IP > /dev/null
    if [ $? -eq 0 ]; then
        echo "✓ HTTP connectivity works as expected"
        echo "ACTUAL: HTTP connectivity works as expected"
        echo -e "RESULT: ${GREEN}✅ PASS (Port-based policy correctly allows HTTP traffic)${NC}"
        test_description="TCP port policies correctly allow HTTP traffic"
    else
        echo -e "${RED}✗ HTTP connectivity failed${NC}"
        echo "ACTUAL: HTTP connectivity failed unexpectedly"
        echo -e "RESULT: ${RED}❌ FAIL (Port-based policy incorrectly blocks HTTP traffic)${NC}"
        test_passed=false
        test_description="TCP port policies incorrectly block HTTP traffic"
    fi
    
    # Record test results
    if [ "$test_passed" = true ]; then
        BASIC_PORTS_RESULT="${GREEN}PASS${NC}"
    else
        BASIC_PORTS_RESULT="${RED}FAIL${NC}"
    fi
    BASIC_PORTS_DESCRIPTION="$test_description"
    
    print_step "Policy '$(basename $policy_file)' has been applied for testing"
    echo "✓ TCP port ingress policy test completed"
    
    # Cleanup this policy before next test
    echo -e "\n>>> Cleaning up only the current policy (preserving test environment) \n"
    kubectl delete cnp -n $NAMESPACE tcp-port-ingress-policy --force --grace-period=0
    sleep 5
}

function test_icmp_policies() {
    print_subheader "ICMP/ICMPv6 TYPE POLICIES (CILIUM CATEGORY 2)"
    
    print_step "This policy type controls ICMP traffic based on ICMP types"
    local test_passed=true
    local test_description=""

    # Test ICMP type policy
    echo -e "\n>>> Testing ICMP Type Policy\n"
    local policy_file="$ICMP_DIR/icmp-type-policy.yaml"
    echo "Applied policy: $(basename $policy_file)"
    echo -e "\nPolicy content:"
    cat "$policy_file" | sed -n '/spec:/,/---/p' | sed '/---/d'
    apply_policy "$policy_file"
    
    # Test ICMP connectivity
    print_step "Testing ICMP ping from client1 to API pod..."
    kubectl exec -n $NAMESPACE client1 -- ping -c 2 $API_POD_IP > /dev/null
    if [ $? -eq 0 ]; then
        echo "✓ ICMP ping works as expected"
        echo "ACTUAL: ICMP ping works as expected"
        echo -e "RESULT: ${GREEN}✅ PASS (ICMP type policy correctly allows ping)${NC}"
        test_description="ICMP type policy correctly allows ping traffic"
    else
        echo -e "${RED}✗ ICMP ping failed${NC}"
        echo "ACTUAL: ICMP ping failed unexpectedly"
        echo -e "RESULT: ${RED}❌ FAIL (ICMP type policy incorrectly blocks ping)${NC}"
        test_passed=false
        test_description="ICMP type policy incorrectly blocks ping traffic"
    fi
    
    # Record test results
    if [ "$test_passed" = true ]; then
        ICMP_RESULT="${GREEN}PASS${NC}"
    else
        ICMP_RESULT="${RED}FAIL${NC}"
    fi
    ICMP_DESCRIPTION="$test_description"
    
    print_step "Policy '$(basename $policy_file)' has been applied for testing"
    echo "✓ ICMP type policy test completed"
    
    # Cleanup this policy before next test
    echo -e "\n>>> Cleaning up only the current policy (preserving test environment) \n"
    kubectl delete cnp -n $NAMESPACE icmp-type-policy --force --grace-period=0
    sleep 5
}

function test_tls_sni_policies() {
    print_subheader "TLS SNI POLICIES (CILIUM CATEGORY 3)"
    
    print_step "This policy type controls TLS traffic based on Server Name Indication (SNI)"
    print_step "NOTE: Full TLS SNI tests require external connectivity and L7 proxy enabled"
    local test_description=""

    # Test basic SNI policy
    echo -e "\n>>> Testing Basic TLS SNI Policy\n"
    local policy_file="$TLS_SNI_DIR/basic-sni-policy.yaml"
    echo "Applied policy: $(basename $policy_file)"
    echo -e "\nPolicy content:"
    cat "$policy_file" | sed -n '/spec:/,/---/p' | sed '/---/d'
    apply_policy "$policy_file"
    
    # We can only verify policy is applied, not functionality in this script
    print_step "TLS SNI policy applied successfully"
    print_step "NOTE: Testing TLS SNI requires external connectivity to TLS endpoints"
    print_step "and having L7 proxy enabled in Cilium"
    
    echo "ACTUAL: TLS SNI policy applied successfully"
    echo -e "RESULT: ${YELLOW}✓ PARTIAL (Policy applied, but full verification requires external connectivity)${NC}"
    test_description="TLS SNI policy applied but needs external validation"
    
    # Record test results
    TLS_SNI_RESULT="${YELLOW}PARTIAL${NC}"
    TLS_SNI_DESCRIPTION="$test_description"
    
    print_step "Policy '$(basename $policy_file)' has been applied for testing"
    echo "✓ TLS SNI policy test completed"
    
    # Cleanup this policy before next test
    echo -e "\n>>> Cleaning up only the current policy (preserving test environment) \n"
    kubectl delete cnp -n $NAMESPACE basic-sni-policy --force --grace-period=0
    sleep 5
}


function run_test_category() {
    local category="$1"
    
    # Always ensure a clean environment before each test
    print_step "Ensuring clean environment before testing $category category..."
    cleanup
    
    CURRENT_TEST="$category"
    
    setup_environment
    test_connectivity
    
    case "$category" in
        basic-ports)
            test_basic_port_policies
            ;;
        icmp)
            test_icmp_policies
            ;;
        tls-sni)
            test_tls_sni_policies
            ;;
        *)
            echo "Unknown test category: $category"
            exit 1
            ;;
    esac
}

function show_summary() {
    # Calculate elapsed time
    local END_TIME=$(date +%s)
    local ELAPSED=$((END_TIME - START_TIME))
    local minutes=$((ELAPSED / 60))
    local seconds=$((ELAPSED % 60))
    
    echo -e "\n==================================================================="
    echo -e "= DETAILED TEST RESULTS SUMMARY "
    echo -e "===================================================================\n"
    
    echo "Tests completed in ${minutes}m ${seconds}s"
    
    if [ -n "$CATEGORIES_TESTED" ]; then
        echo -e "Categories tested: $CATEGORIES_TESTED\n"
    fi
    
    # Display results table with test details
    echo -e "\n----- TEST RESULTS BY CATEGORY -----\n"
    printf "%-20s %-12s %s\n" "CATEGORY" "STATUS" "DESCRIPTION"
    echo "--------------------------------------------------------------------------------"
    
    # Statistics counters
    local passed=0
    local failed=0
    local partial=0
    local total=0
    
    # Display each test result if available
    if [ -n "$BASIC_PORTS_RESULT" ]; then
        printf "%-20s %-12b %s\n" "basic-ports" "$BASIC_PORTS_RESULT" "$BASIC_PORTS_DESCRIPTION"
        
        # Count by status
        if [[ "$BASIC_PORTS_RESULT" == *"PASS"* ]]; then
            ((passed++))
        elif [[ "$BASIC_PORTS_RESULT" == *"FAIL"* ]]; then
            ((failed++))
        elif [[ "$BASIC_PORTS_RESULT" == *"PARTIAL"* ]]; then
            ((partial++))
        fi
        ((total++))
    fi
    
    if [ -n "$ICMP_RESULT" ]; then
        printf "%-20s %-12b %s\n" "icmp" "$ICMP_RESULT" "$ICMP_DESCRIPTION"
        
        # Count by status
        if [[ "$ICMP_RESULT" == *"PASS"* ]]; then
            ((passed++))
        elif [[ "$ICMP_RESULT" == *"FAIL"* ]]; then
            ((failed++))
        elif [[ "$ICMP_RESULT" == *"PARTIAL"* ]]; then
            ((partial++))
        fi
        ((total++))
    fi
    
    if [ -n "$TLS_SNI_RESULT" ]; then
        printf "%-20s %-12b %s\n" "tls-sni" "$TLS_SNI_RESULT" "$TLS_SNI_DESCRIPTION"
        
        # Count by status
        if [[ "$TLS_SNI_RESULT" == *"PASS"* ]]; then
            ((passed++))
        elif [[ "$TLS_SNI_RESULT" == *"FAIL"* ]]; then
            ((failed++))
        elif [[ "$TLS_SNI_RESULT" == *"PARTIAL"* ]]; then
            ((partial++))
        fi
        ((total++))
    fi
    
    # Print statistics footer
    echo -e "\n----- SUMMARY STATISTICS -----\n"
    echo -e "Tests Passed:     ${GREEN}$passed${NC}"
    echo -e "Tests Failed:     ${RED}$failed${NC}"
    echo -e "Tests Partial:    ${YELLOW}$partial${NC}"
    echo -e "Total Tests Run:  ${total}"
    
    echo -e "\n===================================================================="
    
    # Final pass/fail determination
    if [ $failed -gt 0 ]; then
        echo -e "\n${RED}❌ SOME TESTS FAILED${NC} - Review failures above"
    elif [ $partial -gt 0 ]; then
        echo -e "\n${YELLOW}⚠️  SOME TESTS NEED VERIFICATION${NC} - See partial results above"
    else
        echo -e "\n${GREEN}✅ ALL TESTS PASSED SUCCESSFULLY${NC}"
    fi
}

function show_usage() {
    echo "Usage: $0 [test-category|action]"
    echo ""
    echo "Available test categories:"
    echo "  basic-ports   - Test basic port-based policies"
    echo "  icmp          - Test ICMP/ICMPv6 type-based policies"
    echo "  tls-sni       - Test TLS SNI-based policies"
    echo ""
    echo "Available actions:"
    echo "  all           - Test all categories (default)"
    echo "  isolated-all  - Test all categories with cleanup between each test"
    echo "  cleanup       - Only clean up the test environment"
    echo "  list          - List all available subtests"
    echo "  help          - Show this help message"
}

# Determine which test to run
CURRENT_TEST="first"
CATEGORIES_TESTED=""

case "$1" in
    basic-ports|icmp|tls-sni)
        run_test_category "$1"
        CATEGORIES_TESTED="$1"
        ;;
    all|"")
        # Run all tests with fresh environment for each category
        print_header "RUNNING ALL L4 POLICY TESTS"
        
        # Basic port policies
        print_step "Ensuring clean environment before basic port policies tests..."
        cleanup
        setup_environment
        test_connectivity
        test_basic_port_policies
        
        # ICMP policies
        print_step "Ensuring clean environment before ICMP policies tests..."
        cleanup
        setup_environment
        test_connectivity
        test_icmp_policies
        
        # TLS SNI policies
        print_step "Ensuring clean environment before TLS SNI policies tests..."
        cleanup
        setup_environment
        test_connectivity
        test_tls_sni_policies
        
        CATEGORIES_TESTED="all"
        ;;
    isolated-all)
        # Run all tests with cleanup between them
        print_header "RUNNING ALL L4 POLICY TESTS (WITH ISOLATION)"
        
        run_test_category "basic-ports"
        run_test_category "icmp"
        run_test_category "tls-sni"
        
        CATEGORIES_TESTED="all (isolated)"
        ;;
    cleanup)
        cleanup
        echo "Cleanup completed"
        exit 0
        ;;
    list)
        echo "Available test categories:"
        echo "  basic-ports"
        echo "  icmp"
        echo "  tls-sni"
        echo ""
        echo "Available actions:"
        echo "  all (default)"
        echo "  isolated-all"
        echo "  cleanup"
        echo "  list"
        echo "  help"
        exit 0
        ;;
    help|--help|-h)
        show_usage
        exit 0
        ;;
    *)
        echo "Unknown test category or action: $1"
        show_usage
        exit 1
        ;;
esac

# Clean up at the end
print_step "Performing final cleanup..."
cleanup

# Show summary
show_summary

print_header "ALL TESTS COMPLETED"
