/**
 * Access List (ACL) Test Fixtures
 *
 * Mock data for Access List E2E tests.
 * Provides various ACL configurations for testing CRUD operations,
 * rule management, and validation scenarios.
 *
 * The backend expects AccessList with:
 * - type: 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist'
 * - ip_rules: JSON string of {cidr, description} objects
 * - country_codes: comma-separated ISO country codes (for geo types)
 *
 * @example
 * ```typescript
 * import { emptyAccessList, allowOnlyAccessList, invalidACLConfigs } from './fixtures/access-lists';
 *
 * test('create access list with allow rules', async ({ testData }) => {
 *   const { id } = await testData.createAccessList(allowOnlyAccessList);
 * });
 * ```
 */

import { generateUniqueId, generateIPAddress, generateCIDR } from './test-data';
import type { AccessListData } from '../utils/TestDataManager';
import * as crypto from 'crypto';

/**
 * Generate an unbiased random integer in [0, 9999] using rejection sampling.
 * Avoids modulo bias from 16-bit source.
 */
function getRandomIntBelow10000(): number {
  const maxExclusive = 10000;
  const limit = 60000; // Largest multiple of 10000 <= 65535
  while (true) {
    const value = crypto.randomBytes(2).readUInt16BE(0);
    if (value < limit) {
      return value % maxExclusive;
    }
  }
}

/**
 * ACL type - matches backend ValidAccessListTypes
 */
export type ACLType = 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist';

/**
 * Single ACL IP rule configuration (matches backend AccessListRule)
 */
export interface ACLRule {
  /** CIDR notation IP or range */
  cidr: string;
  /** Optional description */
  description?: string;
}

/**
 * Complete access list configuration (matches backend AccessList model)
 */
export interface AccessListConfig extends AccessListData {
  /** Optional description */
  description?: string;
}

/**
 * Empty access list (whitelist with no rules)
 * Useful for testing empty state
 */
export const emptyAccessList: AccessListConfig = {
  name: 'Empty ACL',
  type: 'whitelist',
  ipRules: [],
  description: 'Access list with no rules',
};

/**
 * Allow-only access list (whitelist)
 * Contains CIDR ranges for private networks
 */
export const allowOnlyAccessList: AccessListConfig = {
  name: 'Allow Only ACL',
  type: 'whitelist',
  ipRules: [
    { cidr: '192.168.1.0/24', description: 'Local network' },
    { cidr: '10.0.0.0/8', description: 'Private network' },
    { cidr: '172.16.0.0/12', description: 'Docker network' },
  ],
  description: 'Access list with only allow rules',
};

/**
 * Deny-only access list (blacklist)
 * Blocks specific IP ranges
 */
export const denyOnlyAccessList: AccessListConfig = {
  name: 'Deny Only ACL',
  type: 'blacklist',
  ipRules: [
    { cidr: '192.168.100.0/24', description: 'Blocked subnet' },
    { cidr: '10.255.0.1/32', description: 'Specific blocked IP' },
    { cidr: '203.0.113.0/24', description: 'TEST-NET-3' },
  ],
  description: 'Access list with deny rules (blacklist)',
};

/**
 * Whitelist for specific IPs
 * Only allows traffic from specific IP ranges
 */
export const mixedRulesAccessList: AccessListConfig = {
  name: 'Mixed Rules ACL',
  type: 'whitelist',
  ipRules: [
    { cidr: '192.168.1.100/32', description: 'Allowed specific IP' },
    { cidr: '10.0.0.0/8', description: 'Allow internal' },
  ],
  description: 'Access list with whitelisted IPs',
};

/**
 * Allow all access list (whitelist with 0.0.0.0/0)
 */
export const allowAllAccessList: AccessListConfig = {
  name: 'Allow All ACL',
  type: 'whitelist',
  ipRules: [{ cidr: '0.0.0.0/0', description: 'Allow all' }],
  description: 'Access list that allows all traffic',
};

/**
 * Deny all access list (blacklist with 0.0.0.0/0)
 */
