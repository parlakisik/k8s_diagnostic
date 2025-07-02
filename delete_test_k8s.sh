#!/bin/bash

set -e

# Default values
CLUSTER_NAME="k8s-diagnostic-test"

# Colors for output
RED='\033[0;31m'  
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
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
Usage: $0 [OPTIONS]

Delete a kind Kubernetes cluster.

OPTIONS:
    -n, --name NAME        Cluster name (default: k8s-diagnostic-test)
    -f, --force            Force deletion without confirmation
    -h, --help             Show this help message

EXAMPLES:
    $0                     # Delete cluster with default name (with confirmation)
    $0 -n my-test-cluster  # Delete cluster with custom name
    $0 -f                  # Force delete without confirmation
EOF
}

# Parse command line arguments
FORCE_DELETE=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        -f|--force)
            FORCE_DELETE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if kind is installed
check_prerequisites() {
    if ! command -v kind &> /dev/null; then
        print_error "kind is not installed"
        print_error "Please install from: https://kind.sigs.k8s.io/docs/user/quick-start/"
        exit 1
    fi
}

# Check if cluster exists
cluster_exists() {
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        return 0
    else
        return 1
    fi
}

# Delete the cluster
delete_cluster() {
    print_info "Checking if cluster '$CLUSTER_NAME' exists..."
    
    if ! cluster_exists; then
        print_warn "Cluster '$CLUSTER_NAME' does not exist"
        print_info "Available clusters:"
        kind get clusters 2>/dev/null || echo "  No kind clusters found"
        exit 0
    fi
    
    if [[ "$FORCE_DELETE" == false ]]; then
        print_warn "This will permanently delete the cluster '$CLUSTER_NAME'"
        read -p "Are you sure you want to continue? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deletion cancelled"
            exit 0
        fi
    fi
    
    print_info "Deleting kind cluster: $CLUSTER_NAME"
    
    if kind delete cluster --name "$CLUSTER_NAME"; then
        print_info "Cluster '$CLUSTER_NAME' deleted successfully"
    else
        print_error "Failed to delete cluster '$CLUSTER_NAME'"
        exit 1
    fi
}

# Clean up any leftover resources
cleanup_resources() {
    print_info "Cleaning up leftover resources..."
    
    # Remove any leftover kubectl contexts for this cluster
    local context_name="kind-${CLUSTER_NAME}"
    if kubectl config get-contexts -o name 2>/dev/null | grep -q "^${context_name}$"; then
        print_info "Removing kubectl context: $context_name"
        kubectl config delete-context "$context_name" 2>/dev/null || true
    fi
    
    # Clean up any Docker networks that might be left behind
    local network_name="kind"
    if docker network ls --format "{{.Name}}" 2>/dev/null | grep -q "^${network_name}$"; then
        print_info "Checking for unused kind Docker networks..."
        # Only remove if no containers are using it
        if [ "$(docker network inspect kind --format='{{len .Containers}}' 2>/dev/null || echo 0)" -eq 0 ]; then
            print_info "Removing unused kind Docker network"
            docker network rm kind 2>/dev/null || true
        fi
    fi
    
    print_info "Cleanup completed"
}

# Show remaining clusters
show_remaining_clusters() {
    local remaining_clusters
    remaining_clusters=$(kind get clusters 2>/dev/null || echo "")
    
    if [[ -n "$remaining_clusters" ]]; then
        print_info "Remaining kind clusters:"
        echo "$remaining_clusters" | sed 's/^/  - /'
    else
        print_info "No kind clusters remaining"
    fi
}

# Main execution
main() {
    echo "Kind Cluster Deletion Script"
    echo "================================"
    echo ""
    
    check_prerequisites
    delete_cluster
    cleanup_resources
    show_remaining_clusters
    
    print_info "Done!"
}

# Run main function
main "$@"
