# Charon E2E Testing Plan: Comprehensive Playwright Coverage

**Date:** January 16, 2026
**Status:** Planning - Revised (v2.1)
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

**Goal:** Establish core application testing patterns

#### 1.1 Test Fixtures & Helpers
**Priority:** Critical
**Estimated Effort:** 2 days

**Tasks:**
- [ ] Create `tests/fixtures/test-data.ts` with common test data generators
- [ ] Create `tests/fixtures/proxy-hosts.ts` with mock proxy host data
- [ ] Create `tests/fixtures/access-lists.ts` with mock ACL data
- [ ] Create `tests/fixtures/certificates.ts` with mock certificate data
- [ ] Create `tests/utils/api-helpers.ts` for common API operations

**Acceptance Criteria:**
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
**Estimated Effort:** 3 days

**Test Files to Create:**

**`tests/core/authentication.spec.ts`**
- ✅ Login with valid credentials (covered by auth.setup.ts)
- ❌ Login with invalid credentials
- ❌ Logout functionality
- ❌ Session persistence
- ❌ Session expiration handling
- ❌ Password reset flow (if implemented)

**`tests/core/dashboard.spec.ts`**
- ❌ Dashboard loads successfully
- ❌ Summary cards display correct data
- ❌ Quick action buttons are functional
- ❌ Recent activity shows latest changes
- ❌ System status indicators work

**`tests/core/navigation.spec.ts`**
- ❌ All main menu items are clickable
- ❌ Sidebar navigation works
- ❌ Breadcrumbs display correctly
- ❌ Deep links resolve properly
- ❌ Back button navigation works

**Acceptance Criteria:**
- All authentication flows covered
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

### Phase 5: Tasks (Week 9)

**Goal:** Cover backup, logs, and monitoring features

**Estimated Effort:** 5 days

**Test Files:**
- `tests/tasks/backups-create.spec.ts` - Backup creation
- `tests/tasks/backups-restore.spec.ts` - Backup restoration
- `tests/tasks/logs-viewing.spec.ts` - Log viewer functionality
- `tests/tasks/import-caddyfile.spec.ts` - Caddyfile import
- `tests/tasks/import-crowdsec.spec.ts` - CrowdSec config import
- `tests/monitoring/uptime-monitoring.spec.ts` - Uptime checks
- `tests/monitoring/real-time-logs.spec.ts` - WebSocket log streaming

**Key Features:**
- Backup creation (manual, scheduled)
- Backup restoration (full, selective)
- Log viewing (filtering, search, export)
- Caddyfile import (validation, migration)
- CrowdSec import (scenarios, decisions)
- Uptime monitoring (HTTP checks, alerts)
- Real-time logs (WebSocket, filtering)

### Phase 6: Integration & Buffer (Week 10)

**Goal:** Test cross-feature interactions, edge cases, and provide buffer for overruns

**Estimated Effort:** 5 days (3 days testing + 2 days buffer)

**Test Files:**
- `tests/integration/proxy-acl-integration.spec.ts` - Proxy + ACL
- `tests/integration/proxy-certificate.spec.ts` - Proxy + SSL
- `tests/integration/security-suite-integration.spec.ts` - Full security stack
- `tests/integration/backup-restore-e2e.spec.ts` - Full backup cycle

**Key Scenarios:**
- Create proxy host with ACL and SSL certificate
- Test security stack: WAF + CrowdSec + Rate Limiting
- Full backup → Restore → Verify all data intact
- Multi-feature workflows (e.g., import Caddyfile + enable security)

**Buffer Time:**
- Address flaky tests discovered in previous phases
- Fix any infrastructure issues
- Improve test stability and reliability
- Documentation updates

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
| Authentication | Critical | 100% | 1 | ✅ Covered |
| Dashboard | Core | 100% | 1 | ❌ Not started |
| Navigation | Core | 100% | 1 | ❌ Not started |
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

1. **Review and Approve Plan:** Stakeholder sign-off
2. **Set Up Test Infrastructure:** Fixtures, utilities, CI configuration
3. **Begin Phase 1 Implementation:** Foundation tests
4. **Daily Standup Check-ins:** Progress tracking, blocker resolution
5. **Weekly Demo:** Show completed test coverage
6. **Iterate Based on Feedback:** Adjust plan as needed

---

**Document Status:** Planning
**Last Updated:** January 16, 2026
**Next Review:** Upon Phase 1 completion (estimated Jan 24, 2026)
**Owner:** Planning Agent / QA Team
