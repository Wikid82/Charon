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

import * as crypto from 'crypto';

/**
 * Generate a unique identifier with optional prefix
 * Uses timestamp + high-resolution time + random bytes for maximum uniqueness
 * @param prefix - Optional prefix for the ID
 * @returns Unique identifier string
 *
 * @example
 * ```typescript
 * const id = generateUniqueId('test');
 * // Returns: "test-m1abc123-0042-deadbeef"
 * ```
 */
export function generateUniqueId(prefix = ''): string {
  const timestamp = Date.now().toString(36);
  // Add high-resolution time component for nanosecond-level uniqueness
  const hrTime = process.hrtime.bigint().toString(36).slice(-4);
  const random = crypto.randomBytes(4).toString('hex');
  return prefix ? `${prefix}-${timestamp}-${hrTime}-${random}` : `${timestamp}-${hrTime}-${random}`;
}

/**
 * Generate a unique UUID v4
 * @returns UUID string
 *
 * @example
 * ```typescript
 * const uuid = generateUUID();
 * // Returns: "550e8400-e29b-41d4-a716-446655440000"
 * ```
 */
export function generateUUID(): string {
  return crypto.randomUUID();
}

/**
 * Generate a valid private IP address (RFC 1918)
 * Uses 10.x.x.x range to avoid conflicts with local networks
 * @param options - Configuration options
 * @returns Valid IP address string
 *
 * @example
 * ```typescript
 * const ip = generateIPAddress();
 * // Returns: "10.42.128.15"
 *
 * const specificRange = generateIPAddress({ octet2: 100 });
 * // Returns: "10.100.x.x"
 * ```
 */
export function generateIPAddress(options: {
  /** Second octet (0-255), random if not specified */
  octet2?: number;
  /** Third octet (0-255), random if not specified */
  octet3?: number;
  /** Fourth octet (1-254), random if not specified */
  octet4?: number;
} = {}): string {
  const o2 = options.octet2 ?? secureRandomInt(256);
  const o3 = options.octet3 ?? secureRandomInt(256);
  const o4 = options.octet4 ?? secureRandomInt(253) + 1; // 1-254
  return `10.${o2}.${o3}.${o4}`;
}

/**
 * Generate a valid CIDR range
 * @param prefix - CIDR prefix (8-32)
 * @returns Valid CIDR string
 *
 * @example
 * ```typescript
 * const cidr = generateCIDR(24);
 * // Returns: "10.42.128.0/24"
 * ```
 */
export function generateCIDR(prefix: number = 24): string {
  const ip = generateIPAddress({ octet4: 0 });
  return `${ip}/${prefix}`;
}

/**
 * Generate a valid port number
 * @param options - Configuration options
 * @returns Valid port number
 *
 * @example
 * ```typescript
 * const port = generatePort();
 * // Returns: 8080-65000 range
 *
 * const specificRange = generatePort({ min: 3000, max: 4000 });
 * // Returns: 3000-4000 range
 * ```
 */
export function generatePort(options: {
  /** Minimum port (default: 8080) */
  min?: number;
  /** Maximum port (default: 65000) */
  max?: number;
} = {}): number {
  const { min = 8080, max = 65000 } = options;
  return secureRandomInt(max - min + 1) + min;
}

/**
 * Common ports for testing purposes
 */
export const commonPorts = {
  http: 80,
  https: 443,
  ssh: 22,
  mysql: 3306,
  postgres: 5432,
  redis: 6379,
  mongodb: 27017,
  node: 3000,
  react: 3000,
  vite: 5173,
  backend: 8080,
  proxy: 8081,
} as const;

/**
 * Generate a password that meets common requirements
 * - At least 12 characters
 * - Contains uppercase, lowercase, numbers, and special characters
 * @param length - Password length (default: 16)
 * @returns Strong password string
 *
 * @example
 * ```typescript
 * const password = generatePassword();
 * // Returns: "Xy7!kM2@pL9#nQ4$"
 * ```
 */
function secureRandomInt(maxExclusive: number): number {
  if (maxExclusive <= 0) {
    throw new Error('maxExclusive must be positive');
  }
  const maxUint32 = 0xffffffff;
  const limit = maxUint32 - ((maxUint32 + 1) % maxExclusive);
  while (true) {
    const random = crypto.randomBytes(4).readUInt32BE(0);
    if (random <= limit) {
      return random % maxExclusive;
    }
  }
}

function shuffleArraySecure<T>(arr: T[]): T[] {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = secureRandomInt(i + 1);
    const tmp = arr[i];
    arr[i] = arr[j];
    arr[j] = tmp;
  }
  return arr;
}

export function generatePassword(length: number = 16): string {
  const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const lower = 'abcdefghijklmnopqrstuvwxyz';
  const numbers = '0123456789';
  const special = '!@#$%^&*';
  const all = upper + lower + numbers + special;

  // Ensure at least one of each type
  let password = '';
  password += upper[secureRandomInt(upper.length)];
  password += lower[secureRandomInt(lower.length)];
  password += numbers[secureRandomInt(numbers.length)];
  password += special[secureRandomInt(special.length)];

  // Fill the rest randomly
  for (let i = 4; i < length; i++) {
    password += all[secureRandomInt(all.length)];
  }

  // Shuffle the password using a cryptographically secure shuffle
  return shuffleArraySecure(password.split('')).join('');
}

/**
 * Common test passwords for different scenarios
 */
export const testPasswords = {
  /** Valid strong password */
  valid: 'TestPass123!',
  /** Another valid password */
  validAlt: 'SecureP@ss456',
  /** Too short */
  tooShort: 'Ab1!',
  /** No uppercase */
  noUppercase: 'password123!',
  /** No lowercase */
  noLowercase: 'PASSWORD123!',
  /** No numbers */
  noNumbers: 'Password!!',
  /** No special characters */
  noSpecial: 'Password123',
  /** Common weak password */
  weak: 'password',
} as const;

/**
 * Counter for additional uniqueness within same millisecond
 */
let domainCounter = 0;

/**
 * Generate a unique domain name for testing
 * Uses timestamp + counter for uniqueness while keeping length reasonable
 * @param subdomain - Optional subdomain prefix
 * @returns Unique domain string guaranteed to be unique even in rapid calls
 */
export function generateDomain(subdomain = 'app'): string {
  // Increment counter and wrap at 9999 to keep domain lengths reasonable
  domainCounter = (domainCounter + 1) % 10000;
  // Use shorter ID format: base36 timestamp (7 chars) + random (4 chars)
  const timestamp = Date.now().toString(36).slice(-7);
  const random = crypto.randomBytes(2).toString('hex');
  // Format: subdomain-timestamp-random-counter.test.local
  // Example: proxy-1abc123-a1b2-1.test.local (max ~35 chars)
  return `${subdomain}-${timestamp}-${random}-${domainCounter}.test.local`;
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
    ip: `10.0.${secureRandomInt(255)}.${secureRandomInt(255)}`,
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
