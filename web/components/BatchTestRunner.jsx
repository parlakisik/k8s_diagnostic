import { useState, useEffect, useRef } from 'react';
import { getTestInsights } from '../utils/testInsights';

// Global bounce animation styles
const bounceStyles = `
  @keyframes statusBounce {
    0%, 20%, 53%, 80%, 100% { 
      transform: translate3d(0,0,0); 
    }
    40%, 43% { 
      transform: translate3d(0,-15px,0); 
    }
    70% { 
      transform: translate3d(0,-8px,0); 
    }
    90% { 
      transform: translate3d(0,-3px,0); 
    }
  }
  .status-bounce {
    animation: statusBounce 1s infinite !important;
    display: inline-block !important;
  }
`;

const getTestColorClass = (testName) => {
  // Defensive type checking - handle both strings and objects
  const name = typeof testName === 'string' ? testName : 
               (testName?.name || testName?.testName || String(testName || ''));
  
  if (!name || typeof name !== 'string') {
    return 'test-card-infrastructure'; // fallback color
  }
  
  // Map test names to color categories with improved logic
  
  // Security/Policy Tests (Red - Results color for security focus)
  if (name.includes('deny-all') || name.includes('reject') || name.includes('security') || name.includes('allow-all')) {
    return 'test-card-results';
  }
  
  // Networking Tests (Pink)
  else if (name.includes('pod-to-pod') || name.includes('service-clusterip') || 
           name.includes('service-nodeport') || name.includes('service-loadbalancer') || 
           name.includes('service-') || name === 'dns-resolution') {
    return 'test-card-networking';
  }
  
  // L3 Policy Tests (Orange)
  else if (name.includes('cidr') || name.includes('endpoints') || name.includes('entities') || 
           name.includes('dns-based') || name.includes('node-') || 
           name.includes('kubernetes-service') || name.includes('namespace') || 
           name.includes('label-based') || name.includes('same-namespace') || 
           name.includes('deny-namespace') || name.includes('same-label') || 
           name.includes('deny-label')) {
    return 'test-card-l3';
  }
  
  // L4 Policy Tests (Purple/Blue)
  else if (name.includes('tcp-port') || name.includes('port-') || name.includes('icmp') || 
           name.includes('sni') || name.includes('tls') || name.includes('udp') ||
           name.includes('multiple-port') || name.includes('port-range')) {
    return 'test-card-l4';
  }
  
  // L7 Policy Tests (Yellow)
  else if (name.includes('http') || name.includes('dns-match') || name.includes('grpc') ||
           name.includes('kafka') || name.includes('path-') || name.includes('method-') ||
           name.includes('header')) {
    return 'test-card-l7';
  }
  
  // DNS Service Tests (Purple - specific DNS color)
  else if (name.includes('dns') && !name.includes('dns-resolution')) {
    return 'test-card-dns';
  }
  
  // Infrastructure Tests (Teal - fallback)
  else {
    return 'test-card-infrastructure';
  }
};

const getTestIcon = (testName) => {
  // Defensive type checking - handle both strings and objects
  const name = typeof testName === 'string' ? testName : 
               (testName?.name || testName?.testName || String(testName || ''));
  
  if (!name || typeof name !== 'string') {
    return '⚙️'; // fallback icon
  }
  
  if (name.includes('pod-to-pod')) return '🔗';
  if (name.includes('service-')) return '🌐';
  if (name.includes('dns')) return '🔍';
  if (name.includes('cidr')) return '🛡️';
  if (name.includes('endpoints') || name.includes('entities')) return '🏷️';
  if (name.includes('node-')) return '📡';
  if (name.includes('tcp') || name.includes('port')) return '🔌';
  if (name.includes('icmp')) return '📡';
  if (name.includes('sni')) return '🔒';
  if (name.includes('http')) return '🌍';
  if (name.includes('allow-all')) return '🔓';
  if (name.includes('deny-all')) return '🔒';
  return '⚙️';
};

const getSkeletonColor = (testName) => {
  // Defensive type checking - handle both strings and objects
  const name = typeof testName === 'string' ? testName : 
               (testName?.name || testName?.testName || String(testName || ''));
  
  if (!name || typeof name !== 'string') {
    return 'bg-teal-400'; // fallback color
  }
  
  // Return category-specific colors for skeleton bars - matches getTestColorClass logic
  
  // Security/Policy Tests (Red)
  if (name.includes('deny-all') || name.includes('reject') || name.includes('security') || name.includes('allow-all')) {
    return 'bg-red-400';
  }
  
  // Networking Tests (Pink)
  else if (name.includes('pod-to-pod') || name.includes('service-clusterip') || 
           name.includes('service-nodeport') || name.includes('service-loadbalancer') || 
           name.includes('service-') || name === 'dns-resolution') {
    return 'bg-pink-400';
  }
  
  // L3 Policy Tests (Orange)
  else if (name.includes('cidr') || name.includes('endpoints') || name.includes('entities') || 
           name.includes('dns-based') || name.includes('node-') || 
           name.includes('kubernetes-service') || name.includes('namespace') || 
           name.includes('label-based') || name.includes('same-namespace') || 
           name.includes('deny-namespace') || name.includes('same-label') || 
           name.includes('deny-label')) {
    return 'bg-orange-400';
  }
  
  // L4 Policy Tests (Purple/Blue)
  else if (name.includes('tcp-port') || name.includes('port-') || name.includes('icmp') || 
           name.includes('sni') || name.includes('tls') || name.includes('udp') ||
           name.includes('multiple-port') || name.includes('port-range')) {
    return 'bg-indigo-400';
  }
  
  // L7 Policy Tests (Yellow)
  else if (name.includes('http') || name.includes('dns-match') || name.includes('grpc') ||
           name.includes('kafka') || name.includes('path-') || name.includes('method-') ||
           name.includes('header')) {
    return 'bg-yellow-400';
  }
  
  // DNS Service Tests (Purple)
  else if (name.includes('dns') && !name.includes('dns-resolution')) {
    return 'bg-purple-400';
  }
  
  // Infrastructure Tests (Teal - fallback)
  else {
    return 'bg-teal-400';
  }
};

const getTestCategory = (testName) => {
  // Defensive type checking - handle both strings and objects
  const name = typeof testName === 'string' ? testName : 
               (testName?.name || testName?.testName || String(testName || ''));
  
  if (!name || typeof name !== 'string') {
    return 'Infrastructure Test'; // fallback category
  }
  
  // Map test names to their categories
  if (name.includes('pod-to-pod')) {
    return 'Cross-Node Connectivity';
  } else if (name.includes('service-clusterip')) {
    return 'ClusterIP Service';
  } else if (name.includes('service-nodeport')) {
    return 'NodePort Service';
  } else if (name.includes('service-loadbalancer')) {
    return 'LoadBalancer Service';
  } else if (name.includes('service-')) {
    return 'Service Connectivity';
  } else if (name === 'dns-resolution') {
    return 'DNS Resolution';
  } else if (name.includes('cidr')) {
    return 'L3 CIDR Policies';
  } else if (name.includes('endpoints') || name.includes('entities')) {
    return 'L3 Label Policies';
  } else if (name.includes('dns-based') || name.includes('node-')) {
    return 'L3 DNS & Node Policies';
  } else if (name.includes('kubernetes-service')) {
    return 'L3 Service Policies';
  } else if (name.includes('tcp-port') || name.includes('port-')) {
    return 'L4 Port Policies';
  } else if (name.includes('icmp')) {
    return 'L4 ICMP Policies';
  } else if (name.includes('sni')) {
    return 'L4 TLS/SNI Policies';
  } else if (name.includes('http')) {
    return 'L7 HTTP Policies';
  } else if (name.includes('dns-match')) {
    return 'L7 DNS Policies';
  } else if (name.includes('deny-all') || name.includes('allow-all')) {
    return 'Security Policies';
  } else {
    return 'Infrastructure Test';
  }
};

