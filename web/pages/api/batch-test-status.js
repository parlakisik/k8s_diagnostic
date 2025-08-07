// Simplified batch test status API - no JSONL file dependency
// Uses in-memory tracking for direct streaming compatibility

// This would ideally be replaced with a shared state management system
// For now, return basic status based on active connections

export default function handler(req, res) {
  if (req.method !== 'GET') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { testId } = req.query;
  
  if (!testId) {
    return res.status(400).json({ error: 'Test ID is required' });
  }

  console.log(`[BATCH STATUS API] Status check for testId: ${testId} (direct streaming mode)`);
  
  // In direct streaming mode, status is managed by the SSE connection
  // This endpoint now serves as a simple health check
  res.json({
    testId,
    mode: 'direct_streaming',
    message: 'Status is managed via Server-Sent Events - check SSE stream for real-time updates',
    allComplete: false,
    success: false,
    overallProgress: 0,
    streamingActive: true,
    testResults: {}
  });
}
