// Test result validation utilities for detecting false positives

/**
 * Validates if a test result is authentic or potentially fake
 * @param {Object} result - Test result object
 * @param {string} testName - Name of the test
 * @returns {Object} Validation result with warnings and authenticity flag
 */
export function validateTestResult(result, testName) {
  const validation = {
    isValid: true,
    warnings: [],
    severity: 'none', // none, low, medium, high, critical
    recommendations: []
  };

  if (!result) {
    validation.isValid = false;
    validation.severity = 'critical';
    validation.warnings.push('No test result provided');
    return validation;
  }

  // Check for generic/template messages that indicate fake results
  if (result.summary && typeof result.summary === 'string') {
    const suspiciousPatterns = [
      'PASSED - executed via HTTP API',
      'Test completed via HTTP API',
      'executed via HTTP API',
      'HTTP API execution',
      'generic success message'
    ];

    const foundSuspicious = suspiciousPatterns.find(pattern => 
      result.summary.toLowerCase().includes(pattern.toLowerCase())
    );

    if (foundSuspicious) {
      validation.isValid = false;
      validation.severity = 'high';
      validation.warnings.push(`Generic success message detected: "${foundSuspicious}"`);
      validation.recommendations.push('This may be a fake result - check CLI container logs for actual HTTP requests');
    }
  }

  // Check for suspiciously short success messages
  if (result.success && result.summary && result.summary.length < 15) {
    validation.isValid = false;
    validation.severity = 'medium';
    validation.warnings.push(`Success message is suspiciously short (${result.summary.length} chars): "${result.summary}"`);
    validation.recommendations.push('Real test results typically have detailed success messages');
  }

  // Check for missing duration on successful tests
  if (result.success && (!result.duration || result.duration === null)) {
    validation.severity = validation.severity === 'none' ? 'low' : validation.severity;
    validation.warnings.push('Successful test has no duration - may indicate fake execution');
    validation.recommendations.push('Real tests should report execution time');
  }

  // Check for generic command patterns
  if (result.command && result.command.includes('HTTP API:')) {
    validation.severity = validation.severity === 'none' ? 'medium' : validation.severity;
    validation.warnings.push('Command shows HTTP API placeholder instead of actual CLI command');
    validation.recommendations.push('Real results should show the actual diagnostic command executed');
  }

  // Check for validation warnings from the response
  if (result.validationWarnings && Array.isArray(result.validationWarnings) && result.validationWarnings.length > 0) {
    validation.isValid = false;
    validation.severity = 'high';
    validation.warnings.push(...result.validationWarnings);
    validation.recommendations.push('Server-side validation detected issues with this result');
  }

  return validation;
}

/**
 * Analyzes a batch of test results for overall authenticity
 * @param {Object} results - Object with test results keyed by test name
 * @returns {Object} Batch validation summary
 */
export function validateBatchResults(results) {
  const batchValidation = {
    totalTests: 0,
    validResults: 0,
    suspiciousResults: 0,
    criticalIssues: 0,
    overallTrustScore: 100,
    recommendations: [],
    detailedResults: {}
  };

  if (!results || typeof results !== 'object') {
    batchValidation.overallTrustScore = 0;
    batchValidation.recommendations.push('No test results provided for validation');
    return batchValidation;
  }

  const testNames = Object.keys(results);
  batchValidation.totalTests = testNames.length;

  testNames.forEach(testName => {
    const result = results[testName];
    const validation = validateTestResult(result, testName);
    
    batchValidation.detailedResults[testName] = validation;

    if (validation.isValid) {
      batchValidation.validResults++;
    } else {
      batchValidation.suspiciousResults++;
      
      if (validation.severity === 'critical' || validation.severity === 'high') {
        batchValidation.criticalIssues++;
      }
    }
  });

  // Calculate trust score
  if (batchValidation.totalTests > 0) {
    const validPercentage = (batchValidation.validResults / batchValidation.totalTests) * 100;
    const criticalPenalty = batchValidation.criticalIssues * 30;
    const suspiciousPenalty = batchValidation.suspiciousResults * 15;
    
    batchValidation.overallTrustScore = Math.max(0, validPercentage - criticalPenalty - suspiciousPenalty);
    batchValidation.overallTrustScore = Math.round(batchValidation.overallTrustScore);
  }

  // Generate recommendations
  if (batchValidation.criticalIssues > 0) {
    batchValidation.recommendations.push(`${batchValidation.criticalIssues} critical validation issues detected`);
    batchValidation.recommendations.push('Check CLI container logs to verify HTTP requests are actually being received');
  }

  if (batchValidation.suspiciousResults > batchValidation.validResults) {
    batchValidation.recommendations.push('More suspicious results than valid ones - possible communication failure');
    batchValidation.recommendations.push('Verify Kubernetes mode environment variables and container networking');
  }

  if (batchValidation.overallTrustScore < 50) {
    batchValidation.recommendations.push('Low trust score indicates possible false positive results');
    batchValidation.recommendations.push('Run tests individually and compare with UI batch results');
  }

  return batchValidation;
}

/**
 * Formats validation results for display in UI
 * @param {Object} validation - Validation result from validateTestResult
 * @returns {string} Formatted warning message for UI display
 */
export function formatValidationWarning(validation) {
  if (!validation || validation.isValid) {
    return '';
  }

  const severityEmojis = {
    low: '⚠️',
    medium: '⚠️',  
    high: '🚨',
    critical: '🔴'
  };

  const emoji = severityEmojis[validation.severity] || '⚠️';
  const warningText = validation.warnings.join('; ');
  
  return `${emoji} Validation Warning: ${warningText}`;
}

/**
 * Checks if HTTP requests are actually being made vs simulated
 * @param {Object} stats - HTTP request statistics
 * @returns {Object} Communication validation result
 */
export function validateHttpCommunication(stats) {
  const commValidation = {
    isActuallyMakingRequests: false,
    confidence: 0,
    issues: [],
    recommendations: []
  };

  if (!stats) {
    commValidation.issues.push('No HTTP request statistics available');
    commValidation.recommendations.push('Enable HTTP request tracking in batch API');
    return commValidation;
  }

  // Check if requests were actually attempted
  if (stats.totalRequests > 0) {
    commValidation.isActuallyMakingRequests = true;
    commValidation.confidence = 70;
  } else {
    commValidation.issues.push('No HTTP requests were attempted despite Kubernetes mode being detected');
    commValidation.recommendations.push('Check environment variable detection logic in UI container');
    return commValidation;
  }

  // Check success rate
  const successRate = stats.totalRequests > 0 ? (stats.successful / stats.totalRequests) * 100 : 0;
  
  if (successRate === 0) {
    commValidation.confidence = 20;
    commValidation.issues.push('All HTTP requests failed - CLI container may not be receiving requests');
    commValidation.recommendations.push('Check CLI container logs for HTTP request receipts');
    commValidation.recommendations.push('Verify container networking and port 8080 accessibility');
  } else if (successRate < 50) {
    commValidation.confidence = 40;
    commValidation.issues.push(`Low HTTP success rate: ${successRate.toFixed(1)}%`);
    commValidation.recommendations.push('Some requests are reaching CLI container but failing - check test execution logs');
  } else {
    commValidation.confidence = Math.min(95, 70 + (successRate - 50) * 0.5);
  }

  return commValidation;
}
