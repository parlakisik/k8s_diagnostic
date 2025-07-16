#!/bin/bash

set -e

# Default values
CLUSTER_NAME="k8s-diagnostic-test"
ROUTING_MODE="tunnel"  # Default Cilium routing mode (tunnel, native, direct)

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

Create a simple 3-node kind Kubernetes cluster with Cilium CNI for testing.

OPTIONS:
    -n, --name NAME        Cluster name (default: k8s-diagnostic-test)
    -r, --routing MODE     Cilium routing mode (default: tunnel)
                           Available modes: tunnel, native, direct
    -h, --help             Show this help message

EXAMPLES:
    $0                     # Create cluster with default settings
    $0 -n my-test-cluster  # Create cluster with custom name
    $0 -r native           # Create cluster with native routing mode
    $0 -r direct           # Create cluster with direct routing mode
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        -r|--routing)
            ROUTING_MODE="$2"
            # Validate routing mode
            if [[ "$ROUTING_MODE" != "tunnel" && "$ROUTING_MODE" != "native" && "$ROUTING_MODE" != "direct" ]]; then
                print_error "Invalid routing mode: $ROUTING_MODE"
                print_error "Valid options are: tunnel, native, direct"
                exit 1
            fi
            shift 2
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

# Check if required tools are installed
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed or not in PATH"
        print_error "Please install Docker from: https://www.docker.com/"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running"
        print_error "Please start Docker daemon"
        exit 1
    fi
    
    if ! command -v kind &> /dev/null; then
        print_error "kind is not installed"
        print_error "Please install from: https://kind.sigs.k8s.io/docs/user/quick-start/"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed"
        print_error "Please install from: https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        print_error "helm is not installed"
        print_error "Please install from: https://helm.sh/docs/intro/install/"
        exit 1
    fi

    # Configure system settings for Kubernetes/Cilium
    print_info "Configuring system settings for Kubernetes..."
    
    # Detect operating system
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS specific settings
        print_info "Detected macOS - configuring file limits..."
        
        current_maxfiles=$(sysctl -n kern.maxfiles 2>/dev/null || echo "0")
        current_maxfilesperproc=$(sysctl -n kern.maxfilesperproc 2>/dev/null || echo "0")
        
        if [[ $current_maxfiles -lt 524288 ]]; then
            print_info "Increasing kern.maxfiles from $current_maxfiles to 524288"
            sudo sysctl -w kern.maxfiles=524288
        else
            print_info "kern.maxfiles is already sufficient ($current_maxfiles)"
        fi
        
        if [[ $current_maxfilesperproc -lt 524288 ]]; then
            print_info "Increasing kern.maxfilesperproc from $current_maxfilesperproc to 524288"
            sudo sysctl -w kern.maxfilesperproc=524288
        else
            print_info "kern.maxfilesperproc is already sufficient ($current_maxfilesperproc)"
        fi
        
    else
        # Linux specific settings
        print_info "Detected Linux - configuring inotify limits..."
        
        current_watches=$(sysctl -n fs.inotify.max_user_watches 2>/dev/null || echo "0")
        current_instances=$(sysctl -n fs.inotify.max_user_instances 2>/dev/null || echo "0")
        
        if [[ $current_watches -lt 524288 ]]; then
            print_info "Increasing fs.inotify.max_user_watches from $current_watches to 524288"
            sudo sysctl fs.inotify.max_user_watches=524288
        else
            print_info "fs.inotify.max_user_watches is already sufficient ($current_watches)"
        fi
        
        if [[ $current_instances -lt 512 ]]; then
            print_info "Increasing fs.inotify.max_user_instances from $current_instances to 512"
            sudo sysctl fs.inotify.max_user_instances=512
        else
            print_info "fs.inotify.max_user_instances is already sufficient ($current_instances)"
        fi
    fi
    
    print_info "All prerequisites satisfied"
}

# Create kind cluster configuration
create_kind_config() {
    local config_file="/tmp/kind-config-${CLUSTER_NAME}.yaml"
    
    cat > "$config_file" << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
nodes:
- role: control-plane
- role: worker
- role: worker
networking:
  disableDefaultCNI: true
  podSubnet: "10.244.0.0/16"
EOF
    
    echo "$config_file"
}

# Create kind cluster
create_cluster() {
    print_info "Creating 3-node kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        print_warn "Cluster '$CLUSTER_NAME' already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "Deleting existing cluster..."
            kind delete cluster --name "$CLUSTER_NAME"
        else
            print_info "Using existing cluster"
            return 0
        fi
    fi
    
    # Create kind configuration
    local config_file
    config_file=$(create_kind_config)
    
    # Create the cluster
    print_info "Creating cluster with configuration..."
    kind create cluster --config "$config_file"
    
    # Clean up temporary config
    rm -f "$config_file"
    
    print_info "Cluster created successfully"
}

# Install Cilium CNI
install_cilium() {
    print_info "Installing Cilium CNI using Helm..."
    
    # Add Cilium Helm repository
    print_info "Setting up Cilium Helm repository..."
    if helm repo list | grep -q "^cilium"; then
        print_info "Cilium repository already exists, updating..."
        helm repo update cilium
    else
        print_info "Adding Cilium Helm repository..."
        helm repo add cilium https://helm.cilium.io/
        helm repo update
    fi
    
    # Install Cilium using Helm with specified routing mode
    print_info "Installing Cilium v1.17.5 with routingMode: ${ROUTING_MODE}..."
    helm install cilium cilium/cilium --version 1.17.5 \
        --namespace kube-system \
        --set routingMode=${ROUTING_MODE}
    
    print_info "Waiting for Cilium to be ready..."
    
    # Wait for Cilium daemonset to be ready
    kubectl wait --for=condition=ready pod -l k8s-app=cilium -n kube-system --timeout=300s
    
    # Wait for nodes to be ready now that CNI is installed
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    print_info "Cilium CNI installed successfully with routing mode: ${ROUTING_MODE}"
}

# Show cluster information
show_cluster_info() {
    print_info "Test Kubernetes Cluster Information:"
    echo "====================================="
    echo "Cluster Name: $CLUSTER_NAME"
    echo "Cilium Routing Mode: $ROUTING_MODE"
    echo ""
    
    print_info "Nodes:"
    kubectl get nodes -o wide
    echo ""
    
    print_info "Cilium Pods:"
    kubectl get pods -n kube-system -l k8s-app=cilium
    echo ""
    
    # Display Cilium configuration
    print_info "Cilium Configuration:"
    kubectl get configmaps -n kube-system cilium-config -o yaml
    echo ""
    
    # Show current context
    local context
    context=$(kubectl config current-context 2>/dev/null || echo "unknown")
    print_info "Current kubectl context: $context"
    echo ""
}

# Main execution
main() {
    echo "Building Test Kubernetes Cluster with Cilium CNI"
    echo "=================================================="
    echo ""
    
    check_prerequisites
    create_cluster
    install_cilium
    show_cluster_info
    
    print_info "Test Kubernetes cluster '$CLUSTER_NAME' is ready!"
    print_info "You can now run diagnostic tests using the k8s-diagnostic CLI tool"
    print_info ""
    print_info "To delete this cluster later, run:"
    print_info "  kind delete cluster --name $CLUSTER_NAME"
}

# Run main function
main "$@"
