// Debug endpoint to trace test execution path and diagnose communication issues

export default async function handler(req, res) {
  if (req.method !== 'GET') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const debug = {
    timestamp: new Date().toISOString(),
    diagnosticInfo: {},
    environmentAnalysis: {},
    communicationTests: {},
    recommendations: [],
    overallStatus: 'unknown'
  };

  try {
    // 1. Environment Variable Analysis
    console.log('[DEBUG API] Starting environment variable analysis...');
    debug.environmentAnalysis = analyzeEnvironmentVariables();

    // 2. Kubernetes Mode Detection Test
    console.log('[DEBUG API] Testing Kubernetes mode detection...');
    debug.diagnosticInfo.kubernetesMode = testKubernetesModeDetection();

    // 3. CLI Container Communication Test
    console.log('[DEBUG API] Testing CLI container communication...');
    debug.communicationTests = await testCliContainerCommunication();

    // 4. Test Execution Path Analysis
    console.log('[DEBUG API] Analyzing test execution path...');
    debug.diagnosticInfo.executionPath = analyzeExecutionPath(debug.environmentAnalysis, debug.communicationTests);

    // 5. Generate Recommendations
    debug.recommendations = generateDebugRecommendations(debug);

    // 6. Determine Overall Status
    debug.overallStatus = determineOverallStatus(debug);

    console.log('[DEBUG API] Debug analysis completed:', {
      kubernetesMode: debug.diagnosticInfo.kubernetesMode,
      cliReachable: debug.communicationTests.healthCheck?.success || false,
      overallStatus: debug.overallStatus
    });

    res.status(200).json(debug);

  } catch (error) {
    console.error('[DEBUG API] Error during debug analysis:', error);
    debug.error = error.message;
    debug.overallStatus = 'error';
    res.status(500).json(debug);
  }
}

function analyzeEnvironmentVariables() {
  const analysis = {
    relevant: {},
    issues: [],
    recommendations: []
  };

  // Key environment variables to check
  const keyVars = [
    'NODE_ENV',
    'KUBERNETES_MODE', 
    'USE_DOCKER',
    'CLI_SERVER_URL',
    'SHARED_VOLUME_PATH',
    'PORT'
  ];

  keyVars.forEach(varName => {
    const value = process.env[varName];
    analysis.relevant[varName] = {
      value: value,
      type: typeof value,
      defined: value !== undefined,
      empty: !value || value === ''
    };

    // Check for common issues
    if (varName === 'KUBERNETES_MODE') {
      if (!value) {
        analysis.issues.push('KUBERNETES_MODE is not set - will not use HTTP API communication');
      } else if (value !== 'true') {
        analysis.issues.push(`KUBERNETES_MODE is "${value}" (not "true") - will not trigger Kubernetes mode`);
      }
    }

    if (varName === 'CLI_SERVER_URL' && process.env.KUBERNETES_MODE === 'true') {
      if (!value) {
        analysis.recommendations.push('CLI_SERVER_URL not set - using default http://localhost:8080');
      }
    }
  });

  // Additional environment analysis
  analysis.nodeEnv = process.env.NODE_ENV || 'development';
  analysis.isProduction = analysis.nodeEnv === 'production';
  analysis.kubernetesDetected = process.env.KUBERNETES_MODE === 'true';
  analysis.useDockerDetected = process.env.USE_DOCKER === 'true' || !analysis.isProduction;

  return analysis;
}

function testKubernetesModeDetection() {
  const test = {
    environmentValue: process.env.KUBERNETES_MODE,
    stringComparison: process.env.KUBERNETES_MODE === 'true',
    booleanEvaluation: Boolean(process.env.KUBERNETES_MODE),
    detectedMode: null,
    issues: []
  };

  // Simulate the exact logic from run-batch-tests.js
  const kubernetesMode = process.env.KUBERNETES_MODE === 'true';
  test.detectedMode = kubernetesMode ? 'kubernetes' : 'docker/local';

  if (!kubernetesMode && process.env.KUBERNETES_MODE) {
    test.issues.push(`Environment variable is set to "${process.env.KUBERNETES_MODE}" but not "true"`);
  }

  if (!kubernetesMode) {
    test.issues.push('Kubernetes mode not detected - will use Docker/local execution path');
  }

  return test;
}

async function testCliContainerCommunication() {
  const tests = {
    healthCheck: null,
    statusCheck: null,
    networkConnectivity: null,
    summary: {
      reachable: false,
      responding: false,
      issues: []
    }
  };

  const cliUrl = process.env.CLI_SERVER_URL || 'http://localhost:8080';

  // Test 1: Health endpoint
  try {
    console.log('[DEBUG API] Testing CLI health endpoint...');
    const healthResponse = await Promise.race([
      fetch(`${cliUrl}/api/health`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'k8s-diagnostic-debug-test'
        }
      }),
      new Promise((_, reject) => 
        setTimeout(() => reject(new Error('Health check timeout after 5s')), 5000)
      )
    ]);

    tests.healthCheck = {
      success: healthResponse.ok,
      status: healthResponse.status,
      statusText: healthResponse.statusText,
      headers: Object.fromEntries(healthResponse.headers.entries()),
      responseTime: null // Would need to measure this
    };

    if (healthResponse.ok) {
      const healthData = await healthResponse.json();
      tests.healthCheck.data = healthData;
      tests.summary.reachable = true;
      tests.summary.responding = true;
    } else {
      tests.summary.issues.push(`Health endpoint returned ${healthResponse.status}`);
    }

  } catch (error) {
    tests.healthCheck = {
      success: false,
      error: error.message,
      type: error.name
    };
    tests.summary.issues.push(`Health check failed: ${error.message}`);
  }

  // Test 2: Status endpoint
  if (tests.summary.reachable) {
    try {
      console.log('[DEBUG API] Testing CLI status endpoint...');
      const statusResponse = await fetch(`${cliUrl}/api/status`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
      });

      tests.statusCheck = {
        success: statusResponse.ok,
        status: statusResponse.status
      };

      if (statusResponse.ok) {
        const statusData = await statusResponse.json();
        tests.statusCheck.data = statusData;
      }

    } catch (error) {
      tests.statusCheck = {
        success: false,
        error: error.message
      };
    }
  }

  // Test 3: Basic network connectivity (simplified)
  tests.networkConnectivity = {
    cliUrl: cliUrl,
    localhost: cliUrl.includes('localhost'),
    port8080: cliUrl.includes(':8080'),
    httpProtocol: cliUrl.startsWith('http://')
  };

  return tests;
}

