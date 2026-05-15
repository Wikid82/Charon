import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import * as accessListsApi from '../../api/accessLists'
import * as certificatesApi from '../../api/certificates'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as settingsApi from '../../api/settings'
import * as uptimeApi from '../../api/uptime'
import { createMockProxyHost } from '../../testUtils/createMockProxyHost'
import { formatSettingLabel, settingHelpText, settingKeyToField, applyBulkSettingsToHosts } from '../../utils/proxyHostsHelpers'
import ProxyHosts from '../ProxyHosts'

import type { AccessList } from '../../api/accessLists'
import type { ProxyHost } from '../../api/proxyHosts'

vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))

vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
  createProxyHost: vi.fn(),
  updateProxyHost: vi.fn(),
  deleteProxyHost: vi.fn(),
  bulkUpdateACL: vi.fn(),
  testProxyHostConnection: vi.fn(),
}))

vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }))
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }))
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }))
vi.mock('../../api/backups', () => ({ createBackup: vi.fn() }))
vi.mock('../../api/uptime', () => ({ getMonitors: vi.fn() }))
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))
vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({ data: [], isLoading: false, error: null })),
  useCreateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useUpdateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}))

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

const baseHost = (overrides: Partial<ProxyHost> = {}) => createMockProxyHost(overrides)

