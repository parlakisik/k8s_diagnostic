import fs from 'fs';
import path from 'path';

export default function handler(req, res) {
  if (req.method !== 'GET') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { testId } = req.query;
  
  if (!testId) {
    return res.status(400).json({ error: 'Test ID is required' });
  }

  try {
    // Project root directory (one level up from web/)
    const projectRoot = path.resolve(process.cwd(), '..');
    const logsDir = path.join(projectRoot, 'test_results', 'logs');
    
    console.log(`[STATUS API] Checking status for testId: ${testId}`);
    
    // Look for JSONL file that might match this test
    let jsonlFilePath = null;
    let jsonlFound = false;
    
    try {
      const files = fs.readdirSync(logsDir);
      const jsonlFiles = files.filter(file => file.endsWith('.frontend.jsonl'));
      
      if (jsonlFiles.length > 0) {
        // Get the most recent JSONL file
        const filesWithStats = jsonlFiles.map(file => {
          const filePath = path.join(logsDir, file);
          const stats = fs.statSync(filePath);
          return { file, filePath, mtime: stats.mtime };
        });
        
        filesWithStats.sort((a, b) => b.mtime - a.mtime);
        jsonlFilePath = filesWithStats[0].filePath;
        jsonlFound = true;
        
        console.log(`[STATUS API] Found JSONL file: ${filesWithStats[0].file}`);
      }
    } catch (dirErr) {
      console.log(`[STATUS API] Logs directory not accessible: ${dirErr.message}`);
    }

    if (!jsonlFound) {
      console.log(`[STATUS API] No JSONL file found for testId: ${testId}`);
      return res.json({
        testId,
        status: 'initializing',
        message: 'Test is starting up...',
        jsonlFound: false,
        events: [],
        testStates: {
          'infrastructure': { status: 'pending', progress: 0, message: 'Preparing test environment...' },
          'cleanup': { status: 'pending', progress: 0, message: 'Waiting to start...' },
          'test-execution': { status: 'pending', progress: 0, message: 'Test queued...' }
        }
      });
    }

    // Read and parse JSONL file
    const jsonlContent = fs.readFileSync(jsonlFilePath, 'utf8');
    const lines = jsonlContent.trim().split('\n').filter(line => line.length > 0);
    const events = [];
    
    for (const line of lines) {
      try {
        const event = JSON.parse(line);
        events.push(event);
      } catch (parseErr) {
        console.log(`[STATUS API] Failed to parse line: ${line}`);
      }
    }

    console.log(`[STATUS API] Parsed ${events.length} events from JSONL`);

    // Analyze events to determine test states
    const testStates = analyzeTestProgress(events);
    const overallStatus = determineOverallStatus(events);

    res.json({
      testId,
      status: overallStatus.status,
      message: overallStatus.message,
      jsonlFound: true,
      eventCount: events.length,
      lastEventTime: events.length > 0 ? events[events.length - 1].timestamp : null,
      events: events.slice(-10), // Return last 10 events for context
      testStates
    });

  } catch (error) {
    console.error(`[STATUS API] Error: ${error.message}`);
    res.status(500).json({ 
      error: 'Failed to get test status',
      details: error.message,
      testId
    });
  }
}

function analyzeTestProgress(events) {
  const states = {
    'infrastructure': { status: 'pending', progress: 0, message: 'Preparing test environment...' },
    'cleanup': { status: 'pending', progress: 0, message: 'Waiting to start...' },
    'test-execution': { status: 'pending', progress: 0, message: 'Test queued...' }
  };

  let infrastructureStarted = false;
  let cleanupStarted = false;
  let testStarted = false;
  
  for (const event of events) {
    const message = event.message || '';
    const context = event.context || '';
    const phase = event.phase || '';
    
    // Infrastructure phase detection
    if (message.includes('Collecting cluster infrastructure') || 
        message.includes('Infrastructure collection') ||
        context.includes('infrastructure')) {
      infrastructureStarted = true;
      states['infrastructure'].status = 'running';
      states['infrastructure'].message = 'Gathering cluster information...';
      states['infrastructure'].progress = 30;
    }
    
    if (message.includes('Infrastructure collection completed')) {
      states['infrastructure'].status = 'complete';
      states['infrastructure'].message = 'Cluster info collected ✓';
      states['infrastructure'].progress = 100;
    }

    // Cleanup phase detection
    if (message.includes('CLEANUP PHASE') || 
        message.includes('universal_cleanup') ||
        phase === 'cleanup') {
      cleanupStarted = true;
      states['cleanup'].status = 'running';
      states['cleanup'].message = 'Cleaning up resources...';
      states['cleanup'].progress = 40;
    }
    
    if (message.includes('universal_cleanup completed')) {
      states['cleanup'].status = 'complete';
      states['cleanup'].message = 'Environment cleaned ✓';
      states['cleanup'].progress = 100;
    }

    // Test execution phase
    if (message.includes('TESTING PHASE') || 
        message.includes('Running diagnostic tests') ||
        phase === 'testing') {
      testStarted = true;
      states['test-execution'].status = 'running';
      states['test-execution'].message = 'Executing connectivity test...';
      states['test-execution'].progress = 50;
    }
    
    if (event.type === 'TEST_COMPLETE' || message.includes('Test completed')) {
      states['test-execution'].status = 'complete';
      states['test-execution'].message = 'Test finished ✓';
      states['test-execution'].progress = 100;
    }
  }

  // Set intermediate states based on what's started
  if (infrastructureStarted && states['infrastructure'].status === 'running') {
    states['cleanup'].status = 'waiting';
    states['cleanup'].message = 'Ready to start cleanup...';
  }
  
  if (cleanupStarted && states['cleanup'].status === 'running') {
    states['test-execution'].status = 'waiting';
    states['test-execution'].message = 'Ready to execute test...';
  }

  return states;
}

function determineOverallStatus(events) {
  if (events.length === 0) {
    return { status: 'initializing', message: 'Test is starting up...' };
  }

  const lastEvent = events[events.length - 1];
  
  // Look for completion indicators
  for (const event of events.slice(-5)) {
    if (event.message && event.message.includes('Test completed successfully')) {
      return { status: 'complete', message: 'All tests completed successfully' };
    }
    if (event.message && event.message.includes('test failed')) {
      return { status: 'failed', message: 'Test execution failed' };
    }
  }

  // Determine current phase
  const recentMessages = events.slice(-3).map(e => e.message || '').join(' ');
  
  if (recentMessages.includes('Infrastructure collection')) {
    return { status: 'running', message: 'Collecting cluster information...' };
  }
  if (recentMessages.includes('CLEANUP PHASE')) {
    return { status: 'running', message: 'Cleaning up test environment...' };
  }
  if (recentMessages.includes('TESTING PHASE')) {
    return { status: 'running', message: 'Running connectivity tests...' };
  }
  
  return { status: 'running', message: 'Test in progress...' };
}
