import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import * as crowdsec from '../crowdsec'

vi.mock('../client')

describe('crowdsec API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('startCrowdsec', () => {
    it('should call POST /admin/crowdsec/start', async () => {
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await crowdsec.startCrowdsec()

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/start')
      expect(result).toEqual(mockData)
    })
  })

  describe('stopCrowdsec', () => {
    it('should call POST /admin/crowdsec/stop', async () => {
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await crowdsec.stopCrowdsec()

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/stop')
      expect(result).toEqual(mockData)
    })
  })

  describe('statusCrowdsec', () => {
    it('should call GET /admin/crowdsec/status', async () => {
      const mockData = { running: true, pid: 1234 }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await crowdsec.statusCrowdsec()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/status')
      expect(result).toEqual(mockData)
    })
  })

  describe('importCrowdsecConfig', () => {
    it('should call POST /admin/crowdsec/import with FormData', async () => {
      const mockFile = new File(['content'], 'config.tar.gz', { type: 'application/gzip' })
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await crowdsec.importCrowdsecConfig(mockFile)

      expect(client.post).toHaveBeenCalledWith(
        '/admin/crowdsec/import',
        expect.any(FormData),
        { headers: { 'Content-Type': 'multipart/form-data' } }
      )
      expect(result).toEqual(mockData)
    })
  })

  describe('exportCrowdsecConfig', () => {
    it('should call GET /admin/crowdsec/export with blob responseType', async () => {
      const mockBlob = new Blob(['data'], { type: 'application/gzip' })
      vi.mocked(client.get).mockResolvedValue({ data: mockBlob })

      const result = await crowdsec.exportCrowdsecConfig()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/export', { responseType: 'blob' })
      expect(result).toEqual(mockBlob)
    })
  })

  describe('listCrowdsecFiles', () => {
    it('should call GET /admin/crowdsec/files', async () => {
      const mockData = { files: ['file1.yaml', 'file2.yaml'] }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await crowdsec.listCrowdsecFiles()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/files')
      expect(result).toEqual(mockData)
    })
  })

  describe('readCrowdsecFile', () => {
    it('should call GET /admin/crowdsec/file with encoded path', async () => {
      const mockData = { content: 'file content' }
      const path = '/etc/crowdsec/file.yaml'
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await crowdsec.readCrowdsecFile(path)

      expect(client.get).toHaveBeenCalledWith(
        `/admin/crowdsec/file?path=${encodeURIComponent(path)}`
      )
      expect(result).toEqual(mockData)
    })
  })

  describe('writeCrowdsecFile', () => {
    it('should call POST /admin/crowdsec/file with path and content', async () => {
      const mockData = { success: true }
      const path = '/etc/crowdsec/file.yaml'
      const content = 'new content'
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await crowdsec.writeCrowdsecFile(path, content)

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/file', { path, content })
      expect(result).toEqual(mockData)
    })
  })

  describe('listCrowdsecDecisions', () => {
    it('should call GET /admin/crowdsec/decisions and return data', async () => {
      const mockData = {
        decisions: [
          { id: '1', ip: '1.2.3.4', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'crowdsec' },
        ],
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await crowdsec.listCrowdsecDecisions()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/decisions')
      expect(result).toEqual(mockData)
    })
  })

  describe('banIP', () => {
    it('should call POST /admin/crowdsec/ban with ip, duration, and reason', async () => {
      vi.mocked(client.post).mockResolvedValue({})

      await crowdsec.banIP('1.2.3.4', '24h', 'manual ban')

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/ban', {
        ip: '1.2.3.4',
        duration: '24h',
        reason: 'manual ban',
      })
    })
  })

  describe('unbanIP', () => {
    it('should call DELETE /admin/crowdsec/ban/{encoded ip}', async () => {
      vi.mocked(client.delete).mockResolvedValue({})

      await crowdsec.unbanIP('1.2.3.4')

      expect(client.delete).toHaveBeenCalledWith('/admin/crowdsec/ban/1.2.3.4')
    })

    it('should URL-encode special characters in the IP', async () => {
      vi.mocked(client.delete).mockResolvedValue({})

      await crowdsec.unbanIP('::1')

      expect(client.delete).toHaveBeenCalledWith('/admin/crowdsec/ban/%3A%3A1')
    })
  })

  describe('getCrowdsecKeyStatus', () => {
    it('should call GET /admin/crowdsec/key-status and return data', async () => {
      const mockData = {
        key_source: 'file' as const,
        env_key_rejected: false,
        current_key_preview: 'abc***xyz',
        message: 'Key loaded from file',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await crowdsec.getCrowdsecKeyStatus()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/key-status')
      expect(result).toEqual(mockData)
    })
  })

  describe('listWhitelists', () => {
    it('should call GET /admin/crowdsec/whitelist and return the whitelist array', async () => {
      const mockWhitelist = [
        {
          uuid: 'uuid-1',
          ip_or_cidr: '192.168.1.1',
          reason: 'Home',
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ]
      vi.mocked(client.get).mockResolvedValue({ data: { whitelist: mockWhitelist } })

      const result = await crowdsec.listWhitelists()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/whitelist')
      expect(result).toEqual(mockWhitelist)
    })
  })

  describe('addWhitelist', () => {
    it('should call POST /admin/crowdsec/whitelist and return the created entry', async () => {
      const payload = { ip_or_cidr: '192.168.1.1', reason: 'Home' }
      const mockEntry = {
        uuid: 'uuid-1',
        ip_or_cidr: '192.168.1.1',
        reason: 'Home',
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockEntry })

      const result = await crowdsec.addWhitelist(payload)

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/whitelist', payload)
      expect(result).toEqual(mockEntry)
    })
  })

  describe('deleteWhitelist', () => {
    it('should call DELETE /admin/crowdsec/whitelist/{uuid}', async () => {
      vi.mocked(client.delete).mockResolvedValue({})

      await crowdsec.deleteWhitelist('uuid-1')

      expect(client.delete).toHaveBeenCalledWith('/admin/crowdsec/whitelist/uuid-1')
    })
  })

  describe('default export', () => {
    it('should export all functions', () => {
      expect(crowdsec.default).toHaveProperty('startCrowdsec')
      expect(crowdsec.default).toHaveProperty('stopCrowdsec')
      expect(crowdsec.default).toHaveProperty('statusCrowdsec')
      expect(crowdsec.default).toHaveProperty('importCrowdsecConfig')
      expect(crowdsec.default).toHaveProperty('exportCrowdsecConfig')
      expect(crowdsec.default).toHaveProperty('listCrowdsecFiles')
      expect(crowdsec.default).toHaveProperty('readCrowdsecFile')
      expect(crowdsec.default).toHaveProperty('writeCrowdsecFile')
      expect(crowdsec.default).toHaveProperty('listCrowdsecDecisions')
      expect(crowdsec.default).toHaveProperty('banIP')
      expect(crowdsec.default).toHaveProperty('unbanIP')
      expect(crowdsec.default).toHaveProperty('getCrowdsecKeyStatus')
      expect(crowdsec.default).toHaveProperty('listWhitelists')
      expect(crowdsec.default).toHaveProperty('addWhitelist')
      expect(crowdsec.default).toHaveProperty('deleteWhitelist')
    })
  })

})
