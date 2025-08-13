export default function ResultsViewer({ results }) {
  if (!results) return null;

  const getStatusColor = (success) => {
    return success ? 'text-green-600' : 'text-red-600';
  };

  const getStatusBadge = (success) => {
    return success 
      ? 'bg-green-100 text-green-800 px-2 py-1 rounded-full text-xs font-semibold'
      : 'bg-red-100 text-red-800 px-2 py-1 rounded-full text-xs font-semibold';
  };

  return (
    <div className="results-viewer">
      <div className="results-header mb-6">
        <h3 className="text-xl font-semibold text-gray-900 mb-4">📊 Test Results</h3>
        
        {/* Overall Summary */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <div className="text-2xl font-bold text-blue-600">{results.total_tests || 0}</div>
            <div className="text-sm text-blue-700">Total Tests</div>
          </div>
          <div className="bg-green-50 border border-green-200 rounded-lg p-4">
            <div className="text-2xl font-bold text-green-600">{results.passed_tests || 0}</div>
            <div className="text-sm text-green-700">Passed</div>
          </div>
          <div className="bg-red-50 border border-red-200 rounded-lg p-4">
            <div className="text-2xl font-bold text-red-600">{results.failed_tests || 0}</div>
            <div className="text-sm text-red-700">Failed</div>
          </div>
          <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
            <div className="text-2xl font-bold text-gray-600">
              {results.success_rate ? `${results.success_rate.toFixed(1)}%` : '0%'}
            </div>
            <div className="text-sm text-gray-700">Success Rate</div>
          </div>
        </div>

        {/* Test Configuration */}
        {results.test_configuration && (
          <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 mb-6">
            <h4 className="font-semibold text-gray-900 mb-2">Test Configuration</h4>
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <span className="text-gray-600">Namespace:</span>
                <span className="ml-2 font-mono">{results.test_configuration.namespace}</span>
              </div>
              <div>
                <span className="text-gray-600">Verbose Mode:</span>
                <span className="ml-2">{results.test_configuration.verbose_mode ? 'Yes' : 'No'}</span>
              </div>
              {results.test_configuration.test_group && (
                <div>
                  <span className="text-gray-600">Test Group:</span>
                  <span className="ml-2 font-mono">{results.test_configuration.test_group}</span>
                </div>
              )}
              {results.test_configuration.test_list && results.test_configuration.test_list.length > 0 && (
                <div className="col-span-2">
                  <span className="text-gray-600">Tests:</span>
                  <span className="ml-2 font-mono">{results.test_configuration.test_list.join(', ')}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Infrastructure Information */}
        {results.infrastructure && (
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6">
            <h4 className="font-semibold text-blue-900 mb-2">Cluster Infrastructure</h4>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
              <div>
                <span className="text-blue-700">Kubernetes:</span>
                <div className="font-mono">{results.infrastructure.kubernetes_version || 'Unknown'}</div>
              </div>
              <div>
                <span className="text-blue-700">CNI:</span>
                <div className="font-mono">{results.infrastructure.cni_provider || 'Unknown'}</div>
              </div>
              <div>
                <span className="text-blue-700">Platform:</span>
                <div className="font-mono">{results.infrastructure.platform || 'Unknown'}</div>
              </div>
              <div>
                <span className="text-blue-700">Nodes:</span>
                <div className="font-mono">{results.infrastructure.node_count || 0}</div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Individual Test Results */}
      {results.tests && results.tests.length > 0 && (
        <div className="test-results">
          <h4 className="font-semibold text-gray-900 mb-4">Individual Test Results</h4>
          
          <div className="space-y-4">
            {results.tests.map((test, index) => (
              <div key={index} className={`border rounded-lg p-4 ${
                test.success ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50'
              }`}>
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <div className="flex items-center space-x-3 mb-2">
                      <h5 className="font-semibold text-gray-900">{test.name}</h5>
                      <span className={getStatusBadge(test.success)}>
                        {test.success ? 'PASSED' : 'FAILED'}
                      </span>
                    </div>
                    <p className={`text-sm ${getStatusColor(test.success)}`}>
                      {test.message}
                    </p>
                  </div>
                  <div className="text-right text-sm text-gray-500">
                    <div>Duration: {test.duration ? `${test.duration.toFixed(1)}s` : 'N/A'}</div>
                    {test.start_time && (
                      <div>{new Date(test.start_time).toLocaleTimeString()}</div>
                    )}
                  </div>
                </div>

                {/* Expected vs Actual */}
                {(test.expected_outcome || test.actual_outcome) && (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3 text-sm">
                    {test.expected_outcome && (
                      <div>
                        <span className="text-gray-600 font-medium">Expected:</span>
                        <div className="bg-white p-2 rounded border text-gray-700 font-mono text-xs">
                          {test.expected_outcome}
                        </div>
                      </div>
                    )}
                    {test.actual_outcome && (
                      <div>
                        <span className="text-gray-600 font-medium">Actual:</span>
                        <div className="bg-white p-2 rounded border text-gray-700 font-mono text-xs">
                          {test.actual_outcome}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {/* Test Details */}
                {test.details && test.details.length > 0 && (
                  <div className="mb-3">
                    <span className="text-gray-600 font-medium text-sm">Details:</span>
                    <ul className="list-disc list-inside text-sm text-gray-700 mt-1 space-y-1">
                      {test.details.map((detail, detailIndex) => (
                        <li key={detailIndex}>{detail}</li>
                      ))}
                    </ul>
                  </div>
                )}

                {/* Error Details for Failed Tests */}
                {!test.success && test.error_details && test.error_details.length > 0 && (
                  <div className="mb-3">
                    <span className="text-red-600 font-medium text-sm">Error Details:</span>
                    <div className="bg-red-100 border border-red-200 rounded p-2 mt-1">
                      <ul className="list-disc list-inside text-sm text-red-800 space-y-1">
                        {test.error_details.map((error, errorIndex) => (
                          <li key={errorIndex}>{error}</li>
                        ))}
                      </ul>
                    </div>
                  </div>
                )}

                {/* Commands Executed */}
                {test.commands_executed && test.commands_executed.length > 0 && (
                  <details className="text-sm">
                    <summary className="cursor-pointer text-gray-600 hover:text-gray-800 font-medium">
                      Commands Executed ({test.commands_executed.length})
                    </summary>
                    <div className="mt-2 space-y-2">
                      {test.commands_executed.map((cmd, cmdIndex) => (
                        <div key={cmdIndex} className="bg-white p-2 rounded border">
                          <div className="flex items-center justify-between mb-1">
                            <code className="text-xs text-gray-700">{cmd.command}</code>
                            <span className={`text-xs px-2 py-1 rounded ${
                              cmd.success ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                            }`}>
                              Exit: {cmd.exit_code}
                            </span>
                          </div>
                          {cmd.stdout && (
                            <pre className="text-xs bg-gray-100 p-1 rounded mt-1 overflow-x-auto">
                              {cmd.stdout.substring(0, 200)}{cmd.stdout.length > 200 ? '...' : ''}
                            </pre>
                          )}
                        </div>
                      ))}
                    </div>
                  </details>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Overall Health Status */}
      {results.overall_health_status && (
        <div className="mt-6 text-center">
          <div className={`inline-flex items-center px-4 py-2 rounded-lg font-semibold ${
            results.overall_health_status === 'HEALTHY' 
              ? 'bg-green-100 text-green-800' 
              : results.overall_health_status === 'WARNING'
              ? 'bg-yellow-100 text-yellow-800'
              : 'bg-red-100 text-red-800'
          }`}>
            <span className="mr-2">
              {results.overall_health_status === 'HEALTHY' ? '✅' : 
               results.overall_health_status === 'WARNING' ? '⚠️' : '❌'}
            </span>
            Overall Status: {results.overall_health_status}
          </div>
        </div>
      )}

      {/* Timestamp */}
      {results.timestamp && (
        <div className="mt-4 text-center text-xs text-gray-500">
          Test completed at: {new Date(results.timestamp).toLocaleString()}
        </div>
      )}
    </div>
  );
}
