// In-memory event storage for real-time streaming
const eventStorage = new Map(); // testId -> EventContainer

class EventContainer {
  constructor(testId) {
    this.testId = testId;
    this.events = [];
    this.eventsByContainer = new Map(); // container -> events array
    this.currentPhase = null;
    this.currentContainer = null;
    this.currentStep = null;
    this.startTime = Date.now();
  }

  addEvent(event) {
    // Add to main events list
    this.events.push(event);
    
    // Update current state
    if (event.phase) this.currentPhase = event.phase;
    if (event.container) this.currentContainer = event.container;
    if (event.step) this.currentStep = event.step;
    
    // Group by container for test card mapping
    const container = event.container || 'unknown';
    if (!this.eventsByContainer.has(container)) {
      this.eventsByContainer.set(container, []);
    }
    this.eventsByContainer.get(container).push(event);
    
    // Keep only last 1000 events per container to prevent memory leaks
    const containerEvents = this.eventsByContainer.get(container);
    if (containerEvents.length > 1000) {
      containerEvents.splice(0, 100); // Remove oldest 100 events
    }
  }

  getEvents(container = null) {
    if (container) {
      return this.eventsByContainer.get(container) || [];
    }
    return this.events;
  }

  getEventsByContainer() {
    const result = {};
    for (const [container, events] of this.eventsByContainer.entries()) {
      result[container] = events;
    }
    return result;
  }
}

// Cleanup old event containers (remove after 1 hour)
setInterval(() => {
  const oneHourAgo = Date.now() - (60 * 60 * 1000);
  for (const [testId, container] of eventStorage.entries()) {
    if (container.startTime < oneHourAgo) {
      eventStorage.delete(testId);
      console.log(`[LOG-EVENTS] Cleaned up old events for testId: ${testId}`);
    }
  }
}, 10 * 60 * 1000); // Check every 10 minutes

export default async function handler(req, res) {
  const { method } = req;

  if (method === 'POST') {
    // Receive structured events from CLI
    try {
      const event = req.body;
      console.log('[log-events.js] POST - Raw event received:', event);
      
      if (!event.testId) {
        return res.status(400).json({ error: 'testId is required' });
      }

      // 🧹 ENHANCED: More aggressive cleanup for various event types
      if (event.type === 'batch_start' || 
          event.type === 'test_start' || 
          event.type === 'step_complete' && event.step === 'universal_cleanup') {
        const oldSize = eventStorage.size;
        eventStorage.clear();
        console.log(`[log-events.js] 🧹 CLEANUP: Cleared ${oldSize} old event containers (trigger: ${event.type})`);
      }

      // 🧹 ADDITIONAL: Clear events for specific testId if it already exists and we get a new test_start
      if (event.type === 'test_start' && eventStorage.has(event.testId)) {
        const container = eventStorage.get(event.testId);
        const oldEventCount = container.events.length;
        eventStorage.delete(event.testId);
        console.log(`[log-events.js] 🧹 TESTID CLEANUP: Cleared ${oldEventCount} events for existing testId: ${event.testId}`);
      }

      // Get or create event container
      if (!eventStorage.has(event.testId)) {
        eventStorage.set(event.testId, new EventContainer(event.testId));
        console.log(`[log-events.js] Created new container for testId: ${event.testId}`);
      }
      
      const container = eventStorage.get(event.testId);
      
      // Add timestamp if not provided
      if (!event.timestamp) {
        event.timestamp = new Date().toISOString();
      }
      
      // Add event to container
      container.addEvent(event);
      console.log(`[log-events.js] Event stored. Total events: ${container.events.length}`);
      
      return res.status(200).json({ success: true, eventCount: container.events.length });
      
    } catch (error) {
      console.error('[LOG-EVENTS] Error processing event:', error);
      return res.status(500).json({ error: 'Failed to process event' });
    }
    
  } else if (method === 'GET') {
    // Retrieve events for batch API
    const { testId, container, since } = req.query;
    console.log(`[log-events.js] GET - Request for testId: ${testId}, container: ${container}, since: ${since}`);
    
    if (!testId) {
      return res.status(400).json({ error: 'testId query parameter is required' });
    }

    const eventContainer = eventStorage.get(testId);
    if (!eventContainer) {
      console.log(`[log-events.js] GET - No container found for testId: ${testId}`);
      return res.status(404).json({ error: 'No events found for this testId' });
    }

    try {
      let events;
      
      if (container) {
        // Get events for specific container (test)
        events = eventContainer.getEvents(container);
      } else {
        // Get all events
        events = eventContainer.getEvents();
      }
      
      // Filter by timestamp if 'since' parameter provided
      if (since) {
        const sinceTime = new Date(since);
        events = events.filter(event => new Date(event.timestamp) > sinceTime);
      }
      
      console.log(`[log-events.js] GET - Returning ${events.length} events for testId: ${testId}`);
      if (events.length > 0) {
        console.log(`[log-events.js] GET - Sample events:`, events.slice(0, 3));
      }
      
      return res.status(200).json({
        testId: testId,
        events: events,
        eventsByContainer: eventContainer.getEventsByContainer(),
        currentState: {
          phase: eventContainer.currentPhase,
          container: eventContainer.currentContainer,
          step: eventContainer.currentStep
        },
        totalEvents: eventContainer.events.length
      });
      
    } catch (error) {
      console.error('[LOG-EVENTS] Error retrieving events:', error);
      return res.status(500).json({ error: 'Failed to retrieve events' });
    }
    
  } else if (method === 'DELETE') {
    // Cleanup events - if no testId provided, clear ALL events (frontend batch cleanup)
    const { testId } = req.query;
    
    if (testId && eventStorage.has(testId)) {
      eventStorage.delete(testId);
      return res.status(200).json({ success: true, message: `Events cleaned up for testId: ${testId}` });
    } else if (!testId) {
      // 🧹 CRITICAL: Clear ALL events when no testId provided (frontend batch cleanup)
      const clearedCount = eventStorage.size;
      eventStorage.clear();
      console.log(`[log-events.js] 🧹 FRONTEND CLEANUP: Cleared ALL ${clearedCount} event containers`);
      return res.status(200).json({ success: true, message: `All event containers cleared (${clearedCount})` });
    }
    
    return res.status(404).json({ error: 'TestId not found' });
    
  } else {
    res.setHeader('Allow', ['POST', 'GET', 'DELETE']);
    return res.status(405).json({ error: 'Method not allowed' });
  }
}
