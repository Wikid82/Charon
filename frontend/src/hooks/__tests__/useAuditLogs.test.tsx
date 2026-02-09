import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuditLogs, useAuditLog, useAuditLogsByProvider } from '../useAuditLogs'

// Mock the API module
vi.mock('../../api/auditLogs', () => ({
  getAuditLogs: vi.fn(),
  getAuditLog: vi.fn(),
  getAuditLogsByProvider: vi.fn(),
}))

import { getAuditLogs, getAuditLog, getAuditLogsByProvider } from '../../api/auditLogs'

const mockAuditLog = {
  id: 1,
  uuid: 'test-uuid-123',
  actor: 'admin@test.com',
  action: 'dns_provider_create' as const,
  event_category: 'dns_provider' as const,
  resource_id: 1,
  details: 'Created DNS provider',
  ip_address: '127.0.0.1',
  created_at: '2026-01-24T12:00:00Z',
}

const mockListResponse = {
  logs: [mockAuditLog],
  total: 1,
  page: 1,
  limit: 50,
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useAuditLogs hook', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches audit logs with default parameters', async () => {
    vi.mocked(getAuditLogs).mockResolvedValue(mockListResponse)

    const { result } = renderHook(() => useAuditLogs(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(getAuditLogs).toHaveBeenCalledWith(undefined, 1, 50)
    expect(result.current.data).toEqual(mockListResponse)
  })

  it('fetches audit logs with filters', async () => {
    vi.mocked(getAuditLogs).mockResolvedValue(mockListResponse)

    const filters = { event_category: 'dns_provider' as const }
    const { result } = renderHook(() => useAuditLogs(filters, 2, 25), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(getAuditLogs).toHaveBeenCalledWith(filters, 2, 25)
  })
})

describe('useAuditLog hook', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches a single audit log by UUID', async () => {
    vi.mocked(getAuditLog).mockResolvedValue(mockAuditLog)

    const { result } = renderHook(() => useAuditLog('test-uuid-123'), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(getAuditLog).toHaveBeenCalledWith('test-uuid-123')
    expect(result.current.data).toEqual(mockAuditLog)
  })

  it('does not fetch when uuid is null', () => {
    const { result } = renderHook(() => useAuditLog(null), { wrapper: createWrapper() })

    expect(result.current.fetchStatus).toBe('idle')
    expect(getAuditLog).not.toHaveBeenCalled()
  })
})

describe('useAuditLogsByProvider hook', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches audit logs for a provider', async () => {
    vi.mocked(getAuditLogsByProvider).mockResolvedValue(mockListResponse)

    const { result } = renderHook(() => useAuditLogsByProvider(123), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(getAuditLogsByProvider).toHaveBeenCalledWith(123, 1, 50)
    expect(result.current.data).toEqual(mockListResponse)
  })

  it('does not fetch when providerId is null', () => {
    const { result } = renderHook(() => useAuditLogsByProvider(null), { wrapper: createWrapper() })

    expect(result.current.fetchStatus).toBe('idle')
    expect(getAuditLogsByProvider).not.toHaveBeenCalled()
  })

  it('does not fetch when providerId is 0', () => {
    const { result } = renderHook(() => useAuditLogsByProvider(0), { wrapper: createWrapper() })

    expect(result.current.fetchStatus).toBe('idle')
    expect(getAuditLogsByProvider).not.toHaveBeenCalled()
  })

  it('fetches with custom pagination', async () => {
    vi.mocked(getAuditLogsByProvider).mockResolvedValue(mockListResponse)

    const { result } = renderHook(() => useAuditLogsByProvider(456, 3, 100), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(getAuditLogsByProvider).toHaveBeenCalledWith(456, 3, 100)
  })
})
