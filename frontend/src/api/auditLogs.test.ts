import { describe, it, expect, vi, beforeEach } from 'vitest'
import client from './client'
import {
  getAuditLogs,
  getAuditLog,
  getAuditLogsByProvider,
  exportAuditLogsCSV,
  type AuditLog,
  type AuditLogFilters,
} from './auditLogs'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
}

describe('auditLogs api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getAuditLogs', () => {
    it('fetches audit logs with default pagination', async () => {
      const mockResponse = {
        logs: [
          {
            id: 1,
            uuid: 'log-1',
            actor: 'admin',
            action: 'user_login',
            event_category: 'user',
            details: 'User logged in',
            ip_address: '192.168.1.1',
            created_at: '2024-01-01T00:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        limit: 50,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await getAuditLogs()

      expect(mockedClient.get).toHaveBeenCalledWith('/audit-logs?page=1&limit=50')
      expect(result).toEqual(mockResponse)
      expect(result.logs).toHaveLength(1)
      expect(result.logs[0].uuid).toBe('log-1')
    })

    it('fetches audit logs with custom pagination', async () => {
      const mockResponse = {
        logs: [],
        total: 100,
        page: 3,
        limit: 25,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await getAuditLogs(undefined, 3, 25)

      expect(mockedClient.get).toHaveBeenCalledWith('/audit-logs?page=3&limit=25')
      expect(result.page).toBe(3)
      expect(result.limit).toBe(25)
    })

    it('fetches audit logs with all filters', async () => {
      const filters: AuditLogFilters = {
        event_category: 'dns_provider',
        actor: 'admin',
        action: 'dns_provider_create',
        start_date: '2024-01-01',
        end_date: '2024-12-31',
        resource_uuid: 'resource-123',
      }
      const mockResponse = {
        logs: [],
        total: 0,
        page: 1,
        limit: 50,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      await getAuditLogs(filters)

      expect(mockedClient.get).toHaveBeenCalledWith(
        '/audit-logs?page=1&limit=50&event_category=dns_provider&actor=admin&action=dns_provider_create&start_date=2024-01-01&end_date=2024-12-31&resource_uuid=resource-123'
      )
    })

    it('fetches audit logs with partial filters', async () => {
      const filters: AuditLogFilters = {
        event_category: 'certificate',
        start_date: '2024-01-01',
      }
      const mockResponse = {
        logs: [],
        total: 5,
        page: 1,
        limit: 50,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      await getAuditLogs(filters, 1, 50)

      expect(mockedClient.get).toHaveBeenCalledWith(
        '/audit-logs?page=1&limit=50&event_category=certificate&start_date=2024-01-01'
      )
    })

    it('handles errors when fetching audit logs', async () => {
      const error = new Error('Network error')
      mockedClient.get.mockRejectedValueOnce(error)

      await expect(getAuditLogs()).rejects.toThrow('Network error')
    })
  })

  describe('getAuditLog', () => {
    it('fetches a single audit log by UUID', async () => {
      const mockLog: AuditLog = {
        id: 42,
        uuid: 'log-uuid-123',
        actor: 'admin',
        action: 'certificate_issue',
        event_category: 'certificate',
        resource_id: 10,
        resource_uuid: 'cert-uuid',
        details: 'Certificate issued successfully',
        ip_address: '10.0.0.1',
        user_agent: 'Mozilla/5.0',
        created_at: '2024-06-15T12:30:00Z',
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockLog })

      const result = await getAuditLog('log-uuid-123')

      expect(mockedClient.get).toHaveBeenCalledWith('/audit-logs/log-uuid-123')
      expect(result).toEqual(mockLog)
      expect(result.uuid).toBe('log-uuid-123')
      expect(result.action).toBe('certificate_issue')
    })

    it('handles 404 when audit log not found', async () => {
      const error = new Error('Not found')
      mockedClient.get.mockRejectedValueOnce(error)

      await expect(getAuditLog('nonexistent')).rejects.toThrow('Not found')
    })
  })

  describe('getAuditLogsByProvider', () => {
    it('fetches audit logs for a specific DNS provider with default pagination', async () => {
      const mockResponse = {
        logs: [
          {
            id: 5,
            uuid: 'log-5',
            actor: 'system',
            action: 'dns_provider_update',
            event_category: 'dns_provider',
            resource_id: 123,
            details: 'DNS provider updated',
            created_at: '2024-03-15T10:00:00Z',
          },
        ],
        total: 10,
        page: 1,
        limit: 50,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await getAuditLogsByProvider(123)

      expect(mockedClient.get).toHaveBeenCalledWith('/dns-providers/123/audit-logs?page=1&limit=50')
      expect(result.logs).toHaveLength(1)
      expect(result.logs[0].action).toBe('dns_provider_update')
    })

    it('fetches audit logs for a provider with custom pagination', async () => {
      const mockResponse = {
        logs: [],
        total: 25,
        page: 2,
        limit: 10,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await getAuditLogsByProvider(456, 2, 10)

      expect(mockedClient.get).toHaveBeenCalledWith('/dns-providers/456/audit-logs?page=2&limit=10')
      expect(result.page).toBe(2)
      expect(result.limit).toBe(10)
    })

    it('handles errors when fetching provider audit logs', async () => {
      const error = new Error('Provider not found')
      mockedClient.get.mockRejectedValueOnce(error)

      await expect(getAuditLogsByProvider(999)).rejects.toThrow('Provider not found')
    })
  })

  describe('exportAuditLogsCSV', () => {
    it('exports audit logs to CSV without filters', async () => {
      const mockCSV = 'id,actor,action,created_at\n1,admin,user_login,2024-01-01'
      mockedClient.get.mockResolvedValueOnce({ data: mockCSV })

      const result = await exportAuditLogsCSV()

      expect(mockedClient.get).toHaveBeenCalledWith(
        '/audit-logs/export?',
        { headers: { Accept: 'text/csv' } }
      )
      expect(result).toBe(mockCSV)
    })

    it('exports audit logs to CSV with all filters', async () => {
      const filters: AuditLogFilters = {
        event_category: 'proxy_host',
        actor: 'operator',
        action: 'proxy_host_delete',
        start_date: '2024-01-01',
        end_date: '2024-06-30',
        resource_uuid: 'host-uuid-456',
      }
      const mockCSV = 'id,actor,action,created_at\n'
      mockedClient.get.mockResolvedValueOnce({ data: mockCSV })

      const result = await exportAuditLogsCSV(filters)

      expect(mockedClient.get).toHaveBeenCalledWith(
        '/audit-logs/export?event_category=proxy_host&actor=operator&action=proxy_host_delete&start_date=2024-01-01&end_date=2024-06-30&resource_uuid=host-uuid-456',
        { headers: { Accept: 'text/csv' } }
      )
      expect(result).toBe(mockCSV)
    })

    it('exports audit logs with partial filters', async () => {
      const filters: AuditLogFilters = {
        action: 'settings_update',
        end_date: '2024-12-31',
      }
      const mockCSV = 'header,data\n'
      mockedClient.get.mockResolvedValueOnce({ data: mockCSV })

      await exportAuditLogsCSV(filters)

      expect(mockedClient.get).toHaveBeenCalledWith(
        '/audit-logs/export?action=settings_update&end_date=2024-12-31',
        { headers: { Accept: 'text/csv' } }
      )
    })

    it('handles errors when exporting audit logs', async () => {
      const error = new Error('Export failed')
      mockedClient.get.mockRejectedValueOnce(error)

      await expect(exportAuditLogsCSV()).rejects.toThrow('Export failed')
    })
  })
})
