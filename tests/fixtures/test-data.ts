/**
 * Test Data Generators - Common test data factories
 *
 * This module provides functions to generate realistic test data
 * with unique identifiers to prevent collisions in parallel tests.
 *
 * @example
 * ```typescript
 * import { generateProxyHostData, generateUserData } from './fixtures/test-data';
 *
 * test('create proxy host', async ({ testData }) => {
 *   const hostData = generateProxyHostData();
 *   const { id } = await testData.createProxyHost(hostData);
 * });
 * ```
 */

import crypto from 'crypto';

/**
 * Generate a unique identifier with optional prefix
 * @param prefix - Optional prefix for the ID
 * @returns Unique identifier string
 */
export function generateUniqueId(prefix = ''): string {
  const timestamp = Date.now().toString(36);
  const random = crypto.randomBytes(4).toString('hex');
  return prefix ? `${prefix}-${timestamp}-${random}` : `${timestamp}-${random}`;
}

/**
 * Generate a unique domain name for testing
 * @param subdomain - Optional subdomain prefix
 * @returns Unique domain string
 */
export function generateDomain(subdomain = 'app'): string {
  const id = generateUniqueId();
  return `${subdomain}-${id}.test.local`;
}

/**
 * Generate a unique email address for testing
 * @param prefix - Optional email prefix
 * @returns Unique email string
 */
export function generateEmail(prefix = 'test'): string {
  const id = generateUniqueId();
  return `${prefix}-${id}@test.local`;
}

/**
 * Proxy host test data
 */
export interface ProxyHostTestData {
  domain: string;
  forwardHost: string;
  forwardPort: number;
  scheme: 'http' | 'https';
  websocketSupport: boolean;
}

/**
 * Generate proxy host test data with unique domain
 * @param overrides - Optional overrides for default values
 * @returns ProxyHostTestData object
 */
export function generateProxyHostData(
  overrides: Partial<ProxyHostTestData> = {}
): ProxyHostTestData {
  return {
    domain: generateDomain('proxy'),
    forwardHost: '192.168.1.100',
    forwardPort: 3000,
    scheme: 'http',
    websocketSupport: false,
    ...overrides,
  };
}

/**
 * Generate proxy host data for Docker container
 * @param containerName - Docker container name
 * @param port - Container port
 * @returns ProxyHostTestData object
 */
export function generateDockerProxyHostData(
  containerName: string,
  port = 80
): ProxyHostTestData {
  return {
    domain: generateDomain('docker'),
    forwardHost: containerName,
    forwardPort: port,
    scheme: 'http',
    websocketSupport: false,
  };
}

/**
 * Access list test data
 */
export interface AccessListTestData {
  name: string;
  rules: Array<{ type: 'allow' | 'deny'; value: string }>;
}

/**
 * Generate access list test data with unique name
 * @param overrides - Optional overrides for default values
 * @returns AccessListTestData object
 */
export function generateAccessListData(
  overrides: Partial<AccessListTestData> = {}
): AccessListTestData {
  const id = generateUniqueId();
  return {
    name: `ACL-${id}`,
    rules: [
      { type: 'allow', value: '192.168.1.0/24' },
      { type: 'deny', value: '0.0.0.0/0' },
    ],
    ...overrides,
  };
}

/**
 * Generate an allowlist that permits all traffic
 * @param name - Optional name override
 * @returns AccessListTestData object
 */
export function generateAllowAllAccessList(name?: string): AccessListTestData {
  return {
    name: name || `AllowAll-${generateUniqueId()}`,
    rules: [{ type: 'allow', value: '0.0.0.0/0' }],
  };
}

/**
 * Generate a denylist that blocks specific IPs
 * @param blockedIPs - Array of IPs to block
 * @returns AccessListTestData object
 */
export function generateDenyListAccessList(blockedIPs: string[]): AccessListTestData {
  return {
    name: `DenyList-${generateUniqueId()}`,
    rules: blockedIPs.map((ip) => ({ type: 'deny' as const, value: ip })),
  };
}

/**
 * Certificate test data
 */
export interface CertificateTestData {
  domains: string[];
  type: 'letsencrypt' | 'custom';
  privateKey?: string;
  certificate?: string;
}

/**
 * Generate certificate test data with unique domains
 * @param overrides - Optional overrides for default values
 * @returns CertificateTestData object
 */
export function generateCertificateData(
  overrides: Partial<CertificateTestData> = {}
): CertificateTestData {
  return {
    domains: [generateDomain('cert')],
    type: 'letsencrypt',
    ...overrides,
  };
}

/**
 * Generate custom certificate test data
 * Note: Uses placeholder values - in real tests, use actual cert/key
 * @param domains - Domains for the certificate
 * @returns CertificateTestData object
 */
export function generateCustomCertificateData(domains?: string[]): CertificateTestData {
  return {
    domains: domains || [generateDomain('custom-cert')],
    type: 'custom',
    // Placeholder - real tests should provide actual certificate data
    privateKey: '-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQ...\n-----END PRIVATE KEY-----',
    certificate: '-----BEGIN CERTIFICATE-----\nMIIDazCCAlOgAwIBAgIUa...\n-----END CERTIFICATE-----',
  };
}

