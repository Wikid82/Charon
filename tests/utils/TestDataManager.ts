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

import { APIRequestContext, type APIResponse } from '@playwright/test';
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
  name?: string;
  scheme?: 'http' | 'https';
  websocketSupport?: boolean;
}

/**
 * Data required to create an access list
 */
export interface AccessListData {
  name: string;
  /** Access list type - determines allow/deny behavior */
  type: 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist';
  /** Optional description */
  description?: string;
  /** IP/CIDR rules for whitelist/blacklist types */
  ipRules?: Array<{ cidr: string; description?: string }>;
  /** Comma-separated country codes for geo types */
  countryCodes?: string;
  /** Restrict to local RFC1918 networks */
  localNetworkOnly?: boolean;
  /** Whether the access list is enabled */
  enabled?: boolean;
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
  /** Provider type identifier from backend registry */
  providerType: 'manual' | 'cloudflare' | 'route53' | 'webhook' | 'rfc2136' | string;
  /** Display name for the provider */
  name: string;
  /** Provider-specific credentials */
  credentials: Record<string, string>;
  /** Propagation timeout in seconds */
  propagationTimeout?: number;
  /** Polling interval in seconds */
  pollingInterval?: number;
  /** Whether this is the default provider */
  isDefault?: boolean;
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
   * Keeps it short to avoid overly long domain names
   */
  private sanitize(name: string): string {
    return name
      .toLowerCase()
      .replace(/[^a-z0-9]/g, '-')
      .replace(/-+/g, '-')  // Collapse multiple dashes
      .substring(0, 15);  // Keep short to avoid long domains
  }

  private async postWithRetry(
    url: string,
    data: Record<string, unknown>,
    options: {
      maxAttempts?: number;
      baseDelayMs?: number;
      retryStatuses?: number[];
    } = {}
  ): Promise<APIResponse> {
    const maxAttempts = options.maxAttempts ?? 4;
    const baseDelayMs = options.baseDelayMs ?? 300;
    const retryStatuses = options.retryStatuses ?? [429];

    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      const response = await this.request.post(url, { data });
      if (!retryStatuses.includes(response.status()) || attempt === maxAttempts) {
        return response;
      }

      const retryAfterHeader = response.headers()['retry-after'];
      const retryAfterSeconds = retryAfterHeader ? Number(retryAfterHeader) : Number.NaN;
      const backoffMs = Number.isFinite(retryAfterSeconds)
        ? retryAfterSeconds * 1000
        : Math.round(baseDelayMs * Math.pow(2, attempt - 1));

      await new Promise((resolve) => setTimeout(resolve, backoffMs));
    }

