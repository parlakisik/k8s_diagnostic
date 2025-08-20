#!/usr/bin/env bash
set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo -e "${BLUE}🚀 k8s-diagnostic Kubernetes Deployment Script${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Auto-detect Docker Hub username from various sources
detect_dockerhub_username() {
    local detected_username=""
    
    # Try Docker config first
    if command -v docker >/dev/null 2>&1; then
        # Try to get from docker config without jq dependency
        if [[ -f "$HOME/.docker/config.json" ]]; then
            # Simple grep/sed approach to extract docker.io username
            detected_username=$(grep -o '"https://index.docker.io/v1/[^"]*"' "$HOME/.docker/config.json" 2>/dev/null | head -1 | sed 's/"https:\/\/index.docker.io\/v1\///' | sed 's/"$//' 2>/dev/null || echo "")
            if [[ -z "$detected_username" ]]; then
                detected_username=$(grep -o '"docker.io[^"]*"' "$HOME/.docker/config.json" 2>/dev/null | head -1 | sed 's/"docker.io[\/]*//g' | sed 's/"$//' 2>/dev/null || echo "")
            fi
        fi
        
        # Try docker info command for logged in user
        if [[ -z "$detected_username" ]]; then
            detected_username=$(docker info 2>/dev/null | grep -i "username:" | awk '{print $2}' || echo "")
        fi
        
        # Try docker system info
        if [[ -z "$detected_username" ]]; then
            detected_username=$(docker system info --format '{{.Username}}' 2>/dev/null || echo "")
        fi
        
        # Try getting from docker credentials store
        if [[ -z "$detected_username" ]] && command -v docker-credential-desktop >/dev/null 2>&1; then
            detected_username=$(echo "https://index.docker.io/v1/" | docker-credential-desktop get 2>/dev/null | grep -o '"Username":"[^"]*"' | cut -d'"' -f4 || echo "")
        fi
    fi
    
    echo "$detected_username"
}

# Interactive setup for missing configuration
setup_dockerhub_username() {
    local detected_username
    detected_username=$(detect_dockerhub_username)
    
    if [[ -n "$detected_username" ]]; then
        echo -e "${GREEN}🔍 Detected Docker Hub username: ${detected_username}${NC}"
        echo -e "${YELLOW}Do you want to use this username? (Y/n)${NC}"
        read -r REPLY
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            export DOCKERHUB_USERNAME="$detected_username"
            return 0
        fi
    fi
    
    echo -e "${YELLOW}Please enter your Docker Hub username:${NC}"
    read -r username
    
    if [[ -z "$username" ]]; then
        echo -e "${RED}❌ Username cannot be empty${NC}"
        return 1
    fi
    
    export DOCKERHUB_USERNAME="$username"
    echo -e "${GREEN}✅ Docker Hub username set to: ${username}${NC}"
    return 0
}

# Check if Docker is logged in
check_docker_login() {
    echo -e "${BLUE}🔐 Checking Docker Hub authentication...${NC}"
    
    # Try a simple Docker Hub API call to check auth
    if docker pull hello-world:latest >/dev/null 2>&1; then
        docker rmi hello-world:latest >/dev/null 2>&1
        echo -e "${GREEN}✅ Docker authentication verified${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️  Docker authentication may be required for image pushing${NC}"
        echo -e "${YELLOW}To log in to Docker Hub, run: docker login${NC}"
        echo -e "${YELLOW}Do you want to continue anyway? (y/N)${NC}"
        read -r REPLY
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            return 0
        else
            return 1
        fi
    fi
}

