/**
 * Comprehensive Test Insights Configuration
 * 
 * This file provides detailed success and failure messages for all diagnostic tests,
 * giving users meaningful insights about what each test validates and its real-world impact.
 * 
 * Categories:
 * - Networking Tests (7 tests)
 * - L3 Policy Tests (11 tests) 
 * - L4 Policy Tests (10 tests)
 * - L7 Policy Tests (5 tests)
 * 
 * Total: 33 tests with rich contextual insights
 */

export const testInsights = {
  // =============================================================================
  // NETWORKING TESTS (7 tests)
  // =============================================================================
  
  "pod-to-pod-same-node": {
    category: "networking",
    success: {
      title: "✅ Same-node pod networking working perfectly!",
      details: [
        "📊 Pod-to-pod communication within nodes is functioning correctly",
        "🎯 Your CNI (Container Network Interface) is properly configured",
        "💡 Applications can communicate reliably when scheduled on the same worker node"
      ]
    },
    failure: {
      title: "❌ Same-node pod communication failed",
      details: [
        "📊 Basic pod networking within nodes is not working",
        "🔧 Check CNI plugin configuration and node networking setup",
        "💡 This is a fundamental networking issue that will affect all pod communication"
      ]
    }
  },

  "pod-to-pod-cross-node": {
    category: "networking", 
    success: {
      title: "✅ Cross-node pod networking working perfectly!",
      details: [
        "📊 Pods can communicate seamlessly across different worker nodes",
        "🎯 Your cluster's inter-node networking and CNI routing is operational",
        "💡 Distributed applications and multi-node deployments will work reliably"
      ]
    },
    failure: {
      title: "❌ Cross-node pod communication failed",
      details: [
        "📊 Pods cannot communicate across different worker nodes",
        "🔧 Check inter-node routing, CNI configuration, and network policies",
        "💡 This limits your cluster to single-node deployments only"
      ]
    }
  },

  "service-clusterip": {
    category: "networking",
    success: {
      title: "✅ ClusterIP service networking working perfectly!",
      details: [
        "📊 Internal service discovery and load balancing is operational",
        "🎯 DNS resolution and service-to-pod routing works correctly", 
        "💡 Your applications can reliably communicate via Kubernetes services"
      ]
    },
    failure: {
      title: "❌ ClusterIP service communication failed",
      details: [
        "📊 Internal service discovery or load balancing is not working",
        "🔧 Check kube-proxy, CoreDNS, and service endpoint configuration",
        "💡 Applications won't be able to communicate via service names"
      ]
    }
  },

  "service-nodeport": {
    category: "networking",
    success: {
      title: "✅ NodePort service networking working perfectly!",
      details: [
        "📊 External access through node ports is enabled and functional",
        "🎯 Services are accessible from outside the cluster via node IPs",
        "💡 Your cluster can handle external traffic and expose applications publicly"
      ]
    },
    failure: {
      title: "❌ NodePort service access failed", 
      details: [
        "📊 External access through node ports is not working",
        "🔧 Check node firewall rules, kube-proxy, and NodePort range configuration",
        "💡 External services and ingress controllers may not function properly"
      ]
    }
  },

  "service-loadbalancer": {
    category: "networking",
    success: {
      title: "✅ LoadBalancer service networking working perfectly!",
      details: [
        "📊 LoadBalancer service type is supported and functional",
        "🎯 Cloud provider integration or MetalLB is working correctly",
        "💡 Your cluster can provision external load balancers for high availability"
      ]
    },
    failure: {
      title: "❌ LoadBalancer service provisioning failed",
      details: [
        "📊 LoadBalancer service type is not supported or misconfigured",
        "🔧 Check cloud provider integration or install/configure MetalLB",
        "💡 External load balancing will fall back to NodePort or ClusterIP only"
      ]
    }
  },

  "service-cross-node": {
    category: "networking",
    success: {
      title: "✅ Cross-node service load balancing working perfectly!",
      details: [
        "📊 Services can distribute traffic across pods on multiple nodes seamlessly",
        "🎯 Your cluster supports highly available distributed applications",
        "💡 Multi-node deployments will have proper load distribution and failover capabilities"
      ]
    },
    failure: {
      title: "❌ Cross-node service load balancing failed",
      details: [
        "📊 Services cannot properly balance traffic across nodes",
        "🔧 Check kube-proxy mode, CNI configuration, and inter-node connectivity",
        "💡 High availability deployments may not distribute traffic correctly"
      ]
    }
  },

  "dns-resolution": {
    category: "networking",
    success: {
      title: "✅ DNS resolution working perfectly!",
      details: [
        "📊 CoreDNS and cluster DNS resolution is fully operational",
        "🎯 Services can discover each other using Kubernetes DNS names",
        "💡 Your applications can reliably resolve service names to IP addresses"
      ]
    },
    failure: {
      title: "❌ DNS resolution failed",
      details: [
        "📊 Cluster DNS resolution is not working properly",
        "🔧 Check CoreDNS pods, configuration, and cluster DNS service",
        "💡 Applications won't be able to find services using DNS names"
      ]
    }
  },

  // =============================================================================
  // L3 POLICY TESTS (11 tests)
  // =============================================================================

  "cidr-ingress": {
    category: "l3-policy",
    success: {
      title: "✅ CIDR-based ingress policy working perfectly!",
      details: [
        "📊 Network policies can restrict incoming traffic by IP ranges",
        "🎯 CIDR-based access controls are properly enforced by your CNI",
        "💡 You can implement IP-based segmentation and external access controls"
      ]
    },
    failure: {
      title: "❌ CIDR-based ingress policy enforcement failed",
      details: [
        "📊 Network policies are not properly restricting traffic by IP ranges",
        "🔧 Check CNI network policy support and policy configuration",
        "💡 IP-based security controls may not be effective"
      ]
    }
  },

  "cidr-egress": {
    category: "l3-policy", 
    success: {
      title: "✅ CIDR-based egress policy working perfectly!",
      details: [
        "📊 Network policies can control outbound traffic to specific IP ranges",
        "🎯 Egress filtering by destination CIDR is properly enforced",
        "💡 You can implement fine-grained outbound access controls and data loss prevention"
      ]
    },
    failure: {
      title: "❌ CIDR-based egress policy enforcement failed",
      details: [
        "📊 Network policies cannot restrict outbound traffic by IP ranges", 
        "🔧 Check egress policy support and CNI configuration",
        "💡 Outbound traffic controls and compliance requirements may not be enforceable"
      ]
    }
  },

  "cidr-except": {
    category: "l3-policy",
    success: {
      title: "✅ CIDR exception policy working perfectly!",
      details: [
        "📊 Network policies support complex CIDR rules with exceptions",
        "🎯 'Except' clauses in CIDR policies are properly implemented",
        "💡 You can create sophisticated network segmentation with granular exceptions"
      ]
    },
    failure: {
      title: "❌ CIDR exception policy failed",
      details: [
        "📊 CIDR exception rules are not working as expected",
        "🔧 Check advanced network policy feature support in your CNI",
        "💡 Complex network segmentation scenarios may not work correctly"
      ]
    }
  },

  "endpoints-label": {
    category: "l3-policy",
    success: {
      title: "✅ Endpoint label selector policy working perfectly!",
      details: [
        "📊 Network policies can target pods using label selectors effectively",
        "🎯 Label-based network segmentation is fully functional",
        "💡 You can implement dynamic, label-driven micro-segmentation"
      ]
    },
    failure: {
      title: "❌ Endpoint label selector policy failed",
      details: [
        "📊 Label-based pod targeting in network policies is not working",
        "🔧 Check label selector syntax and CNI label awareness",
        "💡 Dynamic network segmentation based on labels will not function"
      ]
    }
  },

  "entities-based": {
    category: "l3-policy",
    success: {
      title: "✅ Entity-based policy working perfectly!",
      details: [
        "📊 Network policies can reference cluster entities (nodes, namespaces)",
        "🎯 Entity-based network controls are properly enforced",
        "💡 You can implement infrastructure-aware security policies"
      ]
    },
    failure: {
      title: "❌ Entity-based policy enforcement failed",
      details: [
        "📊 Entity references in network policies are not working",
        "🔧 Check CNI support for entity-based policy features",
        "💡 Infrastructure-aware security controls may not be available"
      ]
    }
  },

  "dns-based": {
    category: "l3-policy",
    success: {
      title: "✅ DNS-based policy working perfectly!",
      details: [
        "📊 Network policies can control access using DNS names",
        "🎯 DNS-aware network filtering is operational",
        "💡 You can implement policies based on external service names and FQDNs"
      ]
    },
    failure: {
      title: "❌ DNS-based policy enforcement failed",
      details: [
        "📊 DNS-based network controls are not functioning",
        "🔧 Check DNS proxy configuration and policy DNS resolution",
        "💡 External service access controls via DNS names will not work"
      ]
    }
  },

  "node-selector": {
    category: "l3-policy",
    success: {
      title: "✅ Node selector policy working perfectly!",
      details: [
        "📊 Network policies can target traffic based on node characteristics",
        "🎯 Node-based network segmentation is functional",
        "💡 You can implement node-aware security controls and infrastructure policies"
      ]
    },
    failure: {
      title: "❌ Node selector policy failed",
      details: [
        "📊 Node-based targeting in network policies is not working",
        "🔧 Check node labeling and CNI node awareness features",
        "💡 Infrastructure-based network controls will not be effective"
      ]
    }
  },

  "pod-node-name": {
    category: "l3-policy",
    success: {
      title: "✅ Pod node name policy working perfectly!",
      details: [
        "📊 Network policies can reference specific node names for targeting",
        "🎯 Node-specific network controls are properly enforced",
        "💡 You can implement precise infrastructure-aware security policies"
      ]
    },
    failure: {
      title: "❌ Pod node name policy failed",
      details: [
        "📊 Node name references in policies are not working correctly",
        "🔧 Check node naming consistency and policy node resolution",
        "💡 Node-specific network controls may not function as expected"
      ]
    }
  },

  "node-cidr": {
    category: "l3-policy",
    success: {
      title: "✅ Node CIDR policy working perfectly!",
      details: [
        "📊 Network policies can control traffic based on node IP ranges",
        "🎯 Node CIDR-based network segmentation is operational",
        "💡 You can implement network zones and infrastructure-based access controls"
      ]
    },
    failure: {
      title: "❌ Node CIDR policy enforcement failed",
      details: [
        "📊 Node CIDR-based controls are not functioning properly",
        "🔧 Check node IP configuration and CIDR policy support",
        "💡 Infrastructure-based network zoning will not work correctly"
      ]
    }
  },

  "node-based": {
    category: "l3-policy",
    success: {
      title: "✅ Node-based policy working perfectly!",
      details: [
        "📊 Cluster-wide node-based network policies are fully functional",
        "🎯 Node-level network controls work across the entire cluster",
        "💡 You can implement comprehensive infrastructure-aware security policies"
      ]
    },
    failure: {
      title: "❌ Node-based policy enforcement failed",
      details: [
        "📊 Cluster-wide node-based controls are not working",
        "🔧 Check global policy enforcement and node policy propagation",
        "💡 Infrastructure-wide security controls may have gaps"
      ]
    }
  },

  "kubernetes-service": {
    category: "l3-policy",
    success: {
      title: "✅ Kubernetes service policy working perfectly!",
      details: [
        "📊 Network policies can reference and control Kubernetes services",
        "🎯 Service-aware network controls are properly enforced",
        "💡 You can implement service-level access controls and micro-segmentation"
      ]
    },
    failure: {
      title: "❌ Kubernetes service policy failed",
      details: [
        "📊 Service references in network policies are not working",
        "🔧 Check service policy support and service discovery integration",
        "💡 Service-level network controls will not be effective"
      ]
    }
  },

  // =============================================================================
  // L4 POLICY TESTS (10 tests)
  // =============================================================================

  "tcp-port-ingress": {
    category: "l4-policy",
    success: {
      title: "✅ TCP port ingress policy working perfectly!",
      details: [
        "📊 Network policies can restrict incoming TCP traffic by port",
        "🎯 Port-based access controls are properly enforced for ingress",
        "💡 You can implement granular service-level security and port isolation"
      ]
    },
    failure: {
      title: "❌ TCP port ingress policy failed",
      details: [
        "📊 TCP port restrictions for incoming traffic are not working",
        "🔧 Check L4 policy support and port filtering configuration",
        "💡 Service-level access controls may not be enforceable"
      ]
    }
  },

  "tcp-port-egress": {
    category: "l4-policy",
    success: {
      title: "✅ TCP port egress policy working perfectly!",
      details: [
        "📊 Network policies can control outbound TCP traffic by destination port",
        "🎯 Port-based egress filtering is properly enforced",
        "💡 You can prevent unauthorized outbound connections and implement compliance controls"
      ]
    },
    failure: {
      title: "❌ TCP port egress policy failed",
      details: [
        "📊 TCP port restrictions for outbound traffic are not working",
        "🔧 Check egress policy support and port filtering capabilities",
        "💡 Outbound port controls and compliance requirements cannot be enforced"
      ]
    }
  },

  "port-range": {
    category: "l4-policy",
    success: {
      title: "✅ Port range policy working perfectly!",
      details: [
        "📊 Network policies support port range specifications effectively",
        "🎯 Multiple ports can be controlled with efficient range policies",
        "💡 You can implement comprehensive port controls without managing individual port rules"
      ]
    },
    failure: {
      title: "❌ Port range policy failed",
      details: [
        "📊 Port range specifications in policies are not working",
        "🔧 Check port range syntax support and policy parsing",
        "💡 Efficient multi-port controls will require individual port rules"
      ]
    }
  },

  "multiple-port": {
    category: "l4-policy", 
    success: {
      title: "✅ Multiple port policy working perfectly!",
      details: [
        "📊 Network policies can specify multiple individual ports correctly",
        "🎯 Complex port-based access controls are fully functional",
        "💡 You can implement sophisticated service-level security with multiple port rules"
      ]
    },
    failure: {
      title: "❌ Multiple port policy failed",
      details: [
        "📊 Multiple port specifications in policies are not working properly",
        "🔧 Check policy syntax for multiple ports and CNI parsing",
        "💡 Complex service-level controls may require simplified port rules"
      ]
    }
  },

  "icmp-type": {
    category: "l4-policy",
    success: {
      title: "✅ ICMP type policy working perfectly!",
      details: [
        "📊 Network policies can control ICMP traffic by message type",
        "🎯 ICMP-based network diagnostics and controls are functional", 
        "💡 You can implement fine-grained control over ping, traceroute, and other ICMP tools"
      ]
    },
    failure: {
      title: "❌ ICMP type policy failed",
      details: [
        "📊 ICMP type-based controls are not working correctly",
        "🔧 Check ICMP policy support and protocol-specific filtering",
        "💡 Network diagnostic tools and ICMP-based controls may not be manageable"
      ]
    }
  },

  "icmpv6-type": {
    category: "l4-policy",
    success: {
      title: "✅ ICMPv6 type policy working perfectly!",
      details: [
        "📊 Network policies support IPv6 ICMP traffic controls",
        "🎯 IPv6-specific network diagnostics are properly managed",
        "💡 Your cluster has full IPv6 network policy support and ICMP controls"
      ]
    },
    failure: {
      title: "❌ ICMPv6 type policy failed",
      details: [
        "📊 IPv6 ICMP controls are not functioning properly", 
        "🔧 Check IPv6 support and ICMPv6 policy capabilities",
        "💡 IPv6 network diagnostic controls may not be available"
      ]
    }
  },

  "mixed-icmp": {
    category: "l4-policy",
    success: {
      title: "✅ Mixed ICMP policy working perfectly!",
      details: [
        "📊 Network policies handle both IPv4 and IPv6 ICMP traffic correctly",
        "🎯 Dual-stack ICMP controls are fully operational",
        "💡 Your cluster supports comprehensive ICMP management across both IP versions"
      ]
    },
    failure: {
      title: "❌ Mixed ICMP policy failed",
      details: [
        "📊 Dual-stack ICMP controls are not working properly",
        "🔧 Check IPv4/IPv6 policy coordination and ICMP handling",
        "💡 Network diagnostic controls may not work consistently across IP versions"
      ]
    }
  },

  "basic-sni": {
    category: "l4-policy",
    success: {
      title: "✅ Basic SNI policy working perfectly!",
      details: [
        "📊 Network policies can inspect and control TLS traffic by Server Name",
        "🎯 SNI-based filtering and routing is properly supported",
        "💡 You can implement domain-specific access controls and TLS traffic management"
      ]
    },
    failure: {
      title: "❌ Basic SNI policy failed", 
      details: [
        "📊 TLS SNI inspection and filtering is not working",
        "🔧 Check SNI inspection capabilities and TLS proxy configuration",
        "💡 Domain-based access controls for encrypted traffic will not function"
      ]
    }
  },

  "multi-domain-sni": {
    category: "l4-policy",
    success: {
      title: "✅ Multi-domain SNI policy working perfectly!",
      details: [
        "📊 Network policies support multiple domain names in SNI rules",
        "🎯 Complex domain-based TLS filtering is fully functional",
        "💡 You can implement sophisticated domain access controls for encrypted services"
      ]
    },
    failure: {
      title: "❌ Multi-domain SNI policy failed",
      details: [
        "📊 Multiple domain SNI filtering is not working properly",
        "🔧 Check SNI policy complexity limits and domain parsing",
        "💡 Complex domain-based controls may require simplified rules"
      ]
    }
  },

  "combined-l4-sni": {
    category: "l4-policy",
    success: {
      title: "✅ Combined L4+SNI policy working perfectly!",
      details: [
        "📊 Network policies can combine port and SNI rules effectively",
        "🎯 Sophisticated L4 and TLS controls work together seamlessly",
        "💡 You can implement comprehensive application-layer security with combined rules"
      ]
    },
    failure: {
      title: "❌ Combined L4+SNI policy failed",
      details: [
        "📊 Combined port and SNI rules are not working together",
        "🔧 Check policy rule combination support and precedence handling",
        "💡 Advanced application-layer controls may require separate policies"
      ]
    }
  },

  // =============================================================================
  // L7 POLICY TESTS (5 tests)
  // =============================================================================

  "basic-http-get": {
    category: "l7-policy",
    success: {
      title: "✅ Basic HTTP GET policy working perfectly!",
      details: [
        "📊 Network policies can inspect and control HTTP methods",
        "🎯 Application-layer HTTP filtering is fully operational",
        "💡 You can implement fine-grained API security and HTTP method controls"
      ]
    },
    failure: {
      title: "❌ Basic HTTP GET policy failed",
      details: [
        "📊 HTTP method inspection and filtering is not working",
        "🔧 Check L7 proxy configuration and HTTP inspection capabilities", 
        "💡 Application-layer API security controls will not be available"
      ]
    }
  },

  "http-with-headers": {
    category: "l7-policy",
    success: {
      title: "✅ HTTP headers policy working perfectly!",
      details: [
        "📊 Network policies can inspect and filter HTTP headers",
        "🎯 Header-based application controls are fully functional",
        "💡 You can implement sophisticated API security based on HTTP headers and metadata"
      ]
    },
    failure: {
      title: "❌ HTTP headers policy failed",
      details: [
        "📊 HTTP header inspection and filtering is not working",
        "🔧 Check HTTP header parsing and L7 inspection configuration",
        "💡 Header-based API controls and authentication mechanisms may not work"
      ]
    }
  },

  "path-method": {
    category: "l7-policy",
    success: {
      title: "✅ HTTP path and method policy working perfectly!",
      details: [
        "📊 Network policies can control HTTP traffic by URL path and method",
        "🎯 RESTful API endpoint controls are fully operational",
        "💡 You can implement granular API security with path and method-based rules"
      ]
    },
    failure: {
      title: "❌ HTTP path and method policy failed",
      details: [
        "📊 HTTP path and method filtering is not working correctly",
        "🔧 Check URL parsing and HTTP method inspection capabilities",
        "💡 RESTful API endpoint controls will not be enforceable"
      ]
    }
  },

  "dns-matchname": {
    category: "l7-policy",
    success: {
      title: "✅ DNS match name policy working perfectly!",
      details: [
        "📊 Network policies can inspect and control DNS queries by exact name",
        "🎯 DNS-based application filtering is fully operational",
        "💡 You can implement precise DNS controls and prevent specific domain resolutions"
      ]
    },
    failure: {
      title: "❌ DNS match name policy failed",
      details: [
        "📊 DNS name inspection and filtering is not working",
        "🔧 Check DNS proxy configuration and query inspection capabilities",
        "💡 DNS-based security controls and domain blocking will not function"
      ]
    }
  },

  "dns-matchpattern": {
    category: "l7-policy",
    success: {
      title: "✅ DNS match pattern policy working perfectly!",
      details: [
        "📊 Network policies support DNS wildcard and pattern matching",
        "🎯 Flexible DNS filtering with patterns is fully functional",
        "💡 You can implement comprehensive DNS controls with wildcard domain blocking"
      ]
    },
    failure: {
      title: "❌ DNS match pattern policy failed",
      details: [
        "📊 DNS pattern matching and wildcards are not working",
        "🔧 Check DNS pattern parsing and wildcard support configuration",
        "💡 Flexible DNS controls and pattern-based domain blocking will not work"
      ]
    }
  }
};

