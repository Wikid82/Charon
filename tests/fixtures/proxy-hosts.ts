/**
 * Proxy Host Test Fixtures
 *
 * Mock data for proxy host E2E tests.
 * Provides various configurations for testing CRUD operations,
 * validation, and edge cases.
 *
 * @example
 * ```typescript
 * import { basicProxyHost, proxyHostWithSSL, invalidProxyHosts } from './fixtures/proxy-hosts';
 *
 * test('create basic proxy host', async ({ testData }) => {
 *   const { id } = await testData.createProxyHost(basicProxyHost);
 * });
 * ```
 */

import {
  generateDomain,
  generateIPAddress,
  generatePort,
  generateUniqueId,
} from './test-data';

/**
 * Proxy host configuration interface
 */
export interface ProxyHostConfig {
  /** Domain name for the proxy */
  domain: string;
  /** Target hostname or IP */
  forwardHost: string;
  /** Target port */
  forwardPort: number;
  /** Friendly name for the proxy host */
  name?: string;
  /** Protocol scheme */
  scheme: 'http' | 'https';
  /** Enable WebSocket support */
  websocketSupport: boolean;
  /** Enable HTTP/2 */
  http2Support?: boolean;
  /** Enable gzip compression */
  gzipEnabled?: boolean;
  /** Custom headers */
  customHeaders?: Record<string, string>;
  /** Access list ID (if applicable) */
  accessListId?: string;
  /** Certificate ID (if applicable) */
  certificateId?: string;
  /** Enable HSTS */
  hstsEnabled?: boolean;
  /** Force SSL redirect */
  forceSSL?: boolean;
  /** Block exploits */
  blockExploits?: boolean;
  /** Custom Caddy configuration */
  advancedConfig?: string;
  /** Enable/disable the proxy */
  enabled?: boolean;
}

/**
 * Basic proxy host configuration
 * Minimal setup for simple HTTP proxying
 */
export const basicProxyHost: ProxyHostConfig = {
  domain: 'basic-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
};

/**
 * Proxy host with SSL enabled
 * Uses HTTPS scheme and forces SSL redirect
 */
export const proxyHostWithSSL: ProxyHostConfig = {
  domain: 'secure-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 443,
  scheme: 'https',
  websocketSupport: false,
  hstsEnabled: true,
  forceSSL: true,
};

/**
 * Proxy host with WebSocket support
 * For real-time applications
 */
export const proxyHostWithWebSocket: ProxyHostConfig = {
  domain: 'ws-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: true,
};

/**
 * Proxy host with full security features
 * Includes SSL, HSTS, exploit blocking
 */
export const proxyHostFullSecurity: ProxyHostConfig = {
  domain: 'secure-full.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 8080,
  scheme: 'https',
  websocketSupport: true,
  http2Support: true,
  hstsEnabled: true,
  forceSSL: true,
  blockExploits: true,
  gzipEnabled: true,
};

/**
 * Proxy host with custom headers
 * For testing header injection
 */
export const proxyHostWithHeaders: ProxyHostConfig = {
  domain: 'headers-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
  customHeaders: {
    'X-Custom-Header': 'test-value',
    'X-Forwarded-Proto': 'https',
    'X-Real-IP': '{remote_host}',
  },
};

/**
 * Proxy host with access list
 * Placeholder for ACL integration testing
 */
export const proxyHostWithAccessList: ProxyHostConfig = {
  domain: 'restricted-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
  accessListId: '', // Will be set dynamically in tests
};

/**
 * Proxy host with advanced Caddy configuration
 * For testing custom configuration injection
 */
export const proxyHostWithAdvancedConfig: ProxyHostConfig = {
  domain: 'advanced-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
  advancedConfig: `
    header {
      -Server
      X-Robots-Tag "noindex, nofollow"
    }
    request_body {
      max_size 10MB
    }
  `,
};

/**
 * Disabled proxy host
 * For testing enable/disable functionality
 */
export const disabledProxyHost: ProxyHostConfig = {
  domain: 'disabled-app.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
  enabled: false,
};

/**
 * Docker container proxy host
 * Uses container name as forward host
 */
export const dockerProxyHost: ProxyHostConfig = {
  domain: 'docker-app.example.com',
  forwardHost: 'my-container',
  forwardPort: 80,
  scheme: 'http',
  websocketSupport: false,
};

