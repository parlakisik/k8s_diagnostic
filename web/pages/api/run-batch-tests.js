import { spawn } from 'child_process';
import path from 'path';
import fs from 'fs';

// 🛡️ ENHANCED: Advanced process state tracking with individual test synchronization
let runningTests = new Map(); // Track running batch tests
let activeTestProcesses = new Map(); // Track individual test processes: testName -> { pid, startTime, status }
let testStateSync = new Map(); // Synchronize test states with frontend: testId -> { testStates, lastUpdate }
let processLocks = new Map(); // Prevent concurrent process operations: testId -> Promise

// Docker availability check - no binary building needed
async function ensureDockerIsAvailable(projectRoot, res, testId) {
  try {
    res.write(`data: ${JSON.stringify({
      type: 'build_start',
      message: '🐳 Preparing Docker containers...',
      testId: testId
    })}\n\n`);
    
    // Check if docker compose is available
    const dockerProcess = spawn('docker', ['compose', 'version'], {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe']
    });
    
    return new Promise((resolve) => {
      let output = '';
      let error = '';
      
      dockerProcess.stdout.on('data', (data) => {
        output += data.toString();
      });
      
      dockerProcess.stderr.on('data', (data) => {
        error += data.toString();
      });
      
      dockerProcess.on('close', (code) => {
        if (code === 0) {
          res.write(`data: ${JSON.stringify({
            type: 'build_complete',
            message: '✅ Docker containers ready, starting tests...',
            testId: testId
          })}\n\n`);
          resolve({ success: true });
        } else {
          console.error(`[BATCH API] Docker Compose not available:`, error);
          resolve({ 
            success: false, 
            error: `Docker Compose not available: ${error}` 
          });
        }
      });
      
      dockerProcess.on('error', (err) => {
        console.error(`[BATCH API] Docker check error:`, err);
        resolve({ 
          success: false, 
          error: `Docker check failed: ${err.message}` 
        });
      });
    });
    
  } catch (error) {
    console.error(`[BATCH API] Error checking Docker:`, error);
    return { success: false, error: `Failed to check Docker: ${error.message}` };
  }
}


// Process fallback messages when found under individual test names
function processFallbackMessages(fallbackData, testList, foundTestName) {
  
  const userMessages = {};
  
  // Initialize all tests as having no user message
  testList.forEach(testName => {
    userMessages[testName] = null;
  });

  // Process the fallback events
  if (fallbackData.events && Array.isArray(fallbackData.events)) {
    fallbackData.events.forEach(event => {
      if (event.type === 'user_step' && event.userMessage) {
        
        // For fallback, we already know this message belongs to foundTestName
        userMessages[foundTestName] = {
          title: event.userMessage.title,
          description: event.userMessage.description,
          context: event.userMessage.context,
          hints: event.userMessage.hints || [],
          status: event.userMessage.status
        };
        
      }
    });
  }
  
  return userMessages;
}

// Fetch user-friendly messages from HTTP API log events
async function fetchUserMessagesFromHTTP(testId, testList) {
  try {
    
    // Query the log-events API for user messages
    const response = await fetch(`http://localhost:${process.env.PORT || 3000}/api/log-events?testId=${testId}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      
      // Fallback: Try querying with individual test names
      
      for (const testName of testList) {
        try {
          const fallbackResponse = await fetch(`http://localhost:${process.env.PORT || 3000}/api/log-events?testId=${testName}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
          });
          
          if (fallbackResponse.ok) {
            const fallbackData = await fallbackResponse.json();
            
            if (fallbackData.events && fallbackData.events.length > 0) {
              // Process this fallback data and return it
              return processFallbackMessages(fallbackData, testList, testName);
            }
          }
        } catch (error) {
        }
      }
      
      return {};
    }

    const data = await response.json();
    
    const userMessages = {};
    
    // Initialize all tests as having no user message
    testList.forEach(testName => {
      userMessages[testName] = null;
    });

    // Process events to find user-friendly messages
    if (data.events && Array.isArray(data.events)) {
      
      data.events.forEach((event, index) => {
        
        if (event.type === 'user_step' && event.userMessage) {
          
          // Enhanced matching logic - try multiple approaches
          let matchingTestName = null;
          
          // Approach 1: Direct testName match
          if (event.testName && testList.includes(event.testName)) {
            matchingTestName = event.testName;
          }
          
          // Approach 2: Look for testName in testList that matches this event's testName
          if (!matchingTestName && event.testName) {
            matchingTestName = testList.find(testName => 
              testName === event.testName ||
              testName.includes(event.testName) ||
              event.testName.includes(testName)
            );
            if (matchingTestName) {
            }
          }
          
          // Approach 3: Look in userMessage content for test names
          if (!matchingTestName) {
            const messageContent = JSON.stringify(event.userMessage).toLowerCase();
            matchingTestName = testList.find(testName => 
              messageContent.includes(testName.toLowerCase())
            );
            if (matchingTestName) {
            }
          }
          
          // Approach 4: If only one test in testList, assume it's for that test
          if (!matchingTestName && testList.length === 1) {
            matchingTestName = testList[0];
          }
          
          if (matchingTestName) {
            userMessages[matchingTestName] = {
              title: event.userMessage.title,
              description: event.userMessage.description,
              context: event.userMessage.context,
              hints: event.userMessage.hints || [],
              status: event.userMessage.status
            };
          } else {
          }
        }
      });
    }
    
    
    return userMessages;
    
  } catch (error) {
    console.error(`[BATCH API] Error fetching user messages:`, error);
    return {};
  }
}

