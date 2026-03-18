import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import CrowdSecConfig from '../CrowdSecConfig'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/presets')
vi.mock('../../api/featureFlags')
vi.mock('../../api/backups', () => ({
  createBackup: vi.fn().mockResolvedValue({ filename: 'backup.tar.gz' }),
}))
vi.mock('../../hooks/useConsoleEnrollment', () => ({
  useConsoleStatus: vi.fn(() => ({
    data: {
      status: 'not_enrolled',
      tenant: 'default',
      agent_name: 'charon-agent',
      last_error: null,
      last_attempt_at: null,
      enrolled_at: null,
      last_heartbeat_at: null,
      key_present: false,
      correlation_id: 'corr-1',
    },
    isLoading: false,
    isRefetching: false,
  })),
  useEnrollConsole: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({ status: 'enrolling', key_present: false }),
    isPending: false,
  })),
  useClearConsoleEnrollment: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}))
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({
  CrowdSecBouncerKeyDisplay: () => null,
}))
vi.mock('../../utils/crowdsecExport', () => ({
  buildCrowdsecExportFilename: vi.fn(() => 'crowdsec-default.tar.gz'),
  promptCrowdsecFilename: vi.fn(() => 'crowdsec.tar.gz'),
  downloadCrowdsecExport: vi.fn(),
}))
vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

const baseStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
  waf: { enabled: true, mode: 'enabled' as const },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  })
}

function renderWithSeed(
  crowdsecStartingData: { isStarting: boolean; startedAt?: number },
  lapiStatus: { running: boolean; pid?: number; lapi_ready: boolean }
) {
  const queryClient = makeQueryClient()
  queryClient.setQueryData(['crowdsec-starting'], crowdsecStartingData)
  queryClient.setQueryData(['crowdsec-lapi-status'], lapiStatus)
  queryClient.setQueryData(['feature-flags'], { 'feature.crowdsec.console_enrollment': true })
  queryClient.setQueryData(['security-status'], baseStatus)

  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CrowdSecConfig />
        </MemoryRouter>
      </QueryClientProvider>
    ),
  }
}

describe('CrowdSecConfig — isStartingUp banner suppression', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()

    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': true,
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob())
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('LAPI not-running banner suppressed when isStartingUp is true', async () => {
    renderWithSeed(
      { isStarting: true, startedAt: Date.now() },
      { running: false, lapi_ready: false }
    )

    // Advance past the 3-second initialCheckComplete guard
    await act(async () => { await vi.advanceTimersByTimeAsync(3001) })

    expect(screen.queryByTestId('lapi-not-running-warning')).not.toBeInTheDocument()
  })

  it('LAPI initializing banner suppressed when isStartingUp is true', async () => {
    renderWithSeed(
      { isStarting: true, startedAt: Date.now() },
      { running: true, lapi_ready: false }
    )

    await act(async () => { await vi.advanceTimersByTimeAsync(3001) })

    expect(screen.queryByTestId('lapi-warning')).not.toBeInTheDocument()
  })

  it('LAPI not-running banner shows after isStartingUp expires (100s ago)', async () => {
    renderWithSeed(
      { isStarting: true, startedAt: Date.now() - 100_000 },
      { running: false, lapi_ready: false }
    )

    await act(async () => { await vi.advanceTimersByTimeAsync(3001) })

    expect(screen.getByTestId('lapi-not-running-warning')).toBeInTheDocument()
  })

  it('LAPI not-running banner shows when isStartingUp is false', async () => {
    renderWithSeed(
      { isStarting: false },
      { running: false, lapi_ready: false }
    )

    await act(async () => { await vi.advanceTimersByTimeAsync(3001) })

    expect(screen.getByTestId('lapi-not-running-warning')).toBeInTheDocument()
  })
})
