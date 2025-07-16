#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Cluster names
BAD_CLUSTER="k8s-bad-config"
GOOD_CLUSTER="k8s-good-config"

# Function to print colored output
print_header() {
    echo -e "\n${BOLD}${MAGENTA}===== $1 =====${NC}\n"
}

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
usage() {
    cat << EOF
${BOLD}K8s Diagnostic Demo Script${NC}

This script manages two Kubernetes clusters with different Cilium CNI configurations
for easy demonstration of the k8s-diagnostic tool.

${BOLD}USAGE:${NC}
    $0 [COMMAND]

${BOLD}COMMANDS:${NC}
    create         Create both demo clusters if they don't exist
    test-bad       Run pod-to-pod test on misconfigured cluster (direct mode)
    test-good      Run pod-to-pod test on properly configured cluster (tunnel mode)
    test-bad-all   Run ALL tests on misconfigured cluster (direct mode)
    test-good-all  Run ALL tests on properly configured cluster (tunnel mode)
    status         Show status of both clusters
    cleanup        Delete both demo clusters
    help           Show this help message

${BOLD}EXAMPLES:${NC}
    $0 create       # Create both demo clusters
    $0 test-bad     # Run pod-to-pod test on misconfigured cluster
    $0 test-good    # Run pod-to-pod test on properly configured cluster
    $0 test-good-all # Run ALL tests on properly configured cluster
    $0 cleanup      # Delete both demo clusters when done

${BOLD}DEMO WORKFLOW:${NC}
    1. Run '$0 create' to set up both clusters
    2. Run '$0 test-bad' to show failing tests with misconfigured Cilium
    3. Run '$0 test-good' to show passing tests with proper configuration
    4. Run '$0 cleanup' when done to delete the clusters
EOF
}

# Check if both clusters exist
check_clusters() {
    local bad_exists=false
    local good_exists=false

    if kind get clusters 2>/dev/null | grep -q "^${BAD_CLUSTER}$"; then
        bad_exists=true
    fi
    
    if kind get clusters 2>/dev/null | grep -q "^${GOOD_CLUSTER}$"; then
        good_exists=true
    fi
    
    echo "$bad_exists,$good_exists"
}

# Create the demo clusters
create_clusters() {
    print_header "CREATING DEMO CLUSTERS"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local bad_exists
    local good_exists
    IFS=',' read -r bad_exists good_exists <<< "$cluster_status"
    
    if [[ "$bad_exists" == "true" ]]; then
        print_warn "Cluster '$BAD_CLUSTER' already exists"
    else
        print_info "Creating misconfigured cluster ($BAD_CLUSTER) with direct routing mode..."
        ./build_test_k8s.sh -n "$BAD_CLUSTER" -r direct
    fi
    
    if [[ "$good_exists" == "true" ]]; then
        print_warn "Cluster '$GOOD_CLUSTER' already exists"
    else
        print_info "Creating properly configured cluster ($GOOD_CLUSTER) with tunnel routing mode..."
        ./build_test_k8s.sh -n "$GOOD_CLUSTER" -r tunnel
    fi
    
    print_info "Both demo clusters are now ready"
    print_info "Run '$0 test-bad' or '$0 test-good' to run tests"
}

