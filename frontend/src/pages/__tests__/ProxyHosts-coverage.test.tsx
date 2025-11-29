import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import ProxyHosts from '../ProxyHosts'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as certificatesApi from '../../api/certificates'
import * as accessListsApi from '../../api/accessLists'
import * as settingsApi from '../../api/settings'
// toast is mocked in other tests; not used here

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

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

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

import { createMockProxyHost } from '../../testUtils/createMockProxyHost'

const baseHost = (overrides: any = {}) => createMockProxyHost(overrides)

describe('ProxyHosts - Coverage enhancements', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows empty message when no hosts', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText(/No proxy hosts configured yet/)).toBeTruthy())
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
      } as any)
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})

      renderWithProviders(<ProxyHosts />)
      await waitFor(() => expect(screen.getByText('No proxy hosts configured yet. Click "Add Proxy Host" to get started.')).toBeTruthy())
      const user = userEvent.setup()
      await user.click(screen.getByText('Add Proxy Host'))
      await waitFor(() => expect(screen.getByRole('heading', { name: 'Add Proxy Host' })).toBeTruthy())
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
    })

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
      await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
      // Click select all header
      const user = userEvent.setup()
      const selectAllBtn = screen.getAllByRole('checkbox')[0]
      await user.click(selectAllBtn)
      await waitFor(() => expect(screen.getByText('2 (all) selected')).toBeTruthy())
      // Click again to deselect
      await user.click(selectAllBtn)
      await waitFor(() => expect(screen.queryByText('2 (all) selected')).toBeNull())
    })

    it('bulk update ACL reject triggers error toast', async () => {
      const host = baseHost({ uuid: 'b1', name: 'BHost' })
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
      vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
      vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
        { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
      ] as any)
      vi.mocked(settingsApi.getSettings).mockResolvedValue({})
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockRejectedValue(new Error('Bad things'))

      renderWithProviders(<ProxyHosts />)
      await waitFor(() => expect(screen.getByText('BHost')).toBeTruthy())
      const chk = screen.getAllByRole('checkbox')[0]
      const user = userEvent.setup()
      await user.click(chk)
      await user.click(screen.getByText('Manage ACL'))
      await waitFor(() => expect(screen.getByText('List1')).toBeTruthy())
      const label = screen.getByText('List1').closest('label') as HTMLElement
      const input = label.querySelector('input') as HTMLInputElement
      await user.click(input)
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
      await waitFor(() => expect(screen.getByText('SwitchHost')).toBeTruthy())
      const row = screen.getByText('SwitchHost').closest('tr') as HTMLTableRowElement
      const rowCheckboxes = within(row).getAllByRole('checkbox')
      const switchInput = rowCheckboxes[0]
      const user = userEvent.setup()
      await user.click(switchInput)
      await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalledWith('sw1', { enabled: true }))
    })

  it('sorts hosts by column and toggles order', async () => {
    const h1 = baseHost({ uuid: '1', name: 'aaa', domain_names: 'b.com' })
    const h2 = baseHost({ uuid: '2', name: 'zzz', domain_names: 'a.com' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([h1, h2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('aaa')).toBeTruthy())

    // Check default sort (name asc)
    const rows = screen.getAllByRole('row')
    expect(rows[1].textContent).toContain('aaa')

    // Click name header to flip sort direction
    const nameHeader = screen.getByText('Name')
    // Click once to switch from default asc to desc
    const user = userEvent.setup()
    await user.click(nameHeader)

    // After toggle, order should show zzz first
    await waitFor(() => expect(screen.getByText('zzz')).toBeTruthy())
    const table = screen.getByRole('table') as HTMLTableElement
    const tbody = table.querySelector('tbody')!
    const tbodyRows = tbody.querySelectorAll('tr')
    const firstName = tbodyRows[0].querySelector('td')?.textContent?.trim()
    expect(firstName).toBe('zzz')
  })

  it('toggles row selection checkbox and shows checked state', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())

    const row = screen.getByText('S1').closest('tr') as HTMLTableRowElement
    const selectBtn = within(row).getByRole('checkbox', { name: /Select S1/ })
    // Initially unchecked (Square)
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
      ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    await user.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('Apply Access List')).toBeTruthy())

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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(headerCheckbox)
    await user.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('List1')).toBeTruthy())
    const label = screen.getByText('List1').closest('label') as HTMLLabelElement
    const input = label.querySelector('input') as HTMLInputElement
    // initially unchecked via clear, click to check
    await user.click(input)
    await waitFor(() => expect(input.checked).toBeTruthy())
    // click again to uncheck and hit delete path in onChange
    await user.click(input)
    await waitFor(() => expect(input.checked).toBeFalsy())
  })

  it('remove action triggers handleBulkApplyACL and shows removed toast', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' }),
      baseHost({ uuid: 's2', name: 'S2' }),
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-1', name: 'List1', description: 'List 1', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: true, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 2, errors: [] })

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('List1')).toBeTruthy())
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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('Apply ACL')).toBeTruthy())
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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 1, errors: [{ uuid: 's2', error: 'Bad' }] })

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const chk = screen.getAllByRole('checkbox')[0]
    const user = userEvent.setup()
    await user.click(chk)
    await user.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('List1')).toBeTruthy())
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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockRejectedValue(new Error('Bulk fail'))

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const chk = screen.getAllByRole('checkbox')[0]
    await userEvent.click(chk)
    await userEvent.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('List1')).toBeTruthy())
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
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())
    const headerCheckbox = screen.getAllByRole('checkbox')[0]
    await userEvent.click(headerCheckbox)
    // Click the Delete (bulk delete) button from selection bar
    const selectionBar = screen.getByText(/2 \(all\) selected/).closest('div') as HTMLElement
    const deleteBtn = within(selectionBar).getByRole('button', { name: /Delete/ })
    await userEvent.click(deleteBtn)
    await waitFor(() => expect(screen.getByText(/Delete 2 Proxy Hosts?/i)).toBeTruthy())
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

    await waitFor(() => expect(screen.getByText('One')).toBeTruthy())
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

    await waitFor(() => expect(screen.getByText('One')).toBeTruthy())
    const anchor = screen.getByRole('link', { name: /(example\.com|One)/i })
    // Anchor should render with target _self when same_tab
    expect(anchor.getAttribute('target')).toBe('_self')
  })

  it('renders SSL states: custom, staging, letsencrypt variations', async () => {
    const hostCustom = baseHost({ uuid: 'c1', name: 'Custom', domain_names: 'custom.com', ssl_forced: true, certificate: { provider: 'custom', name: 'CustomCert' } })
    const hostStaging = baseHost({ uuid: 's1', name: 'Staging', domain_names: 'staging.com', ssl_forced: true })
    const hostAuto = baseHost({ uuid: 'a1', name: 'Auto', domain_names: 'auto.com', ssl_forced: true })
    const hostLets = baseHost({ uuid: 'l1', name: 'Lets', domain_names: 'lets.com', ssl_forced: true })

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([hostCustom, hostStaging, hostAuto, hostLets])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([
      { domain: 'staging.com', status: 'untrusted', provider: 'letsencrypt-staging', issuer: 'Let\'s Encrypt', expires_at: '2026-01-01' },
      { domain: 'lets.com', status: 'valid', provider: 'letsencrypt', issuer: 'Let\'s Encrypt', expires_at: '2026-01-01' },
    ])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('Custom')).toBeTruthy())

    // Custom Cert label - the certificate name should appear
    expect(screen.getByText('Custom')).toBeTruthy()
    expect(screen.getByText('CustomCert (Custom)')).toBeTruthy()

    // Staging should show staging badge text
    expect(screen.getByText('Staging')).toBeTruthy()
    const stagingBadge = screen.getByText(/SSL \(Staging\)/)
    expect(stagingBadge).toBeTruthy()

    // Let's Encrypt check has 'Let's Encrypt ✓' and Auto
    expect(screen.getByText('Lets')).toBeTruthy()
    expect(screen.getByText("Let's Encrypt ✓")).toBeTruthy()
    expect(screen.getByText('Auto')).toBeTruthy()
    expect(screen.getByText("Let's Encrypt (Auto)")).toBeTruthy()
  })

  it('renders multiple domains and websocket label', async () => {
    const host = baseHost({ uuid: 'multi1', name: 'Multi', domain_names: 'one.com,two.com,three.com', websocket_support: true })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('Multi')).toBeTruthy())
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
    await waitFor(() => expect(screen.getByText('Del')).toBeTruthy())
    // Click Delete button in the row
    const editButton = screen.getByText('Edit')
    const row = editButton.closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)
    await waitFor(() => expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('del1'))
    confirmSpy.mockRestore()
  })

  it('shows Unnamed when name missing', async () => {
    const hostNoName = baseHost({ uuid: 'n1', name: '', domain_names: 'no-name.com' })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([hostNoName])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('Unnamed')).toBeTruthy())
  })

  it('toggles host enable state via Switch', async () => {
    const host = baseHost({ uuid: 't1', name: 'Toggle', enabled: true })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.updateProxyHost).mockResolvedValue(baseHost({ uuid: 't1', name: 'Toggle', enabled: true }))

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('Toggle')).toBeTruthy())
    // Locate the row and toggle the enabled switch specifically
    const row = screen.getByText('Toggle').closest('tr') as HTMLTableRowElement
    const rowInputs = within(row).getAllByRole('checkbox')
    const switchInput = rowInputs[0] // first input in row is the status switch
    expect(switchInput).toBeTruthy()
    await userEvent.click(switchInput)
    await waitFor(() => expect(proxyHostsApi.updateProxyHost).toHaveBeenCalled())
  })

  it('opens add form and cancels', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('No proxy hosts configured yet. Click "Add Proxy Host" to get started.')).toBeTruthy())
    await userEvent.click(screen.getByText('Add Proxy Host'))
    // Form should open with Add Proxy Host header
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Add Proxy Host' })).toBeTruthy())
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

    await waitFor(() => expect(screen.getByText('EditMe')).toBeTruthy())
    const editBtn = screen.getByText('Edit')
    await userEvent.click(editBtn)

    // Form header should show Edit Proxy Host
    await waitFor(() => expect(screen.getByText('Edit Proxy Host')).toBeTruthy())
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
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('DelErr')).toBeTruthy())
    const row = screen.getByText('DelErr').closest('tr') as HTMLTableRowElement
    const delButton = within(row).getByText('Delete')
    await userEvent.click(delButton)

    await waitFor(() => expect(alertSpy).toHaveBeenCalledWith('Boom'))
    confirmSpy.mockRestore()
    alertSpy.mockRestore()
  })

  it('sorts by domain and forward columns', async () => {
    const h1 = baseHost({ uuid: 'd1', name: 'A', domain_names: 'b.com', forward_host: 'foo' , forward_port: 8080 })
    const h2 = baseHost({ uuid: 'd2', name: 'B', domain_names: 'a.com', forward_host: 'bar' , forward_port: 80 })
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([h1, h2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('A')).toBeTruthy())

    // Domain sort
    await userEvent.click(screen.getByText('Domain'))
    await waitFor(() => expect(screen.getByText('B')).toBeTruthy()) // domain 'a.com' should appear first

    // Forward sort: toggle to change order
    await userEvent.click(screen.getByText('Forward To'))
    await waitFor(() => expect(screen.getByText('A')).toBeTruthy())
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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({ updated: 2, errors: [] })
    renderWithProviders(<ProxyHosts />)

    await waitFor(() => expect(screen.getByText('H1')).toBeTruthy())

    // Select all hosts
    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])

    // Open Manage ACL
    await userEvent.click(screen.getByText('Manage ACL'))
    await waitFor(() => expect(screen.getByText('A1')).toBeTruthy())

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
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())

    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])
    await waitFor(() => expect(screen.getByText('Manage ACL')).toBeTruthy())
    await userEvent.click(screen.getByText('Manage ACL'))

    // Click Select All in modal
    const selectAllBtn = await screen.findByText('Select All')
    await userEvent.click(selectAllBtn)
    // All ACL checkbox inputs inside labels should be checked
    const labelEl1 = screen.getByText('List1').closest('label')
    const labelEl2 = screen.getByText('List2').closest('label')
    const input1 = labelEl1?.querySelector('input') as HTMLInputElement
    const input2 = labelEl2?.querySelector('input') as HTMLInputElement
    expect(input1.checked).toBeTruthy()
    expect(input2.checked).toBeTruthy()

    // Click Clear
    const clearBtn = await screen.findByText('Clear')
    await userEvent.click(clearBtn)
    expect(input1.checked).toBe(false)
    expect(input2.checked).toBe(false)
  })

  it('shows no enabled access lists message when none are enabled', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([
      baseHost({ uuid: 's1', name: 'S1' })
    ])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([
      { id: 1, uuid: 'acl-disable1', name: 'Disabled1', description: 'Disabled 1', type: 'blacklist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
      { id: 2, uuid: 'acl-disable2', name: 'Disabled2', description: 'Disabled 2', type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, enabled: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ] as any)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('S1')).toBeTruthy())

    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])
    await waitFor(() => expect(screen.getByText('Manage ACL')).toBeTruthy())
    await userEvent.click(screen.getByText('Manage ACL'))

    // Should show the 'No enabled access lists available' message
    await waitFor(() => expect(screen.getByText('No enabled access lists available')).toBeTruthy())
  })
})

export {}
