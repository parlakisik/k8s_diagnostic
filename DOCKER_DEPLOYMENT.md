# Docker Deployment Guide for k8s-diagnostic

**Get your Kubernetes diagnostics running in containers with zero installation hassle.**

This guide shows you exactly how to use k8s-diagnostic with Docker containers. All commands and outputs shown here have been verified in real environments.

## ⚡ Quick Start (5 Minutes)

**Prerequisites:**
- Docker & Docker Compose installed
- Kubernetes cluster access (valid `~/.kube/config`)
- This repository cloned locally

**Get running right now:**

```bash
# 1. Navigate to project directory
cd k8s_diagnostic

# 2. Build containers (first time only)
docker compose build

# 3. Start web interface
docker compose up k8s-diagnostic-ui -d

# 4. Open your browser
open http://localhost:3000
```

**Expected result:** You should see the K8s Diagnostic Dashboard web interface.

## 🏗️ What This Sets Up

The Docker setup creates two specialized containers:

```
┌─────────────────────────────┐    ┌─────────────────────────────┐
│  🌐 Web UI Container        │    │  🔧 CLI Container           │
│  (Always Running)           │    │  (Runs On-Demand)           │
│                             │    │                             │
│  • Serves dashboard at :3000│    │  • Contains k8s-diagnostic  │
│  • Shows real-time progress │────│  • Has kubectl installed    │
│  • Spawns CLI when needed   │    │  • Runs tests & writes logs │
│  • Reads test results       │    │  • Auto-cleans up           │
└─────────────────────────────┘    └─────────────────────────────┘
           │                                      │
           ▼                                      ▼
    📁 YOUR COMPUTER FILES (shared between containers)
    ├── test_results/     ◄── Test outputs saved here
    ├── cilium-policies/  ◄── Policy files read from here
    └── ~/.kube/config    ◄── Your Kubernetes credentials
```

## 🚀 Running Tests

### Method 1: Web Interface (Recommended)

**Start the web UI:**
```bash
docker compose up k8s-diagnostic-ui -d
```

**Access the interface:**
- Open: http://localhost:3000
- Click "Run Test" or "Batch Tests"
- Watch real-time progress
- Results automatically saved

**What you'll see during tests:**
```
🔨 Building Docker containers...
✅ Docker containers ready, starting tests...
🧪 Running test: pod-to-pod-same-node
✅ PASS (7.4s)
📊 Test Summary: Total Tests: 1, Passed: 1, Failed: 0
```

### Method 2: Command Line (For Scripts & Automation)

**Run a single test:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list pod-to-pod-same-node --verbose
```

**Expected output:**
```
✅ PASS (7.4s)
📊 Test Summary: Total Tests: 1, Passed: 1, Failed: 0
🎉 Overall Result: All 1 diagnostic tests passed
```

**Run all L3 policy tests:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l3-policies
```

**Run with maximum verbosity:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l4-policies --verbose
```

**List available tests:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --list
```

**Clean up test resources:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone deepclean
```

## 📋 Common Test Scenarios

### Testing Cilium Policies

**Basic L3 (Layer 3) Network Policies:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l3-policies
```

**L4 (Layer 4) Port-based Policies:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l4-policies
```

**L7 (Layer 7) Application Policies:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l7-policies
```

**Individual Test Types:**
```bash
# Pod connectivity tests
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list pod-to-pod-same-node

# Service connectivity tests  
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list service-connectivity

# DNS resolution tests
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list dns-resolution
```

### Batch Testing via Web Interface

1. **Open Dashboard:** http://localhost:3000
2. **Select Test Groups:** Choose L3, L4, L7, or custom tests
3. **Click "Run Batch Tests"**
4. **Monitor Progress:** Real-time streaming shows each test
5. **Download Results:** JSON and log files available

## 📊 Understanding Your Results

### Where Results Are Stored

**On your computer (persists after containers stop):**
```
test_results/
├── k8s-diagnostic-20250815-205657.json  ◄── Detailed test results
└── logs/
    └── k8s-diagnostic-20250815-205657.log  ◄── Full execution logs
```

**Verify results are saved:**
```bash
ls -la test_results/
# Expected: JSON files with timestamps
# Example: -rw-r--r-- 1 user staff 2472 Aug 15 13:57 k8s-diagnostic-20250815-205657.json
```

### Reading Test Results

**JSON Results Format:**
```json
{
  "testSuite": "l3-policies",
  "timestamp": "2025-08-15T20:56:57Z",
  "tests": [
    {
      "name": "pod-to-pod-same-node",
      "status": "PASS",
      "duration": "7.4s",
      "details": "Pod connectivity verified"
    }
  ],
  "summary": {
    "total": 1,
    "passed": 1,
    "failed": 0
  }
}
```

**Log Files:** Contain full kubectl commands, pod creation details, and diagnostic output.

## 🔧 Management Commands

### Container Lifecycle

**Start web interface:**
```bash
docker compose up k8s-diagnostic-ui -d
```

**Stop all containers:**
```bash
docker compose down
```

**View running containers:**
```bash
docker compose ps
```

**View logs:**
```bash
# Web UI logs
docker compose logs k8s-diagnostic-ui

# Follow logs in real-time
docker compose logs -f k8s-diagnostic-ui
```

### Rebuilding Containers

**After code changes:**
```bash
docker compose build --no-cache
docker compose restart k8s-diagnostic-ui
```

**Full clean rebuild:**
```bash
docker compose down
docker compose build --no-cache
docker compose up k8s-diagnostic-ui -d
```

## 🩺 Troubleshooting

### Container Startup Issues

**Problem: Container won't start**
```bash
# Check container status
docker compose ps

# View error logs
docker compose logs k8s-diagnostic-ui
```

