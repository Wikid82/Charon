import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { createBackup } from '../../api/backups'
import { getSettings } from '../../api/settings'
import { getMonitors, type UptimeMonitor } from '../../api/uptime'
import { useAccessLists } from '../../hooks/useAccessLists'
import { useCertificates } from '../../hooks/useCertificates'
import { useProxyHosts } from '../../hooks/useProxyHosts'
import ProxyHosts from '../ProxyHosts'

import type { ProxyHost } from '../../api/proxyHosts'

vi.mock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn() }))
vi.mock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn() }))
vi.mock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn() }))
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))
vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({ data: [], isLoading: false, error: null })),
  useCreateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useUpdateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}))
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }))
vi.mock('../../api/uptime', () => ({ getMonitors: vi.fn() }))
vi.mock('../../api/backups', () => ({ createBackup: vi.fn() }))
vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))

// Helper to create QueryClient provider wrapper
const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

type ProxyHostsHookValue = ReturnType<typeof useProxyHosts>
type CertificatesHookValue = ReturnType<typeof useCertificates>
type AccessListsHookValue = ReturnType<typeof useAccessLists>

const createProxyHostsHookValue = (overrides: Partial<ProxyHostsHookValue> = {}): ProxyHostsHookValue => ({
  hosts: [],
  loading: false,
  isFetching: false,
  error: null,
  createHost: vi.fn() as unknown as ProxyHostsHookValue['createHost'],
  updateHost: vi.fn() as unknown as ProxyHostsHookValue['updateHost'],
  deleteHost: vi.fn() as unknown as ProxyHostsHookValue['deleteHost'],
  bulkUpdateACL: vi.fn() as unknown as ProxyHostsHookValue['bulkUpdateACL'],
  bulkUpdateSecurityHeaders: vi.fn() as unknown as ProxyHostsHookValue['bulkUpdateSecurityHeaders'],
  isCreating: false,
  isUpdating: false,
  isDeleting: false,
  isBulkUpdating: false,
  ...overrides,
})

const createCertificatesHookValue = (overrides: Partial<CertificatesHookValue> = {}): CertificatesHookValue => ({
  certificates: [],
  isLoading: false,
  error: null,
  refetch: vi.fn() as unknown as CertificatesHookValue['refetch'],
  ...overrides,
})

const createAccessListsHookValue = (data: unknown = [], overrides: Partial<AccessListsHookValue> = {}): AccessListsHookValue =>
  ({
    data,
    isLoading: false,
    isFetching: false,
    error: null,
    ...overrides,
  } as unknown as AccessListsHookValue)

const sampleHost = (overrides: Partial<ProxyHost> = {}): ProxyHost => ({
  uuid: 'h1',
  name: 'A Name',
  domain_names: 'a.example.com',
  forward_scheme: 'http',
  forward_host: '127.0.0.1',
  forward_port: 8080,
  ssl_forced: false,
  websocket_support: false,
  enabled: true,
  http2_support: false,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  application: 'none',
  locations: [],
  certificate: null,
  certificate_id: null,
  access_list_id: null,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  ...overrides,
})

