/**
 * DNS Provider Test Fixtures
 *
 * Shared test data for DNS Provider E2E tests.
 * These fixtures provide consistent test data across test files.
 */

import * as crypto from 'crypto';

/**
 * Expected provider types from the API
 */
export const mockProviderTypes = {
  /** Built-in providers from Lego/Caddy */
  built_in: [
    'cloudflare',
    'route53',
    'digitalocean',
    'googleclouddns',
    'azuredns',
    'godaddy',
    'namecheap',
    'hetzner',
    'vultr',
    'dnsimple',
  ],
  /** Custom providers from Phase 2 */
  custom: ['manual', 'webhook', 'rfc2136', 'script'],
};

/**
 * Mock provider data for creating test providers
 */
export const mockCloudflareProvider = {
  name: 'Test Cloudflare',
  provider_type: 'cloudflare',
  credentials: {
    api_token: 'test-token-12345',
  },
};

export const mockManualProvider = {
  name: 'Test Manual Provider',
  provider_type: 'manual',
  credentials: {},
};

export const mockWebhookProvider = {
  name: 'Test Webhook Provider',
  provider_type: 'webhook',
  credentials: {
    create_url: 'https://example.com/dns/create',
    delete_url: 'https://example.com/dns/delete',
  },
};

export const mockRfc2136Provider = {
  name: 'Test RFC2136 Provider',
  provider_type: 'rfc2136',
  credentials: {
    nameserver: 'ns.example.com:53',
    tsig_key_name: 'ddns.example.com',
    tsig_key: 'base64-encoded-key==',
    tsig_algorithm: 'hmac-sha256',
  },
};

export const mockScriptProvider = {
  name: 'Test Script Provider',
  provider_type: 'script',
  credentials: {
    script_path: '/usr/local/bin/dns-update.sh',
  },
};

/**
 * Mock API responses for testing
 */
export const mockTypesResponse = {
  types: [
    {
      type: 'cloudflare',
      name: 'Cloudflare',
      description: 'DNS provider using Cloudflare API',
      fields: [
        {
          name: 'api_token',
          label: 'API Token',
          type: 'password',
          required: true,
        },
      ],
    },
    {
      type: 'manual',
      name: 'Manual (No Automation)',
      description: 'Manual DNS record creation - no automation',
      fields: [],
    },
    {
      type: 'webhook',
      name: 'Webhook (HTTP)',
      description: 'DNS provider using HTTP webhooks',
      fields: [
        {
          name: 'create_url',
          label: 'Create URL',
          type: 'url',
          required: true,
        },
        {
          name: 'delete_url',
          label: 'Delete URL',
          type: 'url',
          required: true,
        },
      ],
    },
  ],
  total: 3,
};

/**
 * Mock manual DNS challenge data
 */
export const mockManualChallenge = {
  id: 1,
  provider_id: 1,
  fqdn: '_acme-challenge.example.com',
  value: 'mock-challenge-token-value-abc123',
  status: 'pending',
  ttl: 300,
  expires_at: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
  created_at: new Date().toISOString(),
  dns_propagated: false,
  last_check_at: null,
};

export const mockExpiredChallenge = {
  ...mockManualChallenge,
  id: 2,
  status: 'expired',
  expires_at: new Date(Date.now() - 60000).toISOString(),
  created_at: new Date(Date.now() - 11 * 60 * 1000).toISOString(),
};

export const mockVerifiedChallenge = {
  ...mockManualChallenge,
  id: 3,
  status: 'verified',
  dns_propagated: true,
};

/**
 * Helper function to create a mock provider via API
 */
export async function createTestProvider(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  providerData: typeof mockManualProvider
): Promise<{ id: number; uuid: string }> {
  const response = await request.post('/api/v1/dns-providers', {
    data: providerData,
  });

  if (!response.ok()) {
    throw new Error('Failed to create test provider');
  }

  return response.json() as Promise<{ id: number; uuid: string }>;
}

/**
 * Helper function to clean up test provider
 */
export async function deleteTestProvider(
  request: { delete: (url: string) => Promise<unknown> },
  providerId: number
): Promise<void> {
  await request.delete(`/api/v1/dns-providers/${providerId}`);
}

/**
 * DNS provider type options
 */
export type DnsProviderType = 'cloudflare' | 'manual' | 'route53' | 'webhook' | 'rfc2136' | 'script' | 'digitalocean' | 'googleclouddns' | 'azuredns' | 'godaddy' | 'namecheap' | 'hetzner' | 'vultr' | 'dnsimple';

/**
 * DNS provider configuration interface
 */
export interface DnsProviderConfig {
  /** Provider name */
  name: string;
  /** Provider type */
  provider_type: DnsProviderType;
  /** Provider credentials (type-specific) */
  credentials: Record<string, string>;
  /** Optional description */
  description?: string;
  /** Enable/disable the provider */
  enabled?: boolean;
}

/**
 * Generate a unique DNS provider configuration
 * Creates a DNS provider with unique name to avoid conflicts
 * @param overrides - Optional configuration overrides
 * @returns DnsProviderConfig with unique name
 *
 * @example
 * ```typescript
 * const provider = generateDnsProvider({ provider_type: 'cloudflare' });
 * ```
 */
export function generateDnsProvider(
  overrides: Partial<DnsProviderConfig> = {}
): DnsProviderConfig {
  const timestamp = Date.now().toString(36);
  const random = crypto.randomBytes(4).toString('hex');
  const uniqueId = `${timestamp}-${random}`;
  const providerType = overrides.provider_type || 'manual';

  // Generate type-specific credentials
  let credentials: Record<string, string> = {};
  switch (providerType) {
    case 'cloudflare':
      credentials = { api_token: `test-token-${uniqueId}` };
      break;
    case 'route53':
      credentials = {
        access_key_id: `AKIATEST${uniqueId.toUpperCase()}`,
        secret_access_key: `secretkey${uniqueId}`,
        region: 'us-east-1',
      };
      break;
    case 'webhook':
      credentials = {
        create_url: `https://example.com/dns/${uniqueId}/create`,
        delete_url: `https://example.com/dns/${uniqueId}/delete`,
      };
      break;
    case 'rfc2136':
      credentials = {
        nameserver: 'ns.example.com:53',
        tsig_key_name: `ddns-${uniqueId}.example.com`,
        tsig_key: 'base64-encoded-key==',
        tsig_algorithm: 'hmac-sha256',
      };
      break;
    case 'script':
      credentials = { script_path: '/usr/local/bin/dns-update.sh' };
      break;
    case 'digitalocean':
      credentials = { api_token: `do-token-${uniqueId}` };
      break;
    case 'manual':
    default:
      credentials = {};
      break;
  }

  return {
    name: `DNS-${providerType}-${uniqueId}`,
    provider_type: providerType,
    credentials,
    ...overrides,
  };
}

/**
 * Generate multiple unique DNS providers
 * @param count - Number of DNS providers to generate
 * @param overrides - Optional configuration overrides for all providers
 * @returns Array of DnsProviderConfig
 */
export function generateDnsProviders(
  count: number,
  overrides: Partial<DnsProviderConfig> = {}
): DnsProviderConfig[] {
  return Array.from({ length: count }, () => generateDnsProvider(overrides));
}