export const denyAllAccessList: AccessListConfig = {
  name: 'Deny All ACL',
  type: 'blacklist',
  ipRules: [{ cidr: '0.0.0.0/0', description: 'Deny all' }],
  description: 'Access list that denies all traffic',
};

/**
 * Geo whitelist with country codes
 * Only allows traffic from specific countries
 */
export const geoWhitelistAccessList: AccessListConfig = {
  name: 'Geo Whitelist ACL',
  type: 'geo_whitelist',
  countryCodes: 'US,CA,GB',
  description: 'Access list allowing only US, Canada, and UK',
};

/**
 * Geo blacklist with country codes
 * Blocks traffic from specific countries
 */
export const geoBlacklistAccessList: AccessListConfig = {
  name: 'Geo Blacklist ACL',
  type: 'geo_blacklist',
  countryCodes: 'CN,RU,KP',
  description: 'Access list blocking China, Russia, and North Korea',
};

/**
 * Local network only access list
 * Restricts to RFC1918 private networks
 */
export const localNetworkAccessList: AccessListConfig = {
  name: 'Local Network ACL',
  type: 'whitelist',
  localNetworkOnly: true,
  description: 'Access list restricted to local/private networks',
};

/**
 * Single IP access list
 * Most restrictive - only one IP allowed
 */
export const singleIPAccessList: AccessListConfig = {
  name: 'Single IP ACL',
  type: 'whitelist',
  ipRules: [{ cidr: '192.168.1.50/32', description: 'Only allowed IP' }],
  description: 'Access list for single IP address',
};

/**
 * Access list with many rules
 * For testing performance and UI with large lists
 */
export const manyRulesAccessList: AccessListConfig = {
  name: 'Many Rules ACL',
  type: 'whitelist',
  ipRules: Array.from({ length: 50 }, (_, i) => ({
    cidr: `10.${Math.floor(i / 256)}.${i % 256}.0/24`,
    description: `Rule ${i + 1}`,
  })),
  description: 'Access list with many rules for stress testing',
};

/**
 * IPv6 access list
 * Contains IPv6 addresses
 */
export const ipv6AccessList: AccessListConfig = {
  name: 'IPv6 ACL',
  type: 'whitelist',
  ipRules: [
    { cidr: '::1/128', description: 'Localhost IPv6' },
    { cidr: 'fe80::/10', description: 'Link-local' },
    { cidr: '2001:db8::/32', description: 'Documentation range' },
  ],
  description: 'Access list with IPv6 rules',
};

/**
 * Disabled access list
 * For testing enable/disable functionality
 */
export const disabledAccessList: AccessListConfig = {
  name: 'Disabled ACL',
  type: 'blacklist',
  ipRules: [{ cidr: '0.0.0.0/0' }],
  description: 'Disabled access list',
  enabled: false,
};

/**
 * Invalid ACL configurations for validation testing
 */
export const invalidACLConfigs = {
  /** Empty name */
  emptyName: {
    name: '',
    type: 'whitelist' as const,
    ipRules: [{ cidr: '192.168.1.0/24' }],
  },

  /** Name too long */
  nameTooLong: {
    name: 'A'.repeat(256),
    type: 'whitelist' as const,
    ipRules: [{ cidr: '192.168.1.0/24' }],
  },

  /** Invalid type */
  invalidType: {
    name: 'Invalid Type ACL',
    type: 'invalid_type' as ACLType,
    ipRules: [{ cidr: '192.168.1.0/24' }],
  },

  /** Invalid IP address */
  invalidIP: {
    name: 'Invalid IP ACL',
    type: 'whitelist' as const,
    ipRules: [{ cidr: '999.999.999.999' }],
  },

  /** Invalid CIDR */
  invalidCIDR: {
    name: 'Invalid CIDR ACL',
    type: 'whitelist' as const,
    ipRules: [{ cidr: '192.168.1.0/99' }],
  },

  /** Empty CIDR value */
  emptyCIDR: {
    name: 'Empty CIDR ACL',
    type: 'whitelist' as const,
    ipRules: [{ cidr: '' }],
  },

  /** XSS in name */
  xssInName: {
    name: '<script>alert(1)</script>',
    type: 'whitelist' as const,
    ipRules: [{ cidr: '192.168.1.0/24' }],
  },

  /** SQL injection in name */
  sqlInjectionInName: {
    name: "'; DROP TABLE access_lists; --",
    type: 'whitelist' as const,
    ipRules: [{ cidr: '192.168.1.0/24' }],
  },

  /** Geo type without country codes */
  geoWithoutCountryCodes: {
    name: 'Geo No Countries ACL',
    type: 'geo_whitelist' as const,
    countryCodes: '',
  },

  /** Invalid country code */
  invalidCountryCode: {
    name: 'Invalid Country ACL',
    type: 'geo_whitelist' as const,
    countryCodes: 'XX,YY,ZZ',
  },
};

