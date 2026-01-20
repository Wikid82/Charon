/**
 * TestDataManager - Manages test data with namespace isolation and automatic cleanup
 *
 * This utility provides:
 * - Unique namespace per test to avoid conflicts in parallel execution
 * - Resource tracking for automatic cleanup
 * - Cleanup in reverse order (newest first) to respect FK constraints
 *
 * @example
 * ```typescript
 * let testData: TestDataManager;
 *
 * test.beforeEach(async ({ request }, testInfo) => {
 *   testData = new TestDataManager(request, testInfo.title);
 * });
 *
 * test.afterEach(async () => {
 *   await testData.cleanup();
 * });
 *
 * test('example', async () => {
 *   const { id, domain } = await testData.createProxyHost({
 *     domain: 'app.example.com',
 *     forwardHost: '192.168.1.100',
 *     forwardPort: 3000
 *   });
 * });
 * ```
 */

import { APIRequestContext } from '@playwright/test';
import * as crypto from 'crypto';

/**
 * Represents a managed resource created during tests
 */
export interface ManagedResource {
  /** Unique identifier of the resource */
  id: string;
  /** Type of resource for cleanup routing */
  type: 'proxy-host' | 'certificate' | 'access-list' | 'dns-provider' | 'user';
  /** Namespace that owns this resource */
  namespace: string;
  /** When the resource was created (for ordering cleanup) */
  createdAt: Date;
}

/**
 * Data required to create a proxy host
 */
export interface ProxyHostData {
  domain: string;
  forwardHost: string;
  forwardPort: number;
  scheme?: 'http' | 'https';
  websocketSupport?: boolean;
}

/**
 * Data required to create an access list
 */
export interface AccessListData {
  name: string;
  rules: Array<{ type: 'allow' | 'deny'; value: string }>;
}

/**
 * Data required to create a certificate
 */
export interface CertificateData {
  domains: string[];
  type: 'letsencrypt' | 'custom';
  privateKey?: string;
  certificate?: string;
}

/**
 * Data required to create a DNS provider
 */
export interface DNSProviderData {
  type: 'manual' | 'cloudflare' | 'route53' | 'webhook' | 'rfc2136';
  name: string;
  credentials?: Record<string, string>;
}

/**
 * Data required to create a user
 */
export interface UserData {
  name: string;
  email: string;
  password: string;
  role: 'admin' | 'user' | 'guest';
}

/**
 * Result of creating a proxy host
 */
export interface ProxyHostResult {
  id: string;
  domain: string;
}

/**
 * Result of creating an access list
 */
export interface AccessListResult {
  id: string;
  name: string;
}

/**
 * Result of creating a certificate
 */
export interface CertificateResult {
  id: string;
  domains: string[];
}

/**
 * Result of creating a DNS provider
 */
export interface DNSProviderResult {
  id: string;
  name: string;
}

/**
 * Result of creating a user
 */
export interface UserResult {
  id: string;
  email: string;
  token: string;
}

/**
 * Manages test data lifecycle with namespace isolation
 */
export class TestDataManager {
  private resources: ManagedResource[] = [];
  private namespace: string;
  private request: APIRequestContext;

  /**
   * Creates a new TestDataManager instance
   * @param request - Playwright API request context
   * @param testName - Optional test name for namespace generation
   */
  constructor(request: APIRequestContext, testName?: string) {
    this.request = request;
    // Create unique namespace per test to avoid conflicts
    this.namespace = testName
      ? `test-${this.sanitize(testName)}-${Date.now()}`
      : `test-${crypto.randomUUID()}`;
  }

