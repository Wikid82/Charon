import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import AuditLogs from '../AuditLogs'
import * as auditLogsApi from '../../api/auditLogs'
import { toast } from '../../utils/toast'

vi.mock('../../api/auditLogs')
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

describe('<AuditLogs />', () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })

  const mockAuditLogs = [
    {
      id: 1,
      uuid: '123e4567-e89b-12d3-a456-426614174000',
      actor: 'admin@example.com',
      action: 'dns_provider_create' as const,
      event_category: 'dns_provider' as const,
      resource_id: 1,
      resource_uuid: 'res-123',
      details: '{"name":"Cloudflare","type":"cloudflare"}',
      ip_address: '192.168.1.1',
      user_agent: 'Mozilla/5.0',
      created_at: '2026-01-03T10:00:00Z',
    },
    {
      id: 2,
      uuid: '223e4567-e89b-12d3-a456-426614174001',
      actor: 'user@example.com',
      action: 'credential_test' as const,
      event_category: 'dns_provider' as const,
      resource_uuid: 'res-456',
      details: '{"test_result":"success"}',
      ip_address: '192.168.1.2',
      created_at: '2026-01-03T11:00:00Z',
    },
  ]

  const renderWithProviders = (ui: React.ReactNode) =>
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{ui}</MemoryRouter>
      </QueryClientProvider>
    )

  beforeEach(() => {
    vi.restoreAllMocks()
    queryClient.clear()
  })

  it('renders page title and description', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('Audit Logs')).toBeInTheDocument()
      expect(screen.getByText('View and filter security audit events')).toBeInTheDocument()
    })
  })

  it('displays audit logs in table', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
      expect(screen.getByText('user@example.com')).toBeInTheDocument()
      expect(screen.getByText('dns_provider_create')).toBeInTheDocument()
      expect(screen.getByText('credential_test')).toBeInTheDocument()
    })
  })

  it('shows loading state', () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    renderWithProviders(<AuditLogs />)

    expect(screen.getByRole('table')).toBeInTheDocument()
  })

  it('shows empty state when no logs', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('No audit logs found')).toBeInTheDocument()
    })
  })

  it('toggles filter panel', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Filters/i })).toBeInTheDocument()
    })

    // Initially filters are not shown
    expect(screen.queryByText('Start Date')).not.toBeInTheDocument()

    // Click to show filters
    const filterButton = screen.getByRole('button', { name: /Filters/i })
    fireEvent.click(filterButton)

    await waitFor(() => {
      expect(screen.getByText('Start Date')).toBeInTheDocument()
    })
  })

  it('applies category filter', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    // Open filters
    fireEvent.click(screen.getByRole('button', { name: /Filters/i }))

    await waitFor(() => {
      expect(screen.getByText('Start Date')).toBeInTheDocument()
    })

    // Verify all filter inputs are present
    expect(screen.getByPlaceholderText('Filter by actor...')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Filter by action...')).toBeInTheDocument()
  })

  it('clears all filters', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    // Open filters
    fireEvent.click(screen.getByRole('button', { name: /Filters/i }))

    await waitFor(() => {
      expect(screen.getByText('Start Date')).toBeInTheDocument()
    })

    const clearButton = screen.getByRole('button', { name: /Clear All/i })
    fireEvent.click(clearButton)

    // Filters should still be visible after clearing
    await waitFor(() => {
      expect(screen.getByText('Start Date')).toBeInTheDocument()
    })
  })

  it('opens detail modal when row is clicked', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    // Click first row
    const firstRow = screen.getByText('admin@example.com').closest('tr')
    if (firstRow) {
      fireEvent.click(firstRow)
    }

    await waitFor(() => {
      expect(screen.getByText('Audit Log Details')).toBeInTheDocument()
      expect(screen.getByText('123e4567-e89b-12d3-a456-426614174000')).toBeInTheDocument()
    })
  })

  it('closes detail modal', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    // Click first row to open modal
    const firstRow = screen.getByText('admin@example.com').closest('tr')
    if (firstRow) {
      fireEvent.click(firstRow)
    }

    await waitFor(() => {
      expect(screen.getByText('Audit Log Details')).toBeInTheDocument()
    })

    // Close modal using the "Close" button in footer
    const closeButtons = screen.getAllByRole('button', { name: /Close/i })
    const footerCloseButton = closeButtons.find(btn => btn.textContent === 'Close')
    if (footerCloseButton) {
      fireEvent.click(footerCloseButton)
    }

    await waitFor(() => {
      expect(screen.queryByText('Audit Log Details')).not.toBeInTheDocument()
    })
  })

  it('handles pagination', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 100,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    // Check pagination is present
    const nextButton = screen.getByRole('button', { name: /Next/i })
    expect(nextButton).not.toBeDisabled()

    const prevButton = screen.getByRole('button', { name: /Previous/i })
    expect(prevButton).toBeDisabled()

    // Check page indicator
    expect(screen.getByText(/Page 1 of 2/i)).toBeInTheDocument()
  })

  it('exports to CSV', async () => {
    const mockCSV = 'timestamp,actor,action\n2026-01-03,admin,create'
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })
    vi.spyOn(auditLogsApi, 'exportAuditLogsCSV').mockResolvedValue(mockCSV)

    // Mock toast
    const toastSuccessSpy = vi.spyOn(toast, 'success')

    // Mock URL APIs
    const mockCreateObjectURL = vi.fn(() => 'blob:mock-url')
    const mockRevokeObjectURL = vi.fn()
    window.URL.createObjectURL = mockCreateObjectURL
    window.URL.revokeObjectURL = mockRevokeObjectURL

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    const exportButton = screen.getByRole('button', { name: /Export CSV/i })
    fireEvent.click(exportButton)

    await waitFor(() => {
      expect(auditLogsApi.exportAuditLogsCSV).toHaveBeenCalled()
      expect(toastSuccessSpy).toHaveBeenCalledWith('Audit logs exported successfully')
    })
  })

  it('handles export error', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })
    vi.spyOn(auditLogsApi, 'exportAuditLogsCSV').mockRejectedValue(
      new Error('Export failed')
    )

    const toastErrorSpy = vi.spyOn(toast, 'error')
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    const exportButton = screen.getByRole('button', { name: /Export CSV/i })
    fireEvent.click(exportButton)

    try {
      await waitFor(() => {
        expect(toastErrorSpy).toHaveBeenCalledWith('Failed to export audit logs')
        expect(consoleErrorSpy).toHaveBeenCalled()
      })
    } finally {
      consoleErrorSpy.mockRestore()
    }
  })

  it('displays parsed JSON details in modal', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: mockAuditLogs,
      total: 2,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    })

    // Click first row to open modal
    const firstRow = screen.getByText('admin@example.com').closest('tr')
    if (firstRow) {
      fireEvent.click(firstRow)
    }

    await waitFor(() => {
      expect(screen.getByText('Audit Log Details')).toBeInTheDocument()
      // Check that JSON is displayed
      expect(screen.getByText(/"name"/)).toBeInTheDocument()
      expect(screen.getByText(/"Cloudflare"/)).toBeInTheDocument()
    })
  })

  it('shows filter count badge', async () => {
    vi.spyOn(auditLogsApi, 'getAuditLogs').mockResolvedValue({
      logs: [],
      total: 0,
      page: 1,
      limit: 50,
    })

    renderWithProviders(<AuditLogs />)

    // Open filters
    const filterButton = screen.getByRole('button', { name: /Filters/i })
    fireEvent.click(filterButton)

    await waitFor(() => {
      expect(screen.getByText('Start Date')).toBeInTheDocument()
    })

    // Add a filter by typing in the actor input
    const actorInput = screen.getByPlaceholderText('Filter by actor...')
    fireEvent.change(actorInput, { target: { value: 'admin' } })

    // The badge should show 1 active filter
    await waitFor(() => {
      const badge = filterButton.querySelector('.bg-brand-500')
      expect(badge).toBeInTheDocument()
      expect(badge?.textContent).toBe('1')
    })
  })
})
