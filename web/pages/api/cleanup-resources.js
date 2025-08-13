import { spawn } from 'child_process';
import path from 'path';

export default function handler(req, res) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { operation } = req.body;

  if (operation !== 'deepclean') {
    return res.status(400).json({ error: 'Invalid operation. Only "deepclean" is supported.' });
  }

  console.log(`[CLEANUP API] Resource cleanup request received - Operation: ${operation}`);

  // Set response headers for Server-Sent Events
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.setHeader('Access-Control-Allow-Origin', '*');

  // Send initial connection event
  res.write(`data: ${JSON.stringify({
    type: 'cleanup_start',
    message: '🧹 Starting resource cleanup...',
    timestamp: new Date().toISOString()
  })}\n\n`);

  console.log(`[CLEANUP API] STARTING: Resource cleanup initiated`);

  // Get project root directory (one level up from web/)
  const projectRoot = path.resolve(process.cwd(), '..');
  
  // Spawn the cleanup process using deepclean command
  const args = ['deepclean'];
  const childProcess = spawn('./k8s_diagnostic', args, {
    cwd: projectRoot,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env }
  });

  // Handle stdout
  childProcess.stdout.on('data', (data) => {
    const output = data.toString();
    console.log(`[CLEANUP API] stdout:`, output);

    // Parse output and send structured events
    const lines = output.split('\n');
    lines.forEach(line => {
      line = line.trim();
      if (!line) return;

      // Send cleanup progress event
      res.write(`data: ${JSON.stringify({
        type: 'cleanup_output',
        output: line,
        timestamp: new Date().toISOString()
      })}\n\n`);

      // Check for specific cleanup messages
      if (line.includes('Deleting') || line.includes('Removing') || line.includes('Cleaning')) {
        res.write(`data: ${JSON.stringify({
          type: 'cleanup_progress',
          message: `🧹 ${line}`,
          timestamp: new Date().toISOString()
        })}\n\n`);
      }
    });
  });

  // Handle stderr
  childProcess.stderr.on('data', (data) => {
    const output = data.toString();
    console.log(`[CLEANUP API] stderr:`, output);
    
    // Parse stderr and send as warnings or errors
    const lines = output.split('\n');
    lines.forEach(line => {
      line = line.trim();
      if (!line) return;

      res.write(`data: ${JSON.stringify({
        type: 'cleanup_output',
        output: `⚠️ ${line}`,
        timestamp: new Date().toISOString()
      })}\n\n`);
    });
  });

  // Handle process completion
  childProcess.on('close', (code) => {
    console.log(`[CLEANUP API] Process finished with code: ${code}`);

    if (code === 0) {
      res.write(`data: ${JSON.stringify({
        type: 'cleanup_complete',
        success: true,
        message: '✅ Resource cleanup completed successfully',
        exitCode: code,
        timestamp: new Date().toISOString()
      })}\n\n`);
    } else {
      res.write(`data: ${JSON.stringify({
        type: 'cleanup_complete',
        success: false,
        message: '❌ Resource cleanup completed with errors',
        exitCode: code,
        timestamp: new Date().toISOString()
      })}\n\n`);
    }

    res.end();
    console.log(`[CLEANUP API] COMPLETED: Resource cleanup finished`);
  });

  // Handle process errors
  childProcess.on('error', (error) => {
    console.error(`[CLEANUP API] Process error:`, error);
    
    res.write(`data: ${JSON.stringify({
      type: 'cleanup_error',
      error: error.message,
      message: `❌ Cleanup process error: ${error.message}`,
      timestamp: new Date().toISOString()
    })}\n\n`);

    res.end();
  });

  // Handle client disconnect
  req.on('close', () => {
    console.log(`[CLEANUP API] Client disconnected`);
    
    // Kill the child process if client disconnects
    if (childProcess && !childProcess.killed) {
      childProcess.kill('SIGTERM');
    }
  });
}