export default function BatchTestRunner({ testQueue, onBack, onTestComplete }) {
  
  const [isRunning, setIsRunning] = useState(false);
  const [hasStarted, setHasStarted] = useState(false);
  const [testResults, setTestResults] = useState({});
  const [testOutputs, setTestOutputs] = useState({});
  const [currentlyRunning, setCurrentlyRunning] = useState(new Set());
  const [error, setError] = useState(null);
  const [overallProgress, setOverallProgress] = useState(0);
  const [testId, setTestId] = useState(null);
  const [isLoading, setIsLoading] = useState(false);
  const [currentPhase, setCurrentPhase] = useState('');
  const [liveOutput, setLiveOutput] = useState([]);
  const [filteredOutput, setFilteredOutput] = useState([]);
  // Initialize with all tests selected, extracting string names from potential objects
  const [selectedTests, setSelectedTests] = useState(new Set(
    testQueue.map(testName => 
      typeof testName === 'string' ? testName : 
      (testName?.name || testName?.testName || String(testName || 'unknown-test'))
    )
  ));
  const [backendError, setBackendError] = useState(null); // Track backend process errors
  const [connectionLost, setConnectionLost] = useState(false); // Track SSE connection status
  const [showCliCommands, setShowCliCommands] = useState(false); // CLI commands visibility toggle
  
  // 🕒 Progressive Status System - Track test timing for user feedback
  const [testStartTimes, setTestStartTimes] = useState(new Map());
  const [testStatusMessages, setTestStatusMessages] = useState(new Map()); // Backend heartbeat messages
  const [statusUpdateCounter, setStatusUpdateCounter] = useState(0); // Force re-renders for status updates

  // 🕒 Progressive status message logic with 1-minute increments
  const getRunningStatusMessage = (testName) => {
    const startTime = testStartTimes.get(testName);
    const customMessage = testStatusMessages.get(testName); // Backend heartbeat override
    
    if (customMessage) return customMessage;
    if (!startTime) return "Running the test...";
    
    const elapsed = Math.floor((Date.now() - startTime) / 1000);
    const minutes = Math.floor(elapsed / 60);
    
    if (minutes === 0) return "Running the test...";
    if (minutes === 1) return "Taking longer than usual...";
    if (minutes === 2) return "Still working, this is normal...";
    if (minutes >= 3) return `Still working... (${minutes} minutes)`;
    
    return "Running the test...";
  };

  // 🎨 Progressive color logic based on timing
  const getStatusColor = (testName) => {
    const startTime = testStartTimes.get(testName);
    if (!startTime) return '#10b981'; // Green
    
    const minutes = Math.floor((Date.now() - startTime) / (60 * 1000));
    if (minutes === 0) return '#10b981'; // Green
    if (minutes === 1) return '#f59e0b'; // Amber  
    return '#ef4444'; // Red for 2+ minutes
  };

  // Inject bounce animation styles
  useEffect(() => {
    const styleId = 'status-bounce-animation';
    if (!document.getElementById(styleId)) {
      const style = document.createElement('style');
      style.id = styleId;
      style.textContent = bounceStyles;
      document.head.appendChild(style);
    }
    return () => {
      const style = document.getElementById(styleId);
      if (style) {
        style.remove();
      }
    };
  }, []);

  // Stage-based message tracker to prevent duplicates using ref
  const lastPhaseMessageRef = useRef('');

  // Transform verbose output into clean stage-based updates
  const transformLine = (line) => {
    const trimmed = line.trim();
    
    // Infrastructure phase grouping
    if (trimmed.includes('Setting up test environment') || 
        trimmed.includes('Collecting cluster infrastructure information')) {
      const newMessage = '🔍 Infrastructure setup phase...';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return ''; // Skip duplicates
    }
    
    if (trimmed.includes('Infrastructure collection completed')) {
      const newMessage = '✅ Infrastructure ready';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    // CRITICAL: Handle direct backend cleanup messages
    if (trimmed.includes('🧹 Pre-test cleanup phase')) {
      const newMessage = '🧹 Pre-test cleanup phase...';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    if (trimmed.includes('✅ Pre-test cleanup completed')) {
      const newMessage = '✅ Pre-test cleanup completed';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    // Legacy cleanup phase grouping (fallback)
    if (trimmed.includes('CLEANUP PHASE') || trimmed.includes('Removing network policies') ||
        trimmed.match(/├──.*Cilium Policies:/)) {
      const newMessage = '🧹 Pre-test cleanup phase...';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    if (trimmed.includes('universal_cleanup completed') || 
        trimmed.includes('Cleanup completed successfully')) {
      
      // CRITICAL FIX: Context-aware cleanup completion mapping
      // Determine if we're in pre-test or post-test cleanup based on actual test execution state
      const hasTestsStarted = currentlyRunning.size > 0 || 
                             Object.values(testResults).some(r => ['success', 'failed', 'running'].includes(r.status));
      
      const newMessage = hasTestsStarted ? 
        '✅ Post Test Cleanup COMPLETED' :     // After tests have actually run
        '✅ Pre-test cleanup completed';       // Before tests have started
        
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    // Test execution phase
    if (trimmed.includes('Running test:') || trimmed.includes('Starting networking tests')) {
      const newMessage = '🧪 Test execution phase...';
      if (lastPhaseMessageRef.current !== newMessage) {
        lastPhaseMessageRef.current = newMessage;
        return newMessage;
      }
      return '';
    }
    
    return ''; // Filter out everything else
  };

  // Ultra-aggressive filtering for stage-based updates only
  const shouldDisplayLine = (line) => {
    const trimmed = line.trim();
    
    // Block all debug and verbose messages
    if (trimmed.match(/\[DEBUG/)) return false;
    if (trimmed.match(/Command stdout|packets transmitted|rtt min/)) return false;
    if (trimmed.match(/✅ Done \(\d+\.\d+s\)$/)) return false;
    if (trimmed.match(/✓.*Kubernetes version:|✓.*Nodes:|✓.*CNI:/)) return false;
    if (trimmed.match(/├──|└──|│/)) return false;
    
    // Block UI status messages that should not appear as content
    if (trimmed.includes('LIVE MONITORING')) return false;
    if (trimmed.includes('Connected • Lines:')) return false;
    if (trimmed.includes('Essential messages only')) return false;
    
    // Only allow high-level phase indicators
    if (trimmed.includes('Setting up test environment')) return true;
    if (trimmed.includes('Collecting cluster infrastructure information')) return true;
    if (trimmed.includes('Infrastructure collection completed')) return true;
    if (trimmed.includes('CLEANUP PHASE')) return true;
    if (trimmed.includes('🧹 Pre-test cleanup phase')) return true;  // CRITICAL: Allow backend cleanup messages
    if (trimmed.includes('✅ Pre-test cleanup completed')) return true;  // CRITICAL: Allow cleanup completion
    if (trimmed.includes('universal_cleanup completed')) return true;
    if (trimmed.includes('Running test:')) return true;
    if (trimmed.includes('Starting networking tests')) return true;
    
    // Allow cleanup status messages
    if (trimmed.includes('In between tests cleanup') && trimmed.includes('COMPLETED')) return true;
    if (trimmed.includes('Post Test Cleanup') && trimmed.includes('COMPLETED')) return true;
    
    // Reject everything else
    return false;
  };


  const runBatchTests = async () => {
    if (selectedTests.size === 0) {
      setError('No tests selected to run');
      return;
    }

    // CRITICAL FIX: Preserve original testQueue order for execution
    // Filter testQueue to only include selected tests, maintaining original order
    const selectedTestsList = testQueue
      .map(testName => typeof testName === 'string' ? testName : 
           (testName?.name || testName?.testName || String(testName || 'unknown-test')))
      .filter(testName => selectedTests.has(testName));
    
    console.log('[BatchTestRunner] 🚀 Starting batch execution with test queue:', selectedTestsList);
    console.log('[BatchTestRunner] 📋 Original testQueue order preserved - tests will run in selection order');
    const newTestId = Date.now().toString();
    setTestId(newTestId);
    setIsRunning(true);
    setError(null);
    setTestResults({});
    setTestOutputs({});
    // Clear all state for fresh start
    setCurrentlyRunning(new Set());
    setOverallProgress(0);
    setLiveOutput([]);
    setFilteredOutput([]);
    setBackendError(null);
    setConnectionLost(false);
    
    // 🧹 CRITICAL: Clear all progressive status state for fresh run
    setTestStartTimes(new Map());
    setTestStatusMessages(new Map());
    setStatusUpdateCounter(0);
    
    // 🧹 CRITICAL: Clear processed events and timing data
    processedEvents.current.clear();
    lastTestActivity.current.clear();
    eventTimestamps.current.clear();
    testStateHistory.current.clear();
    stateTransitionLock.current.clear();
    
    // CRITICAL: Reset phase message tracker for fresh run
    lastPhaseMessageRef.current = '';

    // 🧹 FRONTEND CLEANUP: Clear server-side event storage before starting
    try {
      await fetch('/api/log-events', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' }
      });
    } catch (cleanupError) {
      // Silent cleanup error handling
    }

    // Initialize test states for selected tests only
    const initialResults = {};
    const initialOutputs = {};
    selectedTestsList.forEach(test => {
      initialResults[test] = { status: 'queued', message: 'Waiting to start...' };
      initialOutputs[test] = [];
    });
    setTestResults(initialResults);
    setTestOutputs(initialOutputs);

    try {
      const response = await fetch('/api/run-batch-tests', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          testList: selectedTestsList,
          testId: newTestId
        })
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(`HTTP error! status: ${response.status} - ${errorData.message || 'Unknown error'}`);
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = ''; // Buffer to handle partial SSE events across chunks

      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }

        // Decode chunk and add to buffer
        const chunk = decoder.decode(value, { stream: true });
        buffer += chunk;

        // Process complete lines from buffer
        const lines = buffer.split('\n');
        
        // Keep the last potentially incomplete line in the buffer
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const eventData = JSON.parse(line.substring(6));
              handleTestEvent(eventData);
            } catch (parseError) {
              // Silent parse error handling
            }
          }
        }
      }

    } catch (err) {
      setError(`Failed to execute batch tests: ${err.message}`);
      setIsRunning(false);
      setIsLoading(false); // CRITICAL: Reset loading state on error
      stopStatusPolling();
    }
  };

  // No status polling needed - using direct streaming only
  const stopStatusPolling = () => {
    // No-op function for compatibility
  };

  const stopAllTests = async () => {
    // 🎯 IMMEDIATE: Hide modal instantly for immediate user feedback
    setIsRunning(false);
    setCurrentlyRunning(new Set());
    setIsLoading(false);
    
    try {
      const response = await fetch('/api/stop-tests', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          testId: testId
        })
      });

      if (response.ok) {
        // 🛡️ FIXED: Use atomic state updates for termination
        setTestResults(prev => {
          const updated = { ...prev };
          Object.keys(updated).forEach(testName => {
            // Only terminate tests that are not already completed
            const currentStatus = updated[testName]?.status;
            if (currentStatus === 'running' || currentStatus === 'queued' || currentStatus === 'loading') {
              // Use atomic update for consistency
              updateTestStateAtomic(testName, 'terminated', '🛑 Stopped by user');
            }
          });
          return updated;
        });
        
        setOverallProgress(100);
        stopStatusPolling();
      } else {
        setError('Failed to stop tests - they may continue running');
        // 🛡️ FIXED: Ensure modal stays hidden even if API fails
        setIsRunning(false);
      }
    } catch (err) {
      setError(`Failed to stop tests: ${err.message}`);
      // 🛡️ FIXED: Ensure modal stays hidden even on exceptions
      setIsRunning(false);
    }
    
    // 🛡️ FINAL: Guarantee modal is hidden regardless of any issues above
    setIsRunning(false);
    stopStatusPolling();
  };

  // 🛡️ CRITICAL FIX: Mutex-protected progress calculation with state locking
  const progressMutex = useRef(false);
  const lastValidProgress = useRef(0);
  const progressValidationHistory = useRef([]);
  
  const calculateProgress = (testResults, totalTests) => {
    // 🔒 MUTEX PROTECTION: Prevent concurrent progress calculations
    if (progressMutex.current) {
      return lastValidProgress.current;
    }
    
    progressMutex.current = true;
    
    try {
      if (totalTests === 0) {
        lastValidProgress.current = 0;
        return 0;
      }
      
      // 🛡️ ENHANCED: Comprehensive state validation with corruption detection
      const validResults = Object.values(testResults).filter(r => 
        r && typeof r.status === 'string' && r.status !== undefined
      );
      
      // 🛡️ CRITICAL FIX: Count states atomically to prevent race conditions
      const statusCounts = {
        success: 0,
        failed: 0,
        terminated: 0,
        running: 0,
        queued: 0,
        loading: 0
      };
      
      validResults.forEach(r => {
        if (statusCounts.hasOwnProperty(r.status)) {
          statusCounts[r.status]++;
        }
      });
      
      const totalCompleted = statusCounts.success + statusCounts.failed + statusCounts.terminated;
      
      // 🛡️ CRITICAL VALIDATION: Detect impossible state combinations
      if (totalCompleted > totalTests) {
        return lastValidProgress.current; // Return last known good progress
      }
      
      const progressValue = Math.min(100, Math.max(0, (totalCompleted / totalTests) * 100));
      const rounded = Math.round(progressValue);
      
      // 🛡️ CRITICAL FIX: Atomic backward movement prevention with state validation
      const currentProgress = lastValidProgress.current;
      
      // Record progress validation attempt
      progressValidationHistory.current.push({
        timestamp: Date.now(),
        attempted: rounded,
        current: currentProgress,
        totalCompleted,
        totalTests,
        statusCounts: {...statusCounts}
      });
      
      // Keep only last 20 validation attempts
      if (progressValidationHistory.current.length > 20) {
        progressValidationHistory.current = progressValidationHistory.current.slice(-20);
      }
      
      // 🛡️ ENHANCED: Strict backward movement validation
      const isBackwardMovement = rounded < currentProgress;
      const isResetScenario = currentProgress === 100 && totalCompleted < totalTests;
      const isValidProgression = rounded >= currentProgress || isResetScenario;
      
      if (isBackwardMovement && !isResetScenario) {
        return currentProgress;
      }
      
      // 🛡️ CRITICAL: Update last valid progress atomically
      lastValidProgress.current = rounded;
      
      // Log significant changes for monitoring (removed)
      
      return rounded;
      
    } finally {
      // 🔒 MUTEX RELEASE: Always release mutex
      progressMutex.current = false;
    }
  };

  // 🛡️ FIXED: Enhanced completion handler with state validation (LEGACY - kept for compatibility)
  const handleTestCompletion = (testName, completionData) => {
    console.log('[BatchTestRunner] � LEGACY completion handler called for:', testName, 'Success:', completionData.success);
    console.log('[BatchTestRunner] ℹ️  Note: This should now use updateTestStateAtomic for better state management');
    
    // Use the new atomic state update method
    const newStatus = completionData.success ? 'success' : 'failed';
    const success = updateTestStateAtomic(testName, newStatus, completionData.summary, {
      duration: completionData.duration,
      command: completionData.command,
      userMessage: completionData.userMessage
    });
    
    if (success) {
      // Remove from currently running set
      setCurrentlyRunning(prev => {
        const newSet = new Set(prev);
        newSet.delete(testName);
        console.log('[BatchTestRunner] 📊 Removed from currentlyRunning (legacy):', testName, 'Remaining:', Array.from(newSet));
        return newSet;
      });
      
      // Update progress
      setTimeout(() => {
        setTestResults(currentResults => {
          const newProgress = calculateProgress(currentResults, selectedTests.size);
          setOverallProgress(newProgress);
          return currentResults;
        });
      }, 50);
    }
  };

  // 🛡️ ENHANCED: Advanced event deduplication with sequence numbers and collision prevention
  const processedEvents = useRef(new Set());
  const eventSequence = useRef(0);
  const lastTestActivity = useRef(new Map()); // testName -> timestamp of last activity
  const stateTransitionLock = useRef(new Map()); // testName -> Promise (prevents concurrent updates)
  const testStateHistory = useRef(new Map()); // testName -> array of state transitions for debugging
  const eventTimestamps = useRef(new Map()); // testName -> last event timestamp to prevent out-of-order processing

  // 🔒 ENHANCED: State transition validation with stale result detection
  const isValidStateTransition = (testName, currentStatus, newStatus) => {
    // Define valid state transitions with stricter rules
    const validTransitions = {
      'ready': ['queued', 'loading'],
      'queued': ['loading', 'running', 'terminated'],
      'loading': ['running', 'queued', 'terminated'],
      'running': ['success', 'failed', 'terminated'],
      'success': [], // Final state - no transitions allowed
      'failed': [], // Final state - no transitions allowed  
      'terminated': [] // Final state - no transitions allowed to prevent stale results
    };

    // CRITICAL FIX: Block ALL backend results after user termination (tests are sequential - later tests never ran)
    if (currentStatus === 'terminated' && ['success', 'failed'].includes(newStatus)) {
      console.log(`[BatchTestRunner] 🛑 STALE RESULT BLOCKED: Ignoring backend ${newStatus} for ${testName} - tests are sequential, this test never ran after user termination`);
      return false;
    }

    // Special case: prevent overwriting completed states (except termination override above)
    if (['success', 'failed'].includes(currentStatus)) {
      return false;
    }

    const allowed = validTransitions[currentStatus] || [];
    const isValid = allowed.includes(newStatus);
    
    if (!isValid) {
      // Record invalid transition attempt for debugging (no console spam)
      const history = testStateHistory.current.get(testName) || [];
      history.push({
        timestamp: Date.now(),
        attempted: `${currentStatus} → ${newStatus}`,
        blocked: true,
        reason: 'Invalid transition'
      });
      testStateHistory.current.set(testName, history.slice(-15)); // Keep last 15 transitions
    }
    
    return isValid;
  };

  // 🔒 ENHANCED: True atomic state updates with mutex-style locking and queuing
  const stateUpdateQueue = useRef(new Map()); // testName -> array of queued updates
  const activeMutex = useRef(new Map()); // testName -> boolean (active mutex)
  
  const updateTestStateAtomic = async (testName, newStatus, newMessage, additionalData = {}) => {
    const updateId = `${testName}_${newStatus}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    
    // Create update request
    const updateRequest = {
      id: updateId,
      testName,
      newStatus, 
      newMessage,
      additionalData,
      timestamp: Date.now(),
      resolve: null,
      reject: null
    };
    
    // Wrap in promise for async handling
    const updatePromise = new Promise((resolve, reject) => {
      updateRequest.resolve = resolve;
      updateRequest.reject = reject;
    });
    
    // Add to queue for this test
    if (!stateUpdateQueue.current.has(testName)) {
      stateUpdateQueue.current.set(testName, []);
    }
    stateUpdateQueue.current.get(testName).push(updateRequest);
    
    // Process queue if no active mutex
    processStateUpdateQueue(testName);
    
    return updatePromise;
  };
  
  // 🔒 ENHANCED: Queue processor with true mutual exclusion
  const processStateUpdateQueue = async (testName) => {
    // Check if mutex is already active
    if (activeMutex.current.get(testName)) {
      return; // Queue will be processed by active mutex
    }
    
    // Acquire mutex
    activeMutex.current.set(testName, true);
    
    try {
      const queue = stateUpdateQueue.current.get(testName) || [];
      
      while (queue.length > 0) {
        const updateRequest = queue.shift(); // Get next update FIFO
        
        try {
          const success = await processStateUpdate(updateRequest);
          updateRequest.resolve(success);
        } catch (error) {
          console.error(`[BatchTestRunner] ❌ Queue processing error for ${updateRequest.id}:`, error);
          updateRequest.reject(error);
        }
        
        // Small delay between updates to prevent overwhelming React
        await new Promise(resolve => setTimeout(resolve, 10));
      }
    } finally {
      // Release mutex
      activeMutex.current.delete(testName);
      
      // Check if more updates were added while processing
      const queue = stateUpdateQueue.current.get(testName) || [];
      if (queue.length > 0) {
        // Recursively process remaining updates
        setTimeout(() => processStateUpdateQueue(testName), 5);
      }
    }
  };
  
  // 🔒 ENHANCED: Core state update with comprehensive validation
  const processStateUpdate = async (updateRequest) => {
    const { testName, newStatus, newMessage, additionalData, timestamp, id } = updateRequest;
    
    // Validate timestamp ordering
    const lastEventTime = eventTimestamps.current.get(testName) || 0;
    if (timestamp < lastEventTime) {
      console.warn(`[BatchTestRunner] ⚠️ Out-of-order update blocked: ${id} (${timestamp} < ${lastEventTime})`);
      return false;
    }
    
    let success = false;
    let transitionBlocked = false;
    
    // Atomic state update using React's functional setState
    await new Promise((resolve) => {
      setTestResults(prev => {
        const currentTest = prev[testName];
        const currentStatus = currentTest?.status || 'ready';
        
        // Validate state transition
        if (!isValidStateTransition(testName, currentStatus, newStatus)) {
          transitionBlocked = true;
          resolve();
          return prev; // Block invalid transition
        }
        
        // Update event timestamp
        eventTimestamps.current.set(testName, timestamp);
        
        // Record successful transition in history
        const history = testStateHistory.current.get(testName) || [];
        history.push({
          timestamp: timestamp,
          transition: `${currentStatus} → ${newStatus}`,
          message: newMessage,
          blocked: false,
          updateId: id,
          queuePosition: stateUpdateQueue.current.get(testName)?.length || 0
        });
        testStateHistory.current.set(testName, history.slice(-20)); // Keep more history
        
        success = true;
        
        const newState = {
          ...prev,
          [testName]: { 
            status: newStatus,
            message: newMessage,
            ...additionalData,
            lastUpdated: timestamp,
            updateId: id,
            transitionHistory: history.length
          }
        };
        
        resolve();
        return newState;
      });
    });
    
    if (transitionBlocked) {
      return false;
    }
    
    return success;
  };

  const handleTestEvent = async (eventData) => {
    // CRITICAL FIX: Ensure testName is always converted to string early to prevent object propagation
    const rawTestName = eventData.testName || eventData.data?.testName;
    const testName = typeof rawTestName === 'string' ? rawTestName : 
                     (rawTestName?.name || rawTestName?.testName || String(rawTestName || 'unknown-test'));
    const eventTimestamp = eventData.timestamp || Date.now();
    
    // 🛡️ ENHANCED: Advanced event deduplication with collision prevention
    eventSequence.current++;
    const eventKey = `${eventData.type}_${testName}_${eventSequence.current}_${eventTimestamp}_${Math.random().toString(36).substr(2, 9)}`;
    
    if (processedEvents.current.has(eventKey)) {
      console.log(`[BatchTestRunner] 🔄 Duplicate event detected and blocked:`, eventKey);
      return;
    }
    
    // Check for rapid duplicate events with same type and test
    const duplicateCheckKey = `${eventData.type}_${testName}`;
    const lastEventTime = lastTestActivity.current.get(duplicateCheckKey) || 0;
    if (eventTimestamp - lastEventTime < 50 && eventData.type !== 'live_output') { // 50ms debounce except for output
      console.log(`[BatchTestRunner] 🔄 Rapid duplicate event blocked: ${duplicateCheckKey}`);
      return;
    }
    
    processedEvents.current.add(eventKey);
    lastTestActivity.current.set(duplicateCheckKey, eventTimestamp);
    
  // Clean up old processed events (keep only last 200 for better collision prevention)
  if (processedEvents.current.size > 200) {
    const eventsArray = Array.from(processedEvents.current);
    processedEvents.current.clear();
    // Keep more recent events for better deduplication
    eventsArray.slice(-100).forEach(key => processedEvents.current.add(key));
  }
    
    // Only log critical events
    if (eventData.type === 'test_complete' || eventData.type === 'batch_complete') {
      console.log(`[BatchTestRunner] 🎯 ${eventData.type.toUpperCase()}:`, testName || 'batch', eventData);
    }
    
    switch (eventData.type) {
      case 'build_start':
        setError(null);
        break;

      case 'build_output':
        break;

      case 'build_complete':
        break;

      case 'test_start':
        if (testName) {
          // 🕒 Track test start time for progressive status system
          setTestStartTimes(prev => {
            const newMap = new Map(prev);
            newMap.set(testName, Date.now());
            return newMap;
          });
          
          // 🛡️ ENHANCED: Use async atomic state update with validation
          const success = await updateTestStateAtomic(testName, 'running', 'Test in progress...');
          
          if (success) {
            // 🛡️ FIXED: Simplified currentlyRunning management (atomic updates handle race conditions)
            setCurrentlyRunning(prev => {
              const newSet = new Set(prev);
              newSet.add(testName);
              
              // Enhanced concurrency management (keep this as it's useful)
              const maxConcurrent = 3;
              if (newSet.size > maxConcurrent) {
                console.warn(`[BatchTestRunner] ⚠️ Concurrent limit exceeded (${newSet.size}/${maxConcurrent}), managing queue`);
                
                // If over limit, remove oldest tests (simple FIFO)
                while (newSet.size > maxConcurrent) {
                  const firstTest = newSet.values().next().value;
                  newSet.delete(firstTest);
                  console.warn(`[BatchTestRunner] 🚫 Removed oldest test from running set: ${firstTest}`);
                }
              }
              
              console.log('[BatchTestRunner] 📝 Updated currentlyRunning:', Array.from(newSet));
              return newSet;
            });
            
            // Update activity timestamp with event timestamp
            lastTestActivity.current.set(testName, eventTimestamp);
          }
        }
        break;

      case 'test_output':
        if (testName && eventData.output) {
          // Update activity timestamp for test output
          lastTestActivity.current.set(testName, Date.now());
          
          setTestOutputs(prev => {
            const currentOutputs = prev[testName] || [];
            const newOutputs = [...currentOutputs, eventData.output];
            return {
              ...prev,
              [testName]: newOutputs
            };
          });
        }
        break;

      case 'test_complete':
        if (testName) {
          // Update activity timestamp for completion
          lastTestActivity.current.set(testName, eventTimestamp);
          
          const completionData = {
            success: eventData.success,
            summary: eventData.summary || (eventData.success ? 'PASSED - no issues' : 'FAILED'),
            duration: eventData.duration,
            command: eventData.command,
            userMessage: eventData.userMessage
          };
          
          // Add the clean test completion log
          console.log(`[BatchTestRunner] ${completionData.success ? '✅' : '❌'} Test completed: ${testName} - ${completionData.success ? 'PASSED' : 'FAILED'}`);
          
          // 🛡️ ENHANCED: Use async atomic state update for completion with double-check
          const newStatus = completionData.success ? 'success' : 'failed';
          const success = await updateTestStateAtomic(testName, newStatus, completionData.summary, {
            duration: completionData.duration,
            command: completionData.command,
            userMessage: completionData.userMessage
          });
          
          if (success) {
            // 🛡️ ENHANCED: Atomic removal with validation
            setCurrentlyRunning(prev => {
              const newSet = new Set(prev);
              
              // Ensure test is actually in the set before removing
              if (newSet.has(testName)) {
                newSet.delete(testName);
              }
              
              return newSet;
            });
            
            // 🛡️ ENHANCED: Protected progress calculation with validation
            setTimeout(() => {
              setTestResults(currentResults => {
                // Validate that the test is actually completed before updating progress
                const testState = currentResults[testName];
                if (testState && ['success', 'failed'].includes(testState.status)) {
                  const newProgress = calculateProgress(currentResults, selectedTests.size);
                  
                  // Additional validation: ensure progress doesn't exceed 100%
                  const validatedProgress = Math.min(100, Math.max(0, newProgress));
                  setOverallProgress(validatedProgress);
                }
                return currentResults;
              });
            }, 100); // Slightly longer delay to ensure all state updates are complete
          }
        }
        break;

      case 'batch_complete':
        console.log('[BatchTestRunner] 🏁 BATCH_COMPLETE - clearing all running state');
        
        // Force-complete any tests still in non-final states (fixes sync bug)
        setTestResults(prev => {
          const updated = { ...prev };
          let forcedCompletions = 0;
          
          Object.keys(updated).forEach(testName => {
            const currentStatus = updated[testName]?.status;
            if (!['success', 'failed', 'terminated'].includes(currentStatus)) {
              updated[testName] = {
                status: 'failed',
                message: 'Test not found in results file - execution may have been skipped',
                error: 'TEST_NOT_IN_RESULTS'
              };
              forcedCompletions++;
              console.log(`[BatchTestRunner] 🔧 Force-completed stuck test: ${testName} (was ${currentStatus})`);
            }
          });
          
          if (forcedCompletions > 0) {
            console.log(`[BatchTestRunner] ✅ Force-completed ${forcedCompletions} stuck tests on batch completion`);
          }
          
          return updated;
        });
        
        setIsRunning(false);
        setIsLoading(false);
        setCurrentlyRunning(new Set());
        setOverallProgress(100);
        stopStatusPolling();
        break;

      case 'batch_error':
        // Handle build errors or other batch-level errors
        setError(eventData.error || 'An error occurred during test execution');
        setIsRunning(false);
        stopStatusPolling();
        break;

      case 'live_output':
        // Smart detection: Use live_output patterns to detect when tests are actually running
        const output = eventData.output;
        
        // 🎯 SYNCHRONIZED: Pattern for "Running test:" - show both phase message AND running label
        const runningTestMatch = output.match(/Running test:\s*(.+?)(?:\s|$)/i);
        if (runningTestMatch) {
          let testNameFromRunning = runningTestMatch[1].trim();
          
          // Try to match against our selected test names
          const matchingTest = Array.from(selectedTests).find(testName => 
            testNameFromRunning.includes(testName) || 
            testName.includes(testNameFromRunning) ||
            testName === testNameFromRunning
          );
          
          if (matchingTest) {
            console.log(`[BatchTestRunner] 🎯 SYNCHRONIZED: "Running test:" detected for ${matchingTest} - updating currentlyRunning`);
            setCurrentlyRunning(prev => {
              const newSet = new Set(prev);
              newSet.add(matchingTest);
              console.log('[BatchTestRunner] 📝 Updated currentlyRunning from "Running test:":', Array.from(newSet));
              return newSet;
            });
          }
        }
        
        // Pattern: "Executing in pod diagnostic-test/testname-" indicates test is actively running
        const executingMatch = output.match(/Executing in pod diagnostic-test\/([\w-]+)-/);
        if (executingMatch) {
          const runningTestName = executingMatch[1];
          setCurrentlyRunning(prev => {
            const newSet = new Set(prev);
            newSet.add(runningTestName);
            return newSet;
          });
        }
        
        // Pattern: "✅ PASS" or "❌ FAIL" or test completion indicates test finished
        const completionMatch = output.match(/✅ (\w+[-\w]*): PASSED|❌ (\w+[-\w]*): FAILED|✅ PASS \(\d+\.\d+s\)/);
        if (completionMatch) {
          // Clear all currently running tests when we see completion messages
          setCurrentlyRunning(new Set());
        }
        
        // Handle live terminal output for immediate display
        setLiveOutput(prev => [...prev, output]);
        // Also update filtered output for clean display with transformation
        if (shouldDisplayLine(output)) {
          const transformedLine = transformLine(output);
          if (transformedLine && transformedLine.trim()) {
            setFilteredOutput(prev => [...prev, transformedLine]);
          }
        }
        break;

      case 'phase_update':
        // Handle phase updates (setup, infrastructure, cleanup, testing)
        setCurrentPhase(eventData.phase);
        console.log('[BatchTestRunner] Phase update:', eventData.phase, eventData.message);
        break;

      case 'raw_output':
        // Handle raw terminal output for immediate display
        console.log('[BatchTestRunner] Raw output received:', eventData.output);
        break;

      case 'connected':
        // Handle connection confirmation
        console.log('[BatchTestRunner] Connected to test stream:', eventData.message);
        setLiveOutput(['🔗 Connected to test stream...']);
        break;

      default:
        // Handle unknown event types
        console.log('[BatchTestRunner] Unknown event type:', eventData.type, eventData);
        break;
    }
  };

  const getStatusIcon = (status) => {
    switch (status) {
      case 'success': return '✅';
      case 'failed': return '❌';
      case 'terminated': return '🛑';
      case 'running': return ''; // Use clean loader instead of emoji
      case 'loading': return ''; // Remove emoji since we have clean loader in main area
      case 'queued': return ''; // Remove emoji, use clean loader
      case 'ready': return '';
      default: return '';
    }
  };

  const getStatusClass = (status) => {
    // Status classes removed to preserve category colors
    // Visual status is now indicated by icons, progress bars, and borders only
    return '';
  };

  // Initialize empty test states and cleanup on unmount
  useEffect(() => {
    // Initialize skeleton test states for display
    const initialResults = {};
    const initialOutputs = {};
    testQueue.forEach(test => {
      initialResults[test] = { status: 'ready', message: 'Ready to run...' };
      initialOutputs[test] = [];
    });
    setTestResults(initialResults);
    setTestOutputs(initialOutputs);
    
    return () => {
      stopStatusPolling();
    };
  }, []);

  // Update selectedTests when testQueue changes
  useEffect(() => {
    // Extract string names from potential objects in testQueue
    const testNameStrings = testQueue.map(testName => 
      typeof testName === 'string' ? testName : 
      (testName?.name || testName?.testName || String(testName || 'unknown-test'))
    );
    setSelectedTests(new Set(testNameStrings));
  }, [testQueue]);

  // 🕒 Status update interval for progressive status system - OPTIMIZED
  useEffect(() => {
    if (currentlyRunning.size === 0) return;
    
    const interval = setInterval(() => {
      // Force re-render to update status messages and colors (silent)
      setStatusUpdateCounter(prev => prev + 1);
    }, 10000); // Update every 10 seconds
    
    return () => {
      clearInterval(interval);
    };
  }, [currentlyRunning.size]); // Dependencies: Only currentlyRunning.size (no circular dependency)

  // Toggle individual test selection
  const toggleTestSelection = (testName) => {
    setSelectedTests(prev => {
      const newSet = new Set(prev);
      if (newSet.has(testName)) {
        newSet.delete(testName);
      } else {
        newSet.add(testName);
      }
      return newSet;
    });
  };

  // Toggle all tests selection
  const toggleAllTests = () => {
    if (selectedTests.size === testQueue.length) {
      // All are selected, deselect all
      setSelectedTests(new Set());
    } else {
      // Some or none are selected, select all
      setSelectedTests(new Set(testQueue));
    }
  };

  // Toggle CLI commands visibility
  const toggleCliCommands = () => {
    setShowCliCommands(!showCliCommands);
  };

  // Get the predominant category color for the current test batch
  const getCurrentCategoryColor = () => {
    if (selectedTests.size === 0) return '#14b8a6'; // teal-500 fallback
    
    // Count category occurrences in selected tests
    const categoryCounts = {};
    for (const testName of selectedTests) {
      const colorClass = getTestColorClass(testName);
      categoryCounts[colorClass] = (categoryCounts[colorClass] || 0) + 1;
    }
    
    // Get the most common category
    const predominantCategory = Object.keys(categoryCounts).reduce((a, b) => 
      categoryCounts[a] > categoryCounts[b] ? a : b
    );
    
    // Map category to hex color (matching skeleton colors)
    const categoryColors = {
      'test-card-results': '#f87171',      // red-400 - Security/Policy Tests
      'test-card-networking': '#f472b6',   // pink-400 - Networking Tests  
      'test-card-l3': '#fb923c',           // orange-400 - L3 Policy Tests
      'test-card-l4': '#818cf8',           // indigo-400 - L4 Policy Tests
      'test-card-l7': '#fbbf24',           // yellow-400 - L7 Policy Tests
      'test-card-dns': '#a78bfa',          // purple-400 - DNS Service Tests
      'test-card-infrastructure': '#14b8a6' // teal-500 - Infrastructure Tests
    };
    
    return categoryColors[predominantCategory] || '#14b8a6';
  };

  // Calculate opacity based on progress (25% -> 50% -> 100%)
  const getProgressOpacity = (progress) => {
    if (progress === 0) return 0.25;
    if (progress < 50) return 0.5;
    if (progress < 100) return 0.75;
    return 1.0;
  };

  // Enhanced line formatter for better visual hierarchy
  const formatTerminalLine = (line, index) => {
    const trimmedLine = line.trim();
    
    // Phase headers
    if (trimmedLine.match(/^🧹|^🔍|^🧪|^📊/)) {
      return (
        <div key={index} className="border-l-4 border-blue-400 bg-blue-50 bg-opacity-20 pl-4 py-2 my-2 rounded-r">
          <span className="font-semibold text-blue-200">{trimmedLine}</span>
        </div>
      );
    }
    
    // Success messages
    if (trimmedLine.match(/^✅|PASS|SUCCESS/i)) {
      return (
        <div key={index} className="border-l-4 border-green-400 bg-green-50 bg-opacity-20 pl-4 py-1 my-1 rounded-r">
          <span className="text-green-300 font-medium">{trimmedLine}</span>
        </div>
      );
    }
    
    // Error/failure messages  
    if (trimmedLine.match(/^❌|FAIL|ERROR/i)) {
      return (
        <div key={index} className="border-l-4 border-red-400 bg-red-50 bg-opacity-20 pl-4 py-1 my-1 rounded-r">
          <span className="text-red-300 font-medium">{trimmedLine}</span>
        </div>
      );
    }
    
    // Command output (indented or technical details)
    if (trimmedLine.match(/^\s+|Command stdout|packets transmitted|rtt min\/avg\/max/)) {
      return (
        <div key={index} className="bg-gray-800 bg-opacity-50 px-3 py-1 my-1 rounded text-gray-300 font-mono text-xs">
          {trimmedLine}
        </div>
      );
    }
    
    // Tree structure (├── └──)
    if (trimmedLine.match(/^[├└]──/)) {
      return (
        <div key={index} className="pl-2 py-1 text-cyan-300 font-medium">
          {trimmedLine}
        </div>
      );
    }
    
    // Timestamps and INFO logs
    if (trimmedLine.match(/^\d{4}-\d{2}-\d{2}.*INFO/)) {
      return (
        <div key={index} className="text-xs text-gray-400 py-0.5">
          {trimmedLine}
        </div>
      );
    }
    
    // Default formatting
    return (
      <div key={index} className="py-0.5 text-gray-200">
        {trimmedLine}
      </div>
    );
  };

  // Generate beautiful rich display format using comprehensive test insights
  const generateRichSuccessDisplay = (testName, result) => {
    const duration = result.duration || '6.5';
    const insights = getTestInsights(testName);
    
    if (insights?.success) {
      // Use insights-based success message with duration
      const titleWithDuration = insights.success.title.replace('working perfectly!', `working perfectly! (${duration}s)`);
      
      return {
        mainStatus: titleWithDuration,
        details: insights.success.details
      };
    }
    
    // Fallback for tests not in insights system
    return {
      mainStatus: `✅ Test completed successfully! (${duration}s)`,
      details: [
        `📊 All connectivity checks passed`,
        `🎯 Your cluster networking is functioning`,
        `💡 Infrastructure is ready for workloads`
      ]
    };
  };

  // Generate rich failure display format using comprehensive test insights
  const generateRichFailureDisplay = (testName, result) => {
    const duration = result.duration || '0';
    const insights = getTestInsights(testName);
    
    if (insights?.failure) {
      // Use insights-based failure message with duration
      const titleWithDuration = insights.failure.title.replace('failed', `failed (${duration}s)`);
      
      return {
        mainStatus: titleWithDuration,
        details: insights.failure.details
      };
    }
    
    // Fallback for tests not in insights system or when no specific insights available
    const errorDetails = result.userMessage?.description || result.message || 'Test failed';
    const troubleshooting = result.userMessage?.hints?.[0] || 'Check cluster configuration and try again';
    
    return {
      mainStatus: `❌ Test failed (${duration}s)`,
      details: [
        `📊 ${errorDetails}`,
        `🔧 ${troubleshooting}`,
        `💡 Review the logs above for detailed diagnostics`
      ]
    };
  };

  // Generate rich terminated display format for user-stopped tests
  const generateRichTerminatedDisplay = (testName, result) => {
    // CRITICAL FIX: Ensure testName is always a string to prevent "[object Object]" display
    const testNameString = typeof testName === 'string' ? testName : 
                           (testName?.name || testName?.testName || String(testName || 'unknown-test'));
    const duration = result.duration || '0';
    
    return {
      mainStatus: `🛑 ${testNameString} test terminated (${duration}s)`,
      details: [
        `📋 Test execution was stopped by user request`,
        `🔄 No diagnostic issues - test was manually terminated`,
        `💡 Run the test again to complete validation`
      ]
    };
  };

  // Copy CLI command to clipboard
  const copyCommandToClipboard = async (testName) => {
    const command = `./k8s_diagnostic test list: ${testName} --verbose`;
    try {
      await navigator.clipboard.writeText(command);
      // Could add a toast notification here
      console.log(`[BatchTestRunner] Copied command to clipboard: ${command}`);
    } catch (err) {
      console.error('[BatchTestRunner] Failed to copy to clipboard:', err);
      // Fallback: select text (older browser support)
      const textArea = document.createElement('textarea');
      textArea.value = command;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
    }
  };

  // Handler for starting tests
  const handleRunTests = async () => {
    if (selectedTests.size === 0) {
      setError('No tests selected to run');
      return;
    }

    setHasStarted(true);
    setIsLoading(true);
    setError(null);
    
    // Set all selected tests to loading state immediately
    const loadingResults = {};
    const loadingOutputs = {};
    selectedTests.forEach(testName => {
      loadingResults[testName] = { status: 'loading', message: 'Initializing test...' };
      loadingOutputs[testName] = [];
    });
    setTestResults(loadingResults);
    setTestOutputs(loadingOutputs);
    
    try {
      await runBatchTests();
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="batch-test-runner max-w-7xl mx-auto p-6 animate-fade-in">
      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-3xl font-poppins font-bold text-gray-900 mb-2">
              🚀 Batch Test Execution
            </h1>
            <p className="text-lg font-inter text-gray-600">
              Running {testQueue.length} diagnostic tests
            </p>
          </div>
          
          <div style={{ display: 'flex', gap: '10px' }}>
            {/* 1. Back to Queue - Always visible, disabled when running */}
            <button 
              onClick={onBack}
              disabled={isRunning}
              className={`font-comfortaa hover-lift card-shadow ${isRunning ? 'opacity-50 cursor-not-allowed' : ''}`}
              style={{ 
                padding: '12px 24px', 
                borderRadius: '5px',
                border: '2px solid #000000',
                backgroundColor: '#ffffff',
                color: '#000000'
              }}
            >
              ← Back to Queue
            </button>
            
            {/* 2. Select All/Deselect All - Always visible, disabled when tests have started */}
            <button 
              onClick={toggleAllTests}
              disabled={hasStarted}
              className={`font-comfortaa font-semibold hover-lift card-shadow ${hasStarted ? 'opacity-50 cursor-not-allowed' : ''}`}
              style={{ 
                padding: '12px 24px', 
                borderRadius: '5px',
                border: 'none',
                background: hasStarted ? '#9CA3AF' : (selectedTests.size === testQueue.length 
                  ? 'linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%)' // Red when "Deselect All"
                  : 'linear-gradient(135deg, #48bb78 0%, #38a169 100%)'), // Green when "Select All"
                color: '#ffffff'
              }}
              title={hasStarted ? "Cannot change selection after tests have started" : 
                     (selectedTests.size === testQueue.length ? "Deselect all tests" : "Select all tests")}
            >
              {selectedTests.size === testQueue.length ? '☐ Deselect All' : '☑️ Select All'} ({selectedTests.size}/{testQueue.length})
            </button>

            {/* 3. Start Tests / Run Again - Always visible, disabled when no tests selected or when running */}
            <button 
              onClick={handleRunTests}
              disabled={selectedTests.size === 0 || isRunning}
              className={`font-comfortaa font-semibold hover-lift card-shadow ${(selectedTests.size === 0 || (isRunning && !isLoading)) ? 'opacity-50 cursor-not-allowed' : ''}`}
              style={{ 
                padding: isLoading ? '10px 16px' : '12px 24px', 
                borderRadius: '5px',
                border: 'none',
                background: (selectedTests.size === 0 || (isRunning && !isLoading)) ? '#9CA3AF' : 'linear-gradient(135deg, #a855f7 0%, #7c3aed 100%)',
                color: '#ffffff',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
              }}
              title={selectedTests.size === 0 ? "Select at least one test to run" : 
                     isRunning ? "Processing..." :
                     (hasStarted ? "Run the tests again" : "Start running the selected tests")}
            >
              {isLoading ? (
                <>
                  <div className="animate-spin" style={{ 
                    width: '20px', 
                    height: '20px', 
                    border: '3px solid rgba(255,255,255,0.3)', 
                    borderRadius: '50%', 
                    borderTop: '3px solid #ffffff',
                    marginRight: '7px'
                  }}></div>
                  Processing…
                </>
              ) : 
               hasStarted && !isRunning && Object.keys(testResults).length > 0 ? 'Run Again' : '🚀 Start Tests'}
            </button>
            
            {/* 4. Stop Tests - Always visible, disabled when not running */}
            <button 
              onClick={stopAllTests}
              disabled={!isRunning}
              className={`font-comfortaa font-semibold hover-lift card-shadow ${!isRunning ? 'opacity-50 cursor-not-allowed' : ''}`}
              style={{ 
                padding: '12px 24px', 
                borderRadius: '5px',
                border: 'none',
                background: !isRunning ? '#9CA3AF' : 'linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%)',
                color: '#ffffff'
              }}
              title={!isRunning ? "No tests are currently running" : "Immediately terminate all running tests"}
            >
              🛑 Stop Tests
            </button>

            {/* 5. Toggle CLI Commands - Always visible */}
            <button 
              onClick={toggleCliCommands}
              className="font-comfortaa font-semibold hover-lift card-shadow"
              style={{ 
                padding: '12px 24px', 
                borderRadius: '5px',
                border: 'none',
                backgroundColor: showCliCommands ? '#000000' : '#fbbf24',
                color: showCliCommands ? '#ffffff' : '#000000'
              }}
              title={showCliCommands ? "Hide CLI commands from all test cards" : "Show CLI commands in all test cards"}
            >
              {showCliCommands ? 'Hide CLI Commands' : 'Show CLI Commands'}
            </button>
          </div>
        </div>

        {/* Overall Progress - Clean Simple Progress Bar */}
        {selectedTests.size > 0 && (
          <div className="mb-6 p-4 bg-white border rounded-xl shadow-lg">
            <div className="mb-2">
              <h3 className="text-sm font-semibold text-gray-800">Test Progress {Math.round(overallProgress)}%</h3>
            </div>
            <div style={{ 
                   width: '100%', 
                   height: '16px', 
                   backgroundColor: '#e5e7eb',
                   borderRadius: '9999px'
                 }} 
                 role="progressbar" 
                 aria-valuenow={overallProgress} 
                 aria-valuemin="0" 
                 aria-valuemax="100">
              <div style={{ 
                     width: `${Math.max(overallProgress, 5)}%`,
                     height: '100%',
                     backgroundColor: '#10b981',
                     opacity: getProgressOpacity(overallProgress),
                     borderRadius: '9999px',
                     transition: 'width 0.5s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.5s ease-in-out',
                     minWidth: overallProgress > 0 ? '5%' : '0%'
                   }}></div>
            </div>
          </div>
        )}

        {/* Error Display */}
        {error && (
          <div className="error-card bg-red-50 border border-red-200 mb-6">
            <div className="flex items-center">
              <span className="text-red-500 text-xl mr-3">❌</span>
              <div>
                <div className="font-inter font-semibold text-red-800">Error</div>
                <div className="font-inter text-red-700">{error}</div>
              </div>
            </div>
          </div>
        )}
        
        {/* Live Terminal Output Modal - CleanupButton Style */}
        {isRunning && (
          <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 animate-fade-in">
            <div className="max-w-4xl mx-4 card-shadow-lg font-comfortaa" 
                 style={{ backgroundColor: '#d4edda', padding: '17px', margin: '10px', borderRadius: '5px' }}>
              <div className="text-center mb-6">
                <h3 className="text-xl font-comfortaa font-bold text-gray-900 mb-2">
                  🚨 Live Test Execution Monitor
                </h3>
                <p className="font-comfortaa text-gray-600 text-sm">
                  Real-time status updates from your diagnostic test execution
                  {currentPhase && (
                    <span className="ml-2 bg-blue-100 text-blue-800 px-2 py-1 rounded-full text-xs font-medium">
                      {currentPhase === 'setup' ? '🔍 Setup Phase' : 
                       currentPhase === 'infrastructure' ? '📊 Infrastructure Phase' :
                       currentPhase === 'cleanup' ? '🧹 Cleanup Phase' :
                       currentPhase === 'testing' ? '🔧 Testing Phase' : 
                       `📋 ${currentPhase} Phase`}
                    </span>
                  )}
                </p>
              </div>

              {/* Clean Terminal Content Area */}
              <div 
                className="bg-gray-50 rounded-lg p-4 max-h-96 overflow-y-auto test-output-scroll"
                style={{ scrollBehavior: 'auto' }}
              >
                {filteredOutput.length > 0 ? (
                  <div className="space-y-2">
                    {filteredOutput.map((line, index) => (
                      <div key={index} className="font-comfortaa text-sm text-gray-700">
                        {line}
                      </div>
                    ))}
                    
                  </div>
                ) : (
                  <div className="flex items-center justify-center h-32 text-gray-500 font-comfortaa">
                    {isRunning ? (
                      <>
                        <div className="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent text-gray-500 rounded-full mr-2" role="status" aria-label="loading">
                        </div>
                        Initializing test execution...
                      </>
                    ) : (
                      'No status updates yet...'
                    )}
                  </div>
                )}
              </div>

            </div>
          </div>
        )}
      </div>

      {/* Test Results Grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {testQueue.filter(testName => {
          // Extract test name string for filtering
          const testNameString = typeof testName === 'string' ? testName : 
                                 (testName?.name || testName?.testName || String(testName || 'unknown-test'));
          return !hasStarted || selectedTests.has(testNameString);
        }).map((testName) => {
          // Extract test name string from potential object
          const testNameString = typeof testName === 'string' ? testName : 
                                 (testName?.name || testName?.testName || String(testName || 'unknown-test'));
          
          const result = testResults[testNameString] || { status: 'queued', message: 'Waiting to start...' };
          const outputs = testOutputs[testNameString] || [];
          const colorClass = getTestColorClass(testName);
          const icon = getTestIcon(testName);
          const statusIcon = getStatusIcon(result.status);
          const statusClass = getStatusClass(result.status);

          return (
            <div
              key={testNameString}
              className={`test-card border-2 card-shadow transition-all duration-300 ${colorClass} ${statusClass} ${!selectedTests.has(testNameString) && !hasStarted ? 'opacity-60' : ''}`}
              style={{ borderRadius: '5px', padding: '15px', margin: '10px', position: 'relative' }}
            >
              {/* Individual Test Selection Checkbox - Only when tests haven't started */}
              {!hasStarted && !isRunning && (
                <button
                  onClick={() => toggleTestSelection(testNameString)}
                  className="hover-lift transition-all duration-200"
                  style={{
                    position: 'absolute',
                    top: '10px',
                    right: '10px',
                    width: '24px',
                    height: '24px',
                    borderRadius: '4px',
                    border: '2px solid #000000',
                    backgroundColor: selectedTests.has(testNameString) ? '#10b981' : '#ffffff',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    zIndex: 1000
                  }}
                  title={selectedTests.has(testNameString) ? "Click to deselect this test" : "Click to select this test"}
                >
                  {selectedTests.has(testNameString) ? (
                    <span style={{ color: '#ffffff', fontSize: '14px', fontWeight: 'bold' }}>✓</span>
                  ) : null}
                </button>
              )}

              {/* Test Header */}
              <div className="flex items-center justify-between mb-4">
                <div className="flex-1 pr-8">
                  <h3 className="font-poppins font-semibold text-gray-900 flex items-center">
                    <span>{icon}</span>
                    <span style={{ marginLeft: '5px' }}>{testNameString}</span>
                    {currentlyRunning.has(testNameString) && (
                      <span style={{
                        marginLeft: '12px',
                        backgroundColor: getStatusColor(testNameString),
                        color: '#ffffff',
                        padding: '4px 8px',
                        borderRadius: '12px',
                        fontSize: '0.75rem',
                        fontWeight: '500',
                        display: 'inline-block',
                        transition: 'background-color 0.3s ease-in-out'
                      }}>
                        {getRunningStatusMessage(testNameString)}
                      </span>
                    )}
                  </h3>
                  <p className="font-inter text-sm text-gray-600 mt-1">
                    {getTestCategory(testName)}
                  </p>
                </div>
              </div>

              {/* CLI Command Section - Terminal Style */}
              {showCliCommands && (
                <div className="rounded text-white cli-command mb-3" style={{
                  fontFamily: 'Monaco, Menlo, Consolas, "Courier New", monospace',
                  fontWeight: 'normal',
                  backgroundColor: 'rgb(55, 65, 81)',
                  fontSize: '0.875rem',
                  letterSpacing: '0.025em',
                  lineHeight: '1.5',
                  color: 'rgb(255, 255, 255)',
                  padding: '5px 30px 5px 5px',
                  margin: '5px',
                  display: 'block',
                  position: 'relative'
                }}>
                  ./k8s_diagnostic test list: {testNameString} --verbose
                  <span 
                    title="Copy command to clipboard"
                    onClick={() => copyCommandToClipboard(testNameString)}
                    style={{
                      position: 'absolute',
                      right: '5px',
                      top: '50%',
                      transform: 'translateY(-50%)',
                      cursor: 'pointer',
                      fontSize: '0.875rem',
                      opacity: '0.8',
                      transition: 'opacity 0.2s'
                    }}
                    onMouseEnter={(e) => e.target.style.opacity = '1'}
                    onMouseLeave={(e) => e.target.style.opacity = '0.8'}
                  >
                    📋
                  </span>
                </div>
              )}

              {/* Rich Status Area with User Messages */}
              <div className="bg-white bg-opacity-70 rounded-lg p-3 min-h-20">
                {hasStarted && (result.status === 'queued' || result.status === 'running' || result.status === 'loading') && (
                  <div style={{ minHeight: '80px', display: 'flex', flexDirection: 'column', gap: '16px', paddingTop: '8px' }}>
                    <div style={{ 
                      height: '14px', 
                      borderRadius: '7px',
                      width: '85%',
                      backgroundColor: '#ffffff',
                      animation: 'pulse 2s ease-in-out infinite'
                    }}></div>
                    <div style={{ 
                      height: '14px', 
                      borderRadius: '7px',
                      width: '70%',
                      backgroundColor: '#ffffff',
                      animation: 'pulse 2s ease-in-out infinite 0.5s'
                    }}></div>
                    <div style={{ 
                      height: '14px', 
                      borderRadius: '7px',
                      width: '60%',
                      backgroundColor: '#ffffff',
                      animation: 'pulse 2s ease-in-out infinite 1s'
                    }}></div>
                    <style jsx>{`
                      @keyframes pulse {
                        0%, 100% { opacity: 0.4; }
                        50% { opacity: 1; }
                      }
                    `}</style>
                  </div>
                )}
                
                {result.status === 'success' && (() => {
                  const richDisplay = generateRichSuccessDisplay(testName, result);
                  return (
                    <div className="text-green-600">
                      <div className="font-semibold font-inter text-base mb-3" style={{ lineHeight: '1.4', marginTop: '10px' }}>
                        {richDisplay.mainStatus}
                      </div>
                      <div className="space-y-1">
                        {richDisplay.details.map((detail, index) => (
                          <div key={index} className="text-sm font-inter text-green-700" style={{ lineHeight: '1.5' }}>
                            {detail}
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })()}
                
                {result.status === 'failed' && (() => {
                  const richDisplay = generateRichFailureDisplay(testName, result);
                  return (
                    <div className="text-red-600">
                      <div className="font-semibold font-inter text-base mb-3" style={{ lineHeight: '1.4' }}>
                        {richDisplay.mainStatus}
                      </div>
                      <div className="space-y-1">
                        {richDisplay.details.map((detail, index) => (
                          <div key={index} className="text-sm font-inter text-red-700" style={{ lineHeight: '1.5' }}>
                            {detail}
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })()}
                
                {result.status === 'terminated' && (() => {
                  const richDisplay = generateRichTerminatedDisplay(testName, result);
                  return (
                    <div className="text-amber-600">
                      <div className="font-semibold font-inter text-base mb-3" style={{ lineHeight: '1.4' }}>
                        {richDisplay.mainStatus}
                      </div>
                      <div className="space-y-1">
                        {richDisplay.details.map((detail, index) => (
                          <div key={index} className="text-sm font-inter text-amber-700" style={{ lineHeight: '1.5' }}>
                            {detail}
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })()}
                
                {/* Fallback display for other statuses */}
                {!hasStarted && (
                  <div className="font-inter" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div></div>
                    <span style={{
                      backgroundColor: '#10b981',
                      color: '#ffffff',
                      padding: '4px 8px',
                      borderRadius: '12px',
                      fontSize: '0.75rem',
                      fontWeight: '500',
                      display: 'inline-block'
                    }}>
                      Ready to run
                    </span>
                  </div>
                )}
              </div>

              {/* Progress for running tests */}
              {result.status === 'running' && (
                <div className="mt-3 bg-gray-200 rounded-full h-2">
                  <div className="bg-blue-500 h-2 rounded-full animate-pulse" style={{ width: '60%' }}></div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Results Summary - Show immediately when any test completes */}
      {hasStarted && Object.keys(testResults).length > 0 && 
       Object.values(testResults).some(r => r.status === 'success' || r.status === 'failed') && (
        <div className="summary-card mt-8 bg-gradient-to-r from-gray-50 to-gray-100 border border-gray-200 card-shadow-lg">
          <h3 className="font-poppins font-semibold text-gray-900 text-lg mb-4">
            📊 Test Results Summary
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="text-center p-4 bg-white border" style={{ borderRadius: '5px' }}>
              <div className="font-poppins font-bold text-green-600 flex items-center justify-center">
                <span className="text-xl" style={{ marginRight: '10px' }}>✅</span>
                <span className="text-2xl" style={{ marginRight: '10px' }}>
                  {Object.values(testResults).filter(r => r.status === 'success').length}
                </span>
                <span className="text-sm">Tests Passed</span>
              </div>
            </div>
            <div className="text-center p-4 bg-white border" style={{ borderRadius: '5px' }}>
              <div className="font-poppins font-bold text-red-600 flex items-center justify-center">
                <span className="text-xl" style={{ marginRight: '10px' }}>❌</span>
                <span className="text-2xl" style={{ marginRight: '10px' }}>
                  {Object.values(testResults).filter(r => r.status === 'failed').length}
                </span>
                <span className="text-sm">Tests Failed</span>
              </div>
            </div>
            <div className="text-center p-4 bg-white border" style={{ borderRadius: '5px' }}>
              <div className="font-poppins font-bold text-blue-600 flex items-center justify-center">
                <span className="text-xl" style={{ marginRight: '10px' }}>📊</span>
                <span className="text-2xl" style={{ marginRight: '10px' }}>
                  {selectedTests.size}
                </span>
                <span className="text-sm">Total Tests</span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
