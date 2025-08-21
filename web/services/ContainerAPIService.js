/**
 * Container API Service for handling communication between UI and CLI containers
 * Supports both Kubernetes (HTTP) and local development (spawn) modes
 */

/**
 * Detects if running in Kubernetes environment
 * @returns {boolean} True if running in Kubernetes
 */
function detectKubernetesEnvironment() {
  return process.env.KUBERNETES_MODE === 'true';
}

/**
 * Gets the CLI container API URL
 * @returns {string} The base URL for CLI container API
 */
function getContainerAPIURL() {
  const cliServerURL = process.env.CLI_SERVER_URL || 'http://localhost:8080';
  return cliServerURL;
}

/**
 * Executes a test in Kubernetes environment by sending HTTP request to CLI container
 * @param {string} testId - Unique test identifier
 * @param {string} cliCommand - The CLI command to execute
 * @param {string[]} args - Additional arguments for the command
 * @returns {Promise<Response>} Fetch response from CLI container
 */
async function executeKubernetesTest(testId, cliCommand, args = []) {
  const apiURL = getContainerAPIURL();
  
  console.log(`[ContainerAPI] Executing Kubernetes test: ${testId}`);
  console.log(`[ContainerAPI] CLI Server URL: ${apiURL}`);
  console.log(`[ContainerAPI] Command: ${cliCommand}`);
  
  try {
    const response = await fetch(`${apiURL}/api/execute-test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        testId: testId,
        cliCommand: cliCommand,
        args: args
      })
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`CLI container request failed: ${response.status} - ${errorText}`);
    }

    console.log(`[ContainerAPI] Test request sent successfully for ${testId}`);
    return response;

  } catch (error) {
    console.error(`[ContainerAPI] Failed to execute test ${testId}:`, error);
    throw error;
  }
}

/**
 * Checks health of CLI container
 * @returns {Promise<Object>} Health check response
 */
async function checkCLIHealth() {
  const apiURL = getContainerAPIURL();
  
  try {
    const response = await fetch(`${apiURL}/api/health`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      }
    });

    if (!response.ok) {
      throw new Error(`Health check failed: ${response.status}`);
    }

    const healthData = await response.json();
    console.log('[ContainerAPI] CLI container health check passed:', healthData);
    return healthData;

  } catch (error) {
    console.error('[ContainerAPI] CLI container health check failed:', error);
    throw error;
  }
}

/**
 * Gets status information from CLI container
 * @returns {Promise<Object>} Status information
 */
async function getCLIStatus() {
  const apiURL = getContainerAPIURL();
  
  try {
    const response = await fetch(`${apiURL}/api/status`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      }
    });

    if (!response.ok) {
      throw new Error(`Status check failed: ${response.status}`);
    }

    const statusData = await response.json();
    console.log('[ContainerAPI] CLI container status:', statusData);
    return statusData;

  } catch (error) {
    console.error('[ContainerAPI] CLI container status check failed:', error);
    throw error;
  }
}

/**
 * ContainerAPIService class for managing container communication
 */
class ContainerAPIService {
  constructor() {
    this.isKubernetes = detectKubernetesEnvironment();
    this.cliServerURL = getContainerAPIURL();
    
    console.log(`[ContainerAPI] Initialized - Kubernetes mode: ${this.isKubernetes}`);
    if (this.isKubernetes) {
      console.log(`[ContainerAPI] CLI Server URL: ${this.cliServerURL}`);
    }
  }

  /**
   * Executes a test using the appropriate method based on environment
   * @param {string} testId - Unique test identifier
   * @param {string} cliCommand - The CLI command to execute
   * @param {string[]} args - Additional arguments
   * @returns {Promise<Response>} Response from test execution
   */
  async executeTest(testId, cliCommand, args = []) {
    if (this.isKubernetes) {
      return await executeKubernetesTest(testId, cliCommand, args);
    } else {
      // For local development, we'll return null to indicate fallback to spawn logic
      console.log('[ContainerAPI] Local development mode - using spawn logic');
      return null;
    }
  }

  /**
   * Checks if CLI container is ready
   * @returns {Promise<boolean>} True if CLI container is ready
   */
  async isCLIReady() {
    if (!this.isKubernetes) {
      return true; // Local development doesn't need container health checks
    }

    try {
      await checkCLIHealth();
      return true;
    } catch (error) {
      console.error('[ContainerAPI] CLI container not ready:', error);
      return false;
    }
  }

  /**
   * Gets environment information
   * @returns {Object} Environment information
   */
  getEnvironmentInfo() {
    return {
      isKubernetes: this.isKubernetes,
      cliServerURL: this.cliServerURL,
      kubernetesMode: process.env.KUBERNETES_MODE,
      sharedVolumePath: process.env.SHARED_VOLUME_PATH
    };
  }

  /**
   * Waits for CLI container to be ready with retry logic
   * @param {number} maxRetries - Maximum number of retries
   * @param {number} retryDelay - Delay between retries in milliseconds
   * @returns {Promise<boolean>} True if CLI container becomes ready
   */
  async waitForCLIReady(maxRetries = 10, retryDelay = 2000) {
    if (!this.isKubernetes) {
      return true;
    }

    console.log('[ContainerAPI] Waiting for CLI container to be ready...');
    
    for (let i = 0; i < maxRetries; i++) {
      try {
        if (await this.isCLIReady()) {
          console.log('[ContainerAPI] CLI container is ready!');
          return true;
        }
      } catch (error) {
        console.log(`[ContainerAPI] Attempt ${i + 1}/${maxRetries} failed:`, error.message);
      }

      if (i < maxRetries - 1) {
        console.log(`[ContainerAPI] Retrying in ${retryDelay}ms...`);
        await new Promise(resolve => setTimeout(resolve, retryDelay));
      }
    }

    console.error('[ContainerAPI] CLI container failed to become ready after maximum retries');
    return false;
  }
}

// Export functions for use in API handlers
module.exports = {
  ContainerAPIService,
  detectKubernetesEnvironment,
  getContainerAPIURL,
  executeKubernetesTest,
  checkCLIHealth,
  getCLIStatus
};
