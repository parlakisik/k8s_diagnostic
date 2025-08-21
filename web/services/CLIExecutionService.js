/**
 * Universal CLI Execution Service
 * Handles both local development (spawn) and production (HTTP API) modes
 * Provides unified interface for CLI communication across environments
 */
import { spawn } from 'child_process';
import path from 'path';
import { 
  getExecutionConfig, 
  getEnvironmentPaths, 
  getPollingConfig, 
  getRetryConfig,
  validateConfiguration,
  logConfiguration 
} from '../config/executionConfig.js';

class CLIExecutionService {
  constructor() {
    // Phase 3: Use unified configuration system
    this.config = getExecutionConfig();
    this.paths = getEnvironmentPaths();
    this.pollingConfig = getPollingConfig();
    this.retryConfig = getRetryConfig();
    
    // Validate configuration on startup
    const validation = validateConfiguration();
    if (!validation.valid) {
      console.warn('[CLIExecutionService] Configuration issues detected:', validation.issues);
    }
    
    // Legacy compatibility (maintained for backward compatibility)
    this.isKubernetesMode = this.config.environment.isKubernetes;
    this.cliEndpoint = this.config.cliEndpoint;
    this.eventStorageURL = this.config.eventStorageURL;
    
    // Phase 2: Event Loop Optimization - Smart polling and deduplication
    this.activePollers = new Map(); // testId -> poller instance
    this.eventDeduplication = new Set(); // prevent duplicate events
    this.maxPollers = this.pollingConfig.maxPollers;
    
    // Log configuration if debug mode is enabled
    if (this.config.enableDebugLogging) {
      logConfiguration();
    } else {
      console.log(`[CLIExecutionService] Initialized in ${this.config.mode} mode`);
      if (this.config.cliEndpoint) {
        console.log(`[CLIExecutionService] CLI endpoint: ${this.config.cliEndpoint}`);
      }
    }
  }

