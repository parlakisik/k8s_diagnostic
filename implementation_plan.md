# Implementation Plan

## Overview
Complete rebuild and redeployment of the k8s-diagnostic containers to resolve TestID synchronization issues and ensure fresh code deployment with enhanced CLI ↔ UI communication.

## Scope and Context
The current deployment has a critical TestID mismatch issue where the frontend generates one testId but the CLI container receives a different one, causing SSE event loss. The solution requires complete container refresh, code synchronization, and deployment validation to ensure proper bi-directional communication between UI and CLI containers.

## Types
No new type definitions required - existing TestExecutionRequest and TestExecutionResponse structures in cmd/serve.go are sufficient.

## Files
### Files to be cleaned up:
- Delete existing Kubernetes deployment: `k8s-diagnostic-ui` in namespace `k8s-diagnostic`
- Remove old Docker images: `daryakut453/k8s-diagnostic-ui:reliability-fix-20250820-131400` and `daryakut453/k8s-diagnostic-cli:reliability-fix-20250820-130354`

### Files to be modified:
- `k8s/deployment-ui.yaml` - Update image tags to new timestamp-based tags
- `web/pages/api/run-batch-tests.js` - Fix TestID synchronization in HTTP API calls
- `cmd/serve.go` - Enhance logging and TestID forwarding validation

### Configuration files to update:
- `k8s/deployment-ui.yaml` - New image tags and enhanced health checks
- Docker build scripts for fresh image creation

## Functions
### New functions:
- `validateTestIdSynchronization(testId, cliTestId)` - Validate TestID matching between frontend and CLI
- `logTestIdFlow(stage, testId, source)` - Enhanced logging for TestID flow tracking

### Modified functions:
- `handleTestExecution()` in cmd/serve.go - Add TestID validation and forwarding logs
- `runBatchTests()` in web/pages/api/run-batch-tests.js - Fix TestID parameter passing
- `forwardEventToUI()` in cmd/serve.go - Enhanced error handling and retry logic

## Classes
### Modified classes:
- `EventContainer` in web/pages/api/log-events.js - Enhanced TestID validation and cleanup

## Dependencies
No new dependencies required - using existing Docker, Kubernetes, and Node.js stack.

## Testing
### Test validation strategy:
- Container startup validation (health checks)
- TestID synchronization verification
- CLI ↔ UI communication end-to-end testing
- SSE event streaming validation
- Container restart and reconnection testing

## Implementation Order
1. **Cleanup Phase**: Delete existing pods and old Docker images
2. **Code Preparation**: Ensure latest code is ready for containerization
3. **Image Rebuild**: Build fresh Docker images with new timestamp tags
4. **Deployment**: Deploy new images to Kubernetes
5. **Health Validation**: Verify container startup and communication
6. **TestID Sync Fix**: Implement and test TestID synchronization
7. **End-to-End Testing**: Validate complete CLI ↔ UI communication flow
8. **Connection Testing**: Test container restart scenarios and reconnection
