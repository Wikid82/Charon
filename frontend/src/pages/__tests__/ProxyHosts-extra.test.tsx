import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import '@testing-library/jest-dom'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ProxyHost } from '../../api/proxyHosts'

// Helper to create QueryClient provider wrapper
const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

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
    vi.resetModules()
    vi.clearAllMocks()
  })

  it('shows "No proxy hosts configured" when no hosts', async () => {
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('No proxy hosts configured yet. Click "Add Proxy Host" to get started.')).toBeInTheDocument())
  })

  it('sort toggles by header click', async () => {
    const h1 = sampleHost({ uuid: 'a', name: 'Alpha' })
    const h2 = sampleHost({ uuid: 'b', name: 'Beta' })

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [h2, h1], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')

    renderWithProviders(<ProxyHosts />)

    // initial order Beta, Alpha (as provided)
    await waitFor(() => expect(screen.getByText('Beta')).toBeInTheDocument())

    const nameHeader = screen.getByText('Name')
    await userEvent.click(nameHeader)
    // click toggles sort direction when same column clicked again
    await userEvent.click(nameHeader)

    // After toggling, expect DOM order to include Alpha then Beta
    const rows = screen.getAllByRole('row')
    // find first data row name cell
    const firstHostCell = rows.slice(1)[0].querySelector('td')
    expect(firstHostCell).toBeTruthy()
    if (firstHostCell) expect(firstHostCell.textContent).toContain('Alpha')
  })

  it('delete with associated monitors prompts and deletes with deleteUptime true', async () => {
    const host = sampleHost({ uuid: 'delete-1', name: 'DelHost', forward_host: 'upstream-1' })
    const deleteHostMock = vi.fn().mockResolvedValue(undefined)

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: deleteHostMock, bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))
    vi.doMock('../../api/uptime', () => ({ getMonitors: vi.fn(() => Promise.resolve([{ id: 1, upstream_host: 'upstream-1', proxy_host_id: null }])) }))

    const confirmMock = vi.spyOn(window, 'confirm')
    // first confirm 'Are you sure' -> true, second confirm 'Delete monitors as well' -> true
    confirmMock.mockImplementation(() => true)

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('DelHost')).toBeInTheDocument())
    const deleteBtn = screen.getByText('Delete')
    await userEvent.click(deleteBtn)

    await waitFor(() => expect(deleteHostMock).toHaveBeenCalled())

    // Should have been called with both uuid and deleteUptime true (because monitors exist and second confirm true)
    expect(deleteHostMock).toHaveBeenCalledWith('delete-1', true)
    confirmMock.mockRestore()
  })

  it('renders SSL badges for SSL-enabled hosts', async () => {
    const hostValid = sampleHost({ uuid: 'v1', name: 'ValidHost', domain_names: 'valid.example.com', ssl_forced: true })
    const hostAuto = sampleHost({ uuid: 'a1', name: 'AutoHost', domain_names: 'auto.example.com', ssl_forced: true })

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [hostValid, hostAuto], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [{ id: 1, name: 'LE', domain: 'valid.example.com', status: 'valid', provider: 'letsencrypt' }], isLoading: false, error: null })) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('ValidHost')).toBeInTheDocument())
    // Check that SSL badges are rendered (text removed for better spacing)
    const sslBadges = screen.getAllByText('SSL')
    expect(sslBadges.length).toBeGreaterThan(0)
  })

  it('shows error banner when hook returns an error', async () => {
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [], loading: false, isFetching: false, error: 'Failed to load', createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('Failed to load')).toBeInTheDocument())
  })

  it('select all shows (all) selected in summary', async () => {
    const h1 = sampleHost({ uuid: 'x', name: 'XHost' })
    const h2 = sampleHost({ uuid: 'y', name: 'YHost' })

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [h1, h2], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('XHost')).toBeInTheDocument())
    const selectAllBtn = screen.getByRole('checkbox', { name: /Select all/i })
    // fallback, find by title
    if (!selectAllBtn) {
      await userEvent.click(screen.getByTitle('Select all'))
    } else {
      await userEvent.click(selectAllBtn)
    }

    await waitFor(() => expect(screen.getByText(/\(all\)\s*selected/)).toBeInTheDocument())
  })

  it('shows loader when fetching', async () => {
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [sampleHost()], loading: false, isFetching: true, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    const { container } = renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(container.querySelector('.animate-spin')).toBeInTheDocument())
  })

  it('handles domain link behavior new_window', async () => {
    const host = sampleHost({ uuid: 'link-h1', domain_names: 'link.example.com', ssl_forced: true })
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({ 'ui.domain_link_behavior': 'new_window' })) }))

    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null as any)

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('link.example.com')).toBeInTheDocument())
    const link = screen.getByRole('link', { name: /link.example.com/ })
    await userEvent.click(link)
    expect(openSpy).toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it('shows WS and ACL badges when appropriate', async () => {
    const host = sampleHost({ uuid: 'x2', name: 'XHost2', websocket_support: true, access_list_id: 5 })
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('XHost2')).toBeInTheDocument())
    expect(screen.getByText('WS')).toBeInTheDocument()
    expect(screen.getByText('ACL')).toBeInTheDocument()
  })

  it('bulk ACL remove shows the confirmation card and Apply label updates when selecting ACLs', async () => {
    const host = sampleHost({ uuid: 'acl-1', name: 'AclHost' })
    const acl = { id: 1, name: 'MyACL', enabled: true }

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(() => Promise.resolve({ updated: 1, errors: [] })), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [acl] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('AclHost')).toBeInTheDocument())
    // Select host using checkbox
    const selectBtn = screen.getByLabelText('Select AclHost')
    await userEvent.click(selectBtn)

    // Open Manage ACL modal
    const manageBtn = screen.getByText('Manage ACL')
    await userEvent.click(manageBtn)

    // Switch to Remove ACL action
    const removeBtn = screen.getByText('Remove ACL')
    await userEvent.click(removeBtn)

    await waitFor(() => expect(screen.getByText(/This will remove the access list from all 1 selected host/i)).toBeInTheDocument())

    // Switch back to Apply ACL and select the ACL
    const applyBtn = screen.getByText('Apply ACL')
    await userEvent.click(applyBtn)
    const selectAll = screen.getByText('Select All')
    await userEvent.click(selectAll)
    await waitFor(() => expect(screen.getByText('Apply (1)')).toBeInTheDocument())
  })

  it('bulk ACL remove action calls bulkUpdateACL with null and shows removed toast', async () => {
    const host = sampleHost({ uuid: 'acl-2', name: 'AclHost2' })
    const bulkUpdateACLMock = vi.fn(async () => ({ updated: 1, errors: [] }))
    const toastSuccess = vi.fn()
    vi.doMock('react-hot-toast', () => ({ toast: { success: toastSuccess, error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: bulkUpdateACLMock, isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [{ id: 1, name: 'MyACL', enabled: true }] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('AclHost2')).toBeInTheDocument())
    await userEvent.click(screen.getByLabelText('Select AclHost2'))
    await userEvent.click(screen.getByText('Manage ACL'))
    await userEvent.click(screen.getByText('Remove ACL'))
    // Click Remove ACL confirm button (bottom) - choose the confirmation button rather than the header action
    const removeButtons = screen.getAllByRole('button', { name: 'Remove ACL' })
    await userEvent.click(removeButtons[removeButtons.length - 1])

    await waitFor(() => expect(bulkUpdateACLMock).toHaveBeenCalledWith(['acl-2'], null))
    expect(toastSuccess).toHaveBeenCalledWith(expect.stringContaining('removed'))
  })

  it('shows no enabled access lists available when none exist', async () => {
    const host = sampleHost({ uuid: 'acl-3', name: 'AclHost3' })
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('AclHost3')).toBeInTheDocument())
    await userEvent.click(screen.getByLabelText('Select AclHost3'))
    await userEvent.click(screen.getByText('Manage ACL'))

    await waitFor(() => expect(screen.getByText('No enabled access lists available')).toBeInTheDocument())
  })

  it('bulk delete modal lists hosts to be deleted', async () => {
    const host = sampleHost({ uuid: 'd2', name: 'DeleteMe2' })
    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))
    vi.doMock('../../api/backups', () => ({ createBackup: vi.fn(async () => ({ filename: 'backup-2' })) }))

    const toastSuccess = vi.fn()
    vi.doMock('react-hot-toast', () => ({ toast: { success: toastSuccess, error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))
    const confirmMock = vi.spyOn(window, 'confirm').mockImplementation(() => true)

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await userEvent.click(screen.getByLabelText('Select DeleteMe2'))
    const deleteButtons = screen.getAllByText('Delete')
    const toolbarBtn = deleteButtons.map((btn: Element) => btn.closest('button') as HTMLButtonElement | null).find((b) => b && b.className.includes('bg-red-600')) as HTMLButtonElement | undefined
    if (!toolbarBtn) throw new Error('Toolbar delete button not found')
    await userEvent.click(toolbarBtn)

    await waitFor(() => expect(screen.getByRole('heading', { name: /Delete 1 Proxy Host/i })).toBeInTheDocument())
    // Ensure the modal lists the host by scoping to the modal content
    const listHeader = screen.getByText('Hosts to be deleted:')
    const modalRoot = listHeader.closest('div')
    expect(modalRoot).toBeTruthy()
    if (modalRoot) {
      const { getByText: getByTextWithin } = within(modalRoot)
      expect(getByTextWithin('DeleteMe2')).toBeInTheDocument()
      expect(getByTextWithin('(a.example.com)')).toBeInTheDocument()
    }
    // Confirm delete
    await userEvent.click(screen.getByRole('button', { name: /Delete Permanently/i }))
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith(expect.stringContaining('Backup created')))
    confirmMock.mockRestore()
  })

  it('bulk apply modal returns early when no keys selected (no-op)', async () => {
    const host = sampleHost({ uuid: 'b1', name: 'BlankHost' })
    const updateHost = vi.fn()

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost, deleteHost: vi.fn(), bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('BlankHost')).toBeInTheDocument())
    // Select host
    await userEvent.click(screen.getByLabelText('Select BlankHost'))
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
    const deleteHostMock = vi.fn().mockResolvedValue(undefined)

    vi.doMock('../../hooks/useProxyHosts', () => ({ useProxyHosts: vi.fn(() => ({ hosts: [host], loading: false, isFetching: false, error: null, createHost: vi.fn(), updateHost: vi.fn(), deleteHost: deleteHostMock, bulkUpdateACL: vi.fn(), isBulkUpdating: false })) }))
    vi.doMock('../../hooks/useCertificates', () => ({ useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })) }))
    vi.doMock('../../hooks/useAccessLists', () => ({ useAccessLists: vi.fn(() => ({ data: [] })) }))
    vi.doMock('../../api/settings', () => ({ getSettings: vi.fn(() => Promise.resolve({})) }))
    vi.doMock('../../api/backups', () => ({ createBackup: vi.fn(async () => ({ filename: 'backup-1' })) }))

    const toastSuccess = vi.fn()
    vi.doMock('react-hot-toast', () => ({ toast: { success: toastSuccess, error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))

    const confirmMock = vi.spyOn(window, 'confirm')
    // First confirm to delete overall, returned true for deletion
    confirmMock.mockImplementation(() => true)

    const { default: ProxyHosts } = await import('../ProxyHosts')
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('DeleteMe')).toBeInTheDocument())
    // Select host
    const selectBtn = screen.getByLabelText('Select DeleteMe')
    await userEvent.click(selectBtn)

    // Open Bulk Delete modal - find the toolbar Delete button near the header
    const deleteButtons = screen.getAllByText('Delete')
    const toolbarBtn = deleteButtons.map((btn: Element) => btn.closest('button') as HTMLButtonElement | null).find((b) => b && b.className.includes('bg-red-600')) as HTMLButtonElement | undefined
    if (!toolbarBtn) throw new Error('Toolbar delete button not found')
    await userEvent.click(toolbarBtn)

    // Confirm Delete in modal
    await userEvent.click(screen.getByRole('button', { name: /Delete Permanently/i }))

    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith(expect.stringContaining('Backup created')))
    confirmMock.mockRestore()
  })
})