/**
 * Generate wildcard certificate test data
 * @param baseDomain - Base domain for the wildcard
 * @returns CertificateTestData object
 */
export function generateWildcardCertificateData(baseDomain?: string): CertificateTestData {
  const domain = baseDomain || `${generateUniqueId()}.test.local`;
  return {
    domains: [`*.${domain}`, domain],
    type: 'letsencrypt',
  };
}

/**
 * User test data
 */
export interface UserTestData {
  email: string;
  password: string;
  role: 'admin' | 'user' | 'guest';
  name?: string;
}

/**
 * Generate user test data with unique email
 * @param overrides - Optional overrides for default values
 * @returns UserTestData object
 */
export function generateUserData(overrides: Partial<UserTestData> = {}): UserTestData {
  const id = generateUniqueId();
  return {
    email: generateEmail('user'),
    password: 'TestPass123!',
    role: 'user',
    name: `Test User ${id}`,
    ...overrides,
  };
}

/**
 * Generate admin user test data
 * @param overrides - Optional overrides
 * @returns UserTestData object
 */
export function generateAdminUserData(overrides: Partial<UserTestData> = {}): UserTestData {
  return generateUserData({ ...overrides, role: 'admin' });
}

/**
 * Generate guest user test data
 * @param overrides - Optional overrides
 * @returns UserTestData object
 */
export function generateGuestUserData(overrides: Partial<UserTestData> = {}): UserTestData {
  return generateUserData({ ...overrides, role: 'guest' });
}

/**
 * DNS provider test data
 */
export interface DNSProviderTestData {
  name: string;
  type: 'manual' | 'cloudflare' | 'route53' | 'webhook' | 'rfc2136';
  credentials?: Record<string, string>;
}

/**
 * Generate DNS provider test data with unique name
 * @param providerType - Type of DNS provider
 * @param overrides - Optional overrides for default values
 * @returns DNSProviderTestData object
 */
export function generateDNSProviderData(
  providerType: DNSProviderTestData['type'] = 'manual',
  overrides: Partial<DNSProviderTestData> = {}
): DNSProviderTestData {
  const id = generateUniqueId();
  const baseData: DNSProviderTestData = {
    name: `DNS-${providerType}-${id}`,
    type: providerType,
  };

  // Add type-specific credentials
  switch (providerType) {
    case 'cloudflare':
      baseData.credentials = {
        api_token: `test-token-${id}`,
      };
      break;
    case 'route53':
      baseData.credentials = {
        access_key_id: `AKIATEST${id.toUpperCase()}`,
        secret_access_key: `secretkey${id}`,
        region: 'us-east-1',
      };
      break;
    case 'webhook':
      baseData.credentials = {
        create_url: `https://example.com/dns/${id}/create`,
        delete_url: `https://example.com/dns/${id}/delete`,
      };
      break;
    case 'rfc2136':
      baseData.credentials = {
        nameserver: 'ns.example.com:53',
        tsig_key_name: `ddns-${id}.example.com`,
        tsig_key: 'base64-encoded-key==',
        tsig_algorithm: 'hmac-sha256',
      };
      break;
    case 'manual':
    default:
      baseData.credentials = {};
      break;
  }

  return { ...baseData, ...overrides };
}

/**
 * CrowdSec decision test data
 */
export interface CrowdSecDecisionTestData {
  ip: string;
  duration: string;
  reason: string;
  scope: 'ip' | 'range' | 'country';
}

/**
 * Generate CrowdSec decision test data
 * @param overrides - Optional overrides for default values
 * @returns CrowdSecDecisionTestData object
 */
export function generateCrowdSecDecisionData(
  overrides: Partial<CrowdSecDecisionTestData> = {}
): CrowdSecDecisionTestData {
  return {
    ip: `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}`,
    duration: '4h',
    reason: 'Test ban - automated testing',
    scope: 'ip',
    ...overrides,
  };
}

/**
 * Rate limit rule test data
 */
export interface RateLimitRuleTestData {
  name: string;
  requests: number;
  window: string;
  action: 'block' | 'throttle';
}

/**
 * Generate rate limit rule test data
 * @param overrides - Optional overrides for default values
 * @returns RateLimitRuleTestData object
 */
export function generateRateLimitRuleData(
  overrides: Partial<RateLimitRuleTestData> = {}
): RateLimitRuleTestData {
  const id = generateUniqueId();
  return {
    name: `RateLimit-${id}`,
    requests: 100,
    window: '1m',
    action: 'block',
    ...overrides,
  };
}

/**
 * Backup test data
 */
export interface BackupTestData {
  name: string;
  includeConfig: boolean;
  includeCertificates: boolean;
  includeDatabase: boolean;
}

/**
 * Generate backup test data
 * @param overrides - Optional overrides for default values
 * @returns BackupTestData object
 */
export function generateBackupData(
  overrides: Partial<BackupTestData> = {}
): BackupTestData {
  const id = generateUniqueId();
  return {
    name: `Backup-${id}`,
    includeConfig: true,
    includeCertificates: true,
    includeDatabase: true,
    ...overrides,
  };
}
