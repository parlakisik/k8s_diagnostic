#!/usr/bin/env bash
set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script to detect and provide correct UI access method for Kubernetes deployment
echo -e "${BLUE}🔍 k8s-diagnostic UI Access Detection${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Check if kubectl is available
if ! command -v kubectl >/dev/null 2>&1; then
    echo -e "${RED}❌ kubectl is not installed or not in PATH${NC}"
    echo "Please install kubectl and ensure it's configured"
    exit 1
fi

# Check if we can connect to cluster
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo -e "${RED}❌ Cannot connect to Kubernetes cluster${NC}"
    echo "Please ensure kubectl is configured with a valid context"
    exit 1
fi

echo -e "${GREEN}✅ Connected to Kubernetes cluster${NC}"
echo -e "${BLUE}Current context: $(kubectl config current-context)${NC}"
echo ""

# Check if k8s-diagnostic namespace exists
if ! kubectl get namespace k8s-diagnostic >/dev/null 2>&1; then
    echo -e "${RED}❌ k8s-diagnostic namespace not found${NC}"
    echo "Please deploy the application first using ./k8s/deploy.sh"
    exit 1
fi

# Check if deployment exists and is ready
echo -e "${BLUE}📋 Checking deployment status...${NC}"
if ! kubectl get deployment k8s-diagnostic-ui -n k8s-diagnostic >/dev/null 2>&1; then
    echo -e "${RED}❌ k8s-diagnostic-ui deployment not found${NC}"
    echo "Please deploy the application first using ./k8s/deploy.sh"
    exit 1
fi