# Setup local Kubernetes if not available
setup_kubernetes() {
    echo -e "${YELLOW}⚠️  No Kubernetes cluster detected${NC}"
    echo -e "${BLUE}Available options:${NC}"
    echo "  1. Use Docker Desktop Kubernetes (recommended for local development)"
    echo "  2. Use minikube"
    echo "  3. Configure existing cluster manually"
    echo "  4. Exit and setup later"
    echo ""
    echo -e "${YELLOW}Choose an option (1-4):${NC}"
    read -r choice
    
    case $choice in
        1)
            echo -e "${BLUE}📖 To enable Docker Desktop Kubernetes:${NC}"
            echo "  1. Open Docker Desktop"
            echo "  2. Go to Settings > Kubernetes"
            echo "  3. Check 'Enable Kubernetes'"
            echo "  4. Click 'Apply & Restart'"
            echo "  5. Wait for Kubernetes to start (green indicator)"
            echo ""
            echo -e "${YELLOW}After enabling, press Enter to continue or Ctrl+C to exit${NC}"
            read -r
            ;;
        2)
            echo -e "${BLUE}📖 To setup minikube:${NC}"
            echo "  1. Install minikube: https://minikube.sigs.k8s.io/docs/start/"
            echo "  2. Run: minikube start"
            echo "  3. Run: minikube dashboard (optional)"
            echo ""
            echo -e "${YELLOW}After setup, press Enter to continue or Ctrl+C to exit${NC}"
            read -r
            ;;
        3)
            echo -e "${BLUE}📖 Configure your existing cluster:${NC}"
            echo "  1. Set up your kubeconfig file"
            echo "  2. Test with: kubectl cluster-info"
            echo ""
            echo -e "${YELLOW}After configuration, press Enter to continue or Ctrl+C to exit${NC}"
            read -r
            ;;
        4)
            echo "Exiting. Please set up Kubernetes and run this script again."
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option. Please try again.${NC}"
            setup_kubernetes
            ;;
    esac
}

# Validate prerequisites with interactive setup
validate_prerequisites() {
    echo -e "${BLUE}📋 Validating prerequisites...${NC}"
    
    # Check kubectl
    if ! command -v kubectl >/dev/null 2>&1; then
        echo -e "${RED}❌ kubectl is not installed or not in PATH${NC}"
        echo -e "${BLUE}📖 To install kubectl:${NC}"
        echo "  macOS: brew install kubectl"
        echo "  Linux: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
        echo "  Windows: https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/"
        exit 1
    fi
    echo -e "${GREEN}✅ kubectl found${NC}"
    
    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}❌ Docker is not installed or not in PATH${NC}"
        echo -e "${BLUE}📖 To install Docker:${NC}"
        echo "  Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi
    echo -e "${GREEN}✅ Docker found${NC}"
    
    # Check Docker daemon
    if ! docker info >/dev/null 2>&1; then
        echo -e "${RED}❌ Cannot connect to Docker daemon${NC}"
        echo "Please ensure Docker is running and try again"
        exit 1
    fi
    echo -e "${GREEN}✅ Docker daemon running${NC}"
    
    # Setup Docker Hub username
    if [[ -z "${DOCKERHUB_USERNAME:-}" ]]; then
        if ! setup_dockerhub_username; then
            echo -e "${RED}❌ Docker Hub username is required${NC}"
            exit 1
        fi
    fi
    echo -e "${GREEN}✅ Docker Hub username: ${DOCKERHUB_USERNAME}${NC}"
    
    # Check Docker authentication
    if ! check_docker_login; then
        echo -e "${RED}❌ Docker authentication failed${NC}"
        exit 1
    fi
    
    # Check kubectl context
    if ! kubectl cluster-info >/dev/null 2>&1; then
        setup_kubernetes
        # Re-check after setup
        if ! kubectl cluster-info >/dev/null 2>&1; then
            echo -e "${RED}❌ Still cannot connect to Kubernetes cluster${NC}"
            echo "Please ensure kubectl is configured with a valid context"
            exit 1
        fi
    fi
    echo -e "${GREEN}✅ Kubernetes cluster accessible${NC}"
    
    echo -e "${GREEN}✅ All prerequisites validated${NC}"
    echo ""
}

