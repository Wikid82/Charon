import { describe, it, expect } from 'vitest'
import compareHosts from '../compareHosts'
import type { ProxyHost } from '../../api/proxyHosts'

const hostA: ProxyHost = {
  uuid: 'a',
  name: 'Alpha',
  domain_names: 'alpha.com',
  forward_host: '127.0.0.1',
  forward_port: 80,
  forward_scheme: 'http',
  enabled: true,
  ssl_forced: false,
  websocket_support: false,
  certificate: null,
  http2_support: false,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  application: 'none',
  locations: [],
  created_at: '2025-01-01',
  updated_at: '2025-01-01',
}

const hostB: ProxyHost = {
  uuid: 'b',
  name: 'Beta',
  domain_names: 'beta.com',
  forward_host: '127.0.0.2',
  forward_port: 8080,
  forward_scheme: 'http',
  enabled: true,
  ssl_forced: false,
  websocket_support: false,
  certificate: null,
  http2_support: false,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  application: 'none',
  locations: [],
  created_at: '2025-01-01',
  updated_at: '2025-01-01',
}

describe('compareHosts', () => {
  it('returns 0 for unknown sort column (default case)', () => {
    const compareAny = compareHosts as unknown as (a: ProxyHost, b: ProxyHost, sortColumn: string, sortDirection: 'asc' | 'desc') => number
    const res = compareAny(hostA, hostB, 'unknown', 'asc')
    expect(res).toBe(0)
  })

  it('sorts by name', () => {
    expect(compareHosts(hostA, hostB, 'name', 'asc')).toBeLessThan(0)
    expect(compareHosts(hostB, hostA, 'name', 'asc')).toBeGreaterThan(0)
  })

  it('sorts by domain', () => {
    expect(compareHosts(hostA, hostB, 'domain', 'asc')).toBeLessThan(0)
  })

  it('sorts by forward', () => {
    expect(compareHosts(hostA, hostB, 'forward', 'asc')).toBeLessThan(0)
  })
})
