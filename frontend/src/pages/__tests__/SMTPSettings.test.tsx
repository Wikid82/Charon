import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import SMTPSettings from '../SMTPSettings'
import * as smtpApi from '../../api/smtp'
import { toast } from '../../utils/toast'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'

const translations: Record<string, string> = {
  'smtp.configured': 'SMTP Configured',
  'smtp.notConfigured': 'SMTP Not Configured',
  'smtp.saveSettings': 'Save Settings',
  'smtp.testConnection': 'Test Connection',
  'smtp.sendTestEmail': 'Send Test Email',
  'smtp.sendTest': 'Send Test',
}

const t = (key: string) => translations[key] ?? key

// Mock API
vi.mock('../../api/smtp', () => ({
  getSMTPConfig: vi.fn(),
  updateSMTPConfig: vi.fn(),
  testSMTPConnection: vi.fn(),
  sendTestEmail: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('SMTPSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(toast.success).mockClear()
    vi.mocked(toast.error).mockClear()
  })

  it('renders loading state initially', () => {
    vi.mocked(smtpApi.getSMTPConfig).mockReturnValue(new Promise(() => {}))

    renderWithQueryClient(<SMTPSettings />)

    // Should show loading skeletons (Skeleton components don't use animate-spin)
    expect(document.querySelectorAll('[class*="animate-pulse"]').length).toBeGreaterThan(0)
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

    renderWithQueryClient(<SMTPSettings />)

    // Wait for the form to populate with data
    await waitFor(() => {
      const hostInput = screen.getByPlaceholderText('smtp.gmail.com') as HTMLInputElement
      return hostInput.value === 'smtp.example.com'
    })

    const hostInput = screen.getByPlaceholderText('smtp.gmail.com') as HTMLInputElement
    expect(hostInput.value).toBe('smtp.example.com')

    const portInput = screen.getByPlaceholderText('587') as HTMLInputElement
    expect(portInput.value).toBe('587')

    expect(screen.getByText(t('smtp.configured'))).toBeTruthy()
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

    renderWithQueryClient(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText(t('smtp.notConfigured'))).toBeTruthy()
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

    renderWithQueryClient(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByPlaceholderText('smtp.gmail.com')).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(screen.getByPlaceholderText('smtp.gmail.com'), 'smtp.gmail.com')
    await user.type(
      screen.getByPlaceholderText('Charon <no-reply@example.com>'),
      'test@example.com'
    )

    await user.click(screen.getByRole('button', { name: t('smtp.saveSettings') }))

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

    renderWithQueryClient(<SMTPSettings />)

    const testButton = await screen.findByRole('button', { name: t('smtp.testConnection') })
    await waitFor(() => expect(testButton).toBeEnabled())

    const user = userEvent.setup()
    await user.click(testButton)

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

    renderWithQueryClient(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText(t('smtp.sendTestEmail'))).toBeTruthy()
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

    renderWithQueryClient(<SMTPSettings />)

    await waitFor(() => {
      expect(screen.getByText(t('smtp.sendTestEmail'))).toBeTruthy()
    })

    const user = userEvent.setup()
    await user.type(
      screen.getByPlaceholderText('recipient@example.com'),
      'test@test.com'
    )
    await user.click(screen.getByRole('button', { name: t('smtp.sendTest') }))

    await waitFor(() => {
      expect(smtpApi.sendTestEmail).toHaveBeenCalledWith({ to: 'test@test.com' })
    })
  })

  it('surfaces backend validation errors on save', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: '',
      port: 587,
      username: '',
      password: '',
      from_address: '',
      encryption: 'starttls',
      configured: false,
    })
    vi.mocked(smtpApi.updateSMTPConfig).mockRejectedValue({ response: { data: { error: 'invalid host' } } })

    renderWithQueryClient(<SMTPSettings />)

    const user = userEvent.setup()
    await waitFor(() => expect(screen.getByPlaceholderText('smtp.gmail.com')).toBeInTheDocument())
    await user.type(screen.getByPlaceholderText('smtp.gmail.com'), 'bad.host')
    await user.type(screen.getByPlaceholderText('Charon <no-reply@example.com>'), 'ops@example.com')

    await user.click(screen.getByRole('button', { name: t('smtp.saveSettings') }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('invalid host')
    })
  })

  it('disables test connection until required fields are set and shows error toast on failure', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: '',
      port: 587,
      username: '',
      password: '',
      from_address: '',
      encryption: 'starttls',
      configured: false,
    })
    vi.mocked(smtpApi.testSMTPConnection).mockRejectedValue({ response: { data: { error: 'cannot connect' } } })

    renderWithQueryClient(<SMTPSettings />)

    const user = userEvent.setup()
    await waitFor(() => expect(screen.getByText(t('smtp.testConnection'))).toBeInTheDocument())

    // Button should start disabled until host and from address are provided
    const testButton = screen.getByRole('button', { name: t('smtp.testConnection') })
    expect(testButton).toBeDisabled()

    await user.type(screen.getByPlaceholderText('smtp.gmail.com'), 'smtp.acme.local')
    await user.type(screen.getByPlaceholderText('Charon <no-reply@example.com>'), 'from@acme.local')

    await waitFor(() => expect(testButton).toBeEnabled())
    await user.click(testButton)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('cannot connect')
    })
  })

  it('handles test email failures and keeps input value intact', async () => {
    vi.mocked(smtpApi.getSMTPConfig).mockResolvedValue({
      host: 'smtp.example.com',
      port: 587,
      username: 'user@example.com',
      password: '********',
      from_address: 'noreply@example.com',
      encryption: 'starttls',
      configured: true,
    })
    vi.mocked(smtpApi.sendTestEmail).mockRejectedValue({ response: { data: { error: 'smtp unreachable' } } })

    renderWithQueryClient(<SMTPSettings />)

    const user = userEvent.setup()
    await waitFor(() => expect(screen.getByText(t('smtp.sendTestEmail'))).toBeInTheDocument())
    const input = screen.getByPlaceholderText('recipient@example.com') as HTMLInputElement
    await user.type(input, 'keepme@example.com')

    await user.click(screen.getByRole('button', { name: t('smtp.sendTest') }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('smtp unreachable')
      expect(input.value).toBe('keepme@example.com')
    })
  })
})