/**
 * Generate a unique access list configuration
 * Creates an ACL with unique name to avoid conflicts
 * @param overrides - Optional configuration overrides
 * @returns AccessListConfig with unique name
 *
 * @example
 * ```typescript
 * const acl = generateAccessList({ type: 'blacklist' });
 * ```
 */
export function generateAccessList(
  overrides: Partial<AccessListConfig> = {}
): AccessListConfig {
  const id = generateUniqueId();
  return {
    name: `ACL-${id}`,
    type: 'whitelist',
    ipRules: [
      { cidr: generateCIDR(24) },
    ],
    description: `Generated access list ${id}`,
    ...overrides,
  };
}

/**
 * Generate whitelist for specific IPs
 * @param allowedIPs - Array of IP/CIDR addresses to whitelist
 * @returns AccessListConfig
 */
export function generateAllowListForIPs(allowedIPs: string[]): AccessListConfig {
  return {
    name: `AllowList-${generateUniqueId()}`,
    type: 'whitelist',
    ipRules: allowedIPs.map((ip) => ({
      cidr: ip.includes('/') ? ip : `${ip}/32`,
    })),
    description: `Whitelist for ${allowedIPs.length} IPs`,
  };
}

/**
 * Generate blacklist for specific IPs
 * @param deniedIPs - Array of IP/CIDR addresses to blacklist
 * @returns AccessListConfig
 */
export function generateDenyListForIPs(deniedIPs: string[]): AccessListConfig {
  return {
    name: `DenyList-${generateUniqueId()}`,
    type: 'blacklist',
    ipRules: deniedIPs.map((ip) => ({
      cidr: ip.includes('/') ? ip : `${ip}/32`,
    })),
    description: `Blacklist for ${deniedIPs.length} IPs`,
  };
}

/**
 * Generate multiple unique access lists
 * @param count - Number of access lists to generate
 * @param overrides - Optional configuration overrides for all lists
 * @returns Array of AccessListConfig
 */
export function generateAccessLists(
  count: number,
  overrides: Partial<AccessListConfig> = {}
): AccessListConfig[] {
  return Array.from({ length: count }, () => generateAccessList(overrides));
}

/**
 * Expected API response for access list (matches backend AccessList model)
 */
export interface AccessListAPIResponse {
  id: number;
  uuid: string;
  name: string;
  type: ACLType;
  ip_rules: string;
  country_codes: string;
  local_network_only: boolean;
  enabled: boolean;
  description: string;
  created_at: string;
  updated_at: string;
}

/**
 * Mock API response for testing
 */
export function mockAccessListResponse(
  config: Partial<AccessListConfig> = {}
): AccessListAPIResponse {
  const id = generateUniqueId();
  return {
    id: parseInt(id) || getRandomIntBelow10000(),
    uuid: `acl-${id}`,
    name: config.name || `ACL-${id}`,
    type: config.type || 'whitelist',
    ip_rules: config.ipRules ? JSON.stringify(config.ipRules) : '[]',
    country_codes: config.countryCodes || '',
    local_network_only: config.localNetworkOnly || false,
    enabled: config.enabled !== false,
    description: config.description || '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}
