export default function handler(req, res) {
  if (req.method !== 'GET') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  // Comprehensive environment analysis
  const debugInfo = {
    timestamp: new Date().toISOString(),
    nodeEnv: process.env.NODE_ENV,
    kubernetesMode: process.env.KUBERNETES_MODE,
    kubernetesModeBool: process.env.KUBERNETES_MODE === 'true',
    cliServerUrl: process.env.CLI_SERVER_URL,
    useDocker: process.env.USE_DOCKER,
    sharedVolumePath: process.env.SHARED_VOLUME_PATH,
    port: process.env.PORT,
    allRelevantEnvVars: {}
  };

  // Capture all environment variables related to our app
  Object.keys(process.env).forEach(key => {
    if (key.includes('KUBERNETES') || 
        key.includes('CLI') || 
        key.includes('DOCKER') || 
        key.includes('NODE_ENV') ||
        key.includes('SHARED') ||
        key.includes('PORT')) {
      debugInfo.allRelevantEnvVars[key] = process.env[key];
    }
  });

  // Test Kubernetes mode detection logic exactly as in run-batch-tests.js
  const kubernetesMode = process.env.KUBERNETES_MODE === 'true';
  const isDevelopment = process.env.NODE_ENV !== 'production';
  const useDocker = process.env.USE_DOCKER === 'true' || !isDevelopment;

  debugInfo.detectionResults = {
    kubernetesMode,
    isDevelopment,
    useDocker,
    finalExecutionPath: kubernetesMode ? 'HTTP_API' : (useDocker ? 'DOCKER' : 'LOCAL')
  };

  console.log('[DEBUG ENV] Environment debug request:', debugInfo);

  res.status(200).json(debugInfo);
}