  /**
   * Sanitizes a test name for use in identifiers
   */
  private sanitize(name: string): string {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]/g, '-')
      .substring(0, 30);
  }

  /**
   * Create a proxy host with automatic cleanup tracking
   * @param data - Proxy host configuration
   * @returns Created proxy host details
   */
  async createProxyHost(data: ProxyHostData): Promise<ProxyHostResult> {
    const namespaced = {
      ...data,
      domain: `${this.namespace}.${data.domain}`, // Ensure unique domain
    };

    const response = await this.request.post('/api/v1/proxy-hosts', {
      data: namespaced,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create proxy host: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.uuid || result.id,
      type: 'proxy-host',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: result.uuid || result.id, domain: namespaced.domain };
  }

  /**
   * Create an access list with automatic cleanup tracking
   * @param data - Access list configuration
   * @returns Created access list details
   */
  async createAccessList(data: AccessListData): Promise<AccessListResult> {
    const namespaced = {
      ...data,
      name: `${this.namespace}-${data.name}`,
    };

    const response = await this.request.post('/api/v1/access-lists', {
      data: namespaced,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create access list: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'access-list',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: result.id, name: namespaced.name };
  }

  /**
   * Create a certificate with automatic cleanup tracking
   * @param data - Certificate configuration
   * @returns Created certificate details
   */
  async createCertificate(data: CertificateData): Promise<CertificateResult> {
    const namespacedDomains = data.domains.map((d) => `${this.namespace}.${d}`);
    const namespaced = {
      ...data,
      domains: namespacedDomains,
    };

    const response = await this.request.post('/api/v1/certificates', {
      data: namespaced,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create certificate: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'certificate',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: result.id, domains: namespacedDomains };
  }

  /**
   * Create a DNS provider with automatic cleanup tracking
   * @param data - DNS provider configuration
   * @returns Created DNS provider details
   */
  async createDNSProvider(data: DNSProviderData): Promise<DNSProviderResult> {
    const namespacedName = `${this.namespace}-${data.name}`;
    const namespaced = {
      ...data,
      name: namespacedName,
    };

    const response = await this.request.post('/api/v1/dns-providers', {
      data: namespaced,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create DNS provider: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id || result.uuid,
      type: 'dns-provider',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: result.id || result.uuid, name: namespacedName };
  }

  /**
   * Create a test user with automatic cleanup tracking
   * @param data - User configuration
   * @returns Created user details including auth token
   */
  async createUser(data: UserData): Promise<UserResult> {
    const namespacedEmail = `${this.namespace}+${data.email}`;
    const namespaced = {
      name: data.name,
      email: namespacedEmail,
      password: data.password,
      role: data.role,
    };

    const response = await this.request.post('/api/v1/users', {
      data: namespaced,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create user: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id,
      type: 'user',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    // Automatically log in the user and return token
    const loginResponse = await this.request.post('/api/v1/auth/login', {
      data: { email: namespacedEmail, password: data.password },
    });

    if (!loginResponse.ok()) {
      // User created but login failed - still return user info
      console.warn(`User created but login failed: ${await loginResponse.text()}`);
      return { id: result.id, email: namespacedEmail, token: '' };
    }

    const { token } = await loginResponse.json();

    return { id: result.id, email: namespacedEmail, token };
  }

  /**
   * Clean up all resources in reverse order (respects FK constraints)
   * Resources are deleted newest-first to handle dependencies
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
      console.warn(`Cleanup completed with ${errors.length} errors`);
    }
  }

  /**
   * Delete a single managed resource
   */
  private async deleteResource(resource: ManagedResource): Promise<void> {
    const endpoints: Record<ManagedResource['type'], string> = {
      'proxy-host': `/api/v1/proxy-hosts/${resource.id}`,
      certificate: `/api/v1/certificates/${resource.id}`,
      'access-list': `/api/v1/access-lists/${resource.id}`,
      'dns-provider': `/api/v1/dns-providers/${resource.id}`,
      user: `/api/v1/users/${resource.id}`,
    };

    const endpoint = endpoints[resource.type];
    const response = await this.request.delete(endpoint);

    // 404 is acceptable - resource may have been deleted by another test
    if (!response.ok() && response.status() !== 404) {
      const errorText = await response.text();
      // Skip "Cannot delete your own account" errors - the test user is logged in
      // and will be cleaned up when the auth session ends or by admin cleanup
      if (resource.type === 'user' && errorText.includes('Cannot delete your own account')) {
        return; // Silently skip - this is expected for the authenticated test user
      }
      throw new Error(`Failed to delete ${resource.type}: ${errorText}`);
    }
  }

  /**
   * Get all resources created in this namespace
   * @returns Copy of the resources array
   */
  getResources(): ManagedResource[] {
    return [...this.resources];
  }

  /**
   * Get namespace identifier
   * @returns The unique namespace for this test
   */
  getNamespace(): string {
    return this.namespace;
  }
}
