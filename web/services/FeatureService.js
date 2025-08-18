import ciliumFeatures from '../config/cilium-features.json';

class FeatureService {
  constructor() {
    this.features = ciliumFeatures.features;
    this.validationFeatures = ciliumFeatures.validationFeatures;
    this.testDescriptions = ciliumFeatures.testDescriptions;
  }

  /**
   * Check if a configuration value should be considered "truthy"
   * Matches the logic in the Go backend
   */
  isTruthy(val) {
    if (!val || val === '') return false;
    
    const normalized = val.toString().toLowerCase().trim();
    return ['true', '1', 'yes', 'y', 'on', 'enabled', 'enable', 'strict', 'partial', 'probe'].includes(normalized);
  }

  /**
   * Check if a specific configuration value matches expected values for encryption features
   */
  isEncryptionEnabled(configData, encryptionType) {
    const encType = configData['encryption-type']?.toLowerCase().trim();
    const encMode = configData['encryption']?.toLowerCase().trim();
    const specificKey = configData[`enable-${encryptionType}`];
    
    return this.isTruthy(specificKey) || encType === encryptionType || encMode === encryptionType;
  }

  /**
   * Validate a single feature against configuration data
   */
  validateFeature(featureName, configData) {
    const feature = this.features[featureName];
    if (!feature) {
      return {
        name: featureName,
        enabled: false,
        error: 'Unknown feature',
        feature: null
      };
    }

    let enabled = false;
    let reason = 'Not enabled';

    // Special handling for encryption features
    if (featureName === 'wireguard') {
      enabled = this.isEncryptionEnabled(configData, 'wireguard');
    } else if (featureName === 'ipsec') {
      enabled = this.isEncryptionEnabled(configData, 'ipsec');
    } else {
      // Standard check: any of the config keys is truthy
      enabled = feature.configKeys.some(key => this.isTruthy(configData[key]));
    }

    // Generate specific reason based on config values
    if (enabled) {
      const enabledKey = feature.configKeys.find(key => this.isTruthy(configData[key]));
      const configValue = configData[enabledKey];
      reason = `Active (${enabledKey}=${configValue})`;
    }

    return {
      name: featureName,
      enabled,
      reason,
      feature: {
        ...feature,
        tests: enabled ? feature.tests : []
      }
    };
  }

  /**
   * Validate all features against configuration data
   */
  validateAllFeatures(configData) {
    const results = {
      enabledFeatures: [],
      availableFeatures: [],
      enabledCount: 0,
      availableCount: 0,
      summary: {}
    };

    Object.keys(this.features).forEach(featureName => {
      const validation = this.validateFeature(featureName, configData);
      
      if (validation.enabled) {
        results.enabledFeatures.push({
          name: featureName,
          status: 'enabled',
          displayName: validation.feature.displayName,
          category: validation.feature.category,
          priority: validation.feature.priority,
          description: validation.feature.description,
          reason: validation.reason,
          tests: validation.feature.tests
        });
        results.enabledCount++;
      } else {
        results.availableFeatures.push({
          name: featureName,
          status: 'available',
          displayName: validation.feature.displayName,
          category: validation.feature.category,
          priority: validation.feature.priority,
          description: validation.feature.description,
          useCase: validation.feature.useCase,
          complexity: validation.feature.complexity,
          reason: validation.reason
        });
        results.availableCount++;
      }
    });

    results.summary = {
      enabledFeatures: results.enabledFeatures,
      availableFeatures: results.availableFeatures,
      enabledCount: results.enabledCount,
      availableCount: results.availableCount
    };

    return results;
  }