# Check deployment readiness
READY_REPLICAS=$(kubectl get deployment k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
TOTAL_REPLICAS=$(kubectl get deployment k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "1")

if [[ "${READY_REPLICAS}" != "${TOTAL_REPLICAS}" ]]; then
    echo -e "${YELLOW}⚠️  Deployment not ready yet (${READY_REPLICAS}/${TOTAL_REPLICAS} replicas ready)${NC}"
    echo "Waiting for deployment to be ready..."
    
    if kubectl wait --for=condition=available deployment/k8s-diagnostic-ui -n k8s-diagnostic --timeout=60s; then
        echo -e "${GREEN}✅ Deployment is now ready${NC}"
    else
        echo -e "${RED}❌ Deployment failed to become ready${NC}"
        echo ""
        echo "Troubleshooting information:"
        kubectl get pods -n k8s-diagnostic
        exit 1
    fi
else
    echo -e "${GREEN}✅ Deployment is ready (${READY_REPLICAS}/${TOTAL_REPLICAS} replicas)${NC}"
fi

# Get service information
echo -e "${BLUE}🌐 Checking service configuration...${NC}"
if ! kubectl get service k8s-diagnostic-ui -n k8s-diagnostic >/dev/null 2>&1; then
    echo -e "${RED}❌ k8s-diagnostic-ui service not found${NC}"
    exit 1
fi

SERVICE_TYPE=$(kubectl get service k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.spec.type}')
NODEPORT=$(kubectl get service k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
CLUSTER_IP=$(kubectl get service k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.spec.clusterIP}')

echo -e "${GREEN}✅ Service found: ${SERVICE_TYPE}${NC}"
if [[ -n "${NODEPORT}" ]]; then
    echo -e "${BLUE}   NodePort: ${NODEPORT}${NC}"
fi
echo -e "${BLUE}   Cluster IP: ${CLUSTER_IP}${NC}"
echo ""

# Function to detect UI access method
detect_ui_access_method() {
    echo -e "${BLUE}🔍 Detecting best UI access method...${NC}"
    
    # Method 1: Try NodePort with node IPs (if NodePort service)
    if [[ "${SERVICE_TYPE}" == "NodePort" && -n "${NODEPORT}" ]]; then
        echo -e "${BLUE}📋 Trying NodePort access...${NC}"
        
        # Get node IPs
        NODE_IPS=($(kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo ""))
        
        if [[ ${#NODE_IPS[@]} -gt 0 ]]; then
            echo -e "${BLUE}   Available node IPs: ${NODE_IPS[*]}${NC}"
            
            # Test connectivity to first available node
            NODE_IP="${NODE_IPS[0]}"
            echo -e "${BLUE}   Testing connectivity to ${NODE_IP}:${NODEPORT}...${NC}"
            
            if timeout 5 bash -c "</dev/tcp/${NODE_IP}/${NODEPORT}" 2>/dev/null; then
                echo -e "${GREEN}✅ NodePort access successful${NC}"
                echo -e "${GREEN}🌐 UI Available at: http://${NODE_IP}:${NODEPORT}${NC}"
                
                # List all available node access points
                echo ""
                echo -e "${BLUE}📋 All available access points:${NC}"
                for ip in "${NODE_IPS[@]}"; do
                    echo -e "${BLUE}   • http://${ip}:${NODEPORT}${NC}"
                done
                return 0
            else
                echo -e "${YELLOW}⚠️  NodePort not accessible from host (this is common with kind/minikube)${NC}"
            fi
        else
            echo -e "${YELLOW}⚠️  Could not retrieve node IPs${NC}"
        fi
    fi
    
    # Method 2: Port forwarding (most reliable)
    echo -e "${BLUE}📋 Setting up port forwarding (recommended method)...${NC}"
    return 1
}

# Function to setup port forwarding
setup_port_forwarding() {
    local LOCAL_PORT="${1:-3000}"
    
    echo -e "${BLUE}🔄 Setting up kubectl port-forward...${NC}"
    echo -e "${YELLOW}This will forward local port ${LOCAL_PORT} to the UI service${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop port forwarding when done${NC}"
    echo ""
    
    # Check if port is already in use
    if lsof -Pi :${LOCAL_PORT} -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e "${YELLOW}⚠️  Port ${LOCAL_PORT} is already in use${NC}"
        echo -e "${BLUE}Trying alternative port 3001...${NC}"
        LOCAL_PORT=3001
        
        if lsof -Pi :${LOCAL_PORT} -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo -e "${YELLOW}⚠️  Port ${LOCAL_PORT} is also in use${NC}"
            echo -e "${BLUE}Trying alternative port 3002...${NC}"
            LOCAL_PORT=3002
        fi
    fi
    
    echo -e "${GREEN}🚀 Starting port forward on port ${LOCAL_PORT}...${NC}"
    echo -e "${GREEN}🌐 UI will be available at: http://localhost:${LOCAL_PORT}${NC}"
    echo ""
    echo -e "${BLUE}Command running: kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui ${LOCAL_PORT}:3000${NC}"
    echo ""
    
    # Start port forwarding
    kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui ${LOCAL_PORT}:3000
}

# Function to test HTTP API communication (validates our false positive fixes)
test_http_api_communication() {
    echo -e "${BLUE}🔬 Testing HTTP API Communication (False Positive Fix Validation)${NC}"
    echo -e "${BLUE}================================================================${NC}"
    
    POD_NAME=$(kubectl get pods -n k8s-diagnostic -l app=k8s-diagnostic-ui -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [[ -z "$POD_NAME" ]]; then
        echo -e "${RED}❌ No k8s-diagnostic-ui pods found${NC}"
        return 1
    fi
    
    echo -e "${BLUE}📋 Pod: $POD_NAME${NC}"
    
    # Test 1: Check environment variables
    echo -e "${BLUE}🌍 Testing environment variables...${NC}"
    
    echo -e "${BLUE}  UI Container environment:${NC}"
    UI_ENV=$(kubectl exec -n k8s-diagnostic $POD_NAME -c ui -- env | grep -E "(KUBERNETES|NODE_ENV|CLI|DOCKER)" || echo "No relevant env vars")
    echo -e "${BLUE}    $UI_ENV${NC}"
    
    echo -e "${BLUE}  CLI Container environment:${NC}"
    CLI_ENV=$(kubectl exec -n k8s-diagnostic $POD_NAME -c cli -- env | grep -E "(KUBERNETES|NODE_ENV|CLI|DOCKER|SHARED)" || echo "No relevant env vars")
    echo -e "${BLUE}    $CLI_ENV${NC}"
    
    # Test 2: CLI container health check
    echo -e "${BLUE}🏥 Testing CLI container HTTP server...${NC}"
    CLI_HEALTH=$(kubectl exec -n k8s-diagnostic $POD_NAME -c ui -- wget -qO- --timeout=10 http://localhost:8080/api/health 2>/dev/null || echo "FAILED")
    
    if [[ "$CLI_HEALTH" == "FAILED" ]]; then
        echo -e "${RED}❌ CLI container health check FAILED${NC}"
        echo -e "${YELLOW}  This will cause false positive test results!${NC}"
        
        echo -e "${BLUE}🔍 CLI container logs (last 10 lines):${NC}"
        kubectl logs $POD_NAME -n k8s-diagnostic -c cli --tail=10
        return 1
    else
        echo -e "${GREEN}✅ CLI container health check PASSED${NC}"
        echo -e "${BLUE}  Response: $CLI_HEALTH${NC}"
    fi
    
    # Test 3: Debug endpoint test
    echo -e "${BLUE}🔍 Testing debug endpoint...${NC}"
    
    # First, let's set up a temporary port forward to test the debug endpoint
    echo -e "${BLUE}  Setting up temporary port forward for testing...${NC}"
    kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3001:3000 > /dev/null 2>&1 &
    PORT_FORWARD_PID=$!
    
    # Wait for port forward to establish
    sleep 3
    
    DEBUG_RESPONSE=$(curl -s http://localhost:3001/api/debug-environment 2>/dev/null || echo "FAILED")
    
    # Clean up port forward
    kill $PORT_FORWARD_PID 2>/dev/null || true
    
    if [[ "$DEBUG_RESPONSE" == "FAILED" ]]; then
        echo -e "${YELLOW}⚠️  Debug endpoint test failed (not critical)${NC}"
    else
        echo -e "${GREEN}✅ Debug endpoint responded${NC}"
        
        # Parse key information from debug response
        if echo "$DEBUG_RESPONSE" | grep -q '"kubernetesModeBool":true'; then
            echo -e "${GREEN}  ✅ Kubernetes mode properly detected${NC}"
        else
            echo -e "${RED}  ❌ Kubernetes mode NOT detected - will cause false positives!${NC}"
        fi
        
        if echo "$DEBUG_RESPONSE" | grep -q '"finalExecutionPath":"HTTP_API"'; then
            echo -e "${GREEN}  ✅ HTTP API execution path selected${NC}"
        else
            echo -e "${RED}  ❌ HTTP API execution path NOT selected - will cause false positives!${NC}"
        fi
    fi
    
    echo -e "${GREEN}✅ HTTP API communication test completed${NC}"
    return 0
}

# Function to show testing instructions
show_testing_instructions() {
    echo ""
    echo -e "${BLUE}🧪 MANUAL TESTING INSTRUCTIONS${NC}"
    echo -e "${BLUE}==============================${NC}"
    echo ""
    echo -e "${GREEN}1. Open browser to the UI URL (shown above)${NC}"
    echo -e "${GREEN}2. Open browser developer console (F12)${NC}"
    echo -e "${GREEN}3. Click 'Start Tests' to run a batch of tests${NC}"
    echo ""
    echo -e "${BLUE}🔍 WHAT TO LOOK FOR (Signs of Success):${NC}"
    echo -e "${GREEN}  ✅ Console shows: 'KUBERNETES MODE DETECTED: true'${NC}"
    echo -e "${GREEN}  ✅ Console shows: 'CLI container health check PASSED'${NC}"
    echo -e "${GREEN}  ✅ CLI logs show: 'HTTP REQUEST #X RECEIVED'${NC}"
    echo -e "${GREEN}  ✅ Tests show real results or proper error messages${NC}"
    echo ""
    echo -e "${BLUE}🚨 SIGNS OF PROBLEMS (False Positives):${NC}"
    echo -e "${RED}  ❌ Tests showing 'PASSED - executed via HTTP API' with null duration${NC}"
    echo -e "${RED}  ❌ Console shows health check failed but tests still run${NC}"
    echo -e "${RED}  ❌ CLI logs show NO HTTP requests during test execution${NC}"
    echo -e "${RED}  ❌ All tests pass instantly without real execution${NC}"
    echo ""
    echo -e "${BLUE}📊 MONITORING COMMANDS (run in separate terminals):${NC}"
    echo -e "${BLUE}  UI logs:  kubectl logs -f $POD_NAME -n k8s-diagnostic -c ui${NC}"
    echo -e "${BLUE}  CLI logs: kubectl logs -f $POD_NAME -n k8s-diagnostic -c cli${NC}"
    echo ""
    echo -e "${YELLOW}💡 If you see false positives, the HTTP API communication fix needs debugging${NC}"
}

# Function to show access instructions
show_access_instructions() {
    echo -e "${GREEN}🎉 k8s-diagnostic UI Access Information${NC}"
    echo -e "${GREEN}=====================================${NC}"
    echo ""
    
    if detect_ui_access_method; then
        echo ""
        echo -e "${BLUE}Alternative method (if needed):${NC}"
        echo -e "${BLUE}  kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000${NC}"
        echo -e "${BLUE}  Then access: http://localhost:3000${NC}"
    else
        echo -e "${YELLOW}🔧 NodePort access not available from this host${NC}"
        echo -e "${GREEN}📡 Using port-forward method (most reliable):${NC}"
        echo ""
        echo -e "${BLUE}Option 1: Automatic setup (recommended)${NC}"
        echo -e "${BLUE}  Run this script with: ./k8s/k8s-ui-access.sh --port-forward${NC}"
        echo ""
        echo -e "${BLUE}Option 2: Manual setup${NC}"
        echo -e "${BLUE}  kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000${NC}"
        echo -e "${BLUE}  Then access: http://localhost:3000${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}🔍 Useful Commands:${NC}"
    echo -e "${BLUE}  Check deployment status: kubectl get all -n k8s-diagnostic${NC}"
    echo -e "${BLUE}  View UI logs: kubectl logs -n k8s-diagnostic deployment/k8s-diagnostic-ui -c ui${NC}"
    echo -e "${BLUE}  View CLI logs: kubectl logs -n k8s-diagnostic deployment/k8s-diagnostic-ui -c cli${NC}"
    echo ""
}

# Main function
main() {
    # Get pod name for use in testing instructions
    POD_NAME=$(kubectl get pods -n k8s-diagnostic -l app=k8s-diagnostic-ui -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    # Parse command line arguments
    case "${1:-}" in
        --port-forward)
            setup_port_forwarding "${2:-3000}"
            ;;
        --test)
            echo -e "${BLUE}🔬 Running HTTP API Communication Validation Test${NC}"
            echo ""
            if test_http_api_communication; then
                show_testing_instructions
                echo ""
                echo -e "${GREEN}🎯 Validation test completed successfully!${NC}"
                echo -e "${GREEN}Your deployment is ready for manual testing.${NC}"
            else
                echo ""
                echo -e "${RED}❌ Validation test failed!${NC}"
                echo -e "${YELLOW}Please check the issues above and redeploy if necessary.${NC}"
                exit 1
            fi
            ;;
        --validate)
            # Alias for --test
            echo -e "${BLUE}🔬 Running HTTP API Communication Validation Test${NC}"
            echo ""
            if test_http_api_communication; then
                show_testing_instructions
                echo ""
                echo -e "${GREEN}🎯 Validation test completed successfully!${NC}"
                echo -e "${GREEN}Your deployment is ready for manual testing.${NC}"
            else
                echo ""
                echo -e "${RED}❌ Validation test failed!${NC}"
                echo -e "${YELLOW}Please check the issues above and redeploy if necessary.${NC}"
                exit 1
            fi
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --port-forward [PORT]  Set up port forwarding (default port: 3000)"
            echo "  --test, --validate     Run HTTP API communication validation test"
            echo "  --help, -h             Show this help message"
            echo ""
            echo "Without arguments, shows available access methods and runs validation test"
            echo ""
            echo "Examples:"
            echo "  $0                     # Show access methods and run validation"
            echo "  $0 --port-forward      # Start port forwarding on default port"
            echo "  $0 --port-forward 3001 # Start port forwarding on port 3001"
            echo "  $0 --test              # Run validation test only"
            ;;
        "")
            show_access_instructions
            echo ""
            echo -e "${BLUE}🔬 Running HTTP API Communication Validation Test...${NC}"
            echo ""
            if test_http_api_communication; then
                show_testing_instructions
                echo ""
                echo -e "${GREEN}🎯 All checks passed! Your deployment is ready.${NC}"
            else
                echo ""
                echo -e "${RED}❌ Validation test failed!${NC}"
                echo -e "${YELLOW}Please check the issues above and redeploy if necessary.${NC}"
                echo ""
                show_testing_instructions
            fi
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"