  /**
   * Universal batch test execution - works in both environments
   */
  async executeBatchTests(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] Starting batch execution: ${testId}, tests: ${testList.join(',')}`);
    
    if (this.isKubernetesMode) {
      return await this.executeViaHTTP(testId, testList, responseStream);
    } else {
      return await this.executeViaSpawn(testId, testList, responseStream);
    }
  }

  /**
   * Kubernetes/Production mode: Execute via HTTP API calls
   */
  async executeViaHTTP(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] HTTP mode: Executing ${testList.length} tests via CLI container`);
    
    // BALANCED: Minimal polling needed to fetch CLI-generated events
    console.log(`[CLIExecutionService] HTTP mode: Starting minimal polling to fetch CLI events`);
    const eventPoller = this.startMinimalPolling(testId, testList, responseStream);
    
    try {
      // Health check first
      const healthResponse = await fetch(`${this.cliEndpoint}/api/health`);
      if (!healthResponse.ok) {
        throw new Error(`CLI container health check failed: ${healthResponse.status}`);
      }
      console.log(`[CLIExecutionService] ✅ CLI container health check passed`);

      // FIXED: No cleanup polling needed in HTTP mode - CLI handles everything
      console.log(`[CLIExecutionService] HTTP mode: CLI container handles cleanup - no polling needed`);

      // Execute tests via HTTP
      let httpRequestsSuccessful = 0;
      for (let i = 0; i < testList.length; i++) {
        const testName = testList[i];
        
        responseStream.write(`data: ${JSON.stringify({
          type: 'test_start',
          testName: testName,
          message: `Starting test: ${testName}`,
          testIndex: i,
          totalTests: testList.length,
          timestamp: new Date().toISOString()
        })}\n\n`);
        
        try {
          const requestPayload = {
            testId: testName,
            batchTestId: testId,
            cliCommand: `./k8s-diagnostic test list: ${testName} --verbose`,
            args: ['--verbose'],
            environment: {
              BATCH_TEST_ID: testId,
              HTTP_LOG_URL: this.eventStorageURL
            }
          };

          const response = await fetch(`${this.cliEndpoint}/api/execute-test`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestPayload)
          });

          if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${await response.text()}`);
          }

          const result = await response.json();
          httpRequestsSuccessful++;
          
          responseStream.write(`data: ${JSON.stringify({
            type: 'test_complete',
            testName: testName,
            success: result.success,
            summary: result.success ? `PASSED - ${result.message}` : `FAILED - ${result.message}`,
            timestamp: new Date().toISOString()
          })}\n\n`);
          
        } catch (error) {
          console.error(`[CLIExecutionService] HTTP test ${testName} failed:`, error);
          responseStream.write(`data: ${JSON.stringify({
            type: 'test_complete',
            testName: testName,
            success: false,
            summary: `HTTP API Error: ${error.message}`,
            timestamp: new Date().toISOString()
          })}\n\n`);
        }
      }

      // Complete batch
      responseStream.write(`data: ${JSON.stringify({
        type: 'batch_complete',
        success: httpRequestsSuccessful > 0,
        exitCode: httpRequestsSuccessful > 0 ? 0 : 1,
        overallProgress: 100,
        message: `HTTP API execution completed: ${httpRequestsSuccessful}/${testList.length} successful`,
        timestamp: new Date().toISOString()
      })}\n\n`);

      // Stop minimal event polling
      this.stopEventPolling(eventPoller);
      return { success: httpRequestsSuccessful > 0, mode: 'http' };
      
    } catch (error) {
      console.error(`[CLIExecutionService] HTTP execution error:`, error);
      this.stopEventPolling(eventPoller);
      throw error;
    }
  }

  /**
   * Local Development mode: Execute via spawn process
   * FINAL CORRECT APPROACH: NO HTTP polling - events come from stdout only
   */
  async executeViaSpawn(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] Spawn mode: Executing ${testList.length} tests via spawned process`);
    console.log(`[CLIExecutionService] Dev environment: Events will come from process stdout - NO HTTP polling whatsoever`);
    
    try {
      const testListString = testList.join(',');
      
      let childProcess;
      if (this.config.useDocker) {
        // Docker Compose execution
        console.log(`[CLIExecutionService] Using Docker Compose execution`);
        const dockerArgs = [
          'compose', 'run', '--rm',
          'k8s-diagnostic-cli-standalone',
          'test', 'list:', testListString, '--verbose'
        ];
        
        childProcess = spawn('docker', dockerArgs, {
          cwd: this.paths.projectRoot,
          stdio: ['ignore', 'pipe', 'pipe'],
          detached: true,
          env: {
            ...process.env,
            BATCH_TEST_ID: testId,
            HTTP_LOG_URL: this.config.eventStorageURL
          }
        });
      } else {
        // Local binary execution
        console.log(`[CLIExecutionService] Using local binary execution`);
        const localArgs = ['test', 'list:', testListString, '--verbose'];
        
        childProcess = spawn(this.paths.binaryPath, localArgs, {
          cwd: this.paths.workingDirectory,
          stdio: ['ignore', 'pipe', 'pipe'],
          detached: true,
          env: {
            ...process.env,
            BATCH_TEST_ID: testId,
            HTTP_LOG_URL: this.config.eventStorageURL
          }
        });
      }

      console.log(`[CLIExecutionService] Process spawned with PID: ${childProcess.pid}`);

      // Handle stdout with direct event forwarding - no HTTP polling needed
      childProcess.stdout.on('data', (data) => {
        const output = data.toString();
        this.processSpawnOutput(output, responseStream, testId);
      });

      // Handle stderr
      childProcess.stderr.on('data', (data) => {
        const output = data.toString();
        if (this.config.enableDebugLogging) {
          console.log(`[CLIExecutionService] stderr:`, output.substring(0, 200));
        }
      });

      // Handle process completion
      const processResult = await new Promise((resolve) => {
        childProcess.on('close', (code) => {
          console.log(`[CLIExecutionService] Process completed with code: ${code}`);
          
          // Send final batch complete event
          responseStream.write(`data: ${JSON.stringify({
            type: 'batch_complete',
            success: code === 0,
            exitCode: code,
            overallProgress: 100,
            message: code === 0 ? 'All tests completed successfully' : 'Some tests failed',
            timestamp: new Date().toISOString()
          })}\n\n`);
          
          resolve({ success: code === 0, exitCode: code });
        });

        childProcess.on('error', (error) => {
          console.error(`[CLIExecutionService] Process error:`, error);
          
          // Send error event
          responseStream.write(`data: ${JSON.stringify({
            type: 'batch_error',
            error: error.message,
            testId: testId,
            timestamp: new Date().toISOString()
          })}\n\n`);
          
          resolve({ success: false, error: error.message });
        });
      });

      // Dev mode complete - no HTTP polling was used
      console.log(`[CLIExecutionService] Dev mode completed successfully - all events came from stdout`);
      return { ...processResult, mode: 'spawn' };
      
    } catch (error) {
      console.error(`[CLIExecutionService] Spawn execution error:`, error);
      throw error;
    }
  }

  /**
   * Process stdout output from spawned process and forward events
   * FIXED: Add flush() after every write to ensure SSE events reach frontend
   */
  processSpawnOutput(output, responseStream, testId) {
    const lines = output.split('\n');
    
    lines.forEach(line => {
      const trimmedLine = line.trim();
      if (!trimmedLine) return;

      console.log(`[CLIExecutionService] Stdout line: ${trimmedLine.substring(0, 100)}...`);

      // Check for structured SSE events from Go CLI
      if (trimmedLine.startsWith('SSE_EVENT:')) {
        try {
          const eventData = JSON.parse(trimmedLine.substring(10));
          console.log(`[CLIExecutionService] ✅ Forwarding SSE event to frontend: ${eventData.type}`);
          
          // Forward to frontend immediately with flush
          responseStream.write(`data: ${JSON.stringify(eventData)}\n\n`);
          responseStream.flush();
          
        } catch (parseError) {
          console.error(`[CLIExecutionService] Failed to parse SSE event:`, parseError);
        }
      } 
      // Check for test start patterns
      else if (trimmedLine.includes('Running test:')) {
        const testMatch = trimmedLine.match(/Running test:\s*(.+)/);
        if (testMatch) {
          const testName = testMatch[1].trim();
          const event = {
            type: 'test_start',
            testName: testName,
            message: `Starting test: ${testName}`,
            timestamp: new Date().toISOString()
          };
          responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
          responseStream.flush();
          console.log(`[CLIExecutionService] ✅ Forwarded test start to frontend: ${testName}`);
        }
      }
      // Check for test completion patterns
      else if (trimmedLine.includes('✅') && trimmedLine.includes('PASS')) {
        const testMatch = trimmedLine.match(/✅\s+([^:]+):\s*PASSED?\s*\(([^)]+)\)/);
        if (testMatch) {
          const testName = testMatch[1].trim();
          const duration = testMatch[2];
          const event = {
            type: 'test_complete',
            testName: testName,
            success: true,
            summary: `PASSED (${duration})`,
            duration: duration,
            timestamp: new Date().toISOString()
          };
          responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
          responseStream.flush();
          console.log(`[CLIExecutionService] ✅ Forwarded test success to frontend: ${testName}`);
        }
      }
      // Check for test failure patterns
      else if (trimmedLine.includes('❌') && trimmedLine.includes('FAIL')) {
        const testMatch = trimmedLine.match(/❌\s+([^:]+):\s*FAILED?\s*\(([^)]+)\)/);
        if (testMatch) {
          const testName = testMatch[1].trim();
          const duration = testMatch[2];
          const event = {
            type: 'test_complete',
            testName: testName,
            success: false,
            summary: `FAILED (${duration})`,
            duration: duration,
            timestamp: new Date().toISOString()
          };
          responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
          responseStream.flush();
          console.log(`[CLIExecutionService] ✅ Forwarded test failure to frontend: ${testName}`);
        }
      }
      // Check for cleanup phases - BE MORE SELECTIVE
      else if (trimmedLine.includes('🧹 Pre-test cleanup phase') || 
               trimmedLine.includes('🧹 CLEANUP PHASE') ||
               trimmedLine.includes('✅ Pre-test cleanup completed') ||
               trimmedLine.includes('✅ Inter-test cleanup completed') ||
               trimmedLine.includes('✅ universal_cleanup completed')) {
        const event = {
          type: 'live_output',
          output: line + '\n',
          phase: 'cleanup',
          timestamp: new Date().toISOString()
        };
        responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
        responseStream.flush();
        console.log(`[CLIExecutionService] ✅ Forwarded cleanup phase to frontend: ${trimmedLine.substring(0, 50)}...`);
      }
      // Default live output
      else {
        const event = {
          type: 'live_output',
          output: line + '\n',
          timestamp: new Date().toISOString()
        };
        responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
        responseStream.flush();
      }
    });
  }

  /**
   * FIXED: Minimal polling that fetches from ALL event sources
   */
  startMinimalPolling(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] Starting minimal polling for production: ${testId}, tests: ${testList.join(',')}`);
    
    let processedEventIds = new Set();
    const pollingActive = { value: true };
    let pollCount = 0;
    
    const pollFunction = async () => {
      if (!pollingActive.value) return;
      
      pollCount++;
      let foundEvents = false;
      
      try {
        // Poll batch testId first
        const batchResponse = await fetch(`${this.eventStorageURL}/api/log-events?testId=${testId}`, {
          method: 'GET',
          headers: { 'Content-Type': 'application/json' }
        });
        
        if (batchResponse.ok) {
          const batchData = await batchResponse.json();
          if (batchData.events && batchData.events.length > 0) {
            for (const event of batchData.events) {
              const eventId = `${event.timestamp}_${event.type}_${event.testName || event.testId}`;
              if (!processedEventIds.has(eventId)) {
                processedEventIds.add(eventId);
                responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
                responseStream.flush();
                foundEvents = true;
                console.log(`[CLIExecutionService] Forwarded batch event: ${event.type}`);
              }
            }
          }
        }
        
        // CRITICAL: Poll actual test names from testList parameter where CLI stores events
        console.log(`[CLIExecutionService] Polling individual test events for: ${testList.join(',')}`);
        
        for (const testName of testList) {
          try {
            const testResponse = await fetch(`${this.eventStorageURL}/api/log-events?testId=${testName}`, {
              method: 'GET',
              headers: { 'Content-Type': 'application/json' }
            });
            
            if (testResponse.ok) {
              const testData = await testResponse.json();
              if (testData.events && testData.events.length > 0) {
                for (const event of testData.events) {
                  const eventId = `${event.timestamp}_${event.type}_${event.testName || event.testId}`;
                  if (!processedEventIds.has(eventId)) {
                    processedEventIds.add(eventId);
                    
                    // Transform event to include batch context
                    const transformedEvent = {
                      ...event,
                      testName: testName,
                      batchTestId: testId
                    };
                    
                    responseStream.write(`data: ${JSON.stringify(transformedEvent)}\n\n`);
                    responseStream.flush();
                    foundEvents = true;
                    console.log(`[CLIExecutionService] Forwarded test event: ${event.type} for ${testName}`);
                  }
                }
              }
            }
          } catch (testError) {
            // Continue with other tests
          }
        }
        
        if (foundEvents) {
          console.log(`[CLIExecutionService] ✅ Successfully forwarded events to production UI`);
        }
        
        // Stop after reasonable time
        if (pollCount > 60) {
          console.log(`[CLIExecutionService] Stopping minimal polling after ${pollCount} polls`);
          pollingActive.value = false;
          return;
        }
        
        // Continue polling every 2 seconds
        if (pollingActive.value) {
          setTimeout(pollFunction, 2000);
        }
        
      } catch (error) {
        console.warn(`[CLIExecutionService] Minimal polling error: ${error.message}`);
        if (pollCount < 60) {
          setTimeout(pollFunction, 3000);
        } else {
          pollingActive.value = false;
        }
      }
    };
    
    // Start polling immediately
    pollFunction();
    
    return {
      active: pollingActive,
      testId: testId,
      cleanup: () => {
        processedEventIds.clear();
      }
    };
  }

  /**
   * HTTP event polling - ONLY for Kubernetes mode  
   * Phase 2: Enhanced with smart polling, exponential backoff, and deduplication
   */
  startUniversalEventPolling(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] Kubernetes mode: Starting enhanced HTTP event polling for testId: ${testId}`);
    
    // Prevent multiple pollers for same testId
    if (this.activePollers.has(testId)) {
      console.log(`[CLIExecutionService] Reusing existing poller for ${testId}`);
      return this.activePollers.get(testId);
    }
    
    // Prevent too many concurrent pollers
    if (this.activePollers.size >= this.maxPollers) {
      console.warn(`[CLIExecutionService] Max pollers (${this.maxPollers}) reached, cleaning up oldest`);
      const oldestTestId = this.activePollers.keys().next().value;
      this.stopEventPolling(this.activePollers.get(oldestTestId));
    }
    
    const poller = this.createSmartPoller(testId, testList, responseStream);
    this.activePollers.set(testId, poller);
    return poller;
  }

  /**
   * FINAL SOLUTION: Smart poller that stops after batch completion
   */
  createSmartPoller(testId, testList, responseStream) {
    console.log(`[CLIExecutionService] Creating smart poller for ${testId}`);
    
    let totalEventsSeen = 0;
    let pollInterval = this.pollingConfig.initialInterval;
    let consecutiveEmptyPolls = 0;
    const pollingActive = { value: true };
    let eventHashes = new Set();
    
    const pollFunction = async () => {
      if (!pollingActive.value) return;
      
      try {
        const events = await this.fetchNewEvents(testId, testList, totalEventsSeen);
        
        if (events.length === 0) {
          consecutiveEmptyPolls++;
          
          // AGGRESSIVE backoff - start backing off immediately
          if (consecutiveEmptyPolls >= 2) { // Start backoff after just 2 empty polls
            pollInterval = Math.min(
              this.pollingConfig.maxInterval, 
              1000 * Math.pow(2, consecutiveEmptyPolls - 1) // 2s, 4s, 8s
            );
            
            if (consecutiveEmptyPolls % 5 === 0) { // Log every 5th empty poll
              console.log(`[CLIExecutionService] ${consecutiveEmptyPolls} empty polls, interval now ${pollInterval}ms`);
            }
          }
          
          // Stop polling after 15 empty polls (prevents infinite loops)
          if (consecutiveEmptyPolls >= 15) {
            console.log(`[CLIExecutionService] Stopping after ${consecutiveEmptyPolls} empty polls`);
            pollingActive.value = false;
            return;
          }
          
        } else {
          consecutiveEmptyPolls = 0;
          pollInterval = this.pollingConfig.initialInterval;
          
          // Process events with deduplication
          const newEvents = this.processEventsWithDeduplication(events, eventHashes);
          
          for (const event of newEvents) {
            responseStream.write(`data: ${JSON.stringify(event)}\n\n`);
            responseStream.flush();
          }
          
          totalEventsSeen = Math.max(totalEventsSeen, events.length);
          
          if (newEvents.length > 0) {
            console.log(`[CLIExecutionService] Forwarded ${newEvents.length} new events for ${testId}`);
          }
        }
        
        // Schedule next poll
        if (pollingActive.value) {
          setTimeout(pollFunction, pollInterval);
        }
        
      } catch (error) {
        console.warn(`[CLIExecutionService] Polling error: ${error.message}`);
        consecutiveEmptyPolls += 2; // Count errors as double empty polls
        pollInterval = Math.min(this.pollingConfig.maxInterval * 2, pollInterval * 2);
        
        if (pollingActive.value && consecutiveEmptyPolls < 15) {
          setTimeout(pollFunction, pollInterval);
        } else {
          console.log(`[CLIExecutionService] Too many errors, stopping polling for ${testId}`);
          pollingActive.value = false;
        }
      }
    };
    
    // Start first poll immediately
    pollFunction();
    
    return { 
      active: pollingActive, 
      testId: testId,
      cleanup: () => {
        eventHashes.clear();
        this.activePollers.delete(testId);
      }
    };
  }

  /**
   * FIXED: Fetch new events with robust JSON parsing for both batch and individual events
   */
  async fetchNewEvents(testId, testList, lastEventCount) {
    const allEvents = [];
    
    try {
      // Primary: Poll with batch testId
      const eventResponse = await fetch(`${this.eventStorageURL}/api/log-events?testId=${testId}`, {
        method: 'GET',
        headers: { 
          'Content-Type': 'application/json',
          'Cache-Control': 'no-cache'
        }
      });
      
      if (eventResponse.ok) {
        try {
          const eventData = await eventResponse.json();
          
          if (eventData.events && eventData.events.length > lastEventCount) {
            allEvents.push(...eventData.events.slice(lastEventCount));
          }
        } catch (jsonError) {
          console.warn(`[CLIExecutionService] JSON parsing failed for batch ${testId}: ${jsonError.message}`);
        }
      }
      
      // Fallback: Poll individual test names for additional events
      for (const testName of testList) {
        try {
          const individualResponse = await fetch(`${this.eventStorageURL}/api/log-events?testId=${testName}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
            signal: AbortSignal.timeout(1000)
          });
          
          if (individualResponse.ok) {
            try {
              const individualData = await individualResponse.json();
              
              if (individualData.events && individualData.events.length > 0) {
                // Transform events to include batch context
                for (const event of individualData.events) {
                  const transformedEvent = {
                    ...event,
                    testName: testName,
                    batchTestId: testId
                  };
                  allEvents.push(transformedEvent);
                }
              }
            } catch (jsonError) {
              console.warn(`[CLIExecutionService] JSON parsing failed for test ${testName}: ${jsonError.message}`);
            }
          }
        } catch (individualError) {
          // Individual poll failures are non-critical
          if (this.config.enableVerbosePolling) {
            console.log(`[CLIExecutionService] Individual poll failed for ${testName}: ${individualError.message}`);
          }
        }
      }
      
    } catch (error) {
      console.warn(`[CLIExecutionService] Batch event fetch failed: ${error.message}`);
    }
    
    if (allEvents.length > 0) {
      console.log(`[CLIExecutionService] Fetched ${allEvents.length} new events for batch ${testId}`);
    }
    
    return allEvents;
  }

  /**
   * Process events with advanced deduplication
   * Phase 3: Uses configuration for deduplication limits
   */
  processEventsWithDeduplication(events, eventHashes) {
    const newEvents = [];
    
    for (const event of events) {
      // Create hash for deduplication
      const eventHash = this.createEventHash(event);
      
      // Check both local and global deduplication
      if (!eventHashes.has(eventHash) && !this.eventDeduplication.has(eventHash)) {
        eventHashes.add(eventHash);
        this.eventDeduplication.add(eventHash);
        newEvents.push(event);
      }
    }
    
    // Clean up global deduplication set periodically using configuration
    if (this.eventDeduplication.size > this.config.maxEventDeduplication) {
      const deduplicationArray = Array.from(this.eventDeduplication);
      const keepCount = Math.floor(this.config.maxEventDeduplication / 2);
      this.eventDeduplication = new Set(deduplicationArray.slice(-keepCount));
      
      if (this.config.enableVerbosePolling) {
        console.log(`[CLIExecutionService] Cleaned up event deduplication cache: ${deduplicationArray.length} -> ${keepCount}`);
      }
    }
    
    return newEvents;
  }

  /**
   * Create hash for event deduplication
   * Phase 3: Uses configuration for hash length
   */
  createEventHash(event) {
    // Create unique hash based on key event properties
    const key = `${event.type}_${event.testId || event.testName}_${event.timestamp}_${event.message || ''}`;
    return btoa(key).substring(0, this.config.eventHashLength);
  }

  /**
   * Stop event polling with enhanced cleanup
   */
  stopEventPolling(eventPoller) {
    // CRITICAL FIX: Handle null poller (when no polling was started)
    if (!eventPoller) {
      console.log(`[CLIExecutionService] No event poller to stop`);
      return;
    }
    
    if (eventPoller.active) {
      eventPoller.active.value = false;
    }
    
    // Enhanced cleanup for smart pollers
    if (eventPoller.cleanup) {
      eventPoller.cleanup();
    }
    
    // Legacy support for interval-based pollers
    if (eventPoller.interval) {
      clearInterval(eventPoller.interval);
    }
    
    console.log(`[CLIExecutionService] Event polling stopped for ${eventPoller.testId || 'unknown'}`);
  }

  /**
   * Wait for cleanup completion using batch-centric approach
   */
  async waitForCleanupCompletion(testId, testList, timeoutMs = 60000) {
    console.log(`[CLIExecutionService] Waiting for cleanup completion: ${testId}`);
    
    const startTime = Date.now();
    
    return new Promise((resolve) => {
      const checkCleanup = setInterval(async () => {
        const elapsed = Date.now() - startTime;
        
        if (elapsed >= timeoutMs) {
          console.warn(`[CLIExecutionService] Cleanup timeout after ${elapsed}ms`);
          clearInterval(checkCleanup);
          resolve(false);
          return;
        }
        
        try {
          // Check for cleanup completion events
          const response = await fetch(`${this.eventStorageURL}/api/log-events?testId=${testId}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
          });
          
          if (response.ok) {
            const data = await response.json();
            
            if (data.events && Array.isArray(data.events)) {
              const cleanupCompleteEvents = data.events.filter(event => 
                event.type === 'cleanup_complete' || 
                (event.type === 'step_complete' && event.step && event.step.includes('cleanup'))
              );
              
              if (cleanupCompleteEvents.length > 0) {
                console.log(`[CLIExecutionService] ✅ Cleanup completion detected after ${elapsed}ms`);
                clearInterval(checkCleanup);
                resolve(true);
                return;
              }
            }
          }
          
        } catch (error) {
          console.log(`[CLIExecutionService] Cleanup polling error: ${error.message}`);
        }
      }, 1000);
    });
  }

  /**
   * Universal termination - works for both modes
   */
  async terminateExecution(testId) {
    console.log(`[CLIExecutionService] Terminating execution: ${testId}`);
    
    if (this.isKubernetesMode) {
      // Kubernetes mode: Send termination request to CLI container
      try {
        await fetch(`${this.cliEndpoint}/api/terminate-test`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ testId: testId })
        });
      } catch (error) {
        console.warn(`[CLIExecutionService] Kubernetes termination error: ${error.message}`);
      }
    } else {
      // Local mode: Process termination handled by run-batch-tests.js
      console.log(`[CLIExecutionService] Local mode termination delegated to process manager`);
    }
  }

  /**
   * Get execution configuration for debugging
   * Phase 3: Returns comprehensive unified configuration
   */
  getConfig() {
    return {
      // Legacy compatibility
      mode: this.config.mode,
      cliEndpoint: this.config.cliEndpoint,
      eventStorageURL: this.config.eventStorageURL,
      kubernetesMode: this.config.environment.isKubernetes,
      
      // Full configuration
      config: this.config,
      paths: this.paths,
      pollingConfig: this.pollingConfig,
      retryConfig: this.retryConfig,
      
      // Runtime state
      activePollers: this.activePollers.size,
      eventDeduplicationSize: this.eventDeduplication.size
    };
  }
}

// Export singleton instance
const cliExecutionService = new CLIExecutionService();
export default cliExecutionService;

// Export class for testing
export { CLIExecutionService };
