import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { AlertsList } from '../crowdsec/AlertsList'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallbackOrOpts?: string | Record<string, unknown>, opts?: Record<string, unknown>) => {
      const fallback = typeof fallbackOrOpts === 'string' ? fallbackOrOpts : key
      const params = typeof fallbackOrOpts === 'object' ? fallbackOrOpts : opts
      if (!params) return fallback
      return fallback.replace(/\{\{(\w+)\}\}/g, (_, k) => String(params[k] ?? ''))
    },
  }),
}))

const mockAlerts = {
  data: {
    alerts: [
      {
        id: 1,
        scenario: 'crowdsecurity/http-bad-user-agent',
        ip: '192.168.1.100',
        message: 'test alert',
        events_count: 5,
        start_at: '2025-01-01T00:00:00Z',
        stop_at: '2025-01-01T01:00:00Z',
        created_at: '2025-01-01T00:00:00Z',
        duration: '4h',
        type: 'ban',
        origin: 'crowdsec',
      },
      {
        id: 2,
        scenario: 'crowdsecurity/ssh-bf',
        ip: '10.0.0.1',
        message: 'ssh bruteforce',
        events_count: 12,
        start_at: '2025-01-01T00:00:00Z',
        stop_at: '2025-01-01T01:00:00Z',
        created_at: '2025-01-01T00:30:00Z',
        duration: '4h',
        type: 'ban',
        origin: 'crowdsec',
      },
    ],
    total: 2,
    source: 'cscli',
    cached: false,
  },
  isLoading: false,
  isError: false,
}

const mockUseAlerts = vi.fn((_params?: unknown): { data: typeof mockAlerts['data'] | undefined; isLoading: boolean; isError: boolean } => mockAlerts)

vi.mock('../../hooks/useCrowdsecDashboard', () => ({
  useAlerts: (params: unknown) => mockUseAlerts(params),
}))

describe('AlertsList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAlerts.mockReturnValue(mockAlerts)
  })

  it('renders alert rows with IP, scenario, and events count', () => {
    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.getByText('192.168.1.100')).toBeInTheDocument()
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
    expect(screen.getByText('http-bad-user-agent')).toBeInTheDocument()
    expect(screen.getByText('ssh-bf')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
  })

  it('renders the heading and total count', () => {
    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.getByText('Recent Alerts')).toBeInTheDocument()
    expect(screen.getByText('2 total')).toBeInTheDocument()
  })

  it('shows loading skeleton when isLoading', () => {
    mockUseAlerts.mockReturnValue({ data: undefined, isLoading: true, isError: false })

    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.getByTestId('alerts-list')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('shows error state when isError', () => {
    mockUseAlerts.mockReturnValue({ data: undefined, isLoading: false, isError: true })

    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.getByText('Failed to load alerts.')).toBeInTheDocument()
  })

  it('shows empty state when no alerts exist', () => {
    mockUseAlerts.mockReturnValue({
      data: { alerts: [], total: 0, source: 'cscli', cached: false },
      isLoading: false,
      isError: false,
    })

    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.getByText('No alerts for the selected period.')).toBeInTheDocument()
  })

  it('renders pagination when there are more than PAGE_SIZE alerts', async () => {
    const manyAlerts = Array.from({ length: 10 }, (_, i) => ({
      id: i + 1,
      scenario: 'crowdsecurity/http-bf',
      ip: `10.0.0.${i + 1}`,
      message: 'test',
      events_count: 1,
      start_at: '2025-01-01T00:00:00Z',
      stop_at: '2025-01-01T01:00:00Z',
      created_at: '2025-01-01T00:00:00Z',
      duration: '4h',
      type: 'ban',
      origin: 'crowdsec',
    }))

    mockUseAlerts.mockReturnValue({
      data: { alerts: manyAlerts, total: 25, source: 'cscli', cached: false },
      isLoading: false,
      isError: false,
    })

    const user = userEvent.setup()
    renderWithQueryClient(<AlertsList range="24h" />)

    const nav = screen.getByRole('navigation', { name: /pagination/i })
    expect(nav).toBeInTheDocument()
    expect(within(nav).getByText('Page 1 of 3')).toBeInTheDocument()

    const nextBtn = screen.getByRole('button', { name: /next page/i })
    expect(nextBtn).not.toBeDisabled()

    const prevBtn = screen.getByRole('button', { name: /previous page/i })
    expect(prevBtn).toBeDisabled()

    await user.click(nextBtn)

    expect(mockUseAlerts).toHaveBeenCalledWith(
      expect.objectContaining({ offset: 10 })
    )
  })

  it('does not render pagination for single page', () => {
    renderWithQueryClient(<AlertsList range="24h" />)

    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })
})
