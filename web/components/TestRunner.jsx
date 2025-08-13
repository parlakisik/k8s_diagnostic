import { useState, useEffect, useRef } from 'react';
import LogViewer from './LogViewer';
import ResultsViewer from './ResultsViewer';
import ProgressIndicator from './ProgressIndicator';

export default function TestRunner({ selectedQuestion, onBack, onComplete }) {
  const [isRunning, setIsRunning] = useState(false);
  const [events, setEvents] = useState([]);
  const [results, setResults] = useState(null);
  const [testId, setTestId] = useState(null);
  const [error, setError] = useState(null);
  const [progress, setProgress] = useState(0);
  const [currentTest, setCurrentTest] = useState('');
  const [currentStep, setCurrentStep] = useState('');
  const [totalTests, setTotalTests] = useState(0);
  const [completedTests, setCompletedTests] = useState(0);
  
  // New state for enhanced loading
  const [loadingPhase, setLoadingPhase] = useState('idle');
  const [testStates, setTestStates] = useState({});
  const [statusPolling, setStatusPolling] = useState(false);
  const statusIntervalRef = useRef(null);
  const [jsonlFound, setJsonlFound] = useState(false);

  // Start periodic status polling
  const startStatusPolling = (testId) => {
    setStatusPolling(true);
    setLoadingPhase('initializing');
    
    // Initialize test states
    setTestStates({
      'infrastructure': { status: 'pending', progress: 0, message: 'Preparing test environment...' },
      'cleanup': { status: 'pending', progress: 0, message: 'Waiting to start...' },
      'test-execution': { status: 'pending', progress: 0, message: 'Test queued...' }
    });

    statusIntervalRef.current = setInterval(async () => {
      try {
        const response = await fetch(`/api/test-status?testId=${testId}`);
        
        if (response.ok) {
          const statusData = await response.json();
          
          // Update test states with server data
          setTestStates(statusData.testStates || {});
          setJsonlFound(statusData.jsonlFound);
          setCurrentStep(statusData.message || 'Test in progress...');
          
          // Update loading phase based on status
          if (statusData.jsonlFound && statusData.eventCount > 0) {
            setLoadingPhase('streaming');
          } else if (statusData.jsonlFound) {
            setLoadingPhase('waiting-for-jsonl');
          } else {
            setLoadingPhase('initializing');
          }
          
          // Stop polling when complete
          if (statusData.status === 'complete' || statusData.status === 'failed') {
            clearInterval(statusIntervalRef.current);
            setStatusPolling(false);
            setLoadingPhase('complete');
          }
        }
      } catch (error) {
        console.log(`[TestRunner] Status polling error:`, error.message);
      }
    }, 2000); // Poll every 2 seconds
  };

  // Stop status polling
  const stopStatusPolling = () => {
    if (statusIntervalRef.current) {
      console.log(`[TestRunner] Stopping status polling`);
      clearInterval(statusIntervalRef.current);
      statusIntervalRef.current = null;
    }
    setStatusPolling(false);
  };

  const stopTest = async () => {
    console.log('[TestRunner] STOP button clicked - terminating test');
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
        console.log('[TestRunner] Stop signal sent successfully');
        
        // Update UI immediately
        setEvents(prev => [...prev, {
          type: 'info',
          message: '🛑 Test stopped by user',
          timestamp: new Date().toISOString(),
          level: 'INFO'
        }]);
        
        setIsRunning(false);
        setLoadingPhase('complete');
        stopStatusPolling();
        setCurrentStep('Test stopped by user');
      } else {
        console.error('[TestRunner] Failed to stop test:', response.status);
        setError('Failed to stop test - it may continue running');
      }
    } catch (err) {
      console.error('[TestRunner] Error stopping test:', err);
      setError(`Failed to stop test: ${err.message}`);
    }
  };

  const runTest = async () => {
    if (!selectedQuestion) {
      console.log('[TestRunner] No question selected, aborting runTest');
      return;
    }

    const newTestId = Date.now().toString();
    console.log(`[TestRunner] STARTING test execution - TestID: ${newTestId}, Command: ${selectedQuestion.cliCommand}`);
    console.log(`[TestRunner] Current running state: ${isRunning}`);
    
    // Prevent duplicate runs
    if (isRunning) {
      console.log(`[TestRunner] DUPLICATE PREVENTED - Test already running for TestID: ${testId}`);
      return;
    }

    setTestId(newTestId);
    setIsRunning(true);
    setEvents([]);
    setResults(null);
    setError(null);
    setProgress(0);
    setCurrentTest('');
    setCurrentStep('');
    setTotalTests(0);
    setCompletedTests(0);
    
    // Start status polling immediately
    startStatusPolling(newTestId);

    try {
      console.log(`[TestRunner] FETCHING API for test ${newTestId}`);
      const response = await fetch('/api/run-test', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'text/event-stream',
        },
        body: JSON.stringify({
          cliCommand: selectedQuestion.cliCommand,
          testId: newTestId
        })
      });

      console.log(`[TestRunner] API response status: ${response.status} for test ${newTestId}`);

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        console.log(`[TestRunner] API ERROR for test ${newTestId}:`, errorData);
        
        // Handle 409 conflicts gracefully - another test is running
        if (response.status === 409) {
          console.log(`[TestRunner] Test ${newTestId} blocked - another test is running: ${errorData.runningTestId}`);
          
          // If there's a running test ID, try to switch to monitoring that test instead
          if (errorData.runningTestId) {
            console.log(`[TestRunner] Switching to monitor running test: ${errorData.runningTestId}`);
            setTestId(errorData.runningTestId);
            startStatusPolling(errorData.runningTestId);
            
            // Show a helpful message instead of error
            setCurrentStep('Connecting to already running test...');
        setEvents(prev => [...prev, {
          type: 'info',
          message: `Connecting to test ${errorData.runningTestId} already in progress...`,
          timestamp: new Date().toISOString(),
          level: 'INFO'
        }]);
            
            // Don't throw error - just continue with monitoring the running test
            return;
          } else {
            // If no running test ID provided, show friendly message
            setError('Another test is currently running. Please wait a moment and try again.');
            setIsRunning(false);
            stopStatusPolling();
            setLoadingPhase('idle');
            return;
          }
        }
        
        throw new Error(`HTTP error! status: ${response.status} - ${errorData.message || 'Unknown error'}`);
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value);
        const lines = chunk.split('\n');

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const eventData = JSON.parse(line.substring(6));
              
              // Add event to the events list for LogViewer
              setEvents(prev => [...prev, eventData]);
              
              // Handle different event types for UI updates
              handleEvent(eventData);
              
            } catch (parseError) {
              console.error('Failed to parse event:', parseError, line);
            }
          }
        }
      }

    } catch (err) {
      console.error('Test execution error:', err);
      setError(`Failed to execute test: ${err.message}`);
      setIsRunning(false);
      stopStatusPolling();
      setLoadingPhase('error');
      setEvents(prev => [...prev, {
        type: 'error',
        message: `❌ Error: ${err.message}`,
        timestamp: new Date().toISOString(),
        level: 'ERROR'
      }]);
    }
  };

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      stopStatusPolling();
    };
  }, []);

  // Handle different types of events from the JSONL stream
  const handleEvent = (eventData) => {
    switch (eventData.type) {
      case 'connected':
        console.log('Connected to test stream:', eventData.message);
        break;

      case 'jsonl_found':
        console.log('JSONL monitoring started:', eventData.message);
        break;

      case 'suite_start':
        setTotalTests(eventData.totalTests || 0);
        setCurrentStep('Starting test suite...');
        break;

      case 'test_start':
        setCurrentTest(eventData.testName || eventData.data?.testName || '');
        if (eventData.progress !== undefined) {
          setProgress(eventData.progress);
        }
        break;

      case 'test_complete':
        setCompletedTests(prev => prev + 1);
        if (totalTests > 0) {
          setProgress((completedTests + 1) / totalTests * 100);
        }
        break;

      case 'step':
        setCurrentStep(eventData.data?.stepName || eventData.stepName || '');
        break;

      case 'complete':
        setIsRunning(false);
        stopStatusPolling();
        setLoadingPhase('complete');
        if (onComplete) {
          onComplete(eventData.success);
        }
        break;

      case 'results':
        setResults(eventData.data);
        break;

      case 'error':
        setError(eventData.message);
        setIsRunning(false);
        break;

      default:
        // Handle other event types or fallback events
        if (eventData.type === 'stdout' || eventData.type === 'stderr') {
          // These are fallback events from stdout/stderr
          // The structured events are more reliable, so we can use these for additional context
        }
        break;
    }
  };

  // Auto-run test when question is selected (with proper debouncing)
  const hasAutoRun = useRef(false);
  
  useEffect(() => {
    if (selectedQuestion && !isRunning && events.length === 0 && !hasAutoRun.current) {
      console.log(`[TestRunner] Auto-running test for question: ${selectedQuestion.testType}`);
      hasAutoRun.current = true;
      runTest();
    }
  }, [selectedQuestion]);

  // Reset auto-run flag when question changes
  useEffect(() => {
    hasAutoRun.current = false;
  }, [selectedQuestion?.testType]);

  if (!selectedQuestion) {
    return (
      <div className="test-runner max-w-7xl mx-auto p-6">
        <div className="text-center py-12">
          <p className="text-gray-500">No test selected. Please go back and select a diagnostic question.</p>
          <button 
            onClick={onBack}
            className="mt-4 bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded-lg font-semibold transition-colors"
          >
            ← Back to Questions
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="test-runner max-w-7xl mx-auto p-6">
      {/* Test Header */}
      <div className="test-header mb-6">
        <div className="flex items-start justify-between mb-4">
          <div className="flex-1">
            <div className="flex items-center space-x-3 mb-2">
              <span className="text-3xl">{selectedQuestion.icon}</span>
              <h1 className="text-2xl font-bold text-gray-900">
                Running Diagnostic Test
              </h1>
            </div>
            <h2 className="text-lg text-gray-700 mb-3">
              {selectedQuestion.question}
            </h2>
            <p className="text-gray-600 mb-3">
              {selectedQuestion.description}
            </p>
            
            {/* Test Details */}
            <div className="flex flex-wrap items-center gap-4 text-sm">
              <div className="flex items-center space-x-1">
                <span className="text-gray-500">Test Type:</span>
                <code className="bg-gray-100 px-2 py-1 rounded text-xs font-mono">
                  {selectedQuestion.testType}
                </code>
              </div>
              <div className="flex items-center space-x-1">
                <span className="text-gray-500">⏱️ Estimated Time:</span>
                <span className="text-gray-600">{selectedQuestion.estimatedTime}</span>
              </div>
            </div>
          </div>
          
          <div className="flex space-x-3">
            <button 
              onClick={onBack}
              disabled={isRunning}
              className="bg-gray-500 hover:bg-gray-600 text-white px-4 py-2 rounded-lg font-semibold transition-colors"
            >
              ← Back
            </button>
            {isRunning && (
              <button 
                onClick={stopTest}
                className="stop-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                style={{ padding: '12px 24px', borderRadius: '12px' }}
                title="Immediately terminate the running test"
              >
                🛑 STOP Test
              </button>
            )}
            {!isRunning && (
              <button 
                onClick={runTest}
                className="bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded-lg font-semibold transition-colors"
              >
                Run Again
              </button>
            )}
          </div>
        </div>

        {/* CLI Command Display */}
        <div className="bg-gray-50 p-3 rounded-lg border-l-4 border-gray-400">
          <div className="flex items-center space-x-2 mb-1">
            <span className="text-gray-500 text-sm font-semibold">Executing:</span>
            <span className="text-xs text-gray-500">({testId})</span>
          </div>
          <code className="text-sm font-mono text-gray-700">
            {selectedQuestion.cliCommand}
          </code>
        </div>

        {/* Enhanced Test Progress */}
        {isRunning && (
          <div className="mt-4">
            <div className="bg-white rounded-lg border shadow-sm p-4">
              <h3 className="font-semibold text-gray-900 mb-3">🧪 Test Progress</h3>
              
              <div className="space-y-3">
                {Object.entries(testStates).map(([testKey, testState]) => (
                  <div key={testKey} className="flex items-center space-x-3">
                    {/* Status Icon */}
                    <div className="flex-shrink-0 w-6 h-6 flex items-center justify-center">
                      {testState.status === 'complete' && (
                        <div className="w-5 h-5 bg-green-500 rounded-full flex items-center justify-center">
                          <svg className="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                            <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                          </svg>
                        </div>
                      )}
                      {testState.status === 'running' && (
                        <div className="animate-spin rounded-full h-5 w-5 border-2 border-blue-500 border-t-transparent"></div>
                      )}
                      {testState.status === 'waiting' && (
                        <div className="w-5 h-5 bg-yellow-400 rounded-full animate-pulse"></div>
                      )}
                      {testState.status === 'pending' && (
                        <div className="w-5 h-5 bg-gray-300 rounded-full"></div>
                      )}
                      {testState.status === 'failed' && (
                        <div className="w-5 h-5 bg-red-500 rounded-full flex items-center justify-center">
                          <svg className="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                            <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
                          </svg>
                        </div>
                      )}
                    </div>

                    {/* Test Name */}
                    <div className="flex-1">
                      <div className="font-medium text-gray-900 capitalize">
                        {testKey.replace('-', ' ')}
                      </div>
                      <div className="text-sm text-gray-600">
                        {testState.message}
                      </div>
                    </div>

                    {/* Progress Bar */}
                    {testState.status === 'running' && (
                      <div className="flex-shrink-0 w-20">
                        <div className="bg-gray-200 rounded-full h-2">
                          <div 
                            className="bg-blue-500 h-2 rounded-full transition-all duration-500"
                            style={{ width: `${testState.progress || 0}%` }}
                          ></div>
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>

              {/* Overall Status */}
              <div className="mt-4 pt-3 border-t border-gray-200">
                <div className="flex items-center justify-between">
                  <div className="text-sm text-gray-600">
                    {loadingPhase === 'initializing' && 'Initializing test environment...'}
                    {loadingPhase === 'waiting-for-jsonl' && 'Waiting for test data...'}
                    {loadingPhase === 'streaming' && 'Receiving test results...'}
                    {loadingPhase === 'complete' && 'Test completed'}
                    {loadingPhase === 'error' && 'Test failed'}
                  </div>
                  <div className="text-xs text-gray-500">
                    {jsonlFound ? '📄 Log file detected' : (
                      <div className="flex items-center">
                        <div className="animate-spin inline-block w-3 h-3 border border-current border-t-transparent text-gray-500 rounded-full mr-1" role="status" aria-label="loading">
                          <span className="sr-only">Loading...</span>
                        </div>
                        Waiting for logs...
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Fallback Progress Indicator */}
        {!isRunning && (totalTests > 0 || currentTest || currentStep) && (
          <div className="mt-4">
            <ProgressIndicator
              currentTest={currentTest}
              progress={progress}
              currentStep={currentStep}
              totalTests={totalTests}
              completedTests={completedTests}
              isRunning={isRunning}
            />
          </div>
        )}

        {/* Status Indicator */}
        <div className="mt-4">
          {isRunning && (
            <div className="flex items-center space-x-2 text-blue-600">
              <div className="animate-spin rounded-full h-4 w-4 border-2 border-blue-600 border-t-transparent"></div>
              <span className="font-semibold">Test is running... Please wait</span>
            </div>
          )}
          {error && (
            <div className="flex items-center space-x-2 text-red-600">
              <span className="text-xl">❌</span>
              <span className="font-semibold">Error: {error}</span>
            </div>
          )}
          {!isRunning && !error && events.length > 0 && (
            <div className="flex items-center space-x-2 text-green-600">
              <span className="text-xl">✅</span>
              <span className="font-semibold">Test completed</span>
            </div>
          )}
        </div>
      </div>

      {/* Test Content */}
      <div className="test-content space-y-6">
        {/* Live Events Section */}
        <div className="events-section">
          <LogViewer events={events} isRunning={isRunning} />
        </div>
        
        {/* Results Section */}
        {results && (
          <div className="results-section">
            <ResultsViewer results={results} />
          </div>
        )}
        
        {/* Help Section */}
        {!isRunning && events.length > 0 && (
          <div className="help-section bg-blue-50 border border-blue-200 rounded-lg p-4">
            <h3 className="font-semibold text-blue-900 mb-2">💡 What happened?</h3>
            <p className="text-blue-800 text-sm mb-2">
              This test executed the CLI command and streamed structured events in real-time from the JSONL log file.
              The results are also saved to a JSON file in the <code>test_results/</code> directory.
            </p>
            <p className="text-blue-700 text-xs">
              The structured events provide rich information including test progress, command outputs, and detailed results.
              You can run this test manually using the command shown above.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
