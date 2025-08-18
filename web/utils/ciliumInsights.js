/**
 * Cilium Configuration Analysis and Test Recommendations
 * 
 * This utility provides intelligent analysis of Cilium configuration data
 * using the centralized FeatureService instead of hardcoded mappings.
 * 
 * Features:
 * - Configuration parsing and validation using JSON-based feature definitions
 * - Feature detection from config values using shared feature service
 * - Test recommendations based on enabled features from centralized config
 * - Integration with existing test categories through feature service
 */

import featureService from '../services/FeatureService.js';

/**
 * Helper function to check if a value is truthy (same logic as Go backend)
 * @param {*} val - Value to check
 * @returns {boolean} - True if value is considered truthy
 */
function isTruthy(val) {
  if (!val || val === '') return false;
  
  const normalized = val.toString().toLowerCase().trim();
  return ['true', '1', 'yes', 'y', 'on', 'enabled', 'enable', 'strict', 'partial', 'probe'].includes(normalized);
}

/**
 * Analyze Cilium configuration and generate insights using the feature service
 * @param {Object} config - Cilium configuration object
 * @returns {Object} - Analysis results with insights and recommendations
 */
export function analyzeCiliumConfig(config) {
  // Use the centralized feature service instead of hardcoded mappings
  return featureService.generateInsights(config);
}

/**
 * Get test recommendations by priority using feature service data
 * @param {Object} insights - Analysis insights object from feature service
 * @returns {Object} - Prioritized test recommendations
 */
export function getPrioritizedTestRecommendations(insights) {
  const priority = {
    high: [],
    medium: [],
    low: []
  };

  // Use feature service to determine priority based on JSON config
  if (insights.enabledFeatures && insights.enabledFeatures.length > 0) {
    // Always test basic networking first
    priority.high.push('pod-to-pod-same-node', 'pod-to-pod-cross-node', 'service-clusterip', 'dns-resolution');
    
    // Add feature-specific high priority tests based on enabled features
    insights.enabledFeatures.forEach(feature => {
      if (feature.priority === 'critical' || feature.priority === 'high') {
        if (feature.tests) {
          feature.tests.forEach(test => {
            if (!priority.high.includes(test.name)) {
              priority.high.push(test.name);
            }
          });
        }
      }
    });
  }

  // Medium priority: Features marked as recommended
  if (insights.enabledFeatures) {
    insights.enabledFeatures.forEach(feature => {
      if (feature.priority === 'recommended' || feature.priority === 'medium') {
        if (feature.tests) {
          feature.tests.forEach(test => {
            if (!priority.high.includes(test.name)) {
              priority.medium.push(test.name);
            }
          });
        }
      }
    });
  }

  // Low priority: Optional features
  if (insights.enabledFeatures) {
    insights.enabledFeatures.forEach(feature => {
      if (feature.priority === 'optional' || feature.priority === 'low') {
        if (feature.tests) {
          feature.tests.forEach(test => {
            if (!priority.high.includes(test.name) && !priority.medium.includes(test.name)) {
              priority.low.push(test.name);
            }
          });
        }
      }
    });
  }

  // Remove duplicates and clean up
  Object.keys(priority).forEach(level => {
    priority[level] = [...new Set(priority[level])];
  });

  return priority;
}

/**
 * Validate configuration for specific features using feature service
 * @param {Object} config - Cilium configuration
 * @param {string[]} features - Features to validate
 * @returns {Object} - Validation results
 */
export function validateConfigForFeatures(config, features) {
  // Use feature service validation instead of hardcoded logic
  return featureService.validateAllFeatures(config, features);
}

/**
 * Get configuration summary with categorized features using feature service
 * @param {Object} config - Cilium configuration
 * @returns {Object} - Categorized configuration summary
 */
export function getConfigSummary(config) {
  // Use feature service to generate summary based on JSON config
  const insights = featureService.generateInsights(config);
  
  const summary = {
    networking: {},
    security: {},
    observability: {},
    advanced: {},
    total_keys: Object.keys(config).length,
    enabled_features: 0
  };

  // Use feature service data to categorize instead of hardcoded logic
  if (insights.enabledFeatures) {
    insights.enabledFeatures.forEach(feature => {
      summary.enabled_features++;
      const category = feature.category?.toLowerCase() || 'advanced';
      
      if (category.includes('network')) {
        summary.networking[feature.name] = { 
          value: feature.configValue, 
          enabled: true,
          description: feature.description 
        };
      } else if (category.includes('security')) {
        summary.security[feature.name] = { 
          value: feature.configValue, 
          enabled: true,
          description: feature.description 
        };
      } else if (category.includes('observability')) {
        summary.observability[feature.name] = { 
          value: feature.configValue, 
          enabled: true,
          description: feature.description 
        };
      } else {
        summary.advanced[feature.name] = { 
          value: feature.configValue, 
          enabled: true,
          description: feature.description 
        };
      }
    });
  }

  return summary;
}

export default {
  analyzeCiliumConfig,
  getPrioritizedTestRecommendations,
  validateConfigForFeatures,
  getConfigSummary
};