/**
 * Wildcard domain proxy host
 * For subdomain proxying
 */
export const wildcardProxyHost: ProxyHostConfig = {
  domain: '*.apps.example.com',
  forwardHost: '192.168.1.100',
  forwardPort: 3000,
  scheme: 'http',
  websocketSupport: false,
};

/**
 * Invalid proxy host configurations for validation testing
 */
export const invalidProxyHosts = {
  /** Empty domain */
  emptyDomain: {
    domain: '',
    forwardHost: '192.168.1.100',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Invalid domain format */
  invalidDomain: {
    domain: 'not a valid domain!',
    forwardHost: '192.168.1.100',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Empty forward host */
  emptyForwardHost: {
    domain: 'valid.example.com',
    forwardHost: '',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Invalid IP address */
  invalidIP: {
    domain: 'valid.example.com',
    forwardHost: '999.999.999.999',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Port out of range (too low) */
  portTooLow: {
    domain: 'valid.example.com',
    forwardHost: '192.168.1.100',
    forwardPort: 0,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Port out of range (too high) */
  portTooHigh: {
    domain: 'valid.example.com',
    forwardHost: '192.168.1.100',
    forwardPort: 70000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** Negative port */
  negativePort: {
    domain: 'valid.example.com',
    forwardHost: '192.168.1.100',
    forwardPort: -1,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** XSS in domain */
  xssInDomain: {
    domain: '<script>alert(1)</script>.example.com',
    forwardHost: '192.168.1.100',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },

  /** SQL injection in domain */
  sqlInjectionInDomain: {
    domain: "'; DROP TABLE proxy_hosts; --",
    forwardHost: '192.168.1.100',
    forwardPort: 3000,
    scheme: 'http' as const,
    websocketSupport: false,
  },
};

/**
 * Generate a unique proxy host configuration
 * Creates a proxy host with unique domain to avoid conflicts
 * @param overrides - Optional configuration overrides
 * @returns ProxyHostConfig with unique domain
 *
 * @example
 * ```typescript
 * const host = generateProxyHost({ websocketSupport: true });
 * ```
 */
export function generateProxyHost(
  overrides: Partial<ProxyHostConfig> = {}
): ProxyHostConfig {
  const domain = generateDomain('proxy');
  return {
    domain,
    forwardHost: generateIPAddress(),
    forwardPort: generatePort({ min: 3000, max: 9000 }),
    name: `Test Host ${Date.now()}`,
    scheme: 'http',
    websocketSupport: false,
    ...overrides,
  };
}

/**
 * Generate multiple unique proxy hosts
 * @param count - Number of proxy hosts to generate
 * @param overrides - Optional configuration overrides for all hosts
 * @returns Array of ProxyHostConfig
 */
export function generateProxyHosts(
  count: number,
  overrides: Partial<ProxyHostConfig> = {}
): ProxyHostConfig[] {
  return Array.from({ length: count }, () => generateProxyHost(overrides));
}

/**
 * Proxy host for load balancing tests
 * Multiple backends configuration
 */
export const loadBalancedProxyHost = {
  domain: 'lb-app.example.com',
  backends: [
    { host: '192.168.1.101', port: 3000, weight: 1 },
    { host: '192.168.1.102', port: 3000, weight: 1 },
    { host: '192.168.1.103', port: 3000, weight: 2 },
  ],
  scheme: 'http' as const,
  websocketSupport: false,
  healthCheck: {
    path: '/health',
    interval: '10s',
    timeout: '5s',
  },
};

/**
 * Expected API response for proxy host creation
 */
export interface ProxyHostAPIResponse {
  id: string;
  uuid: string;
  domain: string;
  forward_host: string;
  forward_port: number;
  scheme: string;
  websocket_support: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Mock API response for testing
 */
export function mockProxyHostResponse(
  config: Partial<ProxyHostConfig> = {}
): ProxyHostAPIResponse {
  const id = generateUniqueId();
  return {
    id,
    uuid: `uuid-${id}`,
    domain: config.domain || generateDomain('proxy'),
    forward_host: config.forwardHost || '192.168.1.100',
    forward_port: config.forwardPort || 3000,
    scheme: config.scheme || 'http',
    websocket_support: config.websocketSupport || false,
    enabled: config.enabled !== false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}
