/**
 * TestDefinition - Robust data model for test objects
 * Eliminates the fragile string manipulation approach
 */

// Simple UUID generation for browser environments
function generateUUID() {
  return 'test-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
}

/**
 * Standardized test definition class
 */
export class TestDefinition {
  constructor(input) {
    // Handle multiple input formats for backwards compatibility
    if (typeof input === 'string') {
      // Simple string input (from DiagnosticQuestions)
      this.id = generateUUID();
      this.name = input;
      this.displayName = this.formatDisplayName(input);
      this.description = this.generateDescription(input);
      this.reason = null;
      this.category = this.inferCategory(input);
      this.source = 'diagnostic-questions';
    } else if (input && typeof input === 'object') {
      // Rich object input (from CiliumConfigModal)
      this.id = input.id || generateUUID();
      this.name = input.name || input.testName || 'unknown-test';
      this.displayName = input.displayName || this.formatDisplayName(this.name, input.reason);
      this.description = input.description || input.summary || this.generateDescription(this.name);
      this.reason = input.reason || input.rationale || null;
      this.category = input.category || this.inferCategory(this.name);
      this.source = input.source || 'cilium-config';
      this.metadata = input.metadata || {};
    } else {
      throw new Error('TestDefinition requires string or object input');
    }
    
    // Validation
    this.validate();
  }

  validate() {
    if (!this.id || !this.name) {
      throw new Error(`Invalid TestDefinition: id=${this.id}, name=${this.name}`);
    }
  }

  /**
   * Format display name with clear differentiation for duplicates
   */
  formatDisplayName(name, reason = null) {
    if (!reason) {
      return name.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
    }
    
    // Create clear differentiation for duplicate test names
    const formattedName = name.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
    
    // Handle merged reasons (contains bullet separator ' • ')
    if (reason.includes(' • ')) {
      // Multiple contexts - extract and combine them
      const contexts = reason.split(' • ');
      const contextLabels = contexts.map(ctx => {
        if (ctx.includes('Gateway API')) return 'Gateway API';
        if (ctx.includes('L7 proxy') || ctx.includes('enable-l7-proxy')) return 'L7 Proxy';
        if (ctx.includes('Hubble')) return 'Observability';
        if (ctx.includes('Network Policies') || ctx.includes('networkpolicy')) return 'Network Policy';
        if (ctx.includes('kube-proxy')) return 'Service Mesh';
        return 'Network';
      }).filter((label, index, arr) => arr.indexOf(label) === index); // Remove duplicates
      
      const combinedContext = contextLabels.length > 1 
        ? contextLabels.join(' + ') 
        : contextLabels[0] || 'Network';
      
      return `${formattedName} (${combinedContext})`;
    }
    
    // Single context - extract key context from reason for differentiation
    if (reason.includes('Gateway API')) {
      return `${formattedName} (Gateway API)`;
    } else if (reason.includes('L7 proxy') || reason.includes('enable-l7-proxy')) {
      return `${formattedName} (L7 Proxy)`;
    } else if (reason.includes('Hubble')) {
      return `${formattedName} (Observability)`;
    } else if (reason.includes('Network Policies') || reason.includes('networkpolicy')) {
      return `${formattedName} (Network Policy)`;
    } else if (reason.includes('kube-proxy')) {
      return `${formattedName} (Service Mesh)`;
    }
    
    return formattedName;
  }

  /**
   * Generate description based on test name
   */
  generateDescription(name) {
    const descriptions = {
      'basic-http-get': 'Validates HTTP GET request policies and connectivity',
      'http-with-headers': 'Tests HTTP policies with custom headers',
      'path-method': 'Validates HTTP path and method filtering',
      'pod-to-pod-cross-node': 'Tests cross-node pod communication',
      'service-clusterip': 'Validates ClusterIP service functionality',
      'cidr-ingress': 'Validates CIDR-based ingress filtering',
      'cidr-egress': 'Tests CIDR-based egress filtering',
      'allow-all': 'Tests allow-all network policy behavior',
      'deny-all': 'Validates deny-all network policy enforcement',
      'service-nodeport': 'Validates NodePort service functionality',
      'service-loadbalancer': 'Tests LoadBalancer service functionality'
    };
    
    return descriptions[name] || `Validates ${name.replace(/-/g, ' ')} functionality`;
  }

  /**
   * Infer category based on test name
   */
  inferCategory(name) {
    if (name.includes('pod-to-pod') || name.includes('service-') || name.includes('dns-resolution')) {
      return 'Networking';
    } else if (name.includes('cidr') || name.includes('endpoints') || name.includes('entities') || 
               name.includes('node-') || name.includes('allow-all') || name.includes('deny-all')) {
      return 'L3 Policies';
    } else if (name.includes('tcp-port') || name.includes('port-') || name.includes('icmp') || name.includes('sni')) {
      return 'L4 Policies';
    } else if (name.includes('http') || name.includes('dns-match')) {
      return 'L7 Policies';
    }
    return 'General';
  }

  /**
   * Get execution name for backend (simple name without context)
   */
  getExecutionName() {
    return this.name;
  }

  /**
   * Get user-friendly display name
   */
  getDisplayName() {
    return this.displayName;
  }

  /**
   * Check if this test has the same purpose as another
   */
  hasSamePurpose(other) {
    return this.name === other.name && this.reason === other.reason;
  }

  /**
   * Create a plain object representation
   */
  toObject() {
    return {
      id: this.id,
      name: this.name,
      displayName: this.displayName,
      description: this.description,
      reason: this.reason,
      category: this.category,
      source: this.source,
      metadata: this.metadata
    };
  }
}

/**
 * Utility functions for working with test collections
 */
export class TestCollection {
  constructor(tests = []) {
    this.tests = new Map();
    
    // Convert input tests to TestDefinition objects
    tests.forEach(test => {
      const testDef = test instanceof TestDefinition ? test : new TestDefinition(test);
      this.tests.set(testDef.id, testDef);
    });
  }

  /**
   * Add test to collection
   */
  add(test) {
    const testDef = test instanceof TestDefinition ? test : new TestDefinition(test);
    this.tests.set(testDef.id, testDef);
    return testDef;
  }

  /**
   * Get test by ID
   */
  get(id) {
    return this.tests.get(id);
  }

  /**
   * Get all tests as array
   */
  getAll() {
    return Array.from(this.tests.values());
  }

  /**
   * Get execution names for backend
   */
  getExecutionNames() {
    return this.getAll().map(test => test.getExecutionName());
  }

  /**
   * Find tests by name (may return multiple for different purposes)
   */
  findByName(name) {
    return this.getAll().filter(test => test.name === name);
  }

  /**
   * Get size
   */
  size() {
    return this.tests.size;
  }

  /**
   * Clear all tests
   */
  clear() {
    this.tests.clear();
  }
}

/**
 * Helper function to normalize test input from various sources
 */
export function normalizeTestInput(input) {
  if (Array.isArray(input)) {
    return input.map(item => new TestDefinition(item));
  } else if (input) {
    return [new TestDefinition(input)];
  }
  return [];
}

/**
 * Create TestCollection from various input formats
 */
export function createTestCollection(input) {
  const normalizedTests = normalizeTestInput(input);
  return new TestCollection(normalizedTests);
}
