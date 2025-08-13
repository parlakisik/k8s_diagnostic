export default function ProgressIndicator({ 
  currentTest, 
  progress, 
  currentStep, 
  totalTests, 
  completedTests,
  isRunning = false 
}) {
  return (
    <div className="progress-container p-4 bg-blue-50 rounded-lg border border-blue-200">
      {/* Overall Progress Bar */}
      {totalTests > 0 && (
        <div className="mb-4">
          <div className="flex justify-between items-center mb-2">
            <span className="text-sm font-medium text-blue-900">Overall Progress</span>
            <span className="text-sm text-blue-700">
              {completedTests}/{totalTests} tests ({Math.round(progress)}%)
            </span>
          </div>
          <div className="w-full bg-blue-200 rounded-full h-3">
            <div 
              className="bg-blue-600 h-3 rounded-full transition-all duration-300 progress-bar"
              style={{ width: `${progress}%` }}
            ></div>
          </div>
        </div>
      )}

      {/* Current Status */}
      <div className="current-status space-y-2">
        {currentTest && (
          <div className="flex items-center space-x-2">
            <span className="text-sm font-medium text-blue-900">Current Test:</span>
            <span className="text-sm text-blue-800 font-mono">{currentTest}</span>
            {isRunning && (
              <div className="animate-spin rounded-full h-3 w-3 border-2 border-blue-600 border-t-transparent"></div>
            )}
          </div>
        )}
        
        {currentStep && (
          <div className="flex items-center space-x-2">
            <span className="text-sm font-medium text-blue-900">Current Step:</span>
            <span className="text-sm text-blue-800">{currentStep}</span>
          </div>
        )}
        
        {/* Time-based progress indicator */}
        {isRunning && (
          <div className="flex items-center space-x-2 text-xs text-blue-600">
            <div className="animate-spin inline-block w-3 h-3 border border-current border-t-transparent text-blue-600 rounded-full" role="status" aria-label="loading">
              <span className="sr-only">Loading...</span>
            </div>
            <span>Test execution in progress...</span>
          </div>
        )}
      </div>

      {/* Progress Summary for Completed Tests */}
      {!isRunning && totalTests > 0 && completedTests === totalTests && (
        <div className="mt-3 pt-3 border-t border-blue-300">
          <div className="flex items-center space-x-2">
            <span className="text-green-600">✅</span>
            <span className="text-sm text-green-800 font-medium">
              All tests completed ({completedTests}/{totalTests})
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