# Run pod-to-pod test on the "bad" cluster
test_bad_cluster() {
    print_header "TESTING MISCONFIGURED CLUSTER (DIRECT MODE)"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local bad_exists
    IFS=',' read -r bad_exists _ <<< "$cluster_status"
    
    if [[ "$bad_exists" == "false" ]]; then
        print_error "Cluster '$BAD_CLUSTER' doesn't exist"
        print_info "Run '$0 create' first to create the demo clusters"
        exit 1
    fi
    
    print_info "Switching context to cluster '$BAD_CLUSTER'"
    kubectl config use-context "kind-${BAD_CLUSTER}"
    
    print_info "Getting Cilium configuration (should show 'routing-mode: direct')"
    kubectl get configmaps -n kube-system cilium-config -o yaml | grep routing-mode
    
    print_info "Checking Cilium pod status (should show problems):"
    kubectl get pods -n kube-system -l k8s-app=cilium
    
    print_info "Running pod-to-pod test (should FAIL due to Cilium misconfiguration):"
    echo -e "${BLUE}============================================================${NC}"
    ./k8s-diagnostic test --test-list pod-to-pod
    echo -e "${BLUE}============================================================${NC}"
    
    print_info "Test complete - Cilium direct routing mode is incompatible with Kind clusters"
    print_info "Run '$0 test-good' to test the properly configured cluster"
}

# Run ALL tests on the "bad" cluster
test_bad_cluster_all() {
    print_header "RUNNING ALL TESTS ON MISCONFIGURED CLUSTER (DIRECT MODE)"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local bad_exists
    IFS=',' read -r bad_exists _ <<< "$cluster_status"
    
    if [[ "$bad_exists" == "false" ]]; then
        print_error "Cluster '$BAD_CLUSTER' doesn't exist"
        print_info "Run '$0 create' first to create the demo clusters"
        exit 1
    fi
    
    print_info "Switching context to cluster '$BAD_CLUSTER'"
    kubectl config use-context "kind-${BAD_CLUSTER}"
    
    print_info "Getting Cilium configuration (should show 'routing-mode: direct')"
    kubectl get configmaps -n kube-system cilium-config -o yaml | grep routing-mode
    
    print_info "Checking Cilium pod status (should show problems):"
    kubectl get pods -n kube-system -l k8s-app=cilium
    
    print_info "Running ALL diagnostic tests (should FAIL due to Cilium misconfiguration):"
    echo -e "${BLUE}============================================================${NC}"
    ./k8s-diagnostic test
    echo -e "${BLUE}============================================================${NC}"
    
    print_info "All tests complete - Most should fail due to Cilium routing mode misconfiguration"
    print_info "Run '$0 test-good-all' to run all tests on the properly configured cluster"
}

# Run pod-to-pod test on the "good" cluster
test_good_cluster() {
    print_header "TESTING PROPERLY CONFIGURED CLUSTER (TUNNEL MODE)"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local good_exists
    IFS=',' read -r _ good_exists <<< "$cluster_status"
    
    if [[ "$good_exists" == "false" ]]; then
        print_error "Cluster '$GOOD_CLUSTER' doesn't exist"
        print_info "Run '$0 create' first to create the demo clusters"
        exit 1
    fi
    
    print_info "Switching context to cluster '$GOOD_CLUSTER'"
    kubectl config use-context "kind-${GOOD_CLUSTER}"
    
    print_info "Getting Cilium configuration (should show 'routing-mode: tunnel')"
    kubectl get configmaps -n kube-system cilium-config -o yaml | grep routing-mode
    
    print_info "Checking Cilium pod status (should be healthy):"
    kubectl get pods -n kube-system -l k8s-app=cilium
    
    print_info "Running pod-to-pod test (should PASS with proper Cilium configuration):"
    echo -e "${BLUE}============================================================${NC}"
    ./k8s-diagnostic test --test-list pod-to-pod
    echo -e "${BLUE}============================================================${NC}"
    
    print_info "Test complete - Cilium tunnel routing mode works correctly in Kind clusters"
}

