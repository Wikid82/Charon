import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import type { ProxyHost, BulkUpdateACLResponse } from '../../api/proxyHosts'
import ProxyHosts from '../ProxyHosts'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as certificatesApi from '../../api/certificates'
import * as accessListsApi from '../../api/accessLists'
import * as settingsApi from '../../api/settings'
import { toast } from 'react-hot-toast'

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

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

const baseHost = (overrides: Partial<ProxyHost> = {}): ProxyHost => ({
  uuid: 'host-1',
  name: 'Host',
  domain_names: 'example.com',
  forward_host: '127.0.0.1',
  forward_port: 8080,
  forward_scheme: 'http' as const,
  enabled: true,
  ssl_forced: false,
  websocket_support: false,
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

describe('ProxyHosts progress apply', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows progress when applying multiple ACLs', async () => {
    const host1 = baseHost({ uuid: 'h1', name: 'H1' })
    const host2 = baseHost({ uuid: 'h2', name: 'H2' })
    const acls = [
      { id: 1, uuid: 'acl-1', name: 'ACL1', description: 'Test ACL1', enabled: true, type: 'whitelist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
      { id: 2, uuid: 'acl-2', name: 'ACL2', description: 'Test ACL2', enabled: true, type: 'blacklist' as const, ip_rules: '[]', country_codes: '', local_network_only: false, created_at: '2025-01-01', updated_at: '2025-01-01' },
    ]

    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([host1, host2])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue(acls)
    vi.mocked(settingsApi.getSettings).mockResolvedValue({})

    // Create controllable promises for bulkUpdateACL invocations
    const resolvers: Array<(value: BulkUpdateACLResponse) => void> = []
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockImplementation((...args: unknown[]) => {
      const [_hostUUIDs, _aclId] = args
      void _hostUUIDs; void _aclId
      return new Promise((resolve: (v: BulkUpdateACLResponse) => void) => { resolvers.push(resolve); })
    })

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('H1')).toBeTruthy())

    // Select both hosts via select-all
    const checkboxes = screen.getAllByRole('checkbox')
    await userEvent.click(checkboxes[0])

    // Open bulk ACL modal
    await waitFor(() => expect(screen.getByText('Manage ACL')).toBeTruthy())
    await userEvent.click(screen.getByText('Manage ACL'))

    // Wait for ACL list
    await waitFor(() => expect(screen.getByText('ACL1')).toBeTruthy())

    // Select both ACLs
    const aclCheckboxes = screen.getAllByRole('checkbox')
    const adminCheckbox = aclCheckboxes.find(cb => cb.closest('label')?.textContent?.includes('ACL1'))
    const localCheckbox = aclCheckboxes.find(cb => cb.closest('label')?.textContent?.includes('ACL2'))
    if (adminCheckbox) await userEvent.click(adminCheckbox)
    if (localCheckbox) await userEvent.click(localCheckbox)

    // Click Apply; should start progress (total 2)
    const applyBtn = await screen.findByRole('button', { name: /Apply\s*\(2\)/i })
    await userEvent.click(applyBtn)

    // Progress indicator should appear
    await waitFor(() => expect(screen.getByText(/Applying ACLs/)).toBeTruthy())
    // After the first bulk operation starts, we should have a resolver
    await waitFor(() => expect(resolvers.length).toBeGreaterThanOrEqual(1))

    // Resolve first bulk operation to allow the sequential loop to continue
    resolvers[0]({ updated: 2, errors: [] })

    // Wait for the second bulk operation to start and create its resolver
    await waitFor(() => expect(resolvers.length).toBeGreaterThanOrEqual(2))
    // Resolve second operation
    resolvers[1]({ updated: 2, errors: [] })

    await waitFor(() => expect(toast.success).toHaveBeenCalled())
  })

  it('does not open window for same_tab link behavior', async () => {
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([baseHost({ uuid: '1', name: 'One' })])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({ 'ui.domain_link_behavior': 'same_tab' })

    renderWithProviders(<ProxyHosts />)
    await waitFor(() => expect(screen.getByText('One')).toBeTruthy())
    const anchor = screen.getByRole('link', { name: /example\.com/i })
    expect(anchor.getAttribute('target')).toBe('_self')
  })
})

export {}
