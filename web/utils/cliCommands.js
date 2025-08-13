// Question to CLI command mapping for k8s diagnostic UI
export const diagnosticQuestions = [
  {
    id: 'q1',
    question: "Is the broken traffic just one pod talking directly to another pod, without using a Service?",
    cliCommand: './k8s_diagnostic test pod-to-pod-cross-node --verbose',
    testType: 'pod-to-pod-cross-node',
    description: 'Verifies basic pod ↔ pod reachability across nodes',
    category: 'basic-connectivity',
    icon: '🔗',
    estimatedTime: '2-5 minutes'
  },
  {
    id: 'q2',
    question: "Are those two pods on different worker nodes?",
    cliCommand: './k8s_diagnostic test pod-to-pod-same-node --verbose',
    testType: 'pod-to-pod-same-node',
    description: 'Tests pod connectivity on same node',
    category: 'basic-connectivity',
    icon: '🌐',
    estimatedTime: '3-6 minutes'
  },
  {
    id: 'q3',
    question: "Does the traffic go through a normal ClusterIP Service inside the cluster?",
    cliCommand: './k8s_diagnostic test service-clusterip --verbose',
    testType: 'service-clusterip',
    description: 'Checks ClusterIP service connectivity',
    category: 'service-connectivity',
    icon: '⚖️',
    estimatedTime: '2-4 minutes'
  },
  {
    id: 'q4',
    question: "Is the Service being reached via a NodePort on the node's IP?",
    cliCommand: './k8s_diagnostic test service-nodeport --verbose',
    testType: 'service-nodeport',
    description: 'Validates NodePort service connectivity',
    category: 'service-connectivity',
    icon: '🚪',
    estimatedTime: '3-5 minutes'
  },
  {
    id: 'q5',
    question: "Is it exposed with an external LoadBalancer IP from your cloud or MetalLB?",
    cliCommand: './k8s_diagnostic test service-loadbalancer --verbose',
    testType: 'service-loadbalancer',
    description: 'Tests LoadBalancer service connectivity',
    category: 'service-connectivity',
    icon: '🏗️',
    estimatedTime: '4-7 minutes'
  },
  {
    id: 'q6',
    question: "Inside the pod, does the DNS lookup fail for that host or service name?",
    cliCommand: './k8s_diagnostic test dns-resolution --verbose',
    testType: 'dns-resolution',
    description: 'Tests DNS resolution for services',
    category: 'dns-issues',
    icon: '📛',
    estimatedTime: '2-3 minutes'
  },
  {
    id: 'q7',
    question: "Did someone recently add a policy that's supposed to allow everything or block everything between pods?",
    cliCommand: './k8s_diagnostic test list: accepting-all-pods,rejecting-all-pods --verbose',
    testType: 'basic-policies',
    description: 'Sanity-check cluster-wide allow/deny',
    category: 'policy-issues',
    icon: '🛡️',
    estimatedTime: '5-8 minutes'
  },
  {
    id: 'q8',
    question: "Are there network policies based on IP ranges, pod labels, entities, or node selectors that might apply?",
    cliCommand: './k8s_diagnostic test l3-policies --verbose',
    testType: 'l3-policies',
    description: 'Covers every L3 rule type (CIDR, label, DNS, node…)',
    category: 'policy-issues',
    icon: '📋',
    estimatedTime: '10-15 minutes'
  },
  {
    id: 'q9',
    question: "Are there port or protocol restrictions (TCP/UDP ports, ICMP types, TLS / SNI) in place?",
    cliCommand: './k8s_diagnostic test l4-policies --verbose',
    testType: 'l4-policies',
    description: 'Hits single-port, ranges, ICMP, SNI, etc.',
    category: 'policy-issues',
    icon: '🔐',
    estimatedTime: '8-12 minutes'
  },
  {
    id: 'q10',
    question: "Are there any application-level rules (like specific HTTP paths or methods)?",
    cliCommand: './k8s_diagnostic test l7-policies --verbose',
    testType: 'l7-policies',
    description: 'Checks L7 filters: HTTP, DNS allow/deny combos',
    category: 'policy-issues',
    icon: '🌍',
    estimatedTime: '6-10 minutes'
  }
];

// Category groupings for the UI
export const questionCategories = {
  'basic-connectivity': {
    title: 'Basic Connectivity Issues',
    description: 'Test fundamental pod-to-pod and cross-node communication',
    color: 'bg-blue-50 border-blue-200',
    iconColor: 'text-blue-600'
  },
  'service-connectivity': {
    title: 'Service & Load Balancer Issues',
    description: 'Diagnose service discovery and load balancing problems',
    color: 'bg-green-50 border-green-200',
    iconColor: 'text-green-600'
  },
  'dns-issues': {
    title: 'DNS Resolution Issues',
    description: 'Check DNS lookup and service discovery problems',
    color: 'bg-yellow-50 border-yellow-200',
    iconColor: 'text-yellow-600'
  },
  'policy-issues': {
    title: 'Network Policy Issues',
    description: 'Test various network policy configurations and rules',
    color: 'bg-red-50 border-red-200',
    iconColor: 'text-red-600'
  }
};

// Helper function to get questions by category
export function getQuestionsByCategory(category) {
  return diagnosticQuestions.filter(q => q.category === category);
}

// Helper function to get question by ID
export function getQuestionById(id) {
  return diagnosticQuestions.find(q => q.id === id);
}

// Helper function to get all categories with their questions
export function getAllCategoriesWithQuestions() {
  const result = {};
  Object.keys(questionCategories).forEach(categoryKey => {
    result[categoryKey] = {
      ...questionCategories[categoryKey],
      questions: getQuestionsByCategory(categoryKey)
    };
  });
  return result;
}