    return this.request.post(url, { data });
  }

  private async deleteWithRetry(
    url: string,
    options: {
      maxAttempts?: number;
      baseDelayMs?: number;
      retryStatuses?: number[];
    } = {}
  ): Promise<APIResponse> {
    const maxAttempts = options.maxAttempts ?? 4;
    const baseDelayMs = options.baseDelayMs ?? 300;
    const retryStatuses = options.retryStatuses ?? [429];

    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      const response = await this.request.delete(url);
      if (!retryStatuses.includes(response.status()) || attempt === maxAttempts) {
        return response;
      }

      const retryAfterHeader = response.headers()['retry-after'];
      const retryAfterSeconds = retryAfterHeader ? Number(retryAfterHeader) : Number.NaN;
      const backoffMs = Number.isFinite(retryAfterSeconds)
        ? retryAfterSeconds * 1000
        : Math.round(baseDelayMs * Math.pow(2, attempt - 1));

      await new Promise((resolve) => setTimeout(resolve, backoffMs));
    }

    return this.request.delete(url);
  }

  /**
   * Create a proxy host with automatic cleanup tracking
   * @param data - Proxy host configuration
   * @returns Created proxy host details
   */
  async createProxyHost(data: ProxyHostData): Promise<ProxyHostResult> {
    // Domain already contains uniqueness from generateDomain() in fixtures
    // Only add namespace prefix for test identification/cleanup purposes
    const namespacedDomain = `${this.namespace}.${data.domain}`;

    // Build payload matching backend ProxyHost model field names
    const payload: Record<string, unknown> = {
      domain_names: namespacedDomain,  // API expects domain_names, not domain
      forward_host: data.forwardHost,
      forward_port: data.forwardPort,
      forward_scheme: data.scheme ?? 'http',
      websocket_support: data.websocketSupport ?? false,
      enabled: true,
    };

    // Include name if provided (required for UI edits)
    if (data.name) {
      payload.name = data.name;
    }

    try {
      // Add explicit timeout with descriptive error for debugging
      const response = await this.request.post('/api/v1/proxy-hosts', {
        data: payload,
        timeout: 30000, // 30s timeout
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

      return { id: result.uuid || result.id, domain: namespacedDomain };
    } catch (error) {
      // Provide descriptive error for timeout debugging
      if (error instanceof Error && error.message.includes('timeout')) {
        throw new Error(
          `Timeout creating proxy host for ${namespacedDomain} after 30s. ` +
          `This may indicate API bottleneck or feature flag polling overhead.`
        );
      }
      throw error;
    }
  }

  /**
   * Create an access list with automatic cleanup tracking
   * @param data - Access list configuration
   * @returns Created access list details
   */
  async createAccessList(data: AccessListData): Promise<AccessListResult> {
    const namespacedName = `${this.namespace}-${data.name}`;

    // Build payload matching backend AccessList model
    const payload: Record<string, unknown> = {
      name: namespacedName,
      type: data.type,
      description: data.description ?? '',
      enabled: data.enabled ?? true,
      local_network_only: data.localNetworkOnly ?? false,
    };

    // Convert ipRules array to JSON string for ip_rules field
    if (data.ipRules && data.ipRules.length > 0) {
      payload.ip_rules = JSON.stringify(data.ipRules);
    }

    // Add country codes for geo types
    if (data.countryCodes) {
      payload.country_codes = data.countryCodes;
    }

    const response = await this.postWithRetry('/api/v1/access-lists', payload, {
      maxAttempts: 4,
      baseDelayMs: 300,
      retryStatuses: [429],
    });

    if (!response.ok()) {
      throw new Error(`Failed to create access list: ${await response.text()}`);
    }

    const result = await response.json();
    this.resources.push({
      id: result.id?.toString() ?? result.uuid,
      type: 'access-list',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: result.id?.toString() ?? result.uuid, name: namespacedName };
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

    // Build payload matching backend CreateDNSProviderRequest struct
    const payload: Record<string, unknown> = {
      name: namespacedName,
      provider_type: data.providerType,
      credentials: data.credentials,
    };

    // Add optional fields if provided
    if (data.propagationTimeout !== undefined) {
      payload.propagation_timeout = data.propagationTimeout;
    }
    if (data.pollingInterval !== undefined) {
      payload.polling_interval = data.pollingInterval;
    }
    if (data.isDefault !== undefined) {
      payload.is_default = data.isDefault;
    }

    const response = await this.request.post('/api/v1/dns-providers', {
      data: payload,
    });

    if (!response.ok()) {
      throw new Error(`Failed to create DNS provider: ${await response.text()}`);
    }

    const result = await response.json();

    // DNS provider IDs must be numeric (backend uses strconv.ParseUint)
    const id = result.id;
    if (id !== undefined && typeof id !== 'number') {
      console.warn(`DNS provider returned non-numeric ID: ${id} (type: ${typeof id}), using as-is`);
    }
    const resourceId = String(id ?? result.uuid);

    this.resources.push({
      id: resourceId,
      type: 'dns-provider',
      namespace: this.namespace,
      createdAt: new Date(),
    });

    return { id: resourceId, name: namespacedName };
  }

  /**
   * Create a test user with automatic cleanup tracking
   * @param data - User configuration
   * @returns Created user details including auth token
   */
  async createUser(
    data: UserData,
    options: { useNamespace?: boolean } = {}
  ): Promise<UserResult> {
    const useNamespace = options.useNamespace !== false;
    const namespacedEmail = useNamespace ? `${this.namespace}+${data.email}` : data.email;
    const namespaced = {
      name: data.name,
      email: namespacedEmail,
      password: data.password,
      role: data.role,
    };

    const response = await this.postWithRetry('/api/v1/users', namespaced, {
      maxAttempts: 4,
      baseDelayMs: 300,
      retryStatuses: [429],
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
    const response = await this.deleteWithRetry(endpoint, {
      maxAttempts: 4,
      baseDelayMs: 300,
      retryStatuses: [429],
    });

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

  /**
   * Assert that ACL and rate limiting are disabled before proceeding with ACL-dependent operations.
   * Fails fast with actionable error if security is still enabled.
   * Use this before tests that create/modify resources (proxy hosts, certificates, etc.)
   * to prevent 403 errors from security modules.
   *
   * @throws Error if ACL or rate limiting is enabled
   */
  async assertSecurityDisabled(): Promise<void> {
    try {
      const response = await this.request.get('/api/v1/security/config', { timeout: 3000 });
      if (!response.ok()) {
        // Endpoint might not exist or requires different auth - skip check
        return;
      }

      const config = await response.json();
      const aclEnabled = config.acl?.enabled === true;
      const rateLimitEnabled = config.rateLimit?.enabled === true;

      if (aclEnabled || rateLimitEnabled) {
        throw new Error(
          `\n❌ SECURITY MODULES ARE ENABLED - OPERATION WILL FAIL\n` +
          `   ACL: ${aclEnabled}, Rate Limiting: ${rateLimitEnabled}\n` +
          `   Cannot proceed with resource creation.\n` +
          `   Check: global-setup.ts emergency reset completed successfully\n`
        );
      }
    } catch (error) {
      // Re-throw if it's our security error
      if (error instanceof Error && error.message.includes('SECURITY MODULES ARE ENABLED')) {
        throw error;
      }
      // Otherwise, skip check (endpoint might not exist in this environment)
    }
  }

  /**
   * Force cleanup all test-created resources by pattern matching.
   * This is a nuclear option for cleaning up orphaned test data from previous runs.
   * Use sparingly - prefer the regular cleanup() method.
   *
   * @param request - Playwright API request context
   * @returns Object with counts of deleted resources
   */
  static async forceCleanupAll(request: APIRequestContext): Promise<{
    proxyHosts: number;
    accessLists: number;
    dnsProviders: number;
    certificates: number;
  }> {
    const results = {
      proxyHosts: 0,
      accessLists: 0,
      dnsProviders: 0,
      certificates: 0,
    };

    // Pattern to match test-created resources (namespace starts with 'test-')
    const testPattern = /^test-/;

    try {
      // Clean up proxy hosts
      const hostsResponse = await request.get('/api/v1/proxy-hosts');
      if (hostsResponse.ok()) {
        const hosts = await hostsResponse.json();
        for (const host of hosts) {
          // Match domains that contain test namespace pattern
          if (host.domain && testPattern.test(host.domain)) {
            const deleteResponse = await request.delete(`/api/v1/proxy-hosts/${host.uuid || host.id}`);
            if (deleteResponse.ok() || deleteResponse.status() === 404) {
              results.proxyHosts++;
            }
          }
        }
      }

      // Clean up access lists
      const aclResponse = await request.get('/api/v1/access-lists');
      if (aclResponse.ok()) {
        const acls = await aclResponse.json();
        for (const acl of acls) {
          if (acl.name && testPattern.test(acl.name)) {
            const deleteResponse = await request.delete(`/api/v1/access-lists/${acl.id}`);
            if (deleteResponse.ok() || deleteResponse.status() === 404) {
              results.accessLists++;
            }
          }
        }
      }

      // Clean up DNS providers
      const dnsResponse = await request.get('/api/v1/dns-providers');
      if (dnsResponse.ok()) {
        const providers = await dnsResponse.json();
        for (const provider of providers) {
          if (provider.name && testPattern.test(provider.name)) {
            const deleteResponse = await request.delete(`/api/v1/dns-providers/${provider.id}`);
            if (deleteResponse.ok() || deleteResponse.status() === 404) {
              results.dnsProviders++;
            }
          }
        }
      }

      // Clean up certificates
      const certResponse = await request.get('/api/v1/certificates');
      if (certResponse.ok()) {
        const certs = await certResponse.json();
        for (const cert of certs) {
          // Check if any domain matches test pattern
          const domains = cert.domains || [];
          const isTestCert = domains.some((d: string) => testPattern.test(d));
          if (isTestCert) {
            const deleteResponse = await request.delete(`/api/v1/certificates/${cert.id}`);
            if (deleteResponse.ok() || deleteResponse.status() === 404) {
              results.certificates++;
            }
          }
        }
      }
    } catch (error) {
      console.error('Error during force cleanup:', error);
    }

    console.log(`Force cleanup completed: ${JSON.stringify(results)}`);
    return results;
  }
}
