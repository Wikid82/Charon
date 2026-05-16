import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { getSettings } from '../../api/settings'
import { getMonitors } from '../../api/uptime'
import { useAccessLists } from '../../hooks/useAccessLists'
import { useCertificates } from '../../hooks/useCertificates'
import { useDeleteProxyGroup, useProxyGroups } from '../../hooks/useProxyGroups'
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
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

const createQueryClient = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
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
  ({ data, isLoading: false, isFetching: false, error: null, ...overrides } as unknown as AccessListsHookValue)

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

const makeGroup = (overrides = {}) => ({
  uuid: 'grp-1',
  name: 'Production',
  description: '',
  color: '#6366f1',
  host_count: 0,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  ...overrides,
})

describe('ProxyHosts group rendering', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue())
    vi.mocked(useCertificates).mockReturnValue(createCertificatesHookValue())
    vi.mocked(useAccessLists).mockReturnValue(createAccessListsHookValue([]))
    vi.mocked(getSettings).mockResolvedValue({})
    vi.mocked(getMonitors).mockResolvedValue([])
  })

  it('renders group section headers with name and action buttons', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('Production')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /edit group production/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /delete group production/i })).toBeInTheDocument()
  })

  it('places host with proxy_group into correct group section', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup({ uuid: 'grp-1', name: 'Production' })],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)
    vi.mocked(useProxyHosts).mockReturnValue(
      createProxyHostsHookValue({
        hosts: [
          sampleHost({
            name: 'GroupedHost',
            proxy_group: { uuid: 'grp-1', name: 'Production', color: '#6366f1' },
          }),
        ],
      }),
    )

    renderWithProviders(<ProxyHosts />)

    const section = await screen.findByRole('region', { name: 'Production' })
    expect(within(section).getByText('GroupedHost')).toBeInTheDocument()
  })

  it('opens ProxyGroupForm when clicking Manage Groups', async () => {
    renderWithProviders(<ProxyHosts />)

    await user.click(await screen.findByRole('button', { name: /manage groups/i }))

    expect(await screen.findByText('Create Group')).toBeInTheDocument()
  })

  it('closes ProxyGroupForm and resets state when onClose is called', async () => {
    renderWithProviders(<ProxyHosts />)

    await user.click(await screen.findByRole('button', { name: /manage groups/i }))
    expect(await screen.findByText('Create Group')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /cancel/i }))
    await waitFor(() => expect(screen.queryByText('Create Group')).not.toBeInTheDocument())
  })

  it('opens ProxyGroupForm in edit mode when clicking edit group button', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)

    renderWithProviders(<ProxyHosts />)

    await user.click(await screen.findByRole('button', { name: /edit group production/i }))

    expect(await screen.findByText('Edit Group')).toBeInTheDocument()
  })

  it('opens delete confirmation dialog when clicking delete group button', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)

    renderWithProviders(<ProxyHosts />)

    await user.click(await screen.findByRole('button', { name: /delete group production/i }))

    expect(await screen.findByText('Delete Group')).toBeInTheDocument()
    expect(
      screen.getByText('Delete this group? All hosts will be moved to Ungrouped.'),
    ).toBeInTheDocument()
  })

  it('calls deleteGroup.mutateAsync when confirming group deletion', async () => {
    const deleteMock = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useDeleteProxyGroup).mockReturnValue({
      mutateAsync: deleteMock,
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteProxyGroup>)
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)

    renderWithProviders(<ProxyHosts />)

    await user.click(await screen.findByRole('button', { name: /delete group production/i }))
    await screen.findByText('Delete Group')

    const deleteButtons = screen.getAllByRole('button', { name: /^delete$/i })
    await user.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith('grp-1'))
  })

  it('opens Assign to Group dialog when hosts are selected and groups are present', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)
    vi.mocked(useProxyHosts).mockReturnValue(
      createProxyHostsHookValue({
        hosts: [sampleHost({ uuid: 'h1', name: 'UngroupedHost' })],
      }),
    )

    renderWithProviders(<ProxyHosts />)

    await screen.findByText('UngroupedHost')
    const row = screen.getByText('UngroupedHost').closest('tr') as HTMLTableRowElement
    const checkbox = within(row).getAllByRole('checkbox')[0]
    await user.click(checkbox)

    const assignBtn = await screen.findByRole('button', { name: /assign to group/i })
    await user.click(assignBtn)

    expect(await screen.findByRole('option', { name: 'Production' })).toBeInTheDocument()
  })

  it('calls updateHost for each selected host when saving Assign to Group', async () => {
    const updateHostMock = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [makeGroup()],
      isLoading: false,
      error: null,
    } as ReturnType<typeof useProxyGroups>)
    vi.mocked(useProxyHosts).mockReturnValue(
      createProxyHostsHookValue({
        hosts: [sampleHost({ uuid: 'h1', name: 'UngroupedHost' })],
        updateHost: updateHostMock as unknown as ProxyHostsHookValue['updateHost'],
      }),
    )

    renderWithProviders(<ProxyHosts />)

    await screen.findByText('UngroupedHost')
    const row = screen.getByText('UngroupedHost').closest('tr') as HTMLTableRowElement
    const checkbox = within(row).getAllByRole('checkbox')[0]
    await user.click(checkbox)

    await user.click(await screen.findByRole('button', { name: /assign to group/i }))
    await screen.findByRole('option', { name: 'Production' })

    await user.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() =>
      expect(updateHostMock).toHaveBeenCalledWith('h1', { proxy_group_id: 'grp-1' }),
    )
  })
})