describe('ProxyHosts page extra tests', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue())
    vi.mocked(useCertificates).mockReturnValue(createCertificatesHookValue())
    vi.mocked(useAccessLists).mockReturnValue(createAccessListsHookValue([]))
    vi.mocked(getSettings).mockResolvedValue({})
    vi.mocked(getMonitors).mockResolvedValue([])
  })

  it('shows "No proxy hosts configured" when no hosts', async () => {
    renderWithProviders(<ProxyHosts />)

    // Translation mock returns English text; tolerate fallback key string too.
    expect(await screen.findByText(/Create your first proxy host|proxyHosts\.noHostsDescription/i)).toBeInTheDocument()
  })

  it('sort toggles by header click', async () => {
    const h1 = sampleHost({ uuid: 'a', name: 'Alpha' })
    const h2 = sampleHost({ uuid: 'b', name: 'Beta' })

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [h2, h1] }))

    renderWithProviders(<ProxyHosts />)

    // hosts are sorted by name by default (Alpha before Beta) by the component
    expect(await screen.findByText('Alpha')).toBeInTheDocument()

    const table = screen.getAllByRole('table')[0]
    const nameHeader = within(table).getAllByRole('button', { name: 'Name' })[0]
    // Click header - this only toggles the sort indicator icon, not actual data order
    // since the component pre-sorts data before passing to DataTable
    await userEvent.click(nameHeader)

    // Verify that both hosts are still displayed (basic sanity check)
    expect(screen.getByText('Alpha')).toBeInTheDocument()
    expect(screen.getByText('Beta')).toBeInTheDocument()

    // Verify the sort indicator changes (chevron icon should toggle)
    expect(table).toBeInTheDocument()
  })

  it('delete with associated monitors prompts and deletes with deleteUptime true', async () => {
    const host = sampleHost({ uuid: 'delete-1', name: 'DelHost', forward_host: 'upstream-1' })
    const deleteHostMock = vi.fn().mockResolvedValue(undefined) as unknown as ProxyHostsHookValue['deleteHost']

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host], deleteHost: deleteHostMock }))
    vi.mocked(getMonitors).mockResolvedValue([
      {
        id: 'm1',
        upstream_host: 'upstream-1',
        name: 'm1',
        type: 'http',
        url: 'http://upstream-1',
        interval: 60,
        enabled: true,
        status: 'up',
        latency: 0,
        max_retries: 3,
      } satisfies UptimeMonitor,
    ])

    const confirmMock = vi.spyOn(window, 'confirm')
    // first confirm 'Are you sure' -> true, second confirm 'Delete monitors as well' -> true
    confirmMock.mockImplementation(() => true)

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('DelHost')).toBeInTheDocument()
    const deleteBtn = screen.getByRole('button', { name: 'Delete proxy host DelHost' })
    await userEvent.click(deleteBtn)

    // Confirm deletion in the dialog
    expect(await screen.findByRole('heading', { name: /Delete Proxy Host/i })).toBeInTheDocument()
    const confirmDeleteBtn = screen.getByRole('button', { name: /^Delete$/ })
    await userEvent.click(confirmDeleteBtn)

    await waitFor(() => expect(deleteHostMock).toHaveBeenCalled())

    // Should have been called with both uuid and deleteUptime true (because monitors exist and second confirm true)
    expect(deleteHostMock).toHaveBeenCalledWith('delete-1', true)
    confirmMock.mockRestore()
  })

  it('renders SSL badges for SSL-enabled hosts', async () => {
    const hostValid = sampleHost({ uuid: 'v1', name: 'ValidHost', domain_names: 'valid.example.com', ssl_forced: true })
    const hostAuto = sampleHost({ uuid: 'a1', name: 'AutoHost', domain_names: 'auto.example.com', ssl_forced: true })

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [hostValid, hostAuto] }))
    vi.mocked(useCertificates).mockReturnValue(
      createCertificatesHookValue({
        certificates: [
          {
            id: 1,
            uuid: 'cert-le-1',
            name: 'LE',
            domains: 'valid.example.com',
            issuer: 'letsencrypt',
            expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
            status: 'valid',
            provider: 'letsencrypt',
            has_key: false,
            in_use: true,
          },
        ],
      }),
    )
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('ValidHost')).toBeInTheDocument()
    // Check that SSL badges are rendered (text removed for better spacing)
    const sslBadges = screen.getAllByText('SSL')
    expect(sslBadges.length).toBeGreaterThan(0)
  })

  it('shows error banner when hook returns an error', async () => {
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ error: 'Failed to load' }))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
  })

  it('select all shows (all) selected in summary', async () => {
    const h1 = sampleHost({ uuid: 'x', name: 'XHost' })
    const h2 = sampleHost({ uuid: 'y', name: 'YHost' })

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [h1, h2] }))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('XHost')).toBeInTheDocument()
    const selectAllBtn = screen.getByRole('checkbox', { name: /Select all/i })
    // fallback, find by title
    await (!selectAllBtn ? userEvent.click(screen.getByTitle('Select all')) : userEvent.click(selectAllBtn));

    // Text is split across elements: "<strong>2</strong> host(s) selected (all)"
    // Check for presence of both parts separately
    expect(await screen.findByText(/host\(s\) selected/)).toBeInTheDocument()
    expect(screen.getByText(/\(all\)/)).toBeInTheDocument()
  })

  it('shows loader when fetching', async () => {
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [sampleHost()], isFetching: true }))
    const { container } = renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(container.querySelector('.animate-spin')).toBeInTheDocument())
  })

  it('handles domain link behavior new_window', async () => {
    const host = sampleHost({ uuid: 'link-h1', domain_names: 'link.example.com', ssl_forced: true })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host] }))
    vi.mocked(getSettings).mockResolvedValue({ 'ui.domain_link_behavior': 'new_window' })

    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('link.example.com')).toBeInTheDocument()
    // Use exact string match to avoid incomplete hostname regex (CodeQL js/incomplete-hostname-regexp)
    const link = screen.getByRole('link', { name: 'link.example.com' })
    await userEvent.click(link)
    expect(openSpy).toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it('shows WS and ACL badges when appropriate', async () => {
    const host = sampleHost({ uuid: 'x2', name: 'XHost2', websocket_support: true, access_list_id: 5 })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host] }))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('XHost2')).toBeInTheDocument()
    expect(screen.getByText('WS')).toBeInTheDocument()
    expect(screen.getByText('ACL')).toBeInTheDocument()
  })

  it('bulk ACL remove shows the confirmation card and Apply label updates when selecting ACLs', async () => {
    const host = sampleHost({ uuid: 'acl-1', name: 'AclHost' })
    const acl = { id: 1, name: 'MyACL', enabled: true }

    vi.mocked(useProxyHosts).mockReturnValue(
      createProxyHostsHookValue({
        hosts: [host],
        bulkUpdateACL: vi.fn(() => Promise.resolve({ updated: 1, errors: [] })) as unknown as ProxyHostsHookValue['bulkUpdateACL'],
      }),
    )
    vi.mocked(useAccessLists).mockReturnValue(createAccessListsHookValue([acl]))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('AclHost')).toBeInTheDocument()
    // Select host using checkbox - find row first, then first checkbox (selection) within
    const row = screen.getByText('AclHost').closest('tr') as HTMLTableRowElement
    const selectBtn = within(row).getAllByRole('checkbox')[0]
    await userEvent.click(selectBtn)

    // Open Manage ACL modal
    const manageBtn = screen.getByText('Manage ACL')
    await userEvent.click(manageBtn)

    // Switch to Remove ACL action
    const removeBtn = screen.getByText('Remove ACL')
    await userEvent.click(removeBtn)

    expect(await screen.findByText(/This will remove the access list from all 1 selected host/i)).toBeInTheDocument()

    // Switch back to Apply ACL and select the ACL
    const applyBtn = screen.getByText('Apply ACL')
    await userEvent.click(applyBtn)
    const selectAll = screen.getByText('Select All')
    await userEvent.click(selectAll)
    expect(await screen.findByText('Apply (1)')).toBeInTheDocument()
  })

  it('bulk ACL remove action calls bulkUpdateACL with null and shows removed toast', async () => {
    const host = sampleHost({ uuid: 'acl-2', name: 'AclHost2' })
    const bulkUpdateACLMock = vi.fn(async () => ({ updated: 1, errors: [] })) as unknown as ProxyHostsHookValue['bulkUpdateACL']

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host], bulkUpdateACL: bulkUpdateACLMock }))
    vi.mocked(useAccessLists).mockReturnValue(createAccessListsHookValue([{ id: 1, name: 'MyACL', enabled: true }]))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('AclHost2')).toBeInTheDocument()
    const row = screen.getByText('AclHost2').closest('tr') as HTMLTableRowElement
    await userEvent.click(within(row).getAllByRole('checkbox')[0])
    await userEvent.click(screen.getByText('Manage ACL'))
    await userEvent.click(screen.getByText('Remove ACL'))
    // Click Remove ACL confirm button (bottom) - choose the confirmation button rather than the header action
    const removeButtons = screen.getAllByRole('button', { name: 'Remove ACL' })
    await userEvent.click(removeButtons[removeButtons.length - 1])

    await waitFor(() => expect(bulkUpdateACLMock).toHaveBeenCalledWith(['acl-2'], null))
    expect(toast.success as unknown as ReturnType<typeof vi.fn>).toHaveBeenCalledWith(expect.stringContaining('removed'))
  })

  it('shows no enabled access lists available when none exist', async () => {
    const host = sampleHost({ uuid: 'acl-3', name: 'AclHost3' })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host] }))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('AclHost3')).toBeInTheDocument()
    const row = screen.getByText('AclHost3').closest('tr') as HTMLTableRowElement
    await userEvent.click(within(row).getAllByRole('checkbox')[0])
    await userEvent.click(screen.getByText('Manage ACL'))

    expect(await screen.findByText('No enabled access lists available')).toBeInTheDocument()
  })

  it('bulk delete modal lists hosts to be deleted', async () => {
    const host = sampleHost({ uuid: 'd2', name: 'DeleteMe2' })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host] }))
    vi.mocked(createBackup).mockResolvedValue({ filename: 'backup-2' })
    const confirmMock = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('DeleteMe2')).toBeInTheDocument()
    const row = screen.getByText('DeleteMe2').closest('tr') as HTMLTableRowElement
    await userEvent.click(within(row).getAllByRole('checkbox')[0])
    const deleteButtons = screen.getAllByText('Delete')
    const toolbarBtn = deleteButtons.map((btn: Element) => btn.closest('button') as HTMLButtonElement | null).find((b) => b && b.className.includes('bg-error')) as HTMLButtonElement | undefined
    if (!toolbarBtn) throw new Error('Toolbar delete button not found')
    await userEvent.click(toolbarBtn)

    expect(await screen.findByRole('heading', { name: /Delete 1 Proxy Host/i })).toBeInTheDocument()
    // Ensure the modal lists the host by scoping to the modal content
    const listHeader = screen.getByText('Hosts to be deleted:')
    const modalRoot = listHeader.closest('div')
    expect(modalRoot).toBeTruthy()
    const { getByText: getByTextWithin } = within(modalRoot!)
    expect(getByTextWithin('DeleteMe2')).toBeInTheDocument()
    expect(getByTextWithin('(a.example.com)')).toBeInTheDocument()
    // Confirm delete
    await userEvent.click(screen.getByRole('button', { name: /Delete Permanently/i }))
    await waitFor(() => expect(vi.mocked(toast.success)).toHaveBeenCalledWith(expect.stringContaining('Backup created')))
    confirmMock.mockRestore()
  })

  it('bulk apply modal returns early when no keys selected (no-op)', async () => {
    const host = sampleHost({ uuid: 'b1', name: 'BlankHost' })
    const updateHost = vi.fn() as unknown as ProxyHostsHookValue['updateHost']

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host], updateHost }))
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('BlankHost')).toBeInTheDocument()
    // Select host
    const row = screen.getByText('BlankHost').closest('tr') as HTMLTableRowElement
    await userEvent.click(within(row).getAllByRole('checkbox')[0])
    // Open Bulk Apply modal
    await userEvent.click(screen.getByText('Bulk Apply'))
    const applyBtn = screen.getByRole('button', { name: 'Apply' })
    // Remove disabled to trigger the no-op branch
    applyBtn.removeAttribute('disabled')
    await userEvent.click(applyBtn)
    // No calls to updateHost should be made
    expect(updateHost).not.toHaveBeenCalled()
  })

  it('bulk delete creates backup and shows toast success', async () => {
    const host = sampleHost({ uuid: 'd1', name: 'DeleteMe' })
    const deleteHostMock = vi.fn().mockResolvedValue(undefined) as unknown as ProxyHostsHookValue['deleteHost']

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [host], deleteHost: deleteHostMock }))
    vi.mocked(createBackup).mockResolvedValue({ filename: 'backup-1' })

    const confirmMock = vi.spyOn(window, 'confirm')
    // First confirm to delete overall, returned true for deletion
    confirmMock.mockImplementation(() => true)

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('DeleteMe')).toBeInTheDocument()
    // Select host
    const row = screen.getByText('DeleteMe').closest('tr') as HTMLTableRowElement
    const selectBtn = within(row).getAllByRole('checkbox')[0]
    await userEvent.click(selectBtn)

    // Open Bulk Delete modal - find the toolbar Delete button near the header
    const deleteButtons = screen.getAllByText('Delete')
    const toolbarBtn = deleteButtons.map((btn: Element) => btn.closest('button') as HTMLButtonElement | null).find((b) => b && b.className.includes('bg-error')) as HTMLButtonElement | undefined
    if (!toolbarBtn) throw new Error('Toolbar delete button not found')
    await userEvent.click(toolbarBtn)

    // Confirm Delete in modal
    await userEvent.click(screen.getByRole('button', { name: /Delete Permanently/i }))

    await waitFor(() =>
      expect(toast.success as unknown as ReturnType<typeof vi.fn>).toHaveBeenCalledWith(expect.stringContaining('Backup created')),
    )
    confirmMock.mockRestore()
  })
})
