import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import { toast } from '../../utils/toast'
import RateLimiting from '../RateLimiting'

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
    expect(screen.getByLabelText(/Enable Rate Limiting/i)).toBe(toggle)
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
      expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({
        rate_limit_requests: 25,
        rate_limit_burst: 5,
        rate_limit_window_sec: 60,
      })
    })

    // Exact key set: the page sends only its own section, no extra or dropped keys
    const [sentPayload] = vi.mocked(securityApi.updateSecurityConfig).mock.calls[0]
    expect(Object.keys(sentPayload).sort()).toEqual([
      'rate_limit_burst',
      'rate_limit_requests',
      'rate_limit_window_sec',
    ])
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

  it('shows a stored zero as an empty field with the standard numbers as placeholders', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({
      config: { rate_limit_requests: 0, rate_limit_burst: 0, rate_limit_window_sec: 0 },
    })

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toBeInTheDocument()
    })

    expect(screen.getByTestId('rate-limit-rps')).toHaveValue(null)
    expect(screen.getByTestId('rate-limit-rps')).toHaveAttribute('placeholder', '100')
    expect(screen.getByTestId('rate-limit-burst')).toHaveValue(null)
    expect(screen.getByTestId('rate-limit-burst')).toHaveAttribute('placeholder', '20')
    expect(screen.getByTestId('rate-limit-window')).toHaveValue(null)
    expect(screen.getByTestId('rate-limit-window')).toHaveAttribute('placeholder', '60')
  })

  it('falls back to the standard numbers when the stored values are missing', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({
      config: { rate_limit_requests: null, rate_limit_burst: null, rate_limit_window_sec: null },
    })

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toHaveValue(100)
    })

    expect(screen.getByTestId('rate-limit-burst')).toHaveValue(20)
    expect(screen.getByTestId('rate-limit-window')).toHaveValue(60)
  })

  it('refuses to save an empty or zero field when enabled and sends nothing', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({
      config: { rate_limit_requests: 0, rate_limit_burst: 0, rate_limit_window_sec: 0 },
    })
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('save-rate-limit-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('save-rate-limit-btn'))

    expect(await screen.findByTestId('rate-limit-rps-error')).toHaveTextContent(
      'Enter a whole number greater than 0.'
    )
    expect(screen.getByTestId('rate-limit-burst-error')).toBeInTheDocument()
    expect(screen.getByTestId('rate-limit-window-error')).toBeInTheDocument()
    expect(securityApi.updateSecurityConfig).not.toHaveBeenCalled()
  })

  it('refuses a value that is not a whole number and clears its error when edited', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toHaveValue(10)
    })

    const rpsInput = screen.getByTestId('rate-limit-rps')
    await userEvent.clear(rpsInput)
    await userEvent.type(rpsInput, '1.5')
    await userEvent.click(screen.getByTestId('save-rate-limit-btn'))

    expect(await screen.findByTestId('rate-limit-rps-error')).toBeInTheDocument()
    expect(securityApi.updateSecurityConfig).not.toHaveBeenCalled()

    await userEvent.clear(rpsInput)
    await userEvent.type(rpsInput, '40')

    await waitFor(() => {
      expect(screen.queryByTestId('rate-limit-rps-error')).not.toBeInTheDocument()
    })
  })

  it('refuses a typed zero when enabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-window')).toHaveValue(60)
    })

    const windowInput = screen.getByTestId('rate-limit-window')
    await userEvent.clear(windowInput)
    await userEvent.type(windowInput, '0')
    await userEvent.click(screen.getByTestId('save-rate-limit-btn'))

    expect(await screen.findByTestId('rate-limit-window-error')).toBeInTheDocument()
    expect(securityApi.updateSecurityConfig).not.toHaveBeenCalled()
  })

  it('turns the module off with a false setting', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('rate-limit-toggle'))

    await waitFor(() => {
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
        'security.rate_limit.enabled',
        'false',
        'security',
        'bool'
      )
    })
  })

  it('sends edited burst and window values when saved', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-burst')).toHaveValue(5)
    })

    const burstInput = screen.getByTestId('rate-limit-burst')
    await userEvent.clear(burstInput)
    await userEvent.type(burstInput, '8')
    const windowInput = screen.getByTestId('rate-limit-window')
    await userEvent.clear(windowInput)
    await userEvent.type(windowInput, '30')
    await userEvent.click(screen.getByTestId('save-rate-limit-btn'))

    await waitFor(() => {
      expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({
        rate_limit_requests: 10,
        rate_limit_burst: 8,
        rate_limit_window_sec: 30,
      })
    })
  })

  it('explains what the page does with the real text when enabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText('Rate limiting is active and protecting your services')).toBeInTheDocument()
    })
    expect(screen.getByText('10 requests/sec • Burst: 5 • Window: 60s')).toBeInTheDocument()
    expect(screen.getByText('Time window for rate calculations')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save Configuration' })).toBeInTheDocument()
    expect(screen.queryByText('rateLimiting.activeDescription')).not.toBeInTheDocument()
    expect(screen.queryByText('rateLimiting.saveConfiguration')).not.toBeInTheDocument()
  })

  it('explains how to turn rate limiting on with the real text when disabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByText('Rate Limiting Disabled')).toBeInTheDocument()
    })
    expect(
      screen.getByText('Enable rate limiting to configure request limits and protect your services')
    ).toBeInTheDocument()
    expect(screen.getByText('Enable to start limiting request rates')).toBeInTheDocument()
    expect(screen.queryByText('rateLimiting.disabledTitle')).not.toBeInTheDocument()
    expect(screen.queryByText('rateLimiting.disabledDescription')).not.toBeInTheDocument()
  })

  it('confirms a toggle with the localized success message', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    const successSpy = vi.spyOn(toast, 'success')

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('rate-limit-toggle'))

    await waitFor(() => {
      expect(successSpy).toHaveBeenCalledWith('Rate limiting setting updated')
    })
  })

  it('reports a failed toggle with the localized error prefix', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('boom'))
    const errorSpy = vi.spyOn(toast, 'error')

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('rate-limit-toggle'))

    await waitFor(() => {
      expect(errorSpy).toHaveBeenCalledWith('Failed to update: boom')
    })
  })

  it('shows the localized validation message for an empty field', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusEnabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-rps')).toHaveValue(10)
    })
    await userEvent.clear(screen.getByTestId('rate-limit-rps'))
    await userEvent.click(screen.getByTestId('save-rate-limit-btn'))

    expect(await screen.findByTestId('rate-limit-rps-error')).toHaveTextContent('Enter a whole number greater than 0.')
  })

  it('shows an error toast when the toggle fails', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('boom'))
    const errorSpy = vi.spyOn(toast, 'error')

    renderWithProviders(<RateLimiting />)

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('rate-limit-toggle'))

    await waitFor(() => {
      expect(errorSpy).toHaveBeenCalledWith(expect.stringContaining('boom'))
    })
  })

  it('invalidates the status and config queries after the toggle succeeds', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockStatusDisabled)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()

    const qc = createQueryClient()
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <RateLimiting />
        </BrowserRouter>
      </QueryClientProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('rate-limit-toggle')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('rate-limit-toggle'))

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['securityStatus'] })
    })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['securityConfig'] })
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
