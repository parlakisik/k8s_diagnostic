import { useEffect, useRef } from 'react';

export default function LogViewer({ events, isRunning }) {
  const logEndRef = useRef(null);

  useEffect(() => {
    // Auto-scroll to bottom when new events arrive
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [events]);

  // Debug logging for received events
  useEffect(() => {
    if (events.length > 0) {
      console.log('[LogViewer] New events received:', events.slice(-5)); // Log last 5 events
      events.slice(-1).forEach(event => {
        console.log('[LogViewer] Latest event details:', {
          type: event.type,
          isUserFriendly: event.isUserFriendly,
          status: event.status,
          title: event.title,
          description: event.description,
          context: event.context,
          hints: event.hints,
          technicalDetails: event.technicalDetails,
          timestamp: event.timestamp,
          fullEvent: event
        });
      });
    }
  }, [events]);

  const getEventStyle = (event) => {
    const baseClass = "log-entry border-l-4 px-3 py-2 mb-2 rounded-r";
    
    switch (event.level) {
      case 'SUCCESS':
        return `${baseClass} bg-green-50 border-green-400`;
      case 'ERROR':
        return `${baseClass} bg-red-50 border-red-400`;
      case 'WARNING':
        return `${baseClass} bg-yellow-50 border-yellow-400`;
      case 'DEBUG':
        return `${baseClass} bg-gray-50 border-gray-300`;
      default:
        return `${baseClass} bg-blue-50 border-blue-400`;
    }
  };

  const getEventIcon = (event) => {
    // Handle user-friendly messages with status-based icons
    if (event.isUserFriendly || event.status) {
      switch (event.status) {
        case 'success': return '✅';
        case 'failure': return '❌';
        case 'warning': return '⚠️';
      case 'progress': return (
        <div className="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent text-blue-600 rounded-full" role="status" aria-label="loading">
          <span className="sr-only">Loading...</span>
        </div>
      );
      default: return 'ℹ️';
      }
    }

    // Handle regular events
    switch (event.type) {
      case 'connected':
        return '🔗';
      case 'jsonl_found':
        return '📄';
      case 'suite_start':
        return '🚀';
      case 'test_start':
        return '🧪';
      case 'test_complete':
        return event.success ? '✅' : '❌';
      case 'step':
        return '⚙️';
      case 'command_result':
        return event.success ? '✅' : '⚠️';
      case 'complete':
        return event.success ? '🎉' : '🛑';
      case 'results':
        return '📊';
      case 'error':
        return '❌';
      case 'stdout':
        return '📤';
      case 'stderr':
        return '📢';
      // Handle HTTP API event types
      case 'environment_check':
        return '🔍';
      case 'resource_creation':
        return '⚙️';
      case 'connectivity_test':
        return '🔌';
      default:
        return '📝';
    }
  };

  const formatEventMessage = (event) => {
    // Handle user-friendly messages
    if (event.isUserFriendly && event.title) {
      return event.title;
    }

    // Handle different event types with special formatting
    switch (event.type) {
      case 'suite_start':
        return `Starting test suite with ${event.totalTests || 0} tests`;
        
      case 'test_start':
        const testName = event.testName || event.data?.testName;
        return `Starting test: ${testName}`;
        
      case 'test_complete':
        const completedTestName = event.testName || event.data?.testName;
        const duration = event.duration || event.data?.duration;
        const result = event.result || event.data?.result;
        return `Test completed: ${completedTestName} - ${result} ${duration ? `(${duration}s)` : ''}`;
        
      case 'step':
        const stepName = event.stepName || event.data?.stepName;
        return `Step: ${stepName}`;
        
      case 'command_result':
        const exitCode = event.exitCode !== undefined ? event.exitCode : event.data?.exitCode;
        return `Command ${event.success ? 'succeeded' : 'failed'}${exitCode !== undefined ? ` (exit code: ${exitCode})` : ''}`;
        
      case 'stdout':
      case 'stderr':
        return event.data || event.message;

      // Handle HTTP API event types
      case 'environment_check':
      case 'resource_creation':
      case 'connectivity_test':
        return event.title || event.message || `${event.type.replace('_', ' ')} event`;
        
      default:
        return event.message || 'No message';
    }
  };

  const shouldShowDetails = (event) => {
    // Show additional details for certain event types
    return ['test_complete', 'command_result', 'suite_start'].includes(event.type);
  };

  const renderEventDetails = (event) => {
    if (!shouldShowDetails(event) || !event.data) return null;

    switch (event.type) {
      case 'suite_start':
        return (
          <div className="mt-2 text-xs text-gray-600">
            <div>Groups: {event.groups?.join(', ') || 'Unknown'}</div>
            {event.data?.clusterInfo && (
              <div>Cluster: {event.data.clusterInfo.kubernetesVersion} ({event.data.clusterInfo.cniProvider})</div>
            )}
          </div>
        );

      case 'test_complete':
        return (
          <div className="mt-2 text-xs text-gray-600">
            {event.data?.group && <div>Group: {event.data.group}</div>}
            {event.data?.subgroup && <div>Subgroup: {event.data.subgroup}</div>}
          </div>
        );

      case 'command_result':
        return (
          <div className="mt-2 text-xs text-gray-600">
            {event.data?.stdout && (
              <div className="mt-1">
                <strong>Output:</strong>
                <pre className="bg-gray-100 p-1 rounded text-xs overflow-x-auto">
                  {event.data.stdout.substring(0, 200)}{event.data.stdout.length > 200 ? '...' : ''}
                </pre>
              </div>
            )}
            {event.data?.stderr && (
              <div className="mt-1">
                <strong>Error:</strong>
                <pre className="bg-red-100 p-1 rounded text-xs overflow-x-auto">
                  {event.data.stderr.substring(0, 200)}{event.data.stderr.length > 200 ? '...' : ''}
                </pre>
              </div>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="log-viewer">
      <div className="log-header flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900">Live Test Events</h3>
        {isRunning && (
          <div className="flex items-center space-x-2">
            <div className="animate-spin rounded-full h-4 w-4 border-2 border-blue-600 border-t-transparent"></div>
            <span className="text-sm text-blue-600 font-medium">Streaming events...</span>
          </div>
        )}
      </div>
      
      <div className="log-content bg-gray-50 border border-gray-200 rounded-lg p-4 max-h-96 overflow-y-auto">
        {events.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            {isRunning ? (
              <div className="flex items-center justify-center space-x-2">
                <div className="animate-spin rounded-full h-5 w-5 border-2 border-blue-600 border-t-transparent"></div>
                <span>Waiting for test events...</span>
              </div>
            ) : (
              <span>No events yet. Click "Run This Test" to start.</span>
            )}
          </div>
        ) : (
          <div className="space-y-1">
            {events.map((event, index) => (
              <div key={index} className={getEventStyle(event)}>
                <div className="flex items-start space-x-3">
                  <span className="text-lg flex-shrink-0 mt-0.5">
                    {getEventIcon(event)}
                  </span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="font-medium text-sm text-gray-900">
                          {formatEventMessage(event)}
                        </div>
                        
                        {/* User-friendly message details */}
                        {event.isUserFriendly && (
                          <div className="mt-2 space-y-2">
                            {event.description && (
                              <p className="text-sm text-gray-700">{event.description}</p>
                            )}
                            
                            {event.context && (
                              <div className="bg-blue-50 border-l-4 border-blue-200 p-2 rounded">
                                <p className="text-sm text-blue-800">
                                  <span className="font-medium">💡 Context:</span> {event.context}
                                </p>
                              </div>
                            )}
                            
                            {event.hints && event.hints.length > 0 && (
                              <div className="bg-green-50 border-l-4 border-green-200 p-2 rounded">
                                <p className="text-sm text-green-800 font-medium mb-1">🎯 Recommendations:</p>
                                <ul className="text-sm text-green-700 list-disc list-inside space-y-1">
                                  {event.hints.map((hint, i) => (
                                    <li key={i}>{hint}</li>
                                  ))}
                                </ul>
                              </div>
                            )}
                            
                            {event.technicalDetails && (
                              <details className="mt-2">
                                <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                                  📋 Technical Details
                                </summary>
                                <pre className="mt-1 text-xs bg-gray-100 p-2 rounded overflow-x-auto">
                                  {typeof event.technicalDetails === 'string' 
                                    ? event.technicalDetails 
                                    : JSON.stringify(event.technicalDetails, null, 2)}
                                </pre>
                              </details>
                            )}
                          </div>
                        )}
                        
                        {renderEventDetails(event)}
                      </div>
                      <div className="flex-shrink-0 ml-4">
                        <span className="text-xs text-gray-500">
                          {new Date(event.timestamp).toLocaleTimeString()}
                        </span>
                      </div>
                    </div>
                    
                    {/* Context information if available */}
                    {(event.context || event.groupId || event.testId) && (
                      <div className="mt-1 flex flex-wrap gap-2">
                        {event.context && (
                          <span className="inline-block bg-gray-200 text-gray-700 text-xs px-2 py-1 rounded">
                            {event.context}
                          </span>
                        )}
                        {event.groupId && (
                          <span className="inline-block bg-blue-200 text-blue-700 text-xs px-2 py-1 rounded">
                            {event.groupId}
                          </span>
                        )}
                        {event.subgroupId && (
                          <span className="inline-block bg-green-200 text-green-700 text-xs px-2 py-1 rounded">
                            {event.subgroupId}
                          </span>
                        )}
                        {event.testId && (
                          <span className="inline-block bg-purple-200 text-purple-700 text-xs px-2 py-1 rounded">
                            {event.testId}
                          </span>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ))}
            <div ref={logEndRef} />
          </div>
        )}
      </div>

      {events.length > 0 && (
        <div className="mt-2 text-xs text-gray-500 text-center">
          {events.length} events • {isRunning ? 'Live updates enabled' : 'Test completed'}
        </div>
      )}
    </div>
  );
}