  /**
   * Generate test recommendations based on enabled features with config-aware reasoning
   */
  generateTestRecommendations(configData) {
    const recommendations = [];
    const seenTests = new Set();

    Object.keys(this.features).forEach(featureName => {
      const validation = this.validateFeature(featureName, configData);
      
      if (validation.enabled && validation.feature.tests) {
        // Find which config key actually enabled this feature
        const enabledKey = validation.feature.configKeys.find(key => this.isTruthy(configData[key]));
        const configValue = configData[enabledKey];
        
        validation.feature.tests.forEach(test => {
          if (!seenTests.has(test.name)) {
            seenTests.add(test.name);
            recommendations.push({
              name: test.name,
              testName: test.name,
              description: test.description,
              rationale: this.generateConfigAwareRationale(
                featureName, 
                validation.feature.displayName,
                enabledKey, 
                configValue, 
                test.rationale
              ),
              feature: validation.feature.displayName,
              configKey: enabledKey,
              configValue: configValue,
              originalRationale: test.rationale
            });
          }
        });
      }
    });

    return recommendations;
  }

  /**
   * Generate config-aware rationale that references actual configuration data
   */
  generateConfigAwareRationale(featureName, displayName, configKey, configValue, originalRationale) {
    // Create config-specific explanations based on actual values
    const contextualReasons = {
      'l7-proxy': `Your L7 proxy is enabled (${configKey}=${configValue}) - test HTTP method filtering and request inspection capabilities`,
      'dns-proxy': `DNS proxy is active (${configKey}=${configValue}) - validate FQDN-based policy filtering and DNS security`,
      'kube-proxy-replacement': `Kube-proxy replacement is running in ${configValue} mode (${configKey}=${configValue}) - test service mesh and load balancing thoroughly`,
      'host-firewall': `Host firewall is enabled (${configKey}=${configValue}) - validate node-level security and traffic filtering`,
      'wireguard': `WireGuard encryption is active (${configKey}=${configValue}) - verify encrypted pod-to-pod communication`,
      'ipsec': `IPsec encryption is enabled (${configKey}=${configValue}) - test encrypted network traffic between nodes`,
      'bgp-control-plane': `BGP control plane is active (${configKey}=${configValue}) - validate BGP load balancing and IP advertisement`,
      'egress-gateway': `Egress gateway is configured (${configKey}=${configValue}) - test centralized outbound traffic routing`,
      'l2-announcements': `L2 announcements are enabled (${configKey}=${configValue}) - validate layer 2 IP address announcements`,
      'nodeport': `NodePort services are enabled (${configKey}=${configValue}) - test direct node access and port forwarding`,
      'gateway-api': `Gateway API support is active (${configKey}=${configValue}) - test advanced ingress and traffic management capabilities`
    };

    // Return config-specific reason if available, otherwise create a generic one
    if (contextualReasons[featureName]) {
      return contextualReasons[featureName];
    }

    // Fallback: create a generic config-aware reason
    return `${displayName} is enabled in your configuration (${configKey}=${configValue}) - ${originalRationale.toLowerCase()}`;
  }

  /**
   * Generate configuration insights
   */
  generateInsights(configData) {
    const enabledFeatures = [];
    const recommendations = [];
    const configSummary = {};

    // Validate all features
    Object.keys(this.features).forEach(featureName => {
      const validation = this.validateFeature(featureName, configData);
      
      if (validation.enabled) {
        enabledFeatures.push(validation.feature.displayName);
        configSummary[validation.feature.displayName] = {
          enabled: true,
          description: validation.feature.description,
          configKeys: validation.feature.configKeys
        };
      }
    });

    // Generate smart recommendations
    if (enabledFeatures.length === 0) {
      recommendations.push("Basic Cilium installation detected - consider enabling advanced features like L7 proxy or DNS policies");
    } else {
      recommendations.push(`${enabledFeatures.length} advanced features are enabled - your Cilium setup includes enhanced capabilities`);
    }

    // Feature-specific recommendations
    if (enabledFeatures.includes('L7 Proxy')) {
      recommendations.push("L7 proxy is enabled - run HTTP policy tests to validate application-layer security");
    }

    if (enabledFeatures.includes('DNS Proxy')) {
      recommendations.push("DNS proxy is enabled - test DNS-based policies for FQDN filtering capabilities");
    }

    if (enabledFeatures.includes('Kube-proxy Replacement')) {
      recommendations.push("Kube-proxy replacement is active - test service mesh and load balancing thoroughly");
    }

    return {
      enabledFeatures,
      recommendedTests: this.generateTestRecommendations(configData),
      configSummary,
      recommendations
    };
  }