# Display current configuration
show_configuration() {
    echo -e "${BLUE}📊 Current Configuration:${NC}"
    echo "  Docker Hub Username: ${DOCKERHUB_USERNAME}"
    echo "  Image Tag: ${IMAGE_TAG:-latest}"
    echo "  Kubernetes Context: $(kubectl config current-context)"
    echo "  Kubernetes Cluster: $(kubectl cluster-info | head -1 | sed 's/.*https:\/\///' | sed 's/[[:space:]].*//')"
    echo ""
}

# Build and push Docker images
build_and_push_images() {
    echo -e "${BLUE}🔨 Building and pushing Docker images...${NC}"
    
    cd "${ROOT_DIR}"
    
    if [[ -f "${SCRIPT_DIR}/build-and-push-images.sh" ]]; then
        "${SCRIPT_DIR}/build-and-push-images.sh" "${IMAGE_TAG:-latest}"
    else
        echo -e "${YELLOW}⚠️  build-and-push-images.sh not found, building manually...${NC}"
        
        UI_IMAGE="${DOCKERHUB_USERNAME}/k8s-diagnostic-ui:${IMAGE_TAG:-latest}"
        CLI_IMAGE="${DOCKERHUB_USERNAME}/k8s-diagnostic-cli:${IMAGE_TAG:-latest}"
        
        echo "Building UI image: ${UI_IMAGE}"
        docker build -f docker/Dockerfile.ui -t "${UI_IMAGE}" .
        
        echo "Building CLI image: ${CLI_IMAGE}"
        docker build -f docker/Dockerfile.cli -t "${CLI_IMAGE}" .
        
        echo "Pushing images..."
        docker push "${UI_IMAGE}"
        docker push "${CLI_IMAGE}"
    fi
    
    echo -e "${GREEN}✅ Images built and pushed successfully${NC}"
    echo ""
}

# Deploy Kubernetes manifests
deploy_manifests() {
    echo -e "${BLUE}☸️  Deploying Kubernetes manifests...${NC}"
    
    cd "${SCRIPT_DIR}"
    
    if [[ -f "apply-k8s-manifests.sh" ]]; then
        # Use existing script
        DOCKERHUB_USERNAME="${DOCKERHUB_USERNAME}" IMAGE_TAG="${IMAGE_TAG:-latest}" ./apply-k8s-manifests.sh
    else
        echo -e "${YELLOW}⚠️  apply-k8s-manifests.sh not found, applying manually...${NC}"
        
        # Apply manifests manually
        declare -a FILES=(
            "namespace.yaml"
            "pvc.yaml"
            "rbac-cli.yaml"
            "deployment-ui.yaml"
            "service-ui-nodeport.yaml"
        )
        
        for f in "${FILES[@]}"; do
            if [[ -f "${f}" ]]; then
                echo "Applying ${f}..."
                sed -e "s#\${DOCKERHUB_USERNAME}#${DOCKERHUB_USERNAME}#g" \
                    -e "s#\${IMAGE_TAG}#${IMAGE_TAG:-latest}#g" "${f}" | kubectl apply -f -
            else
                echo -e "${YELLOW}⚠️  ${f} not found, skipping...${NC}"
            fi
        done
    fi
    
    echo -e "${GREEN}✅ Manifests deployed successfully${NC}"
    echo ""
}

# Wait for deployment to be ready
wait_for_deployment() {
    echo -e "${BLUE}⏳ Waiting for deployment to be ready...${NC}"
    
    # Wait for deployment to be available
    if kubectl wait --for=condition=available deployment/k8s-diagnostic-ui -n k8s-diagnostic --timeout=300s; then
        echo -e "${GREEN}✅ Deployment is ready!${NC}"
    else
        echo -e "${RED}❌ Deployment failed to become ready within 5 minutes${NC}"
        echo ""
        echo "Troubleshooting information:"
        kubectl get pods -n k8s-diagnostic
        echo ""
        kubectl describe deployment/k8s-diagnostic-ui -n k8s-diagnostic
        return 1
    fi
    echo ""
}

