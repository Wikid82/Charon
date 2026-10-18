import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { AxiosError } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/presets'
import { applyJobPollDelayMs, useCrowdsecApplyJob } from '../useCrowdsecApplyJob'

import type { ReactNode } from 'react'

vi.mock('../../api/presets', async (importOriginal) => ({
  ...(await importOriginal<typeof api>()),
  applyPresetAsync: vi.fn(),
  cancelApplyJob: vi.fn(),
  getApplyJob: vi.fn(),
  getCurrentApplyJob: vi.fn(),
}))

const job = (overrides: Partial<api.CrowdsecApplyJob> = {}): api.CrowdsecApplyJob => ({
  job_id: 'job-1',
  slug: 's',
  item_type: 'collections',
  cache_key: 'k',
  state: 'running',
  phase: 'install',
  warnings: [],
  items_installed: 0,
  items_total: null,
  cancel_requested: false,
  elapsed_seconds: 0,
  created_at: '2026-10-18T10:00:00Z',
  started_at: null,
  finished_at: null,
  ...overrides,
})

const axiosError = (status: number, data: Record<string, unknown>) =>
  new AxiosError('x', undefined, undefined, undefined, { status, statusText: String(status), headers: {}, config: {}, data } as never)

const setup = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries')
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  return { invalidate, ...renderHook(() => useCrowdsecApplyJob(), { wrapper }) }
}

describe('applyJobPollDelayMs', () => {
  it('polls every 2 s, then 5 s after 30 s, then 10 s after 5 minutes', () => {
    expect(applyJobPollDelayMs(0, 0)).toBe(2000)
    expect(applyJobPollDelayMs(29, 0)).toBe(2000)
    expect(applyJobPollDelayMs(30, 0)).toBe(5000)
    expect(applyJobPollDelayMs(299, 0)).toBe(5000)
    expect(applyJobPollDelayMs(300, 0)).toBe(10000)
  })

  it('backs off exponentially after failures and caps at 30 s', () => {
    expect([1, 2, 3, 4, 5, 6, 10].map((n) => applyJobPollDelayMs(5, n))).toEqual([2000, 4000, 8000, 16000, 30000, 30000, 30000])
  })
})