  /**
   * Get all available feature names
   */
  getAllFeatureNames() {
    return Object.keys(this.features);
  }

  /**
   * Get feature by name
   */
  getFeature(featureName) {
    return this.features[featureName] || null;
  }

  /**
   * Get test description
   */
  getTestDescription(testName) {
    return this.testDescriptions[testName] || 'Validates network functionality';
  }

  /**
   * Get features by category
   */
  getFeaturesByCategory(category) {
    return Object.entries(this.features)
      .filter(([_, feature]) => feature.category === category)
      .reduce((acc, [name, feature]) => {
        acc[name] = feature;
        return acc;
      }, {});
  }

  /**
   * Get features by priority
   */
  getFeaturesByPriority(priority) {
    return Object.entries(this.features)
      .filter(([_, feature]) => feature.priority === priority)
      .reduce((acc, [name, feature]) => {
        acc[name] = feature;
        return acc;
      }, {});
  }

  /**
   * Get tests associated with a specific feature
   */
  getTestsForFeature(featureName) {
    const feature = this.features[featureName] || this.validationFeatures[featureName];
    if (feature && feature.tests) {
      return feature.tests;
    }
    
    // Fallback - generate basic test recommendation for known enabled features
    const basicTests = {
      'gateway-api': [{
        name: 'basic-http-get',
        description: 'Validate HTTP connectivity and ingress routing',
        rationale: 'test advanced ingress and traffic management capabilities'
      }],
      'kube-proxy-replacement-strict': [{
        name: 'basic-connectivity',
        description: 'Test service mesh and load balancing',
        rationale: 'validate kube-proxy replacement functionality'
      }]
    };
    
    return basicTests[featureName] || [];
  }

  /**
   * Get validation feature metadata (used for CLI validation results)
   */
  getValidationFeatureMetadata(featureName) {
    const feature = this.validationFeatures[featureName];
    if (feature) {
      return feature;
    }
    
    // Fallback to default metadata
    return {
      displayName: featureName.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
      category: 'other',
      priority: 'optional',
      description: 'Advanced Cilium networking feature',
      useCase: 'Enable based on specific requirements',
      complexity: 'medium'
    };
  }