# Display access information with environment-aware detection
show_access_info() {
    echo -e "${GREEN}🎉 Deployment completed successfully!${NC}"
    echo ""
    echo -e "${BLUE}📱 UI Access Information:${NC}"
    echo ""
    
    # Use the UI access detection script for proper access method
    if [[ -f "${SCRIPT_DIR}/k8s-ui-access.sh" ]]; then
        echo -e "${GREEN}🔍 Detecting best UI access method for your environment...${NC}"
        echo ""
        
        # Run the access detection script
        "${SCRIPT_DIR}/k8s-ui-access.sh"
        
        echo ""
        echo -e "${BLUE}💡 Quick Access Options:${NC}"
        echo -e "${BLUE}  Option 1 (Recommended): ./k8s/k8s-ui-access.sh --port-forward${NC}"
        echo -e "${BLUE}  Option 2 (Manual): kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000${NC}"
        echo -e "${BLUE}             Then access: http://localhost:3000${NC}"
    else
        # Fallback to basic instructions if script not found
        echo -e "${YELLOW}⚠️  UI access detection script not found${NC}"
        echo -e "${BLUE}  Use port-forward for reliable access:${NC}"
        echo -e "${BLUE}     kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000${NC}"
        echo -e "${BLUE}     Then access: http://localhost:3000${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}🔍 Useful Commands:${NC}"
    echo "  📋 Check deployment status:"
    echo "     kubectl get all -n k8s-diagnostic"
    echo ""
    echo "  📄 View logs:"
    echo "     kubectl logs -n k8s-diagnostic deployment/k8s-diagnostic-ui -c ui -f"
    echo "     kubectl logs -n k8s-diagnostic deployment/k8s-diagnostic-ui -c cli -f"
    echo ""
    echo "  🧹 Clean up deployment:"
    echo "     kubectl delete namespace k8s-diagnostic"
    echo ""
    echo -e "${GREEN}✅ UI tests should now execute properly via HTTP API communication!${NC}"
}

# Health check
perform_health_check() {
    echo -e "${BLUE}🏥 Performing health check...${NC}"
    
    # Check if pods are running
    PODS_READY=$(kubectl get pods -n k8s-diagnostic -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' | grep -c "True" || echo "0")
    TOTAL_PODS=$(kubectl get pods -n k8s-diagnostic --no-headers | wc -l)
    
    if [[ "${PODS_READY}" -eq "${TOTAL_PODS}" ]] && [[ "${TOTAL_PODS}" -gt 0 ]]; then
        echo -e "${GREEN}✅ All pods are ready (${PODS_READY}/${TOTAL_PODS})${NC}"
        
        # Try to check CLI container health if possible
        echo -e "${BLUE}🔍 Checking CLI container health...${NC}"
        POD_NAME=$(kubectl get pods -n k8s-diagnostic -o jsonpath='{.items[0].metadata.name}')
        
        if kubectl exec -n k8s-diagnostic "${POD_NAME}" -c cli -- wget -q -O- http://localhost:8080/api/health >/dev/null 2>&1; then
            echo -e "${GREEN}✅ CLI container HTTP server is responding${NC}"
        else
            echo -e "${YELLOW}⚠️  CLI container HTTP server may not be ready yet${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  Some pods may not be ready yet (${PODS_READY}/${TOTAL_PODS})${NC}"
    fi
    echo ""
}

# Main deployment function
main() {
    # Set default image tag
    : "${IMAGE_TAG:=latest}"
    
    validate_prerequisites
    show_configuration
    
    # Confirm deployment
    echo -e "${YELLOW}🤔 Do you want to continue with the deployment? (y/N)${NC}"
    read -r REPLY
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Deployment cancelled."
        exit 0
    fi
    echo ""
    
    build_and_push_images
    deploy_manifests
    wait_for_deployment
    perform_health_check
    show_access_info
    
    echo -e "${GREEN}🚀 k8s-diagnostic is now ready to use!${NC}"
}

# Handle script interruption
trap 'echo -e "\n${RED}❌ Deployment interrupted${NC}"; exit 1' INT

# Run main function
main "$@"