// 🛡️ ENHANCED: Protected test result extraction with state validation
async function extractTestResultsFromJSON(projectRoot, testList, testId) {
  // 🛡️ FIXED: Use correct path for Kubernetes deployment with shared volume
  let resultsDir;
  
  // Check if we're running in Kubernetes mode (same logic as CLI container)
  const kubernetesMode = process.env.KUBERNETES_MODE === 'true';
  const sharedVolumePath = process.env.SHARED_VOLUME_PATH;
  
  if (kubernetesMode && sharedVolumePath) {
    // In Kubernetes, use the shared volume path directly
    resultsDir = sharedVolumePath;
    console.log(`[BATCH API] Using Kubernetes shared volume path: ${resultsDir}`);
  } else if (sharedVolumePath) {
    // Use configured shared volume path
    resultsDir = sharedVolumePath;
    console.log(`[BATCH API] Using configured shared volume path: ${resultsDir}`);
  } else {
    // Fallback to project root for local development
    resultsDir = path.join(projectRoot, 'test_results');
    console.log(`[BATCH API] Using local development path: ${resultsDir}`);
  }
  
  try {
    // 🛡️ Validate input parameters
    if (!testList || !Array.isArray(testList) || testList.length === 0) {
      throw new Error('Invalid testList provided');
    }

    // Find the most recent JSON results file
    const files = await fs.promises.readdir(resultsDir);
    const jsonFiles = files.filter(file => 
      file.startsWith('k8s-diagnostic-') && file.endsWith('.json')
    );
    
    if (jsonFiles.length === 0) {
      console.log(`[BATCH API] No JSON results files found in ${resultsDir}`);
      
      // 🛡️ FIXED: Don't initialize with fake results - return error state instead
      const results = {};
      testList.forEach(testName => {
        // Update process state tracking
        updateTestProcessState(testId, testName, 'failed', 'No results file generated');
        
        results[testName] = {
          success: false,
          summary: 'No results file found - test execution may have failed',
          duration: null,
          command: `./k8s_diagnostic test list: ${testName}`,
          error: 'RESULTS_FILE_MISSING'
        };
      });
      return results;
    }

    // Sort by modification time, most recent first
    const filesWithStats = await Promise.all(
      jsonFiles.map(async file => {
        const filePath = path.join(resultsDir, file);
        const stats = await fs.promises.stat(filePath);
        return { file, mtime: stats.mtime, path: filePath };
      })
    );

    filesWithStats.sort((a, b) => b.mtime - a.mtime);
    const latestFile = filesWithStats[0];
    
    console.log(`[BATCH API] Reading results from: ${latestFile.file}`);
    
    // Read and parse the JSON file with error handling
    let testResults;
    try {
      const jsonContent = await fs.promises.readFile(latestFile.path, 'utf8');
      testResults = JSON.parse(jsonContent);
    } catch (parseError) {
      console.error(`[BATCH API] Failed to parse JSON results file:`, parseError);
      
      // 🛡️ Handle corrupted results file
      const results = {};
      testList.forEach(testName => {
        updateTestProcessState(testId, testName, 'failed', 'Results file corrupted');
        results[testName] = {
          success: false,
          summary: `Results file corrupted: ${parseError.message}`,
          duration: null,
          command: `./k8s_diagnostic test list: ${testName}`,
          error: 'RESULTS_FILE_CORRUPTED'
        };
      });
      return results;
    }
    
    console.log(`[BATCH API] Parsed results:`, {
      total_tests: testResults.total_tests,
      passed_tests: testResults.passed_tests,
      failed_tests: testResults.failed_tests,
      success_rate: testResults.success_rate
    });
    
    // 🛡️ FIXED: Validate results structure before processing
    const results = {};
    const processedTests = new Set();
    
    // Map JSON results to requested tests
    if (testResults.tests && Array.isArray(testResults.tests)) {
      testResults.tests.forEach(test => {
        // Find matching test in our requested list
        const matchingTestName = testList.find(requestedTest => 
          test.name === requestedTest || 
          test.name.includes(requestedTest) ||
          requestedTest.includes(test.name)
        );
        
        if (matchingTestName && !processedTests.has(matchingTestName)) {
          // Update process state tracking
          const status = test.success === true ? 'completed' : 'failed';
          updateTestProcessState(testId, matchingTestName, status, 
            test.success ? 'Test completed successfully' : 'Test failed');
          
          results[matchingTestName] = {
            success: test.success === true,
            summary: test.success ? 'PASSED - connectivity successful' : 'FAILED - connectivity blocked',
            duration: test.duration_seconds ? test.duration_seconds.toFixed(1) : null,
            command: `./k8s_diagnostic test list: ${matchingTestName}`
          };
          
          processedTests.add(matchingTestName);
          console.log(`[BATCH API] ✅ Mapped test result: ${matchingTestName} -> ${test.success ? 'PASSED' : 'FAILED'}`);
        }
      });
    }
    
    // 🛡️ ENHANCED: Handle tests that weren't found in results
    testList.forEach(testName => {
      if (!processedTests.has(testName)) {
        console.warn(`[BATCH API] ⚠️ Test ${testName} not found in results file`);
        updateTestProcessState(testId, testName, 'failed', 'Test not found in results');
        
        results[testName] = {
          success: false,
          summary: 'Test not found in results file - execution may have been skipped',
          duration: null,
          command: `./k8s_diagnostic test list: ${testName}`,
          error: 'TEST_NOT_IN_RESULTS'
        };
      }
    });
    
    return results;
    
  } catch (error) {
    console.error(`[BATCH API] Error reading JSON results:`, error);
    
    // 🛡️ ENHANCED: Better error handling with state updates
    const results = {};
    testList.forEach(testName => {
      updateTestProcessState(testId, testName, 'failed', `Result extraction error: ${error.message}`);
      results[testName] = {
        success: false,
        summary: `Error reading results: ${error.message}`,
        duration: null,
        command: `./k8s_diagnostic test list: ${testName}`,
        error: 'RESULT_EXTRACTION_ERROR'
      };
    });
    return results;
  }
}

// 🛡️ NEW: Atomic test process state management
const updateTestProcessState = (testId, testName, status, message) => {
  try {
    const timestamp = Date.now();
    
    // Update individual test process tracking
    activeTestProcesses.set(testName, {
      testId: testId,
      status: status,
      message: message,
      timestamp: timestamp,
      lastUpdate: new Date().toISOString()
    });
    
    // Update synchronized state for frontend consistency
    if (!testStateSync.has(testId)) {
      testStateSync.set(testId, { testStates: {}, lastUpdate: timestamp });
    }
    
    const syncData = testStateSync.get(testId);
    syncData.testStates[testName] = {
      status: status,
      message: message,
      timestamp: timestamp
    };
    syncData.lastUpdate = timestamp;
    
    console.log(`[BATCH API] 🔄 State updated: ${testName} -> ${status} (${message})`);
    
    // Validate state consistency
    validateStateConsistency(testId);
    
  } catch (error) {
    console.error(`[BATCH API] ❌ Failed to update test process state:`, error);
  }
};

// 🛡️ NEW: State consistency validation
const validateStateConsistency = (testId) => {
  try {
    const syncData = testStateSync.get(testId);
    if (!syncData) return;
    
    const runningProcess = runningTests.get(testId);
    if (!runningProcess) {
      console.warn(`[BATCH API] ⚠️ State inconsistency: No running process found for testId ${testId}`);
      return;
    }
    
    const expectedTests = runningProcess.testList || [];
    const trackedTests = Object.keys(syncData.testStates);
    
    // Check for missing tests
    const missingTests = expectedTests.filter(test => !trackedTests.includes(test));
    if (missingTests.length > 0) {
      console.warn(`[BATCH API] ⚠️ Missing test states: ${missingTests.join(', ')}`);
    }
    
    // Check for unexpected tests
    const unexpectedTests = trackedTests.filter(test => !expectedTests.includes(test));
    if (unexpectedTests.length > 0) {
      console.warn(`[BATCH API] ⚠️ Unexpected test states: ${unexpectedTests.join(', ')}`);
    }
    
  } catch (error) {
    console.error(`[BATCH API] ❌ State consistency validation error:`, error);
  }
};

