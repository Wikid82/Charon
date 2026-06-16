import { screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Dashboard from '../Dashboard'

// localStorage is available in jsdom — clear between tests
beforeEach(() => {
  localStorage.clear()
})

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
      { id: 1, status: 'valid', domain: 'test.com', domains: 'test.com,www.test.com' },
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

// Mock stats hooks
vi.mock('../../hooks/useStats', () => ({
  useStatsSummary: () => ({
    data: { last_24h: 1200, last_7d: 8400, last_30d: 36000 },
    isLoading: false,
  }),
  useTopHosts: () => ({
    data: [
      { host: 'api.example.com', count: 500 },
      { host: 'app.example.com', count: 300 },
    ],
    isLoading: false,
  }),
  useStatusDistribution: () => ({
    data: [
      { status_code: 200, count: 900 },
      { status_code: 404, count: 100 },
    ],
    isLoading: false,
  }),
  useTrafficVolume: () => ({
    data: [
      { bucket: '2024-01-01T00:00:00Z', count: 42 },
    ],
    isLoading: false,
  }),
  useCertExpiry: () => ({
    data: [
      { host: 'expire-soon.com', expires_at: '2024-02-01T00:00:00Z', days_left: 10 },
    ],
    isLoading: false,
  }),
  useStatsHealth: () => ({
    data: { dropped_count: 0 },
    isLoading: false,
  }),
}))

// Mock WebSocket hook
vi.mock('../../hooks/useStatsWebSocket', () => ({
  useStatsWebSocket: () => ({
    connected: true,
    lastMessage: null,
    latestSummary: null,
  }),
}))

// Mock stats components to isolate Dashboard rendering
vi.mock('../../components/stats', () => ({
  RequestCountWidget: ({ isLoading }: { isLoading: boolean }) => (
    <div data-testid="request-count-widget">{isLoading ? 'Loading...' : 'Request Counts'}</div>
  ),
  ServiceHealthWidget: ({ wsConnected }: { wsConnected: boolean }) => (
    <div data-testid="service-health-widget">{wsConnected ? 'Live' : 'Offline'}</div>
  ),
  CertExpiryList: ({ isLoading }: { isLoading: boolean }) => (
    <div data-testid="cert-expiry-list">{isLoading ? 'Loading...' : 'Cert Expiry'}</div>
  ),
  TrafficVolumeChart: ({ bucket }: { bucket: string }) => (
    <div data-testid="traffic-volume-chart">Traffic Volume ({bucket})</div>
  ),
  TopHostsChart: ({ isLoading }: { isLoading: boolean }) => (
    <div data-testid="top-hosts-chart">{isLoading ? 'Loading...' : 'Top Hosts'}</div>
  ),
  StatusDistributionChart: ({ isLoading }: { isLoading: boolean }) => (
    <div data-testid="status-distribution-chart">{isLoading ? 'Loading...' : 'Status Distribution'}</div>
  ),
  PeriodSelector: ({ value, onChange }: { value: string; onChange: (p: string) => void }) => (
    <div data-testid="period-selector">
      <button onClick={() => onChange('7d')}>7d</button>
      <span>{value}</span>
    </div>
  ),
  BucketSelector: ({ value, onChange }: { value: string; onChange: (b: string) => void }) => (
    <div data-testid="bucket-selector">
      <button onClick={() => onChange('6h')}>6h</button>
      <span>{value}</span>
    </div>
  ),
}))

describe('Dashboard page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
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

  it('handles certificates with missing domains field', async () => {
    // The top-level mock returns certs with "domain" (singular) but Dashboard
    // reads "domains" (plural), so the !cert.domains guard on line 48 is
    // already exercised by every render.  Re-render and verify it doesn't crash.
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('Dashboard')).toBeInTheDocument()
    // "1 valid" still renders even though cert.domains is undefined
    expect(screen.getByText('1 valid')).toBeInTheDocument()
  })

  it('renders the uptime widget', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByTestId('uptime-widget')).toBeInTheDocument()
  })

  it('renders stats section heading', async () => {
    renderWithQueryClient(<Dashboard />)

    // The heading text comes from the dashboard.statistics translation key
    expect(await screen.findByRole('heading', { name: 'Statistics' })).toBeInTheDocument()
  })

  it('renders all stats widgets', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByTestId('request-count-widget')).toBeInTheDocument()
    expect(screen.getByTestId('service-health-widget')).toBeInTheDocument()
    expect(screen.getByTestId('cert-expiry-list')).toBeInTheDocument()
    expect(screen.getByTestId('traffic-volume-chart')).toBeInTheDocument()
    expect(screen.getByTestId('top-hosts-chart')).toBeInTheDocument()
    expect(screen.getByTestId('status-distribution-chart')).toBeInTheDocument()
  })

  it('shows WebSocket connected state in service health widget', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('Live')).toBeInTheDocument()
  })

  it('renders period selector with default 24h', async () => {
    renderWithQueryClient(<Dashboard />)

    const periodSelector = await screen.findByTestId('period-selector')
    expect(periodSelector).toBeInTheDocument()
    expect(periodSelector).toHaveTextContent('24h')
  })

  it('renders bucket selector with default 1h', async () => {
    renderWithQueryClient(<Dashboard />)

    const bucketSelector = await screen.findByTestId('bucket-selector')
    expect(bucketSelector).toBeInTheDocument()
    expect(bucketSelector).toHaveTextContent('1h')
  })

  it('updates period state when PeriodSelector fires onChange', async () => {
    renderWithQueryClient(<Dashboard />)

    const periodSelector = await screen.findByTestId('period-selector')
    expect(periodSelector).toHaveTextContent('24h')

    fireEvent.click(screen.getByRole('button', { name: '7d' }))

    expect(periodSelector).toHaveTextContent('7d')
  })

  it('updates bucket state when BucketSelector fires onChange', async () => {
    renderWithQueryClient(<Dashboard />)

    const bucketSelector = await screen.findByTestId('bucket-selector')
    expect(bucketSelector).toHaveTextContent('1h')

    fireEvent.click(screen.getByRole('button', { name: '6h' }))

    expect(bucketSelector).toHaveTextContent('6h')
  })

  it('traffic volume chart reflects current bucket value', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('Traffic Volume (1h)')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '6h' }))

    expect(screen.getByText('Traffic Volume (6h)')).toBeInTheDocument()
  })

  it('renders Customize button in the stats section header', async () => {
    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByRole('button', { name: /customize/i })).toBeInTheDocument()
  })

  it('customize panel is hidden by default', async () => {
    renderWithQueryClient(<Dashboard />)

    await screen.findByRole('button', { name: /customize/i })

    expect(screen.queryByText('Show / hide widgets')).not.toBeInTheDocument()
  })

  it('clicking Customize button opens the settings panel', async () => {
    renderWithQueryClient(<Dashboard />)

    const btn = await screen.findByRole('button', { name: /customize/i })
    fireEvent.click(btn)

    expect(screen.getByText('Show / hide widgets')).toBeInTheDocument()
  })

  it('clicking Customize button again closes the panel', async () => {
    renderWithQueryClient(<Dashboard />)

    const btn = await screen.findByRole('button', { name: /customize/i })
    fireEvent.click(btn)
    expect(screen.getByText('Show / hide widgets')).toBeInTheDocument()

    fireEvent.click(btn)
    expect(screen.queryByText('Show / hide widgets')).not.toBeInTheDocument()
  })

  it('toggling a widget switch hides that widget', async () => {
    renderWithQueryClient(<Dashboard />)

    // Open customize panel
    const btn = await screen.findByRole('button', { name: /customize/i })
    fireEvent.click(btn)

    // The Request Counts widget is visible initially
    expect(screen.getByTestId('request-count-widget')).toBeInTheDocument()

    // Toggle it off
    const toggle = screen.getByRole('checkbox', { name: /request counts/i })
    fireEvent.click(toggle)

    expect(screen.queryByTestId('request-count-widget')).not.toBeInTheDocument()
  })

  it('shows "(all hidden)" message when all widgets are hidden', async () => {
    // Pre-seed localStorage with all hidden
    localStorage.setItem(
      'charon.stats.widgetVisibility',
      JSON.stringify({
        requestCount: false,
        trafficVolume: false,
        topHosts: false,
        statusDistribution: false,
        certExpiry: false,
        serviceHealth: false,
      })
    )

    renderWithQueryClient(<Dashboard />)

    expect(await screen.findByText('(all hidden)')).toBeInTheDocument()
  })
})
