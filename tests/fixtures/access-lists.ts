/**
 * Access List (ACL) Test Fixtures
 *
 * Mock data for Access List E2E tests.
 * Provides various ACL configurations for testing CRUD operations,
 * rule management, and validation scenarios.
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

/**
 * ACL rule types
 */
export type ACLRuleType = 'allow' | 'deny';

/**
 * Single ACL rule configuration
 */
export interface ACLRule {
  /** Rule type: allow or deny */
  type: ACLRuleType;
  /** Value: IP, CIDR range, or special value */
  value: string;
  /** Optional description */
  description?: string;
}

/**
 * Complete access list configuration
 */
export interface AccessListConfig {
  /** Access list name */
  name: string;
  /** List of rules */
  rules: ACLRule[];
  /** Optional description */
  description?: string;
  /** Enable/disable authentication */
  authEnabled?: boolean;
  /** Authentication users (if authEnabled) */
  authUsers?: Array<{ username: string; password: string }>;
  /** Enable/disable the access list */
  enabled?: boolean;
}

/**
 * Empty access list
 * No rules defined - useful for testing empty state
 */
export const emptyAccessList: AccessListConfig = {
  name: 'Empty ACL',
  rules: [],
  description: 'Access list with no rules',
};

/**
 * Allow-only access list
 * Only contains allow rules
 */
export const allowOnlyAccessList: AccessListConfig = {
  name: 'Allow Only ACL',
  rules: [
    { type: 'allow', value: '192.168.1.0/24', description: 'Local network' },
    { type: 'allow', value: '10.0.0.0/8', description: 'Private network' },
    { type: 'allow', value: '172.16.0.0/12', description: 'Docker network' },
  ],
  description: 'Access list with only allow rules',
};

/**
 * Deny-only access list
 * Only contains deny rules (blacklist)
 */
export const denyOnlyAccessList: AccessListConfig = {
  name: 'Deny Only ACL',
  rules: [
    { type: 'deny', value: '192.168.100.0/24', description: 'Blocked subnet' },
    { type: 'deny', value: '10.255.0.1', description: 'Specific blocked IP' },
    { type: 'deny', value: '203.0.113.0/24', description: 'TEST-NET-3' },
  ],
  description: 'Access list with only deny rules',
};

/**
 * Mixed rules access list
 * Contains both allow and deny rules - order matters
 */
export const mixedRulesAccessList: AccessListConfig = {
  name: 'Mixed Rules ACL',
  rules: [
    { type: 'allow', value: '192.168.1.100', description: 'Allowed specific IP' },
    { type: 'deny', value: '192.168.1.0/24', description: 'Deny rest of subnet' },
    { type: 'allow', value: '10.0.0.0/8', description: 'Allow internal' },
    { type: 'deny', value: '0.0.0.0/0', description: 'Deny all others' },
  ],
  description: 'Access list with mixed allow/deny rules',
};

/**
 * Allow all access list
 * Single rule to allow all traffic
 */
export const allowAllAccessList: AccessListConfig = {
  name: 'Allow All ACL',
  rules: [{ type: 'allow', value: '0.0.0.0/0', description: 'Allow all' }],
  description: 'Access list that allows all traffic',
};

/**
 * Deny all access list
 * Single rule to deny all traffic
 */
export const denyAllAccessList: AccessListConfig = {
  name: 'Deny All ACL',
  rules: [{ type: 'deny', value: '0.0.0.0/0', description: 'Deny all' }],
  description: 'Access list that denies all traffic',
};

/**
 * Access list with basic authentication
 * Requires username/password
 */
export const authEnabledAccessList: AccessListConfig = {
  name: 'Auth Enabled ACL',
  rules: [{ type: 'allow', value: '0.0.0.0/0' }],
  description: 'Access list with basic auth requirement',
  authEnabled: true,
  authUsers: [
    { username: 'testuser', password: 'TestPass123!' },
    { username: 'admin', password: 'AdminPass456!' },
  ],
};

/**
 * Access list with single IP
 * Most restrictive - only one IP allowed
 */
export const singleIPAccessList: AccessListConfig = {
  name: 'Single IP ACL',
  rules: [
    { type: 'allow', value: '192.168.1.50', description: 'Only allowed IP' },
    { type: 'deny', value: '0.0.0.0/0', description: 'Block all others' },
  ],
  description: 'Access list for single IP address',
};

/**
 * Access list with many rules
 * For testing performance and UI with large lists
 */
