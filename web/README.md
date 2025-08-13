# K8s Diagnostic Web Interface

A sophisticated Next.js web interface providing real-time monitoring, batch execution, and comprehensive visualization of Kubernetes diagnostic tests. This advanced platform features intelligent test insights, progressive status tracking, and seamless integration with the CLI tool.

## 🚀 **Advanced Features**

### **Batch Test Execution**
- **Simultaneous Testing**: Run multiple diagnostic tests concurrently with intelligent orchestration
- **Smart Selection**: Category-based test organization with individual test selection controls
- **Progress Tracking**: Real-time progress bars with atomic state management
- **Test Queuing**: Advanced queuing system with concurrency management

### **Progressive Status System**
- **Timer-Based Evolution**: Status messages and colors evolve over time
  - 🟢 **0-1 min**: "Running the test..." (Green)
  - 🟡 **1-2 min**: "Taking longer than usual..." (Amber)
  - 🔴 **2+ min**: "Still working... (X minutes)" (Red)
- **Auto-Updates**: 10-second refresh intervals with backend heartbeat integration
- **Custom Messages**: Backend can override with specific status updates

### **Rich Test Insights System**
- **33 Comprehensive Tests** across 4 categories with detailed explanations
- **Success/Failure Messaging**: Contextual insights for each test outcome
- **Real-World Impact**: Explains what each test validates and its implications
- **Troubleshooting Guidance**: Actionable hints and next steps for failures

### **Advanced Architecture**
- **SSE Streaming**: Real-time Server-Sent Events for live updates
- **Atomic State Management**: Race condition prevention with mutex-protected updates
- **Multi-API Design**: 6 specialized API endpoints for different functions
- **Auto-Build Integration**: Automatic binary compilation with source change detection

## 📊 **Test Categories & Coverage**

### **Networking Tests (7 tests)**
- Pod-to-pod communication (same node & cross-node)
- Service connectivity (ClusterIP, NodePort, LoadBalancer)
- DNS resolution and service discovery

### **L3 Policy Tests (11 tests)**
- CIDR-based ingress/egress controls
- Label and endpoint selectors
- DNS-based and node-based policies
- Entity and service references

### **L4 Policy Tests (10 tests)**
- TCP/UDP port controls and ranges
- ICMP type restrictions (IPv4 & IPv6)
- TLS/SNI inspection and filtering

### **L7 Policy Tests (5 tests)**
- HTTP method and path filtering
- HTTP header inspection
- DNS pattern matching and wildcards

## 🏗️ **Architecture Overview**

### **Data Flow**
1. **Frontend Selection**: User selects tests via category-based UI
2. **Batch API**: `/api/run-batch-tests` orchestrates multiple test execution
3. **CLI Integration**: Spawns Go CLI with SSE event streaming
4. **Real-time Updates**: Server-Sent Events stream to frontend components
5. **Rich Results**: Enhanced display with test insights and user messages

### **Core Components**

#### **Frontend Components**
- **`BatchTestRunner.jsx`**: Advanced batch execution with progress tracking
- **`DiagnosticQuestions.jsx`**: Category-based test selection interface
- **`TestRunner.jsx`**: Individual test execution and monitoring
- **`ResultsViewer.jsx`**: Comprehensive results visualization
- **`LogViewer.jsx`**: Real-time event display with structured formatting
- **`CleanupButton.jsx`**: Resource cleanup orchestration
- **`ProgressIndicator.jsx`**: Advanced progress visualization

#### **Backend APIs (6 Endpoints)**
- **`/api/run-batch-tests`**: Batch test execution with SSE streaming
- **`/api/run-test`**: Individual test execution
- **`/api/batch-test-status`**: Status monitoring and process management
- **`/api/stop-tests`**: Process termination and cleanup
- **`/api/cleanup-resources`**: Resource cleanup operations
- **`/api/log-events`**: Event storage and retrieval system

#### **Utility Systems**
- **`testInsights.js`**: 33 comprehensive test definitions with rich messaging
- **`cliCommands.js`**: Diagnostic questions mapped to CLI commands

## 📋 **Installation & Setup**

### **Prerequisites**
- Node.js 18+
- npm or yarn
- Go 1.21+ (for CLI compilation)
- Kubernetes cluster access
- kubectl configured

