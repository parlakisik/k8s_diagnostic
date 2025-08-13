import { spawn } from 'child_process';
import path from 'path';
import fs from 'fs';

// 🛡️ ENHANCED: Advanced process state tracking with individual test synchronization
let runningTests = new Map(); // Track running batch tests
let activeTestProcesses = new Map(); // Track individual test processes: testName -> { pid, startTime, status }
let testStateSync = new Map(); // Synchronize test states with frontend: testId -> { testStates, lastUpdate }
let processLocks = new Map(); // Prevent concurrent process operations: testId -> Promise

// Auto-build functionality - similar to CLI ensureBinaryIsUpToDate()
async function ensureBinaryIsUpToDate(projectRoot, res, testId) {
  const binaryPath = path.join(projectRoot, 'k8s_diagnostic');
  
  try {
    // Check if binary exists
    const binaryStats = await fs.promises.stat(binaryPath).catch(() => null);
    
    if (!binaryStats) {
      // Binary doesn't exist, need to build
      console.log(`[BATCH API] Binary not found at ${binaryPath}, building...`);
      res.write(`data: ${JSON.stringify({
        type: 'build_start',
        message: '🔨 Binary not found, building k8s_diagnostic...',
        testId: testId
      })}\n\n`);
      
      return await buildBinary(projectRoot, res, testId);
    }
    
    // Check if any Go source files are newer than the binary
    const binaryModTime = binaryStats.mtime;
    let sourceModified = false;
    
    try {
      await checkSourceFiles(projectRoot, binaryModTime, (newer) => {
        sourceModified = newer;
      });
      
      if (sourceModified) {
        console.log(`[BATCH API] Source changes detected, rebuilding binary...`);
        res.write(`data: ${JSON.stringify({
          type: 'build_start',
          message: '🔨 Source changes detected, rebuilding binary...',
          testId: testId
        })}\n\n`);
        
        return await buildBinary(projectRoot, res, testId);
      }
    } catch (error) {
      console.log(`[BATCH API] Warning: Could not check source file timestamps: ${error.message}`);
      res.write(`data: ${JSON.stringify({
        type: 'build_start',
        message: '🔨 Rebuilding binary to be safe...',
        testId: testId
      })}\n\n`);
      
      return await buildBinary(projectRoot, res, testId);
    }
    
    // Binary is up to date
    console.log(`[BATCH API] Binary is up to date`);
    return { success: true, binaryPath };
    
  } catch (error) {
    console.error(`[BATCH API] Error checking binary status:`, error);
    return { success: false, error: `Failed to check binary status: ${error.message}` };
  }
}

// Check if any Go source files are newer than binary
async function checkSourceFiles(dir, binaryModTime, callback) {
  const items = await fs.promises.readdir(dir);
  
  for (const item of items) {
    const fullPath = path.join(dir, item);
    const stats = await fs.promises.stat(fullPath);
    
    if (stats.isDirectory()) {
      // Skip certain directories
      if (item === 'node_modules' || item === '.git' || item === 'web' || item === 'build' || item === 'test_results') {
        continue;
      }
      
      // Recursively check subdirectories
      await checkSourceFiles(fullPath, binaryModTime, callback);
    } else if (path.extname(item) === '.go') {
      if (stats.mtime > binaryModTime) {
        callback(true);
        return;
      }
    }
  }
}

// Build the binary - similar to CLI buildBinary()
function buildBinary(projectRoot, res, testId) {
  return new Promise((resolve) => {
    console.log(`[BATCH API] Starting Go build process...`);
    
    const buildProcess = spawn('go', ['build', '-o', 'k8s_diagnostic', '.'], {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env }
    });
    
    let buildOutput = '';
    let buildErrors = '';
    
    buildProcess.stdout.on('data', (data) => {
      const output = data.toString();
      buildOutput += output;
      console.log(`[BATCH API] Build stdout:`, output);
      
      res.write(`data: ${JSON.stringify({
        type: 'build_output',
        output: output,
        testId: testId
      })}\n\n`);
      res.flush();
    });
    
    buildProcess.stderr.on('data', (data) => {
      const output = data.toString();
      buildErrors += output;
      console.error(`[BATCH API] Build stderr:`, output);
      
      res.write(`data: ${JSON.stringify({
        type: 'build_output',
        output: output,
        testId: testId
      })}\n\n`);
    });
    
    buildProcess.on('close', (code) => {
      if (code === 0) {
        console.log(`[BATCH API] Build completed successfully`);
        res.write(`data: ${JSON.stringify({
          type: 'build_complete',
          message: '✅ Binary built successfully, starting tests...',
          testId: testId
        })}\n\n`);
        
        resolve({ 
          success: true, 
          binaryPath: path.join(projectRoot, 'k8s_diagnostic')
        });
      } else {
        console.error(`[BATCH API] Build failed with code ${code}`);
        const errorMessage = buildErrors || buildOutput || `Build process exited with code ${code}`;
        
        resolve({ 
          success: false, 
          error: `Build failed: ${errorMessage}` 
        });
      }
    });
    
    buildProcess.on('error', (error) => {
      console.error(`[BATCH API] Build process error:`, error);
      resolve({ 
        success: false, 
        error: `Build process failed: ${error.message}` 
      });
    });
  });
}

