import { spawn } from 'child_process';
import path from 'path';
import featureService from '../../services/FeatureService.js';

// Helper function to extract JSON from mixed output (debug messages + JSON)
function extractJSONFromOutput(output) {
  // Find the first opening brace
  const startIndex = output.indexOf('{');
  if (startIndex === -1) {
    throw new Error('No JSON object found in output');
  }

  // Find the matching closing brace
  let braceCount = 0;
  let endIndex = -1;
  
  for (let i = startIndex; i < output.length; i++) {
    if (output[i] === '{') {
      braceCount++;
    } else if (output[i] === '}') {
      braceCount--;
      if (braceCount === 0) {
        endIndex = i;
        break;
      }
    }
  }

  if (endIndex === -1) {
    throw new Error('No complete JSON object found in output');
  }

  return output.substring(startIndex, endIndex + 1);
}

export default function handler(req, res) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { operation, features } = req.body;

  if (!operation || !['config', 'validate'].includes(operation)) {
    return res.status(400).json({ error: 'Invalid operation. Supported operations: "config", "validate"' });
  }

  console.log(`[CILIUM CONFIG API] Request received - Operation: ${operation}`);

  // Set response headers for Server-Sent Events
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.setHeader('Access-Control-Allow-Origin', '*');

  // Send initial connection event
  res.write(`data: ${JSON.stringify({
    type: 'cilium_start',
    message: operation === 'config' ? '🔍 Fetching Cilium configuration...' : '✅ Validating Cilium features...',
    timestamp: new Date().toISOString()
  })}\n\n`);

  console.log(`[CILIUM CONFIG API] STARTING: ${operation} operation initiated`);

  // Get project root directory (one level up from web/)
  const projectRoot = path.resolve(process.cwd(), '..');
  
  // Build command arguments based on operation
  let args;
  if (operation === 'config') {
    args = ['cilium', 'configmap', '--json'];
  } else if (operation === 'validate') {
    if (features && features.length > 0) {
      args = ['cilium', 'validate', '--features', features.join(',')];
    } else {
      args = ['cilium', 'validate', '--all'];
    }
  }

  console.log(`[CILIUM CONFIG API] Executing: ./k8s_diagnostic ${args.join(' ')}`);

  // Spawn the cilium process
  const childProcess = spawn('./k8s_diagnostic', args, {
    cwd: projectRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env }
  });

  let configData = '';
  let validationResults = [];

  // Handle stdout
  childProcess.stdout.on('data', (data) => {
    const output = data.toString();
    console.log(`[CILIUM CONFIG API] stdout:`, output);

    if (operation === 'config') {
      // For config operation, accumulate JSON data
      configData += output;
    } else {
      // For validation operation, capture individual validation results
      const lines = output.split('\n');
      lines.forEach(line => {
        line = line.trim();
        if (!line) return;

        validationResults.push(line);

        // Only send the actual output line, no extra progress messages
        res.write(`data: ${JSON.stringify({
          type: 'validation_output',
          output: line,
          timestamp: new Date().toISOString()
        })}\n\n`);
      });
    }

    // Only send progress for config operation, not validation
    if (operation === 'config') {
      res.write(`data: ${JSON.stringify({
        type: 'config_progress',
        message: '📊 Reading configuration data...',
        timestamp: new Date().toISOString()
      })}\n\n`);
    }
  });

  // Handle stderr
  childProcess.stderr.on('data', (data) => {
    const output = data.toString();
    console.log(`[CILIUM CONFIG API] stderr:`, output);
    
    // Parse stderr and send as warnings or errors
    const lines = output.split('\n');
    lines.forEach(line => {
      line = line.trim();
      if (!line) return;

      res.write(`data: ${JSON.stringify({
        type: 'cilium_error',
        output: `⚠️ ${line}`,
        timestamp: new Date().toISOString()
      })}\n\n`);
    });
  });

  // Handle process completion
  childProcess.on('close', (code) => {
    console.log(`[CILIUM CONFIG API] Process finished with code: ${code}`);

    let responseData = {};
    let success = false;
    let message = '';

    if (operation === 'config') {
      // Config operation: only succeed if exit code is 0
      if (code === 0) {
        try {
          // Extract clean JSON from mixed output (debug messages + JSON)
          const jsonPart = extractJSONFromOutput(configData);
          const parsedConfig = JSON.parse(jsonPart);
          responseData = {
            config: parsedConfig,
            insights: generateConfigInsights(parsedConfig)
          };
          
          console.log(`[CILIUM CONFIG API] Successfully parsed config with ${Object.keys(parsedConfig).length} keys`);
          success = true;
          message = '✅ Configuration retrieved successfully';
        } catch (parseError) {
          console.error(`[CILIUM CONFIG API] JSON parse error:`, parseError);
          console.error(`[CILIUM CONFIG API] Raw output:`, configData.substring(0, 200) + '...');
          res.write(`data: ${JSON.stringify({
            type: 'cilium_error',
            error: `Failed to parse configuration JSON: ${parseError.message}`,
            timestamp: new Date().toISOString()
          })}\n\n`);
          res.end();
          return;
        }
      } else {
        success = false;
        message = '❌ Failed to retrieve Cilium configuration';
      }
    } else {
      // Validation operation: process results regardless of exit code
      // Exit code 1 just means some features failed validation (which is normal)
      console.log(`[CILIUM CONFIG API] Processing validation results: ${validationResults.length} lines`);
      
      // Filter out debug messages and keep only actual validation lines
      const cleanValidationResults = validationResults
        .filter(line => line.trim().length > 0)
        .filter(line => !line.startsWith('DEBUG:'))
        .filter(line => !line.includes('NewMultiChannelLogger'))
        .filter(line => !line.includes('Creating shared timestamp'))
        .filter(line => !line.includes('Verbose logger'))
        .filter(line => !line.includes('UI integration mode'))
        .filter(line => !line.includes('HTTP logger'))
        .filter(line => !line.includes('Progress tracker'))
        .filter(line => !line.startsWith('Validating Cilium feature prerequisites'))
        .filter(line => !line.includes('feature(s) failed validation'));
        
      console.log(`[CILIUM CONFIG API] Clean validation results: ${cleanValidationResults.length} lines`);

      if (cleanValidationResults.length > 0) {
        const summary = parseValidationResults(cleanValidationResults);
        responseData = {
          validationResults: cleanValidationResults,
          summary: summary
        };
        success = true;
        message = '✅ Feature validation completed';
      } else {
        success = false;
        message = '❌ No validation results found';
      }
    }

    res.write(`data: ${JSON.stringify({
      type: 'cilium_complete',
      success: success,
      message: message,
      data: responseData,
      exitCode: code,
      timestamp: new Date().toISOString()
    })}\n\n`);

    res.end();
    console.log(`[CILIUM CONFIG API] COMPLETED: ${operation} operation finished`);
  });

  // Handle process errors
  childProcess.on('error', (error) => {
    console.error(`[CILIUM CONFIG API] Process error:`, error);
    
    res.write(`data: ${JSON.stringify({
      type: 'cilium_error',
      error: error.message,
      message: `❌ Cilium ${operation} error: ${error.message}`,
      timestamp: new Date().toISOString()
    })}\n\n`);

    res.end();
  });

  // Handle client disconnect
  req.on('close', () => {
    console.log(`[CILIUM CONFIG API] Client disconnected`);
    
    // Kill the child process if client disconnects
    if (childProcess && !childProcess.killed) {
      childProcess.kill('SIGTERM');
    }
  });
}


// Generate insights and recommendations based on Cilium configuration
function generateConfigInsights(config) {
  // Use the clean feature service instead of hardcoded logic
  return featureService.generateInsights(config);
}

// Parse validation results using the clean feature service
function parseValidationResults(results) {
  return featureService.parseValidationResults(results);
}

// Helper function to check if a value is truthy (same logic as Go backend)
function isTruthy(val) {
  if (!val || val === '') return false;
  
  const normalized = val.toString().toLowerCase().trim();
  return ['true', '1', 'yes', 'y', 'on', 'enabled', 'enable', 'strict', 'partial', 'probe'].includes(normalized);
}
