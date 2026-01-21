# Charon E2E Testing Plan: Comprehensive Playwright Coverage

**Date:** January 18, 2026
**Status:** Phase 3 Complete - 346+ tests passing
**Priority:** Critical - Blocking new feature development
**Objective:** Establish comprehensive E2E test coverage for all existing Charon features
**Timeline:** 10 weeks (with proper infrastructure setup and comprehensive feature coverage)

> **Revision Note:** This document has been completely revised to address critical infrastructure gaps, expand underspecified sections, and provide implementation-ready specifications. Major additions include test data management, authentication strategy, CI/CD integration, flaky test prevention, and detailed security feature testing.

---

## Table of Contents

1. [Current State & Coverage Gaps](#1-current-state--coverage-gaps)
2. [Testing Infrastructure](#2-testing-infrastructure)
   - 2.1 [Test Environment Setup](#21-test-environment-setup)
   - 2.2 [Test Data Management Strategy](#22-test-data-management-strategy)
   - 2.3 [Authentication Strategy](#23-authentication-strategy)
   - 2.4 [Flaky Test Prevention](#24-flaky-test-prevention)
   - 2.5 [CI/CD Integration](#25-cicd-integration)
3. [Test Organization](#3-test-organization)
4. [Implementation Plan](#4-implementation-plan)
   - Phase 0: Infrastructure Setup (Week 1-2)
   - Phase 1: Foundation (Week 3)
   - Phase 2: Critical Path (Week 4-5)
   - Phase 3: Security Features (Week 6-7)
   - Phase 4: Settings (Week 8)
   - Phase 5: Tasks (Week 9)
   - Phase 6: Integration (Week 10)
5. [Security Feature Testing Strategy](#5-security-feature-testing-strategy)
6. [Risk Mitigation](#6-risk-mitigation)
7. [Success Metrics](#7-success-metrics)
8. [Next Steps](#8-next-steps)

---

## 1. Current State & Coverage Gaps

### Existing Test Files

**Current E2E Test Coverage:**
- ✅ `tests/auth.setup.ts` - Authentication setup (shared fixture)
- ✅ `tests/manual-dns-provider.spec.ts` - Manual DNS provider E2E tests (comprehensive)
- ✅ `tests/dns-provider-crud.spec.ts` - DNS provider CRUD operations
- ✅ `tests/dns-provider-types.spec.ts` - DNS provider type validation
- ✅ `tests/example.spec.js` - Legacy example (can be removed)
- ✅ `tests/fixtures/dns-providers.ts` - Shared DNS test fixtures

**Critical Infrastructure Gaps Identified:**
- ❌ No test data management system (causes data conflicts, FK violations)
- ❌ No per-test user creation (shared auth state breaks parallel execution)
- ❌ No CI/CD integration strategy (no automated testing on PRs)
- ❌ No flaky test prevention utilities (arbitrary timeouts everywhere)
- ❌ No environment setup documentation (manual setup, no verification)
- ❌ No mock external service strategy (tests depend on real services)

### Coverage Gaps

**All major features lack E2E test coverage except DNS providers:**
- ❌ Proxy Hosts management
- ❌ Access Lists (ACL)
- ❌ SSL Certificates
- ❌ CrowdSec integration
- ❌ Coraza WAF
- ❌ Rate Limiting
- ❌ Security Headers
- ❌ Backups & Restore
- ❌ User Management
- ❌ System Settings
- ❌ Audit Logs
- ❌ Remote Servers
- ❌ Uptime Monitoring
- ❌ Notifications
- ❌ Import/Export features
- ❌ Encryption Management

---

## 2. Testing Infrastructure

### 2.1 Test Environment Setup

**Objective:** Ensure consistent, reproducible test environments for local development and CI.

#### 2.1.1 Local Development Setup

**Prerequisites:**
- Docker and Docker Compose installed
- Node.js 18+ and npm
- Go 1.21+ (for backend development)
- Playwright browsers installed (`npx playwright install`)

**Environment Configuration:**

```bash
# .env.test (create in project root)
NODE_ENV=test
DATABASE_URL=sqlite:./data/charon_test.db
BASE_URL=http://localhost:8080
PLAYWRIGHT_BASE_URL=http://localhost:8080
TEST_USER_EMAIL=test-admin@charon.local
TEST_USER_PASSWORD=TestPassword123!
DOCKER_HOST=unix:///var/run/docker.sock
ENABLE_CROWDSEC=false  # Disabled for unit tests, enabled for integration
ENABLE_WAF=false
LOG_LEVEL=warn
```

**Required Docker Services:**

> **Note:** Use the committed `docker-compose.playwright.yml` for E2E testing.
> The `docker-compose.test.yml` is gitignored and reserved for personal/local configurations.

```yaml
# .docker/compose/docker-compose.playwright.yml
# See the actual file for the full configuration with:
# - Charon app service with test environment
# - Optional CrowdSec profile: --profile security-tests
# - Optional MailHog profile: --profile notification-tests
#
# Usage:
#   docker compose -f .docker/compose/docker-compose.playwright.yml up -d
#   docker compose -f .docker/compose/docker-compose.playwright.yml --profile security-tests up -d
**Setup Script:**

```bash
#!/bin/bash
# scripts/setup-e2e-env.sh

set -euo pipefail

echo "🚀 Setting up E2E test environment..."

# 1. Check prerequisites
command -v docker >/dev/null 2>&1 || { echo "❌ Docker not found"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "❌ Node.js not found"; exit 1; }

# 2. Install dependencies
echo "📦 Installing dependencies..."
npm ci

# 3. Install Playwright browsers
echo "🎭 Installing Playwright browsers..."
npx playwright install chromium

# 4. Create test environment file
if [ ! -f .env.test ]; then
  echo "📝 Creating .env.test..."
  cp .env.example .env.test
  # Set test-specific values
  sed -i 's/NODE_ENV=.*/NODE_ENV=test/' .env.test
  sed -i 's/DATABASE_URL=.*/DATABASE_URL=sqlite:\.\/data\/charon_test.db/' .env.test
fi

# 5. Start test environment
echo "🐳 Starting Docker services..."
docker compose -f .docker/compose/docker-compose.playwright.yml up -d

# 6. Wait for service health
echo "⏳ Waiting for service to be healthy..."
timeout 60 bash -c 'until docker compose -f .docker/compose/docker-compose.playwright.yml exec -T charon-app curl -f http://localhost:8080/api/v1/health; do sleep 2; done'

# 7. Run database migrations
echo "🗄️  Running database migrations..."
docker compose -f .docker/compose/docker-compose.playwright.yml exec -T charon-app /app/backend/charon migrate

echo "✅ E2E environment ready!"
echo "📍 Application: http://localhost:8080"
echo "🧪 Run tests: npm run test:e2e"
```

**Environment Health Check:**

```typescript
// tests/utils/health-check.ts

export async function waitForHealthyEnvironment(baseURL: string, timeout = 60000): Promise<void> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    try {
      const response = await fetch(`${baseURL}/api/v1/health`);
      if (response.ok) {
        const health = await response.json();
        if (health.status === 'healthy' && health.database === 'connected') {
          console.log('✅ Environment is healthy');
          return;
        }
      }
    } catch (error) {
      // Service not ready yet
    }
    await new Promise(resolve => setTimeout(resolve, 2000));
  }

  throw new Error(`Environment not healthy after ${timeout}ms`);
}

export async function verifyTestPrerequisites(): Promise<void> {
  const checks = {
    'Database writable': async () => {
      // Attempt to create a test record
      const response = await fetch(`${process.env.PLAYWRIGHT_BASE_URL}/api/v1/test/db-check`, {
        method: 'POST'
      });
      return response.ok;
    },
    'Docker socket accessible': async () => {
      // Check if Docker is available (for proxy host tests)
      const response = await fetch(`${process.env.PLAYWRIGHT_BASE_URL}/api/v1/test/docker-check`);
      return response.ok;
    }
  };

  for (const [name, check] of Object.entries(checks)) {
    try {
      const result = await check();
      if (!result) throw new Error(`Check failed: ${name}`);
      console.log(`✅ ${name}`);
    } catch (error) {
      console.error(`❌ ${name}: ${error}`);
      throw new Error(`Prerequisite check failed: ${name}`);
    }
  }
}
```

#### 2.1.2 CI Environment Configuration

**GitHub Actions Environment:**
- Use `localhost:8080` instead of Tailscale IP
- Run services in Docker containers
- Use GitHub Actions cache for dependencies and browsers
- Upload test artifacts on failure

**Network Configuration:**
```yaml
# In CI, all services communicate via Docker network
services:
  charon:
    networks:
      - test-network
networks:
  test-network:
    driver: bridge
```

#### 2.1.3 Mock External Service Strategy

**DNS Provider API Mocks:**
```typescript
// tests/mocks/dns-provider-api.ts

import { rest } from 'msw';
import { setupServer } from 'msw/node';

export const dnsProviderMocks = [
  // Mock Cloudflare API
  rest.post('https://api.cloudflare.com/client/v4/zones/:zoneId/dns_records', (req, res, ctx) => {
    return res(
      ctx.status(200),
      ctx.json({
        success: true,
        result: { id: 'mock-record-id', name: req.body.name }
      })
    );
  }),

  // Mock Route53 API
  rest.post('https://route53.amazonaws.com/*', (req, res, ctx) => {
    return res(ctx.status(200), ctx.xml('<ChangeInfo><Id>mock-change-id</Id></ChangeInfo>'));
  })
];

export const mockServer = setupServer(...dnsProviderMocks);
```

**ACME Server Mock (for certificate tests):**
```typescript
// tests/mocks/acme-server.ts

export const acmeMocks = [
  // Mock Let's Encrypt directory
  rest.get('https://acme-v02.api.letsencrypt.org/directory', (req, res, ctx) => {
    return res(ctx.json({
      newNonce: 'https://mock-acme/new-nonce',
      newAccount: 'https://mock-acme/new-account',
      newOrder: 'https://mock-acme/new-order'
    }));
  })
];
```

---

### 2.2 Test Data Management Strategy

**Critical Problem:** Current approach uses shared test data, causing conflicts in parallel execution and leaving orphaned records.

**Solution:** Implement `TestDataManager` utility with namespaced isolation and guaranteed cleanup.

#### 2.2.1 TestDataManager Design

```typescript
// tests/utils/TestDataManager.ts

import { APIRequestContext } from '@playwright/test';
import crypto from 'crypto';

export interface ManagedResource {
  id: string;
  type: 'proxy-host' | 'certificate' | 'access-list' | 'dns-provider' | 'user';
  namespace: string;
  createdAt: Date;
}

export class TestDataManager {
  private resources: ManagedResource[] = [];
  private namespace: string;
  private request: APIRequestContext;

  constructor(request: APIRequestContext, testName?: string) {
    this.request = request;
    // Create unique namespace per test to avoid conflicts
    this.namespace = testName
      ? `test-${this.sanitize(testName)}-${Date.now()}`
      : `test-${crypto.randomUUID()}`;
  }

  private sanitize(name: string): string {
    return name.toLowerCase().replace(/[^a-z0-9]/g, '-').substring(0, 30);
  }

  /**
   * Create a proxy host with automatic cleanup tracking
   */
  async createProxyHost(data: {
    domain: string;
    forwardHost: string;
    forwardPort: number;
    scheme?: 'http' | 'https';
  }): Promise<{ id: string; domain: string }> {
    const namespaced = {
      ...data,
      domain: `${this.namespace}.${data.domain}` // Ensure unique domain
    };

    const response = await this.request.post('/api/v1/proxy-hosts', {
      data: namespaced
    });

    if (!response.ok()) {
      throw new Error(`Failed to create proxy host: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.uuid,
      type: 'proxy-host',
      namespace: this.namespace,
      createdAt: new Date()
    });

    return { id: result.uuid, domain: namespaced.domain };
  }

  /**
   * Create an access list with automatic cleanup
   */
  async createAccessList(data: {
    name: string;
    rules: Array<{ type: 'allow' | 'deny'; value: string }>;
  }): Promise<{ id: string }> {
    const namespaced = {
      ...data,
      name: `${this.namespace}-${data.name}`
    };

    const response = await this.request.post('/api/v1/access-lists', {
      data: namespaced
    });

    if (!response.ok()) {
      throw new Error(`Failed to create access list: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'access-list',
      namespace: this.namespace,
      createdAt: new Date()
    });

    return { id: result.id };
  }

  /**
   * Create a certificate with automatic cleanup
   */
  async createCertificate(data: {
    domains: string[];
    type: 'letsencrypt' | 'custom';
    privateKey?: string;
    certificate?: string;
  }): Promise<{ id: string }> {
    const namespaced = {
      ...data,
      domains: data.domains.map(d => `${this.namespace}.${d}`)
    };

    const response = await this.request.post('/api/v1/certificates', {
      data: namespaced
    });

    if (!response.ok()) {
      throw new Error(`Failed to create certificate: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'certificate',
      namespace: this.namespace,
      createdAt: new Date()
    });

    return { id: result.id };
  }

  /**
   * Create a DNS provider with automatic cleanup
   */
  async createDNSProvider(data: {
    type: 'manual' | 'cloudflare' | 'route53';
    name: string;
    credentials?: Record<string, string>;
  }): Promise<{ id: string }> {
    const namespaced = {
      ...data,
      name: `${this.namespace}-${data.name}`
    };

    const response = await this.request.post('/api/v1/dns-providers', {
      data: namespaced
    });

    if (!response.ok()) {
      throw new Error(`Failed to create DNS provider: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'dns-provider',
      namespace: this.namespace,
      createdAt: new Date()
    });

    return { id: result.id };
  }

  /**
   * Create a test user with automatic cleanup
   */
  async createUser(data: {
    email: string;
    password: string;
    role: 'admin' | 'user' | 'guest';
  }): Promise<{ id: string; email: string; token: string }> {
    const namespaced = {
      ...data,
      email: `${this.namespace}+${data.email}`
    };

    const response = await this.request.post('/api/v1/users', {
      data: namespaced
    });

    if (!response.ok()) {
      throw new Error(`Failed to create user: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'user',
      namespace: this.namespace,
      createdAt: new Date()
    });

    // Automatically log in the user and return token
    const loginResponse = await this.request.post('/api/v1/auth/login', {
      data: { email: namespaced.email, password: data.password }
    });

    const { token } = await loginResponse.json();

    return { id: result.id, email: namespaced.email, token };
  }

  /**
   * Clean up all resources in reverse order (respects FK constraints)
   */
  async cleanup(): Promise<void> {
    // Sort by creation time (newest first) to respect dependencies
    const sortedResources = [...this.resources].sort(
      (a, b) => b.createdAt.getTime() - a.createdAt.getTime()
    );

    const errors: Error[] = [];

    for (const resource of sortedResources) {
      try {
        await this.deleteResource(resource);
      } catch (error) {
        errors.push(error as Error);
        console.error(`Failed to cleanup ${resource.type}:${resource.id}:`, error);
      }
    }

    this.resources = [];

    if (errors.length > 0) {
      throw new Error(`Cleanup completed with ${errors.length} errors`);
    }
  }

  private async deleteResource(resource: ManagedResource): Promise<void> {
    const endpoints = {
      'proxy-host': `/api/v1/proxy-hosts/${resource.id}`,
      'certificate': `/api/v1/certificates/${resource.id}`,
      'access-list': `/api/v1/access-lists/${resource.id}`,
      'dns-provider': `/api/v1/dns-providers/${resource.id}`,
      'user': `/api/v1/users/${resource.id}`
    };

    const endpoint = endpoints[resource.type];
    const response = await this.request.delete(endpoint);

    if (!response.ok() && response.status() !== 404) {
      throw new Error(`Failed to delete ${resource.type}: ${await response.text()}`);
    }
  }

  /**
   * Get all resources created in this namespace
   */
  getResources(): ManagedResource[] {
    return [...this.resources];
  }

  /**
   * Get namespace identifier
   */
  getNamespace(): string {
    return this.namespace;
  }
}
```

#### 2.2.2 Usage Pattern

```typescript
// Example test using TestDataManager
import { test, expect } from '@playwright/test';
import { TestDataManager } from './utils/TestDataManager';

test.describe('Proxy Host Management', () => {
  let testData: TestDataManager;

  test.beforeEach(async ({ request }, testInfo) => {
    testData = new TestDataManager(request, testInfo.title);
  });

  test.afterEach(async () => {
    await testData.cleanup();
  });

  test('should create and delete proxy host', async ({ page, request }) => {
    await test.step('Create proxy host', async () => {
      const { id, domain } = await testData.createProxyHost({
        domain: 'app.example.com',
        forwardHost: '192.168.1.100',
        forwardPort: 3000,
        scheme: 'http'
      });

      await page.goto('/proxy-hosts');
      await expect(page.getByText(domain)).toBeVisible();
    });

    // Cleanup happens automatically in afterEach
  });
});
```

#### 2.2.3 Database Seeding Strategy

**Seed Data for Reference Tests:**
```typescript
// tests/fixtures/seed-data.ts

export async function seedReferenceData(request: APIRequestContext): Promise<void> {
  // Create stable reference data that doesn't change
  const referenceData = {
    accessLists: [
      { name: 'Global Allowlist', rules: [{ type: 'allow', value: '0.0.0.0/0' }] }
    ],
    dnsProviders: [
      { type: 'manual', name: 'Manual DNS (Default)' }
    ]
  };

  // Idempotent seeding - only create if not exists
  for (const list of referenceData.accessLists) {
    const response = await request.get('/api/v1/access-lists', {
      params: { name: list.name }
    });
    const existing = await response.json();
    if (existing.length === 0) {
      await request.post('/api/v1/access-lists', { data: list });
    }
  }
}
```

#### 2.2.4 Parallel Execution Handling

**Test Isolation Strategy:**
- Each test worker gets its own namespace via `TestDataManager`
- Database transactions are NOT used (not supported in E2E context)
- Unique identifiers prevent collisions (domain names, usernames)
- Cleanup runs independently per test

**Worker ID Integration:**
```typescript
// playwright.config.ts adjustment
export default defineConfig({
  workers: process.env.CI ? 4 : undefined,
  use: {
    storageState: ({ workerIndex }) => `auth/state-worker-${workerIndex}.json`
  }
});
```

---

### 2.3 Authentication Strategy

**Critical Problem:** Current `auth.setup.ts` uses a single shared user, causing race conditions in parallel execution.

**Solution:** Per-test user creation with role-based fixtures.

#### 2.3.1 Per-Test User Creation

```typescript
// tests/fixtures/auth-fixtures.ts

import { test as base, expect, APIRequestContext } from '@playwright/test';
import { TestDataManager } from '../utils/TestDataManager';

export interface TestUser {
  id: string;
  email: string;
  token: string;
  role: 'admin' | 'user' | 'guest';
}

interface AuthFixtures {
  authenticatedUser: TestUser;
  adminUser: TestUser;
  regularUser: TestUser;
  guestUser: TestUser;
  testData: TestDataManager;
}

export const test = base.extend<AuthFixtures>({
  testData: async ({ request }, use, testInfo) => {
    const manager = new TestDataManager(request, testInfo.title);
    await use(manager);
    await manager.cleanup();
  },

  // Default authenticated user (admin role)
  authenticatedUser: async ({ testData }, use, testInfo) => {
    const user = await testData.createUser({
      email: `admin-${Date.now()}@test.local`,
      password: 'TestPass123!',
      role: 'admin'
    });
    await use(user);
  },

  // Explicit admin user fixture
  adminUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      email: `admin-${Date.now()}@test.local`,
      password: 'TestPass123!',
      role: 'admin'
    });
    await use(user);
  },

  // Regular user (non-admin)
  regularUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      email: `user-${Date.now()}@test.local`,
      password: 'TestPass123!',
      role: 'user'
    });
    await use(user);
  },

  // Guest user (read-only)
  guestUser: async ({ testData }, use) => {
    const user = await testData.createUser({
      email: `guest-${Date.now()}@test.local`,
      password: 'TestPass123!',
      role: 'guest'
    });
    await use(user);
  }
});

export { expect } from '@playwright/test';
```

#### 2.3.2 Usage Pattern

```typescript
// Example test with per-test authentication
import { test, expect } from './fixtures/auth-fixtures';

test.describe('User Management', () => {
  test('admin can create users', async ({ page, adminUser }) => {
    await test.step('Login as admin', async () => {
      await page.goto('/login');
      await page.getByLabel('Email').fill(adminUser.email);
      await page.getByLabel('Password').fill('TestPass123!');
      await page.getByRole('button', { name: 'Login' }).click();
      await page.waitForURL('/');
    });

    await test.step('Create new user', async () => {
      await page.goto('/users');
      await page.getByRole('button', { name: 'Add User' }).click();
      // ... rest of test
    });
  });

  test('regular user cannot create users', async ({ page, regularUser }) => {
    await test.step('Login as regular user', async () => {
      await page.goto('/login');
      await page.getByLabel('Email').fill(regularUser.email);
      await page.getByLabel('Password').fill('TestPass123!');
      await page.getByRole('button', { name: 'Login' }).click();
      await page.waitForURL('/');
    });

    await test.step('Verify no access to user management', async () => {
      await page.goto('/users');
      await expect(page.getByText('Access Denied')).toBeVisible();
    });
  });
});
```

#### 2.3.3 Storage State Management

**Per-Worker Storage:**
```typescript
// tests/auth.setup.ts (revised)

import { test as setup, expect } from '@playwright/test';
import { TestDataManager } from './utils/TestDataManager';

// Generate storage state per worker
const authFile = process.env.CI
  ? `auth/state-worker-${process.env.TEST_WORKER_INDEX || 0}.json`
  : 'auth/state.json';

setup('authenticate', async ({ request, page }) => {
  const testData = new TestDataManager(request, 'setup');

  try {
    // Create a dedicated setup user for this worker
    const user = await testData.createUser({
      email: `setup-worker-${process.env.TEST_WORKER_INDEX || 0}@test.local`,
      password: 'SetupPass123!',
      role: 'admin'
    });

    await page.goto('/login');
    await page.getByLabel('Email').fill(user.email);
    await page.getByLabel('Password').fill('SetupPass123!');
    await page.getByRole('button', { name: 'Login' }).click();
    await page.waitForURL('/');

    // Save authenticated state
    await page.context().storageState({ path: authFile });

    console.log(`✅ Auth state saved for worker ${process.env.TEST_WORKER_INDEX || 0}`);
  } finally {
    // Cleanup happens automatically via TestDataManager
    await testData.cleanup();
  }
});
```

---

### 2.4 Flaky Test Prevention

**Critical Problem:** Arbitrary timeouts (`page.waitForTimeout(1000)`) cause flaky tests and slow execution.

**Solution:** Deterministic wait utilities that poll for specific conditions.

#### 2.4.1 Wait Utilities

```typescript
// tests/utils/wait-helpers.ts

import { Page, Locator, expect } from '@playwright/test';

/**
 * Wait for a toast notification with specific text
 */
export async function waitForToast(
  page: Page,
  text: string | RegExp,
  options: { timeout?: number; type?: 'success' | 'error' | 'info' } = {}
): Promise<void> {
  const { timeout = 10000, type } = options;

  const toastSelector = type
    ? `[role="alert"][data-type="${type}"]`
    : '[role="alert"]';

  const toast = page.locator(toastSelector);
  await expect(toast).toContainText(text, { timeout });
}

/**
 * Wait for a specific API response
 */
export async function waitForAPIResponse(
  page: Page,
  urlPattern: string | RegExp,
  options: { status?: number; timeout?: number } = {}
): Promise<Response> {
  const { status, timeout = 30000 } = options;

  const responsePromise = page.waitForResponse(
    (response) => {
      const matchesURL = typeof urlPattern === 'string'
        ? response.url().includes(urlPattern)
        : urlPattern.test(response.url());

      const matchesStatus = status ? response.status() === status : true;

      return matchesURL && matchesStatus;
    },
    { timeout }
  );

  return await responsePromise;
}

/**
 * Wait for loading spinner to disappear
 */
export async function waitForLoadingComplete(
  page: Page,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  // Wait for any loading indicator to disappear
  const loader = page.locator('[role="progressbar"], [aria-busy="true"], .loading-spinner');
  await expect(loader).toHaveCount(0, { timeout });
}

/**
 * Wait for specific element count (e.g., table rows)
 */
export async function waitForElementCount(
  locator: Locator,
  count: number,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 10000 } = options;
  await expect(locator).toHaveCount(count, { timeout });
}

/**
 * Wait for WebSocket connection to be established
 */
export async function waitForWebSocketConnection(
  page: Page,
  urlPattern: string | RegExp,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  await page.waitForEvent('websocket', {
    predicate: (ws) => {
      const matchesURL = typeof urlPattern === 'string'
        ? ws.url().includes(urlPattern)
        : urlPattern.test(ws.url());
      return matchesURL;
    },
    timeout
  });
}

/**
 * Wait for WebSocket message with specific content
 */
export async function waitForWebSocketMessage(
  page: Page,
  matcher: (data: string | Buffer) => boolean,
  options: { timeout?: number } = {}
): Promise<string | Buffer> {
  const { timeout = 10000 } = options;

  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error(`WebSocket message not received within ${timeout}ms`));
    }, timeout);

    page.on('websocket', (ws) => {
      ws.on('framereceived', (event) => {
        const data = event.payload;
        if (matcher(data)) {
          clearTimeout(timer);
          resolve(data);
        }
      });
    });
  });
}

/**
 * Wait for progress bar to complete
 */
export async function waitForProgressComplete(
  page: Page,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 30000 } = options;

  const progressBar = page.locator('[role="progressbar"]');

  // Wait for progress to reach 100% or disappear
  await page.waitForFunction(
    () => {
      const bar = document.querySelector('[role="progressbar"]');
      if (!bar) return true; // Progress bar gone = complete
      const value = bar.getAttribute('aria-valuenow');
      return value === '100';
    },
    { timeout }
  );

  // Wait for progress bar to disappear
  await expect(progressBar).toHaveCount(0, { timeout: 5000 });
}

/**
 * Wait for modal to open
 */
export async function waitForModal(
  page: Page,
  titleText: string | RegExp,
  options: { timeout?: number } = {}
): Promise<Locator> {
  const { timeout = 10000 } = options;

  const modal = page.locator('[role="dialog"]');
  await expect(modal).toBeVisible({ timeout });

  if (titleText) {
    await expect(modal.getByRole('heading')).toContainText(titleText);
  }

  return modal;
}

/**
 * Wait for dropdown/listbox to open
 */
export async function waitForDropdown(
  page: Page,
  triggerId: string,
  options: { timeout?: number } = {}
): Promise<Locator> {
  const { timeout = 5000 } = options;

  const trigger = page.locator(`#${triggerId}`);
  const expanded = await trigger.getAttribute('aria-expanded');

  if (expanded !== 'true') {
    throw new Error(`Dropdown ${triggerId} is not expanded`);
  }

  const listboxId = await trigger.getAttribute('aria-controls');
  if (!listboxId) {
    throw new Error(`Dropdown ${triggerId} has no aria-controls`);
  }

  const listbox = page.locator(`#${listboxId}`);
  await expect(listbox).toBeVisible({ timeout });

  return listbox;
}

/**
 * Wait for table to finish loading and render rows
 */
export async function waitForTableLoad(
  page: Page,
  tableRole: string = 'table',
  options: { minRows?: number; timeout?: number } = {}
): Promise<void> {
  const { minRows = 0, timeout = 10000 } = options;

  const table = page.getByRole(tableRole);
  await expect(table).toBeVisible({ timeout });

  // Wait for loading state to clear
  await waitForLoadingComplete(page);

  // If minimum rows specified, wait for them
  if (minRows > 0) {
    const rows = table.locator('tbody tr');
    await expect(rows).toHaveCount(minRows, { timeout });
  }
}

/**
 * Retry an action until it succeeds or timeout
 */
export async function retryAction<T>(
  action: () => Promise<T>,
  options: {
    maxAttempts?: number;
    interval?: number;
    timeout?: number;
  } = {}
): Promise<T> {
  const { maxAttempts = 5, interval = 1000, timeout = 30000 } = options;

  const startTime = Date.now();
  let lastError: Error | undefined;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    if (Date.now() - startTime > timeout) {
      throw new Error(`Retry timeout after ${timeout}ms`);
    }

    try {
      return await action();
    } catch (error) {
      lastError = error as Error;
      if (attempt < maxAttempts) {
        await new Promise(resolve => setTimeout(resolve, interval));
      }
    }
  }

  throw lastError || new Error('Retry failed');
}
```

#### 2.4.2 Usage Examples

```typescript
import { test, expect } from '@playwright/test';
import {
  waitForToast,
  waitForAPIResponse,
  waitForLoadingComplete,
  waitForModal
} from './utils/wait-helpers';

test('create proxy host with deterministic waits', async ({ page }) => {
  await test.step('Navigate and open form', async () => {
    await page.goto('/proxy-hosts');
    await page.getByRole('button', { name: 'Add Proxy Host' }).click();
    await waitForModal(page, 'Create Proxy Host');
  });

  await test.step('Fill form and submit', async () => {
    await page.getByLabel('Domain Name').fill('test.example.com');
    await page.getByLabel('Forward Host').fill('192.168.1.100');
    await page.getByLabel('Forward Port').fill('3000');

    // Wait for API call to complete
    const responsePromise = waitForAPIResponse(page, '/api/v1/proxy-hosts', { status: 201 });
    await page.getByRole('button', { name: 'Save' }).click();
    await responsePromise;
  });

  await test.step('Verify success', async () => {
    await waitForToast(page, 'Proxy host created successfully', { type: 'success' });
    await waitForLoadingComplete(page);
    await expect(page.getByRole('row', { name: /test.example.com/ })).toBeVisible();
  });
});
```

---

### 2.5 CI/CD Integration

**Objective:** Automate E2E test execution on every PR with parallel execution, comprehensive reporting, and failure artifacts.

#### 2.5.1 GitHub Actions Workflow

```yaml
# .github/workflows/e2e-tests.yml

name: E2E Tests (Playwright)

on:
  pull_request:
    branches: [main, develop]
    paths:
      - 'frontend/**'
      - 'backend/**'
      - 'tests/**'
      - 'playwright.config.js'
      - '.github/workflows/e2e-tests.yml'
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      browser:
        description: 'Browser to test'
        required: false
        default: 'chromium'
        type: choice
        options:
          - chromium
          - firefox
          - webkit
          - all

env:
  NODE_VERSION: '18'
  GO_VERSION: '1.21'

jobs:
  # Build application once, share across test shards
  build:
    name: Build Application
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Build frontend
        run: npm run build
        working-directory: frontend

      - name: Build backend
        run: make build
        working-directory: backend

      - name: Build Docker image
        run: |
          docker build -t charon:test .
          docker save charon:test -o charon-test-image.tar

      - name: Upload Docker image
        uses: actions/upload-artifact@v4
        with:
          name: docker-image
          path: charon-test-image.tar
          retention-days: 1

  # Run tests in parallel shards
  e2e-tests:
    name: E2E Tests (Shard ${{ matrix.shard }})
    runs-on: ubuntu-latest
    needs: build
    timeout-minutes: 30
    strategy:
      fail-fast: false
      matrix:
        shard: [1, 2, 3, 4]
        browser: [chromium] # Can be extended to [chromium, firefox, webkit]

    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: 'npm'

      - name: Download Docker image
        uses: actions/download-artifact@v4
        with:
          name: docker-image

      - name: Load Docker image
        run: docker load -i charon-test-image.tar

      - name: Start test environment
        run: |
          docker compose -f .docker/compose/docker-compose.playwright.yml up -d

      - name: Wait for service healthy
        run: |
          timeout 60 bash -c 'until curl -f http://localhost:8080/api/v1/health; do sleep 2; done'

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright browsers
        run: npx playwright install --with-deps ${{ matrix.browser }}

      - name: Run E2E tests (Shard ${{ matrix.shard }})
        run: |
          npx playwright test \
            --project=${{ matrix.browser }} \
            --shard=${{ matrix.shard }}/4 \
            --reporter=html,json,junit
        env:
          PLAYWRIGHT_BASE_URL: http://localhost:8080
          CI: true
          TEST_WORKER_INDEX: ${{ matrix.shard }}

      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: test-results-${{ matrix.browser }}-shard-${{ matrix.shard }}
          path: |
            playwright-report/
            test-results/
          retention-days: 7

      - name: Upload test traces
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: traces-${{ matrix.browser }}-shard-${{ matrix.shard }}
          path: test-results/**/*.zip
          retention-days: 7

      - name: Collect Docker logs
        if: failure()
        run: |
          docker compose -f .docker/compose/docker-compose.playwright.yml logs > docker-logs.txt

      - name: Upload Docker logs
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: docker-logs-shard-${{ matrix.shard }}
          path: docker-logs.txt
          retention-days: 7

  # Merge reports from all shards
  merge-reports:
    name: Merge Test Reports
    runs-on: ubuntu-latest
    needs: e2e-tests
    if: always()
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: ${{ env.NODE_VERSION }}

      - name: Download all reports
        uses: actions/download-artifact@v4
        with:
          pattern: test-results-*
          path: all-results

      - name: Merge Playwright HTML reports
        run: npx playwright merge-reports --reporter html all-results

      - name: Upload merged report
        uses: actions/upload-artifact@v4
        with:
          name: merged-playwright-report
          path: playwright-report/
          retention-days: 30

      - name: Generate summary
        run: |
          echo "## E2E Test Results" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          cat all-results/*/report.json | jq -s '
            {
              total: (map(.stats.expected + .stats.unexpected + .stats.flaky + .stats.skipped) | add),
              passed: (map(.stats.expected) | add),
              failed: (map(.stats.unexpected) | add),
              flaky: (map(.stats.flaky) | add),
              skipped: (map(.stats.skipped) | add)
            }
          ' | jq -r '"- **Total**: \(.total)\n- **Passed**: \(.passed)\n- **Failed**: \(.failed)\n- **Flaky**: \(.flaky)\n- **Skipped**: \(.skipped)"' >> $GITHUB_STEP_SUMMARY

  # Comment on PR with results
  comment-results:
    name: Comment Test Results on PR
    runs-on: ubuntu-latest
    needs: merge-reports
    if: github.event_name == 'pull_request'
    permissions:
      pull-requests: write
    steps:
      - name: Download merged report
        uses: actions/download-artifact@v4
        with:
          name: merged-playwright-report
          path: playwright-report

      - name: Extract test stats
        id: stats
        run: |
          STATS=$(cat playwright-report/report.json | jq -c '.stats')
          echo "stats=$STATS" >> $GITHUB_OUTPUT

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const stats = JSON.parse('${{ steps.stats.outputs.stats }}');
            const passed = stats.expected;
            const failed = stats.unexpected;
            const flaky = stats.flaky;
            const total = passed + failed + flaky + stats.skipped;

            const emoji = failed > 0 ? '❌' : flaky > 0 ? '⚠️' : '✅';
            const status = failed > 0 ? 'FAILED' : flaky > 0 ? 'FLAKY' : 'PASSED';

            const body = `## ${emoji} E2E Test Results: ${status}

            | Metric | Count |
            |--------|-------|
            | Total | ${total} |
            | Passed | ✅ ${passed} |
            | Failed | ❌ ${failed} |
            | Flaky | ⚠️ ${flaky} |
            | Skipped | ⏭️ ${stats.skipped} |

            [View full Playwright report](https://github.com/${{ github.repository }}/actions/runs/${{ github.run_id }})

            ${failed > 0 ? '⚠️ **Tests failed!** Please review the failures and fix before merging.' : ''}
            ${flaky > 0 ? '⚠️ **Flaky tests detected!** Please investigate and stabilize before merging.' : ''}
            `;

            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: body
            });

  # Block merge if tests fail
  e2e-results:
    name: E2E Test Results
    runs-on: ubuntu-latest
    needs: e2e-tests
    if: always()
    steps:
      - name: Check test results
        run: |
          if [ "${{ needs.e2e-tests.result }}" != "success" ]; then
            echo "E2E tests failed or were cancelled"
            exit 1
          fi
```

#### 2.5.2 Test Sharding Strategy

**Why Shard:** Reduces CI run time from ~40 minutes to ~10 minutes with 4 parallel shards.

**Sharding Configuration:**
```typescript
// playwright.config.ts
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  workers: process.env.CI ? 4 : undefined,
  retries: process.env.CI ? 2 : 0,

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] }
    }
  ],

  // CI-specific optimizations
  ...(process.env.CI && {
    reporter: [
      ['html', { outputFolder: 'playwright-report' }],
      ['json', { outputFile: 'test-results/results.json' }],
      ['junit', { outputFile: 'test-results/junit.xml' }]
    ],
    maxFailures: 10 // Stop after 10 failures to save CI time
  })
});
```

**Shard Distribution:**
- Shard 1: `tests/core/**`, `tests/proxy/**` (~10 min)
- Shard 2: `tests/dns/**`, `tests/certificates/**` (~10 min)
- Shard 3: `tests/security/**` (~10 min)
- Shard 4: `tests/settings/**`, `tests/tasks/**`, `tests/monitoring/**`, `tests/integration/**` (~10 min)

#### 2.5.3 Cache Strategy

```yaml
# Cache Playwright browsers
- name: Cache Playwright browsers
  uses: actions/cache@v4
  with:
    path: ~/.cache/ms-playwright
    key: playwright-browsers-${{ hashFiles('package-lock.json') }}

# Cache npm dependencies
- name: Cache npm dependencies
  uses: actions/cache@v4
  with:
    path: ~/.npm
    key: npm-${{ hashFiles('package-lock.json') }}

# Cache Docker layers
- name: Cache Docker layers
  uses: actions/cache@v4
  with:
    path: /tmp/.buildx-cache
    key: docker-${{ github.sha }}
    restore-keys: docker-
```

#### 2.5.4 Failure Notification Strategy

**Slack Notification (Optional):**
```yaml
- name: Notify Slack on failure
  if: failure() && github.event_name == 'push'
  uses: slackapi/slack-github-action@v1
  with:
    payload: |
      {
        "text": "❌ E2E tests failed on ${{ github.ref }}",
        "blocks": [
          {
            "type": "section",
            "text": {
              "type": "mrkdwn",
              "text": "*E2E Tests Failed*\n\nBranch: `${{ github.ref }}`\nCommit: `${{ github.sha }}`\n<https://github.com/${{ github.repository }}/actions/runs/${{ github.run_id }}|View Run>"
            }
          }
        ]
      }
  env:
    SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

---

## 3. Test Organization

**Directory Structure:**
```
tests/
├── auth.setup.ts                          # ✅ Exists - Authentication setup
├── fixtures/                              # Shared test data and utilities
│   ├── dns-providers.ts                   # ✅ Exists - DNS fixtures
│   ├── proxy-hosts.ts                     # 📝 To create
│   ├── access-lists.ts                    # 📝 To create
│   ├── certificates.ts                    # 📝 To create
│   └── test-data.ts                       # 📝 To create - Common test data
├── core/                                  # Core application features
│   ├── authentication.spec.ts             # 📝 Auth flows
│   ├── dashboard.spec.ts                  # 📝 Dashboard UI
│   └── navigation.spec.ts                 # 📝 App navigation
├── proxy/                                 # Proxy management
│   ├── proxy-hosts-crud.spec.ts           # 📝 CRUD operations
│   ├── proxy-hosts-validation.spec.ts     # 📝 Form validation
│   ├── proxy-hosts-docker.spec.ts         # 📝 Docker discovery
│   └── remote-servers.spec.ts             # 📝 Remote Docker servers
├── dns/                                   # DNS management
│   ├── manual-dns-provider.spec.ts        # ✅ Exists - Manual provider
│   ├── dns-provider-crud.spec.ts          # ✅ Exists - CRUD operations
│   ├── dns-provider-types.spec.ts         # ✅ Exists - Provider types
│   ├── dns-provider-credentials.spec.ts   # 📝 Credential management
│   └── dns-plugins.spec.ts                # 📝 Plugin management
├── certificates/                          # SSL certificate management
│   ├── certificates-list.spec.ts          # 📝 List and view
│   ├── certificates-upload.spec.ts        # 📝 Upload custom certs
│   └── certificates-acme.spec.ts          # 📝 ACME integration
├── security/                              # Cerberus security suite
│   ├── access-lists-crud.spec.ts          # 📝 ACL CRUD
│   ├── access-lists-rules.spec.ts         # 📝 ACL rule engine
│   ├── crowdsec-config.spec.ts            # 📝 CrowdSec setup
│   ├── crowdsec-decisions.spec.ts         # 📝 Ban management
│   ├── crowdsec-presets.spec.ts           # 📝 Preset management
│   ├── waf-config.spec.ts                 # 📝 WAF configuration
│   ├── rate-limiting.spec.ts              # 📝 Rate limit rules
│   ├── security-headers.spec.ts           # 📝 Security headers
│   └── audit-logs.spec.ts                 # 📝 Audit trail
├── settings/                              # System configuration
│   ├── system-settings.spec.ts            # 📝 System config
│   ├── smtp-settings.spec.ts              # 📝 Email config
│   ├── notifications.spec.ts              # 📝 Notification config
│   ├── user-management.spec.ts            # 📝 User CRUD
│   ├── encryption-management.spec.ts      # 📝 Key rotation
│   └── account-settings.spec.ts           # 📝 User profile
├── tasks/                                 # Background tasks & maintenance
│   ├── backups-create.spec.ts             # 📝 Backup creation
│   ├── backups-restore.spec.ts            # 📝 Backup restoration
│   ├── logs-viewing.spec.ts               # 📝 Log viewer
│   ├── import-caddyfile.spec.ts           # 📝 Caddy import
│   └── import-crowdsec.spec.ts            # 📝 CrowdSec import
├── monitoring/                            # Monitoring features
│   ├── uptime-monitoring.spec.ts          # 📝 Uptime checks
│   └── real-time-logs.spec.ts             # 📝 WebSocket logs
└── integration/                           # Cross-feature integration
    ├── proxy-acl-integration.spec.ts      # 📝 Proxy + ACL
    ├── proxy-certificate.spec.ts          # 📝 Proxy + SSL
    ├── security-suite-integration.spec.ts # 📝 Full security stack
    └── backup-restore-e2e.spec.ts         # 📝 Full backup cycle
```

### Test Execution Strategy

**Playwright Configuration:**
- ✅ Base URL: `http://100.98.12.109:8080` (Tailscale IP) or `http://localhost:8080` (CI)
- ✅ Browser support: Chromium (primary), Firefox, WebKit
- ✅ Parallel execution: Enabled for faster runs
- ✅ Authentication: Shared state via `auth.setup.ts`
- ✅ Timeouts: 30s test, 5s expect
- ✅ Retries: 2 on CI, 0 on local

**Test Data Management:**
- Use fixtures for reusable test data
- Clean up created resources after tests
- Use unique identifiers for test resources (timestamps, UUIDs)
- Avoid hardcoded IDs or names that could conflict

**Accessibility Testing:**
- All tests must verify keyboard navigation
- Use `toMatchAriaSnapshot` for component structure validation
- Verify ARIA labels, roles, and live regions
- Test screen reader announcements for state changes

---

## 4. Implementation Plan

### Phase 0: Infrastructure Setup (Week 1-2)

**Goal:** Build robust test infrastructure before writing feature tests

#### Week 1: Core Infrastructure
**Priority:** Critical - Blocking all test development
**Estimated Effort:** 5 days

**Tasks:**
- [ ] Set up `TestDataManager` utility with namespace isolation
- [ ] Implement per-test user creation in `auth-fixtures.ts`
- [ ] Create all wait helper utilities (`waitForToast`, `waitForAPIResponse`, etc.)
- [ ] Configure test environment Docker Compose files (`.test.yml`)
- [ ] Write `setup-e2e-env.sh` script with health checks
- [ ] Implement mock external services (DNS providers, ACME servers)
- [ ] Configure test environment variables in `.env.test`

**Acceptance Criteria:**
- `TestDataManager` can create and cleanup all resource types
- Per-test users can be created with different roles
- Wait utilities replace all `page.waitForTimeout()` calls
- Test environment starts reliably with `npm run test:env:start`
- Mock services respond to API calls correctly

#### Week 2: CI/CD Integration
**Priority:** Critical - Required for PR automation
**Estimated Effort:** 5 days

**Tasks:**
- [ ] Create `.github/workflows/e2e-tests.yml`
- [ ] Implement test sharding strategy (4 shards)
- [ ] Configure artifact upload (reports, traces, logs)
- [ ] Set up PR comment reporting
- [ ] Configure caching for npm, Playwright browsers, Docker layers
- [ ] Test workflow end-to-end on a feature branch
- [ ] Document CI/CD troubleshooting guide

**Acceptance Criteria:**
- E2E tests run automatically on every PR
- Test results appear as PR comments
- Failed tests upload traces and logs
- CI run completes in <15 minutes with sharding
- Flaky test retries work correctly

### Phase 1: Foundation (Week 3)

**Status:** ✅ COMPLETE
**Completion Date:** January 17, 2026
**Test Results:** 112/119 passing (94%)

**Goal:** Establish core application testing patterns

#### 1.1 Test Fixtures & Helpers
**Priority:** Critical
**Status:** ✅ Complete

**Delivered Files:**
- [x] `tests/fixtures/test-data.ts` - Common test data generators
- [x] `tests/fixtures/proxy-hosts.ts` - Mock proxy host data
- [x] `tests/fixtures/access-lists.ts` - Mock ACL data
- [x] `tests/fixtures/certificates.ts` - Mock certificate data
- [x] `tests/fixtures/auth-fixtures.ts` - Per-test authentication
- [x] `tests/fixtures/navigation.ts` - Navigation helpers
- [x] `tests/utils/api-helpers.ts` - Common API operations
- [x] `tests/utils/wait-helpers.ts` - Deterministic wait utilities
- [x] `tests/utils/test-data-manager.ts` - Test data isolation
- [x] `tests/utils/accessibility-helpers.ts` - A11y testing utilities

**Acceptance Criteria:** ✅ Met
- Fixtures provide consistent, reusable test data
- API helpers reduce code duplication
- All utilities have JSDoc comments and usage examples

**Test File Template:**
```typescript
import { test, expect } from './fixtures/auth-fixtures'; // Use custom fixtures
import { TestDataManager } from './utils/TestDataManager';

test.describe('Feature Name', () => {
  let testData: TestDataManager;

  test.beforeEach(async ({ request, page }, testInfo) => {
    testData = new TestDataManager(request, testInfo.title);
    await page.goto('/feature-path');
  });

  test.afterEach(async () => {
    await testData.cleanup(); // Guaranteed cleanup
  });

  test('should perform specific action', async ({ page, authenticatedUser }) => {
    await test.step('User action', async () => {
      // Use authenticatedUser fixture for API calls
      await page.goto('/feature');
      await page.getByRole('button', { name: 'Action' }).click();
    });

    await test.step('Verify result', async () => {
      await expect(page.getByText('Success')).toBeVisible();
    });
  });
});
```

#### 1.2 Core Authentication & Navigation Tests
**Priority:** Critical
**Status:** ✅ Complete (with known issues tracked)

**Delivered Test Files:**

**`tests/core/authentication.spec.ts`** - 16 tests (13 passing, 3 failing - tracked)
- ✅ Login with valid credentials (covered by auth.setup.ts)
- ✅ Login with invalid credentials
- ✅ Logout functionality
- ✅ Session persistence
- ⚠️ Session expiration handling (3 tests failing - see [Issue: e2e-session-expiration-tests](../issues/e2e-session-expiration-tests.md))
- ✅ Password reset flow (if implemented)

**`tests/core/dashboard.spec.ts`** - All tests passing
- ✅ Dashboard loads successfully
- ✅ Summary cards display correct data
- ✅ Quick action buttons are functional
- ✅ Recent activity shows latest changes
- ✅ System status indicators work

**`tests/core/navigation.spec.ts`** - All tests passing
- ✅ All main menu items are clickable
- ✅ Sidebar navigation works
- ✅ Breadcrumbs display correctly
- ✅ Deep links resolve properly
- ✅ Back button navigation works

**Known Issues:**
- 3 session expiration tests require route mocking - tracked in [docs/issues/e2e-session-expiration-tests.md](../issues/e2e-session-expiration-tests.md)

**Acceptance Criteria:** ✅ Met (with known exceptions)
- All authentication flows covered (session expiration deferred)
- Dashboard displays without errors
- Navigation between all pages works
- No console errors during navigation
- Keyboard navigation fully functional

### Phase 2: Critical Path (Week 4-5)

**Goal:** Cover the most critical user journeys

#### 2.1 Proxy Hosts Management
**Priority:** Critical
**Estimated Effort:** 4 days

**Test Files:**

**`tests/proxy/proxy-hosts-crud.spec.ts`**
Test Scenarios:
- ✅ List all proxy hosts (empty state)
- ✅ Create new proxy host with basic configuration
  - Enter domain name (e.g., `test-app.example.com`)
  - Enter forward hostname (e.g., `192.168.1.100`)
  - Enter forward port (e.g., `3000`)
  - Select scheme (HTTP/HTTPS)
  - Enable/disable WebSocket support
  - Save and verify host appears in list
- ✅ View proxy host details
- ✅ Edit existing proxy host
  - Update domain name
  - Update forward hostname/port
  - Toggle WebSocket support
  - Save and verify changes
- ✅ Delete proxy host
  - Delete single host
  - Verify deletion confirmation dialog
  - Verify host removed from list
- ✅ Bulk operations (if supported)

**Key User Flows:**
1. **Create Basic Proxy Host:**
   ```
   Navigate → Click "Add Proxy Host" → Fill form → Save → Verify in list
   ```

2. **Edit Existing Host:**
   ```
   Navigate → Select host → Click edit → Modify → Save → Verify changes
   ```

3. **Delete Host:**
   ```
   Navigate → Select host → Click delete → Confirm → Verify removal
   ```

**Critical Assertions:**
- Host appears in list after creation
- Edit changes are persisted
- Deletion removes host from database
- Validation prevents invalid data
- Success/error messages display correctly

#### 2.2 SSL Certificates Management
**Priority:** Critical
**Estimated Effort:** 4 days

**Test Files:**

**`tests/certificates/certificates-list.spec.ts`**
Test Scenarios:
- ✅ List all certificates (empty state)
- ✅ Display certificate details (domain, expiry, issuer)
- ✅ Filter certificates by status (valid, expiring, expired)
- ✅ Sort certificates by expiry date
- ✅ Search certificates by domain name
- ✅ Show certificate chain details

**`tests/certificates/certificates-upload.spec.ts`**
Test Scenarios:
- ✅ Upload custom certificate with private key
- ✅ Validate PEM format
- ✅ Reject invalid certificate formats
- ✅ Reject mismatched certificate and key
- ✅ Support intermediate certificate chains
- ✅ Update existing certificate
- ✅ Delete custom certificate

**`tests/certificates/certificates-acme.spec.ts`**
Test Scenarios:

**ACME HTTP-01 Challenge:**
- ✅ Request certificate via HTTP-01 challenge
  - Select domain from proxy hosts
  - Choose HTTP-01 validation method
  - Verify challenge file is served at `/.well-known/acme-challenge/`
  - Mock ACME server validates challenge
  - Certificate issued and stored
- ✅ HTTP-01 challenge fails if proxy host not accessible
- ✅ HTTP-01 challenge fails with invalid domain

**ACME DNS-01 Challenge:**
- ✅ Request certificate via DNS-01 challenge
  - Select DNS provider (Cloudflare, Route53, Manual)
  - Mock DNS provider API for TXT record creation
  - Verify TXT record `_acme-challenge.domain.com` created
  - Mock ACME server validates DNS record
  - Certificate issued and stored
- ✅ DNS-01 challenge supports wildcard certificates
  - Request `*.example.com` certificate
  - Verify TXT record for `_acme-challenge.example.com`
  - Certificate covers all subdomains
- ✅ DNS-01 challenge fails with invalid DNS credentials
- ✅ DNS-01 challenge retries on DNS propagation delay

**Certificate Renewal:**
- ✅ Automatic renewal triggered 30 days before expiry
  - Mock certificate with expiry in 29 days
  - Verify renewal task scheduled
  - Renewal completes successfully
  - Old certificate archived
- ✅ Manual certificate renewal
  - Click "Renew Now" button
  - Renewal process uses same validation method
  - New certificate replaces old
- ✅ Renewal fails gracefully
  - Old certificate remains active
  - Error notification displayed
  - Retry mechanism available

**Wildcard Certificates:**
- ✅ Request wildcard certificate (`*.example.com`)
  - DNS-01 challenge required (HTTP-01 not supported)
  - Verify TXT record created
  - Certificate issued with wildcard SAN
- ✅ Wildcard certificate applies to all subdomains
  - Create proxy host `app.example.com`
  - Wildcard certificate auto-selected
  - HTTPS works for any subdomain

**Certificate Revocation:**
- ✅ Revoke Let's Encrypt certificate
  - Click "Revoke" button
  - Confirm revocation reason
  - Certificate marked as revoked
  - ACME server notified
- ✅ Revoked certificate cannot be used
  - Proxy hosts using certificate show warning
  - HTTPS connections fail

**Validation Error Handling:**
- ✅ ACME account registration fails
  - Invalid email address
  - Rate limit exceeded
  - Network error during registration
- ✅ Challenge validation fails
  - HTTP-01: Challenge file not accessible
  - DNS-01: TXT record not found
  - DNS-01: DNS propagation timeout
- ✅ Certificate issuance fails
  - ACME server error
  - Domain validation failed
  - Rate limit exceeded

**Mixed Certificate Sources:**
- ✅ Use Let's Encrypt and custom certificates together
  - Some domains use Let's Encrypt
  - Some domains use custom certificates
  - Certificates don't conflict
- ✅ Migrate from custom to Let's Encrypt
  - Replace custom certificate with Let's Encrypt
  - No downtime during migration
  - Old certificate archived

**Certificate Metadata:**
- ✅ Display certificate information
  - Issuer, subject, validity period
  - SAN (Subject Alternative Names)
  - Signature algorithm
  - Certificate chain
- ✅ Export certificate in various formats
  - PEM, DER, PFX
  - With or without private key
  - Include full chain

**Key User Flows:**
1. **HTTP-01 Challenge Flow:**
   ```
   Navigate → Click "Request Certificate" → Select domain → Choose HTTP-01 →
   Monitor challenge → Certificate issued → Verify in list
   ```

2. **DNS-01 Wildcard Flow:**
   ```
   Navigate → Click "Request Certificate" → Enter *.example.com → Choose DNS-01 →
   Select DNS provider → Monitor DNS propagation → Certificate issued → Verify wildcard works
   ```

3. **Certificate Renewal Flow:**
   ```
   Navigate → Select expiring certificate → Click "Renew" →
   Automatic challenge re-validation → New certificate issued → Old certificate archived
   ```

**Critical Assertions:**
- Challenge files/records created correctly
- ACME server validates challenges
- Certificates issued with correct domains
- Renewal happens before expiry
- Validation errors display helpful messages
- Certificate chain is complete and valid

#### 2.3 Access Lists (ACL)
**Priority:** Critical
**Estimated Effort:** 3 days

**Test Files:**

**`tests/access-lists/access-lists-crud.spec.ts`**
Test Scenarios:
- ✅ List all access lists (empty state)
  - Verify empty state message displayed
  - "Create Access List" CTA visible
- ✅ Create IP whitelist (Allow Only)
  - Enter name (e.g., "Office IPs")
  - Add description
  - Select type: IP Whitelist
  - Add IP rules (single IP, CIDR ranges)
  - Save and verify appears in list
- ✅ Create IP blacklist (Block Only)
  - Select type: IP Blacklist
  - Add blocked IPs/ranges
  - Verify badge shows "Deny"
- ✅ Create geo-whitelist
  - Select type: Geo Whitelist
  - Select allowed countries (US, CA, GB)
  - Verify country badges displayed
- ✅ Create geo-blacklist
  - Select type: Geo Blacklist
  - Block high-risk countries
  - Apply security presets
- ✅ Enable/disable access list
  - Toggle enabled state
  - Verify badge shows correct status
- ✅ Edit access list
  - Update name, description, rules
  - Add/remove IP ranges
  - Change type (whitelist ↔ blacklist)
- ✅ Delete access list
  - Confirm backup creation before delete
  - Verify removed from list
  - Verify proxy hosts unaffected

**`tests/access-lists/access-lists-rules.spec.ts`**
Test Scenarios:
- ✅ Add single IP address
  - Enter `192.168.1.100`
  - Add optional description
  - Verify appears in rules list
- ✅ Add CIDR range
  - Enter `10.0.0.0/24`
  - Verify covers 256 IPs
  - Display IP count badge
- ✅ Add multiple rules
  - Add 5+ IP rules
  - Verify all rules displayed
  - Support pagination/scrolling
- ✅ Remove individual rule
  - Click delete on specific rule
  - Verify removed from list
  - Other rules unaffected
- ✅ RFC1918 Local Network Only
  - Toggle "Local Network Only" switch
  - Verify IP rules section hidden
  - Description shows "RFC1918 ranges only"
- ✅ Invalid IP validation
  - Enter invalid IP (e.g., `999.999.999.999`)
  - Verify error message displayed
  - Form not submitted
- ✅ Invalid CIDR validation
  - Enter invalid CIDR (e.g., `192.168.1.0/99`)
  - Verify error message displayed
- ✅ Get My IP feature
  - Click "Get My IP" button
  - Verify current IP populated in field
  - Toast shows IP source

**`tests/access-lists/access-lists-geo.spec.ts`**
Test Scenarios:
- ✅ Select single country
  - Click country in dropdown
  - Country badge appears
- ✅ Select multiple countries
  - Add US, CA, GB
  - All badges displayed
  - Deselect removes badge
- ✅ Country search/filter
  - Type "united" in search
  - Shows United States, United Kingdom, UAE
  - Select from filtered list
- ✅ Geo-whitelist vs geo-blacklist behavior
  - Whitelist: only selected countries allowed
  - Blacklist: selected countries blocked

**`tests/access-lists/access-lists-presets.spec.ts`**
Test Scenarios:
- ✅ Show security presets (blacklist only)
  - Presets section hidden for whitelists
  - Presets section visible for blacklists
- ✅ Apply "Known Malicious Actors" preset
  - Click "Apply" on preset
  - IP rules populated
  - Toast shows rules added count
- ✅ Apply "High-Risk Countries" preset
  - Apply geo-blacklist preset
  - Countries auto-selected
  - Can add additional countries
- ✅ Preset warning displayed
  - Shows data source and update frequency
  - Warning for aggressive presets

**`tests/access-lists/access-lists-test-ip.spec.ts`**
Test Scenarios:
- ✅ Open Test IP dialog
  - Click test tube icon on ACL row
  - Dialog opens with IP input
- ✅ Test allowed IP
  - Enter IP that should be allowed
  - Click "Test"
  - Success toast: "✅ IP Allowed: [reason]"
- ✅ Test blocked IP
  - Enter IP that should be blocked
  - Click "Test"
  - Error toast: "🚫 IP Blocked: [reason]"
- ✅ Invalid IP test
  - Enter invalid IP
  - Error toast displayed
- ✅ Test RFC1918 detection
  - Test with private IP (192.168.x.x)
  - Verify local network detection
- ✅ Test IPv6 address
  - Enter IPv6 address
  - Verify correct allow/block decision

**`tests/access-lists/access-lists-integration.spec.ts`**
Test Scenarios:
- ✅ Assign ACL to proxy host
  - Edit proxy host
  - Select ACL from dropdown
  - Save and verify assignment
- ✅ ACL selector shows only enabled lists
  - Disabled ACLs hidden from dropdown
  - Enabled ACLs visible with type badge
- ✅ Bulk update ACL on multiple hosts
  - Select multiple hosts
  - Click "Update ACL" bulk action
  - Select ACL from modal
  - Verify all hosts updated
- ✅ Remove ACL from proxy host
  - Select "No Access Control (Public)"
  - Verify ACL unassigned
- ✅ Delete ACL in use
  - Attempt delete of assigned ACL
  - Warning shows affected hosts
  - Confirm or cancel

**Key UI Selectors:**
```typescript
// AccessLists.tsx page selectors
'button >> text=Create Access List'      // Create button
'[role="table"]'                         // ACL list table
'[role="row"]'                           // Individual ACL rows
'button >> text=Edit'                    // Edit action (row)
'button >> text=Delete'                  // Delete action (row)
'button[title*="Test IP"]'               // Test IP button (TestTube2 icon)

// AccessListForm.tsx selectors
'input#name'                             // Name input
'textarea#description'                   // Description input
'select#type'                            // Type dropdown (whitelist/blacklist/geo)
'[data-state="checked"]'                 // Enabled toggle (checked)
'button >> text=Get My IP'               // Get current IP
'button >> text=Add'                     // Add IP rule
'input[placeholder*="192.168"]'          // IP input field

// AccessListSelector.tsx selectors
'select >> text=Access Control List'     // ACL selector in ProxyHostForm
'option >> text=No Access Control'       // Public option
```

**API Endpoints:**
```typescript
// Access Lists CRUD
GET    /api/v1/access-lists           // List all
GET    /api/v1/access-lists/:id       // Get single
POST   /api/v1/access-lists           // Create
PUT    /api/v1/access-lists/:id       // Update
DELETE /api/v1/access-lists/:id       // Delete
POST   /api/v1/access-lists/:id/test  // Test IP against ACL
GET    /api/v1/access-lists/templates // Get presets

// Proxy Host ACL Integration
PUT    /api/v1/proxy-hosts/bulk-update-acl  // Bulk ACL update
```

**Critical Assertions:**
- ACL appears in list after creation
- IP rules correctly parsed and displayed
- Type badges match ACL configuration
- Test IP returns accurate allow/block decisions
- ACL assignment persists on proxy hosts
- Validation prevents invalid CIDR/IP input
- Security presets apply correctly

---

## Phase 2 Implementation Plan (Detailed)

**Timeline:** Week 4-5 (2 weeks)
**Total Tests Estimated:** 95-105 tests
**Based on Phase 1 Velocity:** 112 tests in ~1 week = ~16 tests/day

### Week 4: Proxy Hosts & Access Lists (Days 1-5)

#### Day 1-2: Proxy Hosts CRUD (30-35 tests)

**File: `tests/proxy/proxy-hosts-crud.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | displays empty state when no hosts exist | `[data-testid="empty-state"]`, `text=Add Proxy Host` | `GET /proxy-hosts` | P0 |
| 2 | shows skeleton loading while fetching | `[data-testid="skeleton-table"]` | `GET /proxy-hosts` | P1 |
| 3 | lists all proxy hosts in table | `role=table`, `role=row` | `GET /proxy-hosts` | P0 |
| 4 | displays host details (domain, forward, ssl) | `role=cell` | - | P0 |
| 5 | opens create form when Add clicked | `button >> text=Add Proxy Host`, `role=dialog` | - | P0 |
| 6 | creates basic HTTP proxy host | `#proxy-name`, `#domain-names`, `#forward-host`, `#forward-port` | `POST /proxy-hosts` | P0 |
| 7 | creates HTTPS proxy host with SSL | `[name="ssl_forced"]` | `POST /proxy-hosts` | P0 |
| 8 | creates proxy with WebSocket support | `[name="allow_websocket_upgrade"]` | `POST /proxy-hosts` | P1 |
| 9 | creates proxy with HTTP/2 support | `[name="http2_support"]` | `POST /proxy-hosts` | P1 |
| 10 | shows Docker containers in dropdown | `button >> text=Docker Discovery` | `GET /docker/containers` | P1 |
| 11 | auto-fills from Docker container | Docker container option | - | P1 |
| 12 | validates empty domain name | `#domain-names:invalid` | - | P0 |
| 13 | validates invalid domain format | Error toast | - | P0 |
| 14 | validates empty forward host | `#forward-host:invalid` | - | P0 |
| 15 | validates invalid forward port | `#forward-port:invalid` | - | P0 |
| 16 | validates port out of range (0, 65536) | Error message | - | P1 |
| 17 | rejects XSS in domain name | 422 response | `POST /proxy-hosts` | P0 |
| 18 | rejects SQL injection in fields | 422 response | `POST /proxy-hosts` | P0 |
| 19 | opens edit form for existing host | `button[aria-label="Edit"]` | `GET /proxy-hosts/:uuid` | P0 |
| 20 | updates domain name | Form submission | `PUT /proxy-hosts/:uuid` | P0 |
| 21 | updates forward host and port | Form submission | `PUT /proxy-hosts/:uuid` | P0 |
| 22 | toggles host enabled/disabled | `role=switch` | `PUT /proxy-hosts/:uuid` | P0 |
| 23 | assigns SSL certificate | Certificate selector | `PUT /proxy-hosts/:uuid` | P1 |
| 24 | assigns access list | ACL selector | `PUT /proxy-hosts/:uuid` | P1 |
| 25 | shows delete confirmation dialog | `role=alertdialog` | - | P0 |
| 26 | deletes single host | Confirm button | `DELETE /proxy-hosts/:uuid` | P0 |
| 27 | cancels delete operation | Cancel button | - | P1 |
| 28 | shows success toast after CRUD | `role=alert` | - | P0 |
| 29 | shows error toast on failure | `role=alert[data-type="error"]` | - | P0 |
| 30 | navigates back to list after save | URL check | - | P1 |

**File: `tests/proxy/proxy-hosts-bulk.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 31 | selects single host via checkbox | `role=checkbox` | - | P0 |
| 32 | selects all hosts via header checkbox | Header checkbox | - | P0 |
| 33 | shows bulk actions when selected | Bulk action buttons | - | P0 |
| 34 | bulk updates ACL on multiple hosts | `button >> text=Update ACL` | `PUT /proxy-hosts/bulk-update-acl` | P0 |
| 35 | bulk deletes multiple hosts | `button >> text=Delete` | Multiple `DELETE` | P1 |
| 36 | bulk updates security headers | Security headers modal | `PUT /proxy-hosts/bulk-update-security-headers` | P1 |
| 37 | clears selection after bulk action | Checkbox states | - | P1 |

**File: `tests/proxy/proxy-hosts-search-filter.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 38 | filters hosts by domain search | Search input | - | P1 |
| 39 | filters by enabled/disabled status | Status filter | - | P1 |
| 40 | filters by SSL status | SSL filter | - | P2 |
| 41 | sorts by domain name | Column header click | - | P2 |
| 42 | sorts by creation date | Column header click | - | P2 |
| 43 | paginates large host lists | Pagination controls | `GET /proxy-hosts?page=2` | P2 |

#### Day 3: Access Lists CRUD (20-25 tests)

**File: `tests/access-lists/access-lists-crud.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | displays empty state when no ACLs | `[data-testid="empty-state"]` | `GET /access-lists` | P0 |
| 2 | lists all access lists in table | `role=table` | `GET /access-lists` | P0 |
| 3 | shows ACL type badge (Allow/Deny) | `Badge[variant="success"]` | - | P0 |
| 4 | creates IP whitelist | `select#type`, `option[value="whitelist"]` | `POST /access-lists` | P0 |
| 5 | creates IP blacklist | `option[value="blacklist"]` | `POST /access-lists` | P0 |
| 6 | creates geo-whitelist | `option[value="geo_whitelist"]` | `POST /access-lists` | P0 |
| 7 | creates geo-blacklist | `option[value="geo_blacklist"]` | `POST /access-lists` | P0 |
| 8 | validates empty name | `input#name:invalid` | - | P0 |
| 9 | adds single IP rule | IP input, Add button | - | P0 |
| 10 | adds CIDR range rule | `10.0.0.0/24` input | - | P0 |
| 11 | shows IP count for CIDR | IP count badge | - | P1 |
| 12 | removes IP rule | Delete button on rule | - | P0 |
| 13 | validates invalid CIDR | Error message | - | P0 |
| 14 | enables RFC1918 local network only | Toggle switch | - | P1 |
| 15 | Get My IP populates field | `button >> text=Get My IP` | `GET /system/my-ip` | P1 |
| 16 | edits existing ACL | Edit button, form | `PUT /access-lists/:id` | P0 |
| 17 | deletes ACL with backup | Delete, confirm | `DELETE /access-lists/:id` | P0 |
| 18 | toggles ACL enabled/disabled | Enable switch | `PUT /access-lists/:id` | P0 |
| 19 | shows CGNAT warning on first load | Alert component | - | P2 |
| 20 | dismisses CGNAT warning | Dismiss button | - | P2 |

**File: `tests/access-lists/access-lists-geo.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 21 | selects country from list | Country dropdown | - | P0 |
| 22 | adds multiple countries | Country badges | - | P0 |
| 23 | removes country | Badge X button | - | P0 |
| 24 | shows all 40+ countries | Country list | - | P1 |

**File: `tests/access-lists/access-lists-test.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 25 | opens Test IP dialog | TestTube2 icon button | - | P0 |
| 26 | tests allowed IP shows success | Success toast | `POST /access-lists/:id/test` | P0 |
| 27 | tests blocked IP shows error | Error toast | `POST /access-lists/:id/test` | P0 |
| 28 | validates invalid IP input | Error message | - | P1 |

#### Day 4-5: Access Lists Integration & Presets (10-15 tests)

**File: `tests/access-lists/access-lists-presets.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | shows presets section for blacklist | Presets toggle | - | P1 |
| 2 | hides presets for whitelist | - | - | P1 |
| 3 | applies security preset | Apply button | - | P1 |
| 4 | shows preset warning | Warning icon | - | P2 |
| 5 | shows data source link | External link | - | P2 |

**File: `tests/access-lists/access-lists-integration.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 6 | assigns ACL to proxy host | ACL selector | `PUT /proxy-hosts/:uuid` | P0 |
| 7 | shows only enabled ACLs in selector | Dropdown options | `GET /access-lists` | P0 |
| 8 | bulk assigns ACL to hosts | Bulk ACL modal | `PUT /proxy-hosts/bulk-update-acl` | P0 |
| 9 | removes ACL from proxy host | "No Access Control" | `PUT /proxy-hosts/:uuid` | P0 |
| 10 | warns when deleting ACL in use | Warning dialog | - | P1 |

### Week 5: SSL Certificates (Days 6-10)

#### Day 6-7: Certificate List & Upload (25-30 tests)

**File: `tests/certificates/certificates-list.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | displays empty state when no certs | Empty state | `GET /certificates` | P0 |
| 2 | lists all certificates | Table rows | `GET /certificates` | P0 |
| 3 | shows certificate details | Name, domain, expiry | - | P0 |
| 4 | shows status badge (valid) | `Badge[variant="success"]` | - | P0 |
| 5 | shows status badge (expiring) | `Badge[variant="warning"]` | - | P0 |
| 6 | shows status badge (expired) | `Badge[variant="error"]` | - | P0 |
| 7 | sorts by name column | Header click | - | P1 |
| 8 | sorts by expiry date | Header click | - | P1 |
| 9 | shows associated proxy hosts | Host count/badges | - | P2 |

**File: `tests/certificates/certificates-upload.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 10 | opens upload modal | `button >> text=Add Certificate` | - | P0 |
| 11 | uploads valid cert and key | File inputs | `POST /certificates` (multipart) | P0 |
| 12 | validates PEM format | Error on invalid | - | P0 |
| 13 | rejects mismatched cert/key | Error toast | - | P0 |
| 14 | rejects expired certificate | Error toast | - | P1 |
| 15 | shows upload progress | Progress indicator | - | P2 |
| 16 | closes modal after success | Modal hidden | - | P1 |
| 17 | shows success toast | `role=alert` | - | P0 |
| 18 | deletes certificate | Delete button | `DELETE /certificates/:id` | P0 |
| 19 | shows delete confirmation | Confirm dialog | - | P0 |
| 20 | creates backup before delete | Backup API | `POST /backups` | P1 |

**File: `tests/certificates/certificates-validation.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 21 | rejects empty name | Validation error | - | P0 |
| 22 | rejects missing cert file | Required error | - | P0 |
| 23 | rejects missing key file | Required error | - | P0 |
| 24 | rejects self-signed (if configured) | Warning/Error | - | P2 |
| 25 | handles network error gracefully | Error toast | - | P1 |

#### Day 8-9: ACME Certificates (15-20 tests)

**File: `tests/certificates/certificates-acme.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | shows ACME certificate info | Let's Encrypt badge | - | P0 |
| 2 | displays HTTP-01 challenge type | Challenge type indicator | - | P1 |
| 3 | displays DNS-01 challenge type | Challenge type indicator | - | P1 |
| 4 | shows certificate renewal date | Expiry countdown | - | P0 |
| 5 | shows "Renew Now" for expiring | Renew button visible | - | P1 |
| 6 | hides "Renew Now" for valid | Renew button hidden | - | P1 |
| 7 | displays wildcard indicator | Wildcard badge | - | P1 |
| 8 | shows SAN (multiple domains) | Domain list | - | P2 |

**Note:** Full ACME flow testing requires mocked ACME server (staging.letsencrypt.org) - these tests verify UI behavior with pre-existing ACME certificates.

**File: `tests/certificates/certificates-status.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 9 | dashboard shows certificate stats | CertificateStatusCard | - | P1 |
| 10 | shows valid certificate count | Valid count badge | - | P1 |
| 11 | shows expiring certificate count | Warning count | - | P1 |
| 12 | shows pending certificate count | Pending count | - | P2 |
| 13 | links to certificates page | Card link | - | P2 |
| 14 | progress bar shows coverage | Progress component | - | P2 |

#### Day 10: Certificate Integration & Cleanup (10-15 tests)

**File: `tests/certificates/certificates-integration.spec.ts`**

| # | Test Name | UI Selectors | API Endpoint | Priority |
|---|-----------|-------------|--------------|----------|
| 1 | assigns certificate to proxy host | Certificate selector | `PUT /proxy-hosts/:uuid` | P0 |
| 2 | shows only valid certs in selector | Dropdown filtered | `GET /certificates` | P0 |
| 3 | certificate cleanup dialog on host delete | CertificateCleanupDialog | - | P0 |
| 4 | deletes orphan certs option | Checkbox in dialog | - | P1 |
| 5 | keeps certs option | Default unchecked | - | P1 |
| 6 | shows affected hosts on cert delete | Host list | - | P1 |
| 7 | warns about hosts using certificate | Warning message | - | P1 |

---

### Fixtures Reference

**Proxy Hosts:** `tests/fixtures/proxy-hosts.ts`
- `basicProxyHost` - HTTP proxy to internal service
- `proxyHostWithSSL` - HTTPS with forced SSL
- `proxyHostWithWebSocket` - WebSocket enabled
- `proxyHostFullSecurity` - All security features
- `wildcardProxyHost` - Wildcard domain
- `dockerProxyHost` - From Docker discovery
- `invalidProxyHosts` - Validation test cases (XSS, SQL injection)

**Access Lists:** `tests/fixtures/access-lists.ts`
- `emptyAccessList` - No rules
- `allowOnlyAccessList` - IP whitelist
- `denyOnlyAccessList` - IP blacklist
- `mixedRulesAccessList` - Multiple IP ranges
- `authEnabledAccessList` - With HTTP basic auth
- `ipv6AccessList` - IPv6 ranges
- `invalidACLConfigs` - Validation test cases

**Certificates:** `tests/fixtures/certificates.ts`
- `letsEncryptCertificate` - HTTP-01 ACME
- `multiDomainLetsEncrypt` - SAN certificate
- `wildcardCertificate` - DNS-01 wildcard
- `customCertificateMock` - Uploaded PEM
- `expiredCertificate` - For error testing
- `expiringCertificate` - 25 days to expiry
- `invalidCertificates` - Validation test cases

---

### Acceptance Criteria for Phase 2

**Proxy Hosts (40 tests minimum):**
- [ ] All CRUD operations covered
- [ ] Bulk operations functional
- [ ] Docker discovery integration works
- [ ] Validation prevents all invalid input
- [ ] XSS/SQL injection rejected

**SSL Certificates (30 tests minimum):**
- [ ] List/upload/delete operations covered
- [ ] PEM validation enforced
- [ ] Certificate status displayed correctly
- [ ] Dashboard stats accurate
- [ ] Cleanup dialog handles orphan certs

**Access Lists (25 tests minimum):**
- [ ] All 4 ACL types covered (IP/Geo × Allow/Block)
- [ ] IP/CIDR rule management works
- [ ] Country selection works
- [ ] Test IP feature functional
- [ ] Integration with proxy hosts works

**Overall:**
- [ ] 95+ tests passing
- [ ] <5% flaky test rate
- [ ] All P0 tests complete
- [ ] 90%+ P1 tests complete
- [ ] No hardcoded waits
- [ ] All tests use TestDataManager for cleanup

---

### Phase 3: Security Features (Week 6-7)

**Goal:** Cover all Cerberus security features

#### 3.1 CrowdSec Integration
**Priority:** High
**Estimated Effort:** 4 days

**Test Files:**

**`tests/security/crowdsec-startup.spec.ts`**
**Objective:** Verify CrowdSec container lifecycle and connectivity

Test Scenarios:
- ✅ CrowdSec container starts successfully
  - Verify container health check passes
  - Verify LAPI (Local API) is accessible
  - Verify logs show successful initialization
- ✅ CrowdSec LAPI connection from Charon
  - Backend connects to CrowdSec LAPI
  - Authentication succeeds
  - API health endpoint returns 200
- ✅ CrowdSec bouncer registration
  - Charon registers as a bouncer
  - Bouncer API key generated
  - Bouncer appears in CrowdSec bouncer list
- ✅ CrowdSec graceful shutdown
  - Stop CrowdSec container
  - Charon handles disconnection gracefully
  - No errors in Charon logs
  - Restart CrowdSec, Charon reconnects
- ✅ CrowdSec container restart recovery
  - Kill CrowdSec container abruptly
  - Charon detects connection loss
  - Auto-reconnect after CrowdSec restarts
- ✅ CrowdSec version compatibility
  - Verify minimum version check
  - Warn if CrowdSec version too old
  - Block connection if incompatible

**Key Assertions:**
- Container health: `docker inspect crowdsec --format '{{.State.Health.Status}}' == 'healthy'`
- LAPI reachable: `curl http://crowdsec:8080/health` returns 200
- Bouncer registered: API call to `/v1/bouncers` shows Charon
- Logs clean: No error/warning logs after startup

**`tests/security/crowdsec-decisions.spec.ts`**
**Objective:** Test IP ban management and decision enforcement

Test Scenarios:

**Manual IP Ban:**
- ✅ Add IP ban via Charon UI
  - Navigate to CrowdSec → Decisions
  - Click "Add Decision"
  - Enter IP address (e.g., `192.168.1.100`)
  - Select ban duration (1h, 4h, 24h, permanent)
  - Select scope (IP, range, country)
  - Add ban reason (e.g., "Suspicious activity")
  - Save decision
- ✅ Verify decision appears in CrowdSec
  - Query CrowdSec LAPI `/v1/decisions`
  - Verify decision contains correct IP, duration, reason
- ✅ Banned IP cannot access proxy hosts
  - Make HTTP request from banned IP
  - Verify 403 Forbidden response
  - Verify block logged in audit log

**Automatic IP Ban (Scenario-based):**
- ✅ Trigger ban via brute force scenario
  - Simulate 10 failed login attempts from IP
  - CrowdSec scenario detects pattern
  - Decision created automatically
  - IP banned for configured duration
- ✅ Trigger ban via HTTP flood scenario
  - Send 100 requests/second from IP
  - CrowdSec detects flood pattern
  - IP banned automatically

**Ban Duration & Expiration:**
- ✅ Temporary ban expires automatically
  - Create 5-second ban
  - Verify IP blocked immediately
  - Wait 6 seconds
  - Verify IP can access again
- ✅ Permanent ban persists
  - Create permanent ban
  - Restart Charon and CrowdSec
  - Verify ban still active
- ✅ Manual ban removal
  - Select active ban
  - Click "Remove Decision"
  - Confirm removal
  - Verify IP can access immediately

**Ban Scope Testing:**
- ✅ Single IP ban: `192.168.1.100`
  - Only that IP blocked
  - `192.168.1.101` can access
- ✅ IP range ban: `192.168.1.0/24`
  - All IPs in range blocked
  - `192.168.2.1` can access
- ✅ Country-level ban: `CN` (China)
  - All Chinese IPs blocked
  - Uses GeoIP database
  - Other countries can access

**Decision Priority & Conflicts:**
- ✅ Allow decision overrides ban decision
  - Ban IP range `10.0.0.0/8`
  - Allow specific IP `10.0.0.5`
  - Verify `10.0.0.5` can access
  - Verify `10.0.0.6` is blocked
- ✅ Multiple decisions for same IP
  - Add 1-hour ban
  - Add 24-hour ban
  - Longer duration takes precedence

**Decision Metadata:**
- ✅ Display decision details
  - IP/Range/Country
  - Ban duration and expiry time
  - Scenario that triggered ban
  - Reason/origin
  - Creation timestamp
- ✅ Decision history
  - View past decisions (expired/deleted)
  - Filter by IP, scenario, date range
  - Export decision history

**Key User Flows:**
1. **Manual Ban Flow:**
   ```
   CrowdSec page → Decisions tab → Add Decision →
   Enter IP → Select duration → Save → Verify block
   ```

2. **Automatic Ban Flow:**
   ```
   Trigger scenario (e.g., brute force) →
   CrowdSec detects → Decision created →
   View in Decisions tab → Verify block
   ```

3. **Ban Removal Flow:**
   ```
   CrowdSec page → Decisions tab → Select decision →
   Remove → Confirm → Verify access restored
   ```

**Critical Assertions:**
- Banned IPs receive 403 status code
- Decision sync between Charon and CrowdSec
- Expiration timing accurate (±5 seconds)
- Allow decisions override ban decisions
- Decision changes appear in audit log

**`tests/security/crowdsec-presets.spec.ts`**
**Objective:** Test CrowdSec scenario preset management

Test Scenarios:

**Preset Listing:**
- ✅ View available presets
  - Navigate to CrowdSec → Presets
  - Display preset categories (Web, SSH, System)
  - Show preset descriptions
  - Indicate enabled/disabled status

**Enable/Disable Presets:**
- ✅ Enable web attack preset
  - Select "Web Attacks" preset
  - Click "Enable"
  - Verify scenarios installed in CrowdSec
  - Verify collection appears in CrowdSec collections list
- ✅ Disable web attack preset
  - Select enabled preset
  - Click "Disable"
  - Verify scenarios removed
  - Existing decisions preserved
- ✅ Bulk enable multiple presets
  - Select multiple presets
  - Click "Enable Selected"
  - All scenarios installed

**Custom Scenarios:**
- ✅ Create custom scenario
  - Click "Add Custom Scenario"
  - Enter scenario name (e.g., "api-abuse")
  - Define pattern (e.g., 50 requests to /api in 10s)
  - Set ban duration (e.g., 1h)
  - Save scenario
  - Verify scenario YAML created
- ✅ Test custom scenario
  - Trigger scenario conditions
  - Verify decision created
  - Verify ban enforced
- ✅ Edit custom scenario
  - Modify pattern thresholds
  - Save changes
  - Reload CrowdSec scenarios
- ✅ Delete custom scenario
  - Select scenario
  - Confirm deletion
  - Scenario removed from CrowdSec

**Preset Configuration:**
- ✅ Configure scenario thresholds
  - Select scenario (e.g., "http-bf" brute force)
  - Modify threshold (e.g., 5 → 10 failed attempts)
  - Modify time window (e.g., 30s → 60s)
  - Save configuration
  - Verify new thresholds apply
- ✅ Configure ban duration per scenario
  - Different scenarios have different ban times
  - Brute force: 4h ban
  - Port scan: 24h ban
  - Verify durations respected

**Scenario Testing & Validation:**
- ✅ Test scenario before enabling
  - View scenario details
  - Simulate trigger conditions in test mode
  - Verify pattern matching works
  - No actual bans created (dry-run)
- ✅ Scenario validation on save
  - Invalid regex pattern rejected
  - Impossible thresholds rejected (e.g., 0 requests)
  - Missing required fields flagged

**Key User Flows:**
1. **Enable Preset Flow:**
   ```
   CrowdSec page → Presets tab → Select preset →
   Enable → Verify scenarios active
   ```

2. **Custom Scenario Flow:**
   ```
   CrowdSec page → Presets tab → Add Custom →
   Define pattern → Set duration → Save → Test trigger
   ```

3. **Configure Scenario Flow:**
   ```
   CrowdSec page → Presets tab → Select scenario →
   Edit thresholds → Save → Reload CrowdSec
   ```

**Critical Assertions:**
- Enabled scenarios appear in CrowdSec collections
- Scenario triggers create correct decisions
- Custom scenarios persist after restart
- Threshold changes take effect immediately
- Invalid scenarios rejected with clear error messages

#### 3.2 Coraza WAF (Web Application Firewall)
**Priority:** High
**Estimated Effort:** 3 days

**Test Files:**

**`tests/security/waf-config.spec.ts`**
Test Scenarios:
- ✅ Enable/disable WAF globally
- ✅ Configure WAF for specific proxy host
- ✅ Set WAF rule sets (OWASP Core Rule Set)
- ✅ Configure anomaly scoring thresholds
- ✅ Set blocking/logging mode
- ✅ Custom rule creation
- ✅ Rule exclusions (false positive handling)

**`tests/security/waf-blocking.spec.ts`**
Test Scenarios:
- ✅ Block SQL injection attempts
  - Send request with `' OR 1=1--` in query
  - Verify 403 response
  - Verify attack logged
- ✅ Block XSS attempts
  - Send request with `<script>alert('xss')</script>`
  - Verify 403 response
- ✅ Block path traversal attempts
  - Send request with `../../etc/passwd`
  - Verify 403 response
- ✅ Block command injection attempts
- ✅ Block file upload attacks
- ✅ Allow legitimate requests
  - Normal user traffic passes through
  - No false positives

**Key Assertions:**
- Malicious requests blocked (403 status)
- Legitimate requests allowed (200/3xx status)
- Attacks logged in audit log with details
- WAF performance overhead <50ms

#### 3.3 Rate Limiting
**Priority:** High
**Estimated Effort:** 2 days

### Phase 4: Settings (Week 8)

**Goal:** Cover system configuration and user management

**Estimated Effort:** 5 days

**Test Files:**
- `tests/settings/system-settings.spec.ts` - System configuration
- `tests/settings/smtp-settings.spec.ts` - Email configuration
- `tests/settings/notifications.spec.ts` - Notification rules
- `tests/settings/user-management.spec.ts` - User CRUD and roles
- `tests/settings/encryption-management.spec.ts` - Encryption key rotation
- `tests/settings/account-settings.spec.ts` - User profile management

**Key Features:**
- System configuration (timezone, language, theme)
- Email settings (SMTP, templates)
- Notification rules (email, webhook)
- User management (CRUD, roles, permissions)
- Encryption management (key rotation, backup)
- Account settings (profile, password, 2FA)

### Phase 5: Tasks & Monitoring (Week 9)

**Status:** 🔄 IN PROGRESS
**Detailed Plan:** [phase5-implementation.md](phase5-implementation.md)

**Goal:** Cover backup, logs, import, and monitoring features

**Estimated Effort:** 5 days
**Total Estimated Tests:** 92-114 (updated per Supervisor review)

> **Supervisor Approved:** Plan reviewed and approved with 3 recommendations incorporated:
> - ✅ Added backup download test (P1) to section 5.1
> - ✅ Added import session timeout tests (P2) to section 5.4
> - ✅ Added WebSocket reconnection mock utility note to section 5.7

**Directory Structure:**
```
tests/
├── tasks/
│   ├── backups-create.spec.ts      # Backup creation workflows
│   ├── backups-restore.spec.ts     # Backup restoration workflows
│   ├── logs-viewing.spec.ts        # Log viewer functionality
│   ├── import-caddyfile.spec.ts    # Caddyfile import wizard
│   └── import-crowdsec.spec.ts     # CrowdSec config import
└── monitoring/
    ├── uptime-monitoring.spec.ts   # Uptime monitor CRUD
    └── real-time-logs.spec.ts      # WebSocket log streaming
```

---

#### 5.1 Backups - Create (`tests/tasks/backups-create.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/backups` | `Backups.tsx` | `GET /api/v1/backups`, `POST /api/v1/backups`, `DELETE /api/v1/backups/:filename` |

**Test Scenarios (12-15 tests):**

**Page Layout & Navigation:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should display backups page with correct heading and navigation | P0 |
| 2 | should show Create Backup button for admin users | P0 |
| 3 | should hide Create Backup button for guest users | P1 |

**Backup List Display:**
| # | Test Name | Priority |
|---|-----------|----------|
| 4 | should display empty state when no backups exist | P0 |
| 5 | should display list of existing backups with filename, size, and timestamp | P0 |
| 6 | should sort backups by date (newest first) | P1 |
| 7 | should show loading skeleton while fetching backups | P2 |

**Create Backup Flow:**
| # | Test Name | Priority |
|---|-----------|----------|
| 8 | should create a new backup successfully | P0 |
| 9 | should show success toast after backup creation | P0 |
| 10 | should update backup list with new backup | P0 |
| 11 | should disable create button while backup is in progress | P1 |
| 12 | should handle backup creation failure gracefully | P1 |

**Delete Backup Flow:**
| # | Test Name | Priority |
|---|-----------|----------|
| 13 | should show confirmation dialog before deleting | P0 |
| 14 | should delete backup after confirmation | P0 |
| 15 | should show success toast after deletion | P1 |

**Download Backup Flow:**
| # | Test Name | Priority |
|---|-----------|----------|
| 16 | should download backup file successfully | P0 |
| 17 | should show error toast when download fails | P1 |

> **Supervisor Note (P1):** Explicit backup download test added per review - verifies the `/api/v1/backups/:filename/download` endpoint functions correctly.

**API Endpoints:**
```typescript
GET  /api/v1/backups                    // List backups
POST /api/v1/backups                    // Create backup
DELETE /api/v1/backups/:filename        // Delete backup
GET  /api/v1/backups/:filename/download // Download backup
```

---

#### 5.2 Backups - Restore (`tests/tasks/backups-restore.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/backups` | `Backups.tsx` | `POST /api/v1/backups/:filename/restore` |

**Test Scenarios (6-8 tests):**

**Restore Flow:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should show warning dialog before restore | P0 |
| 2 | should require explicit confirmation for restore action | P0 |
| 3 | should restore backup successfully | P0 |
| 4 | should show success toast after restoration | P0 |
| 5 | should show progress indicator during restore | P1 |
| 6 | should handle restore failure gracefully | P1 |

**Post-Restore Verification:**
| # | Test Name | Priority |
|---|-----------|----------|
| 7 | should reload application state after restore | P1 |
| 8 | should preserve user session after restore | P2 |

**API Endpoints:**
```typescript
POST /api/v1/backups/:filename/restore  // Restore from backup
```

**Mock Data Requirements:**
- Valid backup file for restoration testing
- Corrupt/invalid backup file for error handling

---

#### 5.3 Log Viewer (`tests/tasks/logs-viewing.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/logs` | `Logs.tsx`, `LogTable.tsx`, `LogFilters.tsx` | `GET /api/v1/logs`, `GET /api/v1/logs/:filename` |

**Test Scenarios (15-18 tests):**

**Page Layout:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should display logs page with file selector | P0 |
| 2 | should show list of available log files | P0 |
| 3 | should display log filters (search, level, host, status) | P0 |

**Log File Selection:**
| # | Test Name | Priority |
|---|-----------|----------|
| 4 | should list all available log files | P0 |
| 5 | should display file size and modification time | P1 |
| 6 | should load log content when file is selected | P0 |
| 7 | should show empty state for empty log files | P1 |

**Log Content Display:**
| # | Test Name | Priority |
|---|-----------|----------|
| 8 | should display log entries in table format | P0 |
| 9 | should show timestamp, level, message, and request details | P0 |
| 10 | should paginate large log files | P1 |
| 11 | should sort logs by timestamp | P1 |
| 12 | should highlight error and warning entries | P2 |

**Log Filtering:**
| # | Test Name | Priority |
|---|-----------|----------|
| 13 | should filter logs by search text | P0 |
| 14 | should filter logs by log level | P0 |
| 15 | should filter logs by host | P1 |
| 16 | should filter logs by status code range | P1 |
| 17 | should combine multiple filters | P1 |
| 18 | should clear all filters | P1 |

**API Endpoints:**
```typescript
GET /api/v1/logs                       // List log files
GET /api/v1/logs/:filename             // Read log file with filters
GET /api/v1/logs/:filename/download    // Download log file
```

**Log Entry Interface:**
```typescript
interface CaddyAccessLog {
  level: string;
  ts: number;
  logger: string;
  msg: string;
  request: {
    remote_ip: string;
    method: string;
    host: string;
    uri: string;
    proto: string;
  };
  status: number;
  duration: number;
  size: number;
}
```

---

#### 5.4 Caddyfile Import (`tests/tasks/import-caddyfile.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/import/caddyfile` | `ImportCaddy.tsx`, `ImportReviewTable.tsx`, `ImportSitesModal.tsx` | `POST /api/v1/import/upload`, `GET /api/v1/import/preview`, `POST /api/v1/import/commit` |

**Test Scenarios (14-16 tests):**

**Upload Interface:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should display file upload dropzone | P0 |
| 2 | should accept valid Caddyfile | P0 |
| 3 | should reject invalid file types | P0 |
| 4 | should show upload progress | P1 |
| 5 | should handle multi-file upload | P1 |
| 6 | should detect import directives in Caddyfile | P1 |

**Preview & Review:**
| # | Test Name | Priority |
|---|-----------|----------|
| 7 | should show parsed hosts from Caddyfile | P0 |
| 8 | should display host configuration details | P0 |
| 9 | should allow selection/deselection of hosts | P0 |
| 10 | should show validation warnings for problematic configs | P1 |
| 11 | should highlight conflicts with existing hosts | P1 |

**Commit Import:**
| # | Test Name | Priority |
|---|-----------|----------|
| 12 | should commit selected hosts | P0 |
| 13 | should skip deselected hosts | P1 |
| 14 | should show success toast after import | P0 |
| 15 | should navigate to proxy hosts after import | P1 |
| 16 | should handle partial import failures | P1 |

**Session Management:**
| # | Test Name | Priority |
|---|-----------|----------|
| 17 | should handle import session timeout/expiry | P2 |
| 18 | should show warning when session is about to expire | P2 |

> **Supervisor Note (P2):** Session timeout tests added per review - import sessions have server-side TTL and should gracefully handle expiration.

**API Endpoints:**
```typescript
POST   /api/v1/import/upload         // Upload Caddyfile
POST   /api/v1/import/upload-multi   // Upload multiple files
GET    /api/v1/import/status         // Get import session status
GET    /api/v1/import/preview        // Get parsed hosts preview
POST   /api/v1/import/detect-imports // Detect import directives
POST   /api/v1/import/commit         // Commit import
DELETE /api/v1/import/cancel         // Cancel import session
```

---

#### 5.5 CrowdSec Import (`tests/tasks/import-crowdsec.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/import/crowdsec` | `ImportCrowdSec.tsx` | `POST /api/v1/crowdsec/import` |

**Test Scenarios (6-8 tests):**

**Upload Interface:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should display file upload interface | P0 |
| 2 | should accept YAML configuration files | P0 |
| 3 | should reject invalid file types | P0 |
| 4 | should create backup before import | P0 |

**Import Flow:**
| # | Test Name | Priority |
|---|-----------|----------|
| 5 | should import CrowdSec configuration | P0 |
| 6 | should show success toast after import | P0 |
| 7 | should validate configuration format | P1 |
| 8 | should handle import errors gracefully | P1 |

**Component Behavior (from `ImportCrowdSec.tsx`):**
```typescript
// Import triggers backup creation first
const backupResult = await createBackup();
// Then imports CrowdSec config
await importCrowdsecConfig(file);
```

---

#### 5.6 Uptime Monitoring (`tests/monitoring/uptime-monitoring.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/uptime` | `Uptime.tsx`, `UptimeWidget.tsx` | `GET /api/v1/uptime/monitors`, `POST /api/v1/uptime/monitors`, `PUT /api/v1/uptime/monitors/:id` |

**Test Scenarios (18-22 tests):**

**Page Layout:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should display uptime monitoring page | P0 |
| 2 | should show monitor list or empty state | P0 |
| 3 | should display overall uptime summary | P1 |

**Monitor List Display:**
| # | Test Name | Priority |
|---|-----------|----------|
| 4 | should display all monitors with status indicators | P0 |
| 5 | should show uptime percentage for each monitor | P0 |
| 6 | should show last check timestamp | P1 |
| 7 | should differentiate between up/down/unknown states | P0 |
| 8 | should group monitors by category if configured | P2 |

**Monitor CRUD:**
| # | Test Name | Priority |
|---|-----------|----------|
| 9 | should create new HTTP monitor | P0 |
| 10 | should create new TCP monitor | P1 |
| 11 | should update existing monitor | P0 |
| 12 | should delete monitor with confirmation | P0 |
| 13 | should validate monitor URL format | P0 |
| 14 | should validate check interval | P1 |

**Manual Check:**
| # | Test Name | Priority |
|---|-----------|----------|
| 15 | should trigger manual health check | P0 |
| 16 | should update status after manual check | P0 |
| 17 | should show check in progress indicator | P1 |

**Monitor History:**
| # | Test Name | Priority |
|---|-----------|----------|
| 18 | should display uptime history chart | P1 |
| 19 | should show incident timeline | P2 |
| 20 | should filter history by date range | P2 |

**Sync with Proxy Hosts:**
| # | Test Name | Priority |
|---|-----------|----------|
| 21 | should sync monitors from proxy hosts | P1 |
| 22 | should preserve manually added monitors | P1 |

**API Endpoints:**
```typescript
GET    /api/v1/uptime/monitors           // List monitors
POST   /api/v1/uptime/monitors           // Create monitor
PUT    /api/v1/uptime/monitors/:id       // Update monitor
DELETE /api/v1/uptime/monitors/:id       // Delete monitor
GET    /api/v1/uptime/monitors/:id/history // Get history
POST   /api/v1/uptime/monitors/:id/check // Trigger check
POST   /api/v1/uptime/sync               // Sync with proxy hosts
```

---

#### 5.7 Real-time Logs (`tests/monitoring/real-time-logs.spec.ts`)

**Routes & Components:**

| Route | Component | API Endpoints |
|-------|-----------|---------------|
| `/tasks/logs` (Live tab) | `LiveLogViewer.tsx` | `WS /api/v1/logs/live`, `WS /api/v1/cerberus/logs/ws` |

**Test Scenarios (16-20 tests):**

**WebSocket Connection:**
| # | Test Name | Priority |
|---|-----------|----------|
| 1 | should establish WebSocket connection | P0 |
| 2 | should show connected status indicator | P0 |
| 3 | should handle connection failure gracefully | P0 |
| 4 | should auto-reconnect on connection loss | P1 |
| 5 | should authenticate via HttpOnly cookies | P1 |
| 6 | should recover from network interruption | P1 |

> **Supervisor Note:** Add `simulateNetworkInterruption()` utility to `tests/utils/wait-helpers.ts` for testing WebSocket reconnection scenarios. This mock should temporarily close the WebSocket and verify the component reconnects automatically.

**Log Streaming:**
| # | Test Name | Priority |
|---|-----------|----------|
| 6 | should display incoming log entries in real-time | P0 |
| 7 | should auto-scroll to latest logs | P1 |
| 8 | should respect max log limit (500 entries) | P1 |
| 9 | should format timestamps correctly | P1 |
| 10 | should colorize log levels appropriately | P2 |

**Mode Switching:**
| # | Test Name | Priority |
|---|-----------|----------|
| 11 | should toggle between Application and Security modes | P0 |
| 12 | should clear logs when switching modes | P1 |
| 13 | should reconnect to correct WebSocket endpoint | P0 |

**Live Filters:**
| # | Test Name | Priority |
|---|-----------|----------|
| 14 | should filter by text search | P0 |
| 15 | should filter by log level | P0 |
| 16 | should filter by source (security mode) | P1 |
| 17 | should filter blocked requests only (security mode) | P1 |

**Playback Controls:**
| # | Test Name | Priority |
|---|-----------|----------|
| 18 | should pause log streaming | P0 |
| 19 | should resume log streaming | P0 |
| 20 | should clear all logs | P1 |

**WebSocket Interfaces:**
```typescript
// Application logs
interface LiveLogEntry {
  level: string;
  timestamp: string;
  message: string;
  source?: string;
  data?: Record<string, unknown>;
}

// Security logs (Cerberus)
interface SecurityLogEntry {
  timestamp: string;
  level: string;
  logger: string;
  client_ip: string;
  method: string;
  uri: string;
  status: number;
  duration: number;
  size: number;
  user_agent: string;
  host: string;
  source: 'waf' | 'crowdsec' | 'ratelimit' | 'acl' | 'normal';
  blocked: boolean;
  block_reason?: string;
  details?: Record<string, unknown>;
}
```

**WebSocket Testing Strategy:**
```typescript
// Use Playwright's WebSocket interception
test('should display incoming log entries in real-time', async ({ page }) => {
  await page.goto('/tasks/logs');

  // Wait for WebSocket connection
  await waitForWebSocketConnection(page);

  // Verify connection indicator shows "Connected"
  await expect(page.locator('[data-testid="connection-status"]'))
    .toContainText('Connected');

  // Intercept WebSocket messages
  page.on('websocket', ws => {
    ws.on('framereceived', event => {
      const log = JSON.parse(event.payload);
      // Verify log entry structure
      expect(log).toHaveProperty('timestamp');
      expect(log).toHaveProperty('level');
    });
  });
});
```

---

#### Phase 5 Implementation Priority

| Priority | Test File | Reason | Est. Tests |
|----------|-----------|--------|------------|
| 1 | `backups-create.spec.ts` | Core data protection feature | 12-15 |
| 2 | `backups-restore.spec.ts` | Critical recovery workflow | 6-8 |
| 3 | `logs-viewing.spec.ts` | Essential debugging tool | 15-18 |
| 4 | `uptime-monitoring.spec.ts` | Key operational feature | 18-22 |
| 5 | `real-time-logs.spec.ts` | WebSocket testing complexity | 16-20 |
| 6 | `import-caddyfile.spec.ts` | Multi-step wizard | 14-16 |
| 7 | `import-crowdsec.spec.ts` | Simpler import flow | 6-8 |
| **Total** | | | **87-107** |

---

#### Phase 5 Test Utilities

**Wait Helpers (from `tests/utils/wait-helpers.ts`):**
```typescript
// Key utilities to use:
await waitForToast(page, /success|created|deleted/i);
await waitForLoadingComplete(page);
await waitForAPIResponse(page, '/api/v1/backups', 200);
await waitForWebSocketConnection(page);
await waitForWebSocketMessage(page, (msg) => msg.level === 'error');
await waitForTableLoad(page, locator);
await retryAction(page, async () => { /* action */ }, { maxAttempts: 3 });
```

**Test Data Manager (from `tests/utils/TestDataManager.ts`):**
```typescript
// For creating test data with automatic cleanup:
const manager = new TestDataManager(page, 'backups-test');
const host = await manager.createProxyHost({ domain: 'test.example.com' });
// ... test
await manager.cleanup(); // Auto-cleanup in reverse order
```

**Authentication (from `tests/fixtures/auth-fixtures.ts`):**
```typescript
// Use admin fixture for full access:
test.use({ ...adminUser });

// Or regular user for permission testing:
test.use({ ...regularUser });

// Or guest for read-only testing:
test.use({ ...guestUser });
```

---

#### Phase 5 Acceptance Criteria

**Backups (18-23 tests minimum):**
- [ ] All CRUD operations covered
- [ ] Restore workflow with confirmation
- [ ] Download functionality works
- [ ] Error handling for failures
- [ ] Role-based access verified

**Logs (31-38 tests minimum):**
- [ ] Static log viewing works
- [ ] All filters functional
- [ ] WebSocket streaming works
- [ ] Mode switching (App/Security)
- [ ] Pause/Resume controls

**Imports (20-24 tests minimum):**
- [ ] File upload works
- [ ] Preview shows parsed data
- [ ] Commit creates resources
- [ ] Error handling for invalid files

**Uptime (18-22 tests minimum):**
- [ ] Monitor CRUD operations
- [ ] Status indicators correct
- [ ] Manual check works
- [ ] Sync with proxy hosts

**Overall Phase 5:**
- [ ] 87+ tests passing
- [ ] <5% flaky test rate
- [ ] All P0 tests complete
- [ ] 90%+ P1 tests complete
- [ ] No hardcoded waits (use wait-helpers)
- [ ] All tests use TestDataManager for cleanup

### Phase 6: Integration Testing (Week 10)

**Status:** 📋 PLANNED
**Goal:** Verify cross-feature interactions, system-level workflows, and end-to-end data integrity
**Estimated Effort:** 5 days (3 days integration tests + 2 days buffer/stabilization)
**Total Estimated Tests:** 85-105 tests

> **Planning Note:** Integration tests verify that multiple features work correctly together.
> Unlike unit or feature tests that isolate functionality, integration tests exercise
> realistic user workflows that span multiple components and data relationships.

> **Prerequisites (Supervisor Requirement):**
> - ✅ Phase 5 complete with Backup/Restore and Import tests passing
> - ✅ All Phase 7 remediation fixes applied (toast detection, API path corrections)
> - ✅ CI pipeline stable with <5% flaky test rate
> - ✅ All API endpoints verified against actual backend routes (see API Path Verification below)

---

#### 6.0 Phase 6 Overview & Objectives

**Primary Objectives:**
1. **Cross-Feature Validation:** Verify that interconnected features (Proxy + ACL + Certificate + Security) function correctly when combined
2. **Data Integrity Verification:** Ensure backup/restore preserves all data relationships and configurations
3. **Security Stack Integration:** Validate the complete Cerberus security suite working as a unified system
4. **Real-World Workflow Testing:** Test complex user journeys that span multiple features
5. **System Resilience:** Verify graceful handling of edge cases, failures, and recovery scenarios

**API Path Verification (Supervisor Requirement):**

> ⚠️ **CRITICAL:** Before implementing any Phase 6 test, cross-reference all API endpoints against actual backend routes.
> Phase 7 documented API path mismatches (`/api/v1/crowdsec/import` vs `/api/v1/admin/crowdsec/import`).
> Tests may fail due to undocumented API path changes.

| Endpoint Category | Verification File | Status |
|-------------------|-------------------|--------|
| Access Lists | `backend/api/access_list_handler.go` | ⏳ Pending |
| Certificates | `backend/api/certificate_handler.go` | ⏳ Pending |
| Security/Cerberus | `backend/api/cerberus_handler.go` | ⏳ Pending |
| Backups | `backend/api/backup_handler.go` | ⏳ Pending |
| CrowdSec | `backend/api/crowdsec_handler.go` | ⏳ Pending |

**Directory Structure:**
```
tests/
└── integration/
    ├── proxy-acl-integration.spec.ts       # Proxy + ACL integration
    ├── proxy-certificate.spec.ts           # Proxy + SSL certificate integration
    ├── proxy-dns-integration.spec.ts       # Proxy + DNS challenge integration
    ├── security-suite-integration.spec.ts  # Full security stack (WAF + CrowdSec + Rate Limiting)
    ├── backup-restore-e2e.spec.ts          # Complete backup/restore cycle with verification
    ├── import-to-production.spec.ts        # Import → Configure → Deploy workflows
    └── multi-feature-workflows.spec.ts     # Complex real-world scenarios
```

**Feature Dependency Map:**

```
┌─────────────────────────────────────────────────────────────────┐
│                          ProxyHost                               │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────────┐ │
│  │ CertificateID│  │ AccessListID │  │ SecurityHeaderProfileID │ │
│  └──────┬──────┘  └──────┬───────┘  └────────────┬────────────┘ │
│         │                │                       │               │
│         ▼                ▼                       ▼               │
│  SSLCertificate     AccessList          SecurityHeaderProfile   │
│         │                │                       │               │
│         │                │                       │               │
│         ▼                ▼                       ▼               │
│    DNSProvider      GeoIP Rules            WAF Integration      │
│         │                │                       │               │
│         └────────────────┴───────────┬───────────┘               │
│                                      │                           │
│                                      ▼                           │
│                              Cerberus Security                   │
│                    ┌─────────────────────────────┐               │
│                    │  CrowdSec  │  WAF  │  Rate  │               │
│                    └─────────────────────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

---

#### 6.1 Proxy + Access List Integration (`tests/integration/proxy-acl-integration.spec.ts`)

**Objective:** Verify that Access Lists correctly protect Proxy Hosts and that ACL changes propagate immediately.

**Routes & Components:**

| Route | Components | API Endpoints |
|-------|------------|---------------|
| `/proxy-hosts/:uuid/edit` | `ProxyHostForm.tsx`, `AccessListSelector.tsx` | `PUT /api/v1/proxy-hosts/:uuid` |
| `/access-lists` | `AccessLists.tsx`, `AccessListForm.tsx` | `GET/POST/PUT/DELETE /api/v1/access-lists` |
| `/access-lists/:id/test` | `TestIPDialog.tsx` | `POST /api/v1/access-lists/:id/test` |

**Test Scenarios (18-22 tests):**

**Scenario Group A: Basic ACL Assignment**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should assign IP whitelist to proxy host | P0 | Create ACL with allowed IPs → Assign to proxy host → Verify configuration saved |
| 2 | should assign IP blacklist to proxy host | P0 | Create ACL with blocked IPs → Assign to proxy host → Verify configuration saved |
| 3 | should assign geo-whitelist to proxy host | P1 | Create geo ACL (US, CA, GB) → Assign to proxy host → Verify country rules applied |
| 4 | should assign geo-blacklist to proxy host | P1 | Create geo ACL blocking countries → Assign to proxy host → Verify blocking |
| 5 | should unassign ACL from proxy host | P0 | Remove ACL from proxy host → Verify "No Access Control" state |

**Scenario Group B: ACL Rule Enforcement**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 6 | should block request from denied IP | P0 | Assign blacklist ACL → Test request from blocked IP → Verify 403 response |
| 7 | should allow request from whitelisted IP | P0 | Assign whitelist ACL → Test request from allowed IP → Verify 200 response |
| 8 | should block request from non-whitelisted IP | P0 | Assign whitelist ACL → Test request from unlisted IP → Verify 403 response |
| 9 | should enforce CIDR range correctly | P1 | Add CIDR range to ACL → Test IPs within and outside range → Verify enforcement |
| 10 | should enforce RFC1918 local network only | P1 | Enable local network only → Test private/public IPs → Verify enforcement |

**Scenario Group C: Dynamic ACL Updates**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 11 | should apply ACL changes immediately | P0 | Update ACL rules → Test access instantly → Verify new rules active |
| 12 | should disable ACL without deleting | P1 | Disable ACL → Verify proxy host accessible to all → Re-enable → Verify blocking |
| 13 | should handle ACL deletion with active assignments | P0 | Delete ACL with assigned hosts → Verify warning shown → Verify hosts become public |
| 14 | should bulk update ACL on multiple hosts | P1 | Select 3+ hosts → Bulk assign ACL → Verify all hosts protected |

**Scenario Group D: Edge Cases & Error Handling**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 15 | should handle IPv6 addresses correctly | P2 | Add IPv6 to ACL → Test IPv6 request → Verify correct allow/block |
| 16 | should preserve ACL on proxy host update | P0 | Edit proxy host (change domain) → Verify ACL still assigned |
| 17 | should handle conflicting ACL rules gracefully | P2 | Create overlapping IP/CIDR rules → Verify deterministic behavior |
| 18 | should log ACL enforcement in audit log | P1 | Trigger ACL block → Verify audit entry created with details |

**Key User Flow:**
```typescript
test('complete ACL protection workflow', async ({ page, testData }) => {
  await test.step('Create proxy host', async () => {
    const host = await testData.createProxyHost({
      domain: 'protected-app.example.com',
      forwardHost: '192.168.1.100',
      forwardPort: 8080
    });
  });

  await test.step('Create IP whitelist ACL', async () => {
    const acl = await testData.createAccessList({
      name: 'Office IPs Only',
      type: 'whitelist',
      rules: [
        { type: 'allow', value: '10.0.0.0/8' },
        { type: 'allow', value: '192.168.1.0/24' }
      ]
    });
  });

  await test.step('Assign ACL to proxy host', async () => {
    await page.goto('/proxy-hosts');
    await page.getByRole('row', { name: /protected-app/ }).getByRole('button', { name: /edit/i }).click();
    await page.getByLabel('Access Control').selectOption({ label: /Office IPs Only/ });
    await page.getByRole('button', { name: /save/i }).click();
    await waitForToast(page, /updated|saved/i);
  });

  await test.step('Verify ACL protection active', async () => {
    // Via API test endpoint
    const testResponse = await page.request.post('/api/v1/access-lists/:id/test', {
      data: { ip: '8.8.8.8' } // External IP
    });
    expect(testResponse.status()).toBe(200);
    const result = await testResponse.json();
    expect(result.allowed).toBe(false);
    expect(result.reason).toMatch(/not in whitelist/i);
  });
});
```

**Critical Assertions:**
- ACL assignment persists after page reload
- ACL rules enforce immediately without restart
- Correct HTTP status codes returned (200 for allowed, 403 for blocked)
- Audit log entries created for ACL enforcement events
- Bulk operations apply consistently to all selected hosts

---

#### 6.2 Proxy + SSL Certificate Integration (`tests/integration/proxy-certificate.spec.ts`)

**Objective:** Verify SSL certificate assignment to proxy hosts and HTTPS enforcement.

**Routes & Components:**

| Route | Components | API Endpoints |
|-------|------------|---------------|
| `/proxy-hosts/:uuid/edit` | `ProxyHostForm.tsx`, `CertificateSelector.tsx` | `PUT /api/v1/proxy-hosts/:uuid` |
| `/certificates` | `Certificates.tsx`, `CertificateForm.tsx` | `GET/POST/DELETE /api/v1/certificates` |
| `/certificates/:id` | `CertificateDetails.tsx` | `GET /api/v1/certificates/:id` |

**Test Scenarios (15-18 tests):**

**Scenario Group A: Certificate Assignment**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should assign custom certificate to proxy host | P0 | Upload cert → Assign to host → Verify HTTPS configuration |
| 2 | should assign Let's Encrypt certificate | P1 | Request ACME cert → Assign to host → Verify auto-renewal configured |
| 3 | should assign wildcard certificate to multiple hosts | P0 | Create *.example.com cert → Assign to subdomain hosts → Verify all work |
| 4 | should show only matching certificates in selector | P1 | Create certs for different domains → Verify selector filters correctly |
| 5 | should remove certificate from proxy host | P0 | Unassign cert → Verify HTTP-only mode |

**Scenario Group B: HTTPS Enforcement**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 6 | should enforce SSL redirect when enabled | P0 | Enable SSL forced → Access via HTTP → Verify 301 redirect to HTTPS |
| 7 | should serve HTTP when SSL not forced | P1 | Disable SSL forced → Access via HTTP → Verify 200 response |
| 8 | should enable HSTS when configured | P1 | Enable HSTS → Verify Strict-Transport-Security header |
| 9 | should include subdomains in HSTS when enabled | P2 | Enable HSTS subdomains → Verify header includes subdomain directive |
| 10 | should enable HTTP/2 with certificate | P1 | Assign cert with HTTP/2 enabled → Verify protocol negotiation |

**Scenario Group C: Certificate Lifecycle**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 11 | should warn when certificate expires soon | P0 | Create cert expiring in 25 days → Verify warning badge on proxy host |
| 12 | should prevent deletion of certificate in use | P0 | Attempt delete cert with assigned hosts → Verify warning with host list |
| 13 | should offer cleanup options on host deletion | P1 | Delete host with orphan cert → Verify cleanup dialog appears |
| 14 | should update certificate without downtime | P1 | Replace cert on active host → Verify no request failures during switch |

**Scenario Group D: Multi-Domain & SAN Certificates**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 15 | should support SAN certificates for multiple domains | P1 | Create SAN cert → Assign to host with multiple domain names → Verify all domains work |
| 16 | should validate certificate matches domain names | P0 | Assign mismatched cert → Verify validation error shown |
| 17 | should prefer specific cert over wildcard | P2 | Create specific and wildcard certs → Verify specific cert selected first |

**Key User Flow:**
```typescript
test('complete HTTPS setup workflow', async ({ page, testData }) => {
  await test.step('Create proxy host', async () => {
    const host = await testData.createProxyHost({
      domain: 'secure-app.example.com',
      forwardHost: '192.168.1.100',
      forwardPort: 8080,
      sslForced: true,
      http2Support: true
    });
  });

  await test.step('Upload custom certificate', async () => {
    const cert = await testData.createCertificate({
      domains: ['secure-app.example.com'],
      type: 'custom',
      privateKey: MOCK_PRIVATE_KEY,
      certificate: MOCK_CERTIFICATE
    });
  });

  await test.step('Assign certificate to proxy host', async () => {
    await page.goto('/proxy-hosts');
    await page.getByRole('row', { name: /secure-app/ }).getByRole('button', { name: /edit/i }).click();
    await page.getByLabel('SSL Certificate').selectOption({ label: /secure-app/ });
    await page.getByRole('button', { name: /save/i }).click();
    await waitForToast(page, /updated|saved/i);
  });

  await test.step('Verify HTTPS enforcement', async () => {
    // Verify SSL redirect configured
    await page.goto('/proxy-hosts');
    const row = page.getByRole('row', { name: /secure-app/ });
    await expect(row.getByTestId('ssl-badge')).toContainText(/HTTPS/i);
  });
});
```

---

#### 6.3 Proxy + DNS Challenge Integration (`tests/integration/proxy-dns-integration.spec.ts`)

**Objective:** Verify DNS-01 challenge configuration for SSL certificates with DNS providers.

**Test Scenarios (10-12 tests):**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should configure proxy host with DNS challenge | P0 | Create host → Assign DNS provider → Enable DNS challenge → Verify config |
| 2 | should request wildcard certificate via DNS-01 | P1 | Enable DNS challenge → Request *.domain.com → Verify challenge type |
| 3 | should propagate DNS provider credentials to Caddy | P1 | Configure DNS provider → Verify Caddy config includes provider module |
| 4 | should fall back to HTTP-01 when DNS not configured | P1 | Create host without DNS provider → Request cert → Verify HTTP-01 used |
| 5 | should validate DNS provider before certificate request | P0 | Configure invalid DNS credentials → Attempt cert → Verify clear error |
| 6 | should use correct DNS provider for multi-domain cert | P2 | Different domains with different DNS providers → Verify correct provider used |
| 7 | should handle DNS propagation timeout gracefully | P2 | Mock slow DNS propagation → Verify retry mechanism |
| 8 | should preserve DNS config on proxy host update | P1 | Edit host domain → Verify DNS challenge config preserved |

---

#### 6.4 Security Suite Integration (`tests/integration/security-suite-integration.spec.ts`)

**Objective:** Verify the complete Cerberus security stack (WAF + CrowdSec + Rate Limiting + ACL) working together.

**Routes & Components:**

| Route | Components | API Endpoints |
|-------|------------|---------------|
| `/security` | `SecurityDashboard.tsx` | `GET /api/v1/cerberus/status` |
| `/security/crowdsec` | `CrowdSecConfig.tsx`, `CrowdSecDecisions.tsx` | `GET/POST /api/v1/crowdsec/*` |
| `/security/waf` | `WAFConfig.tsx` | `GET/PUT /api/v1/cerberus/waf` |
| `/security/rate-limiting` | `RateLimitConfig.tsx` | `GET/PUT /api/v1/cerberus/ratelimit` |

**Test Scenarios (20-25 tests):**

**Scenario Group A: Security Stack Initialization**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should display unified security dashboard | P0 | Navigate to /security → Verify all security components shown |
| 2 | should show status of all security features | P0 | Verify CrowdSec, WAF, Rate Limiting status indicators |
| 3 | should enable all security features together | P1 | Enable CrowdSec + WAF + Rate Limiting → Verify all active |
| 4 | should disable individual features independently | P1 | Disable WAF only → Verify CrowdSec and Rate Limiting still active |

**Scenario Group B: Multi-Layer Attack Prevention**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 5 | should block SQL injection at WAF layer | P0 | Send SQLi payload → Verify blocked by WAF → Verify logged |
| 6 | should block XSS at WAF layer | P0 | Send XSS payload → Verify blocked by WAF → Verify logged |
| 7 | should rate limit after threshold exceeded | P0 | Send 50+ requests rapidly → Verify rate limit triggered |
| 8 | should ban IP via CrowdSec after repeated attacks | P1 | Trigger WAF blocks → Verify CrowdSec decision created |
| 9 | should allow legitimate traffic through all layers | P0 | Send normal requests → Verify 200 response through full stack |

**Scenario Group C: Security Rule Precedence**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 10 | should apply ACL before WAF inspection | P1 | Block IP via ACL → Send attack payload → Verify ACL blocks first |
| 11 | should apply WAF before rate limiting | P1 | Verify attack blocked before rate limit counter increments |
| 12 | should apply CrowdSec decisions globally | P0 | Ban IP in CrowdSec → Verify blocked on all proxy hosts |
| 13 | should allow CrowdSec allow-list to override bans | P1 | Add IP to allow decision → Verify access despite previous ban |

**Scenario Group D: Security Logging & Audit**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 14 | should log all security events to security log | P0 | Trigger various security events → Verify all appear in /security/logs |
| 15 | should include attack details in security log | P1 | Trigger WAF block → Verify log contains rule ID, payload snippet |
| 16 | should include source IP and user agent | P0 | Trigger security event → Verify client details logged |
| 17 | should stream security events via WebSocket | P1 | Open live log viewer → Trigger event → Verify real-time display |

**Scenario Group D.1: WebSocket Stability (Supervisor Recommendation)**

> **Note:** Added per Supervisor review - WebSocket real-time features are a known flaky area.
> These tests ensure robust WebSocket handling in security log streaming.

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 17a | should reconnect WebSocket after network interruption | P1 | Simulate network drop → Verify auto-reconnect → Verify no event loss |
| 17b | should maintain event ordering under rapid-fire events | P1 | Send 50+ security events rapidly → Verify correct chronological order |
| 17c | should handle WebSocket connection timeout gracefully | P2 | Mock slow connection → Verify timeout message → Verify retry mechanism |

**Scenario Group E: Security Configuration Persistence**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 18 | should persist WAF configuration after restart | P1 | Configure WAF → Restart app → Verify settings preserved |
| 19 | should persist CrowdSec decisions after restart | P0 | Create ban decision → Restart → Verify decision still active |
| 20 | should persist rate limit configuration | P1 | Configure rate limits → Restart → Verify limits active |

**Scenario Group F: Per-Host Security Overrides**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 21 | should allow WAF disable per proxy host | P1 | Enable global WAF → Disable for specific host → Verify host unprotected |
| 22 | should apply host-specific rate limits | P2 | Set global rate limit → Override for specific host → Verify override |
| 23 | should combine host ACL with global CrowdSec | P1 | Assign ACL to host → Verify both ACL and CrowdSec enforce |

**Key Integration Flow:**
```typescript
test('complete security stack protection', async ({ page, testData }) => {
  await test.step('Create protected proxy host', async () => {
    const host = await testData.createProxyHost({
      domain: 'secure-app.example.com',
      forwardHost: '192.168.1.100',
      forwardPort: 8080
    });
  });

  await test.step('Enable all security features', async () => {
    await page.goto('/security');

    // Enable WAF
    await page.getByRole('switch', { name: /waf/i }).click();
    await waitForToast(page, /waf enabled/i);

    // Enable Rate Limiting
    await page.getByRole('switch', { name: /rate limit/i }).click();
    await waitForToast(page, /rate limiting enabled/i);

    // Verify CrowdSec connected
    await expect(page.getByTestId('crowdsec-status')).toContainText(/connected/i);
  });

  await test.step('Test WAF blocks SQL injection', async () => {
    // Attempt SQL injection
    const response = await page.request.get(
      'https://secure-app.example.com/search?q=\' OR 1=1--'
    );
    expect(response.status()).toBe(403);
  });

  await test.step('Verify security event logged', async () => {
    await page.goto('/security/logs');
    await expect(page.getByRole('row').first()).toContainText(/sql injection/i);
  });

  await test.step('Verify CrowdSec decision created after repeated attacks', async () => {
    // Trigger multiple WAF blocks
    for (let i = 0; i < 5; i++) {
      await page.request.get('https://secure-app.example.com/admin?cmd=whoami');
    }

    await page.goto('/security/crowdsec/decisions');
    await expect(page.getByRole('table')).toContainText(/automatic ban/i);
  });
});
```

---

#### 6.5 Backup & Restore E2E (`tests/integration/backup-restore-e2e.spec.ts`)

**Objective:** Verify complete backup/restore cycle with full data integrity verification.

**Routes & Components:**

| Route | Components | API Endpoints |
|-------|------------|---------------|
| `/tasks/backups` | `Backups.tsx` | `GET/POST/DELETE /api/v1/backups`, `POST /api/v1/backups/:filename/restore` |

**Test Scenarios (18-22 tests):**

**Scenario Group A: Complete Data Backup**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should create backup containing all proxy hosts | P0 | Create hosts → Backup → Verify hosts in backup manifest |
| 2 | should include certificates in backup | P0 | Create certs → Backup → Verify certs archived |
| 3 | should include access lists in backup | P0 | Create ACLs → Backup → Verify ACLs in backup |
| 4 | should include DNS providers in backup | P1 | Create DNS providers → Backup → Verify providers included |
| 5 | should include user accounts in backup | P1 | Create users → Backup → Verify users included |
| 6 | should include security configuration in backup | P1 | Configure security → Backup → Verify config included |
| 7 | should include uptime monitors in backup | P2 | Create monitors → Backup → Verify monitors included |
| 8 | should encrypt sensitive data in backup | P0 | Create backup with encryption key → Verify credentials encrypted |

**Scenario Group B: Full Restore Cycle**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 9 | should restore all proxy hosts from backup | P0 | Restore → Verify all hosts exist with correct config |
| 10 | should restore certificates and assignments | P0 | Restore → Verify certs exist and assigned to correct hosts |
| 11 | should restore access lists and assignments | P0 | Restore → Verify ACLs exist and assigned correctly |
| 12 | should restore user accounts with password hashes | P1 | Restore → Verify users can log in with original passwords |
| 13 | should restore security configuration | P1 | Restore → Verify WAF/CrowdSec/Rate Limit settings restored |
| 14 | should handle restore to empty database | P0 | Clear DB → Restore → Verify all data recovered |
| 15 | should handle restore to existing database | P1 | Have existing data → Restore → Verify merge behavior |

**Scenario Group C: Data Integrity Verification**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 16 | should preserve foreign key relationships | P0 | Restore → Verify host-cert, host-acl, host-dnsProvider relations |
| 17 | should preserve timestamps (created_at, updated_at) | P1 | Restore → Verify original timestamps preserved |
| 18 | should preserve UUIDs for all entities | P0 | Restore → Verify UUIDs match original values |
| 19 | should verify backup checksum before restore | P1 | Corrupt backup file → Attempt restore → Verify rejection |

**Scenario Group D: Edge Cases & Recovery**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 20 | should handle partial backup (missing components) | P2 | Create backup with only hosts → Restore → Verify no errors |
| 21 | should roll back on restore failure | P1 | Inject failure mid-restore → Verify original data preserved |
| 22 | should support backup from older Charon version | P2 | Restore v1.x backup to v2.x → Verify migration applied |

**Scenario Group E: Encryption Handling (Supervisor Recommendation)**

> **Note:** Added per Supervisor review - Section 6.5 Test #8 mentions encryption but restoration decryption wasn't explicitly tested.

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 23 | should restore with correct encryption key | P1 | Create encrypted backup → Restore with correct key → Verify all data decrypted |
| 24 | should show clear error with wrong encryption key | P1 | Create encrypted backup → Restore with wrong key → Verify clear error message |

**Key Integration Flow:**
```typescript
test('complete backup and restore cycle with verification', async ({ page, testData }) => {
  // Step 1: Create comprehensive test data
  const hostData = await test.step('Create test data', async () => {
    const dnsProvider = await testData.createDNSProvider({
      type: 'manual',
      name: 'Test DNS'
    });

    const certificate = await testData.createCertificate({
      domains: ['app.example.com'],
      type: 'custom',
      privateKey: MOCK_KEY,
      certificate: MOCK_CERT
    });

    const accessList = await testData.createAccessList({
      name: 'Test ACL',
      type: 'whitelist',
      rules: [{ type: 'allow', value: '10.0.0.0/8' }]
    });

    const proxyHost = await testData.createProxyHost({
      domain: 'app.example.com',
      forwardHost: '192.168.1.100',
      forwardPort: 8080,
      certificateId: certificate.id,
      accessListId: accessList.id,
      dnsProviderId: dnsProvider.id
    });

    return { dnsProvider, certificate, accessList, proxyHost };
  });

  // Step 2: Create backup
  let backupFilename: string;
  await test.step('Create backup', async () => {
    await page.goto('/tasks/backups');

    const responsePromise = waitForAPIResponse(page, '/api/v1/backups', { status: 201 });
    await page.getByRole('button', { name: /create backup/i }).click();
    const response = await responsePromise;
    const result = await response.json();
    backupFilename = result.filename;

    await waitForToast(page, /backup created/i);
  });

  // Step 3: Delete all data (simulate disaster)
  await test.step('Clear database', async () => {
    // Delete via API to simulate clean slate
    await page.request.delete(`/api/v1/proxy-hosts/${hostData.proxyHost.id}`);
    await page.request.delete(`/api/v1/access-lists/${hostData.accessList.id}`);
    await page.request.delete(`/api/v1/certificates/${hostData.certificate.id}`);
    await page.request.delete(`/api/v1/dns-providers/${hostData.dnsProvider.id}`);

    // Verify data deleted
    await page.goto('/proxy-hosts');
    await expect(page.getByTestId('empty-state')).toBeVisible();
  });

  // Step 4: Restore from backup
  await test.step('Restore from backup', async () => {
    await page.goto('/tasks/backups');
    await page.getByRole('row', { name: new RegExp(backupFilename) })
      .getByRole('button', { name: /restore/i }).click();

    // Confirm restore
    await page.getByRole('button', { name: /confirm|restore/i }).click();
    await waitForToast(page, /restored|complete/i, { timeout: 60000 });
  });

  // Step 5: Verify all data restored with relationships
  await test.step('Verify data integrity', async () => {
    // Verify proxy host exists
    await page.goto('/proxy-hosts');
    await expect(page.getByRole('row', { name: /app.example.com/ })).toBeVisible();

    // Verify proxy host has certificate assigned
    await page.getByRole('row', { name: /app.example.com/ }).getByRole('button', { name: /edit/i }).click();
    await expect(page.getByLabel('SSL Certificate')).toHaveValue(hostData.certificate.id);

    // Verify proxy host has ACL assigned
    await expect(page.getByLabel('Access Control')).toHaveValue(hostData.accessList.id);

    // Verify proxy host has DNS provider assigned
    await expect(page.getByLabel('DNS Provider')).toHaveValue(hostData.dnsProvider.id);
  });
});
```

---

#### 6.6 Import to Production Workflows (`tests/integration/import-to-production.spec.ts`)

**Objective:** Verify end-to-end import workflows from Caddyfile/CrowdSec config to production deployment.

**Test Scenarios (12-15 tests):**

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should import Caddyfile and create working proxy hosts | P0 | Upload Caddyfile → Review → Commit → Verify hosts work |
| 2 | should import and enable security on imported hosts | P1 | Import hosts → Assign ACLs → Enable WAF → Verify protection |
| 3 | should import Caddyfile with SSL configuration | P1 | Import hosts with tls directives → Verify certificates created |
| 4 | should import CrowdSec config and verify decisions | P1 | Import CrowdSec YAML → Verify scenarios active → Test enforcement |
| 5 | should handle import conflict with existing hosts | P0 | Import duplicate domain → Verify conflict resolution options |
| 6 | should preserve advanced config during import | P2 | Import with custom Caddy snippets → Verify preserved |
| 7 | should create backup before import | P0 | Start import → Verify backup created automatically |
| 8 | should allow rollback after import | P1 | Complete import → Click rollback → Verify original state restored |
| 9 | should import and assign DNS providers | P2 | Import with dns challenge directives → Verify provider configured |
| 10 | should validate imported hosts before commit | P0 | Import with invalid config → Verify validation errors shown |

---

#### 6.7 Multi-Feature Workflows (`tests/integration/multi-feature-workflows.spec.ts`)

**Objective:** Test complex real-world user journeys that span multiple features.

**Test Scenarios (15-18 tests):**

**Scenario A: New Application Deployment**
```
Create Proxy Host → Upload Certificate → Assign ACL → Enable WAF → Test Access
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 1 | should complete new app deployment workflow | P0 | Full workflow from host creation to verified access |
| 2 | should handle app deployment with ACME certificate | P1 | Request Let's Encrypt cert during host creation |
| 3 | should configure monitoring after deployment | P1 | Create host → Add uptime monitor → Verify checks running |

**Scenario B: Security Hardening**
```
Audit Existing Host → Add ACL → Enable WAF → Configure Rate Limiting → Verify Protection
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 4 | should complete security hardening workflow | P0 | Add all security layers to existing host |
| 5 | should test security configuration without downtime | P1 | Enable security → Verify no request failures |

**Scenario C: Migration & Cutover**
```
Import from Caddyfile → Verify Configuration → Update DNS → Test Production
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 6 | should complete migration from standalone Caddy | P0 | Import → Configure → Cutover workflow |
| 7 | should support staged migration (one host at a time) | P2 | Import all → Enable one by one |

**Scenario D: Disaster Recovery**
```
Simulate Failure → Restore Backup → Verify All Services → Confirm Monitoring
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 8 | should complete disaster recovery workflow | P0 | Clear DB → Restore → Verify all features working |
| 9 | should verify no data loss after recovery | P0 | Compare pre/post restore entity counts |

**Scenario E: Multi-Tenant Setup**
```
Create Users → Assign Roles → Create User-Specific Resources → Verify Isolation
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 10 | should support multi-user resource management | P1 | Multiple users creating hosts → Verify proper access control |
| 11 | should audit all user actions | P1 | Create resources as different users → Verify audit trail |

**Scenario F: Certificate Lifecycle**
```
Upload Cert → Assign to Hosts → Receive Expiry Warning → Renew → Verify Seamless Transition
```

| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| 12 | should handle certificate renewal workflow | P1 | Mock expiring cert → Renew → Verify no downtime |
| 13 | should alert on certificate expiration | P0 | Create expiring cert → Verify notification sent |

---

#### 6.8 Phase 6 Test Utilities & Fixtures

**New Fixtures Required:**

```typescript
// tests/fixtures/integration-fixtures.ts

import { test as base, expect } from '@bgotink/playwright-coverage';
import { TestDataManager } from '../utils/TestDataManager';

interface IntegrationFixtures {
  // Full environment with all features configured
  fullEnvironment: {
    proxyHost: ProxyHostData;
    certificate: CertificateData;
    accessList: AccessListData;
    dnsProvider: DNSProviderData;
  };

  // Security stack enabled and configured
  securityStack: {
    wafEnabled: boolean;
    crowdsecConnected: boolean;
    rateLimitEnabled: boolean;
  };

  // Backup with known contents for restore testing
  knownBackup: {
    filename: string;
    contents: BackupManifest;
  };
}

export const test = base.extend<IntegrationFixtures>({
  fullEnvironment: async ({ testData }, use) => {
    const dnsProvider = await testData.createDNSProvider({
      type: 'manual',
      name: 'Integration Test DNS'
    });

    const certificate = await testData.createCertificate({
      domains: ['integration-test.example.com'],
      type: 'custom'
    });

    const accessList = await testData.createAccessList({
      name: 'Integration Test ACL',
      type: 'whitelist',
      rules: [{ type: 'allow', value: '10.0.0.0/8' }]
    });

    const proxyHost = await testData.createProxyHost({
      domain: 'integration-test.example.com',
      forwardHost: '192.168.1.100',
      forwardPort: 8080,
      certificateId: certificate.id,
      accessListId: accessList.id,
      dnsProviderId: dnsProvider.id
    });

    await use({ proxyHost, certificate, accessList, dnsProvider });
  },

  securityStack: async ({ page, request }, use) => {
    // Enable all security features via API
    await request.put('/api/v1/cerberus/waf', {
      data: { enabled: true, mode: 'blocking' }
    });
    await request.put('/api/v1/cerberus/ratelimit', {
      data: { enabled: true, requests: 100, windowSec: 60 }
    });

    // Verify CrowdSec connected
    const crowdsecStatus = await request.get('/api/v1/crowdsec/status');
    const status = await crowdsecStatus.json();

    await use({
      wafEnabled: true,
      crowdsecConnected: status.connected,
      rateLimitEnabled: true
    });
  }
});
```

**Wait Helpers Extension:**

```typescript
// Add to tests/utils/wait-helpers.ts

/**
 * Wait for security event to appear in security logs
 */
export async function waitForSecurityEvent(
  page: Page,
  eventType: 'waf_block' | 'crowdsec_ban' | 'rate_limit' | 'acl_block',
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 10000 } = options;

  await page.goto('/security/logs');
  await expect(page.getByRole('row').filter({ hasText: new RegExp(eventType, 'i') }))
    .toBeVisible({ timeout });
}

/**
 * Wait for backup operation to complete
 */
export async function waitForBackupComplete(
  page: Page,
  options: { timeout?: number } = {}
): Promise<string> {
  const { timeout = 60000 } = options;

  const response = await page.waitForResponse(
    resp => resp.url().includes('/api/v1/backups') && resp.status() === 201,
    { timeout }
  );

  const result = await response.json();
  return result.filename;
}

/**
 * Wait for restore operation to complete
 */
export async function waitForRestoreComplete(
  page: Page,
  options: { timeout?: number } = {}
): Promise<void> {
  const { timeout = 120000 } = options;

  await page.waitForResponse(
    resp => resp.url().includes('/restore') && resp.status() === 200,
    { timeout }
  );

  // Wait for page reload after restore
  await page.waitForLoadState('networkidle');
}
```

---

#### 6.8.1 Optional Enhancements (Supervisor Suggestions)

> **Note:** These are non-blocking suggestions from Supervisor review. Implement if time permits or defer to future phases.

**Performance Baseline Tests (2-3 tests):**
| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| O1 | should measure security stack latency impact | P3 | WAF + CrowdSec + Rate Limit adds < 50ms overhead |
| O2 | should complete backup creation within time limit | P3 | Backup 100+ proxy hosts in < 30 seconds |
| O3 | should complete restore within time limit | P3 | Restore benchmark for planning capacity |

**Multi-Tenant Isolation (2 tests):**
| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| O4 | should isolate User A resources from User B | P2 | User A cannot see/modify User B's proxy hosts |
| O5 | should allow admin to see all user resources | P2 | Admin has visibility into all users' resources |

**Certificate Chain Validation (2 tests):**
| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| O6 | should validate full certificate chain | P2 | Upload cert with intermediate + root → Verify chain validated |
| O7 | should warn on incomplete certificate chain | P2 | Upload cert missing intermediate → Verify warning shown |

**Geo-IP Database Integration (2 tests):**
| # | Test Name | Priority | Description |
|---|-----------|----------|-------------|
| O8 | should propagate Geo-IP database updates | P3 | Update GeoIP DB → Verify new country codes recognized |
| O9 | should validate country codes in ACL | P3 | Enter invalid country code → Verify validation error |

---

#### 6.9 Phase 6 Acceptance Criteria

**Proxy + ACL Integration (18-22 tests minimum):**
- [ ] ACL assignment and removal works correctly
- [ ] ACL enforcement verified (block/allow behavior)
- [ ] Dynamic ACL updates apply immediately
- [ ] Bulk ACL operations work correctly
- [ ] Audit logging captures ACL enforcement events

**Proxy + Certificate Integration (15-18 tests minimum):**
- [ ] Certificate assignment and HTTPS enforcement
- [ ] Wildcard and SAN certificates supported
- [ ] Certificate lifecycle management (expiry warnings, renewal)
- [ ] Certificate cleanup on host deletion

**Security Suite Integration (20-25 tests minimum):**
- [ ] All security components work together
- [ ] Attack detection and blocking verified
- [ ] Security event logging complete
- [ ] Rule precedence correct (ACL → WAF → Rate Limit → CrowdSec)
- [ ] Per-host security overrides work

**Backup/Restore (18-22 tests minimum):**
- [ ] All data types included in backup
- [ ] Complete restore with foreign key preservation
- [ ] Data integrity verification passes
- [ ] Encrypted backup/restore works

**Overall Phase 6:**
- [ ] 85+ tests passing
- [ ] <5% flaky test rate
- [ ] All P0 integration scenarios complete
- [ ] 90%+ P1 scenarios complete
- [ ] Cross-feature workflows verified
- [ ] No hardcoded waits (use wait-helpers)

---

#### 6.10 Phase 6 Implementation Schedule

| Day | Focus | Test Files | Est. Tests |
|-----|-------|------------|------------|
| **Day 1** | Proxy + ACL Integration | `proxy-acl-integration.spec.ts` | 18-22 |
| **Day 2** | Proxy + Certificate, DNS Integration | `proxy-certificate.spec.ts`, `proxy-dns-integration.spec.ts` | 22-27 |
| **Day 3** | Security Suite Integration + WebSocket | `security-suite-integration.spec.ts` | 23-28 |
| **Day 4** | Backup/Restore E2E + Encryption | `backup-restore-e2e.spec.ts` | 20-24 |
| **Day 5** | Multi-Feature Workflows + Buffer | `import-to-production.spec.ts`, `multi-feature-workflows.spec.ts` | 12-15 |

**Total Estimated:** 90-110 tests (+ 9 optional enhancement tests)

> **Supervisor Note:** Day 3 includes 3 additional WebSocket stability tests. Day 4 includes 2 additional encryption handling tests.

---

#### 6.11 Buffer Time Allocation

**Buffer Usage (2 days included):**
- **Day 1 Buffer:** Address flaky tests from Phase 1-5, fix any CI pipeline issues
- **Day 2 Buffer:** Improve test stability, add missing edge cases, documentation updates

**Buffer Triggers:**
- If any phase overruns by >20%
- If flaky test rate exceeds 5%
- If critical infrastructure issues discovered
- If new integration scenarios identified during testing

**Buffer Activities:**
1. Stabilize flaky tests (identify root cause, implement fixes)
2. Add retry logic where appropriate
3. Improve wait helper utilities
4. Update CI configuration for reliability
5. Document discovered edge cases for future phases

---

## Success Metrics & Acceptance Criteria

### Coverage Goals

**Test Coverage Targets:**
- 🎯 **Core Features:** 100% coverage (auth, navigation, dashboard)
- 🎯 **Critical Features:** 100% coverage (proxy hosts, ACLs, certificates)
- 🎯 **High Priority Features:** 90% coverage (security suite, backups)
- 🎯 **Medium Priority Features:** 80% coverage (settings, monitoring)
- 🎯 **Nice-to-Have Features:** 70% coverage (imports, plugins)

**Feature Coverage Matrix:**

| Feature | Priority | Target Coverage | Test Files | Status |
|---------|----------|----------------|------------|--------|
| Authentication | Critical | 100% | 1 | ✅ Covered (94% - 3 session tests deferred) |
| Dashboard | Core | 100% | 1 | ✅ Covered |
| Navigation | Core | 100% | 1 | ✅ Covered |
| Proxy Hosts | Critical | 100% | 3 | ❌ Not started |
| Certificates | Critical | 100% | 3 | ❌ Not started |
| Access Lists | Critical | 100% | 2 | ❌ Not started |
| CrowdSec | High | 90% | 3 | ❌ Not started |
| WAF | High | 90% | 1 | ❌ Not started |
| Rate Limiting | High | 90% | 1 | ❌ Not started |
| Security Headers | Medium | 80% | 1 | ❌ Not started |
| Audit Logs | Medium | 80% | 1 | ❌ Not started |
| Backups | High | 90% | 2 | ❌ Not started |
| Users | High | 90% | 2 | ❌ Not started |
| Settings | Medium | 80% | 4 | ❌ Not started |
| Monitoring | Medium | 80% | 3 | ❌ Not started |
| Import/Export | Medium | 80% | 2 | ❌ Not started |
| DNS Providers | Critical | 100% | 4 | ✅ Covered |

---

## Next Steps

1. ~~**Review and Approve Plan:** Stakeholder sign-off~~ ✅
2. ~~**Set Up Test Infrastructure:** Fixtures, utilities, CI configuration~~ ✅
3. ~~**Begin Phase 1 Implementation:** Foundation tests~~ ✅
4. **Begin Phase 2 Implementation:** Critical Path (Proxy Hosts, Certificates, ACLs)
5. **Fix Session Expiration Tests:** See [docs/issues/e2e-session-expiration-tests.md](../issues/e2e-session-expiration-tests.md)
6. **Daily Standup Check-ins:** Progress tracking, blocker resolution
7. **Weekly Demo:** Show completed test coverage

---

## Phase 7: Failing Test Remediation

**Date Added:** January 2026
**Status:** Research Complete - Remediation Pending
**Priority:** High - Unblocks CI Pipeline Stability

### 7.1 Current Test Run Status

**Latest Run Statistics:**
- ✅ **533 passed** - Core functionality verified
- ⏭️ **90 skipped** - Feature flags/dependencies not met
- ❌ **4 unexpected failures** - Require immediate attention

### 7.2 Failing Test Analysis

#### Test 1: Uptime Monitoring - Manual Check Status Update
- **File:** `tests/monitoring/uptime-monitoring.spec.ts:640`
- **Test Name:** `should update status after manual check`
- **Status:** Marked as `test.skip` due to flakiness
- **Error:** `page.waitForResponse: Test timeout of 30000ms exceeded` (13.2s actual)
- **Root Cause:** Race condition + async backend design
  - `CheckMonitor()` in `uptime_handler.go` uses `go h.service.CheckMonitor(*monitor)` (goroutine)
  - Backend returns `{"message": "Check triggered"}` immediately
  - Frontend toast fires before status actually updates
  - `waitForToast()` unreliable with mocked API routes
- **Skip Comment:** "Flaky test - toast detection unreliable with mocked routes"

#### Test 2: Uptime Monitoring - Sync from Proxy Hosts
- **File:** `tests/monitoring/uptime-monitoring.spec.ts:783`
- **Test Name:** `should sync monitors from proxy hosts`
- **Status:** Marked as `test.skip` due to flakiness
- **Error:** `page.waitForResponse: Test timeout of 30000ms exceeded` (13.4s actual)
- **Root Cause:** Same race condition pattern as Test 1
  - Sync button triggers API call
  - `waitForAPIResponse()` called AFTER action completes
  - Response already fulfilled before wait starts
- **Skip Comment:** "Flaky test - toast detection unreliable with mocked routes"

#### Test 3: Account Settings - Save Certificate Email
- **File:** `tests/settings/account-settings.spec.ts:314`
- **Test Name:** `should save certificate email`
- **Status:** Active (NOT skipped) - Failing
- **Error:** `waitForToast: Test timeout` (8.2s actual)
- **Root Cause:** Toast detection failure
  - Test unchecks `#useUserEmail`, fills custom email, clicks save
  - Expects success toast matching `/updated|saved|success/i`
  - Frontend uses `updateSettingMutation` with key `caddy.email`
  - Toast fires via `toast.success(t('account.certEmailUpdated'))`
  - Selector `[data-testid="toast-success"]` may not be present on toast component
- **Fix Required:** Verify `data-testid` attribute exists on toast component

#### Test 4: Related Pattern (from PHASE5_E2E_REMEDIATION.md)
Additional tests sharing the same failure pattern identified in prior remediation docs:
- `backups-create.spec.ts:186` - Create backup
- `backups-restore.spec.ts:157` - Restore backup
- `import-crowdsec.spec.ts:180/237/281` - CrowdSec import (also has API path mismatch)
- `logs-viewing.spec.ts:418` - Log pagination

### 7.3 Root Cause Summary

| Root Cause | Affected Tests | Pattern |
|------------|----------------|---------|
| Race Condition: `waitForAPIResponse()` after action | 6+ tests | Response completes before wait starts |
| Async Backend: Goroutine execution | 2 tests | Status check runs in background |
| Toast `data-testid` Missing/Incorrect | 3+ tests | `[data-testid="toast-success"]` not found |
| API Path Mismatch | 3 tests | `/api/v1/crowdsec/import` vs `/api/v1/admin/crowdsec/import` |

### 7.4 Remediation Fixes

#### Fix A: Race Condition Resolution (All Timeout Failures)

**Pattern to Fix:**
```typescript
// ❌ BROKEN: Race condition - response may complete before wait starts
await page.click(SELECTORS.actionButton);
await waitForAPIResponse(page, '/api/v1/endpoint', { status: 200 });
```

**Fixed Pattern:**
```typescript
// ✅ FIXED: Set up listener before triggering action
await Promise.all([
  page.waitForResponse(
    resp => resp.url().includes('/api/v1/endpoint') && resp.status() === 200
  ),
  page.click(SELECTORS.actionButton),
]);
```

**Alternative - Pre-register Promise:**
```typescript
const responsePromise = page.waitForResponse(
  resp => resp.url().includes('/api/v1/endpoint') && resp.status() === 200
);
await page.click(SELECTORS.actionButton);
await responsePromise;
```

#### Fix B: CrowdSec API Path Correction

**File:** `tests/tasks/import-crowdsec.spec.ts`

| Line | Current | Corrected |
|------|---------|-----------|
| 108 | `**/api/v1/crowdsec/import` | `**/api/v1/admin/crowdsec/import` |
| 144 | Same | Same |
| 202 | Same | Same |
| 226-325 | All waitForAPIResponse calls | Update path pattern |

#### Fix C: Toast Component `data-testid` Verification

**Investigate:**
1. Check toast library configuration (likely `react-hot-toast` or similar)
2. Ensure success toasts have `data-testid="toast-success"`
3. Verify toast container has `data-testid="toast-container"`

**Frontend Location:** Check component that wraps `<Toaster />` in layout

#### Fix D: New Helper Function (Infrastructure)

Add to `tests/utils/wait-helpers.ts`:

```typescript
/**
 * Click an element and wait for an API response atomically.
 * Prevents race condition where response completes before wait starts.
 */
export async function clickAndWaitForResponse(
  page: Page,
  clickTarget: Locator | string,
  urlPattern: string | RegExp,
  options: { status?: number; timeout?: number } = {}
): Promise<Response> {
  const { status = 200, timeout = 30000 } = options;

  const locator = typeof clickTarget === 'string'
    ? page.locator(clickTarget)
    : clickTarget;

  const [response] = await Promise.all([
    page.waitForResponse(
      resp => {
        const urlMatch = typeof urlPattern === 'string'
          ? resp.url().includes(urlPattern)
          : urlPattern.test(resp.url());
        return urlMatch && resp.status() === status;
      },
      { timeout }
    ),
    locator.click(),
  ]);

  return response;
}
```

### 7.5 Skipped Test Categorization (90 Tests)

| Category | Count | Reason | Status |
|----------|-------|--------|--------|
| Cerberus/LiveLogViewer Disabled | 24 | `cerberusEnabled` flag false | Expected - feature flag |
| User Management Features | 15+ | Admin-only features, fixture issues | Needs review |
| DNS Provider Advanced | 6 | Provider-specific validation | Needs provider credentials |
| Notifications | 8+ | SMTP/external service mocks | Needs mock infrastructure |
| Encryption Management | 6 | Encryption key handling | Security-sensitive |
| Account Settings | 3 | Checkbox toggle behavior | Fix UI interactions |
| SMTP Settings | 2 | External service dependency | Needs mock |
| System Settings | 4 | Admin privileges required | Fixture enhancement |
| Security Dashboard | 6 | CrowdSec/WAF integration | Integration dependencies |
| Rate Limiting | 2 | Timing-sensitive | Needs stable mocks |

### 7.6 Implementation Priority

| Priority | Task | Effort | Tests Fixed |
|----------|------|--------|-------------|
| 1 - Critical | Add `clickAndWaitForResponse` helper | 30 min | 0 (infrastructure) |
| 2 - Critical | Apply Promise.all pattern to failing tests | 45 min | 6 tests |
| 3 - High | Fix CrowdSec API paths | 10 min | 3 tests |
| 4 - High | Verify toast `data-testid` in frontend | 20 min | 3+ tests |
| 5 - Medium | Unskip and fix uptime monitoring tests | 30 min | 2 tests |
| 6 - Low | Review and categorize remaining skipped tests | 1 hour | Documentation |

**Total Estimated Effort:** ~3 hours

### 7.7 Verification Commands

```bash
# After applying fixes, run targeted tests:
npx playwright test \
  tests/monitoring/uptime-monitoring.spec.ts \
  tests/settings/account-settings.spec.ts \
  tests/tasks/backups-create.spec.ts \
  tests/tasks/backups-restore.spec.ts \
  tests/tasks/import-crowdsec.spec.ts \
  tests/tasks/logs-viewing.spec.ts \
  --project=chromium

# Expected result: All previously failing tests should pass
# Skipped tests remain skipped until feature flags enabled
```

### 7.8 Success Criteria

- [ ] All 4 previously failing tests now pass
- [ ] No new test failures introduced
- [ ] `clickAndWaitForResponse` helper added to `wait-helpers.ts`
- [ ] CrowdSec API paths corrected
- [ ] Toast `data-testid` attributes verified
- [ ] Skipped test inventory documented for future phases

---

**Document Status:** In Progress - Phase 1 Complete
**Last Updated:** January 2026
**Phase 1 Completed:** January 17, 2026 (112/119 tests passing - 94%)
**Phase 7 Added:** January 2026 - Failing Test Remediation Plan
**Next Review:** Upon Phase 2 completion (estimated Jan 31, 2026)
**Owner:** Planning Agent / QA Team