// Process fallback messages when found under individual test names
function processFallbackMessages(fallbackData, testList, foundTestName) {
  console.log(`[BATCH API] 🔄 Processing fallback messages for ${foundTestName}`);
  
  const userMessages = {};
  
  // Initialize all tests as having no user message
  testList.forEach(testName => {
    userMessages[testName] = null;
  });

  // Process the fallback events
  if (fallbackData.events && Array.isArray(fallbackData.events)) {
    fallbackData.events.forEach(event => {
      if (event.type === 'user_step' && event.userMessage) {
        console.log(`[BATCH API] 🎯 Processing fallback user_step:`, {
          eventTestName: event.testName,
          userMessageTitle: event.userMessage.title
        });
        
        // For fallback, we already know this message belongs to foundTestName
        userMessages[foundTestName] = {
          title: event.userMessage.title,
          description: event.userMessage.description,
          context: event.userMessage.context,
          hints: event.userMessage.hints || [],
          status: event.userMessage.status
        };
        
        console.log(`[BATCH API] 🎨 Fallback message stored for ${foundTestName}:`, {
          title: event.userMessage.title,
          description: event.userMessage.description
        });
      }
    });
  }
  
  return userMessages;
}

// Fetch user-friendly messages from HTTP API log events
async function fetchUserMessagesFromHTTP(testId, testList) {
  try {
    console.log(`[BATCH API] Fetching user messages for testId: ${testId}`);
    
    // Query the log-events API for user messages
    const response = await fetch(`http://localhost:${process.env.PORT || 3000}/api/log-events?testId=${testId}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      console.log(`[BATCH API] HTTP API not available or no messages found for testId: ${testId}`);
      
      // Fallback: Try querying with individual test names
      console.log(`[BATCH API] 🔄 Trying fallback approach with individual test names...`);
      
      for (const testName of testList) {
        try {
          const fallbackResponse = await fetch(`http://localhost:${process.env.PORT || 3000}/api/log-events?testId=${testName}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
          });
          
          if (fallbackResponse.ok) {
            const fallbackData = await fallbackResponse.json();
            console.log(`[BATCH API] ✅ Found messages for ${testName}: ${fallbackData.events?.length || 0} events`);
            
            if (fallbackData.events && fallbackData.events.length > 0) {
              // Process this fallback data and return it
              return processFallbackMessages(fallbackData, testList, testName);
            }
          }
        } catch (error) {
          console.log(`[BATCH API] Fallback failed for ${testName}:`, error.message);
        }
      }
      
      return {};
    }

    const data = await response.json();
    console.log(`[BATCH API] Retrieved ${data.events?.length || 0} events from HTTP API`);
    
    const userMessages = {};
    
    // Initialize all tests as having no user message
    testList.forEach(testName => {
      userMessages[testName] = null;
    });

    // Process events to find user-friendly messages
    if (data.events && Array.isArray(data.events)) {
      console.log(`[BATCH API] 🔍 Searching for user messages in ${data.events.length} events...`);
      
      data.events.forEach((event, index) => {
        console.log(`[BATCH API] 📋 Event ${index}:`, { 
          type: event.type, 
          testName: event.testName, 
          testId: event.testId,
          hasUserMessage: !!event.userMessage 
        });
        
        if (event.type === 'user_step' && event.userMessage) {
          console.log(`[BATCH API] 🎯 Found user_step event with userMessage:`, {
            eventTestName: event.testName,
            userMessageTitle: event.userMessage.title
          });
          
          // Enhanced matching logic - try multiple approaches
          let matchingTestName = null;
          
          // Approach 1: Direct testName match
          if (event.testName && testList.includes(event.testName)) {
            matchingTestName = event.testName;
            console.log(`[BATCH API] ✅ Match found via direct testName: ${matchingTestName}`);
          }
          
          // Approach 2: Look for testName in testList that matches this event's testName
          if (!matchingTestName && event.testName) {
            matchingTestName = testList.find(testName => 
              testName === event.testName ||
              testName.includes(event.testName) ||
              event.testName.includes(testName)
            );
            if (matchingTestName) {
              console.log(`[BATCH API] ✅ Match found via fuzzy testName: ${matchingTestName} (from ${event.testName})`);
            }
          }
          
          // Approach 3: Look in userMessage content for test names
          if (!matchingTestName) {
            const messageContent = JSON.stringify(event.userMessage).toLowerCase();
            matchingTestName = testList.find(testName => 
              messageContent.includes(testName.toLowerCase())
            );
            if (matchingTestName) {
              console.log(`[BATCH API] ✅ Match found via userMessage content: ${matchingTestName}`);
            }
          }
          
          // Approach 4: If only one test in testList, assume it's for that test
          if (!matchingTestName && testList.length === 1) {
            matchingTestName = testList[0];
            console.log(`[BATCH API] ✅ Match found via single test assumption: ${matchingTestName}`);
          }
          
          if (matchingTestName) {
            userMessages[matchingTestName] = {
              title: event.userMessage.title,
              description: event.userMessage.description,
              context: event.userMessage.context,
              hints: event.userMessage.hints || [],
              status: event.userMessage.status
            };
            console.log(`[BATCH API] 🎨 Stored user message for ${matchingTestName}:`, {
              title: event.userMessage.title,
              description: event.userMessage.description
            });
          } else {
            console.log(`[BATCH API] ❌ No match found for user_step event:`, {
              eventTestName: event.testName,
              availableTests: testList,
              userMessageTitle: event.userMessage.title
            });
          }
        }
      });
    }
    
    // Final debug log
    console.log(`[BATCH API] 📊 Final user messages mapping:`, userMessages);
    
    return userMessages;
    
  } catch (error) {
    console.error(`[BATCH API] Error fetching user messages:`, error);
    return {};
  }
}

