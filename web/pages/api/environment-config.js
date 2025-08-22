import { getExecutionConfig, generateCliCommand, getEnvironmentDisplayName } from '../../config/executionConfig';
import { spawn } from 'child_process';

// Get real pod name for kubectl commands
const getRealPodName = () => {
  return new Promise((resolve) => {
    // Try to get pod name from HOSTNAME environment variable first
    const hostname = process.env.HOSTNAME;
    if (hostname && hostname.startsWith('k8s-diagnostic-ui-')) {
      console.log(`[environment-config] Got pod name from HOSTNAME: ${hostname}`);
      resolve(hostname);
      return;
    }
    
    // Fallback: Use kubectl to get pod name
    const kubectl = spawn('kubectl', [
      'get', 'pods', '-n', 'k8s-diagnostic', 
      '-l', 'app=k8s-diagnostic-ui', 
      '-o', 'jsonpath={.items[0].metadata.name}'
    ]);
    
    let podName = '';
    kubectl.stdout.on('data', (data) => {
      podName += data.toString().trim();
    });
    
    kubectl.on('close', (code) => {
      if (code === 0 && podName) {
        console.log(`[environment-config] Got pod name from kubectl: ${podName}`);
        resolve(podName);
      } else {
        console.log(`[environment-config] Failed to get pod name, using fallback`);
        resolve('[pod-name]');
      }
    });
    
    kubectl.on('error', () => {
      console.log(`[environment-config] kubectl error, using fallback pod name`);
      resolve('[pod-name]');
    });
    
    // Timeout after 3 seconds
    setTimeout(() => {
      kubectl.kill();
      resolve('[pod-name]');
    }, 3000);
  });
};

// Generate kubectl command with real pod name
const generateKubectlCommand = async (testName, podName) => {
  return `kubectl exec -it ${podName} -n k8s-diagnostic -c cli -- ./k8s-diagnostic test list: ${testName} --verbose`;
};

export default async function handler(req, res) {
  if (req.method !== 'GET') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  try {
    const config = getExecutionConfig();
    const environmentName = getEnvironmentDisplayName();
    
    // Get real pod name if in Kubernetes mode
    let realPodName = null;
    if (config.environment.isKubernetes) {
      realPodName = await getRealPodName();
    }
    
    // Generate commands with real pod name
    const testNames = ['pod-to-pod-cross-node', 'service-clusterip', 'dns-resolution'];
    const sampleCommands = {};
    
    for (const testName of testNames) {
      if (config.environment.isKubernetes && realPodName) {
        sampleCommands[testName] = await generateKubectlCommand(testName, realPodName);
      } else {
        sampleCommands[testName] = generateCliCommand(testName);
      }
    }
    
    // Get sample CLI command for demo
    const sampleTestName = req.query.testName || 'pod-to-pod-cross-node';
    const cliCommand = sampleCommands[sampleTestName] || generateCliCommand(sampleTestName);
    
    return res.status(200).json({
      mode: config.mode,
      environmentName: environmentName,
      isKubernetes: config.environment.isKubernetes,
      isDevelopment: config.environment.isDevelopment,
      realPodName: realPodName,
      cliCommand: cliCommand,
      sampleCommands: sampleCommands,
      timestamp: new Date().toISOString()
    });
    
  } catch (error) {
    console.error('Error getting environment config:', error);
    return res.status(500).json({ 
      error: 'Failed to get environment configuration',
      details: error.message 
    });
  }
}
