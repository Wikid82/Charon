import { screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Dashboard from '../Dashboard'

vi.mock('../../hooks/useProxyHosts', () => ({
  useProxyHosts: () => ({
    hosts: [
      { id: 1, enabled: true, ssl_forced: false, domain_names: 'test.com' },
      { id: 2, enabled: false, ssl_forced: false, domain_names: 'test2.com' },
    ],
    loading: false,
  }),
}))

vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: () => ({
    servers: [
      { id: 1, enabled: true },
      { id: 2, enabled: true },
    ],
    loading: false,
  }),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: () => ({
    certificates: [
      { id: 1, status: 'valid', domain: 'test.com' },
      { id: 2, status: 'expired', domain: 'expired.com' },
    ],
    isLoading: false,
  }),
}))

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: () => ({
    data: [{ id: 1, enabled: true }],
    isLoading: false,
  }),
}))

vi.mock('../../api/health', () => ({
  checkHealth: vi.fn().mockResolvedValue({ status: 'ok', version: '1.0.0' }),
}))

// Mock UptimeWidget to avoid complex dependencies
vi.mock('../../components/UptimeWidget', () => ({
  default: () => <div data-testid="uptime-widget">Uptime Widget</div>,
}))

describe('Dashboard page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders counts and health status', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('Dashboard')).toBeInTheDocument()
    expect(await screen.findByText('1 enabled')).toBeInTheDocument()
    expect(screen.getByText('2 enabled')).toBeInTheDocument()
    expect(screen.getByText('1 valid')).toBeInTheDocument()
    expect(await screen.findByText('Healthy')).toBeInTheDocument()
  })

  it('shows error state when health check fails', async () => {
    const { checkHealth } = await import('../../api/health')
    vi.mocked(checkHealth).mockResolvedValueOnce({ status: 'fail', version: '1.0.0' } as never)

    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('Error')).toBeInTheDocument()
  })
})
