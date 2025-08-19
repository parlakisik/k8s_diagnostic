# k8s-diagnostic Kubernetes Deployment Guide


## Overview

This guide provides complete instructions for deploying the k8s-diagnostic application to a Kubernetes cluster. The application consists of a Next.js web UI and a Go CLI diagnostic tool running in a multi-container pod architecture.

## Architecture

```
┌─────────────────────────────────────────┐
│ k8s-diagnostic-ui Pod                   │
├─────────────────────┬───────────────────┤
│ UI Container        │ CLI Container     │
│ (Next.js Web App)   │ (Go Diagnostic)   │
│ Port: 3000          │ Sleep infinity    │
└─────────────────────┴───────────────────┘
           │
           ▼
┌─────────────────────────────────────────┐
│ Persistent Volume (1Gi)                 │
│ /app/shared/repository/test_results     │
└─────────────────────────────────────────┘
```

## Prerequisites

- **Kubernetes Cluster**: Kind cluster with Cilium CNI (recommended)
- **kubectl**: Configured and pointing to your cluster
- **Docker**: For building container images
- **Go 1.21+**: For building the CLI binary

### Verify Prerequisites

```bash
# Check cluster access
kubectl cluster-info

# Check storage class (should show 'standard' for kind)
kubectl get storageclass

# Verify Cilium is running
kubectl -n kube-system get pods | grep cilium
```

## Quick Start

For experienced users, use the automated deployment script:

```bash
# Build images and deploy (from project root)
./k8s/build-and-push-images.sh
./k8s/apply-k8s-manifests.sh

# Set up port forwarding
kubectl -n k8s-diagnostic port-forward service/k8s-diagnostic-ui 8080:3000 &

# Access application
open http://localhost:8080
```

## Step-by-Step Deployment

### Step 1: Build the Go CLI Binary

```bash
# From project root directory
make build-linux

# Verify binary was created
ls -la build/k8s-diagnostic-linux-amd64
```

### Step 2: Build Docker Images Locally

Since the images don't exist on Docker Hub, build them locally:

```bash
# Build CLI image (uses pre-built binary)
docker build -f docker/Dockerfile.cli-simple -t [username]/k8s-diagnostic-cli:latest .

# Build UI image (builds Next.js app)
docker build -f docker/Dockerfile.ui -t [username]/k8s-diagnostic-ui:latest .

# Verify images were created
docker images | grep k8s-diagnostic
```

**Expected Output:**
```
[username]/k8s-diagnostic-ui    latest    f2b385ceb0bb   5 minutes ago   450MB
[username]/k8s-diagnostic-cli   latest    98829ca98b12   5 minutes ago   95MB
```

### Step 3: Load Images into Kind Cluster

```bash
# Load both images into kind cluster
kind load docker-image [username]/k8s-diagnostic-cli:latest --name k8s-diagnostic-test
kind load docker-image [username]/k8s-diagnostic-ui:latest --name k8s-diagnostic-test
```

**Expected Output:**
```
Image: "[username]/k8s-diagnostic-cli:latest" with ID "sha256:..." not yet present on node "k8s-diagnostic-test-control-plane", loading...
Image: "[username]/k8s-diagnostic-cli:latest" with ID "sha256:..." not yet present on node "k8s-diagnostic-test-worker", loading...
Image: "[username]/k8s-diagnostic-cli:latest" with ID "sha256:..." not yet present on node "k8s-diagnostic-test-worker2", loading...
```

### Step 4: Deploy Kubernetes Resources

Apply the manifests in the correct order:

```bash
# 1. Create namespace
kubectl apply -f k8s/namespace.yaml

# 2. Set up RBAC
kubectl apply -f k8s/rbac-cli.yaml

# 3. Create PVC
kubectl apply -f k8s/pvc.yaml

# 4. Create service
kubectl apply -f k8s/service-ui-nodeport.yaml

# 5. Deploy application with environment variables
export DOCKERHUB_USERNAME=[username]
export IMAGE_TAG=latest
kubectl apply -f <(envsubst '${DOCKERHUB_USERNAME} ${IMAGE_TAG}' < k8s/deployment-ui.yaml)
```

### Step 5: Verify Deployment

```bash
# Check all resources
kubectl -n k8s-diagnostic get all

# Expected output should show:
# - 1 pod running (2/2 containers ready)
# - 1 service of type NodePort
# - 1 deployment ready (1/1)
# - 1 replicaset ready (1/1)
```

**Successful Deployment Output:**
```
NAME                                     READY   STATUS    RESTARTS   AGE
pod/k8s-diagnostic-ui-559b6cfbcb-ql6gs   2/2     Running   0          30s

NAME                        TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
service/k8s-diagnostic-ui   NodePort   10.96.189.187   <none>        3000:32030/TCP   5m

NAME                                READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/k8s-diagnostic-ui   1/1     1            1           30s

NAME                                           DESIRED   CURRENT   READY   AGE
replicaset.apps/k8s-diagnostic-ui-559b6cfbcb   1         1         1       30s
```

### Step 6: Access the Application

Set up port forwarding to access the web UI:

```bash
# Forward port 8080 to the service
kubectl -n k8s-diagnostic port-forward service/k8s-diagnostic-ui 8080:3000 --address=0.0.0.0 &

# Test connectivity
curl -I http://localhost:8080
```

**Expected HTTP Response:**
```
HTTP/1.1 200 OK
X-Powered-By: Next.js
Content-Type: text/html; charset=utf-8
Content-Length: 24232
```

## Accessing the Application

### Web Interface
- **URL**: http://localhost:8080
- **Features**: 
  - Cilium policy testing interface
  - Test execution and monitoring
  - Results visualization
  - Log viewing