# Run ALL tests on the "good" cluster
test_good_cluster_all() {
    print_header "RUNNING ALL TESTS ON PROPERLY CONFIGURED CLUSTER (TUNNEL MODE)"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local good_exists
    IFS=',' read -r _ good_exists <<< "$cluster_status"
    
    if [[ "$good_exists" == "false" ]]; then
        print_error "Cluster '$GOOD_CLUSTER' doesn't exist"
        print_info "Run '$0 create' first to create the demo clusters"
        exit 1
    fi
    
    print_info "Switching context to cluster '$GOOD_CLUSTER'"
    kubectl config use-context "kind-${GOOD_CLUSTER}"
    
    print_info "Getting Cilium configuration (should show 'routing-mode: tunnel')"
    kubectl get configmaps -n kube-system cilium-config -o yaml | grep routing-mode
    
    print_info "Checking Cilium pod status (should be healthy):"
    kubectl get pods -n kube-system -l k8s-app=cilium
    
    print_info "Running ALL diagnostic tests (should PASS with proper Cilium configuration):"
    echo -e "${BLUE}============================================================${NC}"
    ./k8s-diagnostic test
    echo -e "${BLUE}============================================================${NC}"
    
    print_info "All tests complete - Should show successful results with tunnel routing mode"
}

# Show status of both clusters
show_status() {
    print_header "DEMO CLUSTERS STATUS"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local bad_exists
    local good_exists
    IFS=',' read -r bad_exists good_exists <<< "$cluster_status"
    
    echo "Misconfigured cluster ($BAD_CLUSTER): $(if [[ "$bad_exists" == "true" ]]; then echo "${GREEN}Created${NC}"; else echo "${RED}Not Created${NC}"; fi)"
    echo "Properly configured cluster ($GOOD_CLUSTER): $(if [[ "$good_exists" == "true" ]]; then echo "${GREEN}Created${NC}"; else echo "${RED}Not Created${NC}"; fi)"
    echo ""
    
    if [[ "$bad_exists" == "true" ]]; then
        kubectl config use-context "kind-${BAD_CLUSTER}" > /dev/null 2>&1
        echo "Cilium status in ${BAD_CLUSTER}:"
        kubectl get pods -n kube-system -l k8s-app=cilium 2>/dev/null || echo "Cannot access cluster"
        echo ""
    fi
    
    if [[ "$good_exists" == "true" ]]; then
        kubectl config use-context "kind-${GOOD_CLUSTER}" > /dev/null 2>&1
        echo "Cilium status in ${GOOD_CLUSTER}:"
        kubectl get pods -n kube-system -l k8s-app=cilium 2>/dev/null || echo "Cannot access cluster"
        echo ""
    fi
    
    echo "Current kubectl context: $(kubectl config current-context 2>/dev/null || echo "Not set")"
}

# Delete both demo clusters
cleanup_clusters() {
    print_header "CLEANING UP DEMO CLUSTERS"
    
    local cluster_status
    cluster_status=$(check_clusters)
    local bad_exists
    local good_exists
    IFS=',' read -r bad_exists good_exists <<< "$cluster_status"
    
    if [[ "$bad_exists" == "true" ]]; then
        print_info "Deleting cluster '$BAD_CLUSTER'..."
        kind delete cluster --name "$BAD_CLUSTER"
    else
        print_warn "Cluster '$BAD_CLUSTER' doesn't exist, nothing to delete"
    fi
    
    if [[ "$good_exists" == "true" ]]; then
        print_info "Deleting cluster '$GOOD_CLUSTER'..."
        kind delete cluster --name "$GOOD_CLUSTER"
    else
        print_warn "Cluster '$GOOD_CLUSTER' doesn't exist, nothing to delete"
    fi
    
    print_info "Cleanup complete"
}

# Main command handler
main() {
    case "$1" in
        create)
            create_clusters
            ;;
        test-bad)
            test_bad_cluster
            ;;
        test-good)
            test_good_cluster
            ;;
        test-bad-all)
            test_bad_cluster_all
            ;;
        test-good-all)
            test_good_cluster_all
            ;;
        status)
            show_status
            ;;
        cleanup)
            cleanup_clusters
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            print_error "Unknown command: $1"
            usage
            exit 1
            ;;
    esac
}

# Check if a command was provided
if [ $# -eq 0 ]; then
    usage
    exit 1
fi

# Run the script
main "$1"