function analyzeExecutionPath(envAnalysis, commTests) {
  const analysis = {
    expectedPath: 'unknown',
    actualPath: 'unknown',
    pathMatches: false,
    issues: [],
    steps: []
  };

  // Determine expected path based on environment
  if (envAnalysis.kubernetesDetected) {
    analysis.expectedPath = 'kubernetes-http-api';
    analysis.steps = [
      'Detect KUBERNETES_MODE=true',
      'Skip Docker/local logic',
      'Perform CLI health check',
      'Execute tests via HTTP API calls',
      'Send individual HTTP requests for each test',
      'Parse HTTP responses for real test results'
    ];
  } else if (envAnalysis.useDockerDetected) {
    analysis.expectedPath = 'docker-compose';
    analysis.steps = [
      'Detect production mode or USE_DOCKER=true',
      'Check Docker Compose availability',
      'Spawn CLI container via docker compose',
      'Capture stdout/stderr from process',
      'Parse output for test results'
    ];
  } else {
    analysis.expectedPath = 'local-binary';
    analysis.steps = [
      'Development mode detected',
      'Look for local k8s_diagnostic binary',
      'Execute binary directly',
      'Capture stdout/stderr from process',
      'Parse output for test results'
    ];
  }

  // Analyze if path is working correctly
  if (analysis.expectedPath === 'kubernetes-http-api') {
    if (!commTests.summary.reachable) {
      analysis.issues.push('Expected Kubernetes HTTP API path but CLI container is not reachable');
      analysis.actualPath = 'kubernetes-http-api-failed';
    } else if (!commTests.summary.responding) {
      analysis.issues.push('CLI container reachable but not responding correctly');
      analysis.actualPath = 'kubernetes-http-api-partial';
    } else {
      analysis.actualPath = 'kubernetes-http-api';
    }
  }

  analysis.pathMatches = analysis.expectedPath === analysis.actualPath;

  return analysis;
}

function generateDebugRecommendations(debug) {
  const recommendations = [];

  // Environment variable recommendations
  if (debug.environmentAnalysis.issues.length > 0) {
    recommendations.push({
      category: 'Environment Variables',
      priority: 'high',
      issues: debug.environmentAnalysis.issues,
      actions: [
        'Verify KUBERNETES_MODE=true is set in deployment.yaml',
        'Check if environment variables are being loaded correctly in the UI container',
        'Restart the UI container after environment variable changes'
      ]
    });
  }

  // Communication recommendations
  if (!debug.communicationTests.summary.reachable) {
    recommendations.push({
      category: 'Container Communication',
      priority: 'critical',
      issues: ['CLI container is not reachable'],
      actions: [
        'Check if CLI container is running: kubectl get pods -n k8s-diagnostic',
        'Verify both containers are in the same pod and share localhost network',
        'Check CLI container logs for HTTP server startup messages',
        'Ensure port 8080 is not blocked by network policies'
      ]
    });
  }

  // Execution path recommendations
  if (debug.diagnosticInfo.executionPath && !debug.diagnosticInfo.executionPath.pathMatches) {
    recommendations.push({
      category: 'Test Execution Path',
      priority: 'high',
      issues: debug.diagnosticInfo.executionPath.issues,
      actions: [
        'Verify the correct execution path is being taken',
        'Check container logs for path detection messages',
        'Compare expected vs actual execution flow'
      ]
    });
  }

  // False positive recommendations
  if (debug.overallStatus === 'false-positives-likely') {
    recommendations.push({
      category: 'Result Validation',
      priority: 'critical',
      issues: ['Tests showing false positive results'],
      actions: [
        'Check CLI container logs during test execution for HTTP request receipts',
        'Compare UI test results with manual CLI execution',
        'Verify test result parsing is working correctly',
        'Enable result validation warnings in the UI'
      ]
    });
  }

  return recommendations;
}

function determineOverallStatus(debug) {
  const issues = [];

  // Critical issues
  if (!debug.environmentAnalysis.kubernetesDetected) {
    issues.push('kubernetes-mode-not-detected');
  }

  if (!debug.communicationTests.summary.reachable) {
    issues.push('cli-container-unreachable');
  }

  if (debug.diagnosticInfo.executionPath && !debug.diagnosticInfo.executionPath.pathMatches) {
    issues.push('execution-path-mismatch');
  }

  // Determine overall status
  if (issues.includes('cli-container-unreachable')) {
    return 'communication-failure';
  }

  if (issues.includes('kubernetes-mode-not-detected')) {
    return 'configuration-issue';
  }

  if (issues.includes('execution-path-mismatch')) {
    return 'path-issue';
  }

  if (debug.communicationTests.summary.reachable && debug.environmentAnalysis.kubernetesDetected) {
    // Everything looks good, but we should still check for false positives
    return 'likely-working';
  }

  return 'unknown';
}