// 🛡️ NEW: Process termination with individual test handling
const terminateTestProcess = async (testId, testName = null) => {
  // Check if termination is already in progress
  if (processLocks.has(testId)) {
    console.log(`[BATCH API] ⏳ Termination already in progress for ${testId}`);
    await processLocks.get(testId);
    return;
  }

  // Create termination lock
  const terminationPromise = new Promise(async (resolve) => {
    try {
      console.log(`[BATCH API] 🛑 Terminating process - testId: ${testId}, testName: ${testName}`);
      
      const runningProcess = runningTests.get(testId);
      if (!runningProcess) {
        console.warn(`[BATCH API] ⚠️ No running process found for testId: ${testId}`);
        resolve(false);
        return;
      }
      
      if (testName) {
        // Terminate specific test
        updateTestProcessState(testId, testName, 'terminated', 'Test terminated by user request');
        console.log(`[BATCH API] ✅ Test ${testName} marked as terminated`);
      } else {
        // Terminate entire batch
        if (runningProcess.testList) {
          runningProcess.testList.forEach(test => {
            updateTestProcessState(testId, test, 'terminated', 'Batch terminated by user request');
          });
        }
        
        // 🛡️ CRITICAL: Kill entire process group, not just parent
        if (runningProcess.childProcess && !runningProcess.childProcess.killed) {
          const pid = runningProcess.childProcess.pid;
          try {
            // Kill the entire process group (negative PID kills process group)
            process.kill(-pid, 'SIGTERM');
            console.log(`[BATCH API] 🔪 Killed entire process group (PGID: ${pid}) for testId: ${testId}`);
            
            // Set timeout for force kill if SIGTERM doesn't work
            setTimeout(() => {
              try {
                if (!runningProcess.childProcess.killed) {
                  process.kill(-pid, 'SIGKILL');
                  console.log(`[BATCH API] ⚡ Force killed process group (PGID: ${pid}) with SIGKILL`);
                }
              } catch (killError) {
                console.log(`[BATCH API] Process group already terminated or doesn't exist: ${killError.message}`);
              }
            }, 5000); // 5 second timeout for force kill
            
          } catch (killError) {
            console.warn(`[BATCH API] ⚠️ Failed to kill process group ${pid}: ${killError.message}, falling back to single process kill`);
            // Fallback to single process kill
            runningProcess.childProcess.kill('SIGTERM');
          }
        }
        
        // Clean up running tests
        runningTests.delete(testId);
        console.log(`[BATCH API] 🧹 Cleaned up running test: ${testId}`);
      }
      
      resolve(true);
    } catch (error) {
      console.error(`[BATCH API] ❌ Error during termination:`, error);
      resolve(false);
    }
  });
  
  processLocks.set(testId, terminationPromise);
  const result = await terminationPromise;
  
  // Clean up lock after delay
  setTimeout(() => {
    processLocks.delete(testId);
  }, 1000);
  
  return result;
};

// 🛡️ NEW: Get synchronized state for debugging/monitoring (safe for JSON serialization)
const getProcessState = (testId) => {
  const runningTest = runningTests.get(testId);
  
  // Create a safe copy without circular references (exclude res object)
  const safeRunningTest = runningTest ? {
    testList: runningTest.testList,
    startTime: runningTest.startTime,
    status: runningTest.status,
    lastActivity: runningTest.lastActivity,
    pid: runningTest.pid,
    exitCode: runningTest.exitCode,
    error: runningTest.error,
    endTime: runningTest.endTime
    // Note: res and childProcess objects excluded to prevent circular references
  } : null;
  
  return {
    runningTest: safeRunningTest,
    testStates: testStateSync.get(testId),
    activeProcesses: Array.from(activeTestProcesses.entries()).filter(([_, state]) => state.testId === testId)
  };
};

