# 🚀 K8s Diagnostic Demo Commands - Quick Reference

## 🎯 ESSENTIAL DEMO COMMANDS

### 🔧 Port Forwarding Management
```bash
# Kill all port forwarding
pkill -f "kubectl port-forward.*k8s-diagnostic"

# Start stable port forwarding  
kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000 &

# Check port forwarding status
lsof -i :3000
ps aux | grep port-forward
```

### 📊 Pod Status & Health Checks
```bash
# Quick pod status
kubectl get pods -n k8s-diagnostic

# Detailed pod info with node placement
kubectl get pods -n k8s-diagnostic -o wide

# Check which images are running
kubectl describe pod -n k8s-diagnostic $(kubectl get pods -n k8s-diagnostic -o name | head -1 | cut -d/ -f2) | grep "Image:"

# Pod health summary
kubectl get pods -n k8s-diagnostic && kubectl describe deployment k8s-diagnostic-ui -n k8s-diagnostic | grep -A5 "Conditions:"
```

### 📋 Log Monitoring Commands
```bash
# UI Container Logs (recent)
kubectl logs k8s-diagnostic-ui-<POD_ID> -n k8s-diagnostic -c ui --tail=20

# CLI Container Logs (recent)  
kubectl logs k8s-diagnostic-ui-<POD_ID> -n k8s-diagnostic -c cli --tail=20

# Live UI logs (streaming)
kubectl logs -f -n k8s-diagnostic deployment/k8s-diagnostic-ui -c ui

# Live CLI logs (streaming)  
kubectl logs -f -n k8s-diagnostic deployment/k8s-diagnostic-ui -c cli

# Search for specific events
kubectl logs -n k8s-diagnostic deployment/k8s-diagnostic-ui -c ui --since=2m | grep -E "(CLIExecutionService|progress_update|cleanup)"
```

### 🚀 Deployment Commands
```bash
# Quick deploy with specific version
cd k8s && DOCKERHUB_USERNAME=daryakut453 ./deploy.sh --tag v22-performance-optimized

# Deploy without auto port forwarding  
cd k8s && DOCKERHUB_USERNAME=daryakut453 ./deploy.sh --tag v22-demo --no-launch

# Clean deployment (delete and redeploy)
kubectl delete deployment k8s-diagnostic-ui -n k8s-diagnostic
cd k8s && DOCKERHUB_USERNAME=daryakut453 ./deploy.sh --tag v22-demo

# Apply resource changes only
kubectl apply -f k8s/deployment-ui.yaml
kubectl rollout status deployment/k8s-diagnostic-ui -n k8s-diagnostic
```

### 🧹 Cleanup Commands
```bash
# Stop everything for clean demo
pkill -f "kubectl port-forward"
kubectl delete deployment k8s-diagnostic-ui -n k8s-diagnostic

# Full namespace cleanup (nuclear option)
kubectl delete namespace k8s-diagnostic

# Clean Docker images (local cleanup)
docker rmi $(docker images | grep "k8s-diagnostic" | awk '{print $3}')

# Clean old containers
docker system prune -f
```

### 🔍 Debug & Investigation Commands
```bash
# Check container resource usage
kubectl describe deployment k8s-diagnostic-ui -n k8s-diagnostic | grep -A10 -B5 "resources"

# Test kubectl performance comparison
time kubectl get nodes --no-headers | wc -l
kubectl exec -n k8s-diagnostic <POD_NAME> -c cli -- time kubectl get nodes --no-headers | wc -l

# Check API connectivity
kubectl exec -n k8s-diagnostic <POD_NAME> -c cli -- wget -q -O- http://localhost:8080/api/health

# Environment variable check
kubectl exec -n k8s-diagnostic <POD_NAME> -c ui -- env | grep KUBERNETES_MODE
```

### 📱 UI Access Commands  
```bash
# Auto-detect and setup UI access
./k8s/k8s-ui-access.sh --port-forward

# Manual UI access setup
kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000
# Then open: http://localhost:3000

# Check UI accessibility
curl -s http://localhost:3000/api/debug-environment | head -5
```

### 💾 Git Management
```bash
# Check current status
git status

# Commit changes
git add .
git commit -m "Demo: Production deployment working with performance analysis"

# Push to remote
git push origin feature/kubernetes-deployment

# Check recent commits
git log --oneline -5
```

---

## 🎬 DEMO FLOW SHORTCUTS

### Quick Demo Reset
```bash
# 1. Clean slate
pkill -f port-forward && kubectl delete deployment k8s-diagnostic-ui -n k8s-diagnostic

# 2. Fresh deploy  
cd k8s && DOCKERHUB_USERNAME=daryakut453 ./deploy.sh --tag demo-$(date +%H%M)

# 3. Manual stable port forward if needed
pkill -f port-forward && sleep 2 && kubectl port-forward -n k8s-diagnostic service/k8s-diagnostic-ui 3000:3000 &
```

### Performance Demo
```bash
# Show dev vs prod performance difference
echo "Host kubectl performance:"
time kubectl get nodes --no-headers | wc -l

echo "Container kubectl performance:"  
kubectl exec -n k8s-diagnostic $(kubectl get pods -n k8s-diagnostic -o name | head -1 | cut -d/ -f2) -c cli -- time kubectl get nodes --no-headers | wc -l
```

### Live Demo Monitoring
```bash
# Terminal 1: UI logs
kubectl logs -f -n k8s-diagnostic deployment/k8s-diagnostic-ui -c ui

# Terminal 2: CLI logs
kubectl logs -f -n k8s-diagnostic deployment/k8s-diagnostic-ui -c cli  

# Terminal 3: Pod status
watch kubectl get pods -n k8s-diagnostic
```

---

## 🔗 Quick Copy-Paste Commands

**Get current pod name:**
```bash
POD_NAME=$(kubectl get pods -n k8s-diagnostic -o jsonpath='{.items[0].metadata.name}')
echo $POD_NAME
```

**Check deployment version:**
```bash
kubectl get deployment k8s-diagnostic-ui -n k8s-diagnostic -o jsonpath='{.spec.template.spec.containers[*].image}' && echo
```

**One-liner health check:**
```bash
kubectl get pods -n k8s-diagnostic && kubectl exec -n k8s-diagnostic $(kubectl get pods -n k8s-diagnostic -o jsonpath='{.items[0].metadata.name}') -c cli -- wget -q -O- http://localhost:8080/api/health
```

---

## 🎯 Demo Talking Points

**Show unified architecture success:**
- "67-line API in production (was 800+ lines)"
- "Real-time cleanup and test progress"
- "Environment-aware dev/prod functionality"

**Performance analysis:**
- "Container kubectl 5x slower: 0.650s vs 0.128s host"
- "Tests complete successfully, optimization roadmap documented"

**Implementation highlights:**
- "TestID mismatch resolution"
- "Missing progress_update handler fix"
- "Environment-aware stop functionality"
