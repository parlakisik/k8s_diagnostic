import { useState } from 'react';

const DIAGNOSTIC_QUESTIONS = [
  {
    id: 'q1',
    question: 'Is the broken traffic just one pod talking directly to another pod, without using a Service?',
    description: 'Validates basic pod-to-pod networking across cluster nodes',
    emoji: '🔗',
    color: 'networking',
    tests: ['pod-to-pod-cross-node'],
    category: 'Basic Connectivity'
  },
  {
    id: 'q2',
    question: 'Are you having issues with Services (ClusterIP, NodePort, LoadBalancer)?',
    description: 'Tests Kubernetes service types and load balancing functionality',
    emoji: '🌐',
    color: 'networking',
    tests: ['service-clusterip', 'service-nodeport', 'service-loadbalancer'],
    category: 'Service Connectivity'
  },
  {
    id: 'q3',
    question: 'Is DNS resolution not working properly within the cluster?',
    description: 'Validates internal DNS resolution and service discovery',
    emoji: '🔍',
    color: 'dns',
    tests: ['dns-resolution'],
    category: 'DNS & Discovery'
  },
  {
    id: 'q4',
    question: 'Are there basic network connectivity issues between pods on different nodes?',
    description: 'Tests pod communication and networking across cluster nodes',
    emoji: '🌉',
    color: 'networking',
    tests: ['pod-to-pod-same-node', 'pod-to-pod-cross-node', 'service-cross-node'],
    category: 'Cross-Node Connectivity'
  },
  {
    id: 'q5',
    question: 'Are there network policies blocking traffic based on IP ranges or CIDR blocks?',
    description: 'Validates network policies blocking traffic based on IP ranges',
    emoji: '🛡️',
    color: 'l3',
    tests: ['cidr-ingress', 'cidr-egress', 'cidr-except'],
    category: 'L3 IP Policies'
  },
  {
    id: 'q6',
    question: 'Are there network policies based on pod labels, endpoints, or entity selectors?',
    description: 'Tests network policies using label selectors and entity rules',
    emoji: '🏷️',
    color: 'l3',
    tests: ['endpoints-label', 'entities-based'],
    category: 'L3 Label Policies'
  },
  {
    id: 'q7',
    question: 'Are there DNS-based network policies or node selector restrictions?',
    description: 'Validates DNS-based policies and node-specific restrictions',
    emoji: '📡',
    color: 'l3',
    tests: ['dns-based', 'node-selector', 'pod-node-name', 'node-cidr', 'node-based'],
    category: 'L3 DNS & Node Policies'
  },
  {
    id: 'q8',
    question: 'Are there network policies based on IP ranges, pod labels, entities, or node selectors that might apply?',
    description: 'Comprehensive Layer 3 network policy testing suite',
    emoji: '🔐',
    color: 'l3',
    tests: ['cidr-ingress', 'cidr-egress', 'cidr-except', 'endpoints-label', 'entities-based', 'dns-based', 'node-selector', 'pod-node-name', 'node-cidr', 'node-based', 'kubernetes-service', 'allow-all', 'deny-all'],
    category: 'Complete L3 Analysis'
  },
  {
    id: 'q9',
    question: 'Are there port-specific or protocol restrictions (TCP, UDP, ICMP)?',
    description: 'Tests port-based and protocol-specific network policies',
    emoji: '🔌',
    color: 'l4',
    tests: ['tcp-port-ingress', 'tcp-port-egress', 'port-range', 'multiple-port', 'icmp-type', 'icmpv6-type', 'mixed-icmp'],
    category: 'L4 Port & Protocol Policies'
  },
  {
    id: 'q10',
    question: 'Are there TLS/SNI-based network policy restrictions?',
    description: 'Validates TLS Server Name Indication policy restrictions',
    emoji: '🔒',
    color: 'l4',
    tests: ['basic-sni', 'multi-domain-sni', 'combined-l4-sni'],
    category: 'L4 TLS/SNI Policies'
  },
  {
    id: 'q11',
    question: 'Are there HTTP/HTTPS application-layer restrictions or policies?',
    description: 'Tests Layer 7 HTTP-based application policy enforcement',
    emoji: '🌍',
    color: 'l7',
    tests: ['basic-http-get', 'http-with-headers', 'path-method'],
    category: 'L7 HTTP Policies'
  },
  {
    id: 'q12',
    question: 'Are there DNS query restrictions at the application layer?',
    description: 'Validates Layer 7 DNS query matching and pattern policies',
    emoji: '🎯',
    color: 'l7',
    tests: ['dns-matchname', 'dns-matchpattern'],
    category: 'L7 DNS Policies'
  }
];

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

