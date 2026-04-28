import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import {
  listAgents,
  provisionAgent,
  getAgent,
  deleteAgent,
  revokeAgent,
  getInstallSnippets,
} from '../orthrus'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockAgent = {
  uuid: 'agent-uuid',
  name: 'My Agent',
  status: 'online' as const,
  capabilities: '["proxy"]',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

describe('orthrus API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    Object.defineProperty(window, 'location', {
      value: { origin: 'http://localhost:8080' },
      writable: true,
    })
  })

  describe('listAgents', () => {
    it('calls correct endpoint and returns agents', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [mockAgent] })

      const result = await listAgents()

      expect(client.get).toHaveBeenCalledWith('/orthrus/agents')
      expect(result).toEqual([mockAgent])
    })

    it('propagates errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('network error'))
      await expect(listAgents()).rejects.toThrow('network error')
    })
  })

  describe('provisionAgent', () => {
    it('posts to agents endpoint and returns agent + auth_key', async () => {
      const response = { agent: mockAgent, auth_key: 'plain-text-key' }
      vi.mocked(client.post).mockResolvedValue({ data: response })

      const result = await provisionAgent({ name: 'My Agent' })

      expect(client.post).toHaveBeenCalledWith('/orthrus/agents', { name: 'My Agent' })
      expect(result.agent).toEqual(mockAgent)
      expect(result.auth_key).toBe('plain-text-key')
    })

    it('propagates errors', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('provision failed'))
      await expect(provisionAgent({ name: 'x' })).rejects.toThrow('provision failed')
    })
  })

  describe('getAgent', () => {
    it('calls correct endpoint with uuid', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockAgent })

      const result = await getAgent('agent-uuid')

      expect(client.get).toHaveBeenCalledWith('/orthrus/agents/agent-uuid')
      expect(result).toEqual(mockAgent)
    })
  })

  describe('deleteAgent', () => {
    it('calls DELETE with uuid', async () => {
      vi.mocked(client.delete).mockResolvedValue({ data: undefined })

      await deleteAgent('agent-uuid')

      expect(client.delete).toHaveBeenCalledWith('/orthrus/agents/agent-uuid')
    })

    it('propagates errors', async () => {
      vi.mocked(client.delete).mockRejectedValue(new Error('delete failed'))
      await expect(deleteAgent('agent-uuid')).rejects.toThrow('delete failed')
    })
  })

  describe('revokeAgent', () => {
    it('calls POST to revoke endpoint', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { message: 'revoked' } })

      const result = await revokeAgent('agent-uuid')

      expect(client.post).toHaveBeenCalledWith('/orthrus/agents/agent-uuid/revoke')
      expect(result.message).toBe('revoked')
    })
  })

  describe('getInstallSnippets', () => {
    it('calls snippets endpoint with X-Charon-URL header', async () => {
      const snippets = {
        docker_compose: 'compose-snippet',
        systemd: 'systemd-snippet',
        tarball: 'tarball-snippet',
        homebrew: 'brew-snippet',
        kubernetes_daemonset: 'k8s-snippet',
      }
      vi.mocked(client.get).mockResolvedValue({ data: snippets })

      const result = await getInstallSnippets('agent-uuid')

      expect(client.get).toHaveBeenCalledWith('/orthrus/agents/agent-uuid/snippets', {
        headers: { 'X-Charon-URL': 'http://localhost:8080' },
      })
      expect(result).toEqual(snippets)
    })

    it('propagates errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('snippets failed'))
      await expect(getInstallSnippets('agent-uuid')).rejects.toThrow('snippets failed')
    })
  })
})