describe('useCrowdsecApplyJob', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(null)
  })

  afterEach(() => vi.useRealTimers())

  it('has no job when none is running', async () => {
    const { result } = setup()
    await waitFor(() => expect(api.getCurrentApplyJob).toHaveBeenCalled())
    expect(result.current.job).toBeNull()
    expect(result.current.isActive).toBe(false)
    expect(api.getApplyJob).not.toHaveBeenCalled()
  })

  it('resumes an active job found on load and polls it', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job({ items_installed: 18 }))
    vi.mocked(api.getApplyJob).mockResolvedValue(job({ items_installed: 19 }))

    const { result } = setup()

    await waitFor(() => expect(result.current.job?.items_installed).toBe(19))
    expect(result.current.isActive).toBe(true)
    expect(api.getApplyJob).toHaveBeenCalledWith('job-1')
  })

  it('shows a finished job from the current endpoint without polling it', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job({ state: 'succeeded', items_installed: 41 }))

    const { result, invalidate } = setup()

    await waitFor(() => expect(result.current.job?.state).toBe('succeeded'))
    expect(result.current.isActive).toBe(false)
    expect(api.getApplyJob).not.toHaveBeenCalled()
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ['crowdsec-presets'] }))
  })

  it('hides a dismissed finished job', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job({ state: 'failed' }))
    const { result } = setup()
    await waitFor(() => expect(result.current.job).not.toBeNull())

    act(() => result.current.dismiss())

    expect(result.current.job).toBeNull()
  })

  it('follows a started job, advances on a timer and stops at a terminal state', async () => {
    vi.mocked(api.applyPresetAsync).mockResolvedValue(job({ state: 'queued' }))
    vi.mocked(api.getApplyJob)
      .mockResolvedValueOnce(job({ items_installed: 5 }))
      .mockResolvedValue(job({ state: 'succeeded', items_installed: 41 }))
    const { result, invalidate } = setup()
    await waitFor(() => expect(api.getCurrentApplyJob).toHaveBeenCalled())

    vi.useFakeTimers({ shouldAdvanceTime: true })
    await act(async () => {
      await result.current.start({ slug: 's', cache_key: 'k' })
    })
    expect(api.applyPresetAsync).toHaveBeenCalledWith({ slug: 's', cache_key: 'k' })
    await waitFor(() => expect(result.current.job?.items_installed).toBe(5))
    expect(result.current.isActive).toBe(true)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    await waitFor(() => expect(result.current.job?.state).toBe('succeeded'))
    const pollsAtTerminal = vi.mocked(api.getApplyJob).mock.calls.length

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(vi.mocked(api.getApplyJob).mock.calls.length).toBe(pollsAtTerminal)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['crowdsec-files'] })
  })

  it('keeps the last job, reports waiting for the server and recovers after a poll failure', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job({ items_installed: 7 }))
    vi.mocked(api.getApplyJob)
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValue(job({ items_installed: 9 }))
    vi.useFakeTimers({ shouldAdvanceTime: true })

    const { result } = setup()

    await waitFor(() => expect(result.current.isWaitingForServer).toBe(true))
    expect(result.current.job?.items_installed).toBe(7)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    await waitFor(() => expect(result.current.job?.items_installed).toBe(9))
    expect(result.current.isWaitingForServer).toBe(false)
  })

  it('doubles the delay between failed polls', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job())
    const calledAt: number[] = []
    vi.mocked(api.getApplyJob).mockImplementation(() => {
      calledAt.push(Date.now())
      return Promise.reject(new Error('down'))
    })
    vi.useFakeTimers({ shouldAdvanceTime: true })
    setup()
    await waitFor(() => expect(calledAt).toHaveLength(1))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_000)
    })

    const gaps = calledAt.slice(1).map((at, i) => at - calledAt[i])
    expect(gaps.length).toBeGreaterThanOrEqual(3)
    expect(gaps[0]).toBeGreaterThanOrEqual(2000)
    expect(gaps[0]).toBeLessThan(3000)
    expect(gaps[1]).toBeGreaterThanOrEqual(4000)
    expect(gaps[1]).toBeLessThan(5000)
    expect(gaps[2]).toBeGreaterThanOrEqual(8000)
  })

  it('drops a job the server no longer knows', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job())
    vi.mocked(api.getApplyJob).mockRejectedValue(axiosError(404, { error_code: 'job_not_found' }))

    const { result } = setup()

    await waitFor(() => expect(api.getApplyJob).toHaveBeenCalled())
    await waitFor(() => expect(result.current.job).toBeNull())
  })

  it('follows the job named by a crowdsec_busy refusal and rethrows', async () => {
    vi.mocked(api.applyPresetAsync).mockRejectedValue(axiosError(409, { error_code: 'crowdsec_busy', job_id: 'job-9' }))
    vi.mocked(api.getApplyJob).mockResolvedValue(job({ job_id: 'job-9', items_installed: 4 }))
    const { result } = setup()
    await waitFor(() => expect(api.getCurrentApplyJob).toHaveBeenCalled())

    await act(async () => {
      await expect(result.current.start({ slug: 's' })).rejects.toThrow()
    })

    await waitFor(() => expect(result.current.job?.job_id).toBe('job-9'))
  })

  it('cancels the job and takes the returned state', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job())
    vi.mocked(api.getApplyJob).mockResolvedValue(job())
    vi.mocked(api.cancelApplyJob).mockResolvedValue(job({ cancel_requested: true }))
    const { result } = setup()
    await waitFor(() => expect(result.current.isActive).toBe(true))

    act(() => result.current.cancel('job-1'))

    await waitFor(() => expect(result.current.job?.cancel_requested).toBe(true))
    expect(vi.mocked(api.cancelApplyJob).mock.calls[0][0]).toBe('job-1')
  })

  it('re-reads the job when cancel is refused because it already finished', async () => {
    vi.mocked(api.getCurrentApplyJob).mockResolvedValue(job())
    vi.mocked(api.getApplyJob).mockResolvedValueOnce(job()).mockResolvedValue(job({ state: 'succeeded' }))
    vi.mocked(api.cancelApplyJob).mockRejectedValue(axiosError(409, { error_code: 'job_not_active' }))
    const { result } = setup()
    await waitFor(() => expect(result.current.isActive).toBe(true))

    act(() => result.current.cancel('job-1'))

    await waitFor(() => expect(result.current.job?.state).toBe('succeeded'))
  })
})