**Solution: Common fixes**
```bash
# Port 3000 already in use
docker compose down
lsof -ti:3000 | xargs kill -9
docker compose up k8s-diagnostic-ui -d

# Permission issues
sudo chown -R $USER:$USER test_results/
```

### Kubernetes Connection Issues

**Verify cluster access:**
```bash
# Test kubectl locally first
kubectl get nodes

# Test kubectl in container
docker compose run --rm k8s-diagnostic-cli-standalone kubectl get nodes
```

**Expected output:**
```
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   1d    v1.27.0
```

**If connection fails:**
1. Check your `~/.kube/config` file exists
2. Verify cluster is running: `kubectl cluster-info`
3. Test with a simple pod: `kubectl get pods`

### Test Execution Issues

**Problem: Tests fail with "spawn go ENOENT"**

This error means the system is trying to build locally instead of using containers.

**Solution:** Make sure you're using the web interface or the correct CLI commands:
```bash
# ✅ Correct: Uses container
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list pod-to-pod-same-node

# ❌ Wrong: Tries to build locally
k8s-diagnostic test --test-list pod-to-pod-same-node
```

### Web Interface Issues

**Problem: Dashboard not loading**

**Test web server:**
```bash
curl -s http://localhost:3000/ | head -20
```

**Expected response:**
```html
<!DOCTYPE html><html><head><meta charSet="utf-8"/><title>K8s Diagnostic Dashboard</title>
```

**Problem: Tests not starting from web interface**

**Test API connectivity:**
```bash
curl -X POST http://localhost:3000/api/log-events \
  -H "Content-Type: application/json" \
  -d '{"type":"test","message":"connectivity check"}'
```

**Expected response:**
```json
{"success":true,"eventCount":1}
```

### File Permission Issues

**Problem: Cannot write to test_results/**
```bash
# Fix ownership
sudo chown -R $USER:$USER test_results/

# Set proper permissions
chmod 755 test_results/
chmod 755 test_results/logs/
```

**Verify fix:**
```bash
ls -la test_results/
# Should show your user as owner, not root
```

## 🔍 Verification Commands

### Test Your Setup

**1. Container Build:**
```bash
docker compose build
# Expected: All services build successfully
```

**2. Web Interface:**
```bash
docker compose up k8s-diagnostic-ui -d
curl -s http://localhost:3000 | grep "K8s Diagnostic"
# Expected: HTML with "K8s Diagnostic Dashboard" title
```

**3. Kubernetes Access:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone kubectl get nodes
# Expected: Your cluster nodes listed
```

**4. Test Execution:**
```bash
docker compose run --rm k8s-diagnostic-cli-standalone test --test-list pod-to-pod-same-node
# Expected: ✅ PASS result
```

**5. File Persistence:**
```bash
ls -la test_results/
# Expected: JSON and log files present
```

## 📚 Advanced Usage

### Custom Test Configurations

**Create custom test groups:**
```bash
# Run specific policy tests
docker compose run --rm k8s-diagnostic-cli-standalone test \
  --test-list "policy-deny,policy-allow,dns-resolution"
```

**Environment-specific testing:**
```bash
# Test with custom kubeconfig
docker compose run --rm \
  -v /path/to/custom/kubeconfig:/app/shared/.kube/config:ro \
  k8s-diagnostic-cli-standalone test --test-group l3-policies
```

### CI/CD Integration

**Example GitHub Actions:**
```yaml
- name: Run k8s-diagnostic tests
  run: |
    docker compose build
    docker compose up k8s-diagnostic-ui -d
    docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l3-policies
    docker compose down
```

**Example Jenkins Pipeline:**
```groovy
stage('K8s Diagnostics') {
    steps {
        sh 'docker compose run --rm k8s-diagnostic-cli-standalone test --test-group l4-policies'
        archiveArtifacts 'test_results/*.json'
    }
}
```

### Performance Optimization

**Faster rebuilds:**
```bash
# Build specific service only
docker compose build k8s-diagnostic-ui

# Use build cache
docker compose build --parallel
```

**Resource limits:**
```yaml
# Add to docker-compose.override.yml
services:
  k8s-diagnostic-ui:
    deploy:
      resources:
        limits:
          memory: 512m
        reservations:
          memory: 256m
```

## 🧹 Cleanup

### Stop Services
```bash
# Stop containers but keep them
docker compose stop

# Stop and remove containers
docker compose down

# Stop, remove containers, and clean networks
docker compose down --remove-orphans
```

### Clean Docker Resources
```bash
# Remove k8s-diagnostic images
docker compose down --rmi all

# Remove unused Docker resources
docker system prune -f

# Remove everything (careful!)
docker system prune -af --volumes
```

### Preserve Test Results
Your test results in `test_results/` are stored on your computer and will **not** be deleted when you remove containers. This is intentional - your diagnostic data persists across container lifecycles.

## 🎯 Summary

**For quick testing:** Use the web interface at http://localhost:3000
**For automation:** Use direct CLI commands with `docker compose run`
**For persistence:** All results save to your local `test_results/` directory
**For cleanup:** Use `docker compose down` when finished

The containerized setup provides the same functionality as local installation but with guaranteed dependency isolation and consistent behavior across different environments.

## 🆘 Support

If issues persist:
1. **Check logs:** `docker compose logs k8s-diagnostic-ui`
2. **Verify prerequisites:** Docker, Docker Compose, kubectl access
3. **Test basic connectivity:** `kubectl get nodes`
4. **Restart clean:** `docker compose down && docker compose up k8s-diagnostic-ui -d`

All commands in this guide have been tested and verified in real environments. Copy-paste them with confidence!
