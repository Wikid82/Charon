import { QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act, waitFor } from '@testing-library/react'
import { AxiosError, type AxiosResponse } from 'axios'
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

const BUSY_TEXT = 'CrowdSec is busy installing a preset. Try again when it finishes.'

// The refusal the server sends while a background preset install holds the CrowdSec lock.
const busyError = () =>
  new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
    status: 409,
    statusText: 'Conflict',
    headers: {},
    config: {},
    data: { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' },
  } as AxiosResponse)

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

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to add entry')
  })

  it('shows the busy message when an install refuses the add', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue(busyError())

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '10.0.0.2', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith(BUSY_TEXT)
  })
  it('shows the translated fallback instead of raw Axios text on a non-validation failure', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue(
      new AxiosError('Request failed with status code 500', 'ERR_BAD_RESPONSE', undefined, undefined, {
        status: 500,
        statusText: 'Internal Server Error',
        headers: {},
        config: {},
        data: { error: 'database locked' },
      } as AxiosResponse),
    )

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '10.0.0.3', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to add entry')
    expect(toastUtil.toast.error).not.toHaveBeenCalledWith('Request failed with status code 500')
  })

  it('shows the translated fallback when the request never reached the server', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue(new AxiosError('Network Error', 'ERR_NETWORK'))

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: '10.0.0.4', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to add entry')
  })

  it('keeps the validation message from a 400 response for the inline IP field', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValue(
      new AxiosError('Invalid IP', 'ERR_BAD_REQUEST', undefined, undefined, {
        status: 400,
        statusText: 'Bad Request',
        headers: {},
        config: {},
        data: { error: 'bad ip format' },
      } as AxiosResponse),
    )

    const { result } = renderHook(() => useAddWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate({ ip_or_cidr: 'bad-ip', reason: '' })
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Invalid IP')
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

  it('shows the removal fallback instead of raw Axios text on a non-busy failure', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockRejectedValue(
      new AxiosError('Request failed with status code 404', 'ERR_BAD_REQUEST', undefined, undefined, {
        status: 404,
        statusText: 'Not Found',
        headers: {},
        config: {},
        data: { error: 'entry not found' },
      } as AxiosResponse),
    )

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('bad-uuid')
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to remove entry')
    expect(toastUtil.toast.error).not.toHaveBeenCalledWith('Request failed with status code 404')
  })

  it('shows generic error toast for non-Error failures', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockRejectedValue(null)

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('some-uuid')
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith('Failed to remove entry')
  })

  it('shows the busy message when an install refuses the removal', async () => {
    vi.mocked(crowdsecApi.deleteWhitelist).mockRejectedValue(busyError())

    const { result } = renderHook(() => useDeleteWhitelist(), { wrapper })

    await act(async () => {
      result.current.mutate('some-uuid')
    })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect(toastUtil.toast.error).toHaveBeenCalledWith(BUSY_TEXT)
  })
})