export default function DiagnosticQuestions({ onTestQueueChange }) {
  const [testQueue, setTestQueue] = useState([]);
  const [selectedQuestions, setSelectedQuestions] = useState(new Set());
  const [allDiscoveredTests, setAllDiscoveredTests] = useState([]);
  const [copyFeedback, setCopyFeedback] = useState('');

  const updateTestQueue = (newQueue) => {
    setTestQueue(newQueue);
    if (onTestQueueChange) {
      onTestQueueChange(newQueue);
    }
  };

  // Create a mapping from test name to its category color
  const getTestColor = (testName) => {
    for (const question of DIAGNOSTIC_QUESTIONS) {
      if (question.tests.includes(testName)) {
        return question.color;
      }
    }
    return 'networking'; // fallback
  };

  // Toggle individual test in/out of queue
  const toggleTest = (testName) => {
    const newQueue = testQueue.includes(testName) 
      ? testQueue.filter(test => test !== testName)
      : [...testQueue, testName];
    updateTestQueue(newQueue);
    
    // Check if any questions should be deselected
    const newSelectedQuestions = new Set(selectedQuestions);
    
    DIAGNOSTIC_QUESTIONS.forEach(question => {
      const hasAnyTestsInQueue = question.tests.some(test => newQueue.includes(test));
      if (!hasAnyTestsInQueue) {
        newSelectedQuestions.delete(question.id);
      } else if (question.tests.every(test => newQueue.includes(test))) {
        newSelectedQuestions.add(question.id);
      }
    });
    
    setSelectedQuestions(newSelectedQuestions);
  };

  const handleYesClick = (question) => {
    // Add tests to queue
    const newTests = [...testQueue];
    const newDiscoveredTests = [...allDiscoveredTests];
    
    question.tests.forEach(test => {
      if (!newTests.includes(test)) {
        newTests.push(test);
      }
      if (!newDiscoveredTests.includes(test)) {
        newDiscoveredTests.push(test);
      }
    });
    
    updateTestQueue(newTests);
    setAllDiscoveredTests(newDiscoveredTests);
    setSelectedQuestions(prev => new Set([...prev, question.id]));
  };

  const handleNoClick = (question) => {
    // Remove tests from queue
    const newTests = testQueue.filter(test => !question.tests.includes(test));
    updateTestQueue(newTests);
    setSelectedQuestions(prev => {
      const newSet = new Set(prev);
      newSet.delete(question.id);
      return newSet;
    });
  };

  const clearAllTests = () => {
    updateTestQueue([]);
    setSelectedQuestions(new Set());
  };

  const isQuestionSelected = (questionId) => selectedQuestions.has(questionId);

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

  return (
    <div className="w-full max-w-6xl mx-auto animate-fade-in" style={{ padding: '15px' }}>
      {/* Test Queue Counter */}
      {testQueue.length > 0 && (
        <div className="text-center mb-6" style={{ padding: '12px' }}>
          <div className="inline-flex items-center space-x-3">
            <div 
              className="queue-badge text-white font-comfortaa font-semibold"
              style={{ margin: '10px', padding: '10px', display: 'inline-block', borderRadius: '5px' }}
            >
              📋 {testQueue.length} tests selected
            </div>
            <button
              onClick={clearAllTests}
              className="btn-outline font-comfortaa hover-lift"
              style={{ margin: '10px', padding: '10px', borderRadius: '5px', display: 'inline-block' }}
            >
              🗑️ Clear All
            </button>
          </div>
        </div>
      )}

      {/* Questions CSS Grid Layout - Responsive */}
      <div 
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
          gap: '24px 10px',
          width: '100%',
          alignItems: 'stretch'
        }}
        className="responsive-grid"
      >
        {DIAGNOSTIC_QUESTIONS.map((question) => (
          <div
            key={question.id}
            className={`question-card border-2 card-shadow hover-glow transition-all duration-300 overflow-hidden ${
              isQuestionSelected(question.id)
                ? `${getColorClass(question.color)} border-gray-400`
                : 'bg-white border-gray-200 hover:border-gray-300'
            }`}
            style={{
              padding: '17px',
              borderRadius: '5px',
              position: 'relative',
              display: 'flex',
              flexDirection: 'column',
              height: '380px'
            }}
          >
            {/* Top Right Check Mark */}
            {isQuestionSelected(question.id) && (
              <div 
                className="animate-bounce-subtle"
                style={{
                  position: 'absolute',
                  top: '10px',
                  right: '10px',
                  fontSize: '18px',
                  zIndex: 10
                }}
              >
                ✅
              </div>
            )}

            {/* Content Area - Flexible */}
            <div style={{ flex: '1', display: 'flex', flexDirection: 'column' }}>
              {/* Category + Emoji as Main Heading */}
              <div className="mb-4" style={{ flex: '1' }}>
                <h2 className="font-poppins font-bold text-gray-900 text-xl leading-tight mb-3">
                  {question.category} {question.emoji}
                </h2>
                <p className="font-poppins text-gray-700 text-base italic mb-6 break-words whitespace-normal overflow-wrap-anywhere">
                  {question.question}
                </p>
              </div>

              {/* Description - Lower in card with lighter, smaller font */}
              <p className="font-inter text-gray-400 break-words whitespace-normal overflow-wrap-anywhere mb-3" style={{ fontSize: '11px' }}>
                {question.description}
              </p>

              {/* Test Preview - Lower in card with smaller font */}
              <div className="p-2 bg-gray-50 rounded border-l-2 border-gray-300" style={{ marginBottom: '10px' }}>
                <div className="font-inter text-gray-500 mb-1" style={{ fontSize: '10px' }}>
                  Tests to be added ({question.tests.length}):
                </div>
                <div className="font-inter text-gray-700 break-words" style={{ fontSize: '10px' }}>
                  {question.tests.join(', ')}
                </div>
              </div>
            </div>

            {/* Yes/No Buttons - Always at Bottom */}
            <div className="flex" style={{ marginTop: 'auto' }}>
              <button
                onClick={() => handleYesClick(question)}
                disabled={isQuestionSelected(question.id)}
                className={`flex-1 font-comfortaa font-semibold hover-lift transition-all border-2 border-black ${
                  isQuestionSelected(question.id)
                    ? 'bg-green-100 border-green-400 text-green-700 cursor-not-allowed'
                    : 'bg-white hover:bg-green-50 hover:border-green-400'
                }`}
                style={{
                  padding: '5px',
                  borderRadius: '5px',
                  marginRight: '7px'
                }}
              >
                {isQuestionSelected(question.id) ? '✅ Added' : '✅ Yes'}
              </button>
              <button
                onClick={() => handleNoClick(question)}
                className="flex-1 font-comfortaa font-semibold hover-lift transition-all border-2 border-black bg-white hover:bg-red-50 hover:border-red-400"
                style={{
                  padding: '5px',
                  borderRadius: '5px'
                }}
              >
                ❌ No
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Test Queue Summary - Interactive Toggle Buttons */}
      {testQueue.length > 0 && (
        <div className="bg-white border-b border-gray-200 px-6 py-4">
          <h3 className="font-poppins font-semibold text-gray-900 text-lg mb-4">
            📋 Selected Test Queue ({testQueue.length} tests)
          </h3>
          
          {/* Interactive Test Toggle Buttons */}
          <div className="flex flex-wrap gap-2 mb-4">
            {allDiscoveredTests.map(test => {
              const testColor = getTestColor(test);
              const isSelected = testQueue.includes(test);
              return (
                <button
                  key={test}
                  onClick={() => toggleTest(test)}
                  className={`font-comfortaa font-semibold hover-lift transition-all border-2 border-black ${
                    isSelected 
                      ? `${getColorClass(testColor)} border-gray-400 text-gray-800` 
                      : 'bg-white border-gray-300 text-gray-600 hover:border-gray-400'
                  }`}
                  style={{
                    padding: '5px',
                    borderRadius: '5px',
                    marginRight: '7px',
                    marginBottom: '7px'
                  }}
                  title={isSelected ? "Click to remove from queue" : "Click to add to queue"}
                >
                  {test}
                </button>
              );
            })}
          </div>
          
          {testQueue.length > 0 && (
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
                  ./k8s_diagnostic test list: {testQueue.join(',')} --verbose
                  <span
                    onClick={() => copyToClipboard(`./k8s_diagnostic test list: ${testQueue.join(',')} --verbose`)}
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
          )}
        </div>
      )}

      {/* Help Section */}
      <div className="mt-8 text-center py-6 border-t border-gray-200">
        <p className="text-gray-500 text-sm font-inter mb-2">
          💡 <strong>Tip:</strong> Start with basic connectivity questions before moving to complex policy tests
        </p>
        <p className="text-gray-400 text-xs font-inter">
          Selected tests will run with detailed logging and real-time progress monitoring
        </p>
      </div>
    </div>
  );
}