  /**
   * Parse validation results using our feature registry instead of hardcoded metadata
   */
  parseValidationResults(results) {
    const summary = {
      totalFeatures: 0,
      enabledFeatures: [],
      availableFeatures: [],
      enabledCount: 0,
      availableCount: 0,
      recommendations: [],
      recommendedTests: [], // Add this for test recommendations
      systemStatus: 'healthy'
    };

    results.forEach(line => {
      if (line.includes('[') && line.includes(']')) {
        summary.totalFeatures++;
        
        // Extract feature name from [feature-name] format
        const featureMatch = line.match(/\[([^\]]+)\]/);
        const featureName = featureMatch ? featureMatch[1] : 'unknown';
        const metadata = this.getValidationFeatureMetadata(featureName);
        
        if (line.includes('OK')) {
          summary.enabledCount++;
          
          const enabledFeature = {
            name: featureName,
            status: 'enabled',
            displayName: metadata.displayName,
            category: metadata.category,
            priority: metadata.priority,
            description: metadata.description,
            message: `✅ ${metadata.displayName} is active and working properly`
          };
          
          summary.enabledFeatures.push(enabledFeature);
          
          // Generate test recommendations for enabled features
          const featureTests = this.getTestsForFeature(featureName);
          if (featureTests && featureTests.length > 0) {
            featureTests.forEach(test => {
              // Avoid duplicate tests
              if (!summary.recommendedTests.find(t => t.name === test.name)) {
                summary.recommendedTests.push({
                  name: test.name,
                  testName: test.name,
                  description: test.description,
                  rationale: `${metadata.displayName} is enabled - ${test.rationale}`,
                  feature: metadata.displayName,
                  category: metadata.category,
                  priority: metadata.priority
                });
              }
            });
          }
          
        } else if (line.includes('FAIL')) {
          summary.availableCount++;
          
          // Extract helpful information from failure message
          let requirement = '';
          let configHint = '';
          
          if (line.includes('prerequisite not met:')) {
            const prereqMatch = line.match(/prerequisite not met: (.+?)(?:\. see docs|$)/);
            if (prereqMatch) {
              requirement = prereqMatch[1];
              configHint = `Configuration needed: ${requirement}`;
            }
          }
          
          summary.availableFeatures.push({
            name: featureName,
            status: 'available',
            displayName: metadata.displayName,
            category: metadata.category,
            priority: metadata.priority,
            description: metadata.description,
            useCase: metadata.useCase,
            complexity: metadata.complexity,
            requirement: requirement,
            configHint: configHint,
            message: `💡 ${metadata.displayName} can be enabled for ${metadata.useCase.toLowerCase()}`
          });
        }
      }
    });

    // Generate intelligent recommendations
    this.generateValidationRecommendations(summary);

    return summary;
  }

  /**
   * Generate recommendations for validation results
   */
  generateValidationRecommendations(summary) {
    const enabledNames = summary.enabledFeatures.map(f => f.name);
    const availableNames = summary.availableFeatures.map(f => f.name);
    const recommendedFeatures = summary.availableFeatures.filter(f => f.priority === 'recommended');
    const securityFeatures = summary.availableFeatures.filter(f => f.category === 'security');
    
    // System health assessment
    if (summary.enabledCount === 0) {
      summary.systemStatus = 'basic';
      summary.recommendations.push("📊 Basic Cilium configuration detected. Consider enabling recommended features for enhanced capabilities.");
    } else if (summary.enabledCount <= 2) {
      summary.systemStatus = 'intermediate';
      summary.recommendations.push(`🚀 ${summary.enabledCount} advanced features active. Your Cilium setup includes enhanced networking capabilities.`);
    } else {
      summary.systemStatus = 'advanced';
      summary.recommendations.push(`⭐ ${summary.enabledCount} advanced features active. You have a comprehensive Cilium deployment.`);
    }

    // Feature-specific recommendations
    if (enabledNames.includes('gateway-api')) {
      summary.recommendations.push("🌐 Gateway API is enabled - excellent for modern ingress and traffic management.");
    }
    
    if (recommendedFeatures.length > 0) {
      summary.recommendations.push(`💡 ${recommendedFeatures.length} recommended features available: ${recommendedFeatures.map(f => f.displayName).join(', ')}`);
    }
    
    if (securityFeatures.length > 0 && !enabledNames.some(name => ['wireguard', 'ipsec', 'host-firewall'].includes(name))) {
      summary.recommendations.push("🔒 Consider enabling security features like WireGuard encryption or Host Firewall for enhanced protection.");
    }
    
    // Combination recommendations
    if (availableNames.includes('dns-policies') && availableNames.includes('host-firewall')) {
      summary.recommendations.push("🛡️ DNS Policies + Host Firewall combination provides comprehensive security coverage.");
    }
    
    if (enabledNames.includes('gateway-api') && availableNames.includes('l2-announcements')) {
      summary.recommendations.push("📡 L2 Announcements complement Gateway API for on-premises load balancer scenarios.");
    }
  }
}

// Export singleton instance
const featureService = new FeatureService();
export default featureService;
