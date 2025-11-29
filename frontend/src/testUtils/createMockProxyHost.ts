import { ProxyHost } from '../api/proxyHosts'

export const createMockProxyHost = (overrides: Partial<ProxyHost> = {}): ProxyHost => ({
  uuid: 'host-1',
  name: 'Host',
  domain_names: 'example.com',
  forward_host: '127.0.0.1',
  forward_port: 8080,
  forward_scheme: 'http',
  enabled: true,
  ssl_forced: false,
  websocket_support: false,
  http2_support: false,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  application: 'none',
  locations: [],
  certificate: null,
  access_list_id: null,
  certificate_id: null,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  ...overrides,
})

export default createMockProxyHost
