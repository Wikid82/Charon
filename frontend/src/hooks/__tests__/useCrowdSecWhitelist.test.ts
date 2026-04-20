import { QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act, waitFor } from '@testing-library/react'
import React from 'react'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as toastUtil from '../../utils/toast'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import { useWhitelistEntries, useAddWhitelist, useDeleteWhitelist } from '../useCrowdSecWhitelist'

import type { CrowdSecWhitelistEntry } from '../../api/crowdsec'

vi.mock('../../api/crowdsec', () => ({
  listWhitelists: vi.fn(),
  addWhitelist: vi.fn(),
  deleteWhitelist: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const wrapper = ({ children }: { children: React.ReactNode }) => {
  const qc = createTestQueryClient()
  return React.createElement(QueryClientProvider, { client: qc }, children)
}

const mockEntry: CrowdSecWhitelistEntry = {
  uuid: 'abc-123',
  ip_or_cidr: '192.168.1.1',
  reason: 'trusted device',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

describe('useWhitelistEntries', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns whitelist entries on success', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([mockEntry])

    const { result } = renderHook(() => useWhitelistEntries(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toEqual([mockEntry])
  })

  it('returns empty array when no entries', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([])

    const { result } = renderHook(() => useWhitelistEntries(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data).toEqual([])
  })
})

describe('useAddWhitelist', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls addWhitelist and shows success toast on success', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockResolvedValue(mockEntry)

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '192.168.1.1', reason: 'test' })
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(crowdsecApi.addWhitelist).toHaveBeenCalledWith({ ip_or_cidr: '192.168.1.1', reason: 'test' })
    expect(toastUtil.toast.success).toHaveBeenCalledWith('Whitelist entry added')
  })

  it('shows error toast with server message on failure', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue(new Error('IP already whitelisted'))

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '10.0.0.0/8', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('IP already whitelisted')
  })

  it('shows generic error toast for non-Error failures', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue('unexpected')

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '10.0.0.1', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to add whitelist entry')
  })
})

describe('useDeleteWhitelist', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls deleteWhitelist and shows success toast on success', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('abc-123')
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(crowdsecApi.deleteWhitelist).toHaveBeenCalledWith('abc-123')
    expect(toastUtil.toast.success).toHaveBeenCalledWith('Whitelist entry removed')
  })

  it('shows error toast with server message on failure', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockRejectedValue(new Error('Entry not found'))

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('bad-uuid')
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Entry not found')
  })

  it('shows generic error toast for non-Error failures', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockRejectedValue(null)

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('some-uuid')
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to remove whitelist entry')
  })
})
