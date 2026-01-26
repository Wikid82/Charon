import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import RateLimiting from '../RateLimiting'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import type { SecurityStatus } from '../../api/security'

vi.mock('../../api/security')
vi.mock('../../api/settings')

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>{ui}</BrowserRouter>
    </QueryClientProvider>
  )
}

const mockStatusEnabled: SecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: false, mode: 'disabled', api_url: '' },
  waf: { enabled: false, mode: 'disabled' },
  rate_limit: { enabled: true, mode: 'enabled' },
  acl: { enabled: false },
}

const mockStatusDisabled: SecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: false, mode: 'disabled', api_url: '' },
  waf: { enabled: false, mode: 'disabled' },
  rate_limit: { enabled: false, mode: 'disabled' },
  acl: { enabled: false },
}

const mockSecurityConfig = {
  config: {
    name: 'default',
    rate_limit_requests: 10,
    rate_limit_burst: 5,
    rate_limit_window_sec: 60,
  },
}

describe('RateLimiting page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows loading state while fetching status', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockReturnValue(new Promise(() => {}))
    vi.mocked(securityApi.getSecurityConfig).mockReturnValue(new Promise(() => {}))

    renderWithProviders(<RateLimiting />)

    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('renders rate limiting page with toggle disabled when rate_limit is off', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText('Rate Limiting Configuration')).toBeInTheDocument()
    })

    const toggle = screen.getByTestId('rate-limit-toggle')
    expect(toggle).toBeInTheDocument()
    expect((toggle as HTMLInputElement).checked).toBe(false)
  })

  it('renders rate limiting page with toggle enabled when rate_limit is on', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText('Rate Limiting Configuration')).toBeInTheDocument()
    })

    const toggle = screen.getByTestId('rate-limit-toggle')
    expect((toggle as HTMLInputElement).checked).toBe(true)
  })

  it('shows configuration inputs when enabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toBeInTheDocument()
    })

    expect(screen.getByTestId('rate-limit-burst')).toBeInTheDocument()
    expect(screen.getByTestId('rate-limit-window')).toBeInTheDocument()
  })

  it('calls updateSetting when toggle is clicked', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    const toggle = screen.getByTestId('rate-limit-toggle')
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
        'security.rate_limit.enabled',
        'true',
        'security',
        'bool'
      )
    })
  })

  it('calls updateSecurityConfig when save button is clicked', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toBeInTheDocument()
    })

    // Wait for initial values to be set from config
    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toHaveValue(10)
    })

    // Change RPS value using tripleClick to select all then type
    const rpsInput = screen.getByTestId('rate-limit-rps')
    await userEvent.tripleClick(rpsInput)
    await userEvent.keyboard('25')

    // Click save
    const saveBtn = screen.getByTestId('save-rate-limit-btn')
    await userEvent.click(saveBtn)

    await waitFor(() => {
      expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          rate_limit_requests: 25,
          rate_limit_burst: 5,
          rate_limit_window_sec: 60,
        })
      )
    })
  })

  it('displays default values from config', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toBeInTheDocument()
    })

    expect(screen.getByTestId('rate-limit-rps')).toHaveValue(10)
    expect(screen.getByTestId('rate-limit-burst')).toHaveValue(5)
    expect(screen.getByTestId('rate-limit-window')).toHaveValue(60)
  })

  it('hides configuration inputs when disabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText('Rate Limiting Configuration')).toBeInTheDocument()
    })

    expect(screen.queryByTestId('rate-limit-rps')).not.toBeInTheDocument()
    expect(screen.queryByTestId('rate-limit-burst')).not.toBeInTheDocument()
    expect(screen.queryByTestId('rate-limit-window')).not.toBeInTheDocument()
  })

  it('shows info banner about rate limiting', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText(/Rate limiting helps protect/)).toBeInTheDocument()
    })
  })
})
