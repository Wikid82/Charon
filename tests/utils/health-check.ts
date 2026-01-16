/**
 * Health Check Utilities - Environment verification for E2E tests
 *
 * These utilities ensure the test environment is healthy and ready
 * before running tests, preventing false failures from infrastructure issues.
 *
 * @example
 * ```typescript
 * // In playwright.config.ts or global setup
 * import { waitForHealthyEnvironment, verifyTestPrerequisites } from './utils/health-check';
 *
 * await waitForHealthyEnvironment('http://localhost:8080');
 * await verifyTestPrerequisites();
 * ```
 */

/**
 * Health response from the API
 */
interface HealthResponse {
  status: string;
  database?: string;
  version?: string;
  uptime?: number;
}

/**
 * Options for health check
 */
export interface HealthCheckOptions {
  /** Maximum time to wait for healthy status (default: 60000ms) */
  timeout?: number;
  /** Interval between health check attempts (default: 2000ms) */
  interval?: number;
  /** Whether to log progress (default: true) */
  verbose?: boolean;
}

/**
 * Wait for the environment to be healthy
 *
 * Polls the health endpoint until the service reports healthy status
 * or the timeout is reached.
 *
 * @param baseURL - Base URL of the application
 * @param options - Configuration options
 * @throws Error if environment doesn't become healthy within timeout
 */
export async function waitForHealthyEnvironment(
  baseURL: string,
  options: HealthCheckOptions = {}
): Promise<void> {
  const { timeout = 60000, interval = 2000, verbose = true } = options;
  const startTime = Date.now();

  if (verbose) {
    console.log(`⏳ Waiting for environment to be healthy at ${baseURL}...`);
  }

  while (Date.now() - startTime < timeout) {
    try {
      const response = await fetch(`${baseURL}/api/v1/health`);

      if (response.ok) {
        const health = (await response.json()) as HealthResponse;

        // Check for healthy status
        const isHealthy =
          health.status === 'healthy' ||
          health.status === 'ok' ||
          health.status === 'up';

        // Check database connectivity if present
        const dbHealthy =
          !health.database ||
          health.database === 'connected' ||
          health.database === 'ok' ||
          health.database === 'healthy';

        if (isHealthy && dbHealthy) {
          if (verbose) {
            console.log('✅ Environment is healthy');
            if (health.version) {
              console.log(`   Version: ${health.version}`);
            }
          }
          return;
        }

        if (verbose) {
          console.log(`   Status: ${health.status}, Database: ${health.database || 'unknown'}`);
        }
      }
    } catch (error) {
      // Service not ready yet - continue waiting
      if (verbose && Date.now() - startTime > 10000) {
        console.log(`   Still waiting... (${Math.round((Date.now() - startTime) / 1000)}s)`);
      }
    }

    await new Promise((resolve) => setTimeout(resolve, interval));
  }

  throw new Error(
    `Environment not healthy after ${timeout}ms. ` +
      `Check that the application is running at ${baseURL}`
  );
}

/**
 * Prerequisite check result
 */
export interface PrerequisiteCheck {
  name: string;
  passed: boolean;
  message?: string;
}

/**
 * Options for prerequisite verification
 */
export interface PrerequisiteOptions {
  /** Base URL (defaults to PLAYWRIGHT_BASE_URL env var) */
  baseURL?: string;
  /** Whether to throw on failure (default: true) */
  throwOnFailure?: boolean;
  /** Whether to log results (default: true) */
  verbose?: boolean;
}

/**
 * Verify all test prerequisites are met
 *
 * Checks critical system requirements before running tests:
 * - API is accessible
 * - Database is writable
 * - Docker is accessible (if needed for proxy tests)
 *
 * @param options - Configuration options
 * @returns Array of check results
 * @throws Error if any critical check fails and throwOnFailure is true
 */
export async function verifyTestPrerequisites(
  options: PrerequisiteOptions = {}
): Promise<PrerequisiteCheck[]> {
  const {
    baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:8080',
    throwOnFailure = true,
    verbose = true,
  } = options;

  const results: PrerequisiteCheck[] = [];

  if (verbose) {
    console.log('🔍 Verifying test prerequisites...');
  }

  // Check 1: API Health
  const apiCheck = await checkAPIHealth(baseURL);
  results.push(apiCheck);
  if (verbose) {
    logCheckResult(apiCheck);
  }

  // Check 2: Database writability (via test endpoint if available)
  const dbCheck = await checkDatabaseWritable(baseURL);
  results.push(dbCheck);
  if (verbose) {
    logCheckResult(dbCheck);
  }

  // Check 3: Docker accessibility (optional - for proxy host tests)
  const dockerCheck = await checkDockerAccessible(baseURL);
  results.push(dockerCheck);
  if (verbose) {
    logCheckResult(dockerCheck);
  }

  // Check 4: Authentication service
  const authCheck = await checkAuthService(baseURL);
  results.push(authCheck);
  if (verbose) {
    logCheckResult(authCheck);
  }

  // Determine if critical checks failed
  const criticalChecks = results.filter(
    (r) => r.name === 'API Health' || r.name === 'Database Writable'
  );
  const failedCritical = criticalChecks.filter((r) => !r.passed);

  if (failedCritical.length > 0 && throwOnFailure) {
    const failedNames = failedCritical.map((r) => r.name).join(', ');
    throw new Error(`Critical prerequisite checks failed: ${failedNames}`);
  }

  if (verbose) {
    const passed = results.filter((r) => r.passed).length;
    console.log(`\n📋 Prerequisites: ${passed}/${results.length} passed`);
  }

  return results;
}

