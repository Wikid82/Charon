import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { toast } from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { getSettings } from '../../api/settings'
import { getMonitors } from '../../api/uptime'
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
  bulkUpdateGroup: vi.fn() as unknown as ProxyHostsHookValue['bulkUpdateGroup'],
  isBulkUpdatingGroup: false,
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

describe('ProxyHosts passive note for hosts left out of the configuration', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue())
    vi.mocked(useCertificates).mockReturnValue(createCertificatesHookValue())
    vi.mocked(useAccessLists).mockReturnValue(createAccessListsHookValue([]))
    vi.mocked(getSettings).mockResolvedValue({})
    vi.mocked(getMonitors).mockResolvedValue([])
  })

  it('shows the note only on the host left out of the active configuration', async () => {
    const left = sampleHost({
      uuid: 'left',
      name: 'Left',
      domain_names: 'left.example.com',
      forward_host: '169.254.169.254',
      active_config: false,
    })
    const active = sampleHost({ uuid: 'active', name: 'Active', domain_names: 'active.example.com' })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [left, active] }))

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('left.example.com')).toBeInTheDocument()
    expect(screen.getAllByText('Not in the active configuration')).toHaveLength(1)
    expect(screen.getAllByTestId('proxy-host-not-in-active-config')).toHaveLength(1)
  })

  it('shows no note when every host is active', async () => {
    const active = sampleHost({ uuid: 'active', name: 'Active', domain_names: 'active.example.com', active_config: undefined })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [active] }))

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('active.example.com')).toBeInTheDocument()
    expect(screen.queryByText('Not in the active configuration')).not.toBeInTheDocument()
  })

  it('shows no toast or banner for the left-out host', async () => {
    const left = sampleHost({ uuid: 'left', domain_names: 'left.example.com', active_config: false })
    vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsHookValue({ hosts: [left] }))

    renderWithProviders(<ProxyHosts />)

    expect(await screen.findByText('left.example.com')).toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })
})