### **Quick Start**

```bash
# Navigate to web directory
cd web

# Install dependencies
npm install

# Development server
npm run dev

# Production build
npm run build
npm start
```

### **Environment Configuration**

Create `.env.local` for custom settings:

```bash
# Custom CLI path (default: ../k8s_diagnostic)
CLI_PATH=./path/to/k8s_diagnostic

# Custom results directory (default: ../test_results)
RESULTS_DIR=./custom/results/path

# Development mode debugging
DEBUG=true
```

## 🎯 **Usage Guide**

### **Batch Test Execution**

1. **Access Interface**: Navigate to [http://localhost:3000](http://localhost:3000)

2. **Select Tests**: 
   - Choose from 4 categories: networking, l3-policy, l4-policy, l7-policy
   - Use individual checkboxes or "Select All" for entire categories
   - View estimated execution times for planning

3. **Execute Tests**:
   - Click "🚀 Start Tests" to begin batch execution
   - Monitor real-time progress with category-colored progress bars
   - Observe progressive status evolution (Green → Amber → Red)

4. **Monitor Execution**:
   - **Live Terminal**: Clean phase-based updates (setup, cleanup, testing)
   - **Progressive Status**: Timer-based status messages with color evolution
   - **Overall Progress**: Atomic progress calculation with race condition protection

5. **Review Results**:
   - **Rich Success Messages**: Detailed explanations of what passed
   - **Comprehensive Failure Analysis**: Contextual troubleshooting guidance
   - **Results Summary**: Pass/fail counts with category breakdowns

### **Individual Test Execution**

1. **Diagnostic Questions**: Answer targeted questions about your connectivity issues
2. **CLI Command Preview**: See exact CLI commands before execution
3. **Real-time Monitoring**: Watch detailed step-by-step execution
4. **Rich Results**: Get comprehensive explanations and next steps

## 🔧 **API Reference**

### **POST /api/run-batch-tests**

Execute multiple diagnostic tests with real-time streaming.

**Request:**
```json
{
  "testList": ["cidr-ingress", "tcp-port-ingress", "dns-resolution"],
  "testId": "batch-123456789"
}
```

**Response:** Server-Sent Events stream with:
- `connected`: Initial connection established
- `build_start`/`build_complete`: Auto-build events
- `test_start`: Individual test initiation
- `test_complete`: Test completion with rich results
- `live_output`: Real-time CLI output
- `batch_complete`: All tests finished

### **POST /api/stop-tests**

Terminate running batch tests with proper cleanup.

**Request:**
```json
{
  "testId": "batch-123456789"
}
```

### **GET /api/log-events**

Retrieve stored events and user messages.

**Parameters:**
- `testId`: Test identifier for event filtering

## 🔄 **Advanced Event System**

### **SSE Event Types**

```javascript
// Test lifecycle events
{
  "type": "test_start",
  "testName": "cidr-ingress",
  "timestamp": "2024-01-15T10:30:00Z"
}

{
  "type": "test_complete", 
  "testName": "cidr-ingress",
  "success": true,
  "summary": "PASSED - no issues",
  "duration": "38.5",
  "userMessage": {
    "title": "✅ CIDR-based ingress policy working perfectly!",
    "description": "Network policies can restrict incoming traffic by IP ranges",
    "hints": ["You can implement IP-based segmentation"]
  }
}
```

### **Progressive Status Integration**

The system combines timer-based status evolution with backend heartbeats:

```javascript
// Frontend timer progression
"Running the test..." → "Taking longer than usual..." → "Still working... (3 minutes)"

// Backend can override with specific messages
"Infrastructure setup phase..." → "Policy enforcement validation..." → "Cleanup in progress..."
```

## 🎨 **UI Features & Design**

### **Category-Based Color Coding**
- 🔴 **Security/Policy**: Red accent for critical security tests
- 🩷 **Networking**: Pink accent for connectivity tests  
- 🟠 **L3 Policies**: Orange accent for network layer rules
- 🟣 **L4 Policies**: Indigo accent for transport layer controls
- 🟡 **L7 Policies**: Yellow accent for application layer filtering

### **Responsive Design**
- **Desktop**: Multi-column grid layout with detailed information
- **Mobile**: Single-column stack with touch-optimized controls
- **Real-time Updates**: Smooth animations and progress indicators

## 🛠️ **Development**

### **Adding New Tests**

1. **Update CLI Commands** (`utils/cliCommands.js`):
```javascript
{
  id: 'new-test',
  question: 'Is there a new networking issue?',
  cliCommand: './k8s_diagnostic test new-test --verbose',
  testType: 'new-test',
  category: 'networking',
  estimatedTime: '3-5 minutes'
}
```

2. **Add Test Insights** (`utils/testInsights.js`):
```javascript
"new-test": {
  category: "networking",
  success: {
    title: "✅ New test working perfectly!",
    details: [
      "📊 Specific functionality is operational",
      "🎯 Your configuration is correct"
    ]
  },
  failure: {
    title: "❌ New test failed",
    details: [
      "📊 Check specific configuration",
      "🔧 Review troubleshooting steps"
    ]
  }
}
```

### **Custom Status Messages**

Backend can send custom heartbeat messages:

```javascript
// In your test implementation
res.write(`data: ${JSON.stringify({
  type: 'custom_status',
  testName: 'your-test',
  message: 'Performing advanced validation...'
})}\n\n`);
```

## 📊 **Performance & Monitoring**

### **Performance Characteristics**
- **Memory Usage**: 50-150MB during active batch testing
- **CPU Impact**: Minimal overhead with efficient event processing
- **Network**: Persistent SSE connections with optimal batching
- **Concurrency**: Up to 3 simultaneous tests with intelligent queuing

### **Debug Mode**

Enable comprehensive debug logging:

```bash
DEBUG=true npm run dev
```

Provides detailed logs for:
- SSE event streaming and processing
- State transitions and atomic updates
- CLI process management and termination
- Progress calculation and validation

### **State Management**

Advanced atomic state management prevents race conditions:

```javascript
// Mutex-protected progress updates
// Event deduplication with collision prevention  
// State transition validation with history tracking
// Concurrent test execution with queue management
```

## 🔒 **Security & Best Practices**

### **Security Model**
- **Local Development**: Designed for local cluster diagnostics
- **No Authentication**: Not intended for production deployment  
- **Sandboxed Execution**: CLI processes run in controlled environment
- **Resource Limits**: Automatic cleanup and process termination

### **Best Practices**
- Run in isolated development environments
- Ensure cluster access is properly configured
- Monitor resource usage during large batch executions
- Use cleanup functions between test sessions

## 🚨 **Troubleshooting**

### **Common Issues**

1. **Tests Not Starting**:
   ```bash
   # Check binary compilation
   cd .. && go build -o k8s_diagnostic .
   
   # Verify cluster connectivity  
   kubectl cluster-info
   
   # Check permissions
   kubectl auth can-i '*' '*'
   ```

2. **SSE Connection Issues**:
   ```bash
   # Clear browser cache
   # Check console for connection errors
   # Verify port 3000 accessibility
   ```

3. **Progress Not Updating**:
   ```javascript
   // Check browser console for atomic state errors
   // Verify test state consistency in network tab
   // Monitor for race condition warnings
   ```

## 📈 **Metrics & Analytics**

### **Test Coverage Statistics**
- **Total Tests**: 33 comprehensive diagnostic tests
- **Category Breakdown**: 7 networking, 11 L3, 10 L4, 5 L7 tests  
- **Success Rate Tracking**: Per-category and overall success analytics
- **Execution Time Monitoring**: Performance trends and optimization insights

### **System Health**
- **Process Management**: Active process monitoring and cleanup
- **Memory Usage**: Automatic garbage collection and resource limits
- **Event Processing**: SSE throughput and processing efficiency

## 🤝 **Contributing**

### **Development Workflow**
1. Fork the repository and create feature branch
2. Test thoroughly with real cluster environments
3. Update test insights for new diagnostic tests
4. Ensure responsive design compatibility
5. Submit pull request with comprehensive description

### **Code Standards**
- **React**: Functional components with hooks
- **API Design**: RESTful endpoints with SSE streaming
- **State Management**: Atomic updates with race condition prevention
- **Error Handling**: Comprehensive error boundaries and fallbacks

## 📄 **License**

This project is part of the k8s_diagnostic tool suite. See the main repository for licensing terms.

---

**Built with ❤️ for Kubernetes diagnostics**
