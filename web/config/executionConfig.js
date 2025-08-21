/**
 * Unified Execution Configuration
 * Single configuration source for both production and development environments
 * Automatically detects environment and provides appropriate settings
 */

/**
 * Get unified execution configuration for current environment
 */
export const getExecutionConfig = () => {
  const isKubernetes = process.env.KUBERNETES_MODE === 'true';
  const isDevelopment = process.env.NODE_ENV !== 'production';
  
  return {
    // Execution mode
    mode: isKubernetes ? 'kubernetes' : 'docker-compose',
    
    // API endpoints
    cliEndpoint: isKubernetes 
      ? (process.env.CLI_SERVER_URL || 'http://localhost:8080')
      : null,
    eventStorageURL: process.env.HTTP_LOG_URL || 'http://localhost:3000',
    
    // Behavior settings
    useDocker: process.env.USE_DOCKER === 'true' || !isDevelopment,
    maxRetries: 3,
    pollInterval: 1000,
    maxPollInterval: 8000,
    healthCheckTimeout: 10000,
    cleanupTimeout: 60000,
    
    // Performance settings
    maxPollers: 3,
    maxEventDeduplication: 1000,
    eventHashLength: 16,
    
    // Debug settings
    enableDebugLogging: process.env.DEBUG === 'true',
    enableEventLogging: process.env.LOG_EVENTS === 'true',
    enableVerbosePolling: process.env.VERBOSE_POLLING === 'true',
    
    // Environment detection
    environment: {
      isKubernetes: isKubernetes,
      isDevelopment: isDevelopment,
      nodeEnv: process.env.NODE_ENV,
      kubernetesMode: process.env.KUBERNETES_MODE,
      useDocker: process.env.USE_DOCKER,
      sharedVolumePath: process.env.SHARED_VOLUME_PATH
    }
  };
};

/**
 * Get environment-specific paths and URLs
 */
export const getEnvironmentPaths = () => {
  const config = getExecutionConfig();
  
  if (config.mode === 'kubernetes') {
    return {
      projectRoot: '/app',
      binaryPath: '/app/k8s-diagnostic',
      resultsPath: config.environment.sharedVolumePath || '/app/shared/repository/test_results',
      workingDirectory: '/app'
    };
  } else {
    return {
      projectRoot: process.cwd() + '/..',
      binaryPath: process.cwd() + '/../k8s_diagnostic',
      resultsPath: process.cwd() + '/../test_results',
      workingDirectory: process.cwd() + '/..'
    };
  }
};

/**
 * Get polling configuration with smart defaults
 */
export const getPollingConfig = () => {
  const config = getExecutionConfig();
  
  return {
    initialInterval: config.pollInterval,
    maxInterval: config.maxPollInterval,
    backoffMultiplier: 2,
    backoffThreshold: 3, // Start backoff after 3 empty polls
    maxPollers: config.maxPollers,
    individualPollTimeout: 2000,
    batchPollTimeout: 5000
  };
};

/**
 * Get retry configuration for HTTP requests
 */
export const getRetryConfig = () => {
  const config = getExecutionConfig();
  
  return {
    maxRetries: config.maxRetries,
    baseDelay: 1000,
    maxDelay: 10000,
    exponentialBase: 2,
    jitterMaxMs: 500
  };
};

/**
 * Validate current configuration and environment
 */
export const validateConfiguration = () => {
  const config = getExecutionConfig();
  const paths = getEnvironmentPaths();
  const issues = [];
  
  // Validate required environment variables
  if (config.mode === 'kubernetes') {
    if (!config.cliEndpoint) {
      issues.push('CLI_SERVER_URL not configured for Kubernetes mode');
    }
    if (!config.environment.sharedVolumePath) {
      issues.push('SHARED_VOLUME_PATH not configured for Kubernetes mode');
    }
  }
  
  // Validate URLs
  try {
    new URL(config.eventStorageURL);
  } catch (error) {
    issues.push(`Invalid event storage URL: ${config.eventStorageURL}`);
  }
  
  if (config.cliEndpoint) {
    try {
      new URL(config.cliEndpoint);
    } catch (error) {
      issues.push(`Invalid CLI endpoint URL: ${config.cliEndpoint}`);
    }
  }
  
  return {
    valid: issues.length === 0,
    issues: issues,
    config: config,
    paths: paths
  };
};

/**
 * Log current configuration (for debugging)
 */
export const logConfiguration = () => {
  const config = getExecutionConfig();
  const paths = getEnvironmentPaths();
  
  console.log('[ExecutionConfig] Current configuration:');
  console.log(`  Mode: ${config.mode}`);
  console.log(`  Environment: ${config.environment.isDevelopment ? 'development' : 'production'}`);
  console.log(`  CLI Endpoint: ${config.cliEndpoint || 'local process'}`);
  console.log(`  Event Storage: ${config.eventStorageURL}`);
  console.log(`  Project Root: ${paths.projectRoot}`);
  console.log(`  Binary Path: ${paths.binaryPath}`);
  console.log(`  Results Path: ${paths.resultsPath}`);
  
  if (config.enableDebugLogging) {
    console.log('[ExecutionConfig] Debug settings enabled');
    console.log(`  Verbose Polling: ${config.enableVerbosePolling}`);
    console.log(`  Event Logging: ${config.enableEventLogging}`);
  }
};

// Default export
export default {
  getExecutionConfig,
  getEnvironmentPaths,
  getPollingConfig,
  getRetryConfig,
  validateConfiguration,
  logConfiguration
};
