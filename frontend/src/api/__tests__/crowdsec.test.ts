import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as crowdsec from '../crowdsec'
import client from '../client'

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
    })
  })
})
