import { useState, useEffect } from 'react';

export default function CiliumConfigModal({ 
  showModal, 
  onCloseModal, 
  onConfigComplete, 
  isRunning,
  setIsRunning 
}) {
  const [configLog, setConfigLog] = useState([]);
  const [configData, setConfigData] = useState(null);
  const [validationData, setValidationData] = useState(null);
  const [insights, setInsights] = useState(null);
  const [error, setError] = useState(null);
  const [activeTab, setActiveTab] = useState('config'); // 'config', 'status'
  const [downloadClicked, setDownloadClicked] = useState(false);

  const startConfigFetch = async () => {
    setIsRunning(true);
    // Reset state before starting
    setConfigLog([]);
    setConfigData(null);
    setInsights(null);
    setError(null);
    setActiveTab('config');

    try {
      const response = await fetch('/api/cilium-config', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          operation: 'config'
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
              handleConfigEvent(eventData);
            } catch (parseError) {
              // Ignore non-JSON lines for clean UI
            }
          }
        }
      }

      if (onConfigComplete) {
        onConfigComplete(true);
      }

    } catch (err) {
      console.error('Cilium config error:', err);
      setError(`Failed to fetch Cilium configuration: ${err.message}`);
      setConfigLog(prev => [...prev, { type: 'error', message: `❌ Error: ${err.message}` }]);
      
      if (onConfigComplete) {
        onConfigComplete(false);
      }
    } finally {
      setIsRunning(false);
    }
  };

  const handleConfigEvent = (eventData) => {
    switch (eventData.type) {
      case 'cilium_complete':
        if (eventData.success && eventData.data) {
          setConfigData(eventData.data.config);
          setInsights(eventData.data.insights);
        }
        break;
      case 'cilium_error':
        if (eventData.error) {
          setError(eventData.error);
        }
        break;
    }
  };

  const runValidation = async (features = []) => {
    setIsRunning(true);
    setActiveTab('status');
    setConfigLog([]);

    try {
      const response = await fetch('/api/cilium-config', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          operation: 'validate',
          features: features
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
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
              handleValidationEvent(eventData);
            } catch (parseError) {
              // Ignore non-JSON lines for clean UI
            }
          }
        }
      }

    } catch (err) {
      console.error('Validation error:', err);
      setError(`Failed to validate features: ${err.message}`);
      setConfigLog(prev => [...prev, { type: 'error', message: `❌ Error: ${err.message}` }]);
    } finally {
      setIsRunning(false);
    }
  };

  const handleValidationEvent = (eventData) => {
    switch (eventData.type) {
      case 'cilium_complete':
        if (eventData.success && eventData.data) {
          setValidationData(eventData.data);
        }
        break;
      case 'cilium_error':
        if (eventData.error) {
          setError(eventData.error);
        }
        break;
    }
  };

  // Smart deduplication function to merge tests with same name
  const deduplicateAndMergeTests = (tests1 = [], tests2 = []) => {
    const testMap = new Map();
    
    // Helper function to add or merge test
    const addOrMergeTest = (test) => {
      const testName = test.name || test.testName || '';
      if (!testName) return;
      
      if (testMap.has(testName)) {
        // Merge with existing test
        const existing = testMap.get(testName);
        
        // Combine reasons intelligently
        const existingReason = existing.reason || existing.rationale || '';
        const newReason = test.reason || test.rationale || '';
        
        let combinedReason = '';
        if (existingReason && newReason && existingReason !== newReason) {
          combinedReason = `${existingReason} • ${newReason}`;
        } else {
          combinedReason = existingReason || newReason;
        }
        
        // Combine descriptions
        const existingDesc = existing.description || existing.summary || '';
        const newDesc = test.description || test.summary || '';
        const combinedDesc = existingDesc || newDesc || 'Validates core networking functionality';
        
        // Update the existing test with merged data
        testMap.set(testName, {
          ...existing,
          ...test, // New test data takes precedence for most fields
          reason: combinedReason,
          rationale: combinedReason, // Ensure both fields are updated
          description: combinedDesc,
          summary: combinedDesc
        });
      } else {
        // Add new test
        testMap.set(testName, { ...test });
      }
    };
    
    // Process all tests from both arrays
    [...tests1, ...tests2].forEach(addOrMergeTest);
    
    // Return deduplicated array
    return Array.from(testMap.values());
  };

  const runRecommendedTests = () => {
    // Smart deduplication: merge tests with same name and combine their reasons
    const allRecommendedTests = deduplicateAndMergeTests(
      validationData?.summary?.recommendedTests || [], 
      insights?.recommendedTests || []
    );
    
    console.log('[CiliumConfigModal] Deduplicated tests:', allRecommendedTests);
    
    if (allRecommendedTests.length > 0) {
      // Extract just the test names (strings) for the batch runner
      const testNames = allRecommendedTests.map(test => test.name || test.testName).filter(Boolean);
      
      console.log('[CiliumConfigModal] Extracted test names:', testNames);
      
      // Close modal and pass clean test names to parent
      onCloseModal();
      if (onConfigComplete) {
        onConfigComplete(true, testNames);
      }
    }
  };

  const downloadCiliumJSON = () => {
    if (!configData) {
      console.warn('[CiliumConfigModal] No config data available for download');
      return;
    }

    // Set clicked state for visual feedback
    setDownloadClicked(true);

    try {
      // Create a comprehensive export with both raw config and metadata
      const exportData = {
        metadata: {
          exportedAt: new Date().toISOString(),
          source: 'k8s-diagnostic-tool',
          version: '1.0'
        },
        ciliumConfig: configData,
        ...(insights && { insights: insights })
      };

      // Convert to pretty-printed JSON
      const jsonString = JSON.stringify(exportData, null, 2);
      
      // Create blob with proper MIME type
      const blob = new Blob([jsonString], { type: 'application/json' });
      
      // Generate filename with timestamp
      const now = new Date();
      const timestamp = now.toISOString().split('T')[0] + '-' + 
                       now.toTimeString().split(' ')[0].replace(/:/g, '-');
      const filename = `cilium-config-${timestamp}.json`;
      
      // Create download link and trigger download
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      
      // Cleanup
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      
      console.log(`[CiliumConfigModal] Successfully downloaded: ${filename}`);
      
      // Reset clicked state after a short delay
      setTimeout(() => {
        setDownloadClicked(false);
      }, 1500);
    } catch (error) {
      console.error('[CiliumConfigModal] Download error:', error);
      alert('Failed to download configuration. Please try again.');
      setDownloadClicked(false);
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

  const formatConfigValue = (value) => {
    if (typeof value === 'boolean') return value ? 'Enabled' : 'Disabled';
    if (value === null || value === undefined) return 'Not Set';
    return value.toString();
  };

  // Helper component for styled config values
  const StyledConfigValue = ({ value }) => {
    const formattedValue = formatConfigValue(value);
    return (
      <span
        style={{ 
          color: '#d97706', // Dark orange color (same as config highlights)
          fontWeight: '600',
          backgroundColor: 'rgba(217, 119, 6, 0.1)', // Light orange background
          padding: '4px 12px',
          borderRadius: '9999px',
          fontSize: '0.875rem',
          fontFamily: 'monospace',
          whiteSpace: 'nowrap',
          flexShrink: '0'
        }}
      >
        {formattedValue}
      </span>
    );
  };

  // Helper functions now use data from the API response instead of hardcoded mappings
  const getFeatureDisplayName = (feature) => {
    // Use displayName from API response if available, otherwise fallback to formatting
    return feature.displayName || feature.name?.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase()) || 'Unknown Feature';
  };

  const getFeatureDescription = (feature) => {
    // Use description from API response if available, otherwise fallback
    return feature.description || 'Advanced Cilium networking feature';
  };

  const getFeatureCategory = (feature) => {
    // Use category from API response if available, otherwise fallback
    return feature.category?.replace(/\b\w/g, l => l.toUpperCase()) || 'General';
  };

  const getStatusIcon = (isEnabled) => {
    return isEnabled ? '✅' : '⚠️';
  };

  const getStatusBadge = (isEnabled) => {
    return isEnabled 
      ? 'bg-green-100 text-green-800' 
      : 'bg-blue-100 text-blue-800';
  };

  const getStatusText = (isEnabled) => {
    return isEnabled ? 'Active & Working' : 'Available';
  };

  const getCategoryIcon = (category) => {
    const icons = {
      'Networking': '🌐',
      'Security': '🔐',
      'Observability': '📊',
      'General': '⚙️'
    };
    return icons[category] || '⚙️';
  };

  // Helper function to highlight config references in rationale text
  const formatRationaleWithHighlights = (rationale) => {
    if (!rationale) return 'Essential for your current configuration';
    
    // Regex to match config references like (enable-l7-proxy=true)
    const configRegex = /\(([^=]+)=([^)]+)\)/g;
    
    // Split text and create elements
    const parts = [];
    let lastIndex = 0;
    let match;
    
    while ((match = configRegex.exec(rationale)) !== null) {
      // Add text before the match
      if (match.index > lastIndex) {
        parts.push(rationale.substring(lastIndex, match.index));
      }
      
      // Add highlighted config reference
      parts.push(
        <span 
          key={match.index}
          style={{ 
            color: '#d97706', // Dark orange color
            fontWeight: '600',
            backgroundColor: 'rgba(217, 119, 6, 0.1)', // Light orange background
            padding: '1px 4px',
            borderRadius: '3px',
            fontFamily: 'monospace'
          }}
        >
          ({match[1]}={match[2]})
        </span>
      );
      
      lastIndex = match.index + match[0].length;
    }
    
    // Add remaining text
    if (lastIndex < rationale.length) {
      parts.push(rationale.substring(lastIndex));
    }
    
    return parts.length > 1 ? parts : rationale;
  };

  const renderConfigSection = (title, configs, icon = '📊') => {
    if (!configs || configs.length === 0) return null;

    return (
      <div className="mb-6">
        <h4 className="font-poppins font-bold text-gray-800 text-lg mb-3 flex items-center">
          <span style={{ marginRight: '5px' }}>{icon}</span>
          {title} ({configs.length})
        </h4>
        <div className="space-y-2">
          {configs.map(([key, value]) => {
            const displayKeyName = key.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
            return (
              <div key={key} className="bg-white rounded-lg border border-gray-200 p-3">
                <div 
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: '12px',
                    minWidth: '0',
                    flexWrap: 'nowrap',
                    width: '100%'
                  }}
                >
                  <div 
                    className="font-inter"
                    style={{
                      fontWeight: '700',
                      color: '#000000',
                      fontSize: '0.875rem',
                      minWidth: '0',
                      flex: '1',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      fontFamily: 'Inter, sans-serif'
                    }}
                    title={displayKeyName}
                  >
                    {displayKeyName}
                  </div>
                  <StyledConfigValue value={value} />
                </div>
              </div>
            );
          })}
        </div>
      </div>
    );
  };

  // Start config fetch when modal is first shown using useEffect
  useEffect(() => {
    if (showModal && !configData && !isRunning && configLog.length === 0) {
      startConfigFetch();
    }
  }, [showModal, configData, isRunning, configLog.length]);

  if (!showModal) {
    return null;
  }

  return (
    <div className="mt-4 modal-container" style={{ width: '860px', maxWidth: '860px' }}>
      <div 
        className="rounded-xl flex flex-col card-shadow-lg modal-container" 
        style={{ 
          backgroundColor: '#fef3c7', 
          padding: '17px', 
          width: '826px', 
          minHeight: '500px',
          position: 'relative'
        }}
      >
        {!isRunning && (
          <button
            onClick={onCloseModal}
            className="text-gray-600 hover:text-gray-800 text-2xl transition-all duration-200"
            style={{ 
              position: 'absolute',
              top: '8px',
              right: '8px',
              zIndex: 10,
              background: 'rgba(255, 255, 255, 0.9)',
              borderRadius: '4px',
              width: '32px',
              height: '32px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              border: '1px solid #d1d5db',
              cursor: 'pointer',
              lineHeight: '1'
            }}
          >
            ✖
          </button>
        )}
        <div className="mb-4">
          <h3 className="text-xl font-comfortaa font-bold text-gray-900 flex items-center">
            <span style={{ marginRight: '5px' }}>🛡️</span>
            Cilium Health Dashboard
          </h3>
          <p className="text-sm font-inter text-gray-600 mt-1">
            Current state of your Cilium installation
          </p>
        </div>

        {error && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
            <div className="flex items-center">
              <span className="text-red-500 text-xl" style={{ marginRight: '5px' }}>❌</span>
              <div>
                <div className="font-inter font-semibold text-red-800">Configuration Error</div>
                <div className="font-inter text-red-700 text-sm">{error}</div>
              </div>
            </div>
          </div>
        )}

        {/* Tab Navigation */}
        <div className="flex border-b border-gray-200 mb-4">
          <button
            onClick={() => setActiveTab('config')}
            className="btn-outline font-comfortaa font-semibold transition-all hover-lift flex items-center"
            style={{ 
              marginRight: '7px', 
              padding: '8px 16px', 
              borderRadius: '5px',
              ...(activeTab === 'config' ? {
                background: 'linear-gradient(135deg, #fbbf24, #f59e0b)',
                color: 'black'
              } : {})
            }}
          >
            <span style={{ marginRight: '5px' }}>📊</span>
            Configuration Overview
          </button>
          <button
            onClick={() => {
              setActiveTab('status');
              if (!validationData || !validationData.summary) {
                runValidation([]);
              }
            }}
            className="btn-outline font-comfortaa font-semibold transition-all hover-lift flex items-center"
            style={{ 
              marginRight: '7px',
              padding: '8px 16px', 
              borderRadius: '5px',
              ...(activeTab === 'status' ? {
                background: 'linear-gradient(135deg, #fbbf24, #f59e0b)',
                color: 'black'
              } : {})
            }}
          >
            <span style={{ marginRight: '5px' }}>⚡</span>
            Feature Status Report
          </button>
          <button
            onClick={downloadCiliumJSON}
            disabled={!configData}
            className="btn-outline font-comfortaa font-semibold transition-all hover-lift flex items-center"
            style={{ 
              padding: '8px 16px', 
              borderRadius: '5px',
              background: !configData ? '#f3f4f6' : 
                         downloadClicked ? 'linear-gradient(135deg, rgb(251, 191, 36), rgb(245, 158, 11))' : '#ffffff',
              color: !configData ? '#9ca3af' :
                     downloadClicked ? 'black' : '#374151',
              cursor: configData ? 'pointer' : 'not-allowed',
              opacity: configData ? 1 : 0.6,
              transform: downloadClicked ? 'scale(0.95)' : 'scale(1)',
              transition: 'all 0.2s ease'
            }}
            title={configData ? 'Download Cilium configuration as JSON' : 'Configuration data not available'}
          >
            <span style={{ marginRight: '5px' }}>📥</span>
            Download JSON
          </button>
        </div>

        {/* Tab Content */}
        <div className="flex-1 min-h-0">
          {activeTab === 'config' && configData && (
            <div className="bg-gray-50 rounded-lg h-full overflow-y-auto test-output-scroll" style={{ padding: '15px' }}>
              {/* Organize config data by category */}
              {(() => {
                // Only process raw config data, exclude validation results
                const rawConfigData = configData.summary ? configData : configData; // Handle both cases
                const configOnly = configData.summary ? configData : configData; // If there's a summary, we have validation data mixed in
                
                // Filter out validation/summary data and only keep simple string/number values
                const entries = Object.entries(rawConfigData)
                  .filter(([key, value]) => {
                    // Exclude validation summary data
                    if (key === 'summary' || key === 'enabledFeatures' || key === 'availableFeatures' || key === 'enabledCount' || key === 'availableCount') {
                      return false;
                    }
                    // Only include simple values (strings, numbers, booleans)
                    return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean';
                  })
                  .sort(([a], [b]) => a.localeCompare(b));

                const coreSettings = entries.filter(([key]) => 
                  ['cluster-name', 'cluster-id', 'kube-proxy-replacement', 'ipam', 'tunnel'].includes(key) ||
                  key.includes('identity') || key.includes('endpoint')
                );
                const networkingSettings = entries.filter(([key]) => 
                  key.includes('network') || key.includes('node-port') || key.includes('bgp') || 
                  key.includes('gateway') || key.includes('l2') || key.includes('route')
                );
                const securitySettings = entries.filter(([key]) => 
                  key.includes('policy') || key.includes('encryption') || key.includes('firewall') ||
                  key.includes('ipsec') || key.includes('wireguard') || key.includes('auth')
                );
                const observabilitySettings = entries.filter(([key]) => 
                  key.includes('monitor') || key.includes('hubble') || key.includes('metrics') ||
                  key.includes('debug') || key.includes('log')
                );
                const otherSettings = entries.filter(([key]) => 
                  !coreSettings.find(([k]) => k === key) &&
                  !networkingSettings.find(([k]) => k === key) &&
                  !securitySettings.find(([k]) => k === key) &&
                  !observabilitySettings.find(([k]) => k === key)
                );

                return (
                  <div>
                    {renderConfigSection('Core Settings', coreSettings, '🔧')}
                    {renderConfigSection('Networking Features', networkingSettings, '🌐')}
                    {renderConfigSection('Security Features', securitySettings, '🔐')}
                    {renderConfigSection('Observability Features', observabilitySettings, '📊')}
                    {renderConfigSection('Other Settings', otherSettings, '⚙️')}
                  </div>
                );
              })()}
            </div>
          )}

          {activeTab === 'status' && (
            <div className="bg-gray-50 rounded-lg h-full overflow-y-auto test-output-scroll" style={{ padding: '15px' }}>
              {/* Display validation results if available */}
              {validationData && validationData.summary ? (
                <div className="space-y-6">
                  {/* Overall Status Banner */}
                  <div className="bg-gradient-to-r from-green-50 to-blue-50 rounded-lg p-4 border border-gray-200">
                    <h4 className="font-poppins font-bold text-gray-800 text-xl mb-2 flex items-center">
                      <span style={{ marginRight: '5px' }}>🛡️</span>
                      Cilium Feature Status
                    </h4>
                    <p className="font-inter text-gray-700">
                      Your Cilium installation is working properly. Below are the current feature configurations.
                    </p>
                  </div>

                  {/* Quick Stats */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-green-50 border border-green-200 rounded-lg p-4">
                      <div className="flex items-center">
                        <span className="text-green-600 text-xl" style={{ marginRight: '5px' }}>✅</span>
                        <span className="font-inter text-green-700 text-sm" style={{ marginRight: '7px' }}>Active Features:</span>
                        <span className="font-poppins font-bold text-green-800 text-2xl">
                          {(validationData && validationData.summary && validationData.summary.enabledCount) || 
                           (insights && insights.enabledFeatures && insights.enabledFeatures.length) || 0}
                        </span>
                      </div>
                    </div>
                    <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                      <div className="flex items-center">
                        <span className="text-blue-600 text-xl" style={{ marginRight: '5px' }}>⚠️</span>
                        <span className="font-inter text-blue-700 text-sm" style={{ marginRight: '7px' }}>Available to Enable:</span>
                        <span className="font-poppins font-bold text-blue-800 text-2xl">
                          {(validationData && validationData.summary && validationData.summary.availableCount) || 0}
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Active Features Section */}
                  {((validationData && validationData.summary && validationData.summary.enabledFeatures && validationData.summary.enabledFeatures.length > 0) ||
                    (insights && insights.enabledFeatures && insights.enabledFeatures.length > 0)) && (
                    <div>
                      <h4 className="font-poppins font-bold text-gray-800 text-lg mb-4 flex items-center">
                        <span className="text-green-600 text-lg" style={{ marginRight: '5px' }}>✅</span>
                        Active Features ({((validationData && validationData.summary && validationData.summary.enabledFeatures && validationData.summary.enabledFeatures.length) || 
                                         (insights && insights.enabledFeatures && insights.enabledFeatures.length) || 0)})
                      </h4>
                      <div className="space-y-4" style={{ marginTop: '15px' }}>
                        {/* Show validation data if available, otherwise show insights data */}
                        {(validationData && validationData.summary && validationData.summary.enabledFeatures ? 
                          validationData.summary.enabledFeatures : 
                          (insights && insights.enabledFeatures ? insights.enabledFeatures.map((featureName) => ({ displayName: featureName })) : [])
                        ).map((feature, index) => (
                          <div key={index} style={{ marginBottom: '15px' }}>
                            {/* Line 1: Emoji + Feature Name + Category (all bold) */}
                            <div className="font-inter font-black text-gray-800 text-base" style={{ marginBottom: '2px', fontWeight: '900' }}>
                              <span className="text-green-600" style={{ marginRight: '5px' }}>✅</span>
                              {getFeatureDisplayName(feature)} - {getFeatureCategory(feature)}
                            </div>
                            {/* Line 2: Description */}
                            <div className="font-inter text-gray-600 text-sm" style={{ marginBottom: '2px' }}>
                              {getFeatureDescription(feature)}
                            </div>
                            {/* Line 3: Configuration (for active features, show what's enabled) */}
                            <div className="font-inter text-sm" style={{ marginBottom: '2px' }}>
                              <span className="font-black" style={{ fontWeight: '900' }}>Status: </span>
                              <span style={{
                                color: 'rgb(34, 197, 94)',
                                fontWeight: '600',
                                backgroundColor: 'rgba(34, 197, 94, 0.1)',
                                padding: '4px 12px',
                                borderRadius: '9999px',
                                fontSize: '0.875rem',
                                fontFamily: 'monospace',
                                whiteSpace: 'nowrap',
                                flexShrink: '0'
                              }}>
                                Enabled and Active
                              </span>
                            </div>
                            {/* Line 4: Working Status */}
                            <div className="font-inter text-green-600 text-sm">
                              Active & Working
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Available Features Section */}
                  {validationData.summary.availableFeatures && validationData.summary.availableFeatures.length > 0 && (
                    <div>
                      <h4 className="font-poppins font-bold text-gray-800 text-lg mb-4 flex items-center">
                        <span className="text-blue-600 text-lg" style={{ marginRight: '5px' }}>⚠️</span>
                        Available Features ({validationData.summary.availableFeatures.length})
                      </h4>
                      <div className="bg-blue-50 border border-blue-100 rounded-lg p-3 mb-4">
                        <div className="text-blue-800 text-sm flex items-center">
                          <span style={{ marginRight: '5px' }}>ℹ️</span>
                          These features are not currently enabled but are available for configuration if needed.
                        </div>
                      </div>
                      <div className="space-y-4" style={{ marginTop: '15px' }}>
                        {validationData.summary.availableFeatures.map((feature, index) => (
                          <div key={index} style={{ marginBottom: '15px' }}>
                            {/* Line 1: Emoji + Feature Name + Category (all bold) */}
                            <div className="font-inter font-black text-gray-800 text-base" style={{ marginBottom: '2px', fontWeight: '900' }}>
                              <span className="text-blue-600" style={{ marginRight: '5px' }}>⚠️</span>
                              {getFeatureDisplayName(feature)} - {getFeatureCategory(feature)}
                            </div>
                            {/* Line 2: Description */}
                            <div className="font-inter text-gray-600 text-sm" style={{ marginBottom: '2px' }}>
                              {getFeatureDescription(feature)}
                            </div>
                            {/* Line 3: Requirements (styled) */}
                            {feature.requirement && (
                              <div className="font-inter text-sm" style={{ marginBottom: '2px' }}>
                                <span className="font-black" style={{ fontWeight: '900' }}>Requires: </span>
                                <span style={{
                                  color: 'rgb(217, 119, 6)',
                                  fontWeight: '600',
                                  backgroundColor: 'rgba(217, 119, 6, 0.1)',
                                  padding: '4px 12px',
                                  borderRadius: '9999px',
                                  fontSize: '0.875rem',
                                  fontFamily: 'monospace',
                                  whiteSpace: 'nowrap',
                                  flexShrink: '0'
                                }}>
                                  {feature.requirement}
                                </span>
                              </div>
                            )}
                            {/* Line 4: Status */}
                            <div className="font-inter text-sm">
                              <span style={{
                                color: 'rgb(239, 68, 68)',
                                fontWeight: '600',
                                backgroundColor: 'rgba(239, 68, 68, 0.1)',
                                padding: '4px 12px',
                                borderRadius: '9999px',
                                fontSize: '0.875rem',
                                fontFamily: 'monospace',
                                whiteSpace: 'nowrap',
                                flexShrink: '0'
                              }}>
                                Not Enabled
                              </span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Status Summary */}
                  <div className="bg-gray-50 border border-gray-200 rounded-lg p-4" style={{ marginBottom: '15px' }}>
                    <h4 className="font-poppins font-bold text-gray-800 text-lg mb-3 flex items-center">
                      <span style={{ marginRight: '5px' }}>📋</span>
                      Summary
                    </h4>
                    <div className="space-y-2">
                      <div className="flex items-center text-sm">
                        <span className="text-green-600" style={{ marginRight: '5px' }}>✅</span>
                        <span className="font-inter text-gray-700">
                          {(validationData && validationData.summary && validationData.summary.enabledCount) || 
                           (insights && insights.enabledFeatures && insights.enabledFeatures.length) || 0} features are active and working properly
                        </span>
                      </div>
                      <div className="flex items-center text-sm">
                        <span className="text-blue-600" style={{ marginRight: '5px' }}>⚠️</span>
                        <span className="font-inter text-gray-700">
                          {(validationData && validationData.summary && validationData.summary.availableCount) || 0} additional features can be enabled if needed
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Recommended Tests */}
                  {((validationData && validationData.summary && validationData.summary.recommendedTests && validationData.summary.recommendedTests.length > 0) || 
                    (insights && insights.recommendedTests && insights.recommendedTests.length > 0)) && (
                    <div className="border-t border-gray-200 pt-4">
                      {/* Test Recommendations List */}
                      <div className="mb-4">
                        <h4 className="font-poppins font-bold text-gray-800 text-lg mb-4 flex items-center">
                          <span className="text-blue-600 text-lg" style={{ marginRight: '5px' }}>📋</span>
                          Recommended Tests
                        </h4>
                        <div className="bg-blue-50 border border-blue-100 rounded-lg p-3" style={{ marginBottom: '15px' }}>
                          <div className="text-blue-800 text-sm flex items-center">
                            <span style={{ marginRight: '5px' }}>⚠️</span>
                            Based on your current Cilium configuration, we recommend running these tests to validate your setup:
                          </div>
                        </div>
                        <div>
                          {deduplicateAndMergeTests(
                            validationData?.summary?.recommendedTests || [], 
                            insights?.recommendedTests || []
                          ).map((test, index) => (
                            <div key={index} style={{ marginBottom: '15px' }}>
                              {/* Line 1: Icon + Test Name */}
                              <div style={{ display: 'flex', alignItems: 'center', paddingLeft: '0px !important', marginLeft: '0px !important' }}>
                                <span className="text-blue-600 text-lg" style={{ marginRight: '5px', paddingLeft: '0px !important' }}>🔧</span>
                                <span 
                                  className="font-inter font-bold text-gray-800 text-base" 
                                  style={{ 
                                    paddingLeft: '0px !important',
                                    fontWeight: ['basic-http-get', 'http-with-headers'].includes(test.name || test.testName) ? '900' : '600'
                                  }}
                                >
                                  {test.name || test.testName || `Test ${index + 1}`}
                                </span>
                              </div>
                              {/* Line 2: Description + Reason with highlighting - Forced no indentation */}
                              <div style={{ 
                                display: 'flex', 
                                alignItems: 'center', 
                                fontSize: '0.875rem',
                                paddingLeft: '0px !important',
                                marginLeft: '0px !important',
                                textIndent: '0px !important'
                              }}>
                                <span className="font-inter text-gray-600" style={{ marginRight: '8px', paddingLeft: '0px !important' }}>
                                  {test.description || test.summary || 'Validates core networking functionality'}
                                </span>
                                <span className="font-inter text-gray-700" style={{ paddingLeft: '0px !important' }}>
                                  {formatRationaleWithHighlights(test.reason || test.rationale)}
                                </span>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>

                      {/* Action Button */}
                      <div className="flex gap-3">
                        <button
                          onClick={runRecommendedTests}
                          className="surprise-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                          style={{ padding: '12px 24px', borderRadius: '5px' }}
                        >
                          🚀 Run Recommended Tests ({deduplicateAndMergeTests(
                            validationData?.summary?.recommendedTests || [], 
                            insights?.recommendedTests || []
                          ).length})
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-center py-8">
                  <div className="font-comfortaa text-gray-600 mb-2">
                    {isRunning ? 'Checking feature status...' : 'Click this tab to load feature status'}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Loading state for config tab */}
          {activeTab === 'config' && !configData && (
            <div className="bg-gray-50 rounded-lg h-full flex items-center justify-center" style={{ padding: '15px' }}>
              <div className="text-center">
                <div className="font-comfortaa text-gray-600 mb-2">
                  {isRunning ? 'Fetching Cilium configuration...' : 'Loading configuration data...'}
                </div>
                {configLog.length > 0 && (
                  <div className="mt-4 space-y-2">
                    {configLog.map((log, index) => (
                      <div key={index} className="font-inter text-sm">
                        <span className={getLogMessageClass(log.type)}>{log.message}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        {isRunning && (
          <div className="mt-4">
            <div className="bg-gray-200 rounded-full h-2">
              <div className="bg-gradient-to-r from-blue-500 to-purple-500 h-2 rounded-full animate-pulse" style={{ width: '60%' }}></div>
            </div>
            <div className="text-center mt-2 font-comfortaa text-sm text-gray-600">
              Processing
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
  );
}
