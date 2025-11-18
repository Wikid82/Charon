import { ProxyHost } from '../hooks/useProxyHosts'
import { RemoteServer } from '../hooks/useRemoteServers'

export const mockProxyHosts: ProxyHost[] = [
  {
    uuid: '123e4567-e89b-12d3-a456-426614174000',
    domain_names: 'app.local.dev',
    forward_scheme: 'http',
    forward_host: 'localhost',
    forward_port: 3000,
    access_list_id: undefined,
    certificate_id: undefined,
    ssl_forced: false,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: true,
    advanced_config: undefined,
    enabled: true,
    created_at: '2025-11-18T10:00:00Z',
    updated_at: '2025-11-18T10:00:00Z',
  },
  {
    uuid: '223e4567-e89b-12d3-a456-426614174001',
    domain_names: 'api.local.dev',
    forward_scheme: 'http',
    forward_host: '192.168.1.100',
    forward_port: 8080,
    access_list_id: undefined,
    certificate_id: undefined,
    ssl_forced: false,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: false,
    advanced_config: undefined,
    enabled: true,
    created_at: '2025-11-18T10:00:00Z',
    updated_at: '2025-11-18T10:00:00Z',
  },
]

export const mockRemoteServers: RemoteServer[] = [
  {
    uuid: '323e4567-e89b-12d3-a456-426614174002',
    name: 'Local Docker Registry',
    provider: 'docker',
    host: 'localhost',
    port: 5000,
    username: undefined,
    enabled: true,
    reachable: false,
    last_check: undefined,
    created_at: '2025-11-18T10:00:00Z',
    updated_at: '2025-11-18T10:00:00Z',
  },
  {
    uuid: '423e4567-e89b-12d3-a456-426614174003',
    name: 'Development API Server',
    provider: 'generic',
    host: '192.168.1.100',
    port: 8080,
    username: undefined,
    enabled: true,
    reachable: true,
    last_check: '2025-11-18T10:00:00Z',
    created_at: '2025-11-18T10:00:00Z',
    updated_at: '2025-11-18T10:00:00Z',
  },
]

export const mockImportPreview = {
  hosts: [
    {
      domain_names: 'test.example.com',
      forward_scheme: 'http',
      forward_host: 'localhost',
      forward_port: 8080,
      ssl_forced: true,
      http2_support: true,
      websocket_support: false,
    },
  ],
  conflicts: ['app.local.dev'],
  errors: [],
}
