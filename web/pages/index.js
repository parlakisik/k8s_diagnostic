import { useState, useEffect } from 'react';
import Head from 'next/head';
import DiagnosticQuestions from '../components/DiagnosticQuestions';
import BatchTestRunner from '../components/BatchTestRunner';
import CleanupButton from '../components/CleanupButton';
import CiliumConfigButton from '../components/CiliumConfigButton';
import CiliumConfigModal from '../components/CiliumConfigModal';

export default function Home() {
  const [testQueue, setTestQueue] = useState([]);
  const [currentView, setCurrentView] = useState('questions'); // 'questions' or 'batch_runner'
  const [testsComplete, setTestsComplete] = useState(false);
  const [showCustomPicker, setShowCustomPicker] = useState(false);
  const [customSelectedTests, setCustomSelectedTests] = useState(new Set());
  const [copyFeedback, setCopyFeedback] = useState('');
  const [showCiliumModal, setShowCiliumModal] = useState(false);
  const [ciliumIsRunning, setCiliumIsRunning] = useState(false);
  const [showScrollTop, setShowScrollTop] = useState(false);

  const handleTestQueueChange = (newQueue) => {
    setTestQueue(newQueue);
  };

  const handleRunSelectedTests = () => {
    console.log('[Index] Run Selected Tests button clicked!');
    
    // Check if we're in custom picker mode with selected tests
    if (showCustomPicker && customSelectedTests.size > 0) {
      console.log('[Index] Running custom selected tests:', Array.from(customSelectedTests));
      setTestQueue(Array.from(customSelectedTests));
      setCurrentView('batch_runner');
      setTestsComplete(false);
      setShowCustomPicker(false);
    } else if (testQueue.length > 0) {
      console.log('[Index] Running diagnostic questions tests:', testQueue);
      setCurrentView('batch_runner');
      setTestsComplete(false);
    } else {
      console.log('[Index] No tests selected - not switching views');
    }
  };

  const handleBackToQuestions = () => {
    setCurrentView('questions');
  };

  const handleBatchTestComplete = (success) => {
    console.log(`Batch tests completed with ${success ? 'success' : 'some failures'}`);
    setTestsComplete(true);
  };

  const handleCleanupComplete = (success) => {
    console.log(`Cleanup completed with ${success ? 'success' : 'failure'}`);
  };

  const handleConfigComplete = (success, recommendedTests) => {
    console.log(`Cilium config check completed with ${success ? 'success' : 'failure'}`);
    
    // If recommended tests are provided, set them as the test queue
    if (success && recommendedTests && recommendedTests.length > 0) {
      console.log('[Index] Setting recommended tests from Cilium config:', recommendedTests);
      setTestQueue(recommendedTests);
      setCurrentView('batch_runner');
      setTestsComplete(false);
      setShowCustomPicker(false);
    }
  };

  const handleCiliumConfigClick = () => {
    setShowCiliumModal(true);
  };

  const handleCloseCiliumModal = () => {
    setShowCiliumModal(false);
  };

  const handleSurpriseMe = () => {
    console.log('[Index] Surprise Me button clicked!');
    
    // The curated "Surprise Me" test suite (10 tests)
    const surpriseMeTests = [
      'pod-to-pod-cross-node',    // 1. pod-to-pod - Same-node pod reachability
      'service-clusterip',        // 2. service-to-pod - ClusterIP load-balancing
      'pod-to-pod-same-node',     // 3. cross-node - Inter-node routing
      'dns-resolution',           // 4. dns - CoreDNS + Cilium FQDN engine
      'service-loadbalancer',     // 5. loadbalancer - External LB health-checks
      'deny-all',                 // 6. rejecting-all-pods - Cluster-wide deny-all
      'endpoints-label',          // 7. endpoints-label - L3 policy using label selectors
      'cidr-egress',              // 8. cidr-egress - L3 policy using CIDR ranges
      'tcp-port-egress',          // 9. tcp-port-egress - L4 single-port restriction
      'basic-http-get'            // 10. basic-http-get - L7 HTTP method/path enforcement
    ];
    
    console.log('[Index] Setting surprise me tests:', surpriseMeTests);
    
    // Set the test queue and immediately start the tests
    setTestQueue(surpriseMeTests);
    setCurrentView('batch_runner');
    setTestsComplete(false);
  };

  // All available tests organized by 4 main categories
  const ALL_AVAILABLE_TESTS = {
    'Networking': {
      color: 'networking',
      tests: ['pod-to-pod-cross-node', 'pod-to-pod-same-node', 'service-clusterip', 'service-nodeport', 'service-loadbalancer', 'service-cross-node', 'dns-resolution']
    },
    'L3 Policies': {
      color: 'l3',
      tests: ['cidr-ingress', 'cidr-egress', 'cidr-except', 'endpoints-label', 'entities-based', 'dns-based', 'node-selector', 'pod-node-name', 'node-cidr', 'node-based', 'kubernetes-service', 'allow-all', 'deny-all']
    },
    'L4 Policies': {
      color: 'l4',
      tests: ['tcp-port-ingress', 'tcp-port-egress', 'port-range', 'multiple-port', 'icmp-type', 'icmpv6-type', 'mixed-icmp', 'basic-sni', 'multi-domain-sni', 'combined-l4-sni']
    },
    'L7 Policies': {
      color: 'l7',
      tests: ['basic-http-get', 'http-with-headers', 'path-method', 'dns-matchname', 'dns-matchpattern']
    }
  };

  // Get all unique tests (deduplicated)
  const getAllUniqueTests = () => {
    const allTests = new Set();
    Object.values(ALL_AVAILABLE_TESTS).forEach(category => {
      category.tests.forEach(test => allTests.add(test));
    });
    return Array.from(allTests).sort();
  };

  // Get test color for individual tests
  const getTestColor = (testName) => {
    for (const [categoryName, categoryData] of Object.entries(ALL_AVAILABLE_TESTS)) {
      if (categoryData.tests.includes(testName)) {
        return categoryData.color;
      }
    }
    return 'networking'; // fallback
  };

  // Get color class for styling
  const getColorClass = (color) => {
    const colorMap = {
      'networking': 'test-card-networking',
      'l3': 'test-card-l3',
      'l4': 'test-card-l4', 
      'l7': 'test-card-l7',
      'dns': 'test-card-dns',
      'infrastructure': 'test-card-infrastructure'
    };
    return colorMap[color] || 'test-card-networking';
  };

  const handleCustomTestPicker = () => {
    setShowCustomPicker(!showCustomPicker);
    // If opening, clear previous selection
    if (!showCustomPicker) {
      setCustomSelectedTests(new Set());
    }
  };

  const toggleCustomTest = (testName) => {
    setCustomSelectedTests(prev => {
      const newSet = new Set(prev);
      if (newSet.has(testName)) {
        newSet.delete(testName);
      } else {
        newSet.add(testName);
      }
      return newSet;
    });
  };

  const handleCustomSelectAll = () => {
    const allTests = getAllUniqueTests();
    setCustomSelectedTests(new Set(allTests));
  };

  const handleCustomClearAll = () => {
    setCustomSelectedTests(new Set());
  };

  const handleRunCustomTests = () => {
    if (customSelectedTests.size > 0) {
      setTestQueue(Array.from(customSelectedTests));
      setCurrentView('batch_runner');
      setTestsComplete(false);
      setShowCustomPicker(false);
    }
  };

  const copyToClipboard = async (text) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopyFeedback('✅ Copied!');
      setTimeout(() => setCopyFeedback(''), 2000);
    } catch (err) {
      setCopyFeedback('❌ Failed to copy');
      setTimeout(() => setCopyFeedback(''), 2000);
    }
  };

  // Scroll tracking effect
  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
      setShowScrollTop(scrollTop > 300);
    };

    // Throttle scroll events for better performance
    let ticking = false;
    const throttledHandleScroll = () => {
      if (!ticking) {
        requestAnimationFrame(() => {
          handleScroll();
          ticking = false;
        });
        ticking = true;
      }
    };

    window.addEventListener('scroll', throttledHandleScroll);
    return () => window.removeEventListener('scroll', throttledHandleScroll);
  }, []);

  // Scroll to top function
  const scrollToTop = () => {
    window.scrollTo({
      top: 0,
      behavior: 'smooth'
    });
  };

  return (
    <>
      <Head>
        <title>K8s Diagnostic Dashboard</title>
        <meta name="description" content="Kubernetes network diagnostics dashboard" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <link rel="icon" href="/favicon.ico" />
      </Head>

      <main className="min-h-screen bg-white">
        {currentView === 'questions' ? (
          <div>
            {/* Header with Cleanup Button */}
            <div className="bg-white border-b border-gray-200 px-6 py-4" style={{ marginBottom: '10px' }}>
              <div className="max-w-6xl mx-auto flex items-center justify-between">
                <div>
                  <h1 className="text-2xl font-poppins font-bold text-gray-900">
                    K8s Diagnostic Dashboard
                  </h1>
                  <p className="text-sm font-inter text-gray-600 mt-1">
                    Build your custom test suite by answering diagnostic questions
                  </p>
                </div>
                <div className="flex">
                  <button
                    onClick={handleSurpriseMe}
                    className="surprise-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                    style={{ padding: '15px', borderRadius: '5px', marginRight: '7px' }}
                    title="Run a curated set of 10 essential diagnostic tests"
                  >
                    ✨ Surprise Me (10 tests) ✨
                  </button>
                  <button
                    onClick={handleCustomTestPicker}
                    className="custom-picker-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                    style={{ padding: '15px', borderRadius: '5px', marginRight: '7px' }}
                    title="Choose your own custom test selection"
                  >
                    🎯 I will pick the test myself
                  </button>
                  <CiliumConfigButton 
                    onConfigComplete={handleConfigComplete}
                    disabled={false}
                    isRunning={ciliumIsRunning}
                    onConfigClick={handleCiliumConfigClick}
                  />
                  <CleanupButton 
                    onCleanupComplete={handleCleanupComplete}
                    disabled={false}
                  />
                </div>
              </div>
            </div>

            {/* Custom Test Picker Modal - Right below header */}
            {showCustomPicker && (
              <div className="bg-white border-b border-gray-200 px-6 py-4">
                <h3 className="font-poppins font-semibold text-gray-900 text-lg mb-4">
                  📋 Selected Test Queue ({customSelectedTests.size} tests)
                </h3>
                
                {/* Action Buttons */}
                <div className="flex flex-wrap gap-2 mb-4">
                  <button
                    onClick={handleCustomSelectAll}
                    className="cleanup-btn rounded-xl font-comfortaa font-semibold transition-all hover-lift card-shadow"
                    style={{
                      padding: '8px 16px',
                      borderRadius: '5px',
                      marginRight: '7px',
                      marginBottom: '7px'
                    }}
                    title="Select all available tests"
                  >
                    ✅ Select All ({getAllUniqueTests().length})
                  </button>
                  <button
                    onClick={handleCustomClearAll}
                    className="font-comfortaa font-semibold hover-lift transition-all border-2 border-black bg-red-50 hover:bg-red-100 text-gray-800"
                    style={{
                      padding: '8px 16px',
                      borderRadius: '5px',
                      marginRight: '7px',
                      marginBottom: '7px'
                    }}
                    title="Clear all selected tests"
                  >
                    🗑️ Clear All
                  </button>
                  <button
                    onClick={() => setShowCustomPicker(false)}
                    className="cancel-btn font-comfortaa font-semibold hover-lift transition-all rounded-xl card-shadow"
                    style={{
                      padding: '8px 16px',
                      borderRadius: '5px',
                      marginRight: '7px',
                      marginBottom: '7px'
                    }}
                    title="Close test picker"
                  >
                    Cancel
                  </button>
                </div>

                {/* Test Categories */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                  {Object.entries(ALL_AVAILABLE_TESTS).map(([categoryName, categoryData]) => (
                    <div key={categoryName} className="bg-gray-50 rounded-lg p-4">
                      <h4 className="font-poppins font-semibold text-gray-800 text-sm mb-3">
                        {categoryName} ({categoryData.tests.length})
                      </h4>
                      <div className="flex flex-col gap-2">
                        {categoryData.tests.map(testName => (
                          <button
                            key={testName}
                            onClick={() => toggleCustomTest(testName)}
                            className={`font-comfortaa font-semibold hover-lift transition-all border-2 border-black w-full text-left ${
                              customSelectedTests.has(testName)
                                ? `${getColorClass(categoryData.color)} border-gray-400 text-gray-800`
                                : 'bg-white border-gray-300 text-gray-600 hover:border-gray-400'
                            }`}
                            style={{
                              padding: '5px',
                              borderRadius: '5px',
                              fontSize: '0.875rem',
                              marginRight: '7px',
                              marginBottom: '7px'
                            }}
                            title={customSelectedTests.has(testName) ? "Click to remove from queue" : "Click to add to queue"}
                          >
                            {customSelectedTests.has(testName) ? '✅ ' : '☐ '}{testName}
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>

                {/* Selected Tests Summary */}
                {customSelectedTests.size > 0 && (
                  <>
                    <h4 className="font-poppins font-semibold text-gray-800 text-md mb-3">
                      Selected Tests ({customSelectedTests.size}):
                    </h4>
                    <div className="flex flex-wrap gap-2 mb-4">
                      {Array.from(customSelectedTests).map(testName => (
                        <button
                          key={testName}
                          onClick={() => toggleCustomTest(testName)}
                          className={`font-comfortaa font-semibold hover-lift transition-all border-2 border-black ${getColorClass(getTestColor(testName))} border-gray-400 text-gray-800`}
                          style={{
                            padding: '5px',
                            borderRadius: '5px',
                            marginRight: '7px',
                            marginBottom: '7px'
                          }}
                          title="Click to remove from queue"
                        >
                          {testName}
                        </button>
                      ))}
                    </div>

                    {/* CLI Command Display */}
                    <div 
                      style={{ 
                        marginTop: '15px',
                        fontFamily: 'Roboto, sans-serif',
                        fontWeight: 'normal',
                        color: '#1f2937',
                        fontSize: 'inherit'
                      }} 
                      className="cli-command"
                    >
                      <div 
                        className="cli-command"
                        style={{ 
                          fontFamily: 'Roboto, sans-serif',
                          fontWeight: 'normal',
                          color: '#1f2937',
                          fontSize: '1.125rem',
                          marginBottom: '0.5rem',
                          display: 'block'
                        }}
                      >
                        Generated CLI Command:
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', margin: '5px' }}>
                        <div 
                          className="rounded text-white cli-command"
                          style={{ 
                            fontFamily: "'Monaco', 'Menlo', 'Consolas', 'Courier New', monospace",
                            fontWeight: 'normal',
                            backgroundColor: '#374151',
                            fontSize: '0.875rem',
                            letterSpacing: '0.025em',
                            lineHeight: '1.5',
                            color: '#ffffff',
                            padding: '5px',
                            margin: '5px',
                            display: 'block',
                            position: 'relative',
                            paddingRight: '30px'
                          }}
                        >
                          ./k8s_diagnostic test list: {Array.from(customSelectedTests).join(',')} --verbose
                          <span
                            onClick={() => copyToClipboard(`./k8s_diagnostic test list: ${Array.from(customSelectedTests).join(',')} --verbose`)}
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
                            title="Copy command to clipboard"
                          >
                            📋
                          </span>
                        </div>
                        {copyFeedback && (
                          <span 
                            style={{ 
                              fontSize: '0.75rem', 
                              color: copyFeedback.includes('✅') ? '#10b981' : '#ef4444',
                              fontWeight: 'bold'
                            }}
                          >
                            {copyFeedback}
                          </span>
                        )}
                      </div>
                    </div>
                  </>
                )}
              </div>
            )}

            {/* Cilium Configuration Modal - Position it below the header */}
            <CiliumConfigModal 
              showModal={showCiliumModal}
              onCloseModal={handleCloseCiliumModal}
              onConfigComplete={handleConfigComplete}
              isRunning={ciliumIsRunning}
              setIsRunning={setCiliumIsRunning}
            />

            {/* Main Content */}
            <DiagnosticQuestions onTestQueueChange={handleTestQueueChange} />
          </div>
        ) : (
          <BatchTestRunner
            testQueue={testQueue}
            onBack={handleBackToQuestions}
            onTestComplete={handleBatchTestComplete}
          />
        )}

        {/* Footer */}
        <footer className="bg-gray-50 border-t border-gray-200 py-6 mt-12">
          <div className="max-w-6xl mx-auto px-6 text-center">
            <p className="text-gray-500 text-sm font-inter">
              K8s Diagnostic Tool - Professional Network Policy Testing
            </p>
            <p className="text-gray-400 text-xs font-inter mt-1">
              Built with Next.js, Tailwind CSS, and Go CLI backend
            </p>
          </div>
        </footer>
        
        {/* Floating Action Button - Only show on questions page */}
        {currentView === 'questions' && (
          <button
            onClick={handleRunSelectedTests}
            className="surprise-btn font-comfortaa font-semibold transition-all hover-lift card-shadow"
            style={{ 
              position: 'fixed',
              bottom: '20px',
              right: '20px',
              zIndex: 99999,
              padding: '15px', 
              borderRadius: '5px'
            }}
            title="Run the selected diagnostic tests"
          >
            🚀 Run Tests ({
              showCustomPicker && customSelectedTests.size > 0 
                ? customSelectedTests.size
                : testQueue.length > 0 
                  ? testQueue.length 
                  : 'Select some tests'
            })
          </button>
        )}

        {/* Scroll to Top Button */}
        {showScrollTop && (
          <button
            onClick={scrollToTop}
            className="scroll-top-btn font-comfortaa font-bold transition-all hover-lift card-shadow fade-in-up"
            style={{ 
              position: 'fixed',
              bottom: '20px',
              left: '20px',
              zIndex: 99998,
              padding: '12px', 
              borderRadius: '50%',
              width: '50px',
              height: '50px',
              fontSize: '20px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center'
            }}
            title="Scroll to top"
          >
            ↑
          </button>
        )}
      </main>
    </>
  );
}
