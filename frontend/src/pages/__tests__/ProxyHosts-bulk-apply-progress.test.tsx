import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import * as accessListsApi from '../../api/accessLists'
import * as certificatesApi from '../../api/certificates'
import * as proxyHostsApi from '../../api/proxyHosts'
import * as settingsApi from '../../api/settings'
import { createMockProxyHost } from '../../testUtils/createMockProxyHost'
import ProxyHosts from '../ProxyHosts'

import type { AccessList } from '../../api/accessLists'
import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'

vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))
vi.mock('../../api/proxyHosts', () => ({ getProxyHosts: vi.fn(), createProxyHost: vi.fn(), updateProxyHost: vi.fn(), deleteProxyHost: vi.fn(), bulkUpdateACL: vi.fn(), testProxyHostConnection: vi.fn() }))
vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }))
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }))
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }))
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

const hosts = [
  createMockProxyHost({ uuid: 'p1', name: 'Progress 1', domain_names: 'p1.example.com' }),
  createMockProxyHost({ uuid: 'p2', name: 'Progress 2', domain_names: 'p2.example.com' }),
]

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ProxyHosts - Bulk Apply progress UI', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(hosts as ProxyHost[])
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([] as Certificate[])
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([] as AccessList[])
    vi.mocked(settingsApi.getSettings).mockResolvedValue({} as Record<string, string>)
  })

  it('shows applying progress while updateProxyHost resolves', async () => {
    // Make updateProxyHost return controllable promises so we can assert the progress UI
    const updateMock = vi.mocked(proxyHostsApi.updateProxyHost)
        const resolvers: Array<(v: ProxyHost) => void> = []
        updateMock.mockImplementation(() => new Promise((res: (v: ProxyHost) => void) => { resolvers.push(res) }))
    renderWithProviders(<ProxyHosts />)
    expect(await screen.findByText('Progress 1')).toBeTruthy()

    // Select all
    const selectAll = screen.getByLabelText('Select all rows')
    await userEvent.click(selectAll)

    // Open Bulk Apply
    await userEvent.click(screen.getByText('Bulk Apply'))
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy()

    // Enable one setting (Force SSL) - use Radix Checkbox (role="checkbox") in the row
    const forceLabel = screen.getByText(/Force SSL/i) as HTMLElement
    const forceRow = forceLabel.closest('.p-3') as HTMLElement
    const { within } = await import('@testing-library/react')
    const forceCheckbox = within(forceRow).getAllByRole('checkbox')[0]
    await userEvent.click(forceCheckbox)

    // Click Apply and assert progress UI appears
    const dialog = screen.getByRole('dialog')
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i })
    await userEvent.click(applyButton)

    // During the small delay the progress text should appear (there are two matching nodes)
    await waitFor(() => expect(screen.getAllByText(/Applying settings/i).length).toBeGreaterThan(0))

    // Resolve both pending update promises to finish the operation
    for (const r of resolvers) r(hosts[0])
    // Ensure subsequent tests aren't blocked by the special mock: make updateProxyHost resolve normally
    updateMock.mockImplementation(() => Promise.resolve(hosts[0] as ProxyHost))

    // Wait for updates to complete
    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(2))
  })
})

export {}
