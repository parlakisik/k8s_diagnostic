import { exec } from 'child_process';
import { 
  terminateTestProcess, 
  getRunningTests, 
  getActiveTestProcesses, 
  getTestStateSync,
  getProcessState 
} from './run-batch-tests.js';

export default async function handler(req, res) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { testId, testName } = req.body;

  if (!testId) {
    return res.status(400).json({ error: 'testId is required' });
  }

  console.log(`[STOP API] 🛑 Received stop request - testId: ${testId}, testName: ${testName || 'ALL'}`);

  try {
    // 🛡️ ENHANCED: Use the integrated state management system
    const beforeState = getProcessState(testId);
    console.log(`[STOP API] 📊 Before termination state:`, {
      hasRunningTest: !!beforeState.runningTest,
      activeProcessesCount: beforeState.activeProcesses?.length || 0,
      testStatesCount: Object.keys(beforeState.testStates?.testStates || {}).length
    });

    // Use the enhanced termination function from run-batch-tests.js
    const terminated = await terminateTestProcess(testId, testName);
    
    if (terminated) {
      // Also perform system-level cleanup as fallback
      const systemKilled = await performSystemCleanup(testId);
      
      const afterState = getProcessState(testId);
      
      console.log(`[STOP API] ✅ Termination successful - testId: ${testId}`);
      console.log(`[STOP API] 📊 After termination state:`, {
        hasRunningTest: !!afterState.runningTest,
        activeProcessesCount: afterState.activeProcesses?.length || 0,
        systemProcessesKilled: systemKilled
      });
      
      res.status(200).json({ 
        success: true, 
        message: testName 
          ? `Successfully terminated test: ${testName}` 
          : `Successfully terminated all tests in batch: ${testId}`,
        testId: testId,
        testName: testName,
        stateManaged: true,
        systemProcessesKilled: systemKilled,
        beforeState: {
          running: !!beforeState.runningTest,
          activeTests: beforeState.activeProcesses?.length || 0
        },
        afterState: {
          running: !!afterState.runningTest,
          activeTests: afterState.activeProcesses?.length || 0
        }
      });
    } else {
      // If state management termination failed, try system cleanup
      const systemKilled = await performSystemCleanup(testId);
      
      console.log(`[STOP API] ⚠️ State management termination failed, system cleanup killed: ${systemKilled}`);
      
      if (systemKilled > 0) {
        res.status(200).json({ 
          success: true, 
          message: `Terminated ${systemKilled} system processes (state management failed)`,
          testId: testId,
          stateManaged: false,
          systemProcessesKilled: systemKilled,
          warning: 'Used fallback system termination - state may be inconsistent'
        });
      } else {
        res.status(200).json({ 
          success: true, 
          message: 'No running processes found to terminate',
          testId: testId,
          stateManaged: false,
          systemProcessesKilled: 0
        });
      }
    }
  } catch (error) {
    console.error(`[STOP API] ❌ Error stopping processes for testId ${testId}:`, error);
    
    // Emergency system cleanup on error
    try {
      const emergencyKilled = await performSystemCleanup(testId);
      res.status(500).json({ 
        error: 'Termination error, performed emergency cleanup',
        details: error.message,
        testId: testId,
        emergencyCleanup: emergencyKilled
      });
    } catch (cleanupError) {
      res.status(500).json({ 
        error: 'Failed to stop test processes', 
        details: error.message,
        cleanupError: cleanupError.message,
        testId: testId
      });
    }
  }
}

