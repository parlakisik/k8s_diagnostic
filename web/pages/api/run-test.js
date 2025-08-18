import { spawn } from 'child_process';
import fs from 'fs';
import path from 'path';
// Removed JSONL monitoring - using HTTP API events instead

// Global state to track running tests
let runningTest = null;
let testQueue = [];

export default function handler(req, res) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { cliCommand, testId } = req.body;
  
  console.log(`[API] Test request received - TestID: ${testId}, Command: ${cliCommand}`);
  console.log(`[API] Current running test: ${runningTest ? runningTest.testId : 'None'}`);
  
  if (!cliCommand) {
    return res.status(400).json({ error: 'CLI command is required' });
  }

  // Check if a test is already running
  if (runningTest && runningTest.testId !== testId) {
    console.log(`[API] BLOCKING: Test ${testId} blocked because ${runningTest.testId} is already running`);
    return res.status(409).json({ 
      error: 'Another test is currently running',
      runningTestId: runningTest.testId,
      message: 'Please wait for the current test to complete before starting a new one'
    });
  }

  // If the same test is already running, reject duplicate
  if (runningTest && runningTest.testId === testId) {
    console.log(`[API] DUPLICATE: Test ${testId} is already running, rejecting duplicate request`);
    return res.status(409).json({ 
      error: 'This test is already running',
      testId: testId,
      message: 'This test is already in progress'
    });
  }

  // Parse the CLI command
  let command, args;
  if (cliCommand.includes('./k8s_diagnostic')) {
    const parts = cliCommand.trim().split(/\s+/);
    command = parts[0];
    args = parts.slice(1);
  } else {
    return res.status(400).json({ error: 'Invalid command format' });
  }

  // Mark this test as running
  runningTest = {
    testId: testId,
    startTime: new Date(),
    command: cliCommand
  };
  
  console.log(`[API] STARTING: Test ${testId} is now running`);

  // Set up Server-Sent Events (SSE) for real-time streaming
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache, no-store, must-revalidate',
    'Connection': 'keep-alive',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Cache-Control'
  });

  // Send initial connection confirmation
  res.write(`data: ${JSON.stringify({
    type: 'connected',
    message: 'Connected to test execution stream',
    timestamp: new Date().toISOString(),
    testId: testId || Date.now().toString()
  })}\n\n`);

  // Use mounted volume paths or local development paths
  const getProjectRoot = () => {
    return process.env.SHARED_VOLUME_PATH 
      ? path.join(process.env.SHARED_VOLUME_PATH, 'repository')
      : path.resolve(process.cwd(), '..');  // Local development
  };

  const projectRoot = getProjectRoot();
  
  console.log(`[API] SPAWNING: CLI process for test ${testId} - Command: ${command} ${args.join(' ')}`);
  
  // Check if running in Docker environment
  const isDockerEnvironment = process.env.SHARED_VOLUME_PATH !== undefined;
  
  let childProcess;
  if (isDockerEnvironment) {
    // Spawn CLI container instead of local binary
    console.log(`[API] DOCKER MODE: Running CLI in container`);
    childProcess = spawn('docker', [
      'compose',
      '--profile', 'cli', 
      'run', '--rm',
      '-e', `BATCH_TEST_ID=${testId}`,
      'k8s-diagnostic-cli',
      'test',  // Remove './k8s_diagnostic' prefix
      ...args.slice(1)
    ], {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env }
    });
  } else {
    // Local development - run binary directly
    console.log(`[API] LOCAL MODE: Running local binary`);
    childProcess = spawn(command, args, {
      cwd: projectRoot,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env }
    });
  }

  // Function to clear running test state
  const clearRunningTest = () => {
    if (runningTest && runningTest.testId === testId) {
      console.log(`[API] CLEARING: Test ${testId} is no longer running`);
      runningTest = null;
    }
  };

  let httpPollingInterval = null;
  let lastEventTime = new Date().toISOString();

  // Poll HTTP API for events instead of monitoring JSONL files
  const pollHttpEvents = async () => {
    try {
      const response = await fetch(`http://localhost:3000/api/log-events?testId=${testId}&since=${lastEventTime}`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
      });
      
      if (response.ok) {
        const data = await response.json();
        
        if (data.events && data.events.length > 0) {
          data.events.forEach(event => {
            try {
              // Transform HTTP API event for frontend
              const frontendEvent = transformHttpEvent(event);
              res.write(`data: ${JSON.stringify(frontendEvent)}\n\n`);
              
              // Update last event time
              if (event.timestamp) {
                lastEventTime = event.timestamp;
              }
            } catch (error) {
              console.error('Error transforming HTTP event:', error);
            }
          });
        }
      }
    } catch (error) {
      console.error('Error polling HTTP events:', error);
    }
  };

  // Start HTTP API polling every 500ms
  httpPollingInterval = setInterval(pollHttpEvents, 500);

  // Transform HTTP API event for frontend
  const transformHttpEvent = (event) => {
    console.log('[run-test.js] Raw HTTP event received:', event);
    
    // Handle user-friendly messages from HTTP API
    if (event.type === 'user_step' && event.userMessage) {
      const transformed = {
        type: mapPhaseToType(event.userMessage.phase),
        timestamp: event.timestamp || new Date().toISOString(),
        title: event.userMessage.title,
        description: event.userMessage.description,
        context: event.userMessage.context,
        status: event.userMessage.status,
        hints: event.userMessage.hints || [],
        isUserFriendly: true,
        technicalDetails: event.technicalDetails,
        showTechnicalDetails: false
      };
      console.log('[run-test.js] Transformed user-friendly event:', transformed);
      return transformed;
    }
    
    // Handle regular log events
    const transformed = {
      type: event.type || 'info',
      timestamp: event.timestamp || new Date().toISOString(),
      message: event.message || '',
      level: event.level || 'INFO',
      context: event.context,
      data: event.data || {},
      testId: event.testId
    };
    console.log('[run-test.js] Transformed regular event:', transformed);
    return transformed;
  };

  // Map user message phases to frontend event types
  const mapPhaseToType = (phase) => {
    switch (phase) {
      case 'environment': return 'environment_check';
      case 'setup': return 'resource_creation';
      case 'execution': return 'connectivity_test';
      default: return 'info';
    }
  };

  // Transform log entry from CLI format to frontend format (legacy)
  const transformLogEntry = (logEntry) => {
    const baseEvent = {
      type: logEntry.type || 'info',
      timestamp: logEntry.timestamp,
      message: logEntry.message,
      level: logEntry.level || 'INFO',
      context: logEntry.context,
      data: logEntry.data || {}
    };

    // Add hierarchy information if available
    if (logEntry.groupId) baseEvent.groupId = logEntry.groupId;
    if (logEntry.subgroupId) baseEvent.subgroupId = logEntry.subgroupId;
    if (logEntry.testId) baseEvent.testId = logEntry.testId;
    if (logEntry.phase) baseEvent.phase = logEntry.phase;

    // Transform specific event types for better frontend handling
    switch (logEntry.type) {
      case 'SUITE_START':
        return {
          ...baseEvent,
          type: 'suite_start',
          totalTests: logEntry.data?.totalTests || 0,
          groups: logEntry.data?.groups || []
        };

      case 'TEST_START':
        return {
          ...baseEvent,
          type: 'test_start',
          testName: logEntry.data?.testName,
          progress: logEntry.data?.progress || 0
        };

      case 'TEST_COMPLETE':
        return {
          ...baseEvent,
          type: 'test_complete',
          testName: logEntry.data?.testName,
          success: logEntry.data?.success,
          result: logEntry.data?.result,
          duration: logEntry.data?.duration
        };

      case 'STEP':
        return {
          ...baseEvent,
          type: 'step',
          stepName: logEntry.data?.stepName,
          status: logEntry.data?.status
        };

      case 'COMMAND_RESULT':
        return {
          ...baseEvent,
          type: 'command_result',
          success: logEntry.data?.success,
          exitCode: logEntry.data?.exitCode,
          stdout: logEntry.data?.stdout,
          stderr: logEntry.data?.stderr
        };

      default:
        return baseEvent;
    }
  };

  // Also capture stdout/stderr as fallback
  childProcess.stdout.on('data', (data) => {
    const output = data.toString();
    
    // Send stdout as fallback log entry
    res.write(`data: ${JSON.stringify({
      type: 'stdout',
      data: output,
      timestamp: new Date().toISOString()
    })}\n\n`);
  });

  childProcess.stderr.on('data', (data) => {
    const output = data.toString();
    
    res.write(`data: ${JSON.stringify({
      type: 'stderr', 
      data: output,
      timestamp: new Date().toISOString()
    })}\n\n`);
  });

  // Handle process completion
  childProcess.on('close', (code) => {
    console.log(`[API] COMPLETED: Test ${testId} finished with exit code ${code}`);
    
    // Clear the running test state
    clearRunningTest();
    
    // Stop HTTP polling
    if (httpPollingInterval) {
      clearInterval(httpPollingInterval);
      httpPollingInterval = null;
    }

    const completion = {
      type: 'complete',
      exitCode: code,
      success: code === 0,
      timestamp: new Date().toISOString(),
      message: code === 0 ? 'Test completed successfully' : 'Test completed with errors'
    };
    
    res.write(`data: ${JSON.stringify(completion)}\n\n`);
    
    // Try to find and send the latest JSON results file
    setTimeout(() => {
      findLatestResultsFile(projectRoot)
        .then(resultsFile => {
          if (resultsFile) {
            const resultsPath = path.join(projectRoot, 'test_results', resultsFile);
            fs.readFile(resultsPath, 'utf8', (err, data) => {
              if (!err) {
                try {
                  const results = JSON.parse(data);
                  console.log(`[API] RESULTS: Sending test results for ${testId} - Success: ${results.success_rate}%`);
                  res.write(`data: ${JSON.stringify({
                    type: 'results',
                    data: results,
                    timestamp: new Date().toISOString()
                  })}\n\n`);
                } catch (parseErr) {
                  console.error('Error parsing results file:', parseErr);
                }
              }
              res.end();
            });
          } else {
            console.log(`[API] WARNING: No results file found for test ${testId}`);
            res.end();
          }
        })
        .catch(err => {
          console.error('Error finding results file:', err);
          res.end();
        });
    }, 1000);
  });

  // Handle process errors
  childProcess.on('error', (err) => {
    console.log(`[API] ERROR: Test ${testId} failed with process error: ${err.message}`);
    
    // Clear the running test state
    clearRunningTest();
    
    // Stop HTTP polling
    if (httpPollingInterval) {
      clearInterval(httpPollingInterval);
      httpPollingInterval = null;
    }
    
    const errorEntry = {
      type: 'error',
      message: `Process error: ${err.message}`,
      timestamp: new Date().toISOString()
    };
    res.write(`data: ${JSON.stringify(errorEntry)}\n\n`);
    res.end();
  });

  // Handle client disconnect
  req.on('close', () => {
    console.log(`[API] DISCONNECT: Client disconnected for test ${testId}`);
    
    // Clear the running test state
    clearRunningTest();
    
    if (childProcess && !childProcess.killed) {
      console.log(`[API] KILLING: Terminating process for disconnected test ${testId}`);
      childProcess.kill('SIGTERM');
    }
    
    // Stop HTTP polling
    if (httpPollingInterval) {
      clearInterval(httpPollingInterval);
      httpPollingInterval = null;
    }
  });
}

// Helper function to find the latest results file
async function findLatestResultsFile(projectRoot) {
  // Use shared volume path if available for results directory
  const resultsDir = process.env.SHARED_VOLUME_PATH 
    ? path.join(process.env.SHARED_VOLUME_PATH, 'repository', 'test_results')
    : path.join(projectRoot, 'test_results');
  
  try {
    const files = await fs.promises.readdir(resultsDir);
    const jsonFiles = files.filter(file => 
      file.startsWith('k8s-diagnostic-') && file.endsWith('.json')
    );
    
    if (jsonFiles.length === 0) {
      return null;
    }

    // Sort by modification time, most recent first
    const filesWithStats = await Promise.all(
      jsonFiles.map(async file => {
        const filePath = path.join(resultsDir, file);
        const stats = await fs.promises.stat(filePath);
        return { file, mtime: stats.mtime };
      })
    );

    filesWithStats.sort((a, b) => b.mtime - a.mtime);
    return filesWithStats[0].file;

  } catch (err) {
    console.error('Error reading results directory:', err);
    return null;
  }
}

// Configure body parser to handle JSON
export const config = {
  api: {
    bodyParser: {
      sizeLimit: '1mb',
    },
  },
}
