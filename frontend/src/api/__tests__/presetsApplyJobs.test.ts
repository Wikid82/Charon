import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import { isCrowdsecBusyError } from '../errorBody'
import * as presets from '../presets'

vi.mock('../client')

const job = (overrides: Partial<presets.CrowdsecApplyJob> = {}): presets.CrowdsecApplyJob => ({
  job_id: '7b1f6c1e-0c55-4a53-9d3c-5a0d3b6f2e10',
  slug: 'sigmahq/windows_proc_creation',
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

describe('preset apply job API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('starts an async apply with the default timeout', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: job({ state: 'queued' }) })

    const started = await presets.applyPresetAsync({ slug: 's', type: 'collections', cache_key: 'k' })

    expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/apply', { slug: 's', type: 'collections', cache_key: 'k', async: true })
    expect(started.state).toBe('queued')
  })

  it('unwraps the current job and returns null when there is none', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: { job: job() } }).mockResolvedValueOnce({ data: { job: null } })

    expect((await presets.getCurrentApplyJob())?.state).toBe('running')
    expect(await presets.getCurrentApplyJob()).toBeNull()
    expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/presets/jobs/current')
  })

  it('reads, lists and cancels jobs by id', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: job() }).mockResolvedValueOnce({ data: { jobs: [job()] } })
    vi.mocked(client.delete).mockResolvedValue({ data: job({ cancel_requested: true }) })

    await presets.getApplyJob('a/b')
    expect(client.get).toHaveBeenLastCalledWith('/admin/crowdsec/presets/jobs/a%2Fb')
    expect(await presets.listApplyJobs(5)).toHaveLength(1)
    expect(client.get).toHaveBeenLastCalledWith('/admin/crowdsec/presets/jobs', { params: { limit: 5 } })
    expect((await presets.cancelApplyJob('id1')).cancel_requested).toBe(true)
    expect(client.delete).toHaveBeenCalledWith('/admin/crowdsec/presets/jobs/id1')
  })

  it('reads job error codes and the busy job id', () => {
    expect(presets.getApplyJobErrorCode(axiosError(404, { error_code: 'job_not_found' }))).toBe('job_not_found')
    expect(presets.getApplyJobErrorCode(axiosError(409, { error_code: 'job_not_active' }))).toBe('job_not_active')
    expect(presets.getApplyJobErrorCode(axiosError(500, { error_code: 'other' }))).toBeUndefined()

    const busy = axiosError(409, { error_code: 'crowdsec_busy', job_id: 'abc' })
    expect(isCrowdsecBusyError(busy)).toBe(true)
    expect(presets.getCrowdsecBusyJobId(busy)).toBe('abc')
    expect(presets.getCrowdsecBusyJobId(axiosError(409, { error_code: 'crowdsec_busy' }))).toBeUndefined()
    expect(presets.getCrowdsecBusyJobId(new Error('x'))).toBeUndefined()
    expect(isCrowdsecBusyError(new Error('x'))).toBe(false)
  })
})