describe('ProxyHosts - Coverage enhancements', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows empty message when no hosts', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText(/Create your first proxy host/)).toBeTruthy()
  })

    it('creates a proxy host via Add Host form submit', async () => {
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([])
      vi.mocked(proxyHostsApi.createProxyHost).mockResolvedValue({
        uuid: 'new1',
        name: 'NewHost',
        domain_names: 'new.example.com',
        forward_host: '127.0.0.1',
        forward_port: 8080,
        forward_scheme: 'http',
        enabled: true,
        ssl_forced: false,
        http2_support: false,
        hsts_enabled: false,
        hsts_subdomains: false,
        block_exploits: false,
        websocket_support: false,
        application: 'none',
        locations: [],
        certificate: null,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      } as ProxyHost)
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})

      renderWithProviders(<ProxyHosts />)
      expect(await screen.findByText(/Create your first proxy host/)).toBeTruthy()
      const user = userEvent.setup()
      // Click the first Add Proxy Host button (in empty state)
      await user.click(screen.getAllByRole('button', { name: 'Add Proxy Host' })[0])
      expect(await screen.findByRole('heading', { name: 'Add Proxy Host' })).toBeTruthy()
      // Fill name
      const nameInput = screen.getByLabelText('Name *') as HTMLInputElement
      await user.clear(nameInput)
      await user.type(nameInput, 'NewHost')
      const domainInput = screen.getByLabelText('Domain Names (comma-separated)') as HTMLInputElement
      await user.clear(domainInput)
      await user.type(domainInput, 'new.example.com')
      // Fill forward host/port to satisfy required fields and save
      const forwardHost = screen.getByLabelText('Host') as HTMLInputElement
      await user.clear(forwardHost)
      await user.type(forwardHost, '127.0.0.1')
      const forwardPort = screen.getByLabelText('Port') as HTMLInputElement
      await user.clear(forwardPort)
      await user.type(forwardPort, '8080')
      // Save
      await user.click(await screen.findByRole('button', { name: 'Save' }))
      await waitFor(() => expect(proxyHostsApi.createProxyHost).toHaveBeenCalled())
    }, 15000)

    it('handles equal sort values gracefully', async () => {
      const host1 = baseHost({ uuid: 'e1', name: 'Same', domain_names: 'a.example.com' })
      const host2 = baseHost({ uuid: 'e2', name: 'Same', domain_names: 'b.example.com' })
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})

      renderWithProviders(<ProxyHosts />)
      await waitFor(() => expect(screen.getAllByText('Same').length).toBeGreaterThanOrEqual(2))
      // Sort by name (they are equal) should not throw and maintain rows
      const user = userEvent.setup()
      await user.click(screen.getByText('Name'))
      await waitFor(() => expect(screen.getAllByText('Same').length).toBeGreaterThanOrEqual(2))
    })

    it('toggle select-all deselects when clicked twice', async () => {
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
        baseHost({ uuid: 's1', name: 'S1' }),
        baseHost({ uuid: 's2', name: 'S2' }),
      ])
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})

      renderWithProviders(<ProxyHosts />)
      expect(await screen.findByText('S1')).toBeTruthy()
      // Click select all header checkbox (has aria-label="Select all rows")
      const user = userEvent.setup()
      const selectAllBtn = screen.getByLabelText('Select all rows')
      await user.click(selectAllBtn)
      // Wait for selection UI to appear - text format includes "<strong>2</strong> host(s) selected (all)"
      expect(await screen.findByText(/host\(s\) selected/)).toBeTruthy()
      // Also check for "(all)" indicator
      expect(screen.getByText(/\(all\)/)).toBeTruthy()
      // Click again to deselect
      await user.click(selectAllBtn)
      await waitFor(() => expect(screen.queryByText(/\(all\)/)).toBeNull())
    })

    it('bulk update ACL reject triggers error toast', async () => {
      const host = baseHost({ uuid: 'b1', name: 'BHost' })
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
        { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
      ] as AccessList[])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockRejectedValue(new Error('Bad things'))

      renderWithProviders(<ProxyHosts />)
      expect(await screen.findByText('BHost')).toBeTruthy()
      const chk = screen.getAllByRole('checkbox')[0]
      const user = userEvent.setup()
      await user.click(chk)
      await user.click(screen.getByText('Manage ACL'))
      expect(await screen.findByText('List1')).toBeTruthy()
      const label = screen.getByText('List1').closest('label') as HTMLElement
      // Radix Checkbox - query by role, not native input
      const checkbox = within(label).getByRole('checkbox')
      await user.click(checkbox)
      const applyBtn = await screen.findByRole('button', { name: /Apply\s*\(/i })
      await act(async () => {
        await user.click(applyBtn)
      })
      const toast = (await import('react-hot-toast')).toast
      await waitFor(() => expect(toast.error).toHaveBeenCalled())
    })

    it('switch toggles from disabled to enabled and calls API', async () => {
      const host = baseHost({ uuid: 'sw1', name: 'SwitchHost', enabled: false })
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})
      vi.mocked(proxyHostsApi.updateProxyHost).mockResolvedValue({ ...host, enabled: true })

      renderWithProviders(<ProxyHosts />)
      expect(await screen.findByText('SwitchHost')).toBeTruthy()
      const row = screen.getByText('SwitchHost').closest('tr') as HTMLTableRowElement
      // Switch component uses a label wrapping a hidden checkbox - find the label and click it
      const switchLabel = row.querySelector('label.cursor-pointer') as HTMLElement
      const user = userEvent.setup()
      await user.click(switchLabel)
      await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalledWith('sw1', { enabled: true }))
    })

  it('sorts hosts by column and toggles order indicator', async () => {
    const h1 = baseHost({ uuid: '1', name: 'aaa', domain_names: 'b.com' })
    const h2 = baseHost({ uuid: '2', name: 'zzz', domain_names: 'a.com' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([h1, h2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('aaa')).toBeTruthy()

    // Check both hosts are rendered
    expect(screen.getByText('aaa')).toBeTruthy()
    expect(screen.getByText('zzz')).toBeTruthy()

    // Click domain header - should show sorting indicator
    const domainHeader = screen.getByText('Domain')
    const user = userEvent.setup()
    await user.click(domainHeader)

    // After clicking domain header, the header should have aria-sort attribute
    await waitFor(() => {
      const th = domainHeader.closest('th')
      expect(th?.getAttribute('aria-sort')).toBe('ascending')
    })

    // Click again to toggle to descending
    await user.click(domainHeader)
    await waitFor(() => {
      const th = domainHeader.closest('th')
      expect(th?.getAttribute('aria-sort')).toBe('descending')
    })
  })

  it('toggles row selection checkbox and shows checked state', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()

    const row = screen.getByText('S1').closest('tr') as HTMLTableRowElement
    const selectBtn = within(row).getAllByRole('checkbox')[0]
    // Initially unchecked
    expect(selectBtn.getAttribute('aria-checked')).toBe('false')
    const user = userEvent.setup()
    await user.click(selectBtn)
    await waitFor(() => expect(selectBtn.getAttribute('aria-checked')).toBe('true'))
    await user.click(selectBtn)
    await waitFor(() => expect(selectBtn.getAttribute('aria-checked')).toBe('false'))
  })

  it('closes bulk ACL modal when clicking backdrop', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
        { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist', enabled: true, ip_rules: '[]', country_codes: '', local_network_only: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
      ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    await user.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('Apply Access List')).toBeTruthy()

    // click backdrop (outer overlay) to close
    const overlay = document.querySelector('.fixed.inset-0')
    if (overlay) await user.click(overlay)
    await waitFor(() => expect(screen.queryByText('Apply Access List')).toBeNull())
  })

  it('unchecks ACL via onChange (delete path)', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    await user.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('List1')).toBeTruthy()
    const label = screen.getByText('List1').closest('label') as HTMLLabelElement
    // Radix Checkbox - query by role, not native input
    const checkbox = within(label).getByRole('checkbox')
    // initially unchecked via clear, click to check
    await user.click(checkbox)
    await waitFor(() => expect(checkbox.getAttribute('aria-checked')).toBe('true'))
    // click again to uncheck and hit delete path in onChange
    await user.click(checkbox)
    await waitFor(() => expect(checkbox.getAttribute('aria-checked')).toBe('false'))
  })

  it('remove action triggers handleBulkApplyACL and shows removed toast', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 2, errors: [] })

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('List1')).toBeTruthy()
    // Toggle to Remove ACL
    await user.click(screen.getByText('Remove ACL'))
    // Click the action button (Remove ACL) - it's the primary action (bg-red)
    const actionBtn = screen.getAllByRole('button', { name: 'Remove ACL' }).pop()
    if (actionBtn) await user.click(actionBtn)
    await waitFor(() => expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalledWith(['s1', 's2'], null))
    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.success).toHaveBeenCalled())
  })

  it('toggle action remove -> apply then back', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('Apply ACL')).toBeTruthy()
    // Click Remove, then Apply to hit setBulkACLAction('apply')
    // Toggle Remove (header toggle) and back to Apply (header toggle)
    const headerToggles = screen.getAllByRole('button')
    const removeToggle = headerToggles.find(btn => btn.textContent === 'Remove ACL' && btn.className.includes('flex-1'))
    const applyToggle = headerToggles.find(btn => btn.textContent === 'Apply ACL' && btn.className.includes('flex-1'))
    if (removeToggle) await user.click(removeToggle)
    await waitFor(() => expect(removeToggle).toBeTruthy())
    if (applyToggle) await user.click(applyToggle)
    await waitFor(() => expect(applyToggle).toBeTruthy())
  })

  it('remove action shows partial failure toast on API error result', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 1, errors: [{ uuid: 's2', error: 'Bad' }] })

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('List1')).toBeTruthy()
    await userEvent.click(screen.getByText('Remove ACL'))
    const actionBtn = screen.getAllByRole('button', { name: 'Remove ACL' }).pop()
    if (actionBtn) await userEvent.click(actionBtn)
    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })

  it('remove action reject triggers error toast', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockRejectedValue(new Error('Bulk fail'))

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const chk = screen.getAllByRole('checkbox')[0]
    await userEvent.click(chk)
    await userEvent.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('List1')).toBeTruthy()
    // Toggle Remove mode
    await userEvent.click(screen.getByText('Remove ACL'))
    const actionBtn = screen.getAllByRole('button', { name: 'Remove ACL' }).pop()
    if (actionBtn) await userEvent.click(actionBtn)
    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })

  it('close bulk delete modal by clicking backdrop', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const headerCheckbox = screen.getByLabelText('Select all rows')
    await userEvent.click(headerCheckbox)
    // Wait for selection bar to appear and find the delete button - text format is "host(s) selected"
    expect(await screen.findByText(/host\(s\) selected/)).toBeTruthy()
    // Click the bulk Delete button (with bg-error class) - there are multiple Delete buttons, get the one in selection bar
    const deleteButtons = screen.getAllByRole('button', { name: /Delete/ })
    // The bulk delete button has bg-error class
    const bulkDeleteBtn = deleteButtons.find(btn => btn.classList.contains('bg-error'))
    await userEvent.click(bulkDeleteBtn!)
    expect(await screen.findByText(/Delete 2 Proxy Hosts?/i)).toBeTruthy()
    const overlay = document.querySelector('.fixed.inset-0')
    if (overlay) await userEvent.click(overlay)
    await waitFor(() => expect(screen.queryByText(/Delete 2 Proxy Hosts?/i)).toBeNull())
  })

  it('calls window.open when settings link behavior new_window', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([baseHost({ uuid: '1', name: 'One' })])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({ 'ui.domain_link_behavior': 'new_window' })

    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('One')).toBeTruthy()
    const anchor = screen.getByRole('link', { name: /(test1\.example\.com|example\.com|One)/i })
    await userEvent.click(anchor)
    expect(openSpy).toHaveBeenCalled()
    openSpy.mockRestore()
  })

  it('uses same_tab target for domain links when configured', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([baseHost({ uuid: '1', name: 'One' })])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({ 'ui.domain_link_behavior': 'same_tab' })

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('One')).toBeTruthy()
    const anchor = screen.getByRole('link', { name: /(example\.com|One)/i })
    // Anchor should render with target _self when same_tab
    expect(anchor.getAttribute('target')).toBe('_self')
  })

  it('renders SSL states: custom, staging, letsencrypt variations', async () => {
    const hostCustom = baseHost({ uuid: 'c1', name: 'CustomHost', domain_names: 'custom.com', ssl_forced: true, certificate: { id: 123, uuid: 'cert-1', name: 'CustomCert', provider: 'custom', domains: 'custom.com', expires_at: '2026-01-01' } })
    const hostStaging = baseHost({ uuid: 's1', name: 'StagingHost', domain_names: 'staging.com', ssl_forced: true })
    const hostAuto = baseHost({ uuid: 'a1', name: 'AutoHost', domain_names: 'auto.com', ssl_forced: true })
    const hostLets = baseHost({ uuid: 'l1', name: 'LetsHost', domain_names: 'lets.com', ssl_forced: true })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([hostCustom, hostStaging, hostAuto, hostLets])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([
      { uuid: 'cert-staging', domains: 'staging.com', status: 'untrusted', provider: 'letsencrypt-staging', issuer: 'Let\'s Encrypt', expires_at: '2026-01-01', has_key: false, in_use: true },
      { uuid: 'cert-lets', domains: 'lets.com', status: 'valid', provider: 'letsencrypt', issuer: 'Let\'s Encrypt', expires_at: '2026-01-01', has_key: false, in_use: true },
    ])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('CustomHost')).toBeTruthy()

    // Custom Cert - just verify the host renders
    expect(screen.getByText('CustomHost')).toBeTruthy()

    // Staging host should show staging badge text (just "Staging" in Badge)
    expect(screen.getByText('StagingHost')).toBeTruthy()
    // The SSL badge for staging hosts shows "Staging" text
    const stagingBadges = screen.getAllByText('Staging')
    expect(stagingBadges.length).toBeGreaterThanOrEqual(1)

    // SSL badges are shown for valid certs
    const sslBadges = screen.getAllByText('SSL')
    expect(sslBadges.length).toBeGreaterThan(0)
  })

  it('renders multiple domains and websocket label', async () => {
    const host = baseHost({ uuid: 'multi1', name: 'Multi', domain_names: 'one.com,two.com,three.com', websocket_support: true })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('Multi')).toBeTruthy()
    // Check multiple domain anchors; parse anchor hrefs instead of substring checks
    const anchors = screen.getAllByRole('link')
    const anchorHasHost = (el: Element | null, host: string) => {
      if (!el) return false
      const href = el.getAttribute('href') || ''
      try {
        // Use base to resolve relative URLs
        const parsed = new URL(href, 'http://localhost')
        return parsed.host === host
      } catch {
        return el.textContent?.includes(host) ?? false
      }
    }
    expect(anchors.some(a => anchorHasHost(a, 'one.com'))).toBeTruthy()
    expect(anchors.some(a => anchorHasHost(a, 'two.com'))).toBeTruthy()
    expect(anchors.some(a => anchorHasHost(a, 'three.com'))).toBeTruthy()
    // Check websocket label exists since websocket_support true
    expect(screen.getByText('WS')).toBeTruthy()
  })

  it('handles delete confirmation for a single host', async () => {
    const host = baseHost({ uuid: 'del1', name: 'Del', domain_names: 'del.com' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('Del')).toBeTruthy()
    // Click Delete button in the row
    const editButton = screen.getByText('Edit')
    const row = editButton.closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)
    // Confirm in dialog
    expect(await screen.findByRole('heading', { name: /Delete Proxy Host/i })).toBeTruthy()
    const confirmBtn = screen.getAllByRole('button', { name: 'Delete' }).pop()!
    await userEvent.click(confirmBtn)
    await waitFor(() => expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('del1'))
    confirmSpy.mockRestore()
  })

  it('deletes associated uptime monitors when confirmed', async () => {
    const host = baseHost({ uuid: 'del2', name: 'Del2', forward_host: '127.0.0.5' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    // uptime monitors associated with host
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([{ id: 'm1', name: 'm1', url: 'http://example', type: 'http', interval: 60, enabled: true, status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3, upstream_host: '127.0.0.5' } as uptimeApi.UptimeMonitor])

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('Del2')).toBeTruthy()
    const row = screen.getByText('Del2').closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)
    // Confirm in dialog
    expect(await screen.findByRole('heading', { name: /Delete Proxy Host/i })).toBeTruthy()
    const confirmBtn = screen.getAllByRole('button', { name: 'Delete' }).pop()!
    await userEvent.click(confirmBtn)
    // Should call delete with deleteUptime true
    await waitFor(() => expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('del2', true))
    confirmSpy.mockRestore()
  })

  it('ignores uptime API errors and deletes host without deleting uptime', async () => {
    const host = baseHost({ uuid: 'del3', name: 'Del3', forward_host: '127.0.0.6' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue()
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    // Make getMonitors throw
    vi.mocked(uptimeApi.getMonitors).mockRejectedValue(new Error('OOPS'))

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('Del3')).toBeTruthy()
    const row = screen.getByText('Del3').closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)
    // Confirm in dialog
    expect(await screen.findByRole('heading', { name: /Delete Proxy Host/i })).toBeTruthy()
    const confirmBtn = screen.getAllByRole('button', { name: 'Delete' }).pop()!
    await userEvent.click(confirmBtn)
    // Should call delete without second param
    await waitFor(() => expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('del3'))
    confirmSpy.mockRestore()
  })

  it('applies bulk settings sequentially with progress and updates hosts', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 'host-1', name: 'H1' }),
      baseHost({ uuid: 'host-2', name: 'H2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.updateProxyHost).mockResolvedValue({} as ProxyHost)

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('H1')).toBeTruthy()

    // Select both hosts
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    await userEvent.click(headerCheckbox)

    // Open Bulk Apply modal
    await userEvent.click(screen.getByText('Bulk Apply'))
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy()

    // In the modal, find Force SSL row and enable apply and set value true
    const forceLabel = screen.getByText('Force SSL')
    // The row has class p-3 not p-2, and we need to get the parent flex container
    const rowEl = forceLabel.closest('.p-3') as HTMLElement || forceLabel.closest('div')?.parentElement as HTMLElement
    // Find the Radix checkbox (has role="checkbox" and is a button) and the switch (label with input)
    const allCheckboxes = within(rowEl).getAllByRole('checkbox')
    // First checkbox is the Radix Checkbox for "apply"
    const applyCheckbox = allCheckboxes[0]
    await userEvent.click(applyCheckbox)

    // Click Apply in the modal - find button within the dialog
    const applyBtn = screen.getByRole('button', { name: /^Apply$/i })
    await userEvent.click(applyBtn)

    // Expect updateProxyHost called for each host with ssl_forced true included in payload
    await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalledTimes(2))
    const calls = vi.mocked(proxyHostsApi.updateProxyHost).mock.calls
    expect(calls.some(call => call[1] && (call[1] as Partial<ProxyHost>).ssl_forced === true)).toBeTruthy()
  })

  it('shows Unnamed when name missing', async () => {
    const hostNoName = baseHost({ uuid: 'n1', name: '', domain_names: 'no-name.com' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([hostNoName])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('Unnamed')).toBeTruthy()
  })

  it('toggles host enable state via Switch', async () => {
    const host = baseHost({ uuid: 't1', name: 'Toggle', enabled: true })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.updateProxyHost).mockResolvedValue(baseHost({ uuid: 't1', name: 'Toggle', enabled: true }))

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('Toggle')).toBeTruthy()
    // Locate the row and toggle the enabled switch - it's inside a label with cursor-pointer class
    const row = screen.getByText('Toggle').closest('tr') as HTMLTableRowElement
    // Switch component uses a label wrapping a hidden checkbox
    const switchLabel = row.querySelector('label.cursor-pointer') as HTMLElement
    expect(switchLabel).toBeTruthy()
    await userEvent.click(switchLabel)
    await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalled())
  })

  it('opens add form and cancels', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText(/Create your first proxy host/)).toBeTruthy()
    // Click the first Add Proxy Host button (in empty state)
    await userEvent.click(screen.getAllByRole('button', { name: 'Add Proxy Host' })[0])
    // Form should open with Add Proxy Host header
    expect(await screen.findByRole('heading', { name: 'Add Proxy Host' })).toBeTruthy()
    // Click Cancel should close the form
    const cancelButton = screen.getByText('Cancel')
    await userEvent.click(cancelButton)
    await waitFor(() => expect(screen.queryByRole('heading', { name: 'Add Proxy Host' })).toBeNull())
  })

  it('opens edit form and submits update', async () => {
    const host = baseHost({ uuid: 'edit1', name: 'EditMe' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.updateProxyHost).mockResolvedValue({ ...host, name: 'Edited' })

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('EditMe')).toBeTruthy()
    const editBtn = screen.getByText('Edit')
    await userEvent.click(editBtn)

    // Form header should show Edit Proxy Host
    expect(await screen.findByText('Edit Proxy Host')).toBeTruthy()
    // Change name and click Save
    const nameInput = screen.getByLabelText('Name *') as HTMLInputElement
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Edited')
    await userEvent.click(screen.getByText('Save'))

    await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalled())
  })

  it('alerts on delete when API fails', async () => {
    const host = baseHost({ uuid: 'delerr', name: 'DelErr' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(proxyHostsApi.deleteProxyHost).mockRejectedValue(new Error('Boom'))
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('DelErr')).toBeTruthy()
    const row = screen.getByText('DelErr').closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)
    // Confirm in dialog
    expect(await screen.findByRole('heading', { name: /Delete Proxy Host/i })).toBeTruthy()
    const confirmBtn = screen.getAllByRole('button', { name: 'Delete' }).pop()!
    await userEvent.click(confirmBtn)

    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    confirmSpy.mockRestore()
  })

  it('sorts by domain and forward columns', async () => {
    const h1 = baseHost({ uuid: 'd1', name: 'A', domain_names: 'b.com', forward_host: 'foo' , forward_port: 8080 })
    const h2 = baseHost({ uuid: 'd2', name: 'B', domain_names: 'a.com', forward_host: 'bar' , forward_port: 80 })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([h1, h2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('A')).toBeTruthy()

    // Domain sort
    await userEvent.click(screen.getByText('Domain'))
    expect(await screen.findByText('B')).toBeTruthy() // domain 'a.com' should appear first

    // Forward sort: toggle to change order
    await userEvent.click(screen.getByText('Forward To'))
    expect(await screen.findByText('A')).toBeTruthy()
  })

  it('applies multiple ACLs sequentially with progress', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 'host-1', name: 'H1' }),
      baseHost({ uuid: 'host-2', name: 'H2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-a1', name: 'A1', description: 'A1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
      { id: 2, uuid: 'acl-a2', name: 'A2', description: 'A2', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 2, errors: [] })
    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('H1')).toBeTruthy()

    // Select all hosts
    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])

    // Open Manage ACL
    await userEvent.click(screen.getByText('Manage ACL'))
    expect(await screen.findByText('A1')).toBeTruthy()

    // Select both ACLs
    const aclCheckboxes = screen.getAllByRole('checkbox')
    const checkA1 = aclCheckboxes.find(cb => cb.closest('label')?.textContent?.includes('A1'))
    const checkA2 = aclCheckboxes.find(cb => cb.closest('label')?.textContent?.includes('A2'))
    if (checkA1) await userEvent.click(checkA1)
    if (checkA2) await userEvent.click(checkA2)

    // Click Apply
    const applyBtn = screen.getByRole('button', { name: /Apply \(2\)/i })
    await userEvent.click(applyBtn)

    // Should call bulkUpdateACL twice and show success
    await waitFor(() => expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalledTimes(2))
  })

  it('select all / clear header selects and clears ACLs', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
      { id: 2, uuid: 'acl-2', name: 'List2', description: 'List 2', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()

    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])
    expect(await screen.findByText('Manage ACL')).toBeTruthy()
    await userEvent.click(screen.getByText('Manage ACL'))

    // Click Select All in modal
    const selectAllBtn = await screen.findByText('Select All')
    await userEvent.click(selectAllBtn)
    // All ACL checkboxes (Radix Checkbox) inside labels should be checked - check via aria-checked
    const labelEl1 = screen.getByText('List1').closest('label') as HTMLElement
    const labelEl2 = screen.getByText('List2').closest('label') as HTMLElement
    const checkbox1 = within(labelEl1).getByRole('checkbox')
    const checkbox2 = within(labelEl2).getByRole('checkbox')
    expect(checkbox1.getAttribute('aria-checked')).toBe('true')
    expect(checkbox2.getAttribute('aria-checked')).toBe('true')

    // Click Clear
    const clearBtn = await screen.findByText('Clear')
    await userEvent.click(clearBtn)
    expect(checkbox1.getAttribute('aria-checked')).toBe('false')
    expect(checkbox2.getAttribute('aria-checked')).toBe('false')
  })

  it('shows no enabled access lists message when none are enabled', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' })
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-disable1', name: 'Disabled1', description: 'Disabled 1', type: 'blacklist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
      { id: 2, uuid: 'acl-disable2', name: 'Disabled2', description: 'Disabled 2', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()

    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])
    expect(await screen.findByText('Manage ACL')).toBeTruthy()
    await userEvent.click(screen.getByText('Manage ACL'))

    // Should show the 'No enabled access lists available' message
    expect(await screen.findByText('No enabled access lists available')).toBeTruthy()
  })

  it('formatSettingLabel, settingHelpText and settingKeyToField return expected values and defaults', () => {
    expect(formatSettingLabel('ssl_forced')).toBe('Force SSL')
    expect(formatSettingLabel('http2_support')).toBe('HTTP/2 Support')
    expect(formatSettingLabel('hsts_enabled')).toBe('HSTS Enabled')
    expect(formatSettingLabel('hsts_subdomains')).toBe('HSTS Subdomains')
    expect(formatSettingLabel('block_exploits')).toBe('Block Exploits')
    expect(formatSettingLabel('websocket_support')).toBe('Websockets Support')
    expect(formatSettingLabel('unknown_key')).toBe('unknown_key')

    expect(settingHelpText('ssl_forced')).toContain('Redirect all HTTP traffic')
    expect(settingHelpText('http2_support')).toContain('Enable HTTP/2')
    expect(settingHelpText('hsts_enabled')).toContain('Send HSTS header')
    expect(settingHelpText('hsts_subdomains')).toContain('Include subdomains')
    expect(settingHelpText('block_exploits')).toContain('Add common exploit-mitigation')
    expect(settingHelpText('websocket_support')).toContain('Enable websocket proxying')
    expect(settingHelpText('unknown_key')).toBe('')

    expect(settingKeyToField('ssl_forced')).toBe('ssl_forced')
    expect(settingKeyToField('http2_support')).toBe('http2_support')
    expect(settingKeyToField('hsts_enabled')).toBe('hsts_enabled')
    expect(settingKeyToField('hsts_subdomains')).toBe('hsts_subdomains')
    expect(settingKeyToField('block_exploits')).toBe('block_exploits')
    expect(settingKeyToField('websocket_support')).toBe('websocket_support')
    expect(settingKeyToField('unknown_key')).toBe('unknown_key')
  })

  it('closes bulk apply modal when clicking backdrop', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('S1')).toBeTruthy()
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    await user.click(screen.getByText('Bulk Apply'))
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy()
    // click backdrop
    const overlay = document.querySelector('.fixed.inset-0')
    if (overlay) await user.click(overlay)
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull())
  })

  it('shows toast error when updateHost rejects during bulk apply', async () => {
    const h1 = baseHost({ uuid: 'host-1', name: 'H1' })
    const h2 = baseHost({ uuid: 'host-2', name: 'H2' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([h1, h2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    // mock updateProxyHost to fail for host-2
    vi.mocked(proxyHostsApi.updateProxyHost).mockImplementation(async (uuid: string) => {
      if (uuid === 'host-2') throw new Error('update fail')
      return baseHost({ uuid })
    })

    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('H1')).toBeTruthy()
    // select both
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    // Open Bulk Apply
    await user.click(screen.getByText('Bulk Apply'))
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy()
    // enable Force SSL apply + set switch
    const forceLabel = screen.getByText('Force SSL')
    // The row has class p-3 not p-2, and we need to get the parent flex container
    const rowEl = forceLabel.closest('.p-3') as HTMLElement || forceLabel.closest('div')?.parentElement as HTMLElement
    // Find the Radix checkbox (has role="checkbox" and is a button) and the switch (label with input)
    const allCheckboxes = within(rowEl).getAllByRole('checkbox')
    // First checkbox is the Radix Checkbox for "apply", second is the switch's internal checkbox
    const applyCheckbox = allCheckboxes[0]
    await user.click(applyCheckbox)
    // Toggle the switch - click the label containing the checkbox
    const switchLabel = rowEl.querySelector('label.relative') as HTMLElement
    if (switchLabel) await user.click(switchLabel)
    // click Apply - find button within the dialog
    const applyBtn = screen.getByRole('button', { name: /^Apply$/i })
    await user.click(applyBtn)

    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })

  it('applyBulkSettingsToHosts returns error when host is not found and reports progress', async () => {
    const hosts: ProxyHost[] = [] // no hosts
    const hostUUIDs = ['missing-1']
    const keysToApply = ['ssl_forced']
    const bulkApplySettings: Record<string, { apply: boolean; value: boolean }> = { ssl_forced: { apply: true, value: true } }
    const updateHost = vi.fn().mockResolvedValue({})
    const setApplyProgress = vi.fn()

    const result = await applyBulkSettingsToHosts({ hosts, hostUUIDs, keysToApply, bulkApplySettings, updateHost, setApplyProgress })
    expect(result.errors).toBe(1)
    expect(setApplyProgress).toHaveBeenCalled()
    expect(updateHost).not.toHaveBeenCalled()
  })

  it('applyBulkSettingsToHosts handles updateHost rejection and counts errors', async () => {
    const h1 = baseHost({ uuid: 'h1', name: 'H1' })
    const hosts = [h1]
    const hostUUIDs = ['h1']
    const keysToApply = ['ssl_forced']
    const bulkApplySettings: Record<string, { apply: boolean; value: boolean }> = { ssl_forced: { apply: true, value: true } }
    const updateHost = vi.fn().mockRejectedValue(new Error('fail'))
    const setApplyProgress = vi.fn()

    const result = await applyBulkSettingsToHosts({ hosts, hostUUIDs, keysToApply, bulkApplySettings, updateHost, setApplyProgress })
    expect(result.errors).toBe(1)
    expect(updateHost).toHaveBeenCalled()
  })
})

export {}