### CLI Container Access
```bash
# Get pod name
POD_NAME=$(kubectl -n k8s-diagnostic get pods -l app=k8s-diagnostic-ui -o jsonpath='{.items[0].metadata.name}')

# Access CLI container
kubectl -n k8s-diagnostic exec -it $POD_NAME -c cli -- /bin/sh

# Run diagnostic commands
./k8s-diagnostic --help
```

## Troubleshooting

### Common Issues and Solutions

#### 1. PVC Stuck in Pending State

**Problem:**
```bash
kubectl -n k8s-diagnostic get pvc
NAME                         STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
k8s-diagnostic-results-pvc   Pending                                                     1m
```

**Solution:**
```bash
# Check storage classes available
kubectl get storageclass

# Update pvc.yaml to use correct storage class (usually 'standard' for kind)
# Ensure accessModes is ReadWriteOnce for single-node clusters
```

#### 2. ImagePullBackOff Error

**Problem:**
```bash
kubectl -n k8s-diagnostic get pods
NAME                                     READY   STATUS             RESTARTS   AGE
k8s-diagnostic-ui-55f7d478d4-dstw5       0/2     ImagePullBackOff   0          2m
```

**Root Cause:** Docker images don't exist on Docker Hub

**Solution:** Build and load images locally (Steps 2-3 above)

#### 3. Cannot Access Application on localhost:8080

**Problem:** Connection refused when accessing http://localhost:8080

**Solutions:**
```bash
# Check if port-forward is running
ps aux | grep "kubectl.*port-forward"

# Restart port-forward if needed
kubectl -n k8s-diagnostic port-forward service/k8s-diagnostic-ui 8080:3000 &

# Alternative: Use NodePort (if cluster supports it)
kubectl get nodes -o wide  # Get node IP
# Access via http://<NODE_IP>:32030
```

#### 4. CLI Container Not Starting

**Problem:** CLI container shows error or restart loop

**Diagnostic Commands:**
```bash
# Check container logs
kubectl -n k8s-diagnostic logs deployment/k8s-diagnostic-ui -c cli

# Check if binary exists and is executable
kubectl -n k8s-diagnostic exec deployment/k8s-diagnostic-ui -c cli -- ls -la /app/
```

### Checking Deployment Status

```bash
# Overall status
kubectl -n k8s-diagnostic get all

# Detailed pod information
kubectl -n k8s-diagnostic describe pod <pod-name>

# Check logs
kubectl -n k8s-diagnostic logs deployment/k8s-diagnostic-ui -c ui
kubectl -n k8s-diagnostic logs deployment/k8s-diagnostic-ui -c cli

# Check PVC binding
kubectl -n k8s-diagnostic get pv,pvc

# Verify RBAC
kubectl auth can-i create pods --as=system:serviceaccount:k8s-diagnostic:k8s-diagnostic-cli-sa
```

## Security Considerations

### ⚠️ Current Security Issues

This deployment has the following security vulnerabilities:

1. **Excessive RBAC Permissions**: ClusterRole has DELETE access to critical resources
2. **Root Containers**: Both containers run as root (UID 0)
3. **No Resource Limits**: Containers can consume unlimited resources
4. **No Security Context**: Missing security hardening configurations
5. **Docker CLI in Container**: Potential attack surface

### Recommended Security Improvements

```yaml
# Add to deployment-ui.yaml containers
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL

resources:
  limits:
    memory: "512Mi"
    cpu: "500m"
  requests:
    memory: "256Mi"
    cpu: "250m"
```

## Cleanup

### Remove Deployment

```bash
# Delete all resources
kubectl delete namespace k8s-diagnostic

# Or remove individually
kubectl -n k8s-diagnostic delete deployment k8s-diagnostic-ui
kubectl -n k8s-diagnostic delete service k8s-diagnostic-ui
kubectl -n k8s-diagnostic delete pvc k8s-diagnostic-results-pvc
kubectl delete clusterrolebinding k8s-diagnostic-cli-binding
kubectl delete clusterrole k8s-diagnostic-cli-role
kubectl delete namespace k8s-diagnostic

# Stop port-forward
pkill -f "kubectl.*port-forward.*k8s-diagnostic"
```

### Cleanup Docker Images (Optional)

```bash
# Remove locally built images
docker rmi [username]/k8s-diagnostic-ui:latest
docker rmi [username]/k8s-diagnostic-cli:latest
```

## File Structure

```
k8s/
├── README.md                    # This file
├── namespace.yaml              # Namespace definition
├── rbac-cli.yaml              # ServiceAccount & ClusterRole
├── pvc.yaml                   # Persistent Volume Claim
├── service-ui-nodeport.yaml   # NodePort Service
├── deployment-ui.yaml         # Main deployment manifest
├── ingress-ui.yaml            # Ingress (optional)
├── build-and-push-images.sh   # Image build script
└── apply-k8s-manifests.sh     # Deployment script
```

## Production Readiness Checklist

Before using in any production-like environment:

- [ ] **Security**: Implement proper security contexts
- [ ] **RBAC**: Use least-privilege principle
- [ ] **Resources**: Define resource limits and requests  
- [ ] **Health Checks**: Add liveness and readiness probes
- [ ] **High Availability**: Increase replica count
- [ ] **Storage**: Implement proper backup strategy
- [ ] **Monitoring**: Add observability and alerting
- [ ] **Network**: Use proper ingress instead of NodePort
- [ ] **Secrets**: Externalize configuration and secrets

---

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review pod logs: `kubectl -n k8s-diagnostic logs <pod-name> -c <container>`
3. Verify cluster compatibility with Cilium CNI
4. Ensure all prerequisites are met

**Last Updated**: August 18, 2025
**Tested On**: Kind v0.20.0, Kubernetes v1.33.1, Cilium CNI