export const manyRulesAccessList: AccessListConfig = {
  name: 'Many Rules ACL',
  rules: Array.from({ length: 50 }, (_, i) => ({
    type: (i % 2 === 0 ? 'allow' : 'deny') as ACLRuleType,
    value: `10.${Math.floor(i / 256)}.${i % 256}.0/24`,
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
  rules: [
    { type: 'allow', value: '::1', description: 'Localhost IPv6' },
    { type: 'allow', value: 'fe80::/10', description: 'Link-local' },
    { type: 'allow', value: '2001:db8::/32', description: 'Documentation range' },
    { type: 'deny', value: '::/0', description: 'Deny all IPv6' },
  ],
  description: 'Access list with IPv6 rules',
};

/**
 * Disabled access list
 * For testing enable/disable functionality
 */
export const disabledAccessList: AccessListConfig = {
  name: 'Disabled ACL',
  rules: [{ type: 'deny', value: '0.0.0.0/0' }],
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
    rules: [{ type: 'allow' as const, value: '192.168.1.0/24' }],
  },

  /** Name too long */
  nameTooLong: {
    name: 'A'.repeat(256),
    rules: [{ type: 'allow' as const, value: '192.168.1.0/24' }],
  },

  /** Invalid rule type */
  invalidRuleType: {
    name: 'Invalid Type ACL',
    rules: [{ type: 'maybe' as ACLRuleType, value: '192.168.1.0/24' }],
  },

  /** Invalid IP address */
  invalidIP: {
    name: 'Invalid IP ACL',
    rules: [{ type: 'allow' as const, value: '999.999.999.999' }],
  },

  /** Invalid CIDR */
  invalidCIDR: {
    name: 'Invalid CIDR ACL',
    rules: [{ type: 'allow' as const, value: '192.168.1.0/99' }],
  },

  /** Empty rule value */
  emptyRuleValue: {
    name: 'Empty Value ACL',
    rules: [{ type: 'allow' as const, value: '' }],
  },

  /** XSS in name */
  xssInName: {
    name: '<script>alert(1)</script>',
    rules: [{ type: 'allow' as const, value: '192.168.1.0/24' }],
  },

  /** SQL injection in name */
  sqlInjectionInName: {
    name: "'; DROP TABLE access_lists; --",
    rules: [{ type: 'allow' as const, value: '192.168.1.0/24' }],
  },

  /** XSS in rule value */
  xssInRuleValue: {
    name: 'XSS Rule ACL',
    rules: [{ type: 'allow' as const, value: '<img src=x onerror=alert(1)>' }],
  },

  /** Duplicate rules */
  duplicateRules: {
    name: 'Duplicate Rules ACL',
    rules: [
      { type: 'allow' as const, value: '192.168.1.0/24' },
      { type: 'allow' as const, value: '192.168.1.0/24' },
    ],
  },

  /** Conflicting rules */
  conflictingRules: {
    name: 'Conflicting Rules ACL',
    rules: [
      { type: 'allow' as const, value: '192.168.1.100' },
      { type: 'deny' as const, value: '192.168.1.100' },
    ],
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
 * const acl = generateAccessList({ authEnabled: true });
 * ```
 */
export function generateAccessList(
  overrides: Partial<AccessListConfig> = {}
): AccessListConfig {
  const id = generateUniqueId();
  return {
    name: `ACL-${id}`,
    rules: [
      { type: 'allow', value: generateCIDR(24) },
      { type: 'deny', value: '0.0.0.0/0' },
    ],
    description: `Generated access list ${id}`,
    ...overrides,
  };
}

/**
 * Generate access list with specific IPs allowed
 * @param allowedIPs - Array of IP addresses to allow
 * @param denyOthers - Whether to add a deny-all rule at the end
 * @returns AccessListConfig
 */
export function generateAllowListForIPs(
  allowedIPs: string[],
  denyOthers: boolean = true
): AccessListConfig {
  const rules: ACLRule[] = allowedIPs.map((ip) => ({
    type: 'allow' as const,
    value: ip,
  }));

  if (denyOthers) {
    rules.push({ type: 'deny', value: '0.0.0.0/0' });
  }

  return {
    name: `AllowList-${generateUniqueId()}`,
    rules,
    description: `Allow list for ${allowedIPs.length} IPs`,
  };
}

/**
 * Generate access list with specific IPs denied
 * @param deniedIPs - Array of IP addresses to deny
 * @param allowOthers - Whether to add an allow-all rule at the end
 * @returns AccessListConfig
 */
export function generateDenyListForIPs(
  deniedIPs: string[],
  allowOthers: boolean = true
): AccessListConfig {
  const rules: ACLRule[] = deniedIPs.map((ip) => ({
    type: 'deny' as const,
    value: ip,
  }));

  if (allowOthers) {
    rules.push({ type: 'allow', value: '0.0.0.0/0' });
  }

  return {
    name: `DenyList-${generateUniqueId()}`,
    rules,
    description: `Deny list for ${deniedIPs.length} IPs`,
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
 * Expected API response for access list creation
 */
export interface AccessListAPIResponse {
  id: string;
  name: string;
  rules: ACLRule[];
  description?: string;
  auth_enabled: boolean;
  enabled: boolean;
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
    id,
    name: config.name || `ACL-${id}`,
    rules: config.rules || [],
    description: config.description,
    auth_enabled: config.authEnabled || false,
    enabled: config.enabled !== false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}
