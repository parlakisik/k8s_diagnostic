import { useState } from 'react';

export default function CleanupButton({ onCleanupComplete, disabled }) {
  const [isRunning, setIsRunning] = useState(false);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [cleanupLog, setCleanupLog] = useState([]);
  const [showLog, setShowLog] = useState(false);
  const [error, setError] = useState(null);

  // Enhanced cleanup operation tracker
  const cleanupOperationCounter = { current: 0 };
  const cleanupOperations = [
    "Network policies removed",
    "Test pods cleaned up", 
    "Namespaces deleted",
    "Services removed",
    "ConfigMaps cleared",
    "Secrets cleaned up"
  ];

  // Enhanced message transformation function
  const transformCleanupMessage = (message) => {
    const trimmed = message.trim();
    
    // Transform generic "Done" messages into descriptive ones
    if (trimmed.match(/^✅.*Done \(\d+\.\d+s\)$/)) {
      const timeMatch = trimmed.match(/\((\d+\.\d+s)\)$/);
      const duration = timeMatch ? timeMatch[1] : '0.0s';
      
      // Get next cleanup operation description
      const operationIndex = cleanupOperationCounter.current % cleanupOperations.length;
      const operationDescription = cleanupOperations[operationIndex];
      cleanupOperationCounter.current++;
      
      return `✅ ${operationDescription} (${duration})`;
    }
    
    return trimmed; // Return original message if no transformation needed
  };

  // Filter function to show only essential cleanup messages
  const shouldDisplayCleanupLine = (message) => {
    const trimmed = message.trim();
    
    // Hide any hierarchy lines completely
    if (trimmed.includes('├──') || trimmed.includes('└──') || trimmed.includes('│')) return false;
    if (trimmed.includes('🧹 ├──') || trimmed.includes('🧹 └──')) return false;
    
    // Hide verbose details and separators
    if (trimmed.includes('=====================================')) return false;
    if (trimmed.includes('🎯 Target namespace:')) return false;
    if (trimmed.includes('🧹 Starting deep cleanup')) return false;
    if (trimmed.includes('🔍 Verifying')) return false;
    if (trimmed.includes('All test resources confirmed deleted')) return false;
    if (trimmed.includes('Deep cleanup finished')) return false;
    if (trimmed.includes('Comprehensive Test Resource Cleanup')) return false;
    
    // Allow essential status messages
    if (trimmed.match(/^🧹.*Starting resource cleanup/)) return true;
    if (trimmed.match(/^🧹.*CLEANUP PHASE/)) return true;
    if (trimmed.match(/^✅.*Done \(\d+\.\d+s\)$/)) return true;
    if (trimmed.match(/^✅.*Cleanup completed/)) return true;
    if (trimmed.match(/^🎉.*All test resources have been removed/)) return true;
    if (trimmed.match(/^⏱️.*cleanup completed in \d+\.\d+ seconds/)) return true;
    
    return false; // Hide everything else by default
  };

  const handleCleanupClick = () => {
    // Reset all previous state before showing confirmation
    setShowLog(false);
    setCleanupLog([]);
    setError(null);
    setIsRunning(false);
    setShowConfirmModal(true);
  };

  const handleConfirmCleanup = async () => {
    setShowConfirmModal(false);
    setIsRunning(true);
    setError(null);
    setCleanupLog([]);
    setShowLog(true);

    try {
      const response = await fetch('/api/cleanup-resources', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          operation: 'deepclean'
        })
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
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
              handleCleanupEvent(eventData);
            } catch (parseError) {
              // Handle non-JSON lines as raw output
              const rawOutput = line.substring(6);
              if (rawOutput.trim() && shouldDisplayCleanupLine(rawOutput)) {
                const transformedMessage = transformCleanupMessage(rawOutput);
                setCleanupLog(prev => [...prev, { type: 'output', message: transformedMessage }]);
              }
            }
          }
        }
      }

      const finalMessage = '✅ Cleanup completed successfully';
      if (shouldDisplayCleanupLine(finalMessage)) {
        setCleanupLog(prev => [...prev, { type: 'success', message: finalMessage }]);
      }
      
      if (onCleanupComplete) {
        onCleanupComplete(true);
      }

    } catch (err) {
      console.error('Cleanup error:', err);
      setError(`Failed to cleanup resources: ${err.message}`);
      setCleanupLog(prev => [...prev, { type: 'error', message: `❌ Error: ${err.message}` }]);
      
      if (onCleanupComplete) {
        onCleanupComplete(false);
      }
    } finally {
      setIsRunning(false);
    }
  };

  const handleCleanupEvent = (eventData) => {
    switch (eventData.type) {
      case 'cleanup_start':
        const startMessage = '🧹 Starting resource cleanup...';
        if (shouldDisplayCleanupLine(startMessage)) {
          setCleanupLog(prev => [...prev, { type: 'info', message: startMessage }]);
        }
        break;
      case 'cleanup_progress':
        const progressMessage = eventData.message || 'Cleanup in progress...';
        if (shouldDisplayCleanupLine(progressMessage)) {
          setCleanupLog(prev => [...prev, { type: 'info', message: progressMessage }]);
        }
        break;
      case 'cleanup_complete':
        const completeMessage = '✅ Cleanup completed';
        if (shouldDisplayCleanupLine(completeMessage)) {
          setCleanupLog(prev => [...prev, { type: 'success', message: completeMessage }]);
        }
        break;
      case 'cleanup_output':
        if (eventData.output && shouldDisplayCleanupLine(eventData.output)) {
          const transformedMessage = transformCleanupMessage(eventData.output);
          setCleanupLog(prev => [...prev, { type: 'output', message: transformedMessage }]);
        }
        break;
      default:
        if (eventData.message && shouldDisplayCleanupLine(eventData.message)) {
          const transformedMessage = transformCleanupMessage(eventData.message);
          setCleanupLog(prev => [...prev, { type: 'info', message: transformedMessage }]);
        }
        break;
    }
  };

  const getLogMessageClass = (type) => {
    switch (type) {
      case 'success': return 'text-green-700';
      case 'error': return 'text-red-700';
      case 'info': return 'text-blue-700';
      case 'output': return 'text-gray-700';
      default: return 'text-gray-700';
    }
  };

  return (
    <>
      {/* Cleanup Button */}
      <button
        onClick={handleCleanupClick}
        disabled={disabled || isRunning}
        className={`cleanup-btn rounded-xl font-comfortaa font-semibold transition-all ${
          disabled || isRunning
            ? 'opacity-50 cursor-not-allowed'
            : 'hover-lift card-shadow'
        }`}
        style={{ padding: '15px', borderRadius: '5px' }}
        title="Clean up test resources: namespaces, policies, and pods"
      >
        {isRunning ? (
          <>
            <span className="animate-spin inline-block mr-2">🧹</span>
            Cleaning...
          </>
        ) : (
          <>🧹 Cleanup Resources</>
        )}
      </button>

      {/* Confirmation Modal */}
      {showConfirmModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 animate-fade-in">
          <div className="rounded-xl max-w-md mx-4 card-shadow-lg" style={{ backgroundColor: '#d4edda', padding: '17px', margin: '10px' }}>
            <div className="text-center mb-6">
              <h3 className="text-xl font-poppins font-bold text-gray-900 mb-2">
                🚨 Confirm Resource Cleanup
              </h3>
              <p className="font-inter text-gray-600 text-sm">
                This will remove all test namespaces, network policies, and resources created by previous diagnostic tests.
              </p>
            </div>

            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-6">
              <div className="font-inter text-yellow-800 text-sm">
                <div className="font-semibold mb-1">⚠️ This action will:</div>
                <ul className="list-disc list-inside space-y-1 text-xs">
                  <li>Delete test namespaces (diagnostic-test, etc.)</li>
                  <li>Remove Cilium network policies</li>
                  <li>Clean up test pods and services</li>
                  <li>Reset cluster to clean state</li>
                </ul>
              </div>
            </div>

            <div className="flex">
              <button
                onClick={() => setShowConfirmModal(false)}
                className="flex-1 btn-outline font-comfortaa hover-lift"
                style={{ padding: '15px', borderRadius: '5px', marginRight: '7px' }}
              >
                ❌ Cancel
              </button>
              <button
                onClick={handleConfirmCleanup}
                className="flex-1 cleanup-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                style={{ padding: '15px', borderRadius: '5px' }}
              >
                ✅ Confirm Cleanup
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cleanup Log Modal */}
      {showLog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 animate-fade-in">
          <div className="rounded-xl max-w-2xl mx-4 max-h-[80vh] flex flex-col card-shadow-lg relative" style={{ backgroundColor: '#d4edda', padding: '17px', margin: '10px' }}>
            {!isRunning && (
              <button
                onClick={() => setShowLog(false)}
                className="absolute top-2 right-2 text-gray-400 hover:text-gray-600 text-xl z-10"
              >
                ✖️
              </button>
            )}
            <div className="mb-4">
              <h3 className="text-xl font-comfortaa font-bold text-gray-900">
                🧹 Resource Cleanup Progress
              </h3>
            </div>

            {error && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
                <div className="flex items-center">
                  <span className="text-red-500 text-xl mr-3">❌</span>
                  <div>
                    <div className="font-inter font-semibold text-red-800">Cleanup Error</div>
                    <div className="font-inter text-red-700 text-sm">{error}</div>
                  </div>
                </div>
              </div>
            )}

            <div className="bg-gray-50 rounded-lg flex-1 overflow-y-auto test-output-scroll" style={{ padding: '5px' }}>
              {cleanupLog.length > 0 && (
                <div className="space-y-3">
                  {cleanupLog.map((log, index) => {
                    const message = log.message;
                    const messageType = log.type;
                    
                    // Enhanced styling for different message types
                    let messageClass = 'font-inter text-sm';
                    let containerClass = '';
                    
                    if (message.includes('🧹 Starting resource cleanup') || message.includes('🧹 CLEANUP PHASE')) {
                      // Phase headers - larger, bold, with background
                      messageClass += ' font-semibold text-blue-800 text-base';
                      containerClass = 'bg-blue-50 border-l-4 border-blue-400 p-3 rounded-r-lg';
                    } else if (message.includes('✅ Done (') || message.includes('✅ Cleanup completed') || message.includes('🎉 All test resources')) {
                      // Success messages - green with background
                      messageClass += ' font-medium text-green-700';
                      containerClass = 'bg-green-50 border-l-4 border-green-400 p-2 rounded-r-lg';
                    } else if (message.includes('⏱️') && message.includes('completed in')) {
                      // Timing summary - special highlight
                      messageClass += ' font-medium text-purple-700';
                      containerClass = 'bg-purple-50 border-l-4 border-purple-400 p-2 rounded-r-lg';
                    } else {
                      // Regular messages - clean, subtle
                      messageClass += ' text-gray-700';
                      containerClass = 'pl-4 py-1';
                    }
                    
                    return (
                      <div key={index} className={containerClass}>
                        <div className={messageClass}>
                          {message}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {isRunning && (
              <div className="mt-4">
                <div className="bg-gray-200 rounded-full h-2">
                  <div className="bg-gradient-to-r from-orange-500 to-red-500 h-2 rounded-full animate-pulse" style={{ width: '60%' }}></div>
                </div>
                <div className="text-center mt-2 font-comfortaa text-sm text-gray-600">
                  Cleanup in progress
                  <span className="inline-block w-6 text-left">
                    <span 
                      className="inline-block" 
                      style={{ 
                        animation: 'fadeInOut 1.5s infinite',
                        animationDelay: '0s'
                      }}
                    >.</span>
                    <span 
                      className="inline-block" 
                      style={{ 
                        animation: 'fadeInOut 1.5s infinite',
                        animationDelay: '0.5s'
                      }}
                    >.</span>
                    <span 
                      className="inline-block" 
                      style={{ 
                        animation: 'fadeInOut 1.5s infinite',
                        animationDelay: '1s'
                      }}
                    >.</span>
                  </span>
                  <style jsx>{`
                    @keyframes fadeInOut {
                      0%, 60% { opacity: 0.3; }
                      30% { opacity: 1; }
                    }
                  `}</style>
                </div>
              </div>
            )}

          </div>
        </div>
      )}
    </>
  );
}