/**
 * Check if the API is responding
 */
async function checkAPIHealth(baseURL: string): Promise<PrerequisiteCheck> {
  try {
    const response = await fetch(`${baseURL}/api/v1/health`, {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });

    if (response.ok) {
      return { name: 'API Health', passed: true };
    }

    return {
      name: 'API Health',
      passed: false,
      message: `HTTP ${response.status}: ${response.statusText}`,
    };
  } catch (error) {
    return {
      name: 'API Health',
      passed: false,
      message: `Connection failed: ${(error as Error).message}`,
    };
  }
}

/**
 * Check if the database is writable
 */
async function checkDatabaseWritable(baseURL: string): Promise<PrerequisiteCheck> {
  try {
    // Try the test endpoint if available
    const response = await fetch(`${baseURL}/api/v1/test/db-check`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });

    if (response.ok) {
      return { name: 'Database Writable', passed: true };
    }

    // If test endpoint doesn't exist, check via health endpoint
    if (response.status === 404) {
      const healthResponse = await fetch(`${baseURL}/api/v1/health`);
      if (healthResponse.ok) {
        const health = (await healthResponse.json()) as HealthResponse;
        const dbOk =
          health.database === 'connected' ||
          health.database === 'ok' ||
          !health.database; // Assume OK if not reported

        return {
          name: 'Database Writable',
          passed: dbOk,
          message: dbOk ? undefined : `Database status: ${health.database}`,
        };
      }
    }

    return {
      name: 'Database Writable',
      passed: false,
      message: `Check failed: HTTP ${response.status}`,
    };
  } catch (error) {
    return {
      name: 'Database Writable',
      passed: false,
      message: `Check failed: ${(error as Error).message}`,
    };
  }
}

/**
 * Check if Docker is accessible (for proxy host tests)
 */
async function checkDockerAccessible(baseURL: string): Promise<PrerequisiteCheck> {
  try {
    const response = await fetch(`${baseURL}/api/v1/test/docker-check`, {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });

    if (response.ok) {
      return { name: 'Docker Accessible', passed: true };
    }

    // If endpoint doesn't exist, mark as skipped (not critical)
    if (response.status === 404) {
      return {
        name: 'Docker Accessible',
        passed: true,
        message: 'Check endpoint not available (skipped)',
      };
    }

    return {
      name: 'Docker Accessible',
      passed: false,
      message: `HTTP ${response.status}`,
    };
  } catch (error) {
    // Docker check is optional - mark as passed with warning
    return {
      name: 'Docker Accessible',
      passed: true,
      message: 'Could not verify (optional)',
    };
  }
}

/**
 * Check if authentication service is working
 */
async function checkAuthService(baseURL: string): Promise<PrerequisiteCheck> {
  try {
    // Try to access login page or auth endpoint
    const response = await fetch(`${baseURL}/api/v1/auth/status`, {
      method: 'GET',
      headers: { Accept: 'application/json' },
    });

    // 401 Unauthorized is expected without auth token
    if (response.ok || response.status === 401) {
      return { name: 'Auth Service', passed: true };
    }

    // Try login endpoint
    if (response.status === 404) {
      const loginResponse = await fetch(`${baseURL}/login`, {
        method: 'GET',
      });

      if (loginResponse.ok || loginResponse.status === 200) {
        return { name: 'Auth Service', passed: true };
      }
    }

    return {
      name: 'Auth Service',
      passed: false,
      message: `Unexpected status: HTTP ${response.status}`,
    };
  } catch (error) {
    return {
      name: 'Auth Service',
      passed: false,
      message: `Check failed: ${(error as Error).message}`,
    };
  }
}

/**
 * Log a check result to console
 */
function logCheckResult(check: PrerequisiteCheck): void {
  const icon = check.passed ? '✅' : '❌';
  const suffix = check.message ? ` (${check.message})` : '';
  console.log(`   ${icon} ${check.name}${suffix}`);
}

/**
 * Quick health check - returns true if environment is ready
 *
 * Use this for conditional test skipping or quick validation.
 *
 * @param baseURL - Base URL of the application
 * @returns true if environment is healthy
 */
export async function isEnvironmentReady(baseURL: string): Promise<boolean> {
  try {
    const response = await fetch(`${baseURL}/api/v1/health`);
    if (!response.ok) return false;

    const health = (await response.json()) as HealthResponse;
    return (
      health.status === 'healthy' ||
      health.status === 'ok' ||
      health.status === 'up'
    );
  } catch {
    return false;
  }
}

/**
 * Get environment info for debugging
 *
 * @param baseURL - Base URL of the application
 * @returns Environment information object
 */
export async function getEnvironmentInfo(
  baseURL: string
): Promise<Record<string, unknown>> {
  try {
    const response = await fetch(`${baseURL}/api/v1/health`);
    if (!response.ok) {
      return { status: 'unhealthy', httpStatus: response.status };
    }

    const health = await response.json();
    return {
      ...health,
      baseURL,
      timestamp: new Date().toISOString(),
    };
  } catch (error) {
    return {
      status: 'unreachable',
      error: (error as Error).message,
      baseURL,
      timestamp: new Date().toISOString(),
    };
  }
}
