import cliExecutionService from '../../services/CLIExecutionService.js';

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

  // Set SSE headers
  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');
  res.setHeader('Access-Control-Allow-Origin', '*');

  // Send initial connection event
  res.write(`data: ${JSON.stringify({
    type: 'connected',
    message: `Connected to batch test stream for ${testList.length} tests`,
    testId: testId,
    testList: testList
  })}\n\n`);
  res.flush();

  try {
    console.log(`[BATCH API] Using unified CLIExecutionService for ${testList.length} tests`);
    
    // Use unified service - no more duplicate logic!
    const result = await cliExecutionService.executeBatchTests(testId, testList, res);
    
    console.log(`[BATCH API] CLIExecutionService completed:`, {
      success: result.success,
      mode: result.mode,
      testId: testId
    });
    
    res.write(`data: ${JSON.stringify({
      type: 'batch_complete',
      success: result.success,
      mode: result.mode, // 'http' or 'spawn'
      message: `Batch execution completed via ${result.mode} mode`,
      timestamp: new Date().toISOString()
    })}\n\n`);
    
  } catch (error) {
    console.error(`[BATCH API] CLIExecutionService error:`, error);
    
    res.write(`data: ${JSON.stringify({
      type: 'batch_error',
      error: error.message,
      testId: testId,
      timestamp: new Date().toISOString()
    })}\n\n`);
  }
  
  res.end();
}