// 🛡️ ENHANCED: System-level cleanup as fallback (when state management fails)
async function performSystemCleanup(testId) {
  let killedCount = 0;

  try {
    console.log(`[STOP API] 🧹 Performing system-level cleanup for testId: ${testId}`);

    // Method 1: Kill process groups by pattern (most reliable)
    const killByPattern = () => {
      return new Promise((resolve) => {
        // 🛡️ ENHANCED: Kill entire process groups, not just parent processes
        exec(`pkill -f -g "k8s_diagnostic.*test"`, (error, stdout, stderr) => {
          if (error) {
            console.log(`[STOP API] pkill process groups k8s_diagnostic processes: No processes found`);
          } else {
            console.log(`[STOP API] 🔪 Killed k8s_diagnostic test process groups`);
            killedCount++;
          }

          // Also try to kill process groups with our binary name
          exec(`pkill -f -g "./k8s_diagnostic"`, (error2, stdout2, stderr2) => {
            if (error2) {
              console.log(`[STOP API] pkill process groups ./k8s_diagnostic: No processes found`);
            } else {
              console.log(`[STOP API] 🔪 Killed ./k8s_diagnostic binary process groups`);
              killedCount++;
            }
            
            // Fallback: Also try individual process killing
            exec(`pkill -f "k8s_diagnostic"`, (error3, stdout3, stderr3) => {
              if (error3) {
                console.log(`[STOP API] pkill individual k8s_diagnostic: No processes found`);
              } else {
                console.log(`[STOP API] 🔪 Killed individual k8s_diagnostic processes (fallback)`);
                killedCount++;
              }
              resolve();
            });
          });
        });
      });
    };

    // Method 2: Kill processes by exact name
    const killByName = () => {
      return new Promise((resolve) => {
        exec(`pkill -9 k8s_diagnostic`, (error, stdout, stderr) => {
          if (error) {
            console.log(`[STOP API] pkill by name: No processes found`);
          } else {
            console.log(`[STOP API] 🔪 Force killed processes by name`);
            killedCount++;
          }
          resolve();
        });
      });
    };

    // Method 3: Kill any kubectl processes that might be hanging (optional)
    const killKubectl = () => {
      return new Promise((resolve) => {
        // Only kill kubectl processes that seem related to our tests
        exec(`pkill -f "kubectl.*diagnostic-test"`, (error, stdout, stderr) => {
          if (error) {
            console.log(`[STOP API] pkill kubectl diagnostic-test: No processes found`);
          } else {
            console.log(`[STOP API] 🔪 Killed kubectl diagnostic-test processes`);
            killedCount++;
          }
          resolve();
        });
      });
    };

    // Method 4: Get process information for debugging
    const logProcessInfo = () => {
      return new Promise((resolve) => {
        exec(`pgrep -f k8s_diagnostic`, (error, stdout, stderr) => {
          if (!error && stdout) {
            console.log(`[STOP API] 📊 Found k8s_diagnostic PIDs:`, stdout.trim().split('\n'));
          }
          resolve();
        });
      });
    };

    // Execute cleanup methods sequentially
    await logProcessInfo();
    await killByPattern();
    await killByName();
    await killKubectl();

    console.log(`[STOP API] 🏁 System cleanup completed - killed ${killedCount} process groups`);
    return killedCount;
    
  } catch (error) {
    console.error(`[STOP API] ❌ Error in system cleanup:`, error);
    throw error;
  }
}

// 🛡️ NEW: Health check function to validate state consistency
export async function validateStopState(testId) {
  try {
    const processState = getProcessState(testId);
    const runningTests = getRunningTests();
    const activeProcesses = getActiveTestProcesses();
    
    return {
      testId: testId,
      isRunning: runningTests.has(testId),
      hasActiveProcesses: processState.activeProcesses && processState.activeProcesses.length > 0,
      stateConsistent: !runningTests.has(testId) || processState.activeProcesses?.length > 0,
      processState: processState
    };
  } catch (error) {
    console.error(`[STOP API] ❌ Error validating stop state:`, error);
    return {
      testId: testId,
      error: error.message,
      stateConsistent: false
    };
  }
}

// 🛡️ NEW: Emergency cleanup function for critical situations
export async function emergencyCleanup() {
  try {
    console.log(`[STOP API] 🚨 EMERGENCY CLEANUP - Terminating all test processes`);
    
    const runningTests = getRunningTests();
    const cleanupResults = [];
    
    // Terminate all tracked running tests
    for (const [testId] of runningTests) {
      try {
        const result = await terminateTestProcess(testId);
        cleanupResults.push({ testId, success: result });
      } catch (error) {
        cleanupResults.push({ testId, success: false, error: error.message });
      }
    }
    
    // Also perform system-level cleanup
    const systemKilled = await performSystemCleanup('emergency');
    
    return {
      success: true,
      terminatedTests: cleanupResults,
      systemProcessesKilled: systemKilled,
      message: `Emergency cleanup completed - ${cleanupResults.length} test batches, ${systemKilled} system processes`
    };
  } catch (error) {
    console.error(`[STOP API] ❌ Emergency cleanup failed:`, error);
    throw error;
  }
}
