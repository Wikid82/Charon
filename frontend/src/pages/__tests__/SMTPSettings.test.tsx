import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import SMTPSettings from '../SMTPSettings'
import * as smtpApi from '../../api/smtp'

// Mock API
vi.mock('../../api/smtp', () => ({
  getSMTPConfig: vi.fn(),
  updateSMTPConfig: vi.fn(),
  testSMTPConnection: vi.fn(),
  sendTestEmail: vi.fn(),
}))

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('SMTPSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders loading state initially', () => {
    vi.mocked(smtpApi.getSMTPConfig).mockReturnValue(new Promise(() => {}))

    renderWithProviders(<SMTPSettings />)

    // Should show loading spinner
    expect(document.querySelector('.animate-spin')).toBeTruthy()
  })

  it('renders SMTP form with existing config', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: 'smtp.example.com',
      port: 587,
      username: 'user@example.com',
      password: '********',
      from_address: 'noreply@example.com',
      encryption: 'starttls',
      configured: true,
    })

    renderWithProviders(<SMTPSettings />)

    // Wait for the form to populate with data
    await waitFor(() => {
      const hostInput = screen.getByPlaceholderText('smtp.gmail.com') as HTMLInputElement
      return hostInput.value === 'smtp.example.com'
    })

    const hostInput = screen.getByPlaceholderText('smtp.gmail.com') as HTMLInputElement
    expect(hostInput.value).toBe('smtp.example.com')

    const portInput = screen.getByPlaceholderText('587') as HTMLInputElement
    expect(portInput.value).toBe('587')

    expect(screen.getByText('SMTP Configured')).toBeTruthy()
  })

  it('shows not configured state when SMTP is not set up', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: '',
      port: 587,
      username: '',
      password: '',
      from_address: '',
      encryption: 'starttls',
      configured: false,
    })

    renderWithProviders(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText('SMTP Not Configured')).toBeTruthy()
    })
  })

  it('saves SMTP settings successfully', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: '',
      port: 587,
      username: '',
      password: '',
      from_address: '',
      encryption: 'starttls',
      configured: false,
    })
    vi.mocked(smtpApi.updateSMTPConfig).mockResolvedValue({
      message: 'SMTP configuration saved successfully',
    })

    renderWithProviders(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByPlaceholderText('smtp.gmail.com')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('smtp.gmail.com'), 'smtp.gmail.com')
    await user.type(
      screen.getByPlaceholderText('Charon <no-reply@example.com>'),
      'test@example.com'
    )

    await user.click(screen.getByRole('button', { name: 'Save Settings' }))

    await waitFor(() => {
      expect(smtpApi.updateSMTPConfig).toHaveBeenCalled()
    })
  })

  it('tests SMTP connection', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: 'smtp.example.com',
      port: 587,
      username: 'user@example.com',
      password: '********',
      from_address: 'noreply@example.com',
      encryption: 'starttls',
      configured: true,
    })
    vi.mocked(smtpApi.testSMTPConnection).mockResolvedValue({
      success: true,
      message: 'Connection successful',
    })

    renderWithProviders(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText('Test Connection')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.click(screen.getByText('Test Connection'))

    await waitFor(() => {
      expect(smtpApi.testSMTPConnection).toHaveBeenCalled()
    })
  })

  it('shows test email form when SMTP is configured', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: 'smtp.example.com',
      port: 587,
      username: 'user@example.com',
      password: '********',
      from_address: 'noreply@example.com',
      encryption: 'starttls',
      configured: true,
    })

    renderWithProviders(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText('Send Test Email')).toBeTruthy()
    })

    expect(screen.getByPlaceholderText('recipient@example.com')).toBeTruthy()
  })

  it('sends test email', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: 'smtp.example.com',
      port: 587,
      username: 'user@example.com',
      password: '********',
      from_address: 'noreply@example.com',
      encryption: 'starttls',
      configured: true,
    })
    vi.mocked(smtpApi.sendTestEmail).mockResolvedValue({
      success: true,
      message: 'Email sent',
    })

    renderWithProviders(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText('Send Test Email')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(
      screen.getByPlaceholderText('recipient@example.com'),
      'test@test.com'
    )
    await user.click(screen.getByRole('button', { name: /Send Test/i }))

    await waitFor(() => {
      expect(smtpApi.sendTestEmail).toHaveBeenCalledWith({ to: 'test@test.com' })
    })
  })
})
