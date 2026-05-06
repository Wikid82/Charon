import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import {
  getTunnelStatus,
  listTunnels,
  createTunnel,
  getTunnel,
  updateTunnel,
  deleteTunnel,
  startTunnel,
  stopTunnel,
  rotateCredentials,
  listCloudflareTunnels,
  getCloudflaredConfig,
  listTailscaleDevices,
  listZeroTierNetworks,
  listZeroTierMembers,
  listNetBirdPeers,
  syncNetBird,
  connectTunnelLogs,
} from '../hecate'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('hecate API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    Object.defineProperty(window, 'location', {
      value: { protocol: 'http:', host: 'localhost:8080' },
      writable: true,
    })
  })

  describe('getTunnelStatus', () => {
    it('calls correct endpoint and returns data', async () => {
      const mockStatuses = [{ uuid: 'abc', name: 'cf-tunnel', provider: 'cloudflare', state: 'connected', uptime_seconds: 100, last_error: '' }]
      vi.mocked(client.get).mockResolvedValue({ data: mockStatuses })

      const result = await getTunnelStatus()

      expect(client.get).toHaveBeenCalledWith('/hecate/status')
      expect(result).toEqual(mockStatuses)
    })

    it('propagates errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('network error'))
      await expect(getTunnelStatus()).rejects.toThrow('network error')
    })
  })

  describe('listTunnels', () => {
    it('calls correct endpoint and returns data', async () => {
      const mockTunnels = [{ uuid: 'abc', name: 'cf', provider: 'cloudflare', configuration: '{}', is_active: true, created_at: '', updated_at: '' }]
      vi.mocked(client.get).mockResolvedValue({ data: mockTunnels })

      const result = await listTunnels()

      expect(client.get).toHaveBeenCalledWith('/hecate/tunnels')
      expect(result).toEqual(mockTunnels)
    })

    it('propagates errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('server error'))
      await expect(listTunnels()).rejects.toThrow('server error')
    })
  })

  describe('createTunnel', () => {
    it('posts to correct endpoint with payload', async () => {
      const req = { name: 'My Tunnel', provider: 'cloudflare' as const, credentials: 'tok', configuration: '{}', is_active: true }
      const created = { uuid: 'new-uuid', ...req, created_at: '', updated_at: '' }
      vi.mocked(client.post).mockResolvedValue({ data: created })

      const result = await createTunnel(req)

      expect(client.post).toHaveBeenCalledWith('/hecate/tunnels', req)
      expect(result.uuid).toBe('new-uuid')
    })

    it('propagates errors', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('create failed'))
      await expect(createTunnel({ name: 'x', provider: 'tailscale', credentials: 'y' })).rejects.toThrow('create failed')
    })
  })

  describe('getTunnel', () => {
    it('calls correct endpoint with uuid', async () => {
      const mockTunnel = { uuid: 'abc', name: 'cf', provider: 'cloudflare' as const, configuration: '{}', is_active: true, created_at: '', updated_at: '' }
      vi.mocked(client.get).mockResolvedValue({ data: mockTunnel })

      const result = await getTunnel('abc')

      expect(client.get).toHaveBeenCalledWith('/hecate/tunnels/abc')
      expect(result).toEqual(mockTunnel)
    })
  })

  describe('updateTunnel', () => {
    it('calls PUT with uuid and payload', async () => {
      vi.mocked(client.put).mockResolvedValue({ data: { message: 'updated' } })

      const result = await updateTunnel('abc', { name: 'New Name', provider: 'tailscale' })

      expect(client.put).toHaveBeenCalledWith('/hecate/tunnels/abc', { name: 'New Name', provider: 'tailscale' })
      expect(result.message).toBe('updated')
    })
  })

  describe('deleteTunnel', () => {
    it('calls DELETE with uuid', async () => {
      vi.mocked(client.delete).mockResolvedValue({ data: undefined })

      await deleteTunnel('abc')

      expect(client.delete).toHaveBeenCalledWith('/hecate/tunnels/abc')
    })

    it('propagates errors', async () => {
      vi.mocked(client.delete).mockRejectedValue(new Error('delete failed'))
      await expect(deleteTunnel('abc')).rejects.toThrow('delete failed')
    })
  })

  describe('startTunnel', () => {
    it('calls POST to start endpoint', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { message: 'started' } })

      const result = await startTunnel('abc')

      expect(client.post).toHaveBeenCalledWith('/hecate/tunnels/abc/start')
      expect(result.message).toBe('started')
    })
  })

  describe('stopTunnel', () => {
    it('calls POST to stop endpoint', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { message: 'stopped' } })

      const result = await stopTunnel('abc')

      expect(client.post).toHaveBeenCalledWith('/hecate/tunnels/abc/stop')
      expect(result.message).toBe('stopped')
    })
  })

  describe('rotateCredentials', () => {
    it('calls POST to rotate endpoint with credentials', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { message: 'rotated' } })

      const result = await rotateCredentials('abc', 'new-creds')

      expect(client.post).toHaveBeenCalledWith('/hecate/tunnels/abc/rotate-credentials', { credentials: 'new-creds' })
      expect(result.message).toBe('rotated')
    })
  })

  describe('listCloudflareTunnels', () => {
    it('calls correct endpoint', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [{ id: 'cf1', name: 'tunnel1', status: 'active', created_at: '' }] })

      const result = await listCloudflareTunnels()

      expect(client.get).toHaveBeenCalledWith('/hecate/cloudflare/tunnels')
      expect(result[0].id).toBe('cf1')
    })
  })

  describe('getCloudflaredConfig', () => {
    it('calls correct endpoint with uuid', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: 'yaml-config' })

      const result = await getCloudflaredConfig('abc')

      expect(client.get).toHaveBeenCalledWith('/hecate/tunnels/abc/config/cloudflared')
      expect(result).toBe('yaml-config')
    })
  })

  describe('listTailscaleDevices', () => {
    it('calls correct endpoint', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [{ id: 'ts1', hostname: 'box', addresses: [], os: 'linux', last_seen: '', online: true }] })

      const result = await listTailscaleDevices()

      expect(client.get).toHaveBeenCalledWith('/hecate/tailscale/devices')
      expect(result[0].id).toBe('ts1')
    })
  })

  describe('listZeroTierNetworks', () => {
    it('calls correct endpoint', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [{ id: 'zt1', name: 'net', description: '', private: true, total_member_count: 2 }] })

      const result = await listZeroTierNetworks()

      expect(client.get).toHaveBeenCalledWith('/hecate/zerotier/networks')
      expect(result[0].id).toBe('zt1')
    })
  })

  describe('listZeroTierMembers', () => {
    it('calls correct endpoint with networkId', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [{ node_id: 'n1', name: 'peer', description: '', ip_assignments: [], authorized: true, online: true }] })

      const result = await listZeroTierMembers('net123')

      expect(client.get).toHaveBeenCalledWith('/hecate/zerotier/networks/net123/members')
      expect(result[0].node_id).toBe('n1')
    })
  })

  describe('listNetBirdPeers', () => {
    it('calls correct endpoint', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [{ id: 'nb1', name: 'peer1', ip: '10.0.0.1', os: 'linux', connection_state: 'connected', last_seen: '', online: true }] })

      const result = await listNetBirdPeers()

      expect(client.get).toHaveBeenCalledWith('/hecate/netbird/peers')
      expect(result[0].id).toBe('nb1')
    })
  })

  describe('syncNetBird', () => {
    it('calls POST to sync endpoint', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: [] })

      const result = await syncNetBird()

      expect(client.post).toHaveBeenCalledWith('/hecate/netbird/sync')
      expect(result).toEqual([])
    })
  })

  describe('connectTunnelLogs', () => {
    let mockWs: { url: string; onmessage: ((e: MessageEvent) => void) | null }

    beforeEach(() => {
      mockWs = { url: '', onmessage: null }
      ;(globalThis as any).WebSocket = class {
        url: string
        onmessage: ((e: MessageEvent) => void) | null = null
        constructor(url: string) {
          this.url = url
          mockWs = this as typeof mockWs
        }
        close() {}
      }
    })

    it('constructs ws URL with correct uuid', () => {
      connectTunnelLogs('my-uuid', vi.fn())
      expect(mockWs.url).toBe('ws://localhost:8080/api/v1/ws/hecate/logs/my-uuid')
    })

    it('uses wss when page is https', () => {
      Object.defineProperty(window, 'location', {
        value: { protocol: 'https:', host: 'example.com' },
        writable: true,
      })
      connectTunnelLogs('my-uuid', vi.fn())
      expect(mockWs.url).toBe('wss://example.com/api/v1/ws/hecate/logs/my-uuid')
    })

    it('calls onMessage when a message is received', () => {
      const onMessage = vi.fn()
      connectTunnelLogs('my-uuid', onMessage)

      const event = new MessageEvent('message', { data: 'log line 1' })
      mockWs.onmessage?.(event)

      expect(onMessage).toHaveBeenCalledWith('log line 1')
    })

    it('returns the WebSocket instance', () => {
      const ws = connectTunnelLogs('my-uuid', vi.fn())
      expect(ws).toBeDefined()
    })
  })
})