export default async function handler(req, res) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { testList, testId } = req.body;

  if (!testList || !Array.isArray(testList) || testList.length === 0) {
    return res.status(400).json({ error: 'Test list is required and must be a non-empty array' });
  }

  if (!testId) {
    return res.status(400).json({ error: 'Test ID is required' });
  }

  console.log(`[BATCH API] Batch test request received - TestID: ${testId}, Tests: ${testList.join(',')}`);

  // 🛡️ ENHANCED: Better concurrency management with detailed conflict resolution
  if (runningTests.size > 0) {
    const runningTestId = Array.from(runningTests.keys())[0];
    const runningProcess = runningTests.get(runningTestId);
    
    console.log(`[BATCH API] ⚠️ Conflict detected: ${testId} vs running ${runningTestId}`);
    
    // Check if the running process is actually still active
    const isStillActive = runningProcess.childProcess && !runningProcess.childProcess.killed;
    
    if (!isStillActive) {
      console.log(`[BATCH API] 🧹 Cleaning up stale running test: ${runningTestId}`);
      runningTests.delete(runningTestId);
      // Continue with new test
    } else {
      return res.status(409).json({ 
        error: 'Another test is currently running',
        runningTestId: runningTestId,
        runningTests: runningProcess.testList,
        startTime: runningProcess.startTime,
        message: 'Please wait for the current test to complete before starting a new one',
        currentState: getProcessState(runningTestId)
      });
    }
  }

  // Set response headers for Server-Sent Events
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.setHeader('Access-Control-Allow-Origin', '*');

  // 🛡️ ENHANCED: Store comprehensive process state with synchronization
  runningTests.set(testId, {
    testList: testList,
    startTime: new Date(),
    // Note: res object removed to prevent circular reference issues when serializing
    status: 'initializing',
    childProcess: null, // Will be set when process spawns
    lastActivity: Date.now()
  });
  
  // Initialize synchronized test states
  testStateSync.set(testId, {
    testStates: {},
    lastUpdate: Date.now()
  });
  
  // Initialize individual test process states
  testList.forEach(testName => {
    updateTestProcessState(testId, testName, 'queued', 'Waiting to start...');
  });

  console.log(`[BATCH API] STARTING: Batch test ${testId} is now running`);

  // Send initial connection event
  res.write(`data: ${JSON.stringify({
    type: 'connected',
    message: `Connected to batch test stream for ${testList.length} tests`,
    testId: testId,
    testList: testList
  })}\n\n`);
  res.flush();

  // CRITICAL FIX: Do NOT send fake cleanup messages in development mode
  // The real cleanup output will be captured from the CLI process stdout
  console.log(`[BATCH API] 📡 Development mode - cleanup will be captured from CLI process output`);

  // Build CLI command using the comma-separated list format
  const testListString = testList.join(',');
  const cliCommand = `./k8s_diagnostic test list: ${testListString} --verbose`;
  
  console.log(`[BATCH API] SPAWNING: CLI process for batch test ${testId} - Command: ${cliCommand}`);

  // Get project root directory (one level up from web/)
  const projectRoot = path.resolve(process.cwd(), '..');
  
  console.log(`[BATCH API] Project root: ${projectRoot}`);
  console.log(`[BATCH API] Current working directory: ${process.cwd()}`);
  
  // 🛡️ KUBERNETES MODE: Check FIRST before any other logic - ENHANCED DEBUGGING
  const kubernetesMode = process.env.KUBERNETES_MODE === 'true';
  console.log(`[BATCH API] 🔍 ENHANCED DEBUG: Environment variable analysis:`);
  console.log(`[BATCH API] NODE_ENV: "${process.env.NODE_ENV}" (${typeof process.env.NODE_ENV})`);
  console.log(`[BATCH API] KUBERNETES_MODE: "${process.env.KUBERNETES_MODE}" (${typeof process.env.KUBERNETES_MODE})`);
  console.log(`[BATCH API] USE_DOCKER: "${process.env.USE_DOCKER}" (${typeof process.env.USE_DOCKER})`);
  console.log(`[BATCH API] SHARED_VOLUME_PATH: "${process.env.SHARED_VOLUME_PATH}"`);
  console.log(`[BATCH API] CLI_SERVER_URL: "${process.env.CLI_SERVER_URL}"`);
  console.log(`[BATCH API] 🎯 KUBERNETES MODE DETECTED: ${kubernetesMode}`);
  console.log(`[BATCH API] 🔍 DEBUG: Boolean evaluations:`);
  console.log(`[BATCH API]   process.env.KUBERNETES_MODE === 'true': ${process.env.KUBERNETES_MODE === 'true'}`);
  console.log(`[BATCH API]   process.env.KUBERNETES_MODE == 'true': ${process.env.KUBERNETES_MODE == 'true'}`);
  console.log(`[BATCH API]   Boolean(process.env.KUBERNETES_MODE): ${Boolean(process.env.KUBERNETES_MODE)}`);
  console.log(`[BATCH API] 🔍 DEBUG: All environment variables containing 'KUBERNETES', 'CLI', 'DOCKER', 'NODE_ENV':`);
  Object.keys(process.env).filter(key => 
    key.includes('KUBERNETES') || key.includes('CLI') || key.includes('DOCKER') || key.includes('NODE_ENV')
  ).forEach(key => {
    console.log(`[BATCH API]   ${key}: "${process.env[key]}" (${typeof process.env[key]})`);
  });
  
  // 🚨 CRITICAL CHECK: Validate that we're actually detecting Kubernetes mode correctly
  if (!kubernetesMode) {
    console.log(`[BATCH API] ⚠️ WARNING: Not in Kubernetes mode - will use Docker/local execution`);
    console.log(`[BATCH API] ⚠️ If this is a Kubernetes deployment, check environment variables!`);
  } else {
    console.log(`[BATCH API] ✅ CONFIRMED: Running in Kubernetes mode - will use HTTP API`);
  }
  
  if (kubernetesMode) {
    console.log(`[BATCH API] 🚀 KUBERNETES MODE: Using HTTP API to communicate with CLI container`);
    console.log(`[BATCH API] 🔍 DEBUG: CLI container should be available at http://localhost:8080`);
    console.log(`[BATCH API] 📋 DEBUG: Test list to execute:`, testList);
    
    // 🚨 CRITICAL: Mandatory pre-execution validation
    console.log(`[BATCH API] 🛡️ MANDATORY PRE-EXECUTION VALIDATION:`);
    console.log(`[BATCH API]   ✅ Kubernetes mode confirmed: ${kubernetesMode}`);
    console.log(`[BATCH API]   📋 Tests to execute: ${testList.length} tests`);
    console.log(`[BATCH API]   🎯 Expected CLI endpoint: http://localhost:8080/api/execute-test`);
    
    // ENHANCED: Comprehensive CLI container health check with mandatory validation
    console.log(`[BATCH API] 🏥 MANDATORY: Testing CLI container health before test execution...`);
    console.log(`[BATCH API] 🔍 DEBUG: Environment variables:`);
    console.log(`[BATCH API]   NODE_ENV: ${process.env.NODE_ENV}`);
    console.log(`[BATCH API]   KUBERNETES_MODE: ${process.env.KUBERNETES_MODE}`);
    console.log(`[BATCH API]   CLI_SERVER_URL: ${process.env.CLI_SERVER_URL}`);
    
    const cliUrl = process.env.CLI_SERVER_URL || 'http://localhost:8080';
    console.log(`[BATCH API] 🎯 Using CLI URL: ${cliUrl}`);
    
    let healthCheckPassed = false;
    
    try {
      console.log(`[BATCH API] 📡 MANDATORY HEALTH CHECK: Attempting request to: ${cliUrl}/api/health`);
      
      const healthResponse = await Promise.race([
        fetch(`${cliUrl}/api/health`, {
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
            'User-Agent': 'k8s-diagnostic-ui-health-check'
          }
        }),
        new Promise((_, reject) => 
          setTimeout(() => reject(new Error('Health check timeout after 10s')), 10000)
        )
      ]);
      
      console.log(`[BATCH API] 📥 Health response received:`, {
        status: healthResponse.status,
        statusText: healthResponse.statusText,
        ok: healthResponse.ok,
        headers: Object.fromEntries(healthResponse.headers.entries())
      });
      
      if (healthResponse.ok) {
        const healthData = await healthResponse.json();
        console.log(`[BATCH API] ✅ CLI container health check PASSED:`, healthData);
        healthCheckPassed = true;
      } else {
        console.log(`[BATCH API] ❌ CLI container health check FAILED with status ${healthResponse.status}`);
        const healthText = await healthResponse.text();
        console.log(`[BATCH API] 📄 Health response text:`, healthText);
        healthCheckPassed = false;
      }
    } catch (healthError) {
      console.error(`[BATCH API] ❌ CLI container health check FAILED:`, {
        error: healthError.message,
        name: healthError.name,
        stack: healthError.stack,
        cause: healthError.cause
      });
      console.log(`[BATCH API] 🔧 This indicates CLI container communication failure`);
      console.log(`[BATCH API] 🔍 Possible causes:`);
      console.log(`[BATCH API]   - CLI container not running`);
      console.log(`[BATCH API]   - Port 8080 not accessible`);
      console.log(`[BATCH API]   - Network policy blocking localhost communication`);
      console.log(`[BATCH API]   - Container networking misconfiguration`);
      healthCheckPassed = false;
    }
    
    // 🚨 CRITICAL: Block execution if health check fails
    if (!healthCheckPassed) {
      console.error(`[BATCH API] 🚨 BLOCKING EXECUTION: CLI container health check failed!`);
      console.error(`[BATCH API] 🚨 This would result in fake positive results - ABORTING`);
      
      // Send error to all tests
      for (const testName of testList) {
        res.write(`data: ${JSON.stringify({
          type: 'test_complete',
          testName: testName,
          success: false,
          summary: `CLI container unreachable - health check failed`,
          duration: null,
          command: `HTTP API: ${testName}`,
          timestamp: new Date().toISOString(),
          error: 'CLI_CONTAINER_UNREACHABLE'
        })}\n\n`);
        res.flush();
      }
      
      res.write(`data: ${JSON.stringify({
        type: 'batch_complete',
        success: false,
        exitCode: 1,
        overallProgress: 100,
        message: 'Batch aborted - CLI container unreachable',
        timestamp: new Date().toISOString()
      })}\n\n`);
      
      runningTests.delete(testId);
      res.end();
      return;
    }
    
    // Execute tests via HTTP API calls to CLI container - ENHANCED WITH VALIDATION
    let httpRequestsMade = 0;
    let httpRequestsSuccessful = 0;
    let httpRequestsFailed = 0;
    
    console.log(`[BATCH API] 🚀 Starting HTTP API execution for ${testList.length} tests`);
    console.log(`[BATCH API] 🛡️ Health check passed - proceeding with test execution`);
    
    // 🚀 ENHANCED: Robust event polling with fallback mechanisms
    let eventPoller = null;
    let volumeEventPoller = null;
    let lastEventCount = 0;
    let eventPollingActive = true;
    let lastProcessedVolumeEvents = new Set();
    
    const startEnhancedEventPolling = () => {
      console.log(`[BATCH API] 🚀 Starting enhanced event polling for testId: ${testId}`);
      
      // Primary mechanism: HTTP event polling
      eventPoller = setInterval(async () => {
        if (!eventPollingActive) return;
        
        try {
          const eventResponse = await Promise.race([
            fetch(`http://localhost:${process.env.PORT || 3000}/api/log-events?testId=${testId}`, {
              method: 'GET',
              headers: { 
                'Content-Type': 'application/json',
                'Cache-Control': 'no-cache'
              }
            }),
            new Promise((_, reject) => 
              setTimeout(() => reject(new Error('Event polling timeout')), 5000)
            )
          ]);
          
          if (eventResponse.ok) {
            const eventData = await eventResponse.json();
            
            if (eventData.events && eventData.events.length > lastEventCount) {
              // Forward new events to BatchTestRunner via SSE
              const newEvents = eventData.events.slice(lastEventCount);
              
              for (const event of newEvents) {
                // Transform CLI events to BatchTestRunner expected format
                let forwardEvent = {
                  type: event.type || 'live_output',
                  testName: event.testId || event.testName,
                  message: event.message || event.line || '',
                  timestamp: event.timestamp || new Date().toISOString(),
                  source: 'http_api'
                };
                
                // Handle different event types
                if (event.type === 'progress_update') {
                  forwardEvent.type = 'live_output';
                  forwardEvent.output = event.message + '\n';
                } else if (event.type === 'test_start') {
                  forwardEvent.type = 'test_start';
                } else if (event.type === 'test_progress') {
                  forwardEvent.type = 'live_output';
                  forwardEvent.output = event.message + '\n';
                } else {
                  forwardEvent.type = 'live_output';
                  forwardEvent.output = event.message + '\n';
                }
                
                console.log(`[BATCH API] 📡 Forwarding HTTP event: ${forwardEvent.type} - ${forwardEvent.message?.substring(0, 50)}...`);
                
                res.write(`data: ${JSON.stringify(forwardEvent)}\n\n`);
                res.flush();
              }
              
              lastEventCount = eventData.events.length;
            }
          } else {
            console.log(`[BATCH API] ⚠️ HTTP event polling failed with status: ${eventResponse.status}`);
          }
        } catch (pollError) {
          console.log(`[BATCH API] ⚠️ HTTP event polling error: ${pollError.message}`);
        }
      }, 1000); // Poll every second
      
      // Fallback mechanism: Shared volume event polling
      volumeEventPoller = setInterval(async () => {
        if (!eventPollingActive) return;
        
        try {
          await pollSharedVolumeEvents(testId, res);
        } catch (volumeError) {
          console.log(`[BATCH API] 🔍 Volume event polling error (non-critical): ${volumeError.message}`);
        }
      }, 2000); // Poll shared volume every 2 seconds
    };
    
    // Enhanced function to poll shared volume events as fallback
    const pollSharedVolumeEvents = async (testId, responseStream) => {
      const sharedPath = process.env.SHARED_VOLUME_PATH || '/app/shared/repository/test_results';
      const eventsDir = path.join(sharedPath, 'events');
      
      try {
        // Check if events directory exists
        const fs = await import('fs');
        if (!fs.existsSync(eventsDir)) {
          return; // No events directory, skip
        }
        
        // Read all event files for this testId
        const eventFiles = fs.readdirSync(eventsDir)
          .filter(file => file.startsWith(`${testId}-`) && file.endsWith('.json'))
          .sort(); // Sort chronologically
        
        for (const eventFile of eventFiles) {
          const eventPath = path.join(eventsDir, eventFile);
          const eventId = `${testId}-${eventFile}`;
          
          // Skip already processed events
          if (lastProcessedVolumeEvents.has(eventId)) {
            continue;
          }
          
          try {
            const eventContent = fs.readFileSync(eventPath, 'utf8');
            const eventData = JSON.parse(eventContent);
            
            // Mark as processed
            lastProcessedVolumeEvents.add(eventId);
            
            // Transform and forward the event
            let forwardEvent = {
              type: eventData.type || 'live_output',
              testName: eventData.testName || testId,
              message: eventData.message || '',
              timestamp: eventData.timestamp || new Date().toISOString(),
              source: 'shared_volume'
            };
            
            if (eventData.type === 'progress_update') {
              forwardEvent.type = 'live_output';
              forwardEvent.output = eventData.message + '\n';
            } else if (eventData.type === 'test_start') {
              forwardEvent.type = 'test_start';
            } else if (eventData.type === 'test_progress') {
              forwardEvent.type = 'live_output';
              forwardEvent.output = eventData.message + '\n';
            } else {
              forwardEvent.type = 'live_output';
              forwardEvent.output = eventData.message + '\n';
            }
            
            console.log(`[BATCH API] 📂 Forwarding volume event: ${forwardEvent.type} - ${forwardEvent.message?.substring(0, 50)}...`);
            
            responseStream.write(`data: ${JSON.stringify(forwardEvent)}\n\n`);
            responseStream.flush();
            
          } catch (parseError) {
            console.log(`[BATCH API] ⚠️ Failed to parse volume event file ${eventFile}: ${parseError.message}`);
          }
        }
        
        // Clean up old processed event IDs to prevent memory leak
        if (lastProcessedVolumeEvents.size > 1000) {
          const eventArray = Array.from(lastProcessedVolumeEvents);
          const keepEvents = eventArray.slice(-500); // Keep last 500
          lastProcessedVolumeEvents = new Set(keepEvents);
        }
        
      } catch (dirError) {
        // Events directory doesn't exist or can't be read - this is normal
      }
    };
    
    // Start enhanced event polling
    startEnhancedEventPolling();
    
    // Send batch start event
    res.write(`data: ${JSON.stringify({
      type: 'batch_start',
      message: `Starting batch execution of ${testList.length} tests via HTTP API`,
      testId: testId,
      testCount: testList.length,
      timestamp: new Date().toISOString()
    })}\n\n`);
    res.flush();
    
    for (let i = 0; i < testList.length; i++) {
      const testName = testList[i];
      console.log(`[BATCH API] 📡 [${httpRequestsMade + 1}/${testList.length}] Executing test via HTTP API: ${testName}`);
      
      // Send test start event
      res.write(`data: ${JSON.stringify({
        type: 'test_start',
        testName: testName,
        message: `Starting test: ${testName}`,
        testIndex: i,
        totalTests: testList.length,
        timestamp: new Date().toISOString()
      })}\n\n`);
      res.flush();
      
      try {
        updateTestProcessState(testId, testName, 'running', 'Sending HTTP request to CLI container...');
        
        const requestPayload = {
          testId: testId,
          cliCommand: `./k8s-diagnostic test list: ${testName} --verbose`,
          args: ['--verbose']
        };
        
        console.log(`[BATCH API] 📤 SENDING HTTP REQUEST #${httpRequestsMade + 1}:`, {
          url: 'http://localhost:8080/api/execute-test',
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          payload: requestPayload,
          timestamp: new Date().toISOString()
        });
        
        // Send progress update
        res.write(`data: ${JSON.stringify({
          type: 'test_progress',
          testName: testName,
          message: `Executing via HTTP API...`,
          progress: Math.round(((i) / testList.length) * 100),
          timestamp: new Date().toISOString()
        })}\n\n`);
        res.flush();
        
        // 🔍 CRITICAL: Log before making the actual HTTP request
        console.log(`[BATCH API] 🌐 ABOUT TO MAKE HTTP REQUEST #${httpRequestsMade + 1} - CLI container should receive this!`);
        httpRequestsMade++;
        
        // Send test execution request to CLI container with timeout
        const httpResponse = await Promise.race([
          fetch('http://localhost:8080/api/execute-test', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestPayload)
          }),
          new Promise((_, reject) => 
            setTimeout(() => reject(new Error('HTTP request timeout after 120s')), 120000)
          )
        ]);
        
        console.log(`[BATCH API] 📥 HTTP RESPONSE #${httpRequestsMade} RECEIVED for ${testName}:`, {
          status: httpResponse.status,
          statusText: httpResponse.statusText,
          ok: httpResponse.ok,
          headers: Object.fromEntries(Array.from(httpResponse.headers.entries())),
          timestamp: new Date().toISOString()
        });
        
        if (!httpResponse.ok) {
          const errorText = await httpResponse.text();
          httpRequestsFailed++;
          console.error(`[BATCH API] ❌ HTTP ERROR RESPONSE #${httpRequestsMade} for ${testName}:`, {
            status: httpResponse.status,
            statusText: httpResponse.statusText,
            errorText: errorText,
            testName: testName
          });
          throw new Error(`HTTP ${httpResponse.status}: ${errorText}`);
        }
        
        const result = await httpResponse.json();
        httpRequestsSuccessful++;
        
        // 🔍 ENHANCED: Validate the response structure and detect fake results
        console.log(`[BATCH API] ✅ HTTP API RESPONSE #${httpRequestsMade} for ${testName}:`, {
          success: result.success,
          testId: result.testId,
          message: result.message,
          hasData: !!result.data,
          messageLength: result.message ? result.message.length : 0,
          timestamp: new Date().toISOString()
        });
        
        // 🚨 CRITICAL: VALIDATE RESULT AUTHENTICITY - BLOCK FAKE RESULTS
        let isValidResult = true;
        let validationWarnings = [];
        
        if (result.message && result.message.includes('executed via HTTP API')) {
          validationWarnings.push('Response contains generic HTTP API message - BLOCKED as fake');
          isValidResult = false;
        }
        
        if (result.success && (!result.message || result.message.length < 10)) {
          validationWarnings.push('Success response has suspiciously short message - BLOCKED as fake');
          isValidResult = false;
        }
        
        // 🚨 BLOCK OBVIOUSLY FAKE RESULTS
        if (!isValidResult) {
          console.error(`[BATCH API] 🚨 BLOCKING FAKE RESULT for ${testName}:`, validationWarnings);
          console.error(`[BATCH API] 🚨 This appears to be a false positive - treating as FAILED`);
          
          updateTestProcessState(testId, testName, 'failed', `Blocked fake result: ${validationWarnings.join(', ')}`);
          
          res.write(`data: ${JSON.stringify({
            type: 'test_complete',
            testName: testName,
            success: false,
            summary: `BLOCKED - Fake result detected: ${validationWarnings.join(', ')}`,
            duration: null,
            command: `HTTP API: ${testName}`,
            timestamp: new Date().toISOString(),
            validationWarnings: validationWarnings,
            blocked: true
          })}\n\n`);
          res.flush();
          continue;
        }
        
        updateTestProcessState(testId, testName, result.success ? 'completed' : 'failed', 
          result.success ? 'Test completed via HTTP API' : `Test failed: ${result.message}`);
        
        // Send test complete event to frontend with validation info
        res.write(`data: ${JSON.stringify({
          type: 'test_complete',
          testName: testName,
          success: result.success,
          summary: result.success ? 
            `PASSED - ${result.message}` : 
            `FAILED - ${result.message}`,
          duration: null,
          command: `HTTP API: ${testName}`,
          timestamp: new Date().toISOString(),
          validated: true
        })}\n\n`);
        res.flush();
        
      } catch (error) {
        httpRequestsFailed++;
        console.error(`[BATCH API] ❌ HTTP API ERROR #${httpRequestsMade} for ${testName}:`, {
          error: error.message,
          stack: error.stack,
          testName: testName,
          requestNumber: httpRequestsMade,
          timestamp: new Date().toISOString()
        });
        
        updateTestProcessState(testId, testName, 'failed', `HTTP API error: ${error.message}`);
        
        res.write(`data: ${JSON.stringify({
          type: 'test_complete',
          testName: testName,
          success: false,
          summary: `HTTP API Error: ${error.message}`,
          duration: null,
          command: `HTTP API: ${testName}`,
          timestamp: new Date().toISOString()
        })}\n\n`);
        res.flush();
      }
    }
    
    // 🔍 SUMMARY: Log HTTP request statistics
    console.log(`[BATCH API] 📊 HTTP REQUEST SUMMARY:`, {
      totalRequests: httpRequestsMade,
      successful: httpRequestsSuccessful,
      failed: httpRequestsFailed,
      successRate: httpRequestsMade > 0 ? ((httpRequestsSuccessful / httpRequestsMade) * 100).toFixed(1) + '%' : '0%'
    });
    
    if (httpRequestsMade === 0) {
      console.error(`[BATCH API] 🚨 CRITICAL: No HTTP requests were made despite being in Kubernetes mode!`);
    } else if (httpRequestsFailed === httpRequestsMade) {
      console.error(`[BATCH API] 🚨 CRITICAL: All HTTP requests failed - CLI container communication broken!`);
    } else {
      console.log(`[BATCH API] ✅ HTTP request execution completed successfully`);
    }
    
    // 🧹 CRITICAL: Stop event polling and clean up
    eventPollingActive = false;
    if (eventPoller) {
      clearInterval(eventPoller);
      console.log(`[BATCH API] 🛑 Stopped SSE event polling for testId: ${testId}`);
    }
    
    // Send batch completion
    res.write(`data: ${JSON.stringify({
      type: 'batch_complete',
      success: httpRequestsSuccessful > 0, // Based on actual HTTP success
      exitCode: httpRequestsSuccessful > 0 ? 0 : 1,
      overallProgress: 100,
      message: `Kubernetes HTTP API execution completed: ${httpRequestsSuccessful}/${httpRequestsMade} requests successful`,
      timestamp: new Date().toISOString()
    })}\n\n`);
    
    // Clean up and end
    runningTests.delete(testId);
    res.end();
    return;
  }

  // Environment detection for non-Kubernetes deployments
  const isDevelopment = process.env.NODE_ENV !== 'production';
  const useDocker = process.env.USE_DOCKER === 'true' || !isDevelopment;
  
  console.log(`[BATCH API] Environment: ${isDevelopment ? 'development' : 'production'}`);
  console.log(`[BATCH API] Use Docker: ${useDocker}`);
  
  let childProcess;
  
  if (useDocker) {
    // Production mode: Use Docker Compose
    console.log(`[BATCH API] Using Docker Compose to spawn CLI container`);
    
    // Ensure Docker Compose is available
    const dockerResult = await ensureDockerIsAvailable(projectRoot, res, testId);
    if (!dockerResult.success) {
      console.error(`[BATCH API] ERROR: Docker check failed:`, dockerResult.error);
      res.write(`data: ${JSON.stringify({
        type: 'batch_error',
        error: `❌ Docker check failed: ${dockerResult.error}`,
        timestamp: new Date().toISOString()
      })}\n\n`);
      runningTests.delete(testId);
      res.end();
      return;
    }
    
    // Spawn the CLI process using Docker Compose
    const dockerArgs = [
      'compose', 'run', '--rm', 
      'k8s-diagnostic-cli-standalone',  // Use standalone service with host networking
      'test', 'list:', testListString, '--verbose'
    ];
    console.log(`[BATCH API] Spawning Docker process with args:`, dockerArgs);
    
    childProcess = spawn('docker', dockerArgs, {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: true,  // 🛡️ CRITICAL: Create new process group to manage child processes
      env: { 
        ...process.env,
        BATCH_TEST_ID: testId  // Pass the batch test ID to the Go process
      }
    });
  } else {
    // Development mode: Use local Go binary
    console.log(`[BATCH API] Using local Go binary for development`);
    
    const localBinaryPath = path.join(projectRoot, 'k8s_diagnostic');
    
    // Check if local binary exists
    try {
      await fs.promises.access(localBinaryPath, fs.constants.F_OK);
      console.log(`[BATCH API] Local binary found: ${localBinaryPath}`);
    } catch (error) {
      console.error(`[BATCH API] ERROR: Local binary not found: ${localBinaryPath}`);
      res.write(`data: ${JSON.stringify({
        type: 'batch_error',
        error: `❌ Local Go binary not found at ${localBinaryPath}. Run 'go build -o k8s_diagnostic .' in the project root first.`,
        timestamp: new Date().toISOString()
      })}\n\n`);
      runningTests.delete(testId);
      res.end();
      return;
    }
    
    // Spawn the local CLI process
    const localArgs = ['test', 'list:', testListString, '--verbose'];
    console.log(`[BATCH API] Spawning local process with args:`, localArgs);
    
    childProcess = spawn(localBinaryPath, localArgs, {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: true,  // 🛡️ CRITICAL: Create new process group to manage child processes
      env: { 
        ...process.env,
        BATCH_TEST_ID: testId  // Pass the batch test ID to the Go process
      }
    });
  }
  
  // 🛡️ ENHANCED: Store process group ID for proper termination
  const processGroupId = childProcess.pid;
  console.log(`[BATCH API] Process spawned with PID: ${childProcess.pid}, PGID: ${processGroupId}`);
  
  console.log(`[BATCH API] Process PID: ${childProcess.pid}`);
  
  // 🛡️ ENHANCED: Store child process reference and update state
  const runningProcess = runningTests.get(testId);
  if (runningProcess) {
    runningProcess.childProcess = childProcess;
    runningProcess.pid = childProcess.pid;
    runningProcess.status = 'running';
  }

  let testResults = {};
  let overallProgress = 0;
  let completedTests = 0;
  let currentTest = null; // Maintain test context across lines
  let activeTests = []; // Track all currently running tests
  let testStarted = {}; // Track which tests have started

  // Initialize test results and tracking
  testList.forEach(test => {
    testResults[test] = { status: 'queued', message: 'Waiting to start...' };
    testStarted[test] = false;
  });

  console.log(`[BATCH API] Initialized tracking for tests:`, testList);

  // Collect all output for final result extraction
  let allOutput = '';
  let allErrorOutput = '';
  
  // DO NOT simulate events - if tests don't start, that's an error to surface

  // Handle stdout - parse SSE events and collect for result parsing
  childProcess.stdout.on('data', (data) => {
    const output = data.toString();
    allOutput += output;

    // Process each line for SSE events and regular output
    const lines = output.split('\n');
    
    lines.forEach(line => {
      const trimmedLine = line.trim();
      if (!trimmedLine) return;

      // Check for structured SSE events from Go CLI
      if (trimmedLine.startsWith('SSE_EVENT:')) {
        try {
          const eventData = JSON.parse(trimmedLine.substring(10));
          
          // Forward the event immediately to frontend
          res.write(`data: ${JSON.stringify(eventData)}\n\n`);
          res.flush();
          
          // 🛡️ ENHANCED: Update internal tracking with state synchronization
          if (eventData.type === 'test_start' && eventData.testName) {
            testStarted[eventData.testName] = true;
            updateTestProcessState(testId, eventData.testName, 'running', 'Test in progress...');
          }
          
          if (eventData.type === 'test_complete' && eventData.testName) {
            const status = eventData.success ? 'completed' : 'failed';
            const message = eventData.success ? 'Test completed successfully' : 'Test failed';
            updateTestProcessState(testId, eventData.testName, status, message);
          }
          
        } catch (parseError) {
          console.error(`[BATCH API] Failed to parse SSE event:`, parseError);
          console.error(`[BATCH API] Raw line:`, trimmedLine.substring(0, 200));
        }
      } else {
        // Send live output for terminal monitoring (non-SSE lines)
        res.write(`data: ${JSON.stringify({
          type: 'live_output',
          output: line + '\n',
          timestamp: new Date().toISOString()
        })}\n\n`);
        res.flush();
      }
    });
  });

  // Handle stderr
  childProcess.stderr.on('data', (data) => {
    const output = data.toString();
    allErrorOutput += output;
    console.log(`[BATCH API] stderr:`, output.substring(0, 200));
  });

  // Extract test results from CLI output
  const extractTestResults = (output, exitCode) => {
    const results = {};
    const lines = output.split('\n');
    
    for (const testName of testList) {
      // Look for test result patterns in the output
      let found = false;
      
      // Pattern 1: "✅ TestName: PASSED (2.3s)"
      const passPattern = new RegExp(`✅\\s+${testName}.*?PASSED.*?\\((\\d+\\.\\d+)s\\)`, 'i');
      const passMatch = output.match(passPattern);
      if (passMatch) {
        results[testName] = {
          success: true,
          summary: 'PASSED - no issues',
          duration: passMatch[1],
          command: `./k8s_diagnostic test list: ${testName}`
        };
        found = true;
      }

      // Pattern 2: "❌ TestName: FAILED (2.3s)"
      if (!found) {
        const failPattern = new RegExp(`❌\\s+${testName}.*?FAILED.*?\\((\\d+\\.\\d+)s\\)`, 'i');
        const failMatch = output.match(failPattern);
        if (failMatch) {
          // Try to extract error message from nearby lines
          const errorContext = extractErrorContext(output, testName);
          results[testName] = {
            success: false,
            summary: `FAILED - ${errorContext || 'see terminal for details'}`,
            duration: failMatch[1],
            command: `./k8s_diagnostic test list: ${testName}`
          };
          found = true;
        }
      }

      // Pattern 3: Tree format results
      if (!found) {
        const treePassPattern = new RegExp(`${testName}.*?✅.*?PASS`, 'i');
        const treeFailPattern = new RegExp(`${testName}.*?❌.*?FAIL`, 'i');
        
        if (treePassPattern.test(output)) {
          results[testName] = {
            success: true,
            summary: 'PASSED - no issues', 
            duration: null,
            command: `./k8s_diagnostic test list: ${testName}`
          };
          found = true;
        } else if (treeFailPattern.test(output)) {
          const errorContext = extractErrorContext(output, testName);
          results[testName] = {
            success: false,
            summary: `FAILED - ${errorContext || 'see terminal for details'}`,
            duration: null,
            command: `./k8s_diagnostic test list: ${testName}`
          };
          found = true;
        }
      }

      // If no explicit result found, that's an error - don't guess
      if (!found) {
        results[testName] = {
          success: false,
          summary: `No test result found - process may have failed (exit code: ${exitCode})`,
          duration: null,
          command: `./k8s_diagnostic test list: ${testName}`,
          error: 'DIAGNOSTIC_NEEDED'
        };
      }
    }

    return results;
  };

  // Extract error context for failed tests
  const extractErrorContext = (output, testName) => {
    const lines = output.split('\n');
    let errorLines = [];
    let foundTest = false;
    
    for (const line of lines) {
      const trimmedLine = line.trim();
      
      // Check if we're in the context of this test
      if (trimmedLine.includes(testName)) {
        foundTest = true;
        continue;
      }
      
      // If we found the test, look for error indicators
      if (foundTest) {
        if (trimmedLine.includes('Error:') || 
            trimmedLine.includes('Failed:') ||
            trimmedLine.includes('error:') ||
            trimmedLine.includes('failed:')) {
          errorLines.push(trimmedLine.replace(/^Error:\s*|^Failed:\s*|^error:\s*|^failed:\s*/i, ''));
        }
        
        // Stop looking after we see the next test or a separator
        if (trimmedLine.includes('Running test:') || 
            trimmedLine.includes('===') ||
            errorLines.length > 2) {
          break;
        }
      }
    }
    
    // Return the most relevant error message
    return errorLines.length > 0 ? errorLines[0].substring(0, 100) : null;
  };

  // Handle process completion
  childProcess.on('close', async (code) => {
    console.log(`[BATCH API] Process finished for test ${testId} with code: ${code}`);
    console.log(`[BATCH API] Reading results from JSON file and HTTP messages...`);

    // No fallback timeout to clear - we don't simulate events anymore

    // 🛡️ ENHANCED: Wait for file system and update process state
    console.log(`[BATCH API] 🔄 Process completion detected, updating final states...`);
    
    // Update running process status
    const currentProcess = runningTests.get(testId);
    if (currentProcess) {
      currentProcess.status = 'completing';
      currentProcess.exitCode = code;
    }
    
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Read both JSON results and HTTP user messages with enhanced state tracking
    const extractedResults = await extractTestResultsFromJSON(projectRoot, testList, testId);
    const userMessages = await fetchUserMessagesFromHTTP(testId, testList);
    
    // Send individual test completion events with rich user messages
    testList.forEach(testName => {
      const result = extractedResults[testName];
      const userMessage = userMessages[testName] || null;
      
      if (userMessage) {
      }
      
      res.write(`data: ${JSON.stringify({
        type: 'test_complete',
        testName: testName,
        success: result.success,
        summary: result.summary,
        duration: result.duration,
        command: result.command,
        userMessage: userMessage, // Add rich user message
        timestamp: new Date().toISOString()
      })}\n\n`);
    });

    // Send batch completion
    const allSuccess = Object.values(extractedResults).every(r => r.success);
    
    res.write(`data: ${JSON.stringify({
      type: 'batch_complete',
      success: allSuccess,
      exitCode: code,
      overallProgress: 100,
      message: allSuccess ? 'All tests completed successfully' : 'Some tests failed',
      timestamp: new Date().toISOString()
    })}\n\n`);

    // 🛡️ ENHANCED: Comprehensive cleanup with state synchronization
    console.log(`[BATCH API] 🧹 Performing comprehensive cleanup for ${testId}...`);
    
    // Update all test states to final completion
    testList.forEach(testName => {
      const result = extractedResults[testName];
      if (result) {
        const finalStatus = result.success ? 'completed' : 'failed';
        updateTestProcessState(testId, testName, finalStatus, result.summary);
      }
    });
    
    // Update running process to completed state
    const completedProcess = runningTests.get(testId);
    if (completedProcess) {
      completedProcess.status = 'completed';
      completedProcess.endTime = new Date();
    }
    
    // Clean up process tracking
    runningTests.delete(testId);
    
    // Clean up individual test states after delay (keep for debugging)
    setTimeout(() => {
      testList.forEach(testName => {
        activeTestProcesses.delete(testName);
      });
      testStateSync.delete(testId);
      console.log(`[BATCH API] 🗑️ Cleaned up state tracking for ${testId}`);
    }, 30000); // Keep states for 30 seconds for debugging
    
    res.end();
    
    console.log(`[BATCH API] ✅ COMPLETED: Batch test ${testId} finished with comprehensive cleanup`);
  });

  // 🛡️ ENHANCED: Process error handling with state synchronization
  childProcess.on('error', (error) => {
    console.error(`[BATCH API] Process error for test ${testId}:`, error);
    
    // Update all test states to error status
    testList.forEach(testName => {
      updateTestProcessState(testId, testName, 'error', `Process error: ${error.message}`);
    });
    
    // Update running process status
    const erroredProcess = runningTests.get(testId);
    if (erroredProcess) {
      erroredProcess.status = 'error';
      erroredProcess.error = error.message;
    }
    
    res.write(`data: ${JSON.stringify({
      type: 'batch_error',
      error: error.message,
      testId: testId,
      timestamp: new Date().toISOString()
    })}\n\n`);

    // Enhanced cleanup
    runningTests.delete(testId);
    res.end();
  });

  // 🛡️ ENHANCED: Client disconnect handling with proper termination
  req.on('close', () => {
    console.log(`[BATCH API] Client disconnected for test ${testId}`);
    
    // Use the enhanced termination function
    terminateTestProcess(testId).then(success => {
      if (success) {
        console.log(`[BATCH API] ✅ Successfully terminated process ${testId} after client disconnect`);
      } else {
        console.log(`[BATCH API] ⚠️ Failed to terminate process ${testId} after client disconnect`);
      }
    });
  });
}

// 🛡️ NEW: Export process state management functions for use by other APIs
export const getRunningTests = () => runningTests;
export const getActiveTestProcesses = () => activeTestProcesses;
export const getTestStateSync = () => testStateSync;
export { terminateTestProcess, getProcessState, updateTestProcessState };