/**
 * Get test insights for a specific test name
 * @param {string} testName - The test identifier
 * @returns {object|null} Test insights object or null if not found
 */
export function getTestInsights(testName) {
  return testInsights[testName] || null;
}

/**
 * Get all test insights for a specific category
 * @param {string} category - Category name (networking, l3-policy, l4-policy, l7-policy)
 * @returns {object} Object containing all tests for the category
 */
export function getTestInsightsByCategory(category) {
  const categoryTests = {};
  
  Object.entries(testInsights).forEach(([testName, insights]) => {
    if (insights.category === category) {
      categoryTests[testName] = insights;
    }
  });
  
  return categoryTests;
}

/**
 * Get a list of all available test names
 * @returns {string[]} Array of all test names
 */
export function getAllTestNames() {
  return Object.keys(testInsights);
}

/**
 * Get test count by category
 * @returns {object} Object with category names and their test counts
 */
export function getTestCountsByCategory() {
  const counts = {
    networking: 0,
    'l3-policy': 0, 
    'l4-policy': 0,
    'l7-policy': 0
  };
  
  Object.values(testInsights).forEach(insights => {
    if (counts.hasOwnProperty(insights.category)) {
      counts[insights.category]++;
    }
  });
  
  return counts;
}

export default testInsights;