// 🛡️ ENHANCED: Protected test result extraction with state validation
async function extractTestResultsFromJSON(projectRoot, testList, testId) {
  const resultsDir = path.join(projectRoot, 'test_results');
  
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

// 🛡️ NEW: Get synchronized state for debugging/monitoring
const getProcessState = (testId) => {
  return {
    runningTest: runningTests.get(testId),
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
    res: res,
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

  // Build CLI command using the comma-separated list format
  const testListString = testList.join(',');
  const cliCommand = `./k8s_diagnostic test list: ${testListString} --verbose`;
  
  console.log(`[BATCH API] SPAWNING: CLI process for batch test ${testId} - Command: ${cliCommand}`);

  // Get project root directory (one level up from web/)
  const projectRoot = path.resolve(process.cwd(), '..');
  
  console.log(`[BATCH API] Project root: ${projectRoot}`);
  console.log(`[BATCH API] Current working directory: ${process.cwd()}`);
  
  // Ensure binary is up to date (auto-build if needed)
  const buildResult = await ensureBinaryIsUpToDate(projectRoot, res, testId);
  if (!buildResult.success) {
    console.error(`[BATCH API] ERROR: Binary build failed:`, buildResult.error);
    res.write(`data: ${JSON.stringify({
      type: 'batch_error',
      error: `❌ Build failed: ${buildResult.error}`,
      timestamp: new Date().toISOString()
    })}\n\n`);
    runningTests.delete(testId);
    res.end();
    return;
  }
  
  const binaryPath = buildResult.binaryPath;
  console.log(`[BATCH API] Using binary at: ${binaryPath}`);
  
  // Spawn the CLI process
  const args = ['test', 'list:', testListString, '--verbose'];
  console.log(`[BATCH API] Spawning process with args:`, args);
  
  const childProcess = spawn(binaryPath, args, {
    cwd: projectRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: true,  // 🛡️ CRITICAL: Create new process group to manage child processes
    env: { 
      ...process.env,
      BATCH_TEST_ID: testId  // Pass the batch test ID to the Go process
    }
  });
  
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
          console.log(`[BATCH API] 🎯 Received SSE event from Go CLI:`, eventData);
          
          // Forward the event immediately to frontend
          res.write(`data: ${JSON.stringify(eventData)}\n\n`);
          res.flush();
          
          // 🛡️ ENHANCED: Update internal tracking with state synchronization
          if (eventData.type === 'test_start' && eventData.testName) {
            testStarted[eventData.testName] = true;
            updateTestProcessState(testId, eventData.testName, 'running', 'Test in progress...');
            console.log(`[BATCH API] ✅ Test started: ${eventData.testName} (from SSE event)`);
          }
          
          if (eventData.type === 'test_complete' && eventData.testName) {
            const status = eventData.success ? 'completed' : 'failed';
            const message = eventData.success ? 'Test completed successfully' : 'Test failed';
            updateTestProcessState(testId, eventData.testName, status, message);
            console.log(`[BATCH API] ✅ Test completed: ${eventData.testName} -> ${status} (from SSE event)`);
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
      
      console.log(`[BATCH API] ✅ Sending result for ${testName}:`, result);
      if (userMessage) {
        console.log(`[BATCH API] 🎨 Including user message for ${testName}:`, userMessage.title);
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
